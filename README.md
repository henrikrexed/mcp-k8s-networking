# mcp-k8s-networking

A Model Context Protocol (MCP) server for Kubernetes networking diagnostics — Gateway API, Istio, Cilium, and core networking.

[![Build](https://github.com/henrikrexed/mcp-k8s-networking/actions/workflows/ci.yaml/badge.svg)](https://github.com/henrikrexed/mcp-k8s-networking/actions)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue)](LICENSE)

## Overview

`mcp-k8s-networking` runs in-cluster and gives AI agents deep visibility into Kubernetes networking. It analyzes Gateway API resources, service mesh configurations, network policies, and DNS — surfacing misconfigurations and providing remediation guidance.

**Key features:**
- 🔍 **Diagnostic tools**: Analyze HTTPRoutes, Gateways, Services, NetworkPolicies, DNS
- 🕸️ **Service mesh support**: Istio, Cilium, and Gateway API providers
- 🎯 **Active probing**: Connectivity tests, DNS resolution, latency checks
- 🏗️ **Design guidance**: Architecture recommendations for networking patterns
- 📊 **OpenTelemetry instrumented**: Full traces, metrics, and logs following GenAI + MCP semantic conventions
- 🔒 **Read-only RBAC**: Safe to run in production clusters
- 🌐 **Multi-cluster**: Every response includes cluster identity

## Quick Start

### Helm (recommended)

```bash
helm repo add henrikrexed https://henrikrexed.github.io/mcp-k8s-networking
helm install mcp-k8s-networking henrikrexed/mcp-k8s-networking \
  --namespace tools --create-namespace
```

### Docker

```bash
docker run -p 8080:8080 ghcr.io/henrikrexed/mcp-k8s-networking:latest
```

## MCP Tools

| Tool | Description |
|------|-------------|
| `diagnose_route` | Analyze HTTPRoute/Ingress for misconfigurations |
| `diagnose_gateway` | Check Gateway resource health and listeners |
| `diagnose_service` | Validate Service → Pod connectivity |
| `diagnose_networkpolicy` | Analyze NetworkPolicy rules and coverage |
| `diagnose_dns` | DNS resolution diagnostics |
| `probe_connectivity` | Active connectivity testing between pods/services |
| `design_guidance` | Architecture recommendations for networking patterns |

## Observability

The server produces OpenTelemetry traces, metrics, and logs following the [GenAI](https://opentelemetry.io/docs/specs/semconv/gen-ai/) and [MCP](https://opentelemetry.io/docs/specs/semconv/gen-ai/mcp/) semantic conventions.

Enable via Helm values:
```yaml
otel:
  enabled: true
  endpoint: "otel-collector.observability.svc.cluster.local:4317"
```

## Documentation

📖 Full documentation: [https://henrikrexed.github.io/mcp-k8s-networking](https://henrikrexed.github.io/mcp-k8s-networking)

## Part of IsItObservable

This project is part of the [IsItObservable](https://youtube.com/@IsItObservable) ecosystem — open-source tools for Kubernetes observability.

- [otel-collector-mcp](https://github.com/henrikrexed/otel-collector-mcp) — OTel Collector pipeline debugging
- [mcp-proxy](https://github.com/henrikrexed/mcp-proxy) — Universal OTel sidecar proxy for any MCP server

## License

Apache License 2.0

### Kubernetes Deployment (plain manifests)

Deploy using kubectl or kustomize:

```bash
# Using kustomize
kubectl apply -k deploy/kubernetes/

# Or apply individually
kubectl apply -f deploy/kubernetes/namespace.yaml
kubectl apply -f deploy/kubernetes/serviceaccount.yaml
kubectl apply -f deploy/kubernetes/rbac.yaml
kubectl apply -f deploy/kubernetes/deployment.yaml
kubectl apply -f deploy/kubernetes/service.yaml
```

Edit `deploy/kubernetes/deployment.yaml` to set your `CLUSTER_NAME` and OTLP endpoint before deploying.
