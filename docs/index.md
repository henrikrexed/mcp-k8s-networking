# mcp-k8s-networking

An MCP (Model Context Protocol) server that provides AI agents with Kubernetes networking diagnostic capabilities.

## What is this?

mcp-k8s-networking is a diagnostic server that AI agents connect to via the MCP protocol. It dynamically discovers installed networking providers (Gateway API, Istio, Cilium, Calico, Linkerd, Kuma, kgateway, Flannel) and exposes diagnostic tools for each.

## Key Features

- **Dynamic CRD Discovery** - Automatically detects installed networking CRDs via watch and registers/unregisters tools in real-time
- **40+ Diagnostic Tools** - Covering core Kubernetes, Gateway API, Istio, kgateway, and Tier 2 providers
- **Active Probing** - Deploy ephemeral pods to test connectivity, DNS, and HTTP reachability
- **Design Guidance** - Generate provider-specific YAML templates based on user intent
- **Agent Skills** - Multi-step playbooks for common networking tasks
- **Structured Diagnostics** - Consistent `DiagnosticFinding` format with severity, category, and remediation suggestions
- **Production Ready** - Helm chart, RBAC, health probes, structured logging, OpenTelemetry support

## Supported Providers

| Provider | Tier | Capabilities |
|----------|------|-------------|
| Gateway API | 1 | Full diagnostics, validation, conformance, design guidance |
| Istio | 1 | Full diagnostics, mTLS, routing analysis, design guidance |
| kgateway | 1 | Resource validation, health summary, design guidance |
| Cilium | 2 | NetworkPolicy listing, agent health |
| Calico | 2 | NetworkPolicy listing, node health |
| Linkerd | 2 | Control plane health, injection status |
| Kuma | 2 | Control plane health, mesh/dataplane status |
| Flannel | 2 | DaemonSet health, configuration |

## Quick Start

```bash
# Deploy with Helm
helm install mcp-k8s-networking ./deploy/helm/mcp-k8s-networking \
  --namespace mcp-k8s-networking --create-namespace \
  --set config.clusterName=my-cluster

# Or deploy with raw YAML
kubectl apply -f deploy/manifests/install.yaml
```

See the [Getting Started](getting-started.md) guide for full details.
