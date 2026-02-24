package probes

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/isitobservable/k8s-networking-mcp/pkg/config"
	"github.com/isitobservable/k8s-networking-mcp/pkg/k8s"
	"github.com/isitobservable/k8s-networking-mcp/pkg/types"
)

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

	// Create a timeout context for the probe
	probeCtx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	ns := req.Namespace
	if ns == "" {
		ns = m.cfg.ProbeNamespace
	}

	start := time.Now()

	// Create the pod
	podName, err := createProbePod(probeCtx, m.clients, m.cfg, ns, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create probe pod: %w", err)
	}

	// Always clean up the pod
	defer func() {
		deleteCtx, deleteCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer deleteCancel()
		if delErr := deleteProbePod(deleteCtx, m.clients, ns, podName); delErr != nil {
			slog.Warn("probe: failed to delete pod", "pod", podName, "namespace", ns, "error", delErr)
		}
	}()

	// Wait for the pod to complete and collect output
	result, err := waitForPod(probeCtx, m.clients, ns, podName)
	if err != nil {
		if probeCtx.Err() != nil {
			return &ProbeResult{
				Success:  false,
				Error:    "probe timed out",
				Duration: time.Since(start),
			}, &types.MCPError{
				Code:    types.ErrCodeProbeTimeout,
				Message: fmt.Sprintf("probe timed out after %s", req.Timeout),
			}
		}
		return nil, fmt.Errorf("probe execution failed: %w", err)
	}

	result.Duration = time.Since(start)
	return result, nil
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
	m.running--
	slog.Debug("probe: released slot", "running", m.running)
}

// Stop signals the cleanup goroutine to exit.
func (m *Manager) Stop() {
	m.stopOnce.Do(func() {
		close(m.stopCh)
	})
}
