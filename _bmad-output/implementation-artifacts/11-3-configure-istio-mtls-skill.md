# Story 11.3: Configure Istio mTLS Skill

Status: done

## Story

As an AI agent,
I want a guided workflow to configure mTLS for a namespace using Istio,
so that I can generate PeerAuthentication and DestinationRule manifests with conflict detection and sidecar injection validation.

## Acceptance Criteria

1. The `configure_istio_mtls` skill is registered when Istio CRDs are detected
2. Step 1 checks sidecar injection by inspecting the namespace's `istio-injection` label
3. Step 2 checks for existing PeerAuthentication resources and warns about them
4. Step 3 checks for conflicting DestinationRule TLS settings (non-ISTIO_MUTUAL mode when STRICT is requested)
5. Step 4 generates a PeerAuthentication manifest with the specified mode (STRICT or PERMISSIVE)
6. Step 5 generates a DestinationRule with ISTIO_MUTUAL TLS mode (only when mode is STRICT)
7. Step 6 provides a summary with manifest count and concatenated output

## Tasks / Subtasks

- [x] Create pkg/skills/istio_mtls.go with ConfigureMTLSSkill struct (embeds skillBase)
- [x] Define package-level GVRs for PeerAuthentication (security.istio.io/v1) and DestinationRule (networking.istio.io/v1)
- [x] Implement Definition() returning skill name, description, required CRDs (security.istio.io), and 2 parameters (namespace, mode)
- [x] Implement Execute() with 6-step workflow:
  - Step 1 (check_sidecar_injection): GET namespace via Clientset, check for `istio-injection=enabled` label; warn if missing
  - Step 2 (check_current_mtls): List PeerAuthentication in namespace; warn if existing policies found with their current mode
  - Step 3 (check_dr_conflicts): List DestinationRules in namespace; flag conflicts where TLS mode is not ISTIO_MUTUAL and requested mode is STRICT (uses boolean drConflictFound flag)
  - Step 4 (generate_peer_auth): Generate PeerAuthentication YAML with namespace-scoped mtls mode (STRICT or PERMISSIVE)
  - Step 5 (generate_destination_rule): Conditional on STRICT mode; generates DestinationRule with `host: "*.{ns}.svc.cluster.local"` and `tls.mode: ISTIO_MUTUAL`
  - Step 6 (complete): Summary with manifest count

## Dev Notes

### 6-Step Workflow

| Step | Name | Action | Fail Behavior |
|---|---|---|---|
| 1 | check_sidecar_injection | Check namespace label | Warning if not enabled (does not fail) |
| 2 | check_current_mtls | List existing PeerAuthentication | Warning per existing PA with current mode |
| 3 | check_dr_conflicts | Check DestinationRule TLS conflicts | Warning/critical for conflicting TLS modes |
| 4 | generate_peer_auth | Generate PeerAuthentication YAML | Always succeeds |
| 5 | generate_destination_rule | Generate DestinationRule YAML | Only for STRICT mode |
| 6 | complete | Summary | Always passes |

### Conflict Detection Logic

The DestinationRule conflict check iterates all DRs in the namespace and flags any where:
1. `spec.trafficPolicy.tls.mode` is set (non-empty)
2. The mode is NOT `ISTIO_MUTUAL`
3. The requested mTLS mode is `STRICT`

This detects configurations like `DISABLE` or `SIMPLE` TLS that would conflict with strict mTLS enforcement. The `drConflictFound` boolean ensures that the "no conflicts" step result is only added when no conflicts were found.

### PeerAuthentication vs DestinationRule

- **PeerAuthentication**: Controls the server-side mTLS policy (what the pod accepts)
- **DestinationRule**: Controls the client-side TLS policy (what the client sends)
- For STRICT mode, both are needed to ensure all traffic is encrypted
- For PERMISSIVE mode, only PeerAuthentication is generated (accepts both plaintext and mTLS)

### Unstructured Field Access

The skill uses `unstructured.NestedString` to extract fields from dynamic client results:
- `pa.Object, "spec", "mtls", "mode"` for PeerAuthentication mode
- `dr.Object, "spec", "trafficPolicy", "tls", "mode"` for DestinationRule TLS mode

### GVR Definitions

```go
var paGVR = schema.GroupVersionResource{Group: "security.istio.io", Version: "v1", Resource: "peerauthentications"}
var drGVR = schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1", Resource: "destinationrules"}
```

## File List

| File | Action |
|---|---|
| `pkg/skills/istio_mtls.go` | Created |
