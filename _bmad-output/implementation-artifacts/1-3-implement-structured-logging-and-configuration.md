# Story 1.3: Implement Structured Logging and Configuration

Status: ready-for-dev

## Story

As a platform engineer,
I want the MCP server to produce structured JSON logs and be configurable via environment variables,
so that I can integrate it with my cluster's log aggregation pipeline and tune its behavior.

## Acceptance Criteria

1. All log output uses Go's `log/slog` package with JSON handler (FR48)
2. Every log line during tool execution includes `tool_name` field when applicable
3. No log line contains secrets, tokens, or certificate private keys (NFR9)
4. Configuration loads from env vars: CLUSTER_NAME, PORT, LOG_LEVEL, NAMESPACE, CACHE_TTL, TOOL_TIMEOUT, PROBE_NAMESPACE, PROBE_IMAGE, MAX_CONCURRENT_PROBES
5. LOG_LEVEL controls slog output level (debug, info, warn, error)
6. Missing required CLUSTER_NAME env var causes server to exit with a clear error message
7. All config values have documented defaults matching the architecture spec

## Tasks / Subtasks

- [ ] Task 1: Expand Config struct and loading (AC: 4, 6, 7)
  - [ ] Add fields to `pkg/config/config.go`:
    - `CacheTTL time.Duration` (default: 30s, env: CACHE_TTL)
    - `ToolTimeout time.Duration` (default: 10s, env: TOOL_TIMEOUT)
    - `ProbeNamespace string` (default: "mcp-diagnostics", env: PROBE_NAMESPACE)
    - `ProbeImage string` (default: "ghcr.io/mcp-k8s-networking/probe:latest", env: PROBE_IMAGE)
    - `MaxConcurrentProbes int` (default: 5, env: MAX_CONCURRENT_PROBES)
  - [ ] Add validation: CLUSTER_NAME must be non-empty, exit with `os.Exit(1)` and clear error if missing
  - [ ] Parse duration strings for CacheTTL and ToolTimeout using `time.ParseDuration`
  - [ ] Parse integer for MaxConcurrentProbes with bounds check (1-20)
- [ ] Task 2: Replace all log usage with slog (AC: 1, 2, 5)
  - [ ] Create slog setup function in `pkg/config/config.go` or new `pkg/logging/logging.go`
  - [ ] Initialize `slog.SetDefault()` with `slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parsedLevel})`
  - [ ] Parse LOG_LEVEL string to `slog.Level` (debug=-4, info=0, warn=4, error=8)
  - [ ] Replace ALL `log.Printf` calls across the codebase with `slog.Info`, `slog.Error`, `slog.Debug`, `slog.Warn`
  - [ ] Add `tool_name` attribute to log calls within tool execution paths
- [ ] Task 3: Remove `log` package imports (AC: 1)
  - [ ] Search all .go files for `"log"` import and replace with `"log/slog"` usage
  - [ ] Files to update: `cmd/server/main.go`, `pkg/mcp/server.go`, `pkg/discovery/discovery.go`, `pkg/tools/logs.go`
- [ ] Task 4: Ensure no sensitive data in logs (AC: 3)
  - [ ] Review all slog calls — do NOT log: ServiceAccount tokens, TLS private keys, full resource YAML bodies
  - [ ] Log resource references only: kind/namespace/name

## Dev Notes

### Current Implementation

`pkg/config/config.go` currently has only 4 fields: ClusterName, Port, LogLevel, Namespace. Missing 5 fields specified by architecture.

Current logging uses `log.Printf` in:
- `cmd/server/main.go` — startup messages
- `pkg/mcp/server.go` — tool sync, server start
- `pkg/discovery/discovery.go` — discovery errors
- `pkg/tools/logs.go` — (no direct logging, but uses fmt)

### Architecture Constraints

- **slog + JSON handler**: Architecture specifies `log/slog` with JSON output. NOT third-party loggers.
- **otelslog bridge**: Architecture mentions otelslog for trace context injection — this will be added in Story 12.5, NOT in this story. This story sets up plain slog.
- **Mandatory log fields**: `tool_name`, `session_id` (session_id will be added when sessions are implemented, not this story)
- **Forbidden in logs**: secrets, tokens, certificate private keys, full YAML bodies

### Config Defaults (from Architecture)

| Variable | Default | Type |
|---|---|---|
| CLUSTER_NAME | (required) | string |
| PORT | 8080 | int |
| LOG_LEVEL | info | string |
| NAMESPACE | "" (all) | string |
| CACHE_TTL | 30s | duration |
| TOOL_TIMEOUT | 10s | duration |
| PROBE_NAMESPACE | mcp-diagnostics | string |
| PROBE_IMAGE | ghcr.io/mcp-k8s-networking/probe:latest | string |
| MAX_CONCURRENT_PROBES | 5 | int (1-20) |

### slog Setup Pattern

```go
func SetupLogging(level string) {
    var slogLevel slog.Level
    switch strings.ToLower(level) {
    case "debug":
        slogLevel = slog.LevelDebug
    case "warn", "warning":
        slogLevel = slog.LevelWarn
    case "error":
        slogLevel = slog.LevelError
    default:
        slogLevel = slog.LevelInfo
    }
    handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slogLevel})
    slog.SetDefault(slog.New(handler))
}
```

### Files to Modify

| File | Action |
|---|---|
| `pkg/config/config.go` | Expand Config struct, add validation, add duration parsing |
| `cmd/server/main.go` | Setup slog, replace log.Printf, validate config on startup |
| `pkg/mcp/server.go` | Replace log.Printf with slog |
| `pkg/discovery/discovery.go` | Replace log.Printf with slog |

### Testing

- Build must pass: `make build`
- `go vet ./...` must pass
- Manual test: set `LOG_LEVEL=debug` and verify JSON structured output
- Manual test: unset `CLUSTER_NAME` and verify server exits with clear error

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#Infrastructure & Deployment - Logging]
- [Source: _bmad-output/planning-artifacts/architecture.md#Configuration Specification]
- [Source: _bmad-output/planning-artifacts/architecture.md#Communication Patterns - Structured Logging]
- [Source: pkg/config/config.go - current 4-field config]

## Dev Agent Record

### Agent Model Used

### Debug Log References

### Completion Notes List

### File List
