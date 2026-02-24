# Log Collection Tools

Always available.

## get_proxy_logs

Retrieve logs from proxy sidecar containers (istio-proxy, envoy, linkerd-proxy).

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `pod` | string | Yes | Pod name |
| `namespace` | string | Yes | Namespace |
| `tail` | integer | No | Number of lines (default: 100) |
| `since` | string | No | Time duration (e.g., "5m", "1h") |

## get_gateway_logs

Retrieve logs from gateway controller pods.

## get_infra_logs

Retrieve logs from infrastructure components.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `component` | string | Yes | Component: kube-proxy, coredns, or cni |
| `tail` | integer | No | Number of lines |

## analyze_log_errors

Analyze pod logs and extract categorized error patterns.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `pod` | string | Yes | Pod name |
| `namespace` | string | Yes | Namespace |
| `lines` | integer | No | Lines to scan (default: 500) |

Error categories: connection_errors, tls_errors, rate_limiting, misconfig, rbac_denied, upstream_issues, timeout, other_errors.
