# Active Probing Tools

Always available. These tools deploy ephemeral pods to actively test networking.

!!! note "Resource Controls"
    Probe pods run with restricted security context (runAsNonRoot, drop all capabilities, seccomp RuntimeDefault) and are automatically cleaned up after a 5-minute TTL.

## probe_connectivity

Test TCP connectivity between namespaces or services.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `target_host` | string | Yes | Target hostname or IP |
| `target_port` | integer | Yes | Target port |
| `source_namespace` | string | No | Source namespace for probe pod |
| `timeout_seconds` | integer | No | Timeout (default: 10, max: 30) |

## probe_dns

Test DNS resolution from within the cluster.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `hostname` | string | Yes | Hostname to resolve |
| `source_namespace` | string | No | Source namespace for probe pod |
| `record_type` | string | No | DNS record type (default: A) |

## probe_http

Perform HTTP/HTTPS requests against services from within the cluster.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `url` | string | Yes | Target URL |
| `method` | string | No | HTTP method (default: GET) |
| `headers` | string | No | Headers as "Key: Value" separated by semicolons |
| `source_namespace` | string | No | Source namespace for probe pod |
| `timeout_seconds` | integer | No | Timeout (default: 10, max: 30) |
