package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/isitobservable/k8s-networking-mcp/pkg/types"
)

// --- suggest_remediation ---

type SuggestRemediationTool struct{ BaseTool }

func (t *SuggestRemediationTool) Name() string { return "suggest_remediation" }
func (t *SuggestRemediationTool) Description() string {
	return "Suggest remediations for identified diagnostic issues with actionable YAML fixes"
}
func (t *SuggestRemediationTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"issue_type": map[string]interface{}{
				"type":        "string",
				"description": "Type of issue: missing_endpoints, no_matching_pods, network_policy_blocking, dns_failure, mtls_conflict, route_misconfigured, missing_reference_grant, gateway_listener_conflict, sidecar_missing, weight_mismatch",
			},
			"resource_kind": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes resource kind (e.g., Service, HTTPRoute, VirtualService)",
			},
			"resource_name": map[string]interface{}{
				"type":        "string",
				"description": "Name of the affected resource",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Namespace of the affected resource",
			},
			"additional_context": map[string]interface{}{
				"type":        "string",
				"description": "Additional context about the issue (e.g., specific error, conflicting resource)",
			},
		},
		"required": []string{"issue_type"},
	}
}

func (t *SuggestRemediationTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	issueType := getStringArg(args, "issue_type", "")
	resourceKind := getStringArg(args, "resource_kind", "")
	resourceName := getStringArg(args, "resource_name", "")
	ns := getStringArg(args, "namespace", "default")
	additionalCtx := getStringArg(args, "additional_context", "")

	findings := make([]types.DiagnosticFinding, 0, 3)

	ref := &types.ResourceRef{
		Kind:      resourceKind,
		Namespace: ns,
		Name:      resourceName,
	}

	switch strings.ToLower(issueType) {
	case "missing_endpoints", "no_matching_pods":
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityWarning,
			Category: types.CategoryRouting,
			Resource: ref,
			Summary:  fmt.Sprintf("Service %s/%s has no matching endpoints", ns, resourceName),
			Detail: fmt.Sprintf(`Remediation steps:
1. Verify pods exist: kubectl get pods -n %s -l app=%s
2. Check selector labels match pod template labels
3. Ensure pods are in Running state
4. Verify the service port matches a container port`, ns, resourceName),
			Suggestion: fmt.Sprintf(`# Verify selector match
kubectl get svc %s -n %s -o jsonpath='{.spec.selector}'
kubectl get pods -n %s --show-labels

# Check endpoints
kubectl get endpoints %s -n %s`, resourceName, ns, ns, resourceName, ns),
		})

	case "network_policy_blocking":
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityWarning,
			Category: types.CategoryPolicy,
			Resource: ref,
			Summary:  fmt.Sprintf("NetworkPolicy may be blocking traffic to %s/%s", ns, resourceName),
			Detail: `Remediation steps:
1. Review existing NetworkPolicies for the namespace
2. Ensure ingress rules allow traffic from the source
3. Verify egress rules allow DNS (port 53) and the target port`,
			Suggestion: fmt.Sprintf(`# Example: Allow ingress from specific namespace
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-ingress-%s
  namespace: %s
spec:
  podSelector:
    matchLabels:
      app: %s
  policyTypes:
  - Ingress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          kubernetes.io/metadata.name: %s  # source namespace
    ports:
    - protocol: TCP
      port: 80  # adjust to your service port`, resourceName, ns, resourceName, additionalCtx),
		})

	case "dns_failure":
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityCritical,
			Category: types.CategoryDNS,
			Resource: ref,
			Summary:  "DNS resolution failure",
			Detail: `Remediation steps:
1. Verify CoreDNS pods are running: kubectl get pods -n kube-system -l k8s-app=kube-dns
2. Check CoreDNS logs for errors: kubectl logs -n kube-system -l k8s-app=kube-dns
3. Verify the service exists in the expected namespace
4. Check NetworkPolicies are not blocking DNS traffic (UDP/TCP port 53)`,
			Suggestion: `# Check CoreDNS health
kubectl get pods -n kube-system -l k8s-app=kube-dns
kubectl logs -n kube-system -l k8s-app=kube-dns --tail=50

# Verify service DNS name format: <service>.<namespace>.svc.cluster.local`,
		})

	case "mtls_conflict":
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityCritical,
			Category: types.CategoryTLS,
			Resource: ref,
			Summary:  fmt.Sprintf("mTLS configuration conflict in %s", ns),
			Detail: `Remediation steps:
1. Review PeerAuthentication policies for the namespace
2. Check DestinationRule TLS settings for conflicting modes
3. Ensure STRICT mTLS has matching DestinationRule with ISTIO_MUTUAL`,
			Suggestion: fmt.Sprintf(`# Align DestinationRule TLS with PeerAuthentication
apiVersion: networking.istio.io/v1
kind: DestinationRule
metadata:
  name: %s-mtls
  namespace: %s
spec:
  host: %s
  trafficPolicy:
    tls:
      mode: ISTIO_MUTUAL`, resourceName, ns, resourceName),
		})

	case "route_misconfigured":
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityWarning,
			Category: types.CategoryRouting,
			Resource: ref,
			Summary:  fmt.Sprintf("Route misconfiguration on %s %s/%s", resourceKind, ns, resourceName),
			Detail: `Remediation steps:
1. Verify backend service references exist
2. Check port numbers match the target service
3. Verify parentRef (Gateway) exists and has matching listeners
4. Check for conflicting route rules`,
			Suggestion: fmt.Sprintf(`# Verify backend service exists
kubectl get svc -n %s
# Check route status
kubectl get %s %s -n %s -o yaml | grep -A 20 status`, ns, strings.ToLower(resourceKind), resourceName, ns),
		})

	case "missing_reference_grant":
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityWarning,
			Category: types.CategoryRouting,
			Resource: ref,
			Summary:  "Cross-namespace reference missing ReferenceGrant",
			Detail:   "A route references a backend service in a different namespace without a ReferenceGrant allowing the cross-namespace reference.",
			Suggestion: fmt.Sprintf(`# Create ReferenceGrant in the target service namespace
apiVersion: gateway.networking.k8s.io/v1beta1
kind: ReferenceGrant
metadata:
  name: allow-cross-ns-%s
  namespace: %s  # namespace of the target service
spec:
  from:
  - group: gateway.networking.k8s.io
    kind: HTTPRoute
    namespace: %s  # namespace of the route
  to:
  - group: ""
    kind: Service`, resourceName, ns, additionalCtx),
		})

	case "gateway_listener_conflict":
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityWarning,
			Category: types.CategoryRouting,
			Resource: ref,
			Summary:  "Gateway listener conflict (port/protocol collision)",
			Detail: `Remediation steps:
1. Check for multiple listeners on the same port with different protocols
2. Ensure hostnames are unique across listeners on the same port
3. Review Gateway status conditions for conflict details`,
			Suggestion: `# Check Gateway status for conflict details
kubectl get gateway <gateway-name> -n <namespace> -o yaml | grep -A 30 status`,
		})

	case "sidecar_missing":
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityWarning,
			Category: types.CategoryMesh,
			Resource: ref,
			Summary:  fmt.Sprintf("Sidecar injection missing for workloads in %s", ns),
			Detail: `Remediation steps:
1. Label the namespace for injection: kubectl label namespace <ns> istio-injection=enabled
2. Restart deployments to trigger injection: kubectl rollout restart deployment -n <ns>
3. Verify injection: kubectl get pods -n <ns> -o jsonpath='{.items[*].spec.containers[*].name}'`,
			Suggestion: fmt.Sprintf(`# Enable sidecar injection
kubectl label namespace %s istio-injection=enabled --overwrite
# Restart deployments
kubectl rollout restart deployment -n %s`, ns, ns),
		})

	case "weight_mismatch":
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityCritical,
			Category: types.CategoryRouting,
			Resource: ref,
			Summary:  "Traffic split weights do not sum to 100%",
			Detail:   "VirtualService or HTTPRoute weight configuration is invalid. Weights must sum to exactly 100.",
			Suggestion: `# Fix weights in VirtualService to sum to 100
# Example: 80/20 split
spec:
  http:
  - route:
    - destination:
        host: my-service
        subset: v1
      weight: 80
    - destination:
        host: my-service
        subset: v2
      weight: 20`,
		})

	default:
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryRouting,
			Summary:  fmt.Sprintf("General remediation guidance for issue: %s", issueType),
			Detail: fmt.Sprintf(`General diagnostic steps:
1. Check resource exists: kubectl get %s %s -n %s
2. Review resource status: kubectl describe %s %s -n %s
3. Check events: kubectl get events -n %s --sort-by='.lastTimestamp'
4. Review logs of related controllers

Additional context: %s`, strings.ToLower(resourceKind), resourceName, ns,
				strings.ToLower(resourceKind), resourceName, ns, ns, additionalCtx),
		})
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, ""), nil
}
