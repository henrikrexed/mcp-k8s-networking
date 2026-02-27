# Story 1.1: Migrate MCP Server to Official go-sdk with Streamable HTTP

Status: ready-for-dev

## Story

As an AI agent,
I want to connect to the MCP server via the official MCP protocol implementation,
so that I get reliable, spec-compliant communication with proper session management.

## Acceptance Criteria

1. The MCP server accepts agent connections via Streamable HTTP transport and completes the MCP initialize/initialized handshake
2. The server responds to `tools/list` with all registered tools from the registry
3. The server responds to `tools/call` with structured JSON results
4. When tools are added or removed from the registry, the MCP server reflects the updated tool list without restart
5. The server sends heartbeats to maintain active agent sessions (FR44)
6. The existing 18 tools continue to function correctly after migration
7. The health endpoint `/healthz` returns HTTP 200

## Tasks / Subtasks

- [ ] Task 1: Replace mcp-go dependency with official go-sdk (AC: 1)
  - [ ] Remove `github.com/mark3labs/mcp-go` from go.mod
  - [ ] Add `github.com/modelcontextprotocol/go-sdk` (latest stable)
  - [ ] Run `go mod tidy` to clean up dependencies
- [ ] Task 2: Rewrite pkg/mcp/server.go using go-sdk API (AC: 1, 2, 3, 5)
  - [ ] Replace `server.MCPServer` + `server.SSEServer` with go-sdk `mcp.Server` + Streamable HTTP transport
  - [ ] Implement tool registration bridge: Tool interface -> go-sdk tool registration
  - [ ] Fix SyncTools bug: current code adds tools but never removes deregistered ones. New implementation must diff registry state and remove tools no longer present
  - [ ] Remove hardcoded `http://localhost` base URL (current bug)
  - [ ] Implement heartbeat/keep-alive via go-sdk transport configuration
- [ ] Task 3: Update buildMCPTool and buildHandler functions (AC: 2, 3)
  - [ ] Adapt tool schema conversion from `map[string]interface{}` to go-sdk expected format
  - [ ] Adapt handler function signature to go-sdk tool call handler interface
  - [ ] Preserve JSON result marshaling behavior
- [ ] Task 4: Update cmd/server/main.go (AC: 4, 7)
  - [ ] Update MCP server initialization to use new go-sdk API
  - [ ] Wire SyncTools to CRD discovery onChange callback (existing pattern)
  - [ ] Ensure health endpoint continues to work on separate port
- [ ] Task 5: Verify all 18 existing tools work (AC: 6)
  - [ ] Build and run locally: `make build && CLUSTER_NAME=test go run ./cmd/server/`
  - [ ] Verify `/healthz` returns 200
  - [ ] Verify SSE/Streamable HTTP endpoint accepts connections

## Dev Notes

### Current Implementation (to be replaced)

The current MCP server at `pkg/mcp/server.go` uses:
- `github.com/mark3labs/mcp-go/mcp` for tool types
- `github.com/mark3labs/mcp-go/server` for MCPServer + SSEServer
- Key types: `mcp.Tool`, `mcp.CallToolRequest`, `mcp.CallToolResult`, `mcp.NewToolResultText`, `mcp.NewToolResultError`
- `server.NewMCPServer()` with `server.WithRecovery()`
- `server.NewSSEServer()` with `server.WithBaseURL()`
- `server.ToolHandlerFunc` for tool handlers

### Architecture Constraints

- **Transport**: Architecture specifies Streamable HTTP (SSE + POST hybrid) — NOT pure SSE. go-sdk provides this natively.
- **Tool interface stays the same**: The `tools.Tool` interface in `pkg/tools/types.go` does NOT change. Only the MCP bridge layer changes.
- **No changes to tool implementations**: All 18 tool files in `pkg/tools/*.go` remain untouched. Only `pkg/mcp/server.go` is rewritten.
- **Registry integration**: `pkg/tools/registry.go` stays the same. The MCP server reads from it via `registry.List()`.

### Critical Bugs to Fix

1. **SyncTools never removes tools**: Current `SyncTools()` calls `mcpServer.AddTool()` for each tool but never removes tools that were deregistered. When CRDs are removed and tools deregistered, old tools remain in the MCP tool list. The new implementation must track registered tools and remove any that are no longer in the registry.
2. **Hardcoded base URL**: `server.WithBaseURL(fmt.Sprintf("http://localhost:%d", port))` should not be hardcoded. go-sdk Streamable HTTP transport handles this differently.

### go-sdk API Reference

Research the official go-sdk API at `github.com/modelcontextprotocol/go-sdk`. Key patterns:
- `mcp.NewServer(name, version)` or similar server constructor
- Transport setup for Streamable HTTP
- Tool registration with schema + handler
- Tool deregistration capability (critical for dynamic tool registry)

### Files to Modify

| File | Action |
|---|---|
| `go.mod` | Replace mcp-go with go-sdk |
| `pkg/mcp/server.go` | Full rewrite |
| `cmd/server/main.go` | Update MCP server initialization |

### Files NOT to Modify

All tool files (`pkg/tools/*.go`), registry (`pkg/tools/registry.go`), types (`pkg/tools/types.go`), config, k8s client, discovery — all remain unchanged.

### Testing

- Build must pass: `make build`
- `go vet ./...` must pass
- Manual test: start server, verify `/healthz` returns 200, verify Streamable HTTP endpoint responds

### Project Structure Notes

- Module: `github.com/henrikrexed/k8s-networking-mcp`
- Go version: 1.25.0
- MCP server: `pkg/mcp/server.go` (single file)
- Entry point: `cmd/server/main.go`

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#Starter Template Evaluation]
- [Source: _bmad-output/planning-artifacts/architecture.md#MCP Protocol Implementation Specification]
- [Source: _bmad-output/planning-artifacts/architecture.md#Core Architectural Decisions]
- [Source: pkg/mcp/server.go - current implementation]
- [Source: pkg/tools/types.go - Tool interface definition]
- [Source: pkg/tools/registry.go - Registry implementation]

## Dev Agent Record

### Agent Model Used

### Debug Log References

### Completion Notes List

### File List
