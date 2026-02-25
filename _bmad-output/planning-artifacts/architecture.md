---
stepsCompleted: [1, 2, 3, 4, 5, 6, 7, 8]
status: 'complete'
completedAt: '2026-02-22'
inputDocuments: ['prd.md', 'product-brief-mcp-k8s-networking-2026-02-22.md']
workflowType: 'architecture'
project_name: 'mcp-k8s-networking'
user_name: 'Henrik.rexed'
date: '2026-02-22'
---

# Architecture Decision Document

_This document builds collaboratively through step-by-step discovery. Sections are appended as we work through each architectural decision together._

## Project Context Analysis

### Requirements Overview

**Functional Requirements:**
49 FRs across 9 capability areas. The requirements break into three architectural domains:
1. **Cluster introspection engine** (FR1-FR5): Dynamic CRD discovery, provider detection, tool registry adaptation — the foundational layer everything else depends on
2. **Diagnostic tool suite** (FR6-FR34): Provider-specific diagnostics (Tier 1 deep, Tier 2 basic), core K8s networking checks, and active probing via ephemeral pods — the primary value delivery
3. **Agent communication layer** (FR35-FR49): MCP protocol (SSE + JSON-RPC 2.0), compact/detail output modes, design guidance responses, deployment and operations — the interface contract with AI agents

**Non-Functional Requirements:**
23 NFRs that shape architectural decisions:
- **Performance**: 5s compact responses, 30s active probing, <500 tokens compact output — drives caching strategy and response formatting
- **Security**: Minimum-privilege RBAC, restricted pod security context, SSE authentication, no secrets in logs — constrains the access model
- **Scalability**: 2-10 concurrent agent sessions, horizontal scaling via replicas — drives SSE session management and state decisions
- **Reliability**: Multi-replica HA, orphaned pod cleanup, API server failure recovery, SSE reconnection — drives resilience patterns
- **Integration**: MCP spec compliance, K8s 1.28+, Gateway API stable+beta versions — constrains protocol and compatibility choices

**Scale & Complexity:**

- Primary domain: Backend infrastructure / Kubernetes-native server
- Complexity level: Medium-high
- Estimated architectural components: 6-8 major components (MCP server, CRD discovery, tool registry, provider modules, probe manager, response formatter, K8s client layer, config)

### Technical Constraints & Dependencies

- **Go 1.22+** with client-go and dynamic client — non-negotiable, aligns with K8s ecosystem
- **MCP protocol over SSE + JSON-RPC 2.0** — protocol is specified, transport is SSE
- **Distroless container** — minimal attack surface, constrains debugging in production
- **Kubernetes 1.28+ minimum** — sets API compatibility floor
- **ClusterRole RBAC** — read-only on networking resources, create/delete for diagnostic pods in dedicated namespace
- **No persistent storage** — all state derived from cluster API; no database dependency
- **Solo developer resource constraint** — architecture must be maintainable by one person, extensible for community contributors

### Cross-Cutting Concerns Identified

1. **CRD-aware graceful degradation**: Every tool must handle "provider not installed" as an informative response, not an error. This pattern pervades the entire codebase.
2. **Token-efficient output formatting**: Compact/detail modes affect every tool's response path. Needs a consistent response formatting layer.
3. **Structured logging**: JSON logs across all components, must never leak secrets/tokens/keys.
4. **SSE connection lifecycle**: Heartbeats, reconnection, graceful disconnect, connection draining on shutdown — affects the server core.
5. **Ephemeral pod lifecycle**: Deploy, execute, collect results, cleanup with TTL — a cross-cutting concern for all active probing tools.
6. **Health and readiness**: K8s probes must reflect actual server state (CRD discovery complete, SSE server ready, K8s client connected).

## Starter Template Evaluation

### Primary Technology Domain

Backend infrastructure — Go-based MCP server deployed as an in-cluster Kubernetes workload. No frontend, no database, no operator framework needed.

### Starter Options Considered

**Option 1: Official Go MCP SDK (`modelcontextprotocol/go-sdk` v1.0.0)**
- Stable release with backward-compatibility guarantee
- Maintained by Go team at Google + Anthropic
- Supports Streamable HTTP (successor to pure SSE) — fewer TCP connections under load, better session management
- 395 importers, MIT license
- Built-in transport layer, JSON-RPC handling, tool registration API

**Option 2: Community Go MCP SDK (`mark3labs/mcp-go`)**
- Most popular Go MCP library — 1,307 importers
- Supports SSE, Streamable HTTP, and stdio transports
- Implements MCP spec 2025-11-25 with backward compatibility
- MIT license, wider community battle-testing

**Option 3: Custom implementation (no SDK)**
- Maximum control but significant effort for solo developer
- High maintenance burden for MCP spec evolution — not recommended

### Selected Starter: Official Go MCP SDK (`modelcontextprotocol/go-sdk` v1.0.0)

**Rationale for Selection:**
- **Official + stable**: v1.0.0 with backward-compatibility guarantee. For a CNCF-targeting project, the official SDK adds credibility.
- **Maintained by Google's Go team + Anthropic**: Long-term alignment with both Go ecosystem and MCP spec evolution.
- **Streamable HTTP built-in**: Better performance for multi-agent concurrency (2-10 sessions). Fewer TCP connections under load than pure SSE.
- **Clean API**: `mcp.Server` + `mcp.Transport` pattern aligns with dynamic tool registry architecture.

**Transport Update**: The PRD specified SSE, but the MCP spec has evolved. **Streamable HTTP** is now the recommended transport — a hybrid that uses HTTP POST for requests and SSE streams for responses. This is a positive evolution for the horizontal scaling and concurrent session requirements.

**Initialization Command:**

```bash
mkdir k8s-networking-mcp && cd k8s-networking-mcp
go mod init github.com/mcp-k8s-networking/mcp-k8s-networking
go get github.com/modelcontextprotocol/go-sdk@v1.0.0
go get k8s.io/client-go@latest
go get k8s.io/apimachinery@latest
```

**Architectural Decisions Provided by SDK:**

| Decision | Provided By |
|---|---|
| MCP protocol compliance | go-sdk handles JSON-RPC 2.0, tool registration, capabilities negotiation |
| Transport layer | Streamable HTTP (SSE + POST hybrid) |
| Session management | Built-in session lifecycle, cleanup, heartbeats |
| Tool registration API | `mcp.Server` API for registering/listing tools |
| Auth primitives | `auth` package for OAuth support |

**Decisions Still Needed (not provided by SDK):**

| Decision | Needs Architecture |
|---|---|
| K8s client setup | client-go configuration, in-cluster auth, dynamic client |
| CRD discovery mechanism | Polling interval, discovery endpoint usage |
| Dynamic tool registry | Mapping discovered CRDs to tool registration |
| Provider module interface | Tier 1 vs Tier 2 abstraction |
| Ephemeral pod management | Deploy, execute, cleanup, TTL, concurrency |
| Response formatting | Compact/detail modes, token efficiency |
| Project structure | Manual Go layout per PRD spec |
| Build tooling | Makefile + multi-stage Dockerfile |
| Testing | Go standard testing + testify |

**Note:** Project initialization using these commands should be the first implementation story.

## Core Architectural Decisions

### Decision Priority Analysis

**Critical Decisions (Block Implementation):**
- Cluster data access pattern (dynamic client everywhere)
- CRD re-discovery mechanism (watch-based)
- MCP transport (Streamable HTTP via go-sdk)
- Provider module interface (tiered interfaces)
- Tool granularity (concern-based with dispatcher)

**Important Decisions (Shape Architecture):**
- Agent authentication (K8s ServiceAccount + TokenReview)
- Diagnostic pod security and image (Alpine minimal, restricted PSS)
- Session-scoped caching for diagnostic workflows
- Logging with OTel bridge

**Deferred Decisions (Post-MVP):**
- Out-of-cluster agent authentication (Phase 3, multi-cluster)
- OAuth 2.0 support (if needed for external agent platforms)
- Metrics endpoint implementation (Phase 2)

### Cluster Data Access

**Resource Access Pattern: Dynamic Client for Everything**
- All K8s resource reads use `client-go` dynamic client with on-demand API calls
- No informer setup — simpler codebase, lower memory footprint, no cache synchronization
- Tradeoff: slightly higher per-query latency, more API server calls — mitigated by session-scoped caching
- Rationale: Solo developer maintainability. Informers can be introduced selectively in Phase 2 if performance demands it.

**CRD Re-Discovery: Watch-Based**
- Watch the CRD resource (`apiextensions.k8s.io/v1`) for create/delete events
- Real-time detection of provider installations/removals — tools appear/disappear immediately
- Single watch connection — lightweight overhead
- On startup: full discovery scan. After startup: watch for changes.

**Diagnostic Session Cache: Session-Scoped**
- Cache resource query results within an agent session for a short TTL (default: 30 seconds)
- Reduces redundant API calls during multi-step diagnostic workflows (e.g., Journey 4's 4-step sequence)
- Cache keyed by: session ID + resource type + namespace + name
- Cache invalidated on session close or TTL expiry
- No cross-session caching — keeps state simple and avoids stale data across agents

### Authentication & Security

**Agent Authentication: K8s ServiceAccount + TokenReview API**
- Agents send their ServiceAccount token as `Bearer` token in HTTP `Authorization` header
- MCP server validates the token using the K8s TokenReview API
- K8s-native: zero external auth infrastructure, automatic token rotation
- Future: out-of-cluster auth (kubeconfig, OAuth) deferred to Phase 3

**Diagnostic Pod Security: Restricted PSS + Custom Alpine Image**
- Custom minimal Alpine-based image (~20MB) with: `curl`, `dig`, `ncat`
- Published alongside the MCP server image on ghcr.io
- Restricted Pod Security Standards compliant:
  - `runAsNonRoot: true`
  - Drop all capabilities
  - `seccompProfile: RuntimeDefault`
  - `emptyDir` volume at `/tmp` for temp files
- Resource limits enforced (CPU/memory) via MCP server configuration

### Provider Module Architecture

**Tiered Provider Interfaces:**
```
Provider (base interface)
├── Detect() — is this provider installed?
├── Status() — basic health/status
├── ListTools() — what tools does this provider expose?
└── Diagnose(concern) — basic diagnostic for a concern

DeepProvider (extends Provider — Tier 1 only)
├── DeepDiagnose(concern) — full diagnostic with root cause analysis
├── DesignGuidance(context) — CRD-aware design recommendations
└── ValidateConfig(resource) — configuration validation
```
- All providers implement `Provider` (Tier 2 boundary)
- Tier 1 providers (Istio, Gateway API, kgateway) additionally implement `DeepProvider`
- Community contributors implement `Provider` for new providers, promote to `DeepProvider` for deep coverage

**Concern-Based Tool Grouping with Dispatcher:**
- Agent-facing MCP tools are provider-agnostic: `diagnose_routing`, `diagnose_dns`, `diagnose_tls`, `diagnose_network_policy`, `check_connectivity`, `design_guidance`
- Dispatcher layer maps tool calls to discovered providers
- If multiple providers are relevant (e.g., both Istio and Calico for network policies), dispatcher queries both and aggregates results
- Agent never needs to know which mesh/CNI is installed — it describes the concern, the MCP routes to the right providers
- Design guidance tools use CRD introspection to tailor recommendations to what's actually available

### Infrastructure & Deployment

**Logging: `slog` + `otelslog`**
- Go standard library `log/slog` for structured JSON logging
- `otelslog` bridge from OpenTelemetry contrib for trace context injection (trace/span IDs)
- OTel-native from day one — aligns with Phase 3 OpenTelemetry awareness expansion
- Secrets/tokens/keys filtered from log output at the slog handler level

**Testing: `testing` + `testify` + `envtest`**
- Go standard `testing` package as runner
- `testify` for assertions and mocks
- `sigs.k8s.io/controller-runtime/pkg/envtest` for integration tests against a real API server
- CRD discovery, tool registration, and K8s interactions tested against envtest API server
- Unit tests for provider logic, integration tests for K8s interactions, E2E tests for full MCP workflows

**CI/CD: GitHub Actions + GoReleaser**
- GitHub Actions for CI (lint, test, build) on every PR
- GoReleaser for automated releases: multi-arch container builds, Helm chart packaging, changelog generation
- Semantic versioning tags
- envtest in CI for K8s integration tests

**Container Registry: ghcr.io**
- GitHub Container Registry for all images (MCP server + diagnostic probe image)
- Free for public repos, integrated with GitHub Actions
- Semantic versioning tags + `latest`

### Decision Impact Analysis

**Implementation Sequence:**
1. Project init (go mod, go-sdk, client-go)
2. K8s client setup with dynamic client + CRD watch
3. Provider interface definitions (Provider + DeepProvider)
4. Concern-based dispatcher + tool registration with go-sdk
5. Core K8s diagnostic tools (Services, DNS, NetworkPolicy)
6. Tier 1 provider modules (Istio, Gateway API, kgateway)
7. Active probing (ephemeral pod manager + Alpine diagnostic image)
8. Session-scoped caching
9. SA auth via TokenReview
10. Helm chart + deployment manifests
11. Tier 2 basic provider modules

**Cross-Component Dependencies:**
- Dispatcher depends on: CRD discovery (to know what's available) + Provider interfaces (to route calls)
- Tool registration depends on: Dispatcher (to expose concern-based tools via go-sdk)
- Session cache depends on: Streamable HTTP session management (provided by go-sdk)
- Active probing depends on: Diagnostic container image built and published first
- Auth depends on: K8s client setup (TokenReview API access)

### MCP Protocol Implementation Specification

**JSON-RPC 2.0 Message Format:**
- Tool listing: `tools/list` → returns array of tool schemas
- Tool invocation: `tools/call` with `{name, arguments}` → returns structured result
- Connection lifecycle: `initialize` → `initialized` → `tools/list` → `tools/call` (repeatable) → disconnect

**Tool Schema Format:**
- `name`: string — concern-based tool name (e.g., `diagnose_routing`)
- `description`: string — agent-readable purpose description
- `inputSchema`: JSON Schema defining accepted parameters

**Error Handling:**
- Standard JSON-RPC 2.0 error codes + MCP-specific codes
- Provider-not-installed returns informative message, not error
- Tool timeout returns structured timeout response with partial results if available

### Tool Interface Specification

**Go Interface:**
```go
type Tool interface {
    Name() string
    Description() string
    InputSchema() json.RawMessage
    Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error)
}

type ToolResult struct {
    Content   []Content         `json:"content"`
    Metadata  *ClusterMetadata  `json:"metadata,omitempty"`
    IsError   bool              `json:"isError,omitempty"`
}

type ClusterMetadata struct {
    ClusterName string    `json:"clusterName"`
    Timestamp   time.Time `json:"timestamp"`
    Namespace   string    `json:"namespace,omitempty"`
    Provider    string    `json:"provider,omitempty"`
}
```

- Input validation via JSON Schema before `Execute()`
- Per-tool timeout via context (default from `TOOL_TIMEOUT` config)
- All tool results include `ClusterMetadata` for multi-cluster correlation
- Structured JSON output with compact/detail modes

### CRD Discovery Specification

- **On startup**: Check available API groups via discovery endpoint, register tools for detected CRDs
- **Watch-based re-check**: Watch `apiextensions.k8s.io/v1` CRDs for install/removal after startup
- **Dynamic tool registration**: CRD appears → register corresponding tools with MCP server. CRD removed → deregister tools.
- **Graceful degradation**: Istio CRDs unavailable → Istio tools not registered, all other tools function normally. Agent sees only available tools.

### Tool Category Specifications

**Gateway API Tools (Tier 1):**
- `gateway_list` — list Gateways with status conditions
- `gateway_get` — detailed Gateway with listeners, attached routes
- `check_route_resolution` — validate HTTPRoute/GRPCRoute resolves to healthy backends
- `check_gateway_status` — Gateway readiness, listener status, accepted routes count

**Istio Tools (Tier 1):**
- `istio_list` — list Istio resources by type (VirtualService, DestinationRule, AuthorizationPolicy)
- `istio_get` — detailed resource with validation findings
- `istio_proxy_config` — sidecar proxy configuration for a pod
- `istio_mtls_status` — mTLS validation between services
- `istio_sidecar_injection` — injection status across deployments in namespace

**Cilium Tools (Tier 2):**
- `cilium_list` — list CiliumNetworkPolicies
- `cilium_get` — detailed policy with endpoint matching
- `cilium_connectivity` — basic connectivity status
- `cilium_identity` — endpoint identity mapping

**Core K8s Networking Tools:**
- `k8s_services` — service list with endpoint health, selector matching
- `k8s_endpoints` — endpoint details with ready/not-ready breakdown
- `k8s_networkpolicies` — policy analysis with ingress/egress rule summary

**Diagnostic Tools (concern-based, cross-provider):**
- `diagnose_service` — full stack diagnosis for a service (endpoints, routing, mesh, DNS)
- `diff_routes` — compare expected vs actual route resolution
- `find_blocking_policies` — identify NetworkPolicies/AuthorizationPolicies blocking traffic between two services

### Configuration Specification

| Variable | Required | Default | Description |
|---|---|---|---|
| `CLUSTER_NAME` | Yes | — | Identifier for this cluster (used in ClusterMetadata) |
| `PORT` | No | `8080` | HTTP server listen port |
| `NAMESPACE` | No | all | Filter diagnostics to specific namespace |
| `LOG_LEVEL` | No | `info` | Logging level (debug, info, warn, error) |
| `CACHE_TTL` | No | `30s` | Session-scoped cache time-to-live |
| `TOOL_TIMEOUT` | No | `10s` | Default per-tool execution timeout |
| `PROBE_NAMESPACE` | No | `mcp-diagnostics` | Namespace for ephemeral diagnostic pods |
| `PROBE_IMAGE` | No | `ghcr.io/mcp-k8s-networking/probe:latest` | Diagnostic probe container image |
| `MAX_CONCURRENT_PROBES` | No | `5` | Maximum simultaneous diagnostic pods |

### Helm Chart Values Specification

```yaml
# Image
image:
  repository: ghcr.io/mcp-k8s-networking/server
  tag: latest
  pullPolicy: IfNotPresent

# Scaling
replicas: 1
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 512Mi

# Service Account & RBAC
serviceAccount:
  create: true
  name: mcp-k8s-networking
rbac:
  create: true

# Service
service:
  type: ClusterIP
  port: 8080

# Optional Ingress
ingress:
  enabled: false

# Configuration
config:
  clusterName: ""          # Required
  logLevel: info
  cacheTTL: 30s
  toolTimeout: 10s

# Diagnostic Probing
probe:
  namespace: mcp-diagnostics
  image: ghcr.io/mcp-k8s-networking/probe:latest
  maxConcurrent: 5

# Feature Flags (auto-detected but overridable)
features:
  enableIstio: auto
  enableCilium: auto
  enableGatewayAPI: auto
  enableKuma: auto
  enableLinkerd: auto
  enableCalico: auto
  enableFlannel: auto
```

## ADR: OpenTelemetry GenAI + MCP Semantic Conventions

### Decision

Adopt the OpenTelemetry GenAI semantic conventions and MCP-specific attributes for all MCP server self-instrumentation. All three OTel signals (traces, metrics, logs) are emitted via OTLP gRPC. The instrumentation follows a middleware pattern that wraps every MCP tool call handler.

### Context

The MCP server needs observability into tool execution performance, error rates, and end-to-end trace correlation from AI agent through to K8s API calls. The OTel GenAI semantic conventions (https://opentelemetry.io/docs/specs/semconv/gen-ai/) define standard attribute names for AI/ML tool invocations. The MCP protocol has emerging conventions for session and method tracing.

### Middleware Pattern

All MCP tool calls are instrumented via a middleware wrapper in `pkg/mcp/server.go` that intercepts the `buildHandler` function:

```
AI Agent (traceparent in params._meta)
  │
  ▼ (extract W3C trace context)
MCP Tool Call Middleware (pkg/mcp/server.go)
  │ ─── Creates span: "execute_tool {tool_name}"
  │ ─── Sets GenAI + MCP span attributes
  │ ─── Records request duration metric
  │
  ▼ (propagated context)
Tool.Run(ctx, args)
  │
  ▼ (child spans for K8s API calls)
K8s API Server
```

### Span Attribute Mapping

Every MCP tool invocation span carries these attributes:

| Attribute | Source | Example |
|---|---|---|
| `gen_ai.operation.name` | Constant | `"execute_tool"` |
| `gen_ai.tool.name` | `tool.Name()` | `"list_services"` |
| `mcp.method.name` | JSON-RPC method | `"tools/call"` |
| `mcp.protocol.version` | MCP version | `"2025-03-26"` |
| `mcp.session.id` | Session context | `"sess_abc123"` |
| `jsonrpc.request.id` | Request ID | `"req_42"` |
| `gen_ai.tool.call.arguments` | Sanitized args JSON | `{"namespace":"default"}` |
| `gen_ai.tool.call.result` | Truncated result (1024 chars) | `{"cluster":"prod",...}` |
| `error.type` | MCPError code or `"tool_error"` | `"PROVIDER_NOT_FOUND"` |

### Context Propagation

W3C Trace Context (`traceparent`/`tracestate`) is extracted from `params._meta` in the MCP request. This enables end-to-end traces: AI agent → MCP server → K8s API. The extracted context becomes the parent of the tool invocation span. If no trace context is present, a new trace is started.

```go
// Extract from params._meta
if meta, ok := request.Params.Meta; ok {
    carrier := propagation.MapCarrier(meta)
    ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)
}
```

### Metrics

**GenAI semantic convention metrics:**
- `gen_ai.server.request.duration` (histogram, seconds) — tool execution time, dimensioned by `gen_ai.tool.name`, `error.type`
- `gen_ai.server.request.count` (counter) — tool invocation count, dimensioned by `gen_ai.tool.name`, `error.type`

**Custom domain metrics:**
- `mcp.findings.total` (counter) — diagnostic findings emitted, dimensioned by `severity`, `analyzer` (tool name)
- `mcp.errors.total` (counter) — tool errors, dimensioned by `error.code`, `gen_ai.tool.name`

### Log Bridge

slog output is bridged to OTel Logs via `go.opentelemetry.io/contrib/bridges/otelslog`. When an active span context exists, every log entry is automatically enriched with `trace_id` and `span_id` for cross-signal correlation. When tracing is disabled, logs emit normally without trace fields.

### Telemetry Package (pkg/telemetry/)

The `pkg/telemetry/` package initializes all three signal providers:

- `TracerProvider` with OTLP gRPC trace exporter
- `MeterProvider` with OTLP gRPC metric exporter (periodic reader, 30s interval)
- `LoggerProvider` with OTLP gRPC log exporter (batch processor)

All providers share a common `resource.Resource` with `service.name`, `service.version`, and `k8s.cluster.name`. When `OTEL_EXPORTER_OTLP_ENDPOINT` is not set, noop providers are used and the server operates without telemetry overhead.

### Consequences

- Every MCP tool call produces a span with consistent attributes — enables dashboards and alerting on tool performance
- End-to-end traces from agent to K8s API — enables root cause analysis for slow diagnostics
- GenAI metrics enable SLO monitoring on tool response times
- Custom metrics enable monitoring of diagnostic finding patterns (severity spikes, error trends)
- Log-trace correlation enables jumping from a log entry to its full trace in observability platforms
- Minimal performance overhead — noop providers when telemetry is disabled

## Implementation Patterns & Consistency Rules

### Critical Conflict Points Identified

12 areas where AI agents could make different choices, resolved below.

### Naming Patterns

**Go Package & Directory Naming:**
- Directories: **plural** — `pkg/tools/`, `pkg/providers/`, `pkg/probes/`
- Package names: match directory — `package tools`, `package providers`
- Exported functions: **verb-first** — `NewProvider()`, `ListGateways()`, `DiagnoseRouting()` (Go convention)
- Unexported helpers: **camelCase** — `buildFinding()`, `parseResource()`

**MCP Tool Naming:**
- Tool names: **snake_case** — `diagnose_routing`, `check_gateway_status`, `find_blocking_policies`
- Consistent verb prefixes: `diagnose_`, `check_`, `list_`, `get_`, `find_`, `diff_`

**JSON Field Naming:**
- All JSON output: **camelCase** — `clusterName`, `apiGroup`, `isError`
- Matches MCP protocol conventions and agent expectations

**Log Field Naming:**
- All structured log fields: **snake_case** — `tool_name`, `session_id`, `provider`
- Matches Go slog conventions

**Error Sentinels:**
```go
var (
    ErrProviderNotFound  = errors.New("provider not installed")
    ErrCRDNotAvailable   = errors.New("CRD not available in cluster")
    ErrProbeTimeout      = errors.New("diagnostic probe timed out")
    ErrProbeLimitReached = errors.New("concurrent probe limit reached")
    ErrAuthFailed        = errors.New("service account token validation failed")
)
// Always wrap with context: fmt.Errorf("istio mtls check for %s/%s: %w", ns, name, err)
```

### Structure Patterns

**Provider Module Structure (concern-split):**
```
pkg/tools/istio/
├── provider.go         # Provider + DeepProvider implementation, Detect(), Status()
├── virtualservice.go   # VirtualService diagnostics
├── destinationrule.go  # DestinationRule diagnostics
├── authpolicy.go       # AuthorizationPolicy diagnostics
├── mtls.go             # mTLS validation
├── sidecar.go          # Sidecar injection checks
├── guidance.go         # Design guidance for Istio
└── types.go            # Istio-specific types (if needed)
```

Every provider follows this pattern. New providers copy the structure. Each file is focused on one K8s resource type or one diagnostic concern.

**Test Organization (separate directory):**
```
test/
├── unit/
│   └── tools/
│       ├── istio/
│       │   ├── virtualservice_test.go
│       │   ├── authpolicy_test.go
│       │   └── mtls_test.go
│       ├── gateway/
│       └── k8s/
├── integration/          # envtest-based tests
│   ├── discovery_test.go
│   ├── registry_test.go
│   └── probe_test.go
└── e2e/                  # Full MCP workflow tests
    └── diagnostic_flow_test.go
```

### Format Patterns

**Diagnostic Finding Structure (all tools must use):**
```go
type DiagnosticFinding struct {
    Severity   string       `json:"severity"`              // critical, warning, info, ok
    Category   string       `json:"category"`              // routing, dns, tls, policy, mesh
    Resource   *ResourceRef `json:"resource"`              // what's affected
    Summary    string       `json:"summary"`               // compact one-liner (always present)
    Detail     string       `json:"detail,omitempty"`      // full explanation (detail mode only)
    Suggestion string       `json:"suggestion,omitempty"`  // recommended fix (detail mode only)
}
```

**Severity Levels (strict — no other values allowed):**
- `critical` — service is broken, immediate action needed
- `warning` — degradation or misconfiguration detected
- `info` — informational finding, no action needed
- `ok` — explicitly healthy check result

**Compact vs Detail Mode:**
- Compact: `Summary` only. `Detail` and `Suggestion` omitted (zero-value).
- Detail: All fields populated.
- Mode determined by tool input parameter `"detail": true/false`

**Agent-Facing Error Structure (with tool context):**
```go
type MCPError struct {
    Code    string `json:"code"`              // error code (e.g., "PROVIDER_NOT_FOUND")
    Message string `json:"message"`           // human/agent-readable message
    Tool    string `json:"tool"`              // which tool was executing
    Detail  string `json:"detail,omitempty"`  // additional context
}
```

**Standard Error Codes:**
- `PROVIDER_NOT_FOUND` — requested provider not installed in cluster
- `CRD_NOT_AVAILABLE` — required CRD not present
- `PROBE_TIMEOUT` — diagnostic pod execution exceeded timeout
- `PROBE_LIMIT_REACHED` — concurrent probe limit hit
- `AUTH_FAILED` — ServiceAccount token validation failed
- `INVALID_INPUT` — tool input parameters failed validation
- `INTERNAL_ERROR` — unexpected server error

### Communication Patterns

**Context Propagation (mandatory for all functions):**
```go
// Every function that hits K8s API or takes non-trivial time:
func (p *IstioProvider) CheckMTLS(ctx context.Context, ns, name string) ([]DiagnosticFinding, error)
```

Context carries:
- Session ID (for cache keying)
- Request trace ID (for OTel correlation)
- Timeout deadline (from TOOL_TIMEOUT config)

**Structured Logging (every log line must include):**
```go
slog.InfoContext(ctx, "checking mtls status",
    "tool_name", "istio_mtls_status",
    "provider", "istio",
    "session_id", sessionID,
    "namespace", namespace,
    "service", serviceName,
)
```

Mandatory fields: `tool_name`, `provider` (if applicable), `session_id`
Optional fields: `namespace`, `resource_kind`, `resource_name`
Forbidden in logs: secrets, tokens, certificate private keys, full YAML bodies

### Process Patterns

**Graceful Degradation (every provider must follow):**
```go
// Provider.Detect() returns false → tools not registered
// Tool called for missing CRD → return informative MCPError, NOT Go error
// Example: Istio not installed → agent sees no Istio tools in tools/list
// If CRD disappears mid-session → tool returns MCPError{Code: "CRD_NOT_AVAILABLE"}
```

**Ephemeral Pod Lifecycle (probe manager must enforce):**
1. Check concurrent probe count < MAX_CONCURRENT_PROBES
2. Create pod with restricted PSS, resource limits, TTL label
3. Wait for pod Running (with timeout)
4. Execute diagnostic command
5. Collect output
6. Delete pod (always — even on error)
7. Background cleanup goroutine: delete any pods with expired TTL labels on startup and periodically

**Resource Cleanup Pattern:**
```go
// Always use defer for cleanup
pod, err := probeManager.Deploy(ctx, spec)
if err != nil {
    return nil, fmt.Errorf("deploying probe: %w", err)
}
defer probeManager.Cleanup(ctx, pod) // runs even if Execute panics
```

### Enforcement Guidelines

**All AI Agents MUST:**
1. Use `DiagnosticFinding` struct for all diagnostic output — no custom response shapes
2. Return `MCPError` (with tool context) for all agent-facing errors — no raw error strings
3. Accept `context.Context` as first parameter for all K8s-interacting functions
4. Include mandatory log fields (`tool_name`, `session_id`) in every log statement
5. Follow concern-split file structure when adding new provider modules
6. Place tests in `test/` directory mirroring `pkg/` structure
7. Use severity levels exactly as defined — no custom severity strings

**Anti-Patterns to Avoid:**
- Returning raw `kubectl`-style text output instead of structured `DiagnosticFinding`
- Swallowing errors without wrapping — always `fmt.Errorf("context: %w", err)`
- Logging full resource YAML — log resource references only (`kind/namespace/name`)
- Creating custom response types per provider instead of using shared types
- Putting tests in the same directory as source code

## Project Structure & Boundaries

### Complete Project Directory Structure

```
k8s-networking-mcp/
├── cmd/
│   └── server/
│       └── main.go                    # Entry point: config load, K8s client, CRD discovery, MCP server start
│
├── pkg/
│   ├── config/
│   │   └── config.go                  # Configuration struct, env var loading, defaults, validation
│   │
│   ├── mcp/
│   │   ├── server.go                  # MCP server setup with go-sdk, tool registration, Streamable HTTP
│   │   ├── handler.go                 # JSON-RPC request handling, compact/detail mode dispatch
│   │   ├── auth.go                    # K8s ServiceAccount TokenReview authentication
│   │   ├── session.go                 # Session management, session-scoped cache
│   │   └── types.go                   # MCP protocol types, ToolResult, ClusterMetadata, MCPError
│   │
│   ├── discovery/
│   │   ├── discovery.go               # CRD discovery on startup via API server discovery endpoint
│   │   ├── watcher.go                 # CRD watch for real-time install/removal detection
│   │   └── registry.go               # Dynamic tool registry: CRD → provider → tool mapping
│   │
│   ├── dispatcher/
│   │   ├── dispatcher.go              # Concern-based routing: tool call → provider(s) dispatch
│   │   ├── aggregator.go             # Multi-provider result aggregation
│   │   └── types.go                   # DiagnosticFinding, ResourceRef, severity constants
│   │
│   ├── providers/
│   │   ├── interfaces.go              # Provider + DeepProvider interface definitions
│   │   ├── istio/                     # Tier 1 — Istio
│   │   │   ├── provider.go            # Provider + DeepProvider implementation, Detect(), Status()
│   │   │   ├── virtualservice.go      # VirtualService diagnostics
│   │   │   ├── destinationrule.go     # DestinationRule diagnostics
│   │   │   ├── authpolicy.go          # AuthorizationPolicy diagnostics
│   │   │   ├── peerauthentication.go  # PeerAuthentication diagnostics
│   │   │   ├── mtls.go                # mTLS validation between services
│   │   │   ├── sidecar.go             # Sidecar injection status checks
│   │   │   ├── guidance.go            # Istio-specific design guidance
│   │   │   └── types.go               # Istio-specific types
│   │   ├── gateway/                   # Tier 1 — Gateway API
│   │   │   ├── provider.go            # Provider + DeepProvider implementation
│   │   │   ├── gateway.go             # Gateway resource diagnostics
│   │   │   ├── httproute.go           # HTTPRoute validation
│   │   │   ├── grpcroute.go           # GRPCRoute validation
│   │   │   ├── referencegrant.go      # ReferenceGrant inspection
│   │   │   ├── conformance.go         # Spec conformance validation
│   │   │   ├── guidance.go            # Gateway API design guidance
│   │   │   └── types.go
│   │   ├── kgateway/                  # Tier 1 — kgateway
│   │   │   ├── provider.go
│   │   │   ├── diagnostics.go
│   │   │   ├── guidance.go
│   │   │   └── types.go
│   │   ├── cilium/                    # Tier 2 — Cilium
│   │   │   ├── provider.go            # Provider implementation (no DeepProvider)
│   │   │   ├── networkpolicy.go       # CiliumNetworkPolicy inspection
│   │   │   ├── connectivity.go        # Basic connectivity status
│   │   │   └── identity.go            # Endpoint identity mapping
│   │   ├── calico/                    # Tier 2 — Calico
│   │   │   ├── provider.go
│   │   │   └── networkpolicy.go
│   │   ├── kuma/                      # Tier 2 — Kuma
│   │   │   ├── provider.go
│   │   │   └── status.go
│   │   ├── linkerd/                   # Tier 2 — Linkerd
│   │   │   ├── provider.go
│   │   │   └── status.go
│   │   ├── flannel/                   # Tier 2 — Flannel
│   │   │   └── provider.go
│   │   └── k8s/                       # Core K8s networking (always available)
│   │       ├── provider.go            # Core K8s provider (always registered)
│   │       ├── services.go            # Service + Endpoint validation, selector matching
│   │       ├── networkpolicy.go       # NetworkPolicy analysis
│   │       ├── coredns.go             # CoreDNS configuration + resolution diagnostics
│   │       ├── kubeproxy.go           # kube-proxy health + config validation
│   │       └── ingress.go             # Ingress resource inspection
│   │
│   ├── probes/
│   │   ├── manager.go                 # Probe lifecycle: deploy, execute, collect, cleanup
│   │   ├── pod.go                     # Pod spec builder (restricted PSS, resource limits, TTL)
│   │   ├── cleanup.go                 # Orphaned pod cleanup (startup + periodic)
│   │   └── types.go                   # ProbeSpec, ProbeResult types
│   │
│   └── k8s/
│       ├── client.go                  # K8s client factory (in-cluster config, dynamic client)
│       └── resources.go               # Common resource helpers (get, list with dynamic client)
│
├── test/
│   ├── unit/
│   │   ├── config/
│   │   │   └── config_test.go
│   │   ├── mcp/
│   │   │   ├── auth_test.go
│   │   │   ├── session_test.go
│   │   │   └── handler_test.go
│   │   ├── discovery/
│   │   │   ├── discovery_test.go
│   │   │   └── registry_test.go
│   │   ├── dispatcher/
│   │   │   ├── dispatcher_test.go
│   │   │   └── aggregator_test.go
│   │   ├── providers/
│   │   │   ├── istio/
│   │   │   │   ├── virtualservice_test.go
│   │   │   │   ├── authpolicy_test.go
│   │   │   │   └── mtls_test.go
│   │   │   ├── gateway/
│   │   │   │   ├── httproute_test.go
│   │   │   │   └── conformance_test.go
│   │   │   └── k8s/
│   │   │       ├── services_test.go
│   │   │       └── coredns_test.go
│   │   └── probes/
│   │       ├── manager_test.go
│   │       └── pod_test.go
│   ├── integration/                   # envtest-based K8s API tests
│   │   ├── testmain_test.go           # envtest setup/teardown
│   │   ├── discovery_test.go          # CRD discovery against real API
│   │   ├── registry_test.go           # Tool registration with real CRDs
│   │   └── probe_test.go             # Pod lifecycle against real API
│   └── e2e/                           # Full MCP workflow tests
│       ├── diagnostic_flow_test.go    # Agent → MCP → diagnose → response
│       └── fixtures/
│           ├── istio-misconfig.yaml   # Istio misconfiguration scenarios
│           └── gateway-misconfig.yaml # Gateway API misconfiguration scenarios
│
├── deploy/
│   ├── helm/
│   │   └── mcp-k8s-networking/
│   │       ├── Chart.yaml
│   │       ├── values.yaml
│   │       ├── templates/
│   │       │   ├── deployment.yaml
│   │       │   ├── service.yaml
│   │       │   ├── serviceaccount.yaml
│   │       │   ├── clusterrole.yaml
│   │       │   ├── clusterrolebinding.yaml
│   │       │   ├── configmap.yaml
│   │       │   ├── namespace-probe.yaml    # mcp-diagnostics namespace
│   │       │   ├── ingress.yaml            # optional
│   │       │   └── _helpers.tpl
│   │       └── .helmignore
│   └── manifests/
│       └── install.yaml               # Single-file kubectl apply alternative
│
├── build/
│   ├── Dockerfile                     # Multi-stage: Go build → distroless
│   └── Dockerfile.probe               # Alpine-based diagnostic probe image
│
├── .github/
│   └── workflows/
│       ├── ci.yml                     # Lint, test (unit + integration), build
│       └── release.yml                # GoReleaser: multi-arch build, Helm package, ghcr.io push
│
├── .goreleaser.yml                    # GoReleaser configuration
├── Makefile                           # build, test, lint, docker-build, helm-lint targets
├── go.mod
├── go.sum
├── .gitignore
├── LICENSE                            # Apache 2.0
├── README.md
├── CONTRIBUTING.md
└── CODE_OF_CONDUCT.md
```

### Architectural Boundaries

**MCP Protocol Boundary (pkg/mcp/):**
- Owns all agent-facing communication — Streamable HTTP, JSON-RPC, session management
- No K8s awareness — receives tool call requests, returns ToolResult responses
- Auth validation happens here (TokenReview) before any tool execution

**Discovery Boundary (pkg/discovery/):**
- Owns CRD detection and tool registry
- Talks to K8s API server only for discovery/CRD resources
- Notifies dispatcher when providers appear/disappear
- No diagnostic logic — only registration/deregistration

**Dispatcher Boundary (pkg/dispatcher/):**
- Owns concern-to-provider routing
- Receives concern-based tool calls, routes to correct provider(s)
- Aggregates multi-provider results
- No direct K8s API access — delegates to providers

**Provider Boundary (pkg/providers/):**
- Each provider owns its own K8s resource interactions
- Providers never call other providers directly
- All output via DiagnosticFinding struct
- Tier 1 providers implement both Provider + DeepProvider
- Tier 2 providers implement only Provider

**Probe Boundary (pkg/probes/):**
- Owns ephemeral pod lifecycle completely
- No diagnostic logic — just deploy, execute command, return output, cleanup
- Concurrency limiting and orphan cleanup are internal concerns

**K8s Client Boundary (pkg/k8s/):**
- Thin wrapper around client-go
- Provides dynamic client to all consumers
- In-cluster config detection
- No business logic — pure K8s API access

### Requirements to Structure Mapping

| FR Category | Primary Location | Supporting Files |
|---|---|---|
| CRD Discovery (FR1-FR5) | `pkg/discovery/` | `pkg/k8s/client.go` |
| Istio Diagnostics (FR6-FR10) | `pkg/providers/istio/` | `pkg/dispatcher/` |
| Gateway API Diagnostics (FR11-FR16) | `pkg/providers/gateway/` | `pkg/dispatcher/` |
| kgateway Diagnostics (FR17-FR18) | `pkg/providers/kgateway/` | `pkg/dispatcher/` |
| Core K8s Networking (FR19-FR23) | `pkg/providers/k8s/` | `pkg/dispatcher/` |
| Tier 2 Providers (FR24-FR28) | `pkg/providers/{cilium,calico,kuma,linkerd,flannel}/` | `pkg/dispatcher/` |
| Active Probing (FR29-FR34) | `pkg/probes/` | `pkg/k8s/client.go` |
| Design Guidance (FR35-FR38) | `pkg/providers/*/guidance.go` | `pkg/discovery/` |
| MCP Protocol (FR39-FR44) | `pkg/mcp/` | `pkg/dispatcher/` |
| Deployment (FR45-FR49) | `deploy/`, `build/`, `.github/` | `pkg/config/` |

### Data Flow

```
AI Agent
  │
  ▼ (Streamable HTTP + JSON-RPC 2.0)
pkg/mcp/server.go ──► pkg/mcp/auth.go (TokenReview)
  │
  ▼ (tool call with params)
pkg/dispatcher/dispatcher.go
  │
  ├──► pkg/providers/istio/*.go ──► pkg/k8s/client.go ──► K8s API
  ├──► pkg/providers/gateway/*.go ──► pkg/k8s/client.go ──► K8s API
  ├──► pkg/providers/k8s/*.go ──► pkg/k8s/client.go ──► K8s API
  └──► pkg/probes/manager.go ──► pkg/k8s/client.go ──► K8s API (create/delete pods)
  │
  ▼ ([]DiagnosticFinding)
pkg/dispatcher/aggregator.go (if multi-provider)
  │
  ▼ (ToolResult with ClusterMetadata)
pkg/mcp/handler.go (compact/detail formatting)
  │
  ▼ (JSON-RPC response)
AI Agent
```

### Development Workflow

**Build Targets (Makefile):**
- `make build` — compile server binary
- `make test` — run unit tests
- `make test-integration` — run envtest integration tests
- `make lint` — golangci-lint
- `make docker-build` — build server + probe images
- `make helm-lint` — validate Helm chart
- `make all` — lint + test + build

## Architecture Validation Results

### Validation Summary

- **Coherence**: All decisions work together — no conflicts, no version incompatibilities
- **Coverage**: 49/49 FRs covered, 23/23 NFRs addressed (2 with implementation notes)
- **Readiness**: AI agents can implement consistently with documented patterns, examples, and boundaries

### Coherence Validation

**Decision Compatibility:** PASS
- Go 1.22+ + client-go dynamic client + go-sdk v1.0.0 — fully compatible
- Streamable HTTP transport aligns with 2-10 concurrent session requirement
- slog + otelslog — standard library + official OTel bridge, zero conflict
- Dynamic client + session-scoped cache — coherent on-demand access pattern
- GoReleaser + GitHub Actions + ghcr.io — standard Go release pipeline

**Pattern Consistency:** PASS
- Naming conventions are clear and non-overlapping across contexts (Go, MCP tools, JSON, logs)
- Concern-split provider structure is uniform across all providers
- DiagnosticFinding and MCPError enforced as single output/error types

**Structure Alignment:** PASS
- Every FR category maps to a specific pkg/ directory
- MCP → Dispatcher → Providers → K8s dependency chain has no circular references
- Test directory mirrors pkg structure consistently

### Requirements Coverage

**Functional Requirements: 49/49 covered**
All FR categories have explicit architectural homes in the Requirements to Structure Mapping.

**Non-Functional Requirements: 23/23 addressed**

| NFR | Status | Notes |
|---|---|---|
| NFR1-4 (Performance) | Covered | Session cache compensates for dynamic client latency |
| NFR5-9 (Security) | Covered | RBAC, restricted PSS, TokenReview, log filtering |
| NFR10-13 (Scalability) | Covered | Streamable HTTP + replicas + configurable probe limits |
| NFR14-18 (Reliability) | Covered | Multiple replicas, orphan cleanup. K8s API retry: use client-go built-in retry with exponential backoff. Session cache enables reconnection without data loss. |
| NFR19-23 (Integration) | Covered | MCP spec via go-sdk, K8s 1.28+, Helm best practices, ghcr.io |

### Implementation Notes

**K8s API Retry Strategy (NFR16):**
- `pkg/k8s/client.go` configures client-go built-in retry with exponential backoff and jitter for transient API server failures
- No custom retry logic — leverage client-go's retry mechanisms

**Health Probes (FR47):**
- Liveness: HTTP 200 on `/healthz` — server process is alive
- Readiness: HTTP 200 on `/readyz` — K8s client connected AND initial CRD discovery complete AND MCP server accepting connections

**Graceful Shutdown (FR49):**
1. Stop accepting new connections
2. Drain active SSE/HTTP streams (go-sdk handles transport draining)
3. Cancel in-flight tool executions via context
4. Clean up any running diagnostic pods
5. Exit

### Architecture Completeness Checklist

**Requirements Analysis**
- [x] Project context thoroughly analyzed (49 FRs, 23 NFRs)
- [x] Scale and complexity assessed (medium-high)
- [x] Technical constraints identified (Go 1.22+, K8s 1.28+, solo developer)
- [x] Cross-cutting concerns mapped (6 identified)

**Architectural Decisions**
- [x] Critical decisions documented with versions (go-sdk v1.0.0, Go 1.22+, client-go)
- [x] Data access strategy defined (dynamic client + session cache + CRD watch)
- [x] Authentication specified (K8s ServiceAccount + TokenReview)
- [x] Provider architecture defined (tiered interfaces + concern-based dispatcher)
- [x] Tool specifications with input/output schemas
- [x] Configuration and Helm chart values specified

**Implementation Patterns**
- [x] Naming conventions established (Go, MCP tools, JSON, logs)
- [x] Structure patterns defined (concern-split providers)
- [x] Communication patterns specified (DiagnosticFinding, MCPError)
- [x] Process patterns documented (graceful degradation, probe lifecycle, cleanup)
- [x] Enforcement guidelines with anti-patterns

**Project Structure**
- [x] Complete directory structure with ~70 annotated files
- [x] 6 architectural boundaries defined
- [x] FR-to-structure mapping complete
- [x] Data flow diagram documented

### Architecture Readiness Assessment

**Overall Status:** READY FOR IMPLEMENTATION

**Confidence Level:** High

**Key Strengths:**
- Clean layered architecture with well-defined boundaries (MCP → Dispatcher → Providers → K8s)
- Tiered provider interfaces make Tier 2 contributions easy for the community
- Concern-based tool grouping makes the agent experience provider-agnostic
- Comprehensive patterns with code examples prevent agent implementation conflicts
- OTel-native logging from day one

**Areas for Future Enhancement:**
- Informer-based caching for hot-path resources (Phase 2 performance optimization)
- Multi-cluster agent orchestration architecture (Phase 3)
- OAuth 2.0 for out-of-cluster agents (Phase 3)

### Implementation Handoff

**AI Agent Guidelines:**
- Follow all architectural decisions exactly as documented
- Use implementation patterns consistently across all components
- Respect project structure and boundaries — no files outside defined structure
- Refer to this document for all architectural questions
- When in doubt, match the pattern shown in code examples

**First Implementation Priority:**
1. `go mod init` + dependency setup (go-sdk, client-go, apimachinery, testify, otelslog)
2. `pkg/config/config.go` — configuration loading from env vars
3. `pkg/k8s/client.go` — K8s client factory with in-cluster config
4. `pkg/discovery/` — CRD discovery + watcher
5. `pkg/providers/interfaces.go` — Provider + DeepProvider interfaces
6. `pkg/dispatcher/types.go` — DiagnosticFinding, ResourceRef, MCPError
7. `pkg/mcp/server.go` — MCP server with go-sdk + Streamable HTTP
8. First provider: `pkg/providers/k8s/` (always available, no CRD dependency)
