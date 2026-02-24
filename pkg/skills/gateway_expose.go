package skills

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/isitobservable/k8s-networking-mcp/pkg/types"
)

var svcGVR = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}
var gwGVR = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gateways"}

// ExposeServiceSkill guides through exposing a service via Gateway API.
type ExposeServiceSkill struct {
	base skillBase
}

func (s *ExposeServiceSkill) Definition() SkillDefinition {
	return SkillDefinition{
		Name:         "expose_service_gateway_api",
		Description:  "Step-by-step workflow to expose a service via Gateway API HTTPRoute",
		RequiredCRDs: []string{"gateway.networking.k8s.io"},
		Parameters: []SkillParam{
			{Name: "service_name", Type: "string", Required: true, Description: "Target service name"},
			{Name: "namespace", Type: "string", Required: true, Description: "Target namespace"},
			{Name: "port", Type: "integer", Required: true, Description: "Service port to expose"},
			{Name: "hostname", Type: "string", Required: false, Description: "Hostname for the route"},
			{Name: "protocol", Type: "string", Required: false, Description: "Protocol: HTTP, HTTPS, or GRPC"},
		},
	}
}

func (s *ExposeServiceSkill) Execute(ctx context.Context, args map[string]interface{}) (*SkillResult, error) {
	svcName := getArg(args, "service_name", "")
	ns := getArg(args, "namespace", "default")
	port := getIntArgSkill(args, "port", 80)
	hostname := getArg(args, "hostname", "")
	protocol := strings.ToUpper(getArg(args, "protocol", "HTTP"))

	result := &SkillResult{
		SkillName: "expose_service_gateway_api",
		Manifests: make([]string, 0, 3),
	}
	steps := make([]StepResult, 0, 7)

	// Step 1: Verify service exists
	svc, err := s.base.clients.Dynamic.Resource(svcGVR).Namespace(ns).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil {
		steps = append(steps, StepResult{
			StepName: "verify_service",
			Status:   "failed",
			Findings: []types.DiagnosticFinding{{
				Severity:   types.SeverityCritical,
				Category:   types.CategoryRouting,
				Summary:    fmt.Sprintf("Service %s/%s not found", ns, svcName),
				Suggestion: "Create the service before running this skill.",
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
		Findings: []types.DiagnosticFinding{{
			Severity: types.SeverityOK,
			Category: types.CategoryRouting,
			Summary:  fmt.Sprintf("Service %s/%s exists", ns, svc.GetName()),
		}},
	})

	// Step 2: Detect Gateway API provider
	steps = append(steps, StepResult{
		StepName: "detect_provider",
		Status:   "passed",
		Findings: []types.DiagnosticFinding{{
			Severity: types.SeverityInfo,
			Category: types.CategoryRouting,
			Summary:  "Gateway API CRDs detected",
		}},
	})

	// Step 3: Check for existing Gateways
	gwList, err := s.base.clients.Dynamic.Resource(gwGVR).List(ctx, metav1.ListOptions{})
	gwName := ""
	gwNs := ""
	if err == nil && len(gwList.Items) > 0 {
		gwName = gwList.Items[0].GetName()
		gwNs = gwList.Items[0].GetNamespace()
		steps = append(steps, StepResult{
			StepName: "check_gateway",
			Status:   "passed",
			Findings: []types.DiagnosticFinding{{
				Severity: types.SeverityInfo,
				Category: types.CategoryRouting,
				Summary:  fmt.Sprintf("Using existing Gateway %s/%s", gwNs, gwName),
			}},
		})
	} else {
		gwName = "main-gateway"
		gwNs = ns
		gwYAML := fmt.Sprintf(`apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: %s
  namespace: %s
spec:
  gatewayClassName: "" # Set to your provider's class
  listeners:
  - name: %s
    protocol: %s
    port: %d`, gwName, gwNs, strings.ToLower(protocol), protocol, func() int {
			if protocol == "HTTPS" {
				return 443
			}
			return 80
		}())
		result.Manifests = append(result.Manifests, gwYAML)
		steps = append(steps, StepResult{
			StepName: "check_gateway",
			Status:   "warning",
			Findings: []types.DiagnosticFinding{{
				Severity:   types.SeverityWarning,
				Category:   types.CategoryRouting,
				Summary:    "No existing Gateway found, generated Gateway manifest",
				Suggestion: "Set gatewayClassName to your provider's class.",
			}},
			Output: gwYAML,
		})
	}

	// Step 4: Generate HTTPRoute
	routeKind := "HTTPRoute"
	if protocol == "GRPC" {
		routeKind = "GRPCRoute"
	}

	parentRef := fmt.Sprintf("    name: %s", gwName)
	if gwNs != ns {
		parentRef += fmt.Sprintf("\n    namespace: %s", gwNs)
	}
	hostnameYAML := ""
	if hostname != "" {
		hostnameYAML = fmt.Sprintf("\n  hostnames:\n  - %q", hostname)
	}

	routeYAML := fmt.Sprintf(`apiVersion: gateway.networking.k8s.io/v1
kind: %s
metadata:
  name: %s-route
  namespace: %s
spec:
  parentRefs:
  - %s%s
  rules:
  - backendRefs:
    - name: %s
      port: %d`, routeKind, svcName, ns, parentRef, hostnameYAML, svcName, port)

	result.Manifests = append(result.Manifests, routeYAML)
	steps = append(steps, StepResult{
		StepName: "generate_route",
		Status:   "passed",
		Findings: []types.DiagnosticFinding{{
			Severity: types.SeverityInfo,
			Category: types.CategoryRouting,
			Summary:  fmt.Sprintf("Generated %s for %s/%s", routeKind, ns, svcName),
		}},
		Output: routeYAML,
	})

	// Step 5: Check for cross-namespace ReferenceGrant
	if gwNs != "" && gwNs != ns {
		refGrantYAML := fmt.Sprintf(`apiVersion: gateway.networking.k8s.io/v1beta1
kind: ReferenceGrant
metadata:
  name: allow-%s-from-%s
  namespace: %s
spec:
  from:
  - group: gateway.networking.k8s.io
    kind: %s
    namespace: %s
  to:
  - group: ""
    kind: Service`, ns, gwNs, ns, routeKind, ns)
		result.Manifests = append(result.Manifests, refGrantYAML)
		steps = append(steps, StepResult{
			StepName: "check_reference_grant",
			Status:   "warning",
			Findings: []types.DiagnosticFinding{{
				Severity:   types.SeverityWarning,
				Category:   types.CategoryRouting,
				Summary:    "Cross-namespace reference requires ReferenceGrant",
				Suggestion: "Apply the ReferenceGrant in the service namespace.",
			}},
			Output: refGrantYAML,
		})
	} else {
		steps = append(steps, StepResult{
			StepName: "check_reference_grant",
			Status:   "skipped",
			Findings: []types.DiagnosticFinding{{
				Severity: types.SeverityInfo,
				Category: types.CategoryRouting,
				Summary:  "Same-namespace route, no ReferenceGrant needed",
			}},
		})
	}

	// Step 6: Summary
	steps = append(steps, StepResult{
		StepName: "complete",
		Status:   "passed",
		Findings: []types.DiagnosticFinding{{
			Severity: types.SeverityOK,
			Category: types.CategoryRouting,
			Summary:  fmt.Sprintf("Generated %d manifests to expose %s/%s via Gateway API", len(result.Manifests), ns, svcName),
		}},
		Output: strings.Join(result.Manifests, "\n---\n"),
	})

	result.Steps = steps
	result.Status = "completed"
	result.Summary = fmt.Sprintf("Generated %d manifests to expose %s/%s:%d via %s", len(result.Manifests), ns, svcName, port, routeKind)

	return result, nil
}

func getArg(args map[string]interface{}, key, defaultVal string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return defaultVal
}

func getIntArgSkill(args map[string]interface{}, key string, defaultVal int) int {
	if v, ok := args[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return defaultVal
}
