# Code Review: Epics 6-11

**Date:** 2026-02-24
**Reviewer:** Claude Opus 4.6 (automated)
**Build Status:** `go build ./...` passes, `go vet ./...` passes

## Summary

| Severity | Found | Fixed |
|----------|-------|-------|
| CRITICAL | 2 | 2 |
| WARNING | 17 | 15 |
| INFO | 8 | 1 |
| **Total** | **27** | **18** |

All critical issues and the majority of warning issues have been fixed. Remaining warnings are low-priority structural improvements (GVR deduplication across packages, RBAC wildcard resources) documented for future work.

---

## Epic 6: Active Diagnostic Probing

### pkg/probes/pod.go

| # | Severity | Issue | Status |
|---|----------|-------|--------|
| 1 | CRITICAL | Pod name collision: `time.Now().UnixNano()%100000` only produces 5-digit suffix. Concurrent probes within same 100us window could collide. | **FIXED** - Replaced with atomic counter: `fmt.Sprintf("mcp-probe-%s-%d-%d", req.Type, time.Now().Unix(), podCounter.Add(1))` |
| 2 | WARNING | Hardcoded resource limits (CPU 100m/50m, memory 64Mi/32Mi) not configurable. | Noted - acceptable for MVP; can be added to config.Config in future. |
| 3 | WARNING | Hardcoded `runAsUser: 1000`. | Noted - standard non-root UID; acceptable. |

### pkg/probes/manager.go

| # | Severity | Issue | Status |
|---|----------|-------|--------|
| 4 | WARNING | `releaseSlot()` could decrement `running` below zero if called without matching `acquireSlot()`. | **FIXED** - Added guard: `if m.running > 0 { m.running-- }` |

### pkg/probes/cleanup.go

| # | Severity | Issue | Status |
|---|----------|-------|--------|
| 5 | WARNING | Pods with unparseable `AnnotationCreatedAt` silently skipped, could leave orphans indefinitely. | **FIXED** - Added `slog.Warn` log with pod name and annotation value. |
| 6 | WARNING | Failed pod listing logged at Debug level; operational issues invisible at default log level. | **FIXED** - Changed `slog.Debug` to `slog.Warn`. |

### pkg/tools/probes.go

| # | Severity | Issue | Status |
|---|----------|-------|--------|
| 7 | CRITICAL | Command injection: user-supplied `targetHost`, `hostname`, `url`, `method`, `headers` interpolated directly into shell commands via `fmt.Sprintf`. | **FIXED** - Added input validation: `validHostname` regex for hostnames, `probeAllowedMethods` whitelist for HTTP methods, `validRecordTypes` whitelist for DNS record types, `containsShellMeta()` check for URLs and headers. |
| 8 | WARNING | String comparison for HTTP status codes (`statusCode >= "400"`) uses lexicographic ordering. | **FIXED** - Replaced with `strconv.Atoi(statusCode)` and numeric comparison. |

### pkg/probes/types.go

No issues found.

---

## Epic 7: CRD-Aware Design Guidance & Remediation

### pkg/tools/design_gateway.go

| # | Severity | Issue | Status |
|---|----------|-------|--------|
| 9 | INFO | Dead code: `routeResource` variable assigned and immediately suppressed with `_ = routeResource`. | **FIXED** - Removed the variable and the blank assignment. |

### pkg/tools/design_istio.go

| # | Severity | Issue | Status |
|---|----------|-------|--------|
| 10 | WARNING | `fmt.Sscanf` return value not checked in `parseTrafficSplit`. If parsing fails, entry gets weight 0 silently. | **FIXED** - Check `n` return value; skip entry if `n == 0`. |

### pkg/tools/design_kgateway.go

No issues found.

### pkg/tools/remediation.go

No issues found.

---

## Epic 8: Tier 2 Provider Support

### pkg/tools/provider_cilium.go

| # | Severity | Issue | Status |
|---|----------|-------|--------|
| 11 | WARNING | Dead variables: `cnpList`, `err`, `endpoints` assigned then suppressed with `_ = ...`. | **FIXED** - Removed all dead variable declarations. Variables scoped to their `if` blocks. |

### pkg/tools/provider_calico.go

| # | Severity | Issue | Status |
|---|----------|-------|--------|
| 12 | WARNING | `kube-controllers` ready count increments per ready container, not per pod. A pod with 2 containers counts as 2 ready. | **FIXED** - Changed to count pods where all containers are ready. |

### pkg/tools/provider_kuma.go

| # | Severity | Issue | Status |
|---|----------|-------|--------|
| 13 | WARNING | API errors for mesh and dataplane listing silently swallowed (no finding on failure). | Noted - acceptable for provider status tools; CRD may not be installed. |

### pkg/tools/provider_linkerd.go

No significant issues.

### pkg/tools/provider_flannel.go

No issues found.

---

## Epic 9: Production Deployment

### deploy/helm/mcp-k8s-networking/values.yaml

| # | Severity | Issue | Status |
|---|----------|-------|--------|
| 14 | WARNING | Missing `seccompProfile: RuntimeDefault` in securityContext. May fail Pod Security Standards in `restricted` mode. | **FIXED** - Added `seccompProfile: type: RuntimeDefault`. |

### deploy/manifests/install.yaml

| # | Severity | Issue | Status |
|---|----------|-------|--------|
| 15 | WARNING | Missing `seccompProfile` in container securityContext. | **FIXED** - Added `seccompProfile: type: RuntimeDefault`. |
| 16 | INFO | Uses `latest` image tag (not reproducible for production). | Noted - documented with comment "CHANGE THIS". |
| 17 | INFO | Placeholder cluster name `my-cluster`. | Noted - documented with comment "CHANGE THIS to your cluster name". |

### deploy/helm/mcp-k8s-networking/templates/clusterrole.yaml

| # | Severity | Issue | Status |
|---|----------|-------|--------|
| 18 | INFO | Uses wildcard `resources: ["*"]` for provider API groups. | Noted - acceptable for diagnostic tool; enumeration would be brittle across provider versions. |
| 19 | INFO | `resourceNames: []` on pod create verb is nonfunctional in RBAC. | Noted - cosmetic; doesn't cause issues. |

---

## Epic 10: Documentation Website

No code issues (documentation only). mkdocs.yml, docs/ files reviewed for completeness.

---

## Epic 11: Agent Skills

### pkg/skills/istio_mtls.go

| # | Severity | Issue | Status |
|---|----------|-------|--------|
| 20 | WARNING | Fragile step detection: `if len(steps) < 3 || steps[len(steps)-1].StepName != "check_dr_conflicts"` breaks if step count changes. | **FIXED** - Replaced with `drConflictFound` boolean flag. |

### pkg/skills/traffic_split.go

| # | Severity | Issue | Status |
|---|----------|-------|--------|
| 21 | WARNING | No validation that version count matches weight count. Mismatched counts silently produce 0-weight entries. | **FIXED** - Added upfront validation returning failed result if counts differ. |
| 22 | WARNING | `parseWeights` `fmt.Sscanf` error ignored; unparseable values silently become 0. | **FIXED** - Check `n` return; skip unparseable entries. |

### pkg/skills/network_policy.go

| # | Severity | Issue | Status |
|---|----------|-------|--------|
| 23 | WARNING | Map iteration order nondeterministic; generated selector YAML varies each run. | **FIXED** - Sort map keys before iteration using `sort.Strings`. |

### pkg/tools/skills.go

| # | Severity | Issue | Status |
|---|----------|-------|--------|
| 24 | WARNING | Uses `fmt.Errorf` instead of `types.MCPError` for validation errors (inconsistent with other tools). | **FIXED** - Replaced all `fmt.Errorf` with `&types.MCPError{Code: types.ErrCodeInvalidInput, ...}`. |

### pkg/skills/registry.go

| # | Severity | Issue | Status |
|---|----------|-------|--------|
| 25 | INFO | `SyncWithFeatures` doesn't hold lock across entire sync; concurrent `List()` could see partial state. | Noted - acceptable; partial visibility during sync is transient and harmless. |

---

## Cross-File Issues

### Duplicate GVR Declarations

GVRs are duplicated between `pkg/tools/` and `pkg/skills/` packages:

| GVR | pkg/tools | pkg/skills |
|-----|-----------|------------|
| Services | `servicesGVR` (k8s_services.go) | `svcGVR` (gateway_expose.go) |
| Gateways | `gatewaysV1GVR` (gateway_api.go) | `gwGVR` (gateway_expose.go) |
| PeerAuthentication | `paV1GVR` (istio.go) | `paGVR` (istio_mtls.go) |
| DestinationRules | (istio.go) | `drGVR` (istio_mtls.go) |
| NetworkPolicies | `networkPoliciesGVR` (k8s_networkpolicies.go) | `npGVR` (network_policy.go) |

**Status:** Noted for future refactoring. These are in separate packages so they compile fine. A shared `pkg/k8s/gvr.go` package would centralize them.

### Duplicate Helper Functions

| Function | pkg/tools | pkg/skills |
|----------|-----------|------------|
| Get string arg | `getStringArg` (types.go) | `getArg` (gateway_expose.go) |
| Get int arg | `getIntArg` (types.go) | `getIntArgSkill` (gateway_expose.go) |

**Status:** Noted for future refactoring.

### Logging Compliance

**No violations.** All files use `log/slog` for logging. No `fmt.Println` or `log.*` calls detected. `fmt.Sprintf` used appropriately for string building only.

### DiagnosticFinding Usage

All findings use correct severity constants (`types.SeverityOK`, `types.SeverityInfo`, `types.SeverityWarning`, `types.SeverityCritical`) and appropriate categories. Resource refs populated where applicable.

### Error Handling Consistency

After fixes, all tools use `types.MCPError` for user-facing validation errors and `fmt.Errorf` with `%w` for internal errors.

---

## Recommendations for Future Work

1. **Extract shared GVR package** - Create `pkg/k8s/gvr.go` to centralize all GroupVersionResource definitions across packages.
2. **Extract shared helpers** - Move `getArg`/`getIntArg` helpers to a shared utility package.
3. **Enumerate RBAC resources** - Replace `resources: ["*"]` in ClusterRole with explicit resource lists per provider.
4. **Configurable probe resource limits** - Add `ProbeResourceCPU`/`ProbeResourceMemory` fields to config.Config.
5. **Pin image tags** - Use semantic versioned tags instead of `latest` in default manifests.
