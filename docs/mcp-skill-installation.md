# Installing as an MCP Skill

mcp-k8s-networking is an MCP server that AI agents connect to for Kubernetes networking diagnostics. This page explains how to register it as a tool/skill in popular AI agents and platforms.

## Endpoint URL

The MCP server exposes a Streamable HTTP endpoint:

| Scenario | URL |
|----------|-----|
| In-cluster (K8s Service) | `http://mcp-k8s-networking.<namespace>.svc:8080/mcp` |
| Port-forwarded | `http://localhost:8080/mcp` |
| Via Gateway API HTTPRoute | `http://<configured-hostname>/mcp` |

To port-forward for local testing:

```bash
kubectl port-forward -n mcp-k8s-networking svc/mcp-k8s-networking 8080:8080
```

## Claude Desktop

Add to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "mcp-k8s-networking": {
      "url": "http://localhost:8080/mcp",
      "transport": "streamable-http"
    }
  }
}
```

**Config file location:**

- macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`
- Windows: `%APPDATA%\Claude\claude_desktop_config.json`
- Linux: `~/.config/Claude/claude_desktop_config.json`

After saving, restart Claude Desktop. The K8s networking tools will appear in Claude's tool list.

## Claude Code (CLI)

Add to your project's `.mcp.json` or global `~/.claude/mcp.json`:

```json
{
  "mcpServers": {
    "mcp-k8s-networking": {
      "url": "http://localhost:8080/mcp",
      "transport": "streamable-http"
    }
  }
}
```

Or register via the CLI:

```bash
claude mcp add mcp-k8s-networking --transport streamable-http http://localhost:8080/mcp
```

## Cursor / VS Code

Add to your workspace or user `settings.json`:

```json
{
  "mcp": {
    "servers": {
      "mcp-k8s-networking": {
        "url": "http://localhost:8080/mcp",
        "transport": "streamable-http"
      }
    }
  }
}
```

Alternatively, create a `.cursor/mcp.json` file in your project root:

```json
{
  "mcpServers": {
    "mcp-k8s-networking": {
      "url": "http://localhost:8080/mcp",
      "transport": "streamable-http"
    }
  }
}
```

## kagent (Kubernetes-Native)

[kagent](https://kagent.dev) runs AI agents directly in Kubernetes. Register mcp-k8s-networking as an MCP server:

```yaml
apiVersion: kagent.dev/v1alpha1
kind: MCPServer
metadata:
  name: mcp-k8s-networking
spec:
  url: "http://mcp-k8s-networking.mcp-k8s-networking.svc:8080/mcp"
  transport: streamable-http
```

Apply it:

```bash
kubectl apply -f mcp-server.yaml
```

Then reference it in your kagent Agent resource to give the agent access to K8s networking diagnostics.

## OpenClaw

Add mcp-k8s-networking as an MCP tool in your OpenClaw configuration:

```yaml
tools:
  mcp:
    - name: mcp-k8s-networking
      url: "http://mcp-k8s-networking.mcp-k8s-networking.svc:8080/mcp"
      transport: streamable-http
```

## Other MCP Clients

Any MCP-compatible client can connect using:

- **Transport**: Streamable HTTP
- **Endpoint**: `http://<host>:8080/mcp`
- **Protocol**: JSON-RPC 2.0 over MCP

The server supports the standard MCP handshake (`initialize` / `initialized` / `tools/list` / `tools/call`).

## Verifying the Connection

After registering, ask your AI agent:

> "List the Kubernetes services in the default namespace"

If the connection is working, the agent will call the `list_services` tool and return structured results from your cluster.

## Troubleshooting

**Agent can't connect:**

- Ensure the MCP server pod is running: `kubectl get pods -n mcp-k8s-networking`
- Ensure port-forwarding is active if connecting from outside the cluster
- Check server logs: `kubectl logs -n mcp-k8s-networking -l app.kubernetes.io/name=mcp-k8s-networking`

**No tools visible:**

- The server dynamically registers tools based on detected CRDs. Wait for the readiness probe to pass
- Check readiness: `curl http://localhost:8081/readyz`

**Timeout errors:**

- The default tool timeout is 10s. For slow clusters, increase via `TOOL_TIMEOUT` or `config.toolTimeout` Helm value
