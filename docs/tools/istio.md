# Istio Tools

These 7 tools are available when Istio CRDs (`networking.istio.io`, `security.istio.io`) are detected in the cluster. The `design_istio` tool is documented on the [Design Guidance](design-guidance.md) page.

---

## list_istio_resources

List Istio resources (VirtualService, DestinationRule, AuthorizationPolicy, PeerAuthentication) with key summary fields.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `kind` | string | Yes | Resource kind: `VirtualService`, `DestinationRule`, `AuthorizationPolicy`, `PeerAuthentication` |
| `namespace` | string | No | Kubernetes namespace (empty for all namespaces) |

**Example use cases:**

- List all VirtualServices to understand traffic routing
- Find AuthorizationPolicies across namespaces
- Discover PeerAuthentication policies affecting mTLS mode

---

## get_istio_resource

Get full Istio resource detail: spec, status, and validation messages.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `kind` | string | Yes | Resource kind: `VirtualService`, `DestinationRule`, `AuthorizationPolicy`, `PeerAuthentication` |
| `name` | string | Yes | Resource name |
| `namespace` | string | Yes | Kubernetes namespace |

**Example use cases:**

- Inspect VirtualService routing rules and match conditions
- Review DestinationRule subsets and traffic policies
- Check AuthorizationPolicy allow/deny rules

---

## check_sidecar_injection

Check Istio sidecar injection status for all deployments in a namespace: namespace label, annotations, actual sidecar presence.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | Yes | Kubernetes namespace to check |

**Example use cases:**

- Find deployments where sidecar injection is disabled
- Verify the `istio-injection=enabled` namespace label is set
- Detect mismatches between injection annotation and actual sidecar presence

---

## check_istio_mtls

Check Istio mTLS configuration: global/namespace mTLS mode, PeerAuthentication policies, and DestinationRule TLS settings.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | No | Kubernetes namespace to check (empty for cluster-wide) |

**Example use cases:**

- Determine the effective mTLS mode for a namespace
- Find conflicting PeerAuthentication and DestinationRule TLS settings
- Verify STRICT mTLS is enforced cluster-wide

---

## validate_istio_config

Validate Istio configuration: resource relationships, port naming conventions, subset references, and warning/error status.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | No | Kubernetes namespace to validate (empty for all namespaces) |

**Example use cases:**

- Find VirtualServices referencing non-existent DestinationRule subsets
- Detect port naming convention violations (e.g., missing `http-` prefix)
- Run pre-deployment validation of Istio configuration

---

## analyze_istio_authpolicy

Analyze an Istio AuthorizationPolicy: rules, action, conditions, and potentially affected workloads.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | Yes | AuthorizationPolicy name |
| `namespace` | string | Yes | Kubernetes namespace |

**Example use cases:**

- Understand which workloads are affected by a DENY policy
- Debug why requests are being rejected (RBAC denied)
- Review rule conditions and source principals

---

## analyze_istio_routing

Analyze Istio VirtualService routing: destinations, traffic weights, timeouts, retries, and potential issues.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `virtualservice` | string | Yes | VirtualService name |
| `namespace` | string | Yes | Kubernetes namespace |

**Example use cases:**

- Debug traffic not reaching the expected destination
- Verify canary traffic split weights sum to 100
- Check timeout and retry configuration for correctness
- Find shadowed routing rules that never match
