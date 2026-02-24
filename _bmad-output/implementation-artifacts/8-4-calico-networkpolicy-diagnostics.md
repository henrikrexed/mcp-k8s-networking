# Story 8.4: Calico NetworkPolicy Diagnostics

Status: done

## Story

As an AI agent,
I want to list Calico network policies and check Calico node health,
so that I can diagnose Calico-specific policy enforcement and felix/node agent issues.

## Acceptance Criteria

1. The list_calico_policies tool lists Calico NetworkPolicy (namespaced) and GlobalNetworkPolicy (cluster-scoped) resources via crd.projectcalico.org/v1 CRDs
2. The check_calico_status tool checks calico-node DaemonSet pod health with namespace fallback (kube-system, then calico-system)
3. The check_calico_status tool checks calico-kube-controllers pod readiness with the same namespace fallback
4. Node agent readiness is tracked per-node with node names in the detail field
5. Both tools are conditionally registered when Calico CRDs are detected (HasCalico)

## Tasks / Subtasks

- [x] Task 1: Define Calico GVRs (AC: 1)
  - [x] Define `calicoNPGVR`: crd.projectcalico.org/v1/networkpolicies
  - [x] Define `calicoGNPGVR`: crd.projectcalico.org/v1/globalnetworkpolicies
- [x] Task 2: Implement ListCalicoPoliciesTool (AC: 1)
  - [x] Create struct embedding BaseTool
  - [x] Define InputSchema: namespace (optional)
  - [x] List Calico NetworkPolicy: namespace-scoped when namespace given, cluster-wide when empty
  - [x] List GlobalNetworkPolicy cluster-wide (always)
  - [x] Create DiagnosticFinding per policy with ResourceRef including APIVersion (crd.projectcalico.org/v1)
  - [x] Return "No Calico network policies found" info finding when both lists empty
- [x] Task 3: Implement CheckCalicoStatusTool (AC: 2, 3, 4)
  - [x] Create struct embedding BaseTool
  - [x] Define InputSchema: empty properties (no parameters needed)
  - [x] List calico-node pods with `k8s-app=calico-node` in kube-system, fallback to calico-system on error
  - [x] Track per-pod readiness and collect node names
  - [x] Set severity: OK all ready, Warning partial, Critical none ready (with total > 0 guard)
  - [x] List calico-kube-controllers pods with `k8s-app=calico-kube-controllers`, same namespace fallback
  - [x] Check controller readiness: all containers ready and at least one container status present
  - [x] Report controllers ready/total
- [x] Task 4: Register tools in main.go (AC: 5)
  - [x] Register both tools inside `features.HasCalico` conditional block
  - [x] Include "list_calico_policies" and "check_calico_status" in calicoToolNames for unregistration

## Dev Notes

### Key Design Decisions

- **Namespace fallback for Calico components**: Calico can be installed in either `kube-system` (operator-based) or `calico-system` (manifest-based). Both calico-node and calico-kube-controllers use the same fallback pattern: try kube-system first, fallback to calico-system on error.
- **APIVersion in ResourceRef**: Calico policy ResourceRefs include `APIVersion: "crd.projectcalico.org/v1"` to distinguish Calico-specific NetworkPolicy from standard Kubernetes NetworkPolicy (same Kind name, different API group).
- **GlobalNetworkPolicy is cluster-scoped**: Always listed without namespace filter. These policies apply across all namespaces and are a common source of broad traffic restrictions.
- **calico-kube-controllers as secondary health signal**: Beyond calico-node agents, the kube-controllers component manages policy and IP pool synchronization. Its health indicates whether Calico's control plane is functioning.
- **Empty InputSchema for check_calico_status**: No parameters needed because calico-node is a DaemonSet covering all nodes, and the namespace is automatically detected via fallback.
- **Critical only when total > 0 and ready == 0**: Guards against marking severity as Critical when no calico-node pods are found at all (which would be caught by the error path instead).

### Files Created

| File | Purpose |
|---|---|
| `pkg/tools/provider_calico.go` | ListCalicoPoliciesTool, CheckCalicoStatusTool, calicoNPGVR, calicoGNPGVR |

### Files Modified

| File | Action |
|---|---|
| `cmd/server/main.go` | Registered both tools conditionally under HasCalico |

## Dev Agent Record

### Agent Model Used
claude-opus-4-6

### Completion Notes List
- Policy tool uses CategoryPolicy, status tool uses CategoryMesh for findings
- Provider tag "calico" passed to NewToolResultResponse for both tools
- Namespace fallback pattern is independent per API call (calico-node and kube-controllers may be in different namespaces in edge cases)
- Node names in detail field help correlate with node-specific networking issues

### File List
- pkg/tools/provider_calico.go
- cmd/server/main.go
