# Story 10.4: Architecture and Contributing Guide

Status: done

## Story

As a contributor or evaluator,
I want architecture documentation and a contributing guide,
so that I understand the system design and know how to add new tools, providers, or skills.

## Acceptance Criteria

1. `docs/architecture.md` describes the high-level component architecture (MCP server, tool registry, CRD discovery, skills framework)
2. `docs/architecture.md` explains the data flow from agent request through MCP protocol to Kubernetes API and back
3. `docs/architecture.md` documents key design decisions (CRD-based discovery, DiagnosticFinding types, ephemeral probes, skills as multi-step workflows)
4. `docs/contributing.md` explains development setup (Go version, make targets, running locally)
5. `docs/contributing.md` describes how to add a new provider and register its tools
6. `docs/contributing.md` covers code style conventions (slog, MCPError, DiagnosticFinding)

## Tasks / Subtasks

- [x] Create docs/architecture.md with component overview (cmd/server, pkg/mcp, pkg/tools, pkg/skills, pkg/discovery, pkg/k8s, pkg/config, pkg/types, pkg/telemetry)
- [x] Document data flow: agent -> MCP protocol -> tool registry -> tool execution -> K8s API -> DiagnosticFinding response
- [x] Document CRD discovery mechanism (watch-based, Features struct, onChange callback, dynamic tool registration)
- [x] Document design decisions: read-only cluster access, ephemeral probe pods, compact/detail mode, skills as multi-step generators
- [x] Create docs/contributing.md with development prerequisites (Go 1.25+, kubectl, cluster access)
- [x] Document development workflow: make build, make run, make lint, make test
- [x] Explain how to add a new provider: detect CRDs in discovery, create tool implementations, register in onChange callback
- [x] Explain how to add a new skill: implement Skill interface, register in SyncWithFeatures
- [x] Document code conventions: use slog (not log), return MCPError (not fmt.Errorf), use DiagnosticFinding for all output

## Dev Notes

### Component Architecture

The architecture page describes the following components:
- **cmd/server/main.go**: Entry point, wires all components, manages lifecycle
- **pkg/mcp/**: MCP protocol server (go-sdk based Streamable HTTP transport)
- **pkg/tools/**: Tool implementations and registry (Tool interface, BaseTool, StandardResponse)
- **pkg/skills/**: Multi-step guided workflow framework (Skill interface, Registry, SyncWithFeatures)
- **pkg/discovery/**: CRD watch-based provider detection (Features struct, onChange callbacks)
- **pkg/k8s/**: Kubernetes client wrapper (Dynamic, Discovery, Clientset)
- **pkg/config/**: Environment variable configuration loading
- **pkg/types/**: Shared types (DiagnosticFinding, MCPError, ClusterMetadata, ToolResult)
- **pkg/telemetry/**: OpenTelemetry trace provider initialization

### Key Design Decisions Documented

1. **CRD-based tool discovery**: Tools register/deregister dynamically based on installed CRDs, no restart needed
2. **Read-only by default**: All cluster access is read-only; only probe pods are created/deleted
3. **DiagnosticFinding as universal output**: All tools return structured findings, not raw maps
4. **Skills as generators**: Skills produce YAML manifests but never apply them, keeping the server safe
5. **Compact/detail modes**: Token-efficient by default, detailed when requested

### Contributing Guide Structure

The contributing guide covers:
- Dev environment setup
- Adding tools (implement Tool interface, register in onChange)
- Adding skills (implement Skill interface, register in SyncWithFeatures)
- Adding providers (extend Features struct, add CRD detection, create tools)
- Code conventions and review expectations

## File List

| File | Action |
|---|---|
| `docs/architecture.md` | Created |
| `docs/contributing.md` | Created |
