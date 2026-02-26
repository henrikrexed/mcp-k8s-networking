# Getting Started

## Prerequisites

- Kubernetes 1.28+
- Helm 3+ (for Helm installation)
- `kubectl` configured with cluster access

## Installation

### Option 1: Helm Chart

```bash
helm install mcp-k8s-networking ./deploy/helm/mcp-k8s-networking \
  --namespace mcp-k8s-networking \
  --create-namespace \
  --set config.clusterName=my-cluster
```

Common overrides:

```bash
helm install mcp-k8s-networking ./deploy/helm/mcp-k8s-networking \
  --namespace mcp-k8s-networking \
  --create-namespace \
  --set config.clusterName=my-cluster \
  --set config.logLevel=debug \
  --set replicaCount=2 \
  --set probe.maxConcurrent=10
```

### Option 2: Raw YAML Manifests

1. Edit `deploy/manifests/install.yaml` and set `CLUSTER_NAME` to your cluster name
2. Apply:

```bash
kubectl apply -f deploy/manifests/install.yaml
```

### Exposing via Gateway API

To expose the MCP server through a Gateway API HTTPRoute:

```bash
helm install mcp-k8s-networking ./deploy/helm/mcp-k8s-networking \
  --namespace mcp-k8s-networking \
  --create-namespace \
  --set config.clusterName=my-cluster \
  --set gatewayAPI.enabled=true \
  --set gatewayAPI.gatewayName=my-gateway \
  --set gatewayAPI.gatewayNamespace=gateway-system \
  --set gatewayAPI.hostname=mcp.example.com
```

## Verifying the Deployment

```bash
# Check pods are running
kubectl get pods -n mcp-k8s-networking

# Check readiness
kubectl get endpoints mcp-k8s-networking -n mcp-k8s-networking

# View logs
kubectl logs -n mcp-k8s-networking -l app.kubernetes.io/name=mcp-k8s-networking
```

## Connecting an AI Agent

The MCP server exposes a Streamable HTTP endpoint at `/mcp` on port 8080 (default).

From within the cluster:

```
http://mcp-k8s-networking.mcp-k8s-networking.svc.cluster.local:8080/mcp
```

If exposed via Gateway API HTTPRoute, use the configured hostname.

## Enable Observability

mcp-k8s-networking exports traces, metrics, and logs via OTLP gRPC. To enable it, point the server at an OpenTelemetry Collector.

### Via Helm Values

```yaml
otel:
  enabled: true
  endpoint: "otel-collector.observability.svc.cluster.local:4317"
  insecure: true
  serviceName: "mcp-k8s-networking"
```

Or on the command line:

```bash
helm install mcp-k8s-networking ./deploy/helm/mcp-k8s-networking \
  --namespace mcp-k8s-networking \
  --create-namespace \
  --set config.clusterName=my-cluster \
  --set otel.enabled=true \
  --set otel.endpoint="otel-collector.observability.svc.cluster.local:4317"
```

### Via Environment Variables

If deploying without Helm, set these on the container:

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector.observability.svc.cluster.local:4317
OTEL_EXPORTER_OTLP_INSECURE=true
OTEL_SERVICE_NAME=mcp-k8s-networking
```

### Minimal OTel Collector Config

Deploy an OTel Collector that receives from the MCP server:

```yaml
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

Replace the `debug` exporter with your backend of choice. Supported backends include **Dynatrace**, **Grafana** (Tempo + Mimir + Loki), **Jaeger**, **Zipkin**, **Datadog**, and any OTLP-compatible endpoint.

See the full [Observability guide](observability.md) for backend-specific configuration examples.

## Next Steps

- Browse the [Tools Reference](tools/index.md) to see available diagnostics
- Review [Configuration](configuration.md) for all environment variables
- Read the [Architecture](architecture.md) overview
- Learn how to [register this MCP server in your AI agent](mcp-skill-installation.md)
