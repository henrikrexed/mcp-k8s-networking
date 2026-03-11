# Epics & Stories: k8s-networking MCP v2

**Date:** 2026-03-10
**Source PRD:** prd.md

---

## Epic 1: Silent Failure Detection (Sprint 1)

**Goal:** Surface route acceptance status, gateway listener readiness, and cross-namespace reference violations so the model detects problems that currently produce zero signal.

**Effort:** ~1 week | **Risk:** Low

### Story 1.1: Route Status Enrichment in List Views

**Summary:** Add conditional route acceptance status to `list_httproutes` and `list_grpcroutes` summary output.

**Scope:**
- In `ListHTTPRoutesTool.Run()` and `ListGRPCRoutesTool.Run()`: after building each route's finding, extract `status.parents[].conditions`
- If all parents have `Accepted=True`: omit status (happy path, no change)
- If any parent has `Accepted=False`: append `⚠ NOT_ACCEPTED(reason)` to summary string
- If any parent has `ResolvedRefs=False`: append `⚠ UNRESOLVED_REFS(reason)`
- For multiple parents with mixed status: show per-parent status

**Files:** `pkg/tools/gateway_api.go` (ListHTTPRoutesTool ~line 570, ListGRPCRoutesTool ~line 930)

**Acceptance Criteria:**
- [ ] Routes with all-accepted parents show no status suffix
- [ ] Routes with rejected parents show `⚠ NOT_ACCEPTED(reason)` with controller name
- [ ] Routes with unresolved refs show `⚠ UNRESOLVED_REFS(reason)`
- [ ] Mixed multi-parent status shown per-parent
- [ ] `go build` passes

---

### Story 1.2: Gateway Listener Status Enrichment

**Summary:** Add conditional listener status warnings to `get_gateway` and `list_gateways`.

**Scope:**
- In `GetGatewayTool.Run()`: for each listener, check status conditions. If `Ready=False` or `Accepted=False`, emit a warning finding with reason and message.
- In `ListGatewaysTool.Run()`: append per-listener status indicator to summary only when a listener is NOT ready.
- Happy-path listeners produce no additional output.

**Files:** `pkg/tools/gateway_api.go` (GetGatewayTool ~line 397, ListGatewaysTool ~line 298)

**Acceptance Criteria:**
- [ ] `get_gateway` emits warning finding per unhealthy listener with reason/message
- [ ] `get_gateway` emits no extra findings for healthy listeners
- [ ] `list_gateways` appends listener status only when unhealthy
- [ ] Detects port conflicts, unsupported protocols, missing cert refs (from condition reasons)
- [ ] `go build` passes

---

### Story 1.3: Cross-Namespace Reference Validation Helper

**Summary:** Extract shared `validateCrossNamespaceRef()` helper from `scan_gateway_misconfigs` and wire it into `get_httproute` and `get_grpcroute`.

**Scope:**
- Extract `validateCrossNamespaceRef(routeNs, routeKind, backendNs, backendName string, refGrants []refGrantEntry, routeRef *types.ResourceRef) *types.DiagnosticFinding`
- Refactor `scan_gateway_misconfigs` to use the new helper (behavior unchanged)
- In `GetHTTPRouteTool.Run()` and `GetGRPCRouteTool.Run()`: for each backendRef targeting a different namespace, call the helper and append any returned finding
- Fetch ReferenceGrants list once per tool call

**Files:** `pkg/tools/gateway_api.go` (GetHTTPRouteTool ~line 682, GetGRPCRouteTool ~line 1037, ScanGatewayMisconfigsTool ~line 1528)

**Acceptance Criteria:**
- [ ] Shared helper extracted and used by all three tools
- [ ] Same-namespace refs produce no output
- [ ] Cross-namespace refs with valid ReferenceGrant produce no output
- [ ] Cross-namespace refs without ReferenceGrant emit warning with suggestion
- [ ] `scan_gateway_misconfigs` behavior unchanged (refactor only)
- [ ] `go build` passes

---

### Story 1.4: Event Correlation Helper (Foundation)

**Summary:** Implement `fetchRecentEvents()` helper and wire it into route/gateway warning findings.

**Scope:**
- Implement `fetchRecentEvents(ctx, clientset, ns, kind, name) []string`
- Filters: last 1 hour, max 5 events, matching involvedObject
- When a warning/critical finding is created for a route or gateway, call fetchRecentEvents and append event messages to the finding's Detail field
- If event fetch fails (RBAC), silently continue without events

**Files:** `pkg/tools/gateway_api.go`

**Acceptance Criteria:**
- [ ] Helper fetches events filtered by resource and time
- [ ] Events appended to Detail field of warning findings for routes and gateways
- [ ] No events fetched on happy paths (conditional)
- [ ] Graceful degradation if Events API access denied
- [ ] `go build` passes

---

## Epic 2: Traffic Policy Visibility (Sprint 2)

**Goal:** Surface weight distribution, timeout, and retry configurations so the model can diagnose intermittent and timeout-related failures.

**Effort:** ~1 week | **Risk:** Low

### Story 2.1: Weight Distribution Analysis

**Summary:** Validate that backendRef weights sum to 100 and cross-reference with endpoint health.

**Files:** `pkg/tools/gateway_api.go`, `pkg/tools/istio.go`

**Acceptance Criteria:**
- [ ] Weights summing to 100: no additional output
- [ ] Weights not summing to 100: warning with specific sum and shortfall
- [ ] Weighted backend with 0 ready endpoints: warning
- [ ] Istio VirtualService weights validated similarly

---

### Story 2.2: Timeout and Retry Policy Surfacing

**Summary:** Show timeout and retry configurations on routes, warn on misconfigurations.

**Files:** `pkg/tools/gateway_api.go`, `pkg/tools/istio.go`, `pkg/tools/kgateway.go`

**Acceptance Criteria:**
- [ ] HTTPRoute/GRPCRoute timeouts shown when explicitly set
- [ ] Warning when backendRequest timeout > request timeout
- [ ] Istio VS timeout and retries surfaced
- [ ] Warning when `perTryTimeout * attempts > route timeout`

---

## Epic 3: Advanced Traffic Policies (Sprint 3)

**Goal:** Expose rate limiting, TLS configuration, and circuit breaker settings.

**Effort:** ~2 weeks | **Risk:** Medium

### Story 3.1: Rate Limit Policy Detection Tool

**Summary:** New `check_rate_limit_policies` tool discovering rate limits from kgateway TrafficPolicy and Istio EnvoyFilter.

**Files:** NEW `pkg/tools/rate_limiting.go`, `pkg/tools/gateway_api.go`

**Acceptance Criteria:**
- [ ] Discovers rate limits from TrafficPolicy and EnvoyFilter
- [ ] Shows type (local/global), limits, scope
- [ ] Returns info finding when no policies found

---

### Story 3.2: TLS Configuration Validation

**Summary:** Validate TLS config on gateway listeners and surface Istio TLS modes.

**Files:** `pkg/tools/gateway_api.go`, `pkg/tools/istio.go`

**Acceptance Criteria:**
- [ ] HTTPS listeners show TLS mode and cert ref names
- [ ] Warning on missing cert refs, non-existent secrets, contradictory config
- [ ] Istio DestinationRule TLS mode surfaced
- [ ] Graceful handling of Secret read RBAC denial

---

### Story 3.3: Circuit Breaker Detection

**Summary:** Surface circuit breaker and outlier detection settings from Istio DestinationRule and kgateway TrafficPolicy.

**Files:** `pkg/tools/istio.go`, `pkg/tools/kgateway.go`

**Acceptance Criteria:**
- [ ] Connection pool and outlier detection settings surfaced when configured
- [ ] Warning on aggressive outlier detection settings
- [ ] No output when circuit breakers not configured

---

## Epic 4: Extended Providers & Deep Diagnostics (Sprint 4)

**Goal:** Cover Cilium L7 policies, ambient mesh waypoints, and data plane health.

**Effort:** ~2-3 weeks | **Risk:** High

### Story 4.1: Cilium Network Policy L7 Enrichment

**Summary:** Extend existing Cilium tools with L7 rule extraction and detail view.

**Files:** `pkg/tools/provider_cilium.go`

**Acceptance Criteria:**
- [ ] L7 rules (HTTP path/method, gRPC) surfaced in detail view
- [ ] Warning when L7 rule restricts specific paths/methods
- [ ] Cross-reference service pods with policy label selectors

---

### Story 4.2: Waypoint Proxy Health Check

**Summary:** Detect and validate waypoint proxy health for Gamma mesh routing.

**Files:** `pkg/tools/gateway_api.go`, potentially `pkg/tools/istio.go`

**Acceptance Criteria:**
- [ ] Detect `istio.io/use-waypoint` label on namespace/service
- [ ] Verify waypoint Gateway exists and is Programmed
- [ ] Warning on missing/unhealthy waypoint proxies

---

### Story 4.3: Data Plane Health Tool

**Summary:** New `check_dataplane_health` tool for API-level mesh sidecar diagnostics.

**Files:** NEW `pkg/tools/dataplane_health.go`

**Acceptance Criteria:**
- [ ] Detects sidecar presence (istio-proxy, cilium-agent, linkerd-proxy)
- [ ] Warning on sidecar not ready or high restart count
- [ ] Version skew detection for Istio proxy
- [ ] Info finding when no sidecar detected
