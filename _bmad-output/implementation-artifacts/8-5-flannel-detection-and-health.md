# Story 8.5: Flannel Detection and Health

Status: done

## Story

As an AI agent,
I want to check Flannel CNI health and configuration,
so that I can diagnose Flannel overlay network issues including DaemonSet pod health and backend configuration.

## Acceptance Criteria

1. The check_flannel_status tool detects Flannel via DaemonSet pod listing (not CRD-based) using the `app=flannel` label
2. Flannel pods are checked with namespace fallback: kube-flannel first, then kube-system
3. The tool reads the kube-flannel-cfg ConfigMap to report the network backend configuration
4. Pod readiness is tracked per-node with node names in the detail field
5. The tool is conditionally registered when Flannel is detected (HasFlannel)

## Tasks / Subtasks

- [x] Task 1: Implement CheckFlannelStatusTool (AC: 1, 2, 3, 4)
  - [x] Create struct embedding BaseTool
  - [x] Define InputSchema: empty properties (no parameters needed)
  - [x] Iterate namespace candidates `["kube-flannel", "kube-system"]` for pod discovery
  - [x] List pods with `app=flannel` label in each candidate namespace
  - [x] Break on first namespace with pods found
  - [x] Track per-pod readiness and collect node names
  - [x] Set severity: OK all ready, Warning partial, Critical none ready
  - [x] Include ResourceRef pointing to kube-flannel-ds DaemonSet
  - [x] Use local `podCounts` struct to track ready/total for the found namespace
  - [x] Report "Flannel DaemonSet not found" warning when no pods found in any namespace
  - [x] Read kube-flannel-cfg ConfigMap from candidate namespaces
  - [x] Extract net-conf.json from ConfigMap data and include in finding detail
- [x] Task 2: Register tool in main.go (AC: 5)
  - [x] Register inside `features.HasFlannel` conditional block
  - [x] Include "check_flannel_status" in flannelToolNames for unregistration

## Dev Notes

### Key Design Decisions

- **DaemonSet-based detection, not CRD-based**: Unlike other CNI/mesh providers, Flannel does not define custom CRDs. Detection is based on the presence of pods with the `app=flannel` label. The CRD discovery system detects Flannel through other means (DaemonSet presence) and sets `HasFlannel`.
- **Namespace fallback with break on first match**: The namespace candidate loop (`kube-flannel`, `kube-system`) breaks as soon as pods are found. This handles both standalone Flannel (kube-flannel namespace) and bundled installations (kube-system namespace).
- **Local podCounts struct**: A `type podCounts struct{ Ready, Total int }` is defined inline within the Run method rather than at package level, since it's only used for the nil check to determine if any namespace had Flannel pods.
- **ConfigMap for backend mode**: The `kube-flannel-cfg` ConfigMap contains `net-conf.json` which includes the backend type (vxlan, host-gw, wireguard, etc.) and network CIDR. This is key diagnostic information for network overlay issues.
- **CategoryConnectivity instead of CategoryMesh**: Flannel is a CNI plugin, not a service mesh. Findings use `CategoryConnectivity` to reflect its role as the base network layer.
- **Empty InputSchema**: No parameters needed because Flannel is always cluster-wide (DaemonSet) and the namespace is auto-detected.

### Files Created

| File | Purpose |
|---|---|
| `pkg/tools/provider_flannel.go` | CheckFlannelStatusTool with namespace fallback and ConfigMap reading |

### Files Modified

| File | Action |
|---|---|
| `cmd/server/main.go` | Registered CheckFlannelStatusTool conditionally under HasFlannel |

## Dev Agent Record

### Agent Model Used
claude-opus-4-6

### Completion Notes List
- Single tool providing two diagnostic signals: DaemonSet pod health and network configuration
- Uses CategoryConnectivity (not CategoryMesh) since Flannel is a CNI, not a mesh
- Provider tag "flannel" passed to NewToolResultResponse
- ConfigMap discovery uses the same namespace fallback pattern as pod discovery
- net-conf.json content includes subnet CIDR and backend type (e.g., vxlan, host-gw)
- No Flannel-specific GVRs needed since Flannel has no CRDs

### File List
- pkg/tools/provider_flannel.go
- cmd/server/main.go
