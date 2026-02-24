# Story 11.5: Create NetworkPolicy Skill

Status: done

## Story

As an AI agent,
I want a guided workflow to create NetworkPolicies for service isolation,
so that I can generate Kubernetes NetworkPolicy manifests with correct selectors, ingress rules, and automatic DNS egress.

## Acceptance Criteria

1. The `create_network_policy` skill is always registered (NetworkPolicy is core Kubernetes, no CRDs required)
2. Step 1 verifies the target service exists and extracts its pod selector labels (sorted for deterministic output)
3. Step 2 checks for existing NetworkPolicies in the namespace and warns if any exist
4. Step 3 detects the CNI provider (Cilium or Calico) and notes compatibility
5. Step 4 generates a NetworkPolicy with: ingress rules (namespace-scoped with namespaceSelector when allowed_sources is set, or port-only when not), DNS egress rule (UDP+TCP port 53 to any namespace), and same-namespace egress rule
6. The generated policy includes both Ingress and Egress policyTypes
7. A DNS egress finding is automatically included explaining the port 53 requirement

## Tasks / Subtasks

- [x] Create pkg/skills/network_policy.go with NetworkPolicySkill struct (embeds skillBase, has hasCilium and hasCalico flags)
- [x] Implement Definition() returning skill name, description, and 4 parameters (target_service, namespace, allowed_sources, port)
- [x] Implement Execute() with 5-step workflow:
  - Step 1 (verify_service): GET service via dynamic client; extract spec.selector using unstructured.NestedStringMap; sort selector keys for deterministic YAML output; fall back to `app: {serviceName}` if no selector
  - Step 2 (check_existing_policies): List NetworkPolicies in namespace; warn if any exist
  - Step 3 (detect_cni): Report CNI provider (Cilium, Calico, or standard K8s) with compatibility note
  - Step 4 (generate_policy): Generate NetworkPolicy YAML with pod selector, ingress rules, DNS egress, and same-namespace egress
  - Step 5 (complete): Summary with concatenated manifests
- [x] Implement namespace-scoped ingress when allowed_sources is provided (namespaceSelector with kubernetes.io/metadata.name label per source)
- [x] Implement port-only ingress when no allowed_sources specified (allows from any source on the specified port)
- [x] Include automatic DNS egress rule (UDP+TCP port 53 to any namespace via empty namespaceSelector)
- [x] Include same-namespace egress rule (podSelector: {} to allow outbound within namespace)

## Dev Notes

### Always Registered

Unlike other skills, NetworkPolicySkill is always registered in SyncWithFeatures because NetworkPolicy is a core Kubernetes resource (networking.k8s.io/v1), not a CRD. The `hasCilium` and `hasCalico` flags are used only for informational reporting, not for gating registration.

### Selector Extraction and Sorting

The skill extracts the service's pod selector to use as the NetworkPolicy's podSelector. Keys are sorted using `sort.Strings(selectorKeys)` to ensure deterministic YAML output regardless of Go's random map iteration order. This makes generated manifests reproducible and diff-friendly.

If the service has no selector (unusual but possible), the skill falls back to `app: {serviceName}` as a reasonable default.

### Ingress Rule Variants

**With allowed_sources** (e.g., `"frontend,monitoring"`):
```yaml
ingress:
- from:
  - namespaceSelector:
      matchLabels:
        kubernetes.io/metadata.name: frontend
  ports:
  - protocol: TCP
    port: 80
- from:
  - namespaceSelector:
      matchLabels:
        kubernetes.io/metadata.name: monitoring
  ports:
  - protocol: TCP
    port: 80
```

**Without allowed_sources** (port-only):
```yaml
ingress:
- ports:
  - protocol: TCP
    port: 80
```

### Automatic DNS Egress

Every generated NetworkPolicy includes a DNS egress rule. Without this, pods subject to egress restrictions cannot resolve DNS names, which breaks virtually all Kubernetes networking. The rule allows UDP and TCP port 53 to any namespace:

```yaml
egress:
- to:
  - namespaceSelector: {}
  ports:
  - protocol: UDP
    port: 53
  - protocol: TCP
    port: 53
```

A separate DiagnosticFinding with category `dns` explains this inclusion, noting that "DNS (port 53) egress is required for pod name resolution."

### Same-Namespace Egress

A second egress rule allows all outbound traffic within the same namespace via an empty podSelector:
```yaml
- to:
  - podSelector: {}
```

This is a common default that prevents the policy from breaking intra-namespace communication while still restricting cross-namespace egress.

### CNI Provider Detection

The skill reports the detected CNI provider for informational purposes:
- **Cilium detected**: "Cilium detected; using standard K8s NetworkPolicy (compatible)"
- **Calico detected**: "Calico detected; using standard K8s NetworkPolicy (compatible)"
- **Neither**: "Using standard Kubernetes NetworkPolicy"

The generated policy uses standard Kubernetes NetworkPolicy API in all cases, ensuring compatibility across CNI providers.

### GVR Definition

```go
var npGVR = schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"}
```

## File List

| File | Action |
|---|---|
| `pkg/skills/network_policy.go` | Created |
