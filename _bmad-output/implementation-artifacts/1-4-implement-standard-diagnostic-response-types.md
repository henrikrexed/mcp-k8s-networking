# Story 1.4: Implement Standard Diagnostic Response Types

Status: ready-for-dev

## Story

As an AI agent,
I want diagnostic results returned in a consistent structured format with compact/detail modes,
so that I can efficiently parse findings and minimize token consumption.

## Acceptance Criteria

1. All diagnostic tools return results using the `DiagnosticFinding` struct with fields: severity, category, resource, summary, detail, suggestion
2. Without `"detail": true` parameter, only `summary` is populated (compact mode); `detail` and `suggestion` are omitted (FR41, NFR4)
3. With `"detail": true` parameter, all fields including `detail` and `suggestion` are populated (FR42)
4. Expected errors (provider not installed, CRD unavailable) use `MCPError` struct with code, message, tool, detail fields
5. Every tool response includes `ClusterMetadata` with clusterName, timestamp, namespace, provider
6. Severity values are strictly: critical, warning, info, ok — no other values allowed
7. Error codes defined: PROVIDER_NOT_FOUND, CRD_NOT_AVAILABLE, INVALID_INPUT, INTERNAL_ERROR, PROBE_TIMEOUT, PROBE_LIMIT_REACHED, AUTH_FAILED

## Tasks / Subtasks

- [ ] Task 1: Create shared types package (AC: 1, 5, 6, 7)
  - [ ] Create `pkg/types/findings.go` with:
    - `DiagnosticFinding` struct: Severity, Category, Resource (*ResourceRef), Summary, Detail (omitempty), Suggestion (omitempty)
    - `ResourceRef` struct: Kind, Namespace, Name, APIVersion
    - Severity constants: SeverityCritical, SeverityWarning, SeverityInfo, SeverityOK
    - Category constants: CategoryRouting, CategoryDNS, CategoryTLS, CategoryPolicy, CategoryMesh, CategoryConnectivity
  - [ ] Create `pkg/types/errors.go` with:
    - `MCPError` struct: Code, Message, Tool, Detail (omitempty)
    - Error code constants: ErrCodeProviderNotFound, ErrCodeCRDNotAvailable, ErrCodeInvalidInput, ErrCodeInternalError, ErrCodeProbeTimeout, ErrCodeProbeLimitReached, ErrCodeAuthFailed
    - `MCPError` implements `error` interface
  - [ ] Create `pkg/types/metadata.go` with:
    - `ClusterMetadata` struct: ClusterName, Timestamp (time.Time), Namespace, Provider
    - `ToolResult` struct: Findings []DiagnosticFinding, Metadata ClusterMetadata, IsError bool
- [ ] Task 2: Add compact/detail mode support (AC: 2, 3)
  - [ ] Add `"detail"` boolean parameter to all tool InputSchema definitions
  - [ ] Create helper function: `FilterFindings(findings []DiagnosticFinding, detail bool) []DiagnosticFinding` that strips Detail and Suggestion fields when detail=false
  - [ ] Add `getBoolArg(args, key, default)` helper to `pkg/tools/types.go`
- [ ] Task 3: Refactor existing tools to use new types (AC: 1, 5)
  - [ ] Update `pkg/tools/types.go`: keep Tool interface but change `Run` return type from `*StandardResponse` to `*types.ToolResult`
  - [ ] OR: keep `*StandardResponse` as a bridge but have it wrap `ToolResult` — choose whichever is less disruptive
  - [ ] Update `BaseTool` to include a helper: `NewToolResult(findings, namespace, provider)` that auto-populates ClusterMetadata
  - [ ] Refactor at minimum 2 tools as proof of concept: `list_services` and `check_dns_resolution`
  - [ ] Remaining tools can be refactored incrementally in subsequent stories
- [ ] Task 4: Update MCP handler to format responses (AC: 2, 3)
  - [ ] In `pkg/mcp/server.go` handler, check for `detail` parameter
  - [ ] Apply `FilterFindings` before marshaling response to JSON
  - [ ] Ensure MCPError responses are formatted consistently

## Dev Notes

### Current Implementation (to be refactored)

`pkg/tools/types.go` currently defines:
```go
type StandardResponse struct {
    Cluster   string      `json:"cluster"`
    Timestamp string      `json:"timestamp"`
    Tool      string      `json:"tool"`
    Data      interface{} `json:"data"`
}
```
Tools return `map[string]interface{}` in the Data field — completely unstructured. This must be replaced with typed `DiagnosticFinding` structs.

### Architecture Constraints

- **DiagnosticFinding is mandatory**: Architecture says "All AI Agents MUST use DiagnosticFinding struct for all diagnostic output — no custom response shapes"
- **MCPError for agent-facing errors**: "Return MCPError (with tool context) for all agent-facing errors — no raw error strings"
- **Severity levels are strict**: Only critical, warning, info, ok — no custom severity strings
- **JSON camelCase**: All JSON output uses camelCase field names per architecture convention

### Target Type Definitions

```go
type DiagnosticFinding struct {
    Severity   string       `json:"severity"`
    Category   string       `json:"category"`
    Resource   *ResourceRef `json:"resource,omitempty"`
    Summary    string       `json:"summary"`
    Detail     string       `json:"detail,omitempty"`
    Suggestion string       `json:"suggestion,omitempty"`
}

type MCPError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    Tool    string `json:"tool"`
    Detail  string `json:"detail,omitempty"`
}

type ClusterMetadata struct {
    ClusterName string    `json:"clusterName"`
    Timestamp   time.Time `json:"timestamp"`
    Namespace   string    `json:"namespace,omitempty"`
    Provider    string    `json:"provider,omitempty"`
}
```

### Migration Strategy

Do NOT refactor all 18 tools at once. This story:
1. Creates the type definitions
2. Adds compact/detail infrastructure
3. Refactors 2 tools as proof of concept
4. Remaining tools get refactored in their respective epic stories (Stories 1.5-1.8 for core K8s, Story 3.1 for Gateway API, Story 4.1 for Istio, etc.)

### Files to Create

| File | Purpose |
|---|---|
| `pkg/types/findings.go` | DiagnosticFinding, ResourceRef, severity/category constants |
| `pkg/types/errors.go` | MCPError, error code constants |
| `pkg/types/metadata.go` | ClusterMetadata, ToolResult |

### Files to Modify

| File | Action |
|---|---|
| `pkg/tools/types.go` | Add getBoolArg, update Tool interface or add bridge |
| `pkg/mcp/server.go` | Add compact/detail filtering in handler |
| `pkg/tools/k8s_services.go` | Refactor list_services as proof of concept |
| `pkg/tools/k8s_dns.go` | Refactor check_dns_resolution as proof of concept |

### Testing

- Build must pass: `make build`
- `go vet ./...` must pass
- Verify refactored tools return DiagnosticFinding format
- Verify compact mode omits detail/suggestion fields in JSON output

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#Format Patterns - Diagnostic Finding Structure]
- [Source: _bmad-output/planning-artifacts/architecture.md#Format Patterns - Agent-Facing Error Structure]
- [Source: _bmad-output/planning-artifacts/architecture.md#Format Patterns - Compact vs Detail Mode]
- [Source: _bmad-output/planning-artifacts/architecture.md#Enforcement Guidelines]
- [Source: pkg/tools/types.go - current StandardResponse]

## Dev Agent Record

### Agent Model Used

### Debug Log References

### Completion Notes List

### File List
