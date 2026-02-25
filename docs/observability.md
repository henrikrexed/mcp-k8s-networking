# Observability

mcp-k8s-networking has built-in OpenTelemetry instrumentation that exports all three OTel signals — **traces**, **metrics**, and **logs** — via OTLP gRPC. The instrumentation follows the [OTel GenAI semantic conventions](https://opentelemetry.io/docs/specs/semconv/gen-ai/) and MCP-specific attributes.

## Enabling OTel

### Via Helm

```yaml
otel:
  enabled: true
  endpoint: "otel-collector.observability.svc.cluster.local:4317"
  insecure: true
  serviceName: ""  # defaults to chart fullname
```

### Via Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | *(empty = disabled)* | OTLP gRPC endpoint (e.g., `otel-collector:4317`) |
| `OTEL_EXPORTER_OTLP_INSECURE` | `true` | Skip TLS for gRPC connection |
| `OTEL_SERVICE_NAME` | `mcp-k8s-networking` | Service name in resource attributes |

When `OTEL_EXPORTER_OTLP_ENDPOINT` is not set, all telemetry signals use noop providers — zero overhead.

## Traces

Every MCP tool call produces a span following GenAI + MCP semantic conventions.

### Span Format

- **Span name**: `execute_tool {tool_name}` (e.g., `execute_tool list_services`)
- **Span kind**: `SERVER`

### Span Attributes

| Attribute | Description | Example |
|-----------|-------------|---------|
| `gen_ai.operation.name` | Always `execute_tool` | `execute_tool` |
| `gen_ai.tool.name` | The MCP tool being called | `list_services` |
| `gen_ai.tool.call.arguments` | Sanitized input arguments (secrets redacted) | `{"namespace":"default"}` |
| `gen_ai.tool.call.result` | Truncated response (max 1024 chars) | `{"cluster":"prod",...}` |
| `mcp.method.name` | JSON-RPC method | `tools/call` |
| `mcp.protocol.version` | MCP protocol version | `2025-03-26` |
| `mcp.session.id` | Agent session identifier | `sess_abc123` |
| `error.type` | Error classification (on failure only) | `PROVIDER_NOT_FOUND` |

### Context Propagation

The server extracts W3C Trace Context (`traceparent`/`tracestate`) from `params._meta` in each MCP request. This enables **end-to-end traces** spanning:

```
AI Agent → MCP Server → K8s API
```

If your AI agent sets `traceparent` in the `_meta` field of tool call requests, the MCP server will join those traces automatically.

### Error Spans

When a tool call fails:

- Span status is set to `ERROR`
- `error.type` is set to the MCPError code (e.g., `PROVIDER_NOT_FOUND`, `INVALID_INPUT`, `PROBE_TIMEOUT`) or `tool_error` for unclassified failures
- The error is recorded as a span event with the full error message

## Metrics

### GenAI Semantic Convention Metrics

| Metric | Type | Unit | Dimensions | Description |
|--------|------|------|------------|-------------|
| `gen_ai.server.request.duration` | Histogram | seconds | `gen_ai.tool.name`, `error.type` | Tool call execution duration |
| `gen_ai.server.request.count` | Counter | — | `gen_ai.tool.name`, `error.type` | Number of tool call requests |

### Custom Domain Metrics

| Metric | Type | Dimensions | Description |
|--------|------|------------|-------------|
| `mcp.findings.total` | Counter | `severity`, `analyzer` | Diagnostic findings emitted (per severity and tool) |
| `mcp.errors.total` | Counter | `error.code`, `gen_ai.tool.name` | Tool execution errors (per error code and tool) |

### Example Queries

**Average tool call duration by tool (PromQL):**
```promql
histogram_quantile(0.95, rate(gen_ai_server_request_duration_bucket[5m]))
```

**Error rate by tool:**
```promql
rate(gen_ai_server_request_count{error_type!=""}[5m])
/ rate(gen_ai_server_request_count[5m])
```

**Critical findings spike:**
```promql
rate(mcp_findings_total{severity="critical"}[5m])
```

## Logs

When OTel is enabled, structured logs (via Go's `slog`) are bridged to OTel Logs using `otelslog`. Every log entry emitted within an active span context is automatically enriched with:

- `trace_id` — links to the parent trace
- `span_id` — links to the specific span

This enables **log-to-trace correlation** in observability platforms — click a log entry to jump to its full trace.

### Log Format

Logs are JSON-structured:

```json
{
  "time": "2026-02-25T10:30:00Z",
  "level": "INFO",
  "msg": "tool execution completed",
  "tool_name": "list_services",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7"
}
```

When OTel is disabled, logs are still JSON-structured but without `trace_id`/`span_id` fields.

## Backend Integration Examples

### OTel Collector (In-Cluster)

Deploy an OTel Collector in the `observability` namespace:

```yaml
# otel-collector.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: otel-collector-config
  namespace: observability
data:
  config.yaml: |
    receivers:
      otlp:
        protocols:
          grpc:
            endpoint: 0.0.0.0:4317
    exporters:
      debug:
        verbosity: detailed
    service:
      pipelines:
        traces:
          receivers: [otlp]
          exporters: [debug]
        metrics:
          receivers: [otlp]
          exporters: [debug]
        logs:
          receivers: [otlp]
          exporters: [debug]
```

Then set the Helm value:

```yaml
otel:
  enabled: true
  endpoint: "otel-collector.observability.svc.cluster.local:4317"
```

### Jaeger

Export traces to Jaeger via the OTel Collector:

```yaml
exporters:
  otlp/jaeger:
    endpoint: jaeger-collector.observability.svc.cluster.local:4317
    tls:
      insecure: true

service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [otlp/jaeger]
```

### Grafana (Tempo + Mimir + Loki)

Export all three signals to the Grafana stack:

```yaml
exporters:
  otlphttp/tempo:
    endpoint: http://tempo.observability.svc.cluster.local:4318
  prometheusremotewrite:
    endpoint: http://mimir.observability.svc.cluster.local/api/v1/push
  loki:
    endpoint: http://loki.observability.svc.cluster.local:3100/loki/api/v1/push

service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [otlphttp/tempo]
    metrics:
      receivers: [otlp]
      exporters: [prometheusremotewrite]
    logs:
      receivers: [otlp]
      exporters: [loki]
```

### Dynatrace

Export directly to Dynatrace via OTLP:

```yaml
exporters:
  otlphttp/dynatrace:
    endpoint: "https://{your-environment-id}.live.dynatrace.com/api/v2/otlp"
    headers:
      Authorization: "Api-Token {your-api-token}"

service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [otlphttp/dynatrace]
    metrics:
      receivers: [otlp]
      exporters: [otlphttp/dynatrace]
    logs:
      receivers: [otlp]
      exporters: [otlphttp/dynatrace]
```

Or point the MCP server directly to the Dynatrace OTLP endpoint:

```yaml
otel:
  enabled: true
  endpoint: "https://{your-environment-id}.live.dynatrace.com/api/v2/otlp"
  insecure: false
```

## Complete MCP Tool Spans Reference

Every registered tool produces an `execute_tool` span when invoked. Below is the full list:

### Core K8s Tools (Always Available)

| Tool | Span Name |
|------|-----------|
| `list_services` | `execute_tool list_services` |
| `get_service` | `execute_tool get_service` |
| `list_endpoints` | `execute_tool list_endpoints` |
| `list_network_policies` | `execute_tool list_network_policies` |
| `get_network_policy` | `execute_tool get_network_policy` |
| `check_dns` | `execute_tool check_dns` |
| `check_kube_proxy_health` | `execute_tool check_kube_proxy_health` |
| `list_ingresses` | `execute_tool list_ingresses` |
| `get_ingress` | `execute_tool get_ingress` |

### Log Tools (Always Available)

| Tool | Span Name |
|------|-----------|
| `get_proxy_logs` | `execute_tool get_proxy_logs` |
| `get_gateway_logs` | `execute_tool get_gateway_logs` |
| `get_infra_logs` | `execute_tool get_infra_logs` |
| `analyze_log_errors` | `execute_tool analyze_log_errors` |

### Probe Tools (Always Available)

| Tool | Span Name |
|------|-----------|
| `probe_connectivity` | `execute_tool probe_connectivity` |
| `probe_dns` | `execute_tool probe_dns` |
| `probe_http` | `execute_tool probe_http` |

### Skill Tools (Always Available)

| Tool | Span Name |
|------|-----------|
| `list_skills` | `execute_tool list_skills` |
| `run_skill` | `execute_tool run_skill` |
| `suggest_remediation` | `execute_tool suggest_remediation` |

### Gateway API Tools (When Gateway API CRDs Detected)

| Tool | Span Name |
|------|-----------|
| `list_gateways` | `execute_tool list_gateways` |
| `get_gateway` | `execute_tool get_gateway` |
| `list_httproutes` | `execute_tool list_httproutes` |
| `get_httproute` | `execute_tool get_httproute` |
| `list_grpcroutes` | `execute_tool list_grpcroutes` |
| `get_grpcroute` | `execute_tool get_grpcroute` |
| `list_referencegrants` | `execute_tool list_referencegrants` |
| `get_referencegrant` | `execute_tool get_referencegrant` |
| `scan_gateway_misconfigs` | `execute_tool scan_gateway_misconfigs` |
| `check_gateway_conformance` | `execute_tool check_gateway_conformance` |
| `design_gateway_api` | `execute_tool design_gateway_api` |

### Istio Tools (When Istio CRDs Detected)

| Tool | Span Name |
|------|-----------|
| `list_istio_resources` | `execute_tool list_istio_resources` |
| `get_istio_resource` | `execute_tool get_istio_resource` |
| `check_sidecar_injection` | `execute_tool check_sidecar_injection` |
| `check_istio_mtls` | `execute_tool check_istio_mtls` |
| `validate_istio_config` | `execute_tool validate_istio_config` |
| `analyze_istio_authpolicy` | `execute_tool analyze_istio_authpolicy` |
| `analyze_istio_routing` | `execute_tool analyze_istio_routing` |
| `design_istio` | `execute_tool design_istio` |

### kgateway Tools (When kgateway CRDs Detected)

| Tool | Span Name |
|------|-----------|
| `list_kgateway_resources` | `execute_tool list_kgateway_resources` |
| `validate_kgateway_resource` | `execute_tool validate_kgateway_resource` |
| `check_kgateway_health` | `execute_tool check_kgateway_health` |
| `design_kgateway` | `execute_tool design_kgateway` |

### Tier 2 Provider Tools (When Respective CRDs Detected)

| Tool | Span Name |
|------|-----------|
| `check_kuma_status` | `execute_tool check_kuma_status` |
| `check_linkerd_status` | `execute_tool check_linkerd_status` |
| `list_cilium_policies` | `execute_tool list_cilium_policies` |
| `check_cilium_status` | `execute_tool check_cilium_status` |
| `list_calico_policies` | `execute_tool list_calico_policies` |
| `check_calico_status` | `execute_tool check_calico_status` |
| `check_flannel_status` | `execute_tool check_flannel_status` |
