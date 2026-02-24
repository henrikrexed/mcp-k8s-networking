# Architecture

## High-Level Overview

```
AI Agent <--MCP/HTTP--> MCP Server <--K8s API--> Kubernetes Cluster
                           |
                    +------+------+
                    |             |
              Tool Registry   CRD Discovery
                    |             |
              +-----+-----+     Watch
              |     |     |   CRD events
           Core  Gateway  Istio  ...
           Tools  API     Tools
```

## Components

### MCP Server (`pkg/mcp/`)

Implements the Model Context Protocol using the official `modelcontextprotocol/go-sdk`. Provides Streamable HTTP transport at `/mcp`, handles JSON-RPC 2.0 requests, and manages tool registration.

### CRD Discovery (`pkg/discovery/`)

Watch-based discovery of installed networking CRDs. On startup, performs a fast scan via `ServerGroups()`. Then watches `customresourcedefinitions` for real-time detection of CRD installations/removals. Triggers tool registration/deregistration via `onChange` callback.

### Tool Registry (`pkg/tools/`)

Thread-safe registry of diagnostic tools. Each tool implements the `Tool` interface:

```go
type Tool interface {
    Name() string
    Description() string
    InputSchema() map[string]interface{}
    Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error)
}
```

### Probe Manager (`pkg/probes/`)

Manages ephemeral diagnostic pod lifecycle with concurrency limits, TTL-based cleanup, and restricted security contexts.

### Skills Framework (`pkg/skills/`)

Multi-step playbooks that orchestrate multiple tool calls to guide agents through complex networking configurations.

## Data Flow

1. Agent sends `tools/call` request via MCP
2. MCP server dispatches to the tool handler
3. Tool executes K8s API queries via dynamic client
4. Results are formatted as `DiagnosticFinding` structs
5. Compact/detail filtering is applied
6. JSON response is returned to the agent

## Design Decisions

- **Dynamic client over typed clients**: All resource access uses `dynamic.Interface` for flexibility across CRD versions
- **Watch over polling**: CRD discovery uses K8s watch for instant detection
- **Structured findings**: All tools return `DiagnosticFinding` for consistent agent consumption
- **Ephemeral pods**: Active probing uses pods rather than exec to avoid requiring exec permissions
- **slog**: Standard library structured logging with JSON output
