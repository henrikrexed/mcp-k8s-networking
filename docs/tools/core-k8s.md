# Core Kubernetes Tools

These 11 tools are always available regardless of installed CRDs.

---

## list_services

List Kubernetes services with type, clusterIP, ports, and selector.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | No | Kubernetes namespace (empty for all namespaces) |

**Example use cases:**

- Discover all services in a namespace to understand the application topology
- Find services with no endpoints (broken selectors)
- Identify LoadBalancer vs ClusterIP services

---

## get_service

Get detailed service info including endpoints and matching pod status.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | Yes | Service name |
| `namespace` | string | Yes | Kubernetes namespace |

**Example use cases:**

- Check why a service has no ready endpoints
- Verify the service selector matches running pods
- Inspect port mappings and endpoint IPs

---

## list_endpoints

List endpoints with ready/not-ready address counts.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | No | Kubernetes namespace (empty for all namespaces) |

**Example use cases:**

- Find services with zero ready endpoints
- Compare endpoint counts across namespaces
- Identify services with not-ready backends

---

## list_networkpolicies

List NetworkPolicies with podSelector and rule counts.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | No | Kubernetes namespace (empty for all namespaces) |

**Example use cases:**

- Audit network isolation posture across namespaces
- Find pods that are not covered by any NetworkPolicy
- Review ingress/egress rule counts

---

## get_networkpolicy

Get full NetworkPolicy with ingress/egress rule details.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | Yes | NetworkPolicy name |
| `namespace` | string | Yes | Kubernetes namespace |

**Example use cases:**

- Debug why traffic between two pods is blocked
- Inspect allowed ingress sources and egress destinations
- Verify port-level rules match expected application ports

---

## check_dns_resolution

DNS lookup for a hostname plus kube-dns service health check.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `hostname` | string | Yes | Hostname to resolve (e.g., `my-service.default.svc.cluster.local`) |
| `namespace` | string | No | Namespace context for short names |

**Example use cases:**

- Verify a service is resolvable by DNS within the cluster
- Diagnose DNS resolution failures
- Check kube-dns/CoreDNS pod health

---

## check_kube_proxy_health

Check kube-proxy DaemonSet health: pod status across nodes, configuration mode (iptables/IPVS), unhealthy pods.

**Parameters:** None.

**Example use cases:**

- Verify kube-proxy is running on all nodes
- Check if kube-proxy is using iptables or IPVS mode
- Find nodes where kube-proxy is crashlooping

---

## list_ingresses

List Ingress resources with hosts, paths, backends, and TLS configuration.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | No | Kubernetes namespace (empty for all namespaces) |

**Example use cases:**

- Discover all Ingress rules across the cluster
- Find Ingresses without TLS configured
- Audit host-based routing rules

---

## get_ingress

Get full Ingress spec with rules, TLS settings, status, and backend validation.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | Yes | Ingress name |
| `namespace` | string | Yes | Kubernetes namespace |

**Example use cases:**

- Debug why an Ingress backend returns 503
- Verify TLS certificate secret references
- Inspect path-based routing rules and backend service targets

---

## check_dataplane_health

Check data plane health for all pods in a namespace by inspecting mesh sidecar containers (istio-proxy, cilium-agent, linkerd-proxy). Reports sidecar readiness, restart counts, init container failures, and Istio version skew without requiring exec access.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | Yes | Namespace to inspect |

**Example use cases:**

- Find pods with crashlooping sidecar proxies
- Detect Istio version skew across the data plane
- Identify pods where sidecar injection failed (init container errors)
- Check if sidecar containers are ready and accepting traffic

---

## check_rate_limit_policies

Discover rate limiting policies (kgateway TrafficPolicy, Istio EnvoyFilter) affecting a service or route.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `service` | string | Yes | Service name to check for rate limit policies |
| `namespace` | string | Yes | Kubernetes namespace |
| `route` | string | No | Optional route name to scope the search |

**Example use cases:**

- Find all rate limiting policies affecting a specific service
- Debug 429 responses by discovering which rate limits apply
- Verify rate limiting configuration across kgateway TrafficPolicy and Istio EnvoyFilter resources
