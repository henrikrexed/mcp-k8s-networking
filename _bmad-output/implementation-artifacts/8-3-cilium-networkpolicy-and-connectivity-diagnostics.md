# Story 8.3: Cilium NetworkPolicy and Connectivity Diagnostics

Status: done

## Story

As an AI agent,
I want to list Cilium network policies and check Cilium agent health,
so that I can diagnose Cilium-specific policy enforcement and connectivity issues.

## Acceptance Criteria

1. The list_cilium_policies tool lists CiliumNetworkPolicy resources (namespaced) and CiliumClusterwideNetworkPolicy resources (cluster-scoped)
2. The check_cilium_status tool checks Cilium agent pod health using the `k8s-app=cilium` label in kube-system
3. The check_cilium_status tool counts CiliumEndpoint resources to assess data plane coverage
4. Agent pod readiness is tracked per-node with node names in the detail field
5. Both tools are conditionally registered when Cilium CRDs are detected (HasCilium)

## Tasks / Subtasks

- [x] Task 1: Define Cilium GVRs (AC: 1, 3)
  - [x] Define `ciliumNPGVR`: cilium.io/v2/ciliumnetworkpolicies
  - [x] Define `ciliumCNPGVR`: cilium.io/v2/ciliumclusterwidenetworkpolicies
  - [x] Define `ciliumEPGVR`: cilium.io/v2/ciliumendpoints
- [x] Task 2: Implement ListCiliumPoliciesTool (AC: 1)
  - [x] Create struct embedding BaseTool
  - [x] Define InputSchema: namespace (optional)
  - [x] List CiliumNetworkPolicy: namespace-scoped when namespace given, cluster-wide when empty
  - [x] List CiliumClusterwideNetworkPolicy cluster-wide (always)
  - [x] Create DiagnosticFinding per policy with ResourceRef including Kind and namespace
  - [x] Return "No Cilium network policies found" info finding when both lists empty
- [x] Task 3: Implement CheckCiliumStatusTool (AC: 2, 3, 4)
  - [x] Create struct embedding BaseTool
  - [x] Define InputSchema: namespace (optional, for endpoint filtering)
  - [x] List Cilium agent pods with `k8s-app=cilium` in kube-system namespace
  - [x] Track per-pod readiness (all containers ready) and collect node names
  - [x] Set severity: OK when all ready, Warning when partial, Critical when none ready
  - [x] Include node list in Detail field: `nodes={comma-separated}`
  - [x] Count CiliumEndpoint resources, namespace-scoped or cluster-wide
- [x] Task 4: Register tools in main.go (AC: 5)
  - [x] Register both tools inside `features.HasCilium` conditional block
  - [x] Include "list_cilium_policies" and "check_cilium_status" in ciliumToolNames for unregistration

## Dev Notes

### Key Design Decisions

- **Two tools for separation of concerns**: Policy listing and health checking are separate tools. This allows agents to call just the policy tool when investigating policy issues, or just the status tool for health checks.
- **CiliumClusterwideNetworkPolicy always listed**: Unlike namespaced CiliumNetworkPolicy which can be filtered, CiliumClusterwideNetworkPolicy is always listed cluster-wide regardless of the namespace parameter.
- **k8s-app=cilium agent label**: The standard label for Cilium agent pods. These run as a DaemonSet, so each pod corresponds to one node. The node names in the detail field help identify which nodes have unhealthy agents.
- **CiliumEndpoint as data plane metric**: CiliumEndpoint count provides a proxy for how many workloads are managed by Cilium. This correlates with the number of pods that have Cilium network identities.
- **Per-resource findings for policies**: Each policy gets its own DiagnosticFinding rather than a summary count. This allows agents to reference specific policies when discussing issues.
- **kube-system namespace for agents**: Cilium agents always run in kube-system. No namespace fallback needed (unlike Calico which may be in calico-system).

### Files Created

| File | Purpose |
|---|---|
| `pkg/tools/provider_cilium.go` | ListCiliumPoliciesTool, CheckCiliumStatusTool, ciliumNPGVR, ciliumCNPGVR, ciliumEPGVR |

### Files Modified

| File | Action |
|---|---|
| `cmd/server/main.go` | Registered both tools conditionally under HasCilium |

## Dev Agent Record

### Agent Model Used
claude-opus-4-6

### Completion Notes List
- Policy tool uses CategoryPolicy, status tool uses CategoryMesh for findings
- Provider tag "cilium" passed to NewToolResultResponse for both tools
- Node names in agent health detail help correlate with node-specific issues
- Empty policy finding prevents confusing empty responses

### File List
- pkg/tools/provider_cilium.go
- cmd/server/main.go
