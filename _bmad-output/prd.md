# PRD: k8s-networking MCP Server — Diagnostic Enrichment & Gap Closure

**Version:** 1.0
**Date:** 2026-03-10
**Author:** BMAD Party Mode Consensus (Mary, Winston, John, Amelia, Quinn, Murat)
**Status:** Draft

---

## 1. Problem Statement

The k8s-networking MCP server (42 tools, Go) helps AI models diagnose Kubernetes networking issues via dynamic K8s API queries. Despite broad coverage of Gateway API, Istio, kgateway, and core K8s resources, the server has critical blind spots in **silent failure detection** — scenarios where resources appear correctly configured but silently do nothing. These include unaccepted routes, unready gateway listeners, missing cross-namespace grants, and undiagnosed traffic policies (rate limiting, circuit breaking, TLS, timeouts).

The AI model consuming these tools cannot diagnose what isn't surfaced. Adding more data doesn't automatically improve diagnosis — **conditional enrichment** (warnings only when misconfigured) is the design principle.

---

## 2. Goals

1. **Eliminate silent failure blind spots** — Surface route acceptance status, listener readiness, and cross-namespace reference violations so the model detects problems that currently produce zero signal.
2. **Surface traffic policy context** — Expose weight distribution, timeouts, retries, rate limits, circuit breakers, and TLS configuration so the model can diagnose intermittent and connection-level failures.
3. **Extend to Cilium, ambient mesh, and data plane health** — Cover the remaining networking providers and deep diagnostic scenarios.
4. **Apply conditional enrichment throughout** — Only add fields/warnings to tool output when they indicate a problem or deviation from expected state. Never bloat happy-path responses.

### Non-Goals

- Multi-cluster / federation support (MCS API, Cilium ClusterMesh)
- Full TLS certificate chain validation requiring Secret read access (defer to opt-in elevated RBAC)
- Pod exec-based diagnostics (Istio proxy-status) in initial sprints
- Classic Ingress resource enrichment (already covered in `k8s_ingress.go`, out of scope)
- LLM evaluation framework (future work, proposed as follow-up)

---

## 3. Design Principle: Conditional Enrichment

**Every field added to a tool response must answer: "Does this help the model identify a problem?"**

- If the answer is "only when misconfigured" → show it **only when misconfigured**
- If the answer is "always useful for context" → show it always but keep it compact
- Never add data that duplicates what's already visible
- Use `DiagnosticFinding` severity levels: `warning` or `critical` for problems, omit for happy paths

**Examples:**
- Route status `Accepted=True` → omit (expected state, no signal)
- Route status `Accepted=False, reason=NoMatchingListenerHostname` → emit warning finding
- Weights sum to 100 → omit weight analysis
- Weights sum to 80 → emit warning: "backendRef weights sum to 80, not 100 — 20% traffic will be dropped"
- Gateway listener `Ready=True` → omit
- Gateway listener `Ready=False, reason=InvalidCertificateRef` → emit critical finding

---

## 4. User Stories

### Sprint 1: Silent Failure Detection

**US-1.1: Route Acceptance Status in List View**
As an AI model diagnosing routing failures, I need to see which routes are NOT accepted in the list view so I can immediately identify routes that silently do nothing.

*Acceptance Criteria:*
- `list_httproutes` and `list_grpcroutes` show a `status` field per route
- Status field omitted when all parent conditions are `Accepted=True` (conditional enrichment)
- When any parent has `Accepted=False`: show `⚠ NOT_ACCEPTED(reason)` with the controller name
- When `ResolvedRefs=False`: show `⚠ UNRESOLVED_REFS(reason)`
- Multiple parents with mixed status: show per-parent status

*Technical Notes:*
- `get_httproute`/`get_grpcroute` already extract `status.parents[].conditions` — reuse that logic
- List view currently skips status entirely — add conditional status column to markdown table
- Affected file: `pkg/tools/gateway_api.go` (ListHTTPRoutesTool, ListGRPCRoutesTool)

---

**US-1.2: Gateway Listener Status Conditions**
As an AI model diagnosing why routes aren't working, I need to see which gateway listeners are not ready so I can identify listener-level failures.

*Acceptance Criteria:*
- `get_gateway` emits a warning finding per listener with `Ready=False` or `Accepted=False`
- Warning includes condition reason and message
- `list_gateways` adds per-listener status indicator only when a listener is NOT ready
- Detect and warn on: port conflicts, unsupported protocols, missing cert refs (from condition reasons)
- Happy-path listeners (all conditions True) produce no additional output

*Technical Notes:*
- `get_gateway` already shows listener name/port/protocol and attached route count
- Listener status conditions available at `status.listeners[].conditions`
- Affected file: `pkg/tools/gateway_api.go` (GetGatewayTool, ListGatewaysTool)

---

**US-1.3: Cross-Namespace Reference Validation from Route Context**
As an AI model diagnosing routing failures, I need routes that reference cross-namespace backends to warn me when the required ReferenceGrant is missing, so I don't have to manually cross-check grants.

*Acceptance Criteria:*
- `get_httproute` and `get_grpcroute` check each backendRef that targets a different namespace
- If no matching ReferenceGrant exists: emit warning finding with suggestion to create one
- If ReferenceGrant exists but doesn't cover the specific group/kind: emit warning with specifics
- `scan_gateway_misconfigs` already does this check — extract shared validation logic
- No output for same-namespace refs or when grants are correctly in place

*Technical Notes:*
- `scan_gateway_misconfigs` already has cross-namespace ref validation (check #4)
- `get_referencegrant` already does reverse cross-checking
- Refactor: extract `validateCrossNamespaceRef(route, backendRef) *DiagnosticFinding` as shared helper
- Affected files: `pkg/tools/gateway_api.go` (GetHTTPRouteTool, GetGRPCRouteTool)

---

### Sprint 2: Traffic Policy Visibility

**US-2.1: Traffic Splitting Weight Analysis**
As an AI model diagnosing partial traffic failures, I need to see weight distribution across backends and warnings when weights are misconfigured.

*Acceptance Criteria:*
- `list_httproutes` / `list_grpcroutes` already show `weight` in backend refs — no change to happy path
- When weights across a rule's backends don't sum to 100: emit warning finding
- When a weighted backend has 0 ready endpoints: emit warning "canary backend svc-v2 has weight 20% but 0 ready endpoints"
- Istio VirtualService: `get_istio_virtual_service` surfaces `route[].weight` per destination
- When Istio VS weights don't sum to 100: emit warning

*Technical Notes:*
- HTTPRoute/GRPCRoute weight extraction already exists in `extractBackendRefs()`
- Need to add summation check and zero-endpoint cross-ref
- Istio VirtualService handler needs weight extraction from `spec.http[].route[].weight`
- Affected files: `pkg/tools/gateway_api.go`, `pkg/tools/istio.go`

---

**US-2.2: Timeout and Retry Policy Surfacing**
As an AI model diagnosing intermittent failures or slow responses, I need to see timeout and retry configurations on routes so I can identify aggressive timeouts or missing retries.

*Acceptance Criteria:*
- `get_httproute` / `get_grpcroute`: show `timeouts.request` and `timeouts.backendRequest` per rule — only when explicitly set
- Emit info finding when timeout is set: "Rule 0: request timeout 5s, backend timeout 3s"
- Emit warning when `backendRequest` timeout > `request` timeout (misconfiguration)
- Istio VirtualService: surface `timeout` and `retries` (attempts, perTryTimeout, retryOn) per route
- Emit warning when retries are configured but `perTryTimeout * attempts > route timeout`
- kgateway: surface timeout settings from TrafficPolicy when attached

*Technical Notes:*
- HTTPRoute timeouts at `spec.rules[].timeouts` (Gateway API v1 field)
- Istio timeout/retries at `spec.http[].timeout` and `spec.http[].retries`
- Affected files: `pkg/tools/gateway_api.go`, `pkg/tools/istio.go`

---

### Sprint 3: Advanced Traffic Policies

**US-3.1: Rate Limit Policy Detection**
As an AI model diagnosing intermittent request failures (429s), I need to discover rate limiting policies affecting a service or route.

*Acceptance Criteria:*
- New tool: `check_rate_limit_policies` — input: service name + namespace (optional: route name)
- Discovers rate limits from:
  - kgateway `TrafficPolicy` with `rateLimit` field (already partially surfaced via ExtensionRef)
  - Istio `EnvoyFilter` with rate limit configuration
- Output: list of active rate limit policies with type (local/global), limits (requests/unit), and scope
- Conditional: If no rate limit policies found, return single info finding "No rate limit policies found"
- Enrich `get_httproute`: when ExtensionRef points to a TrafficPolicy with rateLimit, show limit params inline

*Technical Notes:*
- kgateway TrafficPolicy already detected via ExtensionRef in filters — need to resolve and extract `rateLimit` config
- EnvoyFilter rate limits: search for `envoy.filters.http.local_ratelimit` or `envoy.filters.http.ratelimit` in EnvoyFilter specs
- New file: `pkg/tools/rate_limiting.go`
- Affected files: `pkg/tools/gateway_api.go` (ExtensionRef enrichment), `pkg/tools/istio.go` (EnvoyFilter)

---

**US-3.2: TLS Configuration Validation (Reference-Level)**
As an AI model diagnosing connection resets and TLS errors, I need to see TLS configuration on gateways and detect reference-level misconfigurations.

*Acceptance Criteria:*
- `get_gateway`: for HTTPS/TLS listeners, show TLS mode (Terminate/Passthrough) and certificateRef names
- Emit warning when: HTTPS listener has no certificateRefs, certificateRef Secret doesn't exist in target namespace, TLS mode is Passthrough but certificateRefs are set (contradiction)
- Istio DestinationRule: surface `trafficPolicy.tls.mode` (ISTIO_MUTUAL, SIMPLE, DISABLE) in `get_istio_destination_rule`
- Istio PeerAuthentication: surface `mtls.mode` in detail view
- `BackendTLSPolicy` (Gateway API): list and show upstream TLS configuration when present
- No cert chain/expiry validation in this sprint (requires Secret read RBAC — defer to future opt-in)

*Technical Notes:*
- Gateway listener TLS at `spec.listeners[].tls` — mode, certificateRefs, options
- Secret existence check: attempt `client.CoreV1().Secrets(ns).Get(name)` — if 403 (RBAC), emit info "cannot verify cert Secret (no read access)" rather than warning
- Affected files: `pkg/tools/gateway_api.go` (GetGatewayTool), `pkg/tools/istio.go`

---

**US-3.3: Circuit Breaker Detection**
As an AI model diagnosing service-down symptoms that are actually circuit breakers, I need to see circuit breaker configurations and detect potential trips.

*Acceptance Criteria:*
- `get_istio_destination_rule`: surface `trafficPolicy.connectionPool` (TCP maxConnections, HTTP http1MaxPendingRequests, http2MaxRequests) and `trafficPolicy.outlierDetection` (consecutiveErrors, interval, baseEjectionTime, maxEjectionPercent)
- Conditional: only emit finding when circuit breaker / outlier detection is configured
- Emit warning when outlier detection has aggressive settings (consecutiveErrors < 3 or baseEjectionTime > 5m)
- kgateway TrafficPolicy: surface circuit breaker settings when present

*Technical Notes:*
- DestinationRule trafficPolicy at `spec.trafficPolicy.connectionPool` and `spec.trafficPolicy.outlierDetection`
- Can also be per-subset: `spec.subsets[].trafficPolicy`
- Affected files: `pkg/tools/istio.go`, `pkg/tools/kgateway.go`

---

### Sprint 4: Extended Provider Coverage & Deep Diagnostics

**US-4.1: Cilium Network Policy Tools**
As an AI model diagnosing L3/L4/L7 blocking in Cilium clusters, I need to list and inspect CiliumNetworkPolicies.

*Acceptance Criteria:*
- New tools: `list_cilium_network_policies`, `get_cilium_network_policy`
- Support both `CiliumNetworkPolicy` (namespaced) and `CiliumClusterWideNetworkPolicy`
- Surface: endpoint selectors, ingress/egress rules, L7 rules (HTTP path/method/header, gRPC)
- Conditional: emit warning when L7 rule restricts specific paths/methods (potential blocking)
- Cross-reference: given a service, show which Cilium policies select its pods (label matching)
- Handle gracefully when Cilium CRDs not present (tool not registered — existing discovery pattern)

*Technical Notes:*
- `provider_cilium.go` already exists with `list_cilium_policies` — extend with detail tool and L7 extraction
- CRD group: `cilium.io/v2`, resources: `ciliumnetworkpolicies`, `ciliumclusterwidenetworkpolicies`
- Discovery flag `HasCilium` already exists
- Affected file: `pkg/tools/provider_cilium.go`

---

**US-4.2: Waypoint Proxy Health Check (Ambient Mesh)**
As an AI model diagnosing Gamma routing failures in Istio ambient mesh, I need to verify waypoint proxies are deployed and healthy.

*Acceptance Criteria:*
- Enrich `scan_gateway_misconfigs`: when a route uses a Gamma Service parentRef, check:
  - Service/namespace has `istio.io/use-waypoint` label
  - If waypoint label present: verify waypoint Gateway exists and is Programmed
  - If waypoint pod(s) exist: check pod readiness
- Emit warning when: waypoint label set but waypoint gateway not found, waypoint gateway not Programmed, waypoint pods not ready
- No output when: no Gamma routes exist, or waypoint is healthy

*Technical Notes:*
- Waypoint detection: label `istio.io/use-waypoint` on namespace or service
- Waypoint gateway: GatewayClass `istio-waypoint`
- Affected files: `pkg/tools/gateway_api.go` (scan_gateway_misconfigs), potentially `pkg/tools/istio.go`

---

**US-4.3: Data Plane Health Diagnostics**
As an AI model diagnosing mesh networking failures, I need basic data plane health information without requiring pod exec.

*Acceptance Criteria:*
- New tool: `check_dataplane_health` — input: pod name + namespace
- Check (API-level only, no exec):
  - Sidecar container present and ready (istio-proxy, cilium-agent, linkerd-proxy)
  - Sidecar container restart count (warning if > 0)
  - Istio: proxy image version vs. control plane version (if detectable via labels)
  - Init container status (istio-init, linkerd-init)
- Conditional: if no mesh sidecar detected, emit info "No service mesh sidecar found on pod"
- Future: opt-in exec-based checks for xDS sync status (out of scope this sprint)

*Technical Notes:*
- All checks use standard Pod API — no special RBAC needed
- Istio proxy version from container image tag or `sidecar.istio.io/status` annotation
- New file: `pkg/tools/dataplane_health.go`

---

## 5. Event Correlation (Cross-Cutting Enhancement)

**Applies to: Sprint 1 onward, incrementally**

*Acceptance Criteria:*
- When emitting a warning/critical finding about a resource, fetch recent Events for that resource
- Include the most recent relevant Event message in the finding's `Detail` field
- Example: Route status `Accepted=False` → Detail includes Event "no matching listener hostname for parent gateway/my-gw"
- Limit to last 5 Events, last 1 hour, to avoid noise
- Only fetch Events when a problem is detected (conditional — never on happy path)

*Technical Notes:*
- `client.CoreV1().Events(ns).List()` with field selector `involvedObject.name=X,involvedObject.kind=Y`
- Keep event fetching as a helper: `fetchRecentEvents(resource) []string`
- Introduce incrementally: Sprint 1 for routes/gateways, Sprint 2 for Istio resources

---

## 6. Implementation Architecture

### Shared Helpers to Extract

| Helper | Purpose | Used By |
|--------|---------|---------|
| `validateCrossNamespaceRef()` | Check if backendRef needs ReferenceGrant and if one exists | get_httproute, get_grpcroute, scan_gateway_misconfigs |
| `checkWeightDistribution()` | Validate backendRef weights sum to 100, check endpoint health | list/get httproute, list/get grpcroute |
| `fetchRecentEvents()` | Get recent Events for a resource when problem detected | All tools emitting warnings |
| `checkListenerTLS()` | Validate TLS config on gateway listener | get_gateway, scan_gateway_misconfigs |
| `resolveExtensionRef()` | Fetch and extract config from ExtensionRef target | get_httproute (rate limits, traffic policies) |

### New Files

| File | Sprint | Tools |
|------|--------|-------|
| `pkg/tools/rate_limiting.go` | 3 | `check_rate_limit_policies` |
| `pkg/tools/dataplane_health.go` | 4 | `check_dataplane_health` |

### Modified Files

| File | Sprints | Changes |
|------|---------|---------|
| `pkg/tools/gateway_api.go` | 1, 2, 3, 4 | Route status in list, listener status, cross-ns validation, weights, timeouts, TLS, waypoint |
| `pkg/tools/istio.go` | 2, 3 | VS weights/timeouts/retries, DR circuit breakers/TLS, PeerAuth mTLS mode |
| `pkg/tools/kgateway.go` | 2, 3 | Timeout settings, circuit breaker settings |
| `pkg/tools/provider_cilium.go` | 4 | L7 policy extraction, detail tool |

### Tool Count Impact

| Sprint | New Tools | Enriched Tools | Running Total |
|--------|-----------|---------------|---------------|
| Current | — | — | 42 |
| Sprint 1 | 0 | 5 (list/get routes x2, get_gateway) | 42 |
| Sprint 2 | 0 | 4 (get routes x2, istio VS, kgateway) | 42 |
| Sprint 3 | 1 (check_rate_limit_policies) | 4 (get_gateway, istio DR, istio PA, kgateway) | 43 |
| Sprint 4 | 1 (check_dataplane_health) | 2 (provider_cilium detail, scan_gateway_misconfigs) | 44–45 |

---

## 7. Risk Register

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Gateway controllers populate status inconsistently | Medium | Low | Test with multiple controllers (Envoy Gateway, Istio, kgateway); handle missing fields gracefully |
| ReferenceGrant matching logic has edge cases (wildcard names, multiple grants) | Medium | Medium | Extract from existing scan_gateway_misconfigs logic; comprehensive test fixtures |
| TLS Secret read blocked by RBAC | High | Medium | Scope to reference-level checks only; emit info when RBAC prevents Secret read |
| Cilium CRD schema differences across versions | Medium | High | Use unstructured dynamic client; test with Cilium 1.14+ |
| Waypoint proxy API changes (ambient mesh evolving) | High | Medium | Feature-flag waypoint checks behind Istio version detection; accept maintenance burden |
| Data plane health requires exec for deep checks | High | High | Scope Sprint 4 to API-level checks only; exec-based as future opt-in |
| Enriched output increases token usage for LLM | Medium | Medium | Conditional enrichment principle — only emit on problems. Measure output size before/after |
| No existing test suite | High | High | Establish test patterns in Sprint 1 with fixture-based unit tests; mock K8s client |

---

## 8. Success Metrics

1. **Silent failure detection rate** — After Sprint 1, the model should correctly identify unaccepted routes and unready listeners in >90% of diagnostic scenarios (measured via manual test cases).
2. **Output efficiency** — Tool response size for happy-path resources should increase by <5% across all sprints (conditional enrichment discipline).
3. **Diagnostic coverage** — Gap analysis items 1-11 all have at least reference-level coverage by end of Sprint 4.
4. **Zero regression** — All existing tool outputs remain valid; enrichments are additive, never breaking.

---

## 9. Sprint Summary

| Sprint | Theme | Items | Effort | Risk |
|--------|-------|-------|--------|------|
| **1** | Silent failure detection | Route status (#10), Listener status (#9), Cross-ns refs (#11), Event correlation (start) | ~1 week | 🟢 Low |
| **2** | Traffic policy visibility | Weight analysis (#1), Timeouts/retries (#5) | ~1 week | 🟢 Low |
| **3** | Advanced traffic policies | Rate limiting (#2), TLS refs (#3), Circuit breaking (#4) | ~2 weeks | 🟡 Medium |
| **4** | Extended providers & deep diag | Cilium L7 (#6), Waypoint (#7), Data plane (#8) | ~2-3 weeks | 🔴 High |

---

## 10. Future Work (Out of Scope)

- **Unified `diagnose_route` tool** — Single entry point walking DNS→Policy→Route→Gateway→TLS→Backend. Depends on Sprints 1-3. Target: Sprint 5+.
- **Full TLS cert validation** — Requires elevated RBAC for Secret reads. Opt-in via Helm value.
- **LLM diagnostic evaluation framework** — Test harness validating model accuracy with enriched outputs.
- **Multi-cluster diagnostics** — ServiceExport/ServiceImport, Cilium ClusterMesh.
- **Exec-based data plane diagnostics** — istioctl proxy-status equivalent via pod exec.
