# Story 9.2: Gateway API HTTPRoute Exposure

Status: done

## Story

As a platform engineer using Gateway API,
I want the Helm chart to optionally create an HTTPRoute for the MCP server,
so that I can expose the MCP endpoint through my existing Gateway infrastructure.

## Acceptance Criteria

1. When `gatewayAPI.enabled=true`, an HTTPRoute resource is rendered in the chart output
2. The HTTPRoute uses a parentRef with configurable `gatewayName` and `gatewayNamespace`
3. When `gatewayAPI.hostname` is set, the HTTPRoute includes hostname matching
4. When `gatewayAPI.gatewayName` is not set but `gatewayAPI.enabled=true`, the chart fails with a required value error
5. The HTTPRoute routes traffic on `/mcp` path prefix to the MCP service backend

## Tasks / Subtasks

- [x] Create templates/httproute.yaml conditionally rendered with `{{- if .Values.gatewayAPI.enabled -}}`
- [x] Implement parentRef with `required` validation for gatewayName when gatewayAPI is enabled
- [x] Add conditional gatewayNamespace in parentRef (only when set)
- [x] Add conditional hostname matching (only when gatewayAPI.hostname is set)
- [x] Configure path match rule (PathPrefix `/mcp`) with backendRef to the MCP service
- [x] Add gatewayAPI section to values.yaml with enabled, provider, gatewayName, gatewayNamespace, hostname fields

## Dev Notes

### Conditional Rendering

The HTTPRoute template is wrapped in `{{- if .Values.gatewayAPI.enabled -}}` so it is only included when explicitly enabled. This prevents errors on clusters without Gateway API CRDs installed.

### Required gatewayName Validation

The template uses Helm's `required` function to enforce that `gatewayAPI.gatewayName` is provided when the feature is enabled:
```yaml
- name: {{ required "gatewayAPI.gatewayName is required when gatewayAPI is enabled" .Values.gatewayAPI.gatewayName }}
```

### Route Configuration

The HTTPRoute matches on `PathPrefix: /mcp` and routes to the MCP service on the configured service port. The optional hostname field supports restricting the route to a specific domain.

### values.yaml gatewayAPI Section

```yaml
gatewayAPI:
  enabled: false
  provider: ""  # e.g., istio, envoy-gateway, kgateway
  gatewayName: ""
  gatewayNamespace: ""
  hostname: ""
```

## File List

| File | Action |
|---|---|
| `deploy/helm/mcp-k8s-networking/templates/httproute.yaml` | Created |
| `deploy/helm/mcp-k8s-networking/values.yaml` | Modified (gatewayAPI section) |
