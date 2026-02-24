# Story 6.1: Ephemeral Pod Manager

Status: done

## Story

As an AI agent,
I want a managed ephemeral pod lifecycle system,
so that I can run active network probes inside the cluster with proper concurrency control, security hardening, and automatic cleanup.

## Acceptance Criteria

1. The probe manager creates ephemeral pods with restricted security contexts (runAsNonRoot, drop ALL capabilities, seccomp RuntimeDefault, resource limits)
2. Concurrent probe execution is limited by a configurable maximum, returning MCPError with ErrCodeProbeLimitReached when exceeded
3. Each probe pod has a context-scoped timeout that cancels the probe if exceeded
4. Orphaned probe pods older than 5 minutes are automatically cleaned up by a background goroutine
5. Pod names are unique across concurrent probes using an atomic counter
6. Probe pods are always deleted after execution, even on failure or timeout

## Tasks / Subtasks

- [x] Task 1: Define probe types and constants (AC: 1)
  - [x] Create `pkg/probes/types.go` with ProbeType (connectivity, dns, http), ProbeRequest, ProbeResult structs
  - [x] Define label constants: LabelManagedBy, LabelProbeType, AnnotationCreatedAt
- [x] Task 2: Implement concurrency-controlled Manager (AC: 2, 3)
  - [x] Create `pkg/probes/manager.go` with Manager struct holding sync.Mutex, running counter, stopCh
  - [x] Implement acquireSlot/releaseSlot with MaxConcurrentProbes from config
  - [x] Implement Execute method: acquire slot, create pod, wait for completion, collect logs, delete pod
  - [x] Wrap probe execution in context.WithTimeout for per-probe timeout
  - [x] Return MCPError with ErrCodeProbeTimeout when context deadline exceeded
  - [x] Always delete pod in deferred cleanup using background context with 10s timeout
- [x] Task 3: Implement secure pod creation and lifecycle (AC: 1, 5, 6)
  - [x] Create `pkg/probes/pod.go` with createProbePod using restricted SecurityContext
  - [x] Set runAsNonRoot=true, runAsUser=1000, allowPrivilegeEscalation=false, readOnlyRootFilesystem=true
  - [x] Drop ALL capabilities, set SeccompProfile to RuntimeDefault
  - [x] Set resource limits (100m CPU, 64Mi memory) and requests (50m CPU, 32Mi memory)
  - [x] Use atomic.Int64 counter for unique pod names: `mcp-probe-{type}-{unix}-{counter}`
  - [x] Implement waitForPod using Kubernetes watch API for terminal state detection
  - [x] Implement collectLogs with io.LimitReader capped at 64KB
  - [x] Implement deleteProbePod for cleanup
- [x] Task 4: Implement periodic orphan cleanup (AC: 4)
  - [x] Create `pkg/probes/cleanup.go` with cleanupLoop goroutine running every 60 seconds
  - [x] Implement cleanupOrphans: list pods by managed-by label, delete those exceeding 5-minute TTL
  - [x] Parse creation timestamp from AnnotationCreatedAt annotation
  - [x] Run initial cleanup on manager startup
  - [x] Cleanup loop exits on context cancellation or stopCh signal

## Dev Notes

### Key Design Decisions

- **Concurrency control via mutex + counter** rather than semaphore channel: simpler implementation, acquireSlot returns typed MCPError directly. The mutex protects a simple `running` int counter checked against `cfg.MaxConcurrentProbes`.
- **Background context for pod deletion**: The deferred deleteProbePod uses `context.Background()` with a 10-second timeout rather than the probe context, ensuring cleanup happens even when the probe context is cancelled.
- **Atomic counter for pod names**: `sync/atomic.Int64` provides lock-free unique suffix generation. Combined with Unix timestamp and probe type, pod names are globally unique: `mcp-probe-connectivity-1706000000-1`.
- **Watch-based pod completion**: Uses Kubernetes watch API (`FieldSelector: metadata.name=podName`) instead of polling, providing immediate notification when the pod reaches Succeeded or Failed phase.
- **64KB log limit**: `io.LimitReader(stream, 64*1024)` prevents memory issues from verbose probe output. Logs are collected from the "probe" container only.
- **TTL-based orphan cleanup**: Annotation `mcp-k8s-networking/created-at` stores RFC3339 creation time. The 5-minute TTL covers slow probes while preventing accumulation from crashes.
- **RestartPolicy: Never**: Probe pods never restart; terminal state (Succeeded/Failed) is the only outcome.

### Files Created

| File | Purpose |
|---|---|
| `pkg/probes/types.go` | ProbeType, ProbeRequest, ProbeResult, label/annotation constants |
| `pkg/probes/manager.go` | Manager struct, NewManager, Execute, acquireSlot/releaseSlot, Stop |
| `pkg/probes/pod.go` | createProbePod, deleteProbePod, waitForPod, collectLogs, podCounter |
| `pkg/probes/cleanup.go` | cleanupLoop, cleanupOrphans, probeTTL/cleanupInterval constants |

### Files Modified

| File | Action |
|---|---|
| `cmd/server/main.go` | Initialize ProbeManager with `probes.NewManager(ctx, cfg, clients)` |

## Dev Agent Record

### Agent Model Used
claude-opus-4-6

### Completion Notes List
- All probe pods use RestartPolicy: Never
- SecurityContext is maximally restrictive: non-root, no caps, read-only FS, seccomp
- Manager integrates with config.Config for ProbeNamespace and MaxConcurrentProbes
- Manager integrates with k8s.Clients for Kubernetes API access
- Manager integrates with types.MCPError for structured error responses

### File List
- pkg/probes/types.go
- pkg/probes/manager.go
- pkg/probes/pod.go
- pkg/probes/cleanup.go
- cmd/server/main.go
