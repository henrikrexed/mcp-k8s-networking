# Story 1.2: Implement CRD Watch-Based Discovery

Status: ready-for-dev

## Story

As an AI agent,
I want the MCP server to detect installed networking CRDs in real-time,
so that diagnostic tools appear immediately when providers are installed and disappear when removed.

## Acceptance Criteria

1. On startup, initial CRD discovery detects all installed networking CRDs within 10 seconds (NFR3)
2. After startup, a CRD watch detects new networking CRD installations and triggers tool registration within one watch cycle (FR2)
3. When a networking CRD is removed, the watch detects the removal and triggers tool deregistration (FR4)
4. When Gateway API CRDs are not installed, no Gateway API tools appear in `tools/list` (FR5)
5. The server can report detected providers and their API group versions (FR3)
6. When the K8s API server is temporarily unavailable, the watch reconnects with exponential backoff (NFR16)
7. Features struct tracks: HasGatewayAPI, HasIstio, HasCilium, HasCalico, HasLinkerd, HasKuma, HasFlannel, HasKgateway

## Tasks / Subtasks

- [ ] Task 1: Rewrite pkg/discovery/discovery.go to use CRD watch (AC: 1, 2, 3, 6)
  - [ ] Replace 60s polling ticker with CRD watch on `apiextensions.k8s.io/v1` CustomResourceDefinitions
  - [ ] On startup: perform full CRD list scan to detect all installed CRDs
  - [ ] After startup: watch for CREATE and DELETE events on CRD resources
  - [ ] Parse CRD group names to detect networking providers (same logic as current `poll()`)
  - [ ] On watch error: reconnect with exponential backoff (initial 1s, max 30s)
  - [ ] On watch event: compute new Features, compare with old, fire onChange if different
- [ ] Task 2: Expand Features struct (AC: 7)
  - [ ] Add HasKuma, HasFlannel, HasKgateway boolean fields
  - [ ] Add detection for: `kuma.io`, `flannel.io` or flannel DaemonSet, `kgateway.dev` or `gateway.kgateway.dev`
  - [ ] Add ProviderVersions map[string]string to track detected API group versions
- [ ] Task 3: Add provider reporting capability (AC: 5)
  - [ ] Add `GetProviders() []ProviderInfo` method to Discovery
  - [ ] ProviderInfo struct: Name, APIGroup, Version, Detected bool
  - [ ] This enables FR3 (report detected providers to agent)
- [ ] Task 4: Update cmd/server/main.go onChange callback (AC: 2, 3)
  - [ ] Ensure onChange callback registers/deregisters tools AND calls MCP server SyncTools (from Story 1.1)
  - [ ] Log provider changes at info level
- [ ] Task 5: Add typed clientset for CRD watch (AC: 1)
  - [ ] The CRD watch needs `apiextensions-apiserver` client or use dynamic client watch
  - [ ] Add `k8s.io/apiextensions-apiserver` dependency if using typed CRD client, OR use dynamic client watch on CRD GVR
  - [ ] Preferred: use dynamic client watch to avoid extra dependency

## Dev Notes

### Current Implementation (to be replaced)

`pkg/discovery/discovery.go` currently:
- Uses `discovery.DiscoveryInterface` to call `ServerGroups()` every 60 seconds
- Detects: gateway.networking.k8s.io, networking.istio.io, security.istio.io, cilium.io, crd.projectcalico.org, linkerd.io
- Missing: kuma.io, flannel, kgateway.dev
- Fires `OnChangeFunc(Features)` when features change
- Thread-safe via `sync.RWMutex`

### Architecture Constraints

- **Watch, not poll**: Architecture document explicitly specifies watch-based CRD discovery to reduce API server pressure
- **Single watch connection**: Watch `apiextensions.k8s.io/v1` CRDs — one watch covers all networking CRD installations
- **Dynamic client preferred**: Use the existing dynamic client from `pkg/k8s/client.go` (Clients.Dynamic) to set up the watch, avoiding a new dependency on `apiextensions-apiserver`
- **Graceful degradation**: Missing CRDs = tools not registered. Not an error.

### Watch Implementation Pattern

```go
// Use dynamic client to watch CRDs
crdGVR := schema.GroupVersionResource{
    Group:    "apiextensions.k8s.io",
    Version:  "v1",
    Resource: "customresourcedefinitions",
}
watcher, err := clients.Dynamic.Resource(crdGVR).Watch(ctx, metav1.ListOptions{})
// Process events from watcher.ResultChan()
// On ADDED/DELETED: extract spec.group from the CRD object, update Features
```

### K8s Client Available

`pkg/k8s/client.go` provides:
```go
type Clients struct {
    Dynamic   dynamic.Interface
    Discovery discovery.DiscoveryInterface
    Clientset kubernetes.Interface
}
```
Use `Clients.Dynamic` for the CRD watch. Keep `Clients.Discovery` for the initial startup scan (ServerGroups is fast for initial detection).

### CRD Group Detection Map

| API Group | Provider | Feature Flag |
|---|---|---|
| `gateway.networking.k8s.io` | Gateway API | HasGatewayAPI |
| `networking.istio.io` | Istio | HasIstio |
| `security.istio.io` | Istio | HasIstio |
| `cilium.io` | Cilium | HasCilium |
| `crd.projectcalico.org` | Calico | HasCalico |
| `linkerd.io` | Linkerd | HasLinkerd |
| `kuma.io` | Kuma | HasKuma |
| `kgateway.dev` or `gateway.kgateway.dev` | kgateway | HasKgateway |

Flannel detection is special — it typically doesn't have CRDs. Detect via DaemonSet `kube-flannel-ds` in `kube-flannel` or `kube-system` namespace.

### Files to Modify

| File | Action |
|---|---|
| `pkg/discovery/discovery.go` | Full rewrite — watch-based |
| `cmd/server/main.go` | Update Discovery initialization (pass Dynamic client) |

### Dependencies

- Depends on Story 1.1 being complete (MCP server SyncTools must support tool removal)
- Uses existing `pkg/k8s/client.go` Clients struct (no changes needed)

### Testing

- Build must pass: `make build`
- `go vet ./...` must pass
- Manual test: start server, verify initial CRD discovery logs detected providers

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#CRD Discovery Specification]
- [Source: _bmad-output/planning-artifacts/architecture.md#Cluster Data Access]
- [Source: pkg/discovery/discovery.go - current polling implementation]
- [Source: pkg/k8s/client.go - Clients struct]

## Dev Agent Record

### Agent Model Used

### Debug Log References

### Completion Notes List

### File List
