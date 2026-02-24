package tools

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/isitobservable/k8s-networking-mcp/pkg/types"
)

// --- design_kgateway ---

type DesignKgatewayTool struct{ BaseTool }

func (t *DesignKgatewayTool) Name() string { return "design_kgateway" }
func (t *DesignKgatewayTool) Description() string {
	return "Generate kgateway configuration guidance with annotated YAML templates based on user intent"
}
func (t *DesignKgatewayTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"intent": map[string]interface{}{
				"type":        "string",
				"description": "What the user wants to achieve (e.g., 'configure rate limiting', 'add header manipulation')",
			},
			"service_name": map[string]interface{}{
				"type":        "string",
				"description": "Target service name",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Target namespace",
			},
			"route_name": map[string]interface{}{
				"type":        "string",
				"description": "HTTPRoute to attach RouteOption to",
			},
			"gateway_name": map[string]interface{}{
				"type":        "string",
				"description": "Gateway to configure with GatewayParameters",
			},
			"resource_type": map[string]interface{}{
				"type":        "string",
				"description": "Specific resource to generate: routeoption, virtualhostoption, or gatewayparameters",
			},
		},
		"required": []string{"namespace"},
	}
}

func (t *DesignKgatewayTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	intent := strings.ToLower(getStringArg(args, "intent", ""))
	svcName := getStringArg(args, "service_name", "")
	ns := getStringArg(args, "namespace", "default")
	routeName := getStringArg(args, "route_name", "")
	gwName := getStringArg(args, "gateway_name", "")
	resourceType := strings.ToLower(getStringArg(args, "resource_type", ""))

	findings := make([]types.DiagnosticFinding, 0, 6)
	resources := make([]string, 0, 3)

	// Check for existing kgateway resources
	existingRO, err := t.Clients.Dynamic.Resource(routeOptionGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	if err == nil && len(existingRO.Items) > 0 {
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryRouting,
			Summary:  fmt.Sprintf("Found %d existing RouteOption(s) in %s", len(existingRO.Items), ns),
		})
	}

	wantRouteOption := resourceType == "routeoption" || strings.Contains(intent, "rate") || strings.Contains(intent, "header") || strings.Contains(intent, "timeout") || strings.Contains(intent, "retry")
	wantVHO := resourceType == "virtualhostoption" || strings.Contains(intent, "cors") || strings.Contains(intent, "virtualhost")
	wantGWParams := resourceType == "gatewayparameters" || gwName != "" || strings.Contains(intent, "gateway param")

	// RouteOption
	if wantRouteOption || (routeName != "" && !wantVHO && !wantGWParams) {
		targetRoute := routeName
		if targetRoute == "" && svcName != "" {
			targetRoute = svcName + "-route"
		}
		if targetRoute == "" {
			targetRoute = "my-route"
		}

		roYAML := fmt.Sprintf(`# RouteOption - Attaches kgateway-specific policies to an HTTPRoute
apiVersion: gateway.kgateway.dev/v1alpha1
kind: RouteOption
metadata:
  name: %s-options
  namespace: %s
spec:
  targetRefs:
  - group: gateway.networking.k8s.io
    kind: HTTPRoute
    name: %s
  options:
    # Add kgateway-specific options here, for example:
    # timeout: 30s
    # retries:
    #   retryOn: "5xx"
    #   numRetries: 3
    {}`, targetRoute, ns, targetRoute)

		resources = append(resources, roYAML)
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityInfo,
			Category:   types.CategoryRouting,
			Summary:    "Generated RouteOption resource",
			Detail:     roYAML,
			Suggestion: "Customize the options block with your desired policies (rate limiting, timeouts, retries, header manipulation).",
		})
	}

	// VirtualHostOption
	if wantVHO {
		vhoYAML := fmt.Sprintf(`# VirtualHostOption - Applies policies at the virtual host level
apiVersion: gateway.kgateway.dev/v1alpha1
kind: VirtualHostOption
metadata:
  name: %s-vho
  namespace: %s
spec:
  targetRefs:
  - group: gateway.networking.k8s.io
    kind: Gateway
    name: %s
  options:
    # Add virtual host-level options here, for example:
    # cors:
    #   allowOrigin:
    #   - "https://example.com"
    #   allowMethods:
    #   - GET
    #   - POST
    {}`, ns, ns, func() string {
			if gwName != "" {
				return gwName
			}
			return "main-gateway"
		}())

		resources = append(resources, vhoYAML)
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityInfo,
			Category:   types.CategoryRouting,
			Summary:    "Generated VirtualHostOption resource",
			Detail:     vhoYAML,
			Suggestion: "Configure CORS, rate limiting, or other gateway-level policies.",
		})
	}

	// GatewayParameters
	if wantGWParams {
		targetGW := gwName
		if targetGW == "" {
			targetGW = "main-gateway"
		}

		gpYAML := fmt.Sprintf(`# GatewayParameters - Configures kgateway-specific Gateway settings
apiVersion: gateway.kgateway.dev/v1alpha1
kind: GatewayParameters
metadata:
  name: %s-params
  namespace: %s
spec:
  kube:
    deployment:
      replicas: 2
    service:
      type: LoadBalancer
    # Additional kgateway-specific parameters:
    # envoyContainer:
    #   resources:
    #     requests:
    #       cpu: "100m"
    #       memory: "128Mi"`, targetGW, ns)

		resources = append(resources, gpYAML)
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityInfo,
			Category:   types.CategoryRouting,
			Summary:    "Generated GatewayParameters resource",
			Detail:     gpYAML,
			Suggestion: "Reference this GatewayParameters from your Gateway using the kgateway.dev/gateway-parameters annotation.",
		})
	}

	if len(resources) == 0 {
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityInfo,
			Category:   types.CategoryRouting,
			Summary:    "No specific kgateway resources generated",
			Suggestion: "Specify resource_type (routeoption, virtualhostoption, gatewayparameters) or describe your intent for more specific guidance.",
		})
	} else {
		allYAML := strings.Join(resources, "\n---\n")
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryRouting,
			Summary:  fmt.Sprintf("Complete kgateway configuration: %d resources to apply", len(resources)),
			Detail:   allYAML,
		})
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, "kgateway"), nil
}
