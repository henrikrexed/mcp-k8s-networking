# Story 2.1: Proxy Sidecar Log Collection

Status: ready-for-dev

## Story

As an AI agent,
I want to retrieve logs from Envoy/proxy sidecar containers,
so that I can diagnose proxy-level networking issues like connection failures and TLS errors.

## Acceptance Criteria

1. Given a pod with a proxy sidecar container (istio-proxy, envoy, or linkerd-proxy), when the agent calls `get_proxy_logs` with pod and namespace, it auto-detects the proxy container and returns its logs
2. Given the agent specifies `tail` and `since` parameters, the tool returns only the specified number of recent lines within the time window
3. Given the log output exceeds 100KB, the output is truncated with a message indicating truncation and total available lines
4. Given a pod has no proxy container, the tool returns an MCPError with code INVALID_INPUT
5. The response uses the DiagnosticFinding/ToolResult format with ClusterMetadata

## Tasks / Subtasks

- [ ] Task 1: Add output size limiting to `getPodLogs` helper (AC: 3)
  - [ ] Add a `maxBytes` parameter to `getPodLogs` (default 100KB = 102400 bytes)
  - [ ] Use `io.LimitReader` to cap the stream read at `maxBytes`
  - [ ] When truncated, return a truncation flag and count total available lines vs returned lines
  - [ ] Return a struct instead of raw string: `type LogResult struct { Logs string; Truncated bool; TotalLines int; ReturnedLines int }`
- [ ] Task 2: Refactor `GetProxyLogsTool.Run` to use DiagnosticFinding response (AC: 1, 2, 5)
  - [ ] Replace `NewResponse` with `NewToolResultResponse`
  - [ ] Create an `info`-level DiagnosticFinding with category `"logs"` containing the log data
  - [ ] Include pod name, namespace, container, and line count in the finding summary
  - [ ] Include `detail` field with the actual logs (only when `detail` param is true or by default for log tools)
  - [ ] Include ClusterMetadata with namespace
- [ ] Task 3: Return MCPError for missing proxy container (AC: 4)
  - [ ] Replace `fmt.Errorf("no proxy container found...")` with `&types.MCPError{Code: types.ErrCodeInvalidInput, Tool: "get_proxy_logs", Message: "no proxy container found", Detail: "..."}`
  - [ ] Ensure the MCPError is returned as a Go error (it implements the `error` interface)
- [ ] Task 4: Handle truncation in the response (AC: 3)
  - [ ] When `LogResult.Truncated` is true, add a `warning`-level DiagnosticFinding indicating truncation
  - [ ] Include total lines available vs lines returned in the warning summary
- [ ] Task 5: Verify and test (AC: 1-5)
  - [ ] Build: `go build ./...`
  - [ ] Vet: `go vet ./...`
  - [ ] Verify the tool still registers correctly in the registry

## Dev Notes

### Current Implementation

The existing `GetProxyLogsTool` in `pkg/tools/logs.go:22-72` already handles:
- Proxy container auto-detection via `findProxyContainer()` (line 341-350) checking for `istio-proxy`, `envoy`, `linkerd-proxy`
- `tail` and `since` parameters
- Pod log retrieval via `getPodLogs()` helper (line 352-380)

**What's missing:**
1. **No output size limit** — `getPodLogs` uses `io.Copy` with no size cap, so a verbose sidecar can return megabytes of logs
2. **Raw map response** — uses `NewResponse` with `map[string]interface{}` instead of `DiagnosticFinding`/`ToolResult`
3. **Generic error** — returns `fmt.Errorf` instead of `MCPError` when no proxy container is found

### Key Code Patterns

**DiagnosticFinding format** (used by refactored tools in Epic 1):
```go
findings := []types.DiagnosticFinding{
    {
        Severity: types.SeverityInfo,
        Category: "logs",
        Resource: &types.ResourceRef{Kind: "Pod", Namespace: ns, Name: podName},
        Summary:  fmt.Sprintf("Retrieved %d log lines from %s/%s container %s", lineCount, ns, podName, container),
        Detail:   logs,
    },
}
return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, ""), nil
```

**MCPError pattern:**
```go
return nil, &types.MCPError{
    Code:    types.ErrCodeInvalidInput,
    Tool:    "get_proxy_logs",
    Message: fmt.Sprintf("no proxy container found in pod %s/%s", ns, podName),
    Detail:  "Expected one of: istio-proxy, envoy, linkerd-proxy",
}
```

**LogResult struct** (new, add to logs.go or types):
```go
type LogResult struct {
    Logs          string
    Truncated     bool
    TotalLines    int
    ReturnedLines int
}
```

### Output Size Limiting Strategy

The 100KB limit should be applied in `getPodLogs` using `io.LimitReader`:
```go
limitedReader := io.LimitReader(stream, maxBytes+1) // +1 to detect truncation
buf := new(strings.Builder)
n, _ := io.Copy(buf, limitedReader)
truncated := n > maxBytes
```

When truncated, count newlines in the returned buffer as `ReturnedLines`. Note: `TotalLines` is approximate since we can't read the full stream efficiently — report "at least N lines (output truncated at 100KB)".

### Files to Modify

| File | Action |
|---|---|
| `pkg/tools/logs.go` | Modify `getPodLogs` to add size limit, refactor `GetProxyLogsTool.Run` to use DiagnosticFinding |

### Files NOT to Modify

- `pkg/types/findings.go` — DiagnosticFinding types already exist
- `pkg/types/errors.go` — MCPError with INVALID_INPUT code already exists
- `pkg/tools/types.go` — `NewToolResultResponse` helper already exists
- Other tools in `pkg/tools/` — unchanged

### Category Constant

A `"logs"` category constant does not yet exist in `pkg/types/findings.go`. Either:
- Add `CategoryLogs = "logs"` to the constants in findings.go, OR
- Use a string literal `"logs"` directly

Prefer adding the constant for consistency with existing categories.

### Testing

- Build: `go build ./...`
- Vet: `go vet ./...`
- Manual: start server, call `get_proxy_logs` with a pod that has a sidecar
- Verify truncation: call with a very chatty proxy to trigger the 100KB limit

### References

- [Source: _bmad-output/planning-artifacts/epics.md - Story 2.1]
- [Source: pkg/tools/logs.go - current implementation]
- [Source: pkg/types/findings.go - DiagnosticFinding struct]
- [Source: pkg/types/errors.go - MCPError struct]
- [Source: pkg/tools/types.go - NewToolResultResponse helper]

## Dev Agent Record

### Agent Model Used

### Debug Log References

### Completion Notes List

### File List
