# Gateway API Tools

These 10 tools are available when Gateway API CRDs (`gateway.networking.k8s.io`) are detected in the cluster. The `design_gateway_api` tool is documented on the [Design Guidance](design-guidance.md) page.

---

## list_gateways

List Gateway API gateways with listeners, status conditions, and attached route count.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | No | Kubernetes namespace (empty for all namespaces) |

**Example use cases:**

- Discover all Gateways across the cluster
- Find Gateways with unhealthy listeners
- Check attached route counts per listener

---

## get_gateway

Get full Gateway detail: listeners, addresses, conditions, and attached routes.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | Yes | Gateway name |
| `namespace` | string | Yes | Kubernetes namespace |

**Example use cases:**

- Debug why a Gateway listener is not ready
- Inspect listener protocol, port, and TLS configuration
- Check Gateway addresses and status conditions

---

## list_httproutes

List HTTPRoutes with parent refs, backend refs, and rule count.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | No | Kubernetes namespace (empty for all namespaces) |

**Example use cases:**

- Find all HTTPRoutes attached to a specific Gateway
- Discover routes with NOT_ACCEPTED status
- Audit routing rules across namespaces

---

## get_httproute

Get full HTTPRoute: rules, matches, filters, backend refs with health.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | Yes | HTTPRoute name |
| `namespace` | string | Yes | Kubernetes namespace |

**Example use cases:**

- Debug path/header matching rules
- Verify backend service references resolve correctly
- Check traffic weight distribution across backends
- Inspect timeout and retry configuration

---

## list_grpcroutes

List GRPCRoutes with parent refs, backend refs, and rule counts.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | No | Kubernetes namespace (empty for all namespaces) |

**Example use cases:**

- Discover gRPC routing across the cluster
- Find GRPCRoutes with unresolved backend references

---

## get_grpcroute

Get full GRPCRoute: method matching rules, backend refs with health, and status conditions.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | Yes | GRPCRoute name |
| `namespace` | string | Yes | Kubernetes namespace |

**Example use cases:**

- Inspect gRPC service/method matching rules
- Verify backend health for gRPC traffic
- Check GAMMA (Gateway API for Mesh) routing with Service parentRefs

---

## list_referencegrants

List ReferenceGrants with from/to resource specifications for cross-namespace reference validation.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | No | Kubernetes namespace (empty for all namespaces) |

**Example use cases:**

- Audit cross-namespace reference permissions
- Find which namespaces are allowed to reference secrets or services

---

## get_referencegrant

Get full ReferenceGrant spec: allowed from-namespaces, from-kinds, to-kinds, to-names, and cross-namespace validation.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | Yes | ReferenceGrant name |
| `namespace` | string | Yes | Kubernetes namespace |

**Example use cases:**

- Debug cross-namespace route-to-service reference failures
- Verify TLS secret cross-namespace access grants

---

## scan_gateway_misconfigs

Scan for Gateway API misconfigurations: missing backends, orphaned routes, missing ReferenceGrants, listener conflicts.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | No | Kubernetes namespace to scan (empty for all) |

**Example use cases:**

- Run a comprehensive health check on all Gateway API resources
- Find orphaned routes not attached to any Gateway
- Detect missing ReferenceGrants for cross-namespace references
- Identify listener port/protocol conflicts

---

## check_gateway_conformance

Validate Gateway API resources (Gateway, HTTPRoute, GRPCRoute) against the specification and report non-conformant fields.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `kind` | string | Yes | Resource kind: `Gateway`, `HTTPRoute`, `GRPCRoute` |
| `name` | string | No | Resource name (validates all of the kind if omitted) |
| `namespace` | string | No | Kubernetes namespace |

**Example use cases:**

- Validate that HTTPRoutes conform to the Gateway API spec
- Check for deprecated or invalid fields in Gateway resources
- Run conformance checks before promoting to production
