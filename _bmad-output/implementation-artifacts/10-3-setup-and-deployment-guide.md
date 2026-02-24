# Story 10.3: Setup and Deployment Guide

Status: done

## Story

As a platform engineer,
I want clear installation instructions and a comprehensive configuration reference,
so that I can deploy and configure the MCP server with either Helm or raw YAML manifests.

## Acceptance Criteria

1. `docs/getting-started.md` provides step-by-step Helm installation instructions with `helm install` command
2. `docs/getting-started.md` provides raw YAML installation instructions with `kubectl apply -f`
3. `docs/getting-started.md` covers prerequisites (Kubernetes cluster, kubectl, optional Helm)
4. `docs/configuration.md` documents every environment variable with its type, default value, and description
5. `docs/configuration.md` covers config sections: core, probing, OTel, and Gateway API Helm values

## Tasks / Subtasks

- [x] Create docs/getting-started.md with prerequisites section (K8s version, kubectl, Helm optional)
- [x] Add Helm installation instructions (helm repo add, helm install with required clusterName value)
- [x] Add raw YAML installation instructions (kubectl apply -f, note about editing CLUSTER_NAME)
- [x] Add verification steps (kubectl get pods, check logs, healthz endpoint)
- [x] Create docs/configuration.md with all environment variables table (CLUSTER_NAME, PORT, LOG_LEVEL, NAMESPACE, CACHE_TTL, TOOL_TIMEOUT, PROBE_NAMESPACE, PROBE_IMAGE, MAX_CONCURRENT_PROBES)
- [x] Document OTel configuration (OTEL_EXPORTER_OTLP_ENDPOINT, OTEL_SERVICE_NAME as standard OTel env vars)
- [x] Document Helm values mapping (config.*, probe.*, otel.*, gatewayAPI.*)

## Dev Notes

### Two Installation Paths

The getting-started guide presents both installation methods:
1. **Helm** (recommended): Provides templating, validation, and easy upgrades via `helm install mcp-k8s-networking deploy/helm/mcp-k8s-networking/ --set config.clusterName=my-cluster`
2. **Raw YAML**: Single-file apply for users who prefer not to use Helm, requires manual editing of CLUSTER_NAME

### Configuration Reference Organization

The configuration page organizes variables by section:
- **Core Config**: CLUSTER_NAME (required), PORT, LOG_LEVEL, NAMESPACE, CACHE_TTL, TOOL_TIMEOUT
- **Probe Config**: PROBE_NAMESPACE, PROBE_IMAGE, MAX_CONCURRENT_PROBES
- **OTel Config**: OTEL_EXPORTER_OTLP_ENDPOINT, OTEL_SERVICE_NAME (standard OTel conventions)
- **Helm Values**: Mapping between values.yaml paths and environment variables

### Helm Values to Env Var Mapping

| values.yaml Path | Environment Variable | Default |
|---|---|---|
| config.clusterName | CLUSTER_NAME | (required) |
| config.port | PORT | 8080 |
| config.logLevel | LOG_LEVEL | info |
| config.namespace | NAMESPACE | "" |
| config.cacheTTL | CACHE_TTL | 30s |
| config.toolTimeout | TOOL_TIMEOUT | 10s |
| probe.namespace | PROBE_NAMESPACE | mcp-diagnostics |
| probe.image | PROBE_IMAGE | ghcr.io/mcp-k8s-networking/probe:latest |
| probe.maxConcurrent | MAX_CONCURRENT_PROBES | 5 |
| otel.endpoint | OTEL_EXPORTER_OTLP_ENDPOINT | (disabled) |

## File List

| File | Action |
|---|---|
| `docs/getting-started.md` | Created |
| `docs/configuration.md` | Created |
