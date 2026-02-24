package skills

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/isitobservable/k8s-networking-mcp/pkg/types"
)

// TrafficSplitSkill guides through configuring traffic splitting.
type TrafficSplitSkill struct {
	base          skillBase
	hasIstio      bool
	hasGatewayAPI bool
}

func (s *TrafficSplitSkill) Definition() SkillDefinition {
	return SkillDefinition{
		Name:         "configure_traffic_split",
		Description:  "Step-by-step workflow to configure traffic splitting between service versions (canary/blue-green)",
		RequiredCRDs: []string{"networking.istio.io OR gateway.networking.k8s.io"},
		Parameters: []SkillParam{
			{Name: "service_name", Type: "string", Required: true, Description: "Target service name"},
			{Name: "namespace", Type: "string", Required: true, Description: "Target namespace"},
			{Name: "versions", Type: "string", Required: true, Description: "Comma-separated version names (e.g., 'v1,v2')"},
			{Name: "weights", Type: "string", Required: true, Description: "Comma-separated weights (e.g., '80,20')"},
		},
	}
}

func (s *TrafficSplitSkill) Execute(ctx context.Context, args map[string]interface{}) (*SkillResult, error) {
	svcName := getArg(args, "service_name", "")
	ns := getArg(args, "namespace", "default")
	versionsStr := getArg(args, "versions", "v1,v2")
	weightsStr := getArg(args, "weights", "80,20")

	versions := strings.Split(versionsStr, ",")
	weights := parseWeights(weightsStr)

	result := &SkillResult{
		SkillName: "configure_traffic_split",
		Manifests: make([]string, 0, 3),
	}
	steps := make([]StepResult, 0, 7)

	// Step 0: Validate version/weight count match
	if len(versions) != len(weights) {
		steps = append(steps, StepResult{
			StepName: "validate_inputs",
			Status:   "failed",
			Findings: []types.DiagnosticFinding{{
				Severity:   types.SeverityCritical,
				Category:   types.CategoryRouting,
				Summary:    fmt.Sprintf("Version count (%d) does not match weight count (%d)", len(versions), len(weights)),
				Suggestion: "Provide the same number of versions and weights.",
			}},
		})
		result.Steps = steps
		result.Status = "failed"
		result.Summary = fmt.Sprintf("Version count (%d) does not match weight count (%d)", len(versions), len(weights))
		return result, nil
	}

	// Step 1: Verify service exists
	_, err := s.base.clients.Dynamic.Resource(svcGVR).Namespace(ns).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil {
		steps = append(steps, StepResult{
			StepName: "verify_service",
			Status:   "failed",
			Findings: []types.DiagnosticFinding{{
				Severity: types.SeverityCritical,
				Category: types.CategoryRouting,
				Summary:  fmt.Sprintf("Service %s/%s not found", ns, svcName),
			}},
		})
		result.Steps = steps
		result.Status = "failed"
		result.Summary = fmt.Sprintf("Service %s/%s not found", ns, svcName)
		return result, nil
	}
	steps = append(steps, StepResult{
		StepName: "verify_service",
		Status:   "passed",
	})

	// Step 2: Validate weights
	totalWeight := 0
	for _, w := range weights {
		totalWeight += w
	}
	if totalWeight != 100 {
		steps = append(steps, StepResult{
			StepName: "validate_weights",
			Status:   "failed",
			Findings: []types.DiagnosticFinding{{
				Severity:   types.SeverityCritical,
				Category:   types.CategoryRouting,
				Summary:    fmt.Sprintf("Weights sum to %d%%, must be 100%%", totalWeight),
				Suggestion: "Adjust weights to sum to exactly 100.",
			}},
		})
		result.Steps = steps
		result.Status = "failed"
		result.Summary = fmt.Sprintf("Invalid weights: sum is %d%%, expected 100%%", totalWeight)
		return result, nil
	}
	steps = append(steps, StepResult{
		StepName: "validate_weights",
		Status:   "passed",
		Findings: []types.DiagnosticFinding{{
			Severity: types.SeverityOK,
			Category: types.CategoryRouting,
			Summary:  fmt.Sprintf("Weights validated: %s = 100%%", weightsStr),
		}},
	})

	// Step 3: Generate manifests based on provider
	if s.hasIstio {
		// Generate Istio VirtualService + DestinationRule
		subsets := ""
		for _, v := range versions {
			v = strings.TrimSpace(v)
			subsets += fmt.Sprintf(`
  - name: %s
    labels:
      version: %s`, v, v)
		}

		drYAML := fmt.Sprintf(`apiVersion: networking.istio.io/v1
kind: DestinationRule
metadata:
  name: %s
  namespace: %s
spec:
  host: %s
  subsets:%s`, svcName, ns, svcName, subsets)
		result.Manifests = append(result.Manifests, drYAML)

		routes := ""
		for i, v := range versions {
			v = strings.TrimSpace(v)
			w := 0
			if i < len(weights) {
				w = weights[i]
			}
			routes += fmt.Sprintf(`
    - destination:
        host: %s
        subset: %s
      weight: %d`, svcName, v, w)
		}

		vsYAML := fmt.Sprintf(`apiVersion: networking.istio.io/v1
kind: VirtualService
metadata:
  name: %s
  namespace: %s
spec:
  hosts:
  - %s
  http:
  - route:%s`, svcName, ns, svcName, routes)
		result.Manifests = append(result.Manifests, vsYAML)

		steps = append(steps, StepResult{
			StepName: "generate_manifests",
			Status:   "passed",
			Findings: []types.DiagnosticFinding{{
				Severity: types.SeverityInfo,
				Category: types.CategoryRouting,
				Summary:  "Generated Istio DestinationRule and VirtualService",
			}},
			Output: drYAML + "\n---\n" + vsYAML,
		})
	} else if s.hasGatewayAPI {
		// Generate Gateway API HTTPRoute with weights
		backends := ""
		for i, v := range versions {
			v = strings.TrimSpace(v)
			w := 0
			if i < len(weights) {
				w = weights[i]
			}
			backends += fmt.Sprintf(`
    - name: %s-%s
      port: 80
      weight: %d`, svcName, v, w)
		}

		routeYAML := fmt.Sprintf(`apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: %s-split
  namespace: %s
spec:
  parentRefs:
  - name: main-gateway  # Update to your Gateway name
  rules:
  - backendRefs:%s`, svcName, ns, backends)
		result.Manifests = append(result.Manifests, routeYAML)

		steps = append(steps, StepResult{
			StepName: "generate_manifests",
			Status:   "passed",
			Findings: []types.DiagnosticFinding{{
				Severity:   types.SeverityInfo,
				Category:   types.CategoryRouting,
				Summary:    "Generated Gateway API HTTPRoute with weighted backends",
				Suggestion: "Each version needs a separate Service (e.g., my-service-v1, my-service-v2).",
			}},
			Output: routeYAML,
		})
	}

	// Summary
	steps = append(steps, StepResult{
		StepName: "complete",
		Status:   "passed",
		Findings: []types.DiagnosticFinding{{
			Severity: types.SeverityOK,
			Category: types.CategoryRouting,
			Summary:  fmt.Sprintf("Generated %d manifests for traffic split on %s/%s", len(result.Manifests), ns, svcName),
		}},
		Output: strings.Join(result.Manifests, "\n---\n"),
	})

	result.Steps = steps
	result.Status = "completed"
	result.Summary = fmt.Sprintf("Traffic split configured: %s with weights %s", versionsStr, weightsStr)

	return result, nil
}

func parseWeights(s string) []int {
	parts := strings.Split(s, ",")
	weights := make([]int, 0, len(parts))
	for _, p := range parts {
		w := 0
		if n, _ := fmt.Sscanf(strings.TrimSpace(p), "%d", &w); n > 0 {
			weights = append(weights, w)
		}
	}
	return weights
}
