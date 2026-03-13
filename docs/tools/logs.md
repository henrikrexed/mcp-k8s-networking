# Log Collection Tools

These 4 tools are always available. They retrieve and analyze logs from networking components.

---

## get_proxy_logs

Get logs from Envoy/proxy sidecars (auto-detects istio-proxy, envoy, linkerd-proxy containers).

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `pod` | string | Yes | Pod name |
| `namespace` | string | Yes | Kubernetes namespace |
| `container` | string | No | Container name (auto-detects proxy container if not specified) |
| `tail` | integer | No | Number of lines from the end (default: 100) |
| `since` | string | No | Duration to look back (e.g., `5m`, `1h`) |

**Example use cases:**

- Inspect Envoy access logs for 5xx errors
- Check istio-proxy startup logs after sidecar injection
- Review linkerd-proxy connection errors

---

## get_gateway_logs

Get logs from Gateway controller pods and Gateway API provider pods.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `gateway_name` | string | No | Gateway resource name (discovers controller pods by labels) |
| `namespace` | string | No | Namespace to search in (default: all common namespaces) |
| `tail` | integer | No | Number of lines from the end (default: 100) |
| `since` | string | No | Duration to look back (e.g., `5m`, `1h`) |

**Example use cases:**

- Debug why a Gateway is not accepting routes
- Check controller reconciliation errors
- Review provider-specific gateway pod logs

---

## get_infra_logs

Get logs from kube-proxy, CoreDNS, or CNI pods.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `component` | string | Yes | Infrastructure component: `kube-proxy`, `coredns`, or `cni` |
| `namespace` | string | No | Namespace override (default: `kube-system`) |
| `tail` | integer | No | Number of lines from the end (default: 100) |
| `since` | string | No | Duration to look back (e.g., `5m`, `1h`) |

**Example use cases:**

- Check CoreDNS logs for NXDOMAIN or timeout errors
- Review kube-proxy iptables sync errors
- Inspect CNI plugin logs for pod networking failures

---

## analyze_log_errors

Read logs and extract error/warning lines related to misconfig, rate limiting, connection issues, TLS errors.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `pod` | string | Yes | Pod name |
| `namespace` | string | Yes | Kubernetes namespace |
| `container` | string | No | Container name (optional, uses first container) |
| `tail` | integer | No | Number of lines to analyze (default: 500) |
| `since` | string | No | Duration to look back (e.g., `5m`, `1h`) |

**Error categories detected:** `connection_errors`, `tls_errors`, `rate_limiting`, `misconfig`, `rbac_denied`, `upstream_issues`, `timeout`, `other_errors`.

**Example use cases:**

- Quickly categorize errors in a misbehaving pod
- Find TLS handshake failures in proxy logs
- Detect rate limiting or RBAC denial patterns
