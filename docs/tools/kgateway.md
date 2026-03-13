# kgateway Tools

These 3 tools are available when kgateway CRDs (`kgateway.dev`) are detected in the cluster. The `design_kgateway` tool is documented on the [Design Guidance](design-guidance.md) page.

---

## list_kgateway_resources

List kgateway resources (GatewayParameters, RouteOption, VirtualHostOption) with key summary fields.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `kind` | string | Yes | Resource kind: `GatewayParameters`, `RouteOption`, `VirtualHostOption` |
| `namespace` | string | No | Kubernetes namespace (empty for all namespaces) |

**Example use cases:**

- List all RouteOptions to see what policies are applied to routes
- Find VirtualHostOptions affecting gateway-level behavior
- Discover GatewayParameters resources for gateway configuration

---

## validate_kgateway_resource

Validate kgateway resources: upstream references, route option conflicts, GatewayParameters references, and status conditions.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `kind` | string | Yes | Resource kind: `GatewayParameters`, `RouteOption`, `VirtualHostOption` |
| `name` | string | Yes | Resource name |
| `namespace` | string | Yes | Kubernetes namespace |

**Example use cases:**

- Verify RouteOption references a valid HTTPRoute
- Check for conflicting VirtualHostOptions on the same Gateway
- Validate upstream references in GatewayParameters

---

## check_kgateway_health

Check kgateway installation health: control plane pod status, resource translation status, and data plane proxy health for kgateway-managed Gateways.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | No | Namespace where kgateway is installed (default: `kgateway-system`) |

**Example use cases:**

- Verify the kgateway control plane is healthy
- Check if resource translation is up to date
- Monitor data plane proxy pod status across Gateways
