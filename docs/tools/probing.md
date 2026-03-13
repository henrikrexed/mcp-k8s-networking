# Active Probing Tools

These 3 tools are always available. They deploy ephemeral pods to actively test networking.

!!! note "Resource Controls"
    Probe pods run with restricted security context (runAsNonRoot, drop all capabilities, seccomp RuntimeDefault) and are automatically cleaned up after a 5-minute TTL. Concurrency is limited to `MAX_CONCURRENT_PROBES` (default: 5).

---

## probe_connectivity

Deploy an ephemeral pod to test TCP network connectivity between namespaces or services. When `target_port` is omitted and `target_host` is a K8s service, the port is auto-resolved from the Service spec; if the service exposes multiple ports, all are tested.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `target_host` | string | Yes | Target hostname or IP (e.g., `my-service.target-ns.svc.cluster.local`) |
| `target_port` | integer | No | Target port. When omitted, auto-resolved from the K8s Service; if the service has multiple ports, all are tested |
| `source_namespace` | string | No | Namespace to deploy the probe pod in (source of connectivity test) |
| `timeout_seconds` | integer | No | Probe timeout in seconds (default: 10, max: 30) |

**Example use cases:**

- Test if namespace A can reach a service in namespace B (NetworkPolicy validation)
- Verify TCP connectivity to an external endpoint from within the cluster
- Diagnose "connection refused" vs "connection timed out" failures

---

## probe_dns

Deploy an ephemeral pod to test DNS resolution from within the cluster.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `hostname` | string | Yes | Hostname to resolve (e.g., `my-service.default.svc.cluster.local`) |
| `source_namespace` | string | No | Namespace to deploy the probe pod in |
| `record_type` | string | No | DNS record type to query: `A`, `AAAA`, `SRV`, `CNAME` (default: `A`) |

**Example use cases:**

- Verify DNS resolution works from a specific namespace (useful with DNS policies)
- Test SRV record resolution for headless services
- Compare DNS behavior between namespaces

---

## probe_http

Deploy an ephemeral pod to perform HTTP/HTTPS requests against services from within the cluster. When the URL omits a port and the hostname is a K8s service, the port is auto-resolved from the Service spec; if the service exposes multiple ports, all are tested.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `url` | string | Yes | Target URL (e.g., `http://my-service.default.svc.cluster.local/health`). When the port is omitted, it is auto-resolved from the K8s Service |
| `method` | string | No | HTTP method: `GET`, `POST`, `HEAD` (default: `GET`) |
| `headers` | string | No | Additional headers as `Key: Value` pairs separated by semicolons |
| `source_namespace` | string | No | Namespace to deploy the probe pod in |
| `timeout_seconds` | integer | No | Request timeout in seconds (default: 10, max: 30) |

**Example use cases:**

- Test HTTP health endpoints from within the mesh
- Verify mTLS is working by making requests between services
- Send requests with custom headers to test routing rules
