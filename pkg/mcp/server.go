package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/isitobservable/k8s-networking-mcp/pkg/telemetry"
	"github.com/isitobservable/k8s-networking-mcp/pkg/tools"
	"github.com/isitobservable/k8s-networking-mcp/pkg/types"
)

const (
	mcpProtocolVersion = "2025-03-26"
	maxResultAttrLen   = 1024
)

// sensitiveKeys are argument key substrings that should be redacted from span attributes.
var sensitiveKeys = []string{"secret", "token", "key", "password", "credential"}

type Server struct {
	mcpServer  *mcp.Server
	httpServer *http.Server
	registry   *tools.Registry
	meters     *telemetry.Meters

	mu              sync.Mutex
	registeredTools map[string]struct{} // tracks tools currently registered in mcpServer
}

func NewServer(registry *tools.Registry) *Server {
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "mcp-k8s-networking",
		Version: "1.0.0",
	}, nil)

	meters, err := telemetry.NewMeters()
	if err != nil {
		slog.Warn("mcp: failed to create OTel meters, metrics will be unavailable", "error", err)
	}

	return &Server{
		mcpServer:       mcpServer,
		registry:        registry,
		meters:          meters,
		registeredTools: make(map[string]struct{}),
	}
}

// SyncTools diffs the registry against what is currently registered in the MCP server,
// adding new tools and removing stale ones.
func (s *Server) SyncTools() {
	s.mu.Lock()
	defer s.mu.Unlock()

	registryTools := s.registry.List()

	// Build a set of tool names currently in the registry
	wanted := make(map[string]struct{}, len(registryTools))
	for _, t := range registryTools {
		wanted[t.Name()] = struct{}{}
	}

	// Remove tools that are registered but no longer in the registry
	var toRemove []string
	for name := range s.registeredTools {
		if _, ok := wanted[name]; !ok {
			toRemove = append(toRemove, name)
		}
	}
	if len(toRemove) > 0 {
		s.mcpServer.RemoveTools(toRemove...)
		for _, name := range toRemove {
			delete(s.registeredTools, name)
		}
		slog.Info("mcp: removed tools", "tools", toRemove)
	}

	// Add tools that are in the registry but not yet registered
	added := 0
	for _, t := range registryTools {
		if _, ok := s.registeredTools[t.Name()]; ok {
			continue
		}
		mcpTool := buildMCPTool(t)
		handler := s.buildInstrumentedHandler(t)
		s.mcpServer.AddTool(mcpTool, handler)
		s.registeredTools[t.Name()] = struct{}{}
		added++
	}

	slog.Info("mcp: synced tools", "total", len(s.registeredTools), "added", added, "removed", len(toRemove))
}

func (s *Server) Start(addr string) error {
	s.SyncTools()

	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return s.mcpServer
	}, nil)

	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	slog.Info("mcp: starting Streamable HTTP server", "addr", addr)
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

func buildMCPTool(t tools.Tool) *mcp.Tool {
	schema := t.InputSchema()
	schemaJSON, _ := json.Marshal(schema)

	tool := &mcp.Tool{
		Name:        t.Name(),
		Description: t.Description(),
	}

	// Parse the JSON schema into the go-sdk's jsonschema.Schema type
	if err := json.Unmarshal(schemaJSON, &tool.InputSchema); err != nil {
		slog.Warn("mcp: failed to parse input schema", "tool", t.Name(), "error", err)
	}

	return tool
}

// buildInstrumentedHandler creates a ToolHandler that wraps tool execution
// with OTel spans, metrics, and context propagation per GenAI + MCP semantic conventions.
func (s *Server) buildInstrumentedHandler(t tools.Tool) mcp.ToolHandler {
	tracer := otel.Tracer("mcp-k8s-networking")

	return func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// --- Context Propagation: extract traceparent/tracestate from params._meta ---
		meta := request.Params.GetMeta()
		if meta != nil {
			carrier := propagation.MapCarrier{}
			for k, v := range meta {
				if str, ok := v.(string); ok {
					carrier.Set(k, str)
				}
			}
			ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)
		}

		// --- Extract session ID ---
		sessionID := ""
		if request.Session != nil {
			sessionID = request.Session.ID()
		}

		// --- Start span following GenAI + MCP semantic conventions ---
		spanName := fmt.Sprintf("execute_tool %s", t.Name())
		ctx, span := tracer.Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindServer),
		)
		defer span.End()

		// Set GenAI + MCP span attributes
		span.SetAttributes(
			attribute.String("gen_ai.operation.name", "execute_tool"),
			attribute.String("gen_ai.tool.name", t.Name()),
			attribute.String("mcp.method.name", "tools/call"),
			attribute.String("mcp.protocol.version", mcpProtocolVersion),
			attribute.String("mcp.session.id", sessionID),
		)

		// --- Unmarshal arguments ---
		var args map[string]interface{}
		if request.Params.Arguments != nil {
			if err := json.Unmarshal(request.Params.Arguments, &args); err != nil {
				errType := "INVALID_INPUT"
				s.recordError(ctx, span, t.Name(), errType, err)
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to parse arguments: %v", err)}},
					IsError: true,
				}, nil
			}
		}
		if args == nil {
			args = make(map[string]interface{})
		}

		// Set sanitized arguments as span attribute
		span.SetAttributes(attribute.String("gen_ai.tool.call.arguments", sanitizeArgs(args)))

		// --- Execute tool with timing ---
		start := time.Now()
		result, err := t.Run(ctx, args)
		duration := time.Since(start).Seconds()

		// --- Record metrics ---
		if err != nil {
			errType := "tool_error"
			if mcpErr, ok := err.(*types.MCPError); ok {
				errType = mcpErr.Code
			}
			s.recordMetrics(ctx, t.Name(), errType, duration)
			s.recordError(ctx, span, t.Name(), errType, err)

			// Format MCPError consistently if available
			if mcpErr, ok := err.(*types.MCPError); ok {
				errJSON, _ := json.MarshalIndent(mcpErr, "", "  ")
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: string(errJSON)}},
					IsError: true,
				}, nil
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
				IsError: true,
			}, nil
		}

		// Success metrics
		s.recordMetrics(ctx, t.Name(), "", duration)
		span.SetStatus(codes.Ok, "")

		// Apply compact/detail filtering if the response contains a ToolResult
		if result != nil {
			if tr, ok := result.Data.(*types.ToolResult); ok {
				detail := false
				if d, ok := args["detail"]; ok {
					if b, ok := d.(bool); ok {
						detail = b
					}
				}
				tr.Findings = types.FilterFindings(tr.Findings, detail)

				// Record findings metrics
				s.recordFindings(ctx, t.Name(), tr.Findings)
			}
		}

		jsonBytes, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			s.recordError(ctx, span, t.Name(), "INTERNAL_ERROR", err)
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to marshal result: %v", err)}},
				IsError: true,
			}, nil
		}

		// Set truncated result as span attribute
		resultStr := string(jsonBytes)
		if len(resultStr) > maxResultAttrLen {
			resultStr = resultStr[:maxResultAttrLen]
		}
		span.SetAttributes(attribute.String("gen_ai.tool.call.result", resultStr))

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(jsonBytes)}},
		}, nil
	}
}

// recordMetrics records GenAI request duration and count metrics.
func (s *Server) recordMetrics(ctx context.Context, toolName, errType string, duration float64) {
	if s.meters == nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("gen_ai.tool.name", toolName),
	}
	if errType != "" {
		attrs = append(attrs, attribute.String("error.type", errType))
	}
	s.meters.RequestDuration.Record(ctx, duration, telemetry.WithAttrs(attrs...))
	s.meters.RequestCount.Add(ctx, 1, telemetry.WithAttrs(attrs...))
}

// recordError records error metrics and sets span error status.
func (s *Server) recordError(ctx context.Context, span trace.Span, toolName, errType string, err error) {
	span.SetStatus(codes.Error, err.Error())
	span.SetAttributes(attribute.String("error.type", errType))
	span.RecordError(err)

	if s.meters == nil {
		return
	}
	s.meters.ErrorsTotal.Add(ctx, 1, telemetry.WithAttrs(
		attribute.String("error.code", errType),
		attribute.String("gen_ai.tool.name", toolName),
	))
}

// recordFindings records custom domain metrics for diagnostic findings.
func (s *Server) recordFindings(ctx context.Context, toolName string, findings []types.DiagnosticFinding) {
	if s.meters == nil || len(findings) == 0 {
		return
	}
	for _, f := range findings {
		s.meters.FindingsTotal.Add(ctx, 1, telemetry.WithAttrs(
			attribute.String("severity", f.Severity),
			attribute.String("analyzer", toolName),
		))
	}
}

// sanitizeArgs returns a JSON string of the arguments with sensitive values redacted.
func sanitizeArgs(args map[string]interface{}) string {
	sanitized := make(map[string]interface{}, len(args))
	for k, v := range args {
		if isSensitiveKey(k) {
			sanitized[k] = "[REDACTED]"
		} else {
			sanitized[k] = v
		}
	}
	b, err := json.Marshal(sanitized)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// isSensitiveKey checks if a key name suggests it contains sensitive data.
func isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, s := range sensitiveKeys {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}
