# Tools Reference

mcp-k8s-networking exposes 52 diagnostic tools dynamically based on which networking CRDs are installed in your cluster.

## Tool Categories

| Category | Tools | Availability |
|----------|-------|-------------|
| [Core Kubernetes](core-k8s.md) | 11 tools | Always available |
| [Log Collection](logs.md) | 4 tools | Always available |
| [Active Probing](probing.md) | 3 tools | Always available |
| [Gateway API](gateway-api.md) | 10 tools | When Gateway API CRDs detected |
| [Istio](istio.md) | 7 tools | When Istio CRDs detected |
| [kgateway](kgateway.md) | 3 tools | When kgateway CRDs detected |
| [Tier 2 Providers](tier2-providers.md) | 8 tools | Per-provider CRD detection |
| [Design Guidance](design-guidance.md) | 4 tools | Per-provider + always |
| [Agent Skills](skills.md) | 2 tools | Always available |

## Response Format

All tools return compact markdown tables optimized for LLM token efficiency. See the [Response Format](../response-format.md) page for full details.

```markdown
| St | Resource | Summary | Detail |
|----|----------|---------|--------|
| ✅ | Service default/my-svc | ClusterIP 10.96.1.5 ports=[8080] | selector={app:my-app} |
| ⚠️ | HTTPRoute prod/api | backend svc not found | create the service |
```

Each finding uses a severity icon:

| Icon | Level | Meaning |
|------|-------|---------|
| ✅ | OK | No issues found |
| ℹ️ | Info | Informational finding |
| ⚠️ | Warning | Potential issue, investigate |
| ❗ | Critical | Requires immediate action |
