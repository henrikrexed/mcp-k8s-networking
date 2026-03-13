# Design Guidance Tools

These 4 tools generate provider-specific networking configurations with annotated YAML templates. Three are CRD-dependent; `suggest_remediation` is always available.

---

## design_gateway_api

Generate Gateway API configuration guidance with annotated YAML templates based on user intent.

**Availability:** When Gateway API CRDs detected

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `service_name` | string | Yes | Target service name |
| `namespace` | string | Yes | Target namespace |
| `port` | integer | Yes | Service port to expose |
| `intent` | string | No | What the user wants to achieve (e.g., "expose service X on port Y via HTTPS") |
| `hostname` | string | No | Hostname for the route (e.g., `app.example.com`) |
| `protocol` | string | No | Protocol: `HTTP`, `HTTPS`, or `GRPC` (default: `HTTP`) |
| `tls_secret` | string | No | Name of TLS secret for HTTPS (`namespace/name` format) |
| `gateway_name` | string | No | Existing Gateway to attach the route to |
| `gateway_namespace` | string | No | Namespace of the existing Gateway |

**Example use cases:**

- Generate Gateway + HTTPRoute YAML to expose a service publicly
- Create HTTPS routing with TLS termination
- Generate GRPCRoute configuration for gRPC services
- Produce ReferenceGrant YAML for cross-namespace references

---

## design_istio

Generate Istio configuration guidance with annotated YAML templates based on user intent.

**Availability:** When Istio CRDs detected

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | Yes | Target namespace |
| `intent` | string | No | What the user wants to achieve (e.g., "enable mTLS", "route 80% to v2") |
| `service_name` | string | No | Target service name |
| `mtls_mode` | string | No | mTLS mode: `STRICT`, `PERMISSIVE`, or `DISABLE` |
| `traffic_split` | string | No | Traffic split as `subset1:weight1,subset2:weight2` (e.g., `v1:80,v2:20`) |
| `allowed_sources` | string | No | Comma-separated list of allowed source namespaces or principals |

**Example use cases:**

- Generate PeerAuthentication YAML for STRICT mTLS
- Create VirtualService + DestinationRule for canary traffic splitting
- Produce AuthorizationPolicy YAML to restrict service access

---

## design_kgateway

Generate kgateway configuration guidance with annotated YAML templates based on user intent.

**Availability:** When kgateway CRDs detected

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | Yes | Target namespace |
| `intent` | string | No | What the user wants to achieve (e.g., "configure rate limiting", "add header manipulation") |
| `service_name` | string | No | Target service name |
| `route_name` | string | No | HTTPRoute to attach RouteOption to |
| `gateway_name` | string | No | Gateway to configure with GatewayParameters |
| `resource_type` | string | No | Specific resource to generate: `routeoption`, `virtualhostoption`, or `gatewayparameters` |

**Example use cases:**

- Generate RouteOption YAML for rate limiting on an HTTPRoute
- Create VirtualHostOption for gateway-level header manipulation
- Produce GatewayParameters for custom gateway configuration

---

## suggest_remediation

Suggest remediations for identified diagnostic issues with actionable YAML fixes.

**Availability:** Always available

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `issue_type` | string | Yes | Type of issue (see supported types below) |
| `resource_kind` | string | No | Kubernetes resource kind (e.g., `Service`, `HTTPRoute`) |
| `resource_name` | string | No | Name of the affected resource |
| `namespace` | string | No | Namespace of the affected resource |
| `additional_context` | string | No | Additional context about the issue |

**Supported issue types:**

| Issue Type | Description |
|------------|-------------|
| `missing_endpoints` | Service has no ready endpoints |
| `no_matching_pods` | Service selector matches zero pods |
| `network_policy_blocking` | NetworkPolicy is blocking expected traffic |
| `dns_failure` | DNS resolution failure |
| `mtls_conflict` | Conflicting mTLS configuration |
| `route_misconfigured` | HTTPRoute or VirtualService routing error |
| `missing_reference_grant` | Cross-namespace reference without ReferenceGrant |
| `gateway_listener_conflict` | Gateway listener port/protocol conflict |
| `sidecar_missing` | Sidecar proxy not injected |
| `weight_mismatch` | Traffic split weights don't sum to 100 |

**Example use cases:**

- Get YAML fix for a service with no matching pods
- Generate a ReferenceGrant to fix cross-namespace reference failures
- Get step-by-step remediation for mTLS configuration conflicts
