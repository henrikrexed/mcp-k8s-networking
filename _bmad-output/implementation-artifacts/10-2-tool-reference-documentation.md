# Story 10.2: Tool Reference Documentation

Status: done

## Story

As an AI agent developer or platform engineer,
I want comprehensive reference documentation for each MCP tool,
so that I understand tool names, descriptions, input parameters, and example usage patterns.

## Acceptance Criteria

1. `docs/tools/index.md` provides an overview of the tool categories and tool discovery mechanism
2. `docs/tools/core-k8s.md` documents core Kubernetes tools (list_services, check_dns_resolution, etc.)
3. `docs/tools/gateway-api.md` documents Gateway API tools (list_gateway_resources, validate_gateway_resource, etc.)
4. `docs/tools/istio.md` documents Istio tools (validate_istio_config, analyze_istio_authpolicy, analyze_istio_routing)
5. `docs/tools/kgateway.md` documents kgateway tools (list_kgateway_resources, validate_kgateway_resource)
6. `docs/tools/tier2-providers.md` documents Cilium, Calico, Kuma, Linkerd, Flannel tools
7. `docs/tools/logs.md` documents log collection tools (get_proxy_logs, get_pod_logs)
8. `docs/tools/probing.md` documents active probing tools (probe_dns, probe_connectivity, probe_http)
9. `docs/tools/design-guidance.md` documents the design guidance tool (suggest_gateway_api_config)
10. `docs/tools/skills.md` documents the skills system (list_skills, run_skill) and available skills

## Tasks / Subtasks

- [x] Create docs/tools/index.md with tool categories overview and CRD-based dynamic registration explanation
- [x] Create docs/tools/core-k8s.md with tool name, description, parameters table, and usage examples for core K8s tools
- [x] Create docs/tools/gateway-api.md documenting Gateway API diagnostic tools
- [x] Create docs/tools/istio.md documenting Istio validation and analysis tools
- [x] Create docs/tools/kgateway.md documenting kgateway-specific tools
- [x] Create docs/tools/tier2-providers.md documenting Cilium, Calico, Kuma, Linkerd, Flannel tools
- [x] Create docs/tools/logs.md documenting proxy and pod log retrieval tools
- [x] Create docs/tools/probing.md documenting ephemeral probe pod tools
- [x] Create docs/tools/design-guidance.md documenting the configuration suggestion tool
- [x] Create docs/tools/skills.md documenting the skills framework, list_skills, run_skill, and individual skill workflows

## Dev Notes

### Documentation Pattern

Each tool page follows a consistent structure:
- Tool name as heading
- Brief description
- Parameters table (name, type, required, description)
- Example MCP tool call JSON
- Example response structure
- Notes on CRD requirements and feature flags

### CRD-Based Availability

The documentation explains that tools appear/disappear dynamically based on installed CRDs. Each tool page notes which CRDs must be installed for the tool to be available (e.g., Gateway API tools require `gateway.networking.k8s.io` CRDs).

### Skills Documentation

The skills page documents both the MCP wrapper tools (list_skills, run_skill) and the individual skill workflows (expose_service_gateway_api, configure_istio_mtls, configure_traffic_split, create_network_policy) with their step-by-step execution flow.

## File List

| File | Action |
|---|---|
| `docs/tools/index.md` | Created |
| `docs/tools/core-k8s.md` | Created |
| `docs/tools/gateway-api.md` | Created |
| `docs/tools/istio.md` | Created |
| `docs/tools/kgateway.md` | Created |
| `docs/tools/tier2-providers.md` | Created |
| `docs/tools/logs.md` | Created |
| `docs/tools/probing.md` | Created |
| `docs/tools/design-guidance.md` | Created |
| `docs/tools/skills.md` | Created |
