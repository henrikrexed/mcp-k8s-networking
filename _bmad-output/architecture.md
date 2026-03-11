# Architecture Document: k8s-networking MCP v2 — Diagnostic Enrichment

**Version:** 1.0
**Date:** 2026-03-10
**Status:** Approved

---

## 1. System Context

The k8s-networking MCP server is a Go binary exposing 42+ tools via the Model Context Protocol (MCP). An AI model (Claude) calls these tools to diagnose Kubernetes networking issues. The server queries the Kubernetes API dynamically using client-go's dynamic and typed clients.

```
AI Model (Claude) <--MCP--> k8s-networking server <--K8s API--> Cluster
```

### Key Architectural Constraints

- **Single binary, no external dependencies** — all state comes from the K8s API at query time
- **Dynamic CRD discovery** — tools are conditionally registered based on which CRDs exist in the cluster
- **RBAC-aware** — tools degrade gracefully when permissions are insufficient
- **Markdown table output** — all tool responses use `DiagnosticFinding` structs rendered as markdown tables via `FindingsToText()`

---

## 2. Architecture Decision Records

### ADR-1: Conditional Enrichment Pattern

**Status:** Accepted
**Context:** Adding diagnostic data to tool responses risks bloating output for the AI model, increasing token usage and reducing signal-to-noise ratio. Happy-path resources (everything healthy) should produce minimal output.

**Decision:** All new fields/warnings follow the **conditional enrichment** principle:
- Omit data when resources are in expected/healthy state
- Emit `DiagnosticFinding` with `warning` or `critical` severity only when a problem is detected
- Never add data that duplicates what's already visible in the response

**Implementation Pattern:**
```go
// DO: conditional warning
if !isAccepted(parentConditions) {
    findings = append(findings, types.DiagnosticFinding{
        Severity: types.SeverityWarning,
        Summary:  fmt.Sprintf("NOT_ACCEPTED(%s)", reason),
    })
}
// DON'T: unconditional status field
// summary += fmt.Sprintf(" status=Accepted") // wasteful on happy path
```

**Consequences:**
- Happy-path output size increases by <5%
- Problem detection requires no additional tool calls by the model
- Each tool handler has conditional logic branches (manageable complexity)

---

### ADR-2: Enrich Existing Tools vs. New Tools

**Status:** Accepted
**Context:** Two approaches for surfacing new diagnostics: (a) add findings to existing tools, (b) create new dedicated tools the model must learn to call.

**Decision:** Prefer enriching existing tools. Create new tools only when the diagnostic requires distinct input parameters (e.g., `check_rate_limit_policies` needs service name, not route name).

**Rationale:**
- The model already knows when to call `get_httproute` — enriching it means zero learning curve
- Tool count remains manageable (42 → 44-45, not 55+)
- Conditional enrichment keeps response sizes stable
- New tools reserved for genuinely new capabilities (rate limit lookup, data plane health)

**Tool Count Impact:**
| Sprint | New Tools | Enriched Existing Tools |
|--------|-----------|------------------------|
| 1 | 0 | 5 |
| 2 | 0 | 4 |
| 3 | 1 | 4 |
| 4 | 1 | 2 |

---

### ADR-3: Shared Helper Extraction for Cross-Namespace Reference Validation

**Status:** Accepted
**Context:** Cross-namespace ReferenceGrant validation exists in `scan_gateway_misconfigs` (lines 1776-1786 of gateway_api.go). The same logic is needed in `get_httproute` and `get_grpcroute` per US-1.3.

**Decision:** Extract a shared helper function `validateCrossNamespaceRef()` that:
1. Takes route namespace, route kind, backend ref namespace, backend ref name, and the list of ReferenceGrants
2. Returns `*DiagnosticFinding` (nil if no issue, warning if grant missing)
3. Is called by `get_httproute`, `get_grpcroute`, and `scan_gateway_misconfigs`

**Implementation:**
```go
func validateCrossNamespaceRef(
    routeNs, routeKind, backendNs, backendName string,
    refGrants []refGrantEntry,
    routeRef *types.ResourceRef,
) *types.DiagnosticFinding
```

**Consequences:**
- Single source of truth for ReferenceGrant matching logic
- `scan_gateway_misconfigs` refactored to use the shared helper (behavior unchanged)
- Edge cases (wildcard names, multiple grants) handled in one place

---

### ADR-4: Cilium CRD Handling via Dynamic Client

**Status:** Accepted
**Context:** Cilium CRDs (`CiliumNetworkPolicy`, `CiliumClusterwideNetworkPolicy`) have schema differences across versions (1.14+). Using generated typed clients would create version coupling.

**Decision:** Use `k8s.io/client-go/dynamic` unstructured client for all Cilium resources, consistent with the existing pattern for Gateway API and Istio CRDs.

**Rationale:**
- Dynamic client already used for Gateway API (`gatewaysV1GVR`), Istio (`virtualServicesGVR`), and existing Cilium tools
- No generated client dependency — works with any Cilium version that has the CRD
- CRD presence detected via existing `HasCilium` discovery flag
- Fields extracted via `unstructured.NestedMap/Slice/String` helpers

**Consequences:**
- No compile-time type safety for Cilium fields (accepted trade-off)
- Must handle missing/renamed fields gracefully with nil checks
- Consistent with all other CRD tool implementations in the codebase

---

### ADR-5: Data Plane Health — API-Level Only (No Exec)

**Status:** Accepted
**Context:** Deep data plane diagnostics (xDS sync status, envoy config dump) require pod exec. This has significant RBAC implications and security concerns.

**Decision:** Sprint 4's `check_dataplane_health` uses only the Pod API (no exec):
- Sidecar container presence and readiness
- Container restart counts
- Image version comparison
- Init container status

Exec-based diagnostics deferred to future opt-in feature.

**Rationale:**
- Pod read is standard RBAC (already required for existing tools)
- Exec requires additional `pods/exec` RBAC — many clusters restrict this
- API-level checks cover ~70% of common sidecar issues (not running, crash-looping, version skew)
- Exec-based features can be added later behind a feature flag

**Consequences:**
- Cannot detect xDS desync, envoy config issues, or proxy-level errors
- Acceptable for initial release; exec-based checks documented as future work

---

### ADR-6: Event Correlation as Cross-Cutting Helper

**Status:** Accepted
**Context:** Kubernetes Events provide valuable context when resources have problems (e.g., why a route was rejected). Events should enrich findings but only when a problem is already detected.

**Decision:** Implement `fetchRecentEvents()` helper that:
1. Fetches Events for a specific resource (name, kind, namespace)
2. Filters to last 1 hour, max 5 events
3. Returns formatted event messages
4. Called only within conditional branches where a problem finding is being created

**Implementation:**
```go
func fetchRecentEvents(ctx context.Context, client kubernetes.Interface,
    ns, kind, name string) []string
```

**Consequences:**
- Additional K8s API calls only on problem paths (not happy paths)
- Event fetching is optional — if it fails (RBAC), findings still work without events
- Incrementally added: Sprint 1 for routes/gateways, Sprint 2 for Istio resources

---

### ADR-7: Status Enrichment in List Views — Summary String Approach

**Status:** Accepted
**Context:** List views (e.g., `list_httproutes`) produce one `DiagnosticFinding` per resource with a summary string. Route acceptance status needs to be visible in this summary.

**Decision:** Append status indicators to the existing summary string rather than adding new columns or separate findings per route.

**Format:**
```
ns/route-name parents=[gw/my-gw] rules=2 backends=[svc:80] ⚠ NOT_ACCEPTED(NoMatchingListenerHostname)
```

**Rationale:**
- Maintains one finding per resource (consistent with current list view pattern)
- Status is only appended when there's a problem (conditional enrichment)
- No change to markdown table structure
- Model sees status inline with the resource it applies to

**Consequences:**
- Summary strings get longer when problems exist (acceptable, problem paths deserve detail)
- Happy-path summaries unchanged (zero overhead)

---

## 3. Component Architecture

### 3.1 Tool Layer (pkg/tools/)

```
pkg/tools/
├── gateway_api.go      # Gateway API tools (primary Sprint 1 target)
├── istio.go            # Istio tools (Sprint 2-3)
├── kgateway.go         # kgateway tools (Sprint 2-3)
├── provider_cilium.go  # Cilium tools (Sprint 4)
├── k8s_services.go     # Core K8s service tools
├── registry.go         # Tool registration
├── types.go            # BaseTool, StandardResponse, helpers
├── rate_limiting.go    # NEW Sprint 3
└── dataplane_health.go # NEW Sprint 4
```

### 3.2 Shared Helpers (to be extracted)

| Helper | Location | Sprint |
|--------|----------|--------|
| `validateCrossNamespaceRef()` | `gateway_api.go` | 1 |
| `fetchRecentEvents()` | `gateway_api.go` or new `helpers.go` | 1 |
| `checkWeightDistribution()` | `gateway_api.go` | 2 |
| `checkListenerTLS()` | `gateway_api.go` | 3 |
| `resolveExtensionRef()` | `gateway_api.go` | 3 |

### 3.3 Response Flow

```
Tool.Run(ctx, args)
  → Query K8s API (dynamic/typed client)
  → Build []DiagnosticFinding
  → Conditional enrichment (append warnings only on problems)
  → NewToolResultResponse(cfg, toolName, findings, ns, provider)
  → FindingsToText() → markdown table
  → MCP response to model
```

---

## 4. RBAC Requirements

### Current (unchanged)
- Gateway API CRDs: get, list
- Istio CRDs: get, list
- Services, Endpoints, Pods: get, list
- Events: list (new for Sprint 1+)

### Sprint 3 Addition
- Secrets: get (optional, for TLS cert reference validation)
  - Degrades gracefully: emits info finding if 403

### Sprint 4 Addition
- No new RBAC (Pod read already required)

---

## 5. Testing Strategy

- **Unit tests:** Fixture-based with mock K8s clients (fake clientset + fake dynamic client)
- **Test pattern:** Create unstructured objects with specific status conditions, run tool, assert findings
- **Coverage priority:** Conditional enrichment paths (problem detection) over happy paths
- **No integration tests in initial sprints** (manual testing against real clusters)
