# Response Format

All tools return **compact markdown tables** optimized for LLM token efficiency.

## Format

```markdown
| St | Resource | Summary | Detail |
|----|----------|---------|--------|
| ✅ | Service otel-demo/product-catalog | ClusterIP 10.96.1.5 ports=[8080] | selector={app:pc} |
| ⚠️ | GRPCRoute otel-demo/mesh-route | parentRef=Service (GAMMA) | check mesh support → enable istio |
| ❗ | HTTPRoute otel-demo/frontend | backend svc not found | create the service |
```

## Severity Icons

| Icon | Level | Meaning |
|------|-------|---------|
| ✅ | OK | No issues found |
| ℹ️ | Info | Informational finding |
| ⚠️ | Warning | Potential issue, investigate |
| ❗ | Critical | Requires immediate action |

## Columns

| Column | Content |
|--------|---------|
| **St** | Severity icon |
| **Resource** | Kubernetes resource kind + namespace/name |
| **Summary** | Key diagnostic information |
| **Detail** | Additional context + suggested action (→) |

## Design Decisions

### Why markdown tables instead of JSON?

MCP tools are primarily consumed by LLM agents. JSON responses are **3x more token-expensive** than compact text:

| Format | Tokens per finding | 17 calls × 5 findings |
|--------|-------------------|----------------------|
| Pretty JSON | ~80 | ~6,800 |
| Line-per-finding | ~25 | ~2,100 |
| **Markdown table** | ~20 | **~1,700** |

### Why not full resource dumps?

Tools return **diagnostic summaries**, not raw Kubernetes objects. An LLM agent diagnosing a networking issue needs:

- ✅ "Service has 2 ready endpoints on port 8080"
- ❌ The full 200-line Service YAML with every annotation and status field

### GAMMA (Gateway API for Mesh)

Routes with `parentRef.kind: Service` are valid **GAMMA mesh routes** for east-west traffic.
The `scan_gateway_misconfigs` tool correctly identifies these and validates the referenced
Service exists, rather than flagging them as "missing Gateway".
