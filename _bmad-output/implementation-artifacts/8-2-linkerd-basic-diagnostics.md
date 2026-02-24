# Story 8.2: Linkerd Basic Diagnostics

Status: done

## Story

As an AI agent,
I want to check Linkerd service mesh health and status,
so that I can diagnose control plane issues, proxy injection configuration, and service profile availability.

## Acceptance Criteria

1. The check_linkerd_status tool checks Linkerd control plane pods in the linkerd namespace using the `linkerd.io/control-plane-component` label
2. The tool tracks individual control plane component readiness (e.g., destination, identity, proxy-injector)
3. The tool checks namespace injection status by scanning the `linkerd.io/inject` annotation
4. The tool counts ServiceProfile resources via linkerd.io/v1alpha2 CRD, optionally filtered by namespace
5. The tool is conditionally registered when Linkerd CRDs are detected (HasLinkerd)

## Tasks / Subtasks

- [x] Task 1: Define Linkerd GVR (AC: 4)
  - [x] Define `linkerdSPGVR`: linkerd.io/v1alpha2/serviceprofiles
- [x] Task 2: Implement CheckLinkerdStatusTool (AC: 1, 2, 3, 4)
  - [x] Create struct embedding BaseTool
  - [x] Define InputSchema: namespace (optional, for filtering service profiles)
  - [x] List control plane pods with label `linkerd.io/control-plane-component` in linkerd namespace
  - [x] Track per-component readiness in a `map[string]bool` keyed by component label value
  - [x] Count total and ready pods, set severity: OK when all ready, Warning when partial, Critical when none ready
  - [x] Include ResourceRef pointing to linkerd Namespace
  - [x] Scan all namespaces for `linkerd.io/inject=enabled` annotation, report count
  - [x] List ServiceProfiles cluster-wide or by namespace, report count
- [x] Task 3: Register tool in main.go (AC: 5)
  - [x] Register inside `features.HasLinkerd` conditional block
  - [x] Include "check_linkerd_status" in linkerdToolNames for unregistration

## Dev Notes

### Key Design Decisions

- **Component-level tracking**: Uses the `linkerd.io/control-plane-component` label to identify individual components (destination, identity, proxy-injector, etc.) and tracks each component's readiness in a map. The Detail field includes `components={map}` for full visibility.
- **Annotation-based injection detection**: Linkerd uses the `linkerd.io/inject: enabled` annotation on namespaces (unlike Istio's label-based approach). The tool scans all namespaces and counts those with injection enabled.
- **Three-tier severity**: OK (all ready), Warning (partial), Critical (none ready). This differs from Kuma which only has OK/Critical, reflecting that Linkerd's multi-component control plane can be partially functional.
- **linkerd namespace hardcoded**: Linkerd's control plane runs in the `linkerd` namespace by convention. No fallback needed.
- **ServiceProfile scoping**: When namespace is provided, service profiles are listed in that namespace. This is useful for checking if service-level configuration exists for a specific application namespace.

### Files Created

| File | Purpose |
|---|---|
| `pkg/tools/provider_linkerd.go` | CheckLinkerdStatusTool, linkerdSPGVR |

### Files Modified

| File | Action |
|---|---|
| `cmd/server/main.go` | Registered CheckLinkerdStatusTool conditionally under HasLinkerd |

## Dev Agent Record

### Agent Model Used
claude-opus-4-6

### Completion Notes List
- Single tool providing three diagnostic signals: control plane health, injection status, service profile count
- Component map in Detail field allows agents to identify which specific component is unhealthy
- All findings use CategoryMesh
- Provider tag "linkerd" passed to NewToolResultResponse

### File List
- pkg/tools/provider_linkerd.go
- cmd/server/main.go
