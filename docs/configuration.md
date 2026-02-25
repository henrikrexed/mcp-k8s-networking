# Configuration Reference

All configuration is via environment variables.

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `CLUSTER_NAME` | string | **(required)** | Cluster identifier included in all responses |
| `PORT` | int | `8080` | MCP server listen port (health on PORT+1) |
| `LOG_LEVEL` | string | `info` | Log level: debug, info, warn, error |
| `NAMESPACE` | string | *(empty)* | Default namespace context (empty = all) |
| `CACHE_TTL` | duration | `30s` | Cache duration for resource lookups |
| `TOOL_TIMEOUT` | duration | `10s` | Per-tool execution timeout |
| `PROBE_NAMESPACE` | string | `mcp-diagnostics` | Namespace for ephemeral probe pods |
| `PROBE_IMAGE` | string | `ghcr.io/mcp-k8s-networking/probe:latest` | Container image for probe pods |
| `MAX_CONCURRENT_PROBES` | int | `5` | Max concurrent probe pods (1-20) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | string | *(empty)* | OTLP gRPC endpoint for telemetry (empty = disabled) |
| `OTEL_EXPORTER_OTLP_INSECURE` | bool | `true` | Use insecure gRPC (no TLS) for OTLP export |
| `OTEL_SERVICE_NAME` | string | `mcp-k8s-networking` | Service name in OTel resource attributes |

## Helm Values

When deploying via Helm, these are configured through `values.yaml`:

```yaml
config:
  clusterName: "my-cluster"
  port: 8080
  logLevel: info
  cacheTTL: "30s"
  toolTimeout: "10s"

probe:
  namespace: mcp-diagnostics
  image: ghcr.io/mcp-k8s-networking/probe:latest
  maxConcurrent: 5

otel:
  enabled: false
  endpoint: "otel-collector.observability.svc.cluster.local:4317"
  insecure: true
  serviceName: ""  # defaults to chart fullname
```

See [Observability](observability.md) for full details on OTel integration.

## RBAC Permissions

The server requires a ClusterRole with read access to networking resources and create/delete access for ephemeral probe pods. See `deploy/helm/mcp-k8s-networking/templates/clusterrole.yaml` for the full RBAC specification.
