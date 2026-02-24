# Contributing

## Development Setup

```bash
# Clone
git clone https://github.com/isitobservable/k8s-networking-mcp.git
cd k8s-networking-mcp

# Build
go build ./...

# Run locally (requires KUBECONFIG)
CLUSTER_NAME=dev go run ./cmd/server/
```

## Adding a New Provider

1. Create `pkg/tools/provider_<name>.go` with tool structs implementing the `Tool` interface
2. Add GVR definitions for the provider's CRDs
3. Add `Has<Provider>` field to `discovery.Features` in `pkg/discovery/discovery.go`
4. Add detection logic in `detectGroup()`
5. Register/unregister tools in the `onChange` callback in `cmd/server/main.go`
6. Add RBAC rules in the Helm chart's ClusterRole template

## Adding a New Tool

1. Define the tool struct embedding `BaseTool`
2. Implement `Name()`, `Description()`, `InputSchema()`, and `Run()`
3. Use `DiagnosticFinding` for results and `MCPError` for errors
4. Register in `cmd/server/main.go`

## Code Style

- Use `slog` for structured logging
- Return `DiagnosticFinding` with appropriate severity levels
- Use `getStringArg`, `getIntArg`, `getBoolArg` helpers for input parsing
- Use `NewToolResultResponse` for response creation
- Handle both v1 and v1beta1 API versions where applicable

## Project Structure

```
cmd/server/         Entry point
pkg/
  config/           Configuration loading
  discovery/        CRD watch-based discovery
  k8s/              Kubernetes client setup
  mcp/              MCP server implementation
  probes/           Ephemeral pod management
  skills/           Agent skill playbooks
  telemetry/        OpenTelemetry integration
  tools/            All diagnostic tool implementations
  types/            Shared types (findings, errors, metadata)
deploy/
  helm/             Helm chart
  manifests/        Raw YAML manifests
docs/               MkDocs documentation
```
