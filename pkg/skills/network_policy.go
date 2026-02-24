package skills

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/isitobservable/k8s-networking-mcp/pkg/types"
)

var npGVR = schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"}

// NetworkPolicySkill guides through creating NetworkPolicies.
type NetworkPolicySkill struct {
	base      skillBase
	hasCilium bool
	hasCalico bool
}

func (s *NetworkPolicySkill) Definition() SkillDefinition {
	return SkillDefinition{
		Name:        "create_network_policy",
		Description: "Step-by-step workflow to create NetworkPolicies for service isolation",
		Parameters: []SkillParam{
			{Name: "target_service", Type: "string", Required: true, Description: "Target service to protect"},
			{Name: "namespace", Type: "string", Required: true, Description: "Target namespace"},
			{Name: "allowed_sources", Type: "string", Required: false, Description: "Comma-separated list of allowed source namespaces"},
			{Name: "port", Type: "integer", Required: false, Description: "Service port (default: 80)"},
		},
	}
}

func (s *NetworkPolicySkill) Execute(ctx context.Context, args map[string]interface{}) (*SkillResult, error) {
	svcName := getArg(args, "target_service", "")
	ns := getArg(args, "namespace", "default")
	allowedSources := getArg(args, "allowed_sources", "")
	port := getIntArgSkill(args, "port", 80)

	result := &SkillResult{
		SkillName: "create_network_policy",
		Manifests: make([]string, 0, 2),
	}
	steps := make([]StepResult, 0, 6)

	// Step 1: Verify service and get selector
	svc, err := s.base.clients.Dynamic.Resource(svcGVR).Namespace(ns).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil {
		steps = append(steps, StepResult{
			StepName: "verify_service",
			Status:   "failed",
			Findings: []types.DiagnosticFinding{{
				Severity: types.SeverityCritical,
				Category: types.CategoryPolicy,
				Summary:  fmt.Sprintf("Service %s/%s not found", ns, svcName),
			}},
		})
		result.Steps = steps
		result.Status = "failed"
		result.Summary = fmt.Sprintf("Service %s/%s not found", ns, svcName)
		return result, nil
	}

	selector, _, _ := unstructured.NestedStringMap(svc.Object, "spec", "selector")
	selectorYAML := ""
	for k, v := range selector {
		selectorYAML += fmt.Sprintf("\n      %s: %s", k, v)
	}
	if selectorYAML == "" {
		selectorYAML = "\n      app: " + svcName
	}

	steps = append(steps, StepResult{
		StepName: "verify_service",
		Status:   "passed",
		Findings: []types.DiagnosticFinding{{
			Severity: types.SeverityOK,
			Category: types.CategoryPolicy,
			Summary:  fmt.Sprintf("Service %s/%s found with selector %v", ns, svcName, selector),
		}},
	})

	// Step 2: Check existing policies
	existingPolicies, err := s.base.clients.Dynamic.Resource(npGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	if err == nil && len(existingPolicies.Items) > 0 {
		steps = append(steps, StepResult{
			StepName: "check_existing_policies",
			Status:   "warning",
			Findings: []types.DiagnosticFinding{{
				Severity:   types.SeverityWarning,
				Category:   types.CategoryPolicy,
				Summary:    fmt.Sprintf("Found %d existing NetworkPolicies in %s", len(existingPolicies.Items), ns),
				Suggestion: "Review existing policies to avoid conflicts.",
			}},
		})
	} else {
		steps = append(steps, StepResult{
			StepName: "check_existing_policies",
			Status:   "passed",
		})
	}

	// Step 3: Detect CNI provider
	providerNote := "Using standard Kubernetes NetworkPolicy"
	if s.hasCilium {
		providerNote = "Cilium detected; using standard K8s NetworkPolicy (compatible)"
	} else if s.hasCalico {
		providerNote = "Calico detected; using standard K8s NetworkPolicy (compatible)"
	}
	steps = append(steps, StepResult{
		StepName: "detect_cni",
		Status:   "passed",
		Findings: []types.DiagnosticFinding{{
			Severity: types.SeverityInfo,
			Category: types.CategoryPolicy,
			Summary:  providerNote,
		}},
	})

	// Step 4: Generate NetworkPolicy
	ingressRules := ""
	if allowedSources != "" {
		for _, src := range strings.Split(allowedSources, ",") {
			src = strings.TrimSpace(src)
			if src != "" {
				ingressRules += fmt.Sprintf(`
    - from:
      - namespaceSelector:
          matchLabels:
            kubernetes.io/metadata.name: %s
      ports:
      - protocol: TCP
        port: %d`, src, port)
			}
		}
	}
	if ingressRules == "" {
		ingressRules = fmt.Sprintf(`
    - ports:
      - protocol: TCP
        port: %d`, port)
	}

	npYAML := fmt.Sprintf(`apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: %s-ingress
  namespace: %s
spec:
  podSelector:
    matchLabels:%s
  policyTypes:
  - Ingress
  - Egress
  ingress:%s
  egress:
  # Allow DNS resolution (required)
  - to:
    - namespaceSelector: {}
    ports:
    - protocol: UDP
      port: 53
    - protocol: TCP
      port: 53
  # Allow outbound to same namespace
  - to:
    - podSelector: {}`, svcName, ns, selectorYAML, ingressRules)

	result.Manifests = append(result.Manifests, npYAML)
	steps = append(steps, StepResult{
		StepName: "generate_policy",
		Status:   "passed",
		Findings: []types.DiagnosticFinding{
			{
				Severity: types.SeverityInfo,
				Category: types.CategoryPolicy,
				Summary:  "Generated NetworkPolicy with ingress and egress rules",
			},
			{
				Severity:   types.SeverityInfo,
				Category:   types.CategoryDNS,
				Summary:    "DNS egress rule automatically included",
				Suggestion: "DNS (port 53) egress is required for pod name resolution.",
			},
		},
		Output: npYAML,
	})

	// Step 5: Summary
	steps = append(steps, StepResult{
		StepName: "complete",
		Status:   "passed",
		Findings: []types.DiagnosticFinding{{
			Severity: types.SeverityOK,
			Category: types.CategoryPolicy,
			Summary:  fmt.Sprintf("Generated NetworkPolicy for %s/%s", ns, svcName),
		}},
		Output: strings.Join(result.Manifests, "\n---\n"),
	})

	result.Steps = steps
	result.Status = "completed"
	result.Summary = fmt.Sprintf("Generated NetworkPolicy to protect %s/%s on port %d", ns, svcName, port)

	return result, nil
}
