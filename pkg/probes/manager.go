package probes

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/isitobservable/k8s-networking-mcp/pkg/config"
	"github.com/isitobservable/k8s-networking-mcp/pkg/k8s"
	"github.com/isitobservable/k8s-networking-mcp/pkg/types"
)

var probeTracer = otel.Tracer("mcp-k8s-networking.probes")

// Manager handles the lifecycle of ephemeral diagnostic pods.
type Manager struct {
	cfg     *config.Config
	clients *k8s.Clients

	mu       sync.Mutex
	running  int
	stopOnce sync.Once
	stopCh   chan struct{}
}

// NewManager creates a probe manager and starts the orphan cleanup goroutine.
func NewManager(ctx context.Context, cfg *config.Config, clients *k8s.Clients) *Manager {
	m := &Manager{
		cfg:     cfg,
		clients: clients,
		stopCh:  make(chan struct{}),
	}

	// Clean up orphaned pods on startup
	m.cleanupOrphans(ctx)

	// Start periodic cleanup
	go m.cleanupLoop(ctx)

	return m
}

// Execute runs a probe by creating an ephemeral pod, waiting for completion, and returning the result.
func (m *Manager) Execute(ctx context.Context, req ProbeRequest) (*ProbeResult, error) {
	if err := m.acquireSlot(); err != nil {
		return nil, err
	}
	defer m.releaseSlot()

	// Default timeout
	if req.Timeout == 0 {
		req.Timeout = 30 * time.Second
	}

	ns := req.Namespace
	if ns == "" {
		ns = m.cfg.ProbeNamespace
	}

	// Start parent span for the entire probe lifecycle
	ctx, parentSpan := probeTracer.Start(ctx, fmt.Sprintf("probe/%s", req.Type),
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("probe.type", string(req.Type)),
			attribute.String("k8s.namespace", ns),
			attribute.String("probe.timeout", req.Timeout.String()),
		),
	)
	defer parentSpan.End()

	// Create a timeout context for the probe
	probeCtx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	start := time.Now()

	// Deploy: create the pod
	podName, err := m.deployProbe(probeCtx, ns, req)
	if err != nil {
		parentSpan.RecordError(err)
		parentSpan.SetStatus(codes.Error, "deploy failed")
		return nil, fmt.Errorf("failed to create probe pod: %w", err)
	}

	parentSpan.SetAttributes(attribute.String("k8s.pod.name", podName))

	// Always clean up the pod
	defer func() {
		m.cleanupProbe(ctx, ns, podName)
	}()

	// Wait + execute: wait for the pod to complete and collect output
	result, err := m.waitProbe(probeCtx, ns, podName)
	if err != nil {
		if probeCtx.Err() != nil {
			parentSpan.AddEvent("probe.timeout", trace.WithAttributes(
				attribute.String("probe.timeout_duration", req.Timeout.String()),
			))
			parentSpan.SetStatus(codes.Error, "probe timed out")
			return &ProbeResult{
				Success:  false,
				Error:    "probe timed out",
				Duration: time.Since(start),
			}, &types.MCPError{
				Code:    types.ErrCodeProbeTimeout,
				Message: fmt.Sprintf("probe timed out after %s", req.Timeout),
			}
		}
		parentSpan.RecordError(err)
		parentSpan.SetStatus(codes.Error, "execution failed")
		return nil, fmt.Errorf("probe execution failed: %w", err)
	}

	result.Duration = time.Since(start)
	parentSpan.SetAttributes(
		attribute.Bool("probe.success", result.Success),
		attribute.Int("probe.exit_code", result.ExitCode),
	)
	if !result.Success {
		parentSpan.SetStatus(codes.Error, result.Error)
	}

	return result, nil
}

// deployProbe creates the probe pod with a child span.
func (m *Manager) deployProbe(ctx context.Context, ns string, req ProbeRequest) (string, error) {
	ctx, span := probeTracer.Start(ctx, "probe/deploy",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	podName, err := createProbePod(ctx, m.clients, m.cfg, ns, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}

	span.SetAttributes(attribute.String("k8s.pod.name", podName))
	return podName, nil
}

// waitProbe waits for the probe pod to complete with a child span.
func (m *Manager) waitProbe(ctx context.Context, ns, podName string) (*ProbeResult, error) {
	ctx, span := probeTracer.Start(ctx, "probe/wait",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	result, err := waitForPod(ctx, m.clients, ns, podName)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.SetAttributes(
		attribute.Bool("probe.success", result.Success),
		attribute.Int("probe.exit_code", result.ExitCode),
	)
	return result, nil
}

// cleanupProbe deletes the probe pod with a child span.
func (m *Manager) cleanupProbe(ctx context.Context, ns, podName string) {
	// Use a fresh context with timeout so cleanup isn't affected by probe timeout cancellation
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Propagate the parent span context into the cleanup context
	cleanupCtx = trace.ContextWithSpan(cleanupCtx, trace.SpanFromContext(ctx))

	_, span := probeTracer.Start(cleanupCtx, "probe/cleanup",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(attribute.String("k8s.pod.name", podName)),
	)
	defer span.End()

	if err := deleteProbePod(cleanupCtx, m.clients, ns, podName); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		slog.Warn("probe: failed to delete pod", "pod", podName, "namespace", ns, "error", err)
	}
}

// acquireSlot checks and reserves a concurrency slot.
func (m *Manager) acquireSlot() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running >= m.cfg.MaxConcurrentProbes {
		return &types.MCPError{
			Code:    types.ErrCodeProbeLimitReached,
			Message: fmt.Sprintf("concurrent probe limit reached (%d/%d)", m.running, m.cfg.MaxConcurrentProbes),
		}
	}
	m.running++
	slog.Debug("probe: acquired slot", "running", m.running, "max", m.cfg.MaxConcurrentProbes)
	return nil
}

// releaseSlot frees a concurrency slot.
func (m *Manager) releaseSlot() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running > 0 {
		m.running--
	}
	slog.Debug("probe: released slot", "running", m.running)
}

// Stop signals the cleanup goroutine to exit.
func (m *Manager) Stop() {
	m.stopOnce.Do(func() {
		close(m.stopCh)
	})
}
