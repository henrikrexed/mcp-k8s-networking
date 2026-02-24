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

## Next Steps

- Browse the [Tools Reference](tools/index.md) to see available diagnostics
- Review [Configuration](configuration.md) for all environment variables
- Read the [Architecture](architecture.md) overview
