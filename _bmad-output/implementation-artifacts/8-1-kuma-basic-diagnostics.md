# Story 8.1: Kuma Basic Diagnostics

Status: done

## Story

As an AI agent,
I want to check Kuma service mesh health and status,
so that I can diagnose control plane issues, mesh configuration, and data plane proxy status.

## Acceptance Criteria

1. The check_kuma_status tool checks Kuma control plane pod health in kuma-system namespace
2. The tool counts Kuma meshes via the kuma.io/v1alpha1 meshes CRD
3. The tool counts data plane proxies via the kuma.io/v1alpha1 dataplanes CRD, optionally filtered by namespace
4. Control plane readiness is reflected in finding severity: OK when all ready, Critical when none ready
5. The tool is conditionally registered when Kuma CRDs are detected (HasKuma)

## Tasks / Subtasks

- [x] Task 1: Define Kuma GVRs (AC: 2, 3)
  - [x] Define `kumaMeshGVR`: kuma.io/v1alpha1/meshes
  - [x] Define `kumaDataplaneGVR`: kuma.io/v1alpha1/dataplanes
- [x] Task 2: Implement CheckKumaStatusTool (AC: 1, 2, 3, 4)
  - [x] Create struct embedding BaseTool
  - [x] Define InputSchema: namespace (optional, for filtering dataplanes)
  - [x] Check control plane pods with label selector `app=kuma-control-plane` in kuma-system namespace
  - [x] Count ready containers across control plane pods
  - [x] Set severity: OK when ready > 0, Critical when ready == 0
  - [x] Include ResourceRef pointing to kuma-control-plane Deployment
  - [x] List meshes cluster-wide via kumaMeshGVR, show count and names via `meshNames()` helper
  - [x] List dataplanes cluster-wide or filtered by namespace, show count
- [x] Task 3: Register tool in main.go (AC: 5)
  - [x] Register inside `features.HasKuma` conditional block
  - [x] Include "check_kuma_status" in kumaToolNames for unregistration

## Dev Notes

### Key Design Decisions

- **Container-level readiness check**: Iterates over `ContainerStatuses` to count ready containers rather than checking pod phase. This provides more accurate health information for multi-container control plane pods.
- **meshNames helper**: Extracts mesh names from the unstructured list for the Detail field, giving agents visibility into which meshes exist (e.g., "default", "production").
- **Namespace-scoped dataplane listing**: When namespace is provided, dataplanes are listed in that namespace only. When empty, cluster-wide listing gives total proxy count. This mirrors how Kuma admins typically check mesh status.
- **kuma-system hardcoded namespace**: Kuma's control plane always runs in kuma-system by convention. No namespace fallback needed unlike CNI providers.
- **Graceful error handling**: Control plane pod listing failure produces a warning finding rather than returning an error. Mesh and dataplane listing failures are silently skipped (no finding added).

### Files Created

| File | Purpose |
|---|---|
| `pkg/tools/provider_kuma.go` | CheckKumaStatusTool, kumaMeshGVR, kumaDataplaneGVR, meshNames helper |

### Files Modified

| File | Action |
|---|---|
| `cmd/server/main.go` | Registered CheckKumaStatusTool conditionally under HasKuma |

## Dev Agent Record

### Agent Model Used
claude-opus-4-6

### Completion Notes List
- Single tool providing three diagnostic signals: control plane health, mesh count, dataplane count
- All findings use CategoryMesh
- Provider tag "kuma" passed to NewToolResultResponse for response metadata
- meshNames helper is reusable for any unstructured list

### File List
- pkg/tools/provider_kuma.go
- cmd/server/main.go
