# Tools Reference

mcp-k8s-networking exposes diagnostic tools dynamically based on which networking CRDs are installed in your cluster.

## Tool Categories

| Category | Tools | Availability |
|----------|-------|-------------|
| [Core Kubernetes](core-k8s.md) | 9 tools | Always available |
| [Log Collection](logs.md) | 4 tools | Always available |
| [Active Probing](probing.md) | 3 tools | Always available |
| [Gateway API](gateway-api.md) | 11 tools | When Gateway API CRDs detected |
| [Istio](istio.md) | 8 tools | When Istio CRDs detected |
| [kgateway](kgateway.md) | 4 tools | When kgateway CRDs detected |
| [Tier 2 Providers](tier2-providers.md) | 9 tools | Per-provider CRD detection |
| [Design Guidance](design-guidance.md) | 4 tools | Per-provider + always |
| [Agent Skills](skills.md) | 6 tools | Per-provider CRD detection |

## Response Format

All tools return a `StandardResponse` envelope:

```json
{
  "cluster": "my-cluster",
  "timestamp": "2024-01-15T10:30:00Z",
  "tool": "list_services",
  "data": {
    "findings": [...],
    "metadata": {
      "clusterName": "my-cluster",
      "timestamp": "2024-01-15T10:30:00Z",
      "namespace": "default",
      "provider": ""
    }
  }
}
```

Each finding uses the `DiagnosticFinding` structure:

```json
{
  "severity": "warning",
  "category": "routing",
  "resource": {
    "kind": "Service",
    "namespace": "default",
    "name": "my-service"
  },
  "summary": "Service has no matching endpoints",
  "detail": "selector=app:my-app matched 0 pods",
  "suggestion": "Check pod labels match the service selector"
}
```

### Compact vs Detail Mode

By default, tools return compact output (only `summary`). Pass `"detail": true` to include `detail` and `suggestion` fields.
