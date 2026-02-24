# Story 12.1: OTel SDK Setup and Trace Provider

Status: ready-for-dev

## Story

As a platform engineer,
I want the MCP server to initialize an OpenTelemetry trace provider,
so that spans are exported to my tracing backend (Jaeger, Tempo, Dynatrace, OTLP collector).

## Acceptance Criteria

1. When `OTEL_EXPORTER_OTLP_ENDPOINT` is set, the server initializes an OTel TracerProvider with OTLP gRPC exporter
2. When `OTEL_EXPORTER_OTLP_ENDPOINT` is NOT set, tracing is disabled (noop tracer) and the server operates normally
3. Spans carry the configured service name from `OTEL_SERVICE_NAME` (default: "mcp-k8s-networking")
4. Spans include resource attributes: `service.name`, `service.version`, `k8s.cluster.name`
5. On graceful shutdown, the TracerProvider flushes pending spans before exiting
6. The global tracer is accessible throughout the codebase via `otel.Tracer("mcp-k8s-networking")`

## Tasks / Subtasks

- [ ] Task 1: Add OTel dependencies (AC: 1)
  - [ ] `go get go.opentelemetry.io/otel`
  - [ ] `go get go.opentelemetry.io/otel/sdk`
  - [ ] `go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc`
  - [ ] `go get go.opentelemetry.io/otel/sdk/resource`
  - [ ] `go get go.opentelemetry.io/otel/semconv/v1.26.0` (or latest)
  - [ ] Run `go mod tidy`
- [ ] Task 2: Create telemetry setup package (AC: 1, 2, 3, 4, 6)
  - [ ] Create `pkg/telemetry/tracer.go` with:
    - `InitTracer(ctx context.Context, cfg *config.Config) (func(context.Context) error, error)` — returns shutdown function
    - Check `OTEL_EXPORTER_OTLP_ENDPOINT` env var
    - If set: create OTLP gRPC exporter, create TracerProvider with resource attributes, set as global provider
    - If not set: return noop shutdown function, log that tracing is disabled
  - [ ] Resource attributes:
    - `semconv.ServiceNameKey` from OTEL_SERVICE_NAME or default "mcp-k8s-networking"
    - `semconv.ServiceVersionKey` from build-time version or "dev"
    - `attribute.String("k8s.cluster.name", cfg.ClusterName)`
  - [ ] Set global tracer provider: `otel.SetTracerProvider(tp)`
  - [ ] Set global text map propagator: `otel.SetTextMapPropagator(propagation.TraceContext{})`
- [ ] Task 3: Wire into main.go (AC: 1, 2, 5)
  - [ ] In `cmd/server/main.go`, call `telemetry.InitTracer(ctx, cfg)` early in startup (after config load)
  - [ ] Store the shutdown function
  - [ ] In graceful shutdown sequence: call tracer shutdown BEFORE exiting (flush pending spans)
  - [ ] Log: "telemetry: tracing enabled, exporting to {endpoint}" or "telemetry: tracing disabled (OTEL_EXPORTER_OTLP_ENDPOINT not set)"
- [ ] Task 4: Verify noop path (AC: 2)
  - [ ] Ensure when OTEL_EXPORTER_OTLP_ENDPOINT is unset, `otel.Tracer()` returns a noop tracer
  - [ ] All subsequent span creation calls (in future stories) become no-ops — zero overhead

## Dev Notes

### Architecture Context

The architecture document specifies "slog + otelslog — OTel-native from day one." This story sets up the OTel trace provider foundation. Subsequent stories (12.2-12.5) will add spans to specific components.

### OTel Environment Variables (standard conventions)

| Variable | Default | Purpose |
|---|---|---|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | (unset = disabled) | OTLP collector endpoint (e.g., `http://otel-collector:4317`) |
| `OTEL_SERVICE_NAME` | mcp-k8s-networking | Service name in traces |
| `OTEL_RESOURCE_ATTRIBUTES` | (optional) | Additional resource attributes |

These are standard OTel env vars — do NOT add them to the Config struct. OTel SDK reads them automatically via `resource.WithFromEnv()`.

### Implementation Pattern

```go
package telemetry

import (
    "context"
    "log/slog"
    "os"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    "go.opentelemetry.io/otel/propagation"
    "go.opentelemetry.io/otel/sdk/resource"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func InitTracer(ctx context.Context, clusterName string) (func(context.Context) error, error) {
    endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
    if endpoint == "" {
        slog.Info("telemetry: tracing disabled (OTEL_EXPORTER_OTLP_ENDPOINT not set)")
        return func(ctx context.Context) error { return nil }, nil
    }

    exporter, err := otlptracegrpc.New(ctx)  // reads OTEL_EXPORTER_OTLP_ENDPOINT automatically
    if err != nil {
        return nil, fmt.Errorf("creating OTLP exporter: %w", err)
    }

    res, err := resource.Merge(
        resource.Default(),
        resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceName("mcp-k8s-networking"),
            attribute.String("k8s.cluster.name", clusterName),
        ),
    )

    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
        sdktrace.WithResource(res),
    )
    otel.SetTracerProvider(tp)
    otel.SetTextMapPropagator(propagation.TraceContext{})

    slog.Info("telemetry: tracing enabled", "endpoint", endpoint)
    return tp.Shutdown, nil
}
```

### Shutdown Order in main.go

```
1. Stop accepting new connections
2. Drain active sessions
3. Cancel in-flight tool executions
4. Flush OTel spans (tracerShutdown(ctx))  <-- NEW
5. Exit
```

### Files to Create

| File | Purpose |
|---|---|
| `pkg/telemetry/tracer.go` | OTel TracerProvider initialization + shutdown |

### Files to Modify

| File | Action |
|---|---|
| `go.mod` | Add OTel dependencies |
| `cmd/server/main.go` | Wire InitTracer, add shutdown call |

### Files NOT to Modify

No tool files, no MCP server, no discovery — this story only sets up the global tracer. Span creation is added in Stories 12.2-12.5.

### Testing

- Build must pass: `make build`
- `go vet ./...` must pass
- Manual test WITHOUT OTEL_EXPORTER_OTLP_ENDPOINT: server starts normally, logs "tracing disabled"
- Manual test WITH OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317: server starts, logs "tracing enabled"
- Verify shutdown flushes spans (check OTel collector receives them)

### Project Structure Notes

- New package `pkg/telemetry/` — follows architecture's package organization
- Uses standard OTel env var conventions — no custom config needed
- The tracer is global via `otel.SetTracerProvider` — any package can get a tracer with `otel.Tracer("component-name")`

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#Infrastructure & Deployment - Logging: slog + otelslog]
- [Source: _bmad-output/planning-artifacts/architecture.md#Communication Patterns - Context Propagation]
- [Source: _bmad-output/planning-artifacts/epics.md#Epic 12 - Story 12.1]

## Dev Agent Record

### Agent Model Used

### Debug Log References

### Completion Notes List

### File List
