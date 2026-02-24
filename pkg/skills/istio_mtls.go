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

var (
	paGVR = schema.GroupVersionResource{Group: "security.istio.io", Version: "v1", Resource: "peerauthentications"}
	drGVR = schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1", Resource: "destinationrules"}
)

// ConfigureMTLSSkill guides through configuring mTLS between services.
type ConfigureMTLSSkill struct {
	base skillBase
}

func (s *ConfigureMTLSSkill) Definition() SkillDefinition {
	return SkillDefinition{
		Name:         "configure_istio_mtls",
		Description:  "Step-by-step workflow to configure mTLS between services using Istio",
		RequiredCRDs: []string{"security.istio.io"},
		Parameters: []SkillParam{
			{Name: "namespace", Type: "string", Required: true, Description: "Target namespace"},
			{Name: "mode", Type: "string", Required: false, Description: "mTLS mode: STRICT or PERMISSIVE (default: STRICT)"},
		},
	}
}

func (s *ConfigureMTLSSkill) Execute(ctx context.Context, args map[string]interface{}) (*SkillResult, error) {
	ns := getArg(args, "namespace", "default")
	mode := strings.ToUpper(getArg(args, "mode", "STRICT"))

	result := &SkillResult{
		SkillName: "configure_istio_mtls",
		Manifests: make([]string, 0, 2),
	}
	steps := make([]StepResult, 0, 7)

	// Step 1: Check sidecar injection
	nsObj, err := s.base.clients.Clientset.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
	injectionEnabled := false
	if err == nil {
		if nsObj.Labels["istio-injection"] == "enabled" {
			injectionEnabled = true
		}
	}

	if !injectionEnabled {
		steps = append(steps, StepResult{
			StepName: "check_sidecar_injection",
			Status:   "warning",
			Findings: []types.DiagnosticFinding{{
				Severity:   types.SeverityWarning,
				Category:   types.CategoryMesh,
				Summary:    fmt.Sprintf("Sidecar injection not enabled for namespace %s", ns),
				Suggestion: fmt.Sprintf("Enable injection: kubectl label namespace %s istio-injection=enabled --overwrite", ns),
			}},
		})
	} else {
		steps = append(steps, StepResult{
			StepName: "check_sidecar_injection",
			Status:   "passed",
			Findings: []types.DiagnosticFinding{{
				Severity: types.SeverityOK,
				Category: types.CategoryMesh,
				Summary:  fmt.Sprintf("Sidecar injection enabled for namespace %s", ns),
			}},
		})
	}

	// Step 2: Check current mTLS status
	existingPAs, err := s.base.clients.Dynamic.Resource(paGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	if err == nil && len(existingPAs.Items) > 0 {
		for _, pa := range existingPAs.Items {
			mtlsMode, _, _ := unstructured.NestedString(pa.Object, "spec", "mtls", "mode")
			steps = append(steps, StepResult{
				StepName: "check_current_mtls",
				Status:   "warning",
				Findings: []types.DiagnosticFinding{{
					Severity:   types.SeverityWarning,
					Category:   types.CategoryTLS,
					Resource:   &types.ResourceRef{Kind: "PeerAuthentication", Namespace: pa.GetNamespace(), Name: pa.GetName()},
					Summary:    fmt.Sprintf("Existing PeerAuthentication %s with mode %s", pa.GetName(), mtlsMode),
					Suggestion: "Review and potentially update this existing policy.",
				}},
			})
		}
	} else {
		steps = append(steps, StepResult{
			StepName: "check_current_mtls",
			Status:   "passed",
			Findings: []types.DiagnosticFinding{{
				Severity: types.SeverityInfo,
				Category: types.CategoryTLS,
				Summary:  "No existing PeerAuthentication in namespace",
			}},
		})
	}

	// Step 3: Check for conflicting DestinationRules
	drConflictFound := false
	drList, err := s.base.clients.Dynamic.Resource(drGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, dr := range drList.Items {
			tlsMode, _, _ := unstructured.NestedString(dr.Object, "spec", "trafficPolicy", "tls", "mode")
			if tlsMode != "" && tlsMode != "ISTIO_MUTUAL" && mode == "STRICT" {
				drConflictFound = true
				steps = append(steps, StepResult{
					StepName: "check_dr_conflicts",
					Status:   "warning",
					Findings: []types.DiagnosticFinding{{
						Severity:   types.SeverityCritical,
						Category:   types.CategoryTLS,
						Resource:   &types.ResourceRef{Kind: "DestinationRule", Namespace: dr.GetNamespace(), Name: dr.GetName()},
						Summary:    fmt.Sprintf("DestinationRule %s has TLS mode %s which conflicts with STRICT mTLS", dr.GetName(), tlsMode),
						Suggestion: "Update DestinationRule TLS mode to ISTIO_MUTUAL.",
					}},
				})
			}
		}
	}
	if !drConflictFound {
		steps = append(steps, StepResult{
			StepName: "check_dr_conflicts",
			Status:   "passed",
			Findings: []types.DiagnosticFinding{{
				Severity: types.SeverityOK,
				Category: types.CategoryTLS,
				Summary:  "No conflicting DestinationRule TLS settings",
			}},
		})
	}

	// Step 4: Generate PeerAuthentication
	paYAML := fmt.Sprintf(`apiVersion: security.istio.io/v1
kind: PeerAuthentication
metadata:
  name: %s-mtls
  namespace: %s
spec:
  mtls:
    mode: %s`, ns, ns, mode)

	result.Manifests = append(result.Manifests, paYAML)
	steps = append(steps, StepResult{
		StepName: "generate_peer_auth",
		Status:   "passed",
		Findings: []types.DiagnosticFinding{{
			Severity: types.SeverityInfo,
			Category: types.CategoryTLS,
			Summary:  fmt.Sprintf("Generated PeerAuthentication with mode %s", mode),
		}},
		Output: paYAML,
	})

	// Step 5: Generate DestinationRule alignment (for STRICT mode)
	if mode == "STRICT" {
		drYAML := fmt.Sprintf(`# Apply if you have services that need explicit ISTIO_MUTUAL TLS
apiVersion: networking.istio.io/v1
kind: DestinationRule
metadata:
  name: %s-mtls-dr
  namespace: %s
spec:
  host: "*.%s.svc.cluster.local"
  trafficPolicy:
    tls:
      mode: ISTIO_MUTUAL`, ns, ns, ns)

		result.Manifests = append(result.Manifests, drYAML)
		steps = append(steps, StepResult{
			StepName: "generate_destination_rule",
			Status:   "passed",
			Findings: []types.DiagnosticFinding{{
				Severity: types.SeverityInfo,
				Category: types.CategoryTLS,
				Summary:  "Generated DestinationRule with ISTIO_MUTUAL TLS",
			}},
			Output: drYAML,
		})
	}

	// Summary
	steps = append(steps, StepResult{
		StepName: "complete",
		Status:   "passed",
		Findings: []types.DiagnosticFinding{{
			Severity: types.SeverityOK,
			Category: types.CategoryTLS,
			Summary:  fmt.Sprintf("Generated %d manifests for %s mTLS in namespace %s", len(result.Manifests), mode, ns),
		}},
		Output: strings.Join(result.Manifests, "\n---\n"),
	})

	result.Steps = steps
	result.Status = "completed"
	result.Summary = fmt.Sprintf("Generated %d manifests for %s mTLS in namespace %s", len(result.Manifests), mode, ns)

	return result, nil
}
