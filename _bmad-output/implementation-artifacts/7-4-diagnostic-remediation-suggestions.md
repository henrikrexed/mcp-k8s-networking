# Story 7.4: Diagnostic Remediation Suggestions

Status: done

## Story

As an AI agent,
I want to get actionable remediation suggestions for identified diagnostic issues,
so that I can guide users through fixing common Kubernetes networking problems with specific commands and YAML patches.

## Acceptance Criteria

1. The suggest_remediation tool accepts an issue_type and returns structured remediation steps with kubectl commands and YAML examples
2. Supported issue types: missing_endpoints/no_matching_pods, network_policy_blocking, dns_failure, mtls_conflict, route_misconfigured, missing_reference_grant, gateway_listener_conflict, sidecar_missing, weight_mismatch
3. Each remediation includes numbered steps, kubectl verification commands, and where applicable, corrective YAML
4. Unknown issue types return general diagnostic guidance rather than an error
5. The tool is always registered (not conditional on CRD discovery)

## Tasks / Subtasks

- [x] Task 1: Implement SuggestRemediationTool (AC: 1, 2, 3, 4)
  - [x] Create struct embedding BaseTool
  - [x] Define InputSchema: issue_type (required), resource_kind, resource_name, namespace, additional_context
  - [x] Build ResourceRef from parameters for finding association
  - [x] Implement switch on lowercase issue_type with remediation cases:
    - [x] "missing_endpoints" / "no_matching_pods": selector mismatch diagnosis, kubectl label verification
    - [x] "network_policy_blocking": ingress/egress rule review, example NetworkPolicy YAML with namespace selector
    - [x] "dns_failure": CoreDNS health check steps, DNS name format verification
    - [x] "mtls_conflict": PeerAuthentication/DestinationRule alignment, example DestinationRule with ISTIO_MUTUAL
    - [x] "route_misconfigured": backend/parentRef verification, status condition check commands
    - [x] "missing_reference_grant": ReferenceGrant YAML template for cross-namespace routing
    - [x] "gateway_listener_conflict": port/protocol/hostname collision diagnosis
    - [x] "sidecar_missing": namespace labeling command, rollout restart command
    - [x] "weight_mismatch": weight sum explanation, corrected VirtualService weight example
  - [x] Implement default case with general diagnostic steps using resource parameters
- [x] Task 2: Register tool in main.go (AC: 5)
  - [x] Register SuggestRemediationTool unconditionally (always available)

## Dev Notes

### Key Design Decisions

- **Switch-based dispatch**: Simple switch statement on issue_type rather than a registry pattern. Each case is self-contained with its own finding severity, category, detail, and suggestion. This keeps the tool easy to extend.
- **Remediation in Detail, commands in Suggestion**: The `Detail` field contains numbered remediation steps explaining what to do. The `Suggestion` field contains the actual kubectl commands or YAML to copy. This separation allows agents to present steps and commands differently.
- **additional_context integration**: For network_policy_blocking and missing_reference_grant, the additional_context parameter is interpolated into the generated YAML (e.g., as source namespace), making the output more specific to the user's situation.
- **No error on unknown issue_type**: The default case provides generic diagnostic steps (get, describe, events) rather than returning an error. This ensures the tool is always useful even with novel issue types.
- **Always available**: Unlike design tools which depend on CRD presence, remediation is always registered since it provides general Kubernetes networking guidance regardless of installed components.
- **Severity mapping by issue type**: dns_failure and mtls_conflict use SeverityCritical (service-affecting). Others use SeverityWarning (degraded but partially functional). The default case uses SeverityInfo (advisory).

### Files Created

| File | Purpose |
|---|---|
| `pkg/tools/remediation.go` | SuggestRemediationTool implementation with 9 issue type handlers + default |

### Files Modified

| File | Action |
|---|---|
| `cmd/server/main.go` | Registered SuggestRemediationTool unconditionally |

## Dev Agent Record

### Agent Model Used
claude-opus-4-6

### Completion Notes List
- 9 specific issue types plus a generic default handler
- Each handler produces exactly one DiagnosticFinding
- Generated YAML in suggestions includes namespace and resource name interpolation
- The tool pairs well with diagnostic tools that identify issues (e.g., validate_istio_config, scan_gateway_misconfigs)
- Uses CategoryRouting, CategoryPolicy, CategoryDNS, CategoryTLS, CategoryMesh for appropriate finding categorization

### File List
- pkg/tools/remediation.go
- cmd/server/main.go
