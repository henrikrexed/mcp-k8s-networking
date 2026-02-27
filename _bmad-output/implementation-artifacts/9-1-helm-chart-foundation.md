# Story 9.1: Helm Chart Foundation

Status: done

## Story

As a platform engineer,
I want to install mcp-k8s-networking via Helm,
so that I can deploy and configure it consistently across clusters with standard Kubernetes packaging.

## Acceptance Criteria

1. A Helm chart exists at `deploy/helm/mcp-k8s-networking/` with Chart.yaml (v0.1.0, appVersion 1.0.0)
2. values.yaml exposes all configurable values: image, replicas, resources, config, probe, securityContext, otel, gatewayAPI
3. `_helpers.tpl` provides standard helpers: name, fullname, chart, labels, selectorLabels, serviceAccountName
4. Namespace template creates both the release namespace and the probe namespace (mcp-diagnostics)
5. ServiceAccount template respects `serviceAccount.create` and `serviceAccount.name` overrides
6. ClusterRole provides comprehensive read-only RBAC for all supported providers (core K8s, Gateway API, Istio, kgateway, Cilium, Calico, Kuma, Linkerd) plus ephemeral probe pod create/delete
7. ClusterRoleBinding binds the ClusterRole to the ServiceAccount
8. Deployment template maps all config/probe/otel env vars, includes liveness/readiness probes on the health port, and applies securityContext
9. Service template exposes the MCP port as ClusterIP

## Tasks / Subtasks

- [x] Create Chart.yaml with apiVersion v2, chart version 0.1.0, appVersion 1.0.0, keywords, home/sources/maintainers
- [x] Create values.yaml with all configurable sections: replicaCount, image, serviceAccount, rbac, config, probe, service, resources, securityContext, gatewayAPI, otel
- [x] Create templates/_helpers.tpl with standard Helm helpers (name, fullname, chart, labels, selectorLabels, serviceAccountName)
- [x] Create templates/namespace.yaml rendering release namespace and conditional probe namespace
- [x] Create templates/serviceaccount.yaml conditional on serviceAccount.create with annotations support
- [x] Create templates/clusterrole.yaml with read-only rules for all provider API groups plus probe pod create/delete
- [x] Create templates/clusterrolebinding.yaml binding ClusterRole to ServiceAccount
- [x] Create templates/deployment.yaml with full env var mapping (CLUSTER_NAME, PORT, LOG_LEVEL, NAMESPACE, CACHE_TTL, TOOL_TIMEOUT, PROBE_NAMESPACE, PROBE_IMAGE, MAX_CONCURRENT_PROBES, OTEL_EXPORTER_OTLP_ENDPOINT), health probes on /healthz and /readyz, securityContext, resource limits
- [x] Create templates/service.yaml exposing MCP port as ClusterIP

## Dev Notes

### Chart Structure

The chart follows standard Helm conventions with `_helpers.tpl` providing reusable template functions. The `required` function is used in the deployment template to enforce that `config.clusterName` is set at install time.

### RBAC Design

The ClusterRole grants read-only access to all supported networking provider CRDs (Gateway API, Istio, kgateway, Cilium, Calico, Kuma, Linkerd) plus core Kubernetes resources (services, endpoints, pods, pods/log, configmaps, namespaces, deployments, daemonsets, networkpolicies, ingresses, CRDs). Probe pod create/delete is scoped to the probe namespace.

### Health Probes

The deployment exposes two ports: `mcp` (config.port, default 8080) and `health` (config.port + 1, default 8081). Liveness probe hits `/healthz` and readiness probe hits `/readyz` on the health port.

### Security

The default securityContext enforces: `runAsNonRoot: true`, `readOnlyRootFilesystem: true`, `allowPrivilegeEscalation: false`, `seccompProfile: RuntimeDefault`.

### values.yaml Defaults

| Section | Key | Default |
|---|---|---|
| image.repository | | ghcr.io/henrikrexed/mcp-k8s-networking |
| image.tag | | "" (defaults to appVersion) |
| config.clusterName | | "" (required at install) |
| config.port | | 8080 |
| config.logLevel | | info |
| config.cacheTTL | | 30s |
| config.toolTimeout | | 10s |
| probe.namespace | | mcp-diagnostics |
| probe.maxConcurrent | | 5 |
| resources.requests | | cpu: 50m, memory: 64Mi |
| resources.limits | | cpu: 200m, memory: 128Mi |
| gatewayAPI.enabled | | false |
| otel.enabled | | false |

## File List

| File | Action |
|---|---|
| `deploy/helm/mcp-k8s-networking/Chart.yaml` | Created |
| `deploy/helm/mcp-k8s-networking/values.yaml` | Created |
| `deploy/helm/mcp-k8s-networking/templates/_helpers.tpl` | Created |
| `deploy/helm/mcp-k8s-networking/templates/namespace.yaml` | Created |
| `deploy/helm/mcp-k8s-networking/templates/serviceaccount.yaml` | Created |
| `deploy/helm/mcp-k8s-networking/templates/clusterrole.yaml` | Created |
| `deploy/helm/mcp-k8s-networking/templates/clusterrolebinding.yaml` | Created |
| `deploy/helm/mcp-k8s-networking/templates/deployment.yaml` | Created |
| `deploy/helm/mcp-k8s-networking/templates/service.yaml` | Created |
