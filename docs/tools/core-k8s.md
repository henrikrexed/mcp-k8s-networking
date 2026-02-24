# Core Kubernetes Tools

These tools are always available regardless of installed CRDs.

## list_services

List Kubernetes services with type, clusterIP, ports, and selector.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | No | Namespace (empty for all) |

## get_service

Get detailed service info including endpoints and matching pod status.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | Yes | Service name |
| `namespace` | string | Yes | Namespace |

## list_endpoints

List endpoints for services in a namespace.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | No | Namespace (empty for all) |

## list_networkpolicies

List NetworkPolicies with pod selectors and rule counts.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | No | Namespace (empty for all) |

## get_networkpolicy

Get detailed NetworkPolicy with ingress/egress rules.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | Yes | Policy name |
| `namespace` | string | Yes | Namespace |

## check_dns_resolution

Check DNS resolution health and kube-dns service status.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `hostname` | string | Yes | Hostname to resolve |
| `namespace` | string | No | Context namespace |

## check_kubeproxy_health

Check kube-proxy health across nodes.

**Parameters:** None required.

## list_ingresses

List Ingress resources with hosts, paths, and backends.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | No | Namespace (empty for all) |

## get_ingress

Get detailed Ingress resource with rules and TLS configuration.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | Yes | Ingress name |
| `namespace` | string | Yes | Namespace |
