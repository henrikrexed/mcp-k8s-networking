package tools

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/isitobservable/k8s-networking-mcp/pkg/types"
)

// --- design_gateway_api ---

type DesignGatewayAPITool struct{ BaseTool }

func (t *DesignGatewayAPITool) Name() string { return "design_gateway_api" }
func (t *DesignGatewayAPITool) Description() string {
	return "Generate Gateway API configuration guidance with annotated YAML templates based on user intent"
}
func (t *DesignGatewayAPITool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"intent": map[string]interface{}{
				"type":        "string",
				"description": "What the user wants to achieve (e.g., 'expose service X on port Y via HTTPS')",
			},
			"service_name": map[string]interface{}{
				"type":        "string",
				"description": "Target service name",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Target namespace",
			},
			"port": map[string]interface{}{
				"type":        "integer",
				"description": "Service port to expose",
			},
			"hostname": map[string]interface{}{
				"type":        "string",
				"description": "Hostname for the route (e.g., app.example.com)",
			},
			"protocol": map[string]interface{}{
				"type":        "string",
				"description": "Protocol: HTTP, HTTPS, or GRPC (default: HTTP)",
			},
			"tls_secret": map[string]interface{}{
				"type":        "string",
				"description": "Name of TLS secret for HTTPS (namespace/name format)",
			},
			"gateway_name": map[string]interface{}{
				"type":        "string",
				"description": "Existing Gateway to attach the route to",
			},
			"gateway_namespace": map[string]interface{}{
				"type":        "string",
				"description": "Namespace of the existing Gateway",
			},
		},
		"required": []string{"service_name", "namespace", "port"},
	}
}

func (t *DesignGatewayAPITool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	svcName := getStringArg(args, "service_name", "")
	ns := getStringArg(args, "namespace", "default")
	port := getIntArg(args, "port", 80)
	hostname := getStringArg(args, "hostname", "")
	protocol := strings.ToUpper(getStringArg(args, "protocol", "HTTP"))
	tlsSecret := getStringArg(args, "tls_secret", "")
	gwName := getStringArg(args, "gateway_name", "")
	gwNamespace := getStringArg(args, "gateway_namespace", "")

	findings := make([]types.DiagnosticFinding, 0, 8)

	// Check service exists
	_, err := t.Clients.Dynamic.Resource(servicesGVR).Namespace(ns).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil {
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityWarning,
			Category:   types.CategoryRouting,
			Resource:   &types.ResourceRef{Kind: "Service", Namespace: ns, Name: svcName},
			Summary:    fmt.Sprintf("Target service %s/%s not found", ns, svcName),
			Suggestion: "Ensure the service is created before applying the generated manifests.",
		})
	}

	// Check for existing gateways
	if gwName == "" {
		gateways, gwErr := listWithFallback(ctx, t.Clients.Dynamic, gatewaysV1GVR, gatewaysV1B1GVR, "")
		if gwErr == nil && len(gateways.Items) > 0 {
			gwNames := make([]string, 0, len(gateways.Items))
			for _, gw := range gateways.Items {
				gwNames = append(gwNames, fmt.Sprintf("%s/%s", gw.GetNamespace(), gw.GetName()))
			}
			findings = append(findings, types.DiagnosticFinding{
				Severity:   types.SeverityInfo,
				Category:   types.CategoryRouting,
				Summary:    fmt.Sprintf("Found %d existing Gateway(s): %s", len(gateways.Items), strings.Join(gwNames, ", ")),
				Suggestion: "Consider using an existing Gateway as the parentRef for your route.",
			})
			// Use the first gateway as default
			gwName = gateways.Items[0].GetName()
			gwNamespace = gateways.Items[0].GetNamespace()
		}
	}

	// Generate manifests
	resources := make([]string, 0, 3)

	// Gateway (if none exists)
	if gwName == "" {
		gwName = "main-gateway"
		gwNamespace = ns
		listenerProtocol := "HTTP"
		listenerPort := 80
		if protocol == "HTTPS" {
			listenerProtocol = "HTTPS"
			listenerPort = 443
		}

		gwYAML := fmt.Sprintf(`# Gateway - Entry point for external traffic
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: %s
  namespace: %s
spec:
  gatewayClassName: "" # Set to your provider's class (e.g., istio, envoy-gateway, kgateway)
  listeners:
  - name: %s
    protocol: %s
    port: %d`,
			gwName, gwNamespace,
			strings.ToLower(listenerProtocol), listenerProtocol, listenerPort)

		if protocol == "HTTPS" && tlsSecret != "" {
			gwYAML += fmt.Sprintf(`
    tls:
      mode: Terminate
      certificateRefs:
      - name: %s`, tlsSecret)
		}
		if hostname != "" {
			gwYAML += fmt.Sprintf(`
    hostname: "%s"`, hostname)
		}

		resources = append(resources, gwYAML)

		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityInfo,
			Category:   types.CategoryRouting,
			Summary:    "Generated Gateway resource",
			Detail:     gwYAML,
			Suggestion: "Set gatewayClassName to match your installed Gateway API provider.",
		})
	}

	// HTTPRoute or GRPCRoute
	routeKind := "HTTPRoute"
	if protocol == "GRPC" {
		routeKind = "GRPCRoute"
	}

	parentRefYAML := fmt.Sprintf(`  parentRefs:
  - name: %s`, gwName)
	if gwNamespace != "" && gwNamespace != ns {
		parentRefYAML += fmt.Sprintf(`
    namespace: %s`, gwNamespace)
	}

	hostnameYAML := ""
	if hostname != "" {
		hostnameYAML = fmt.Sprintf(`
  hostnames:
  - "%s"`, hostname)
	}

	routeYAML := fmt.Sprintf(`# %s - Routes traffic to the target service
apiVersion: gateway.networking.k8s.io/v1
kind: %s
metadata:
  name: %s-route
  namespace: %s
spec:
%s%s
  rules:
  - backendRefs:
    - name: %s
      port: %d`,
		routeKind, routeKind, svcName, ns,
		parentRefYAML, hostnameYAML, svcName, port)

	resources = append(resources, routeYAML)
	findings = append(findings, types.DiagnosticFinding{
		Severity: types.SeverityInfo,
		Category: types.CategoryRouting,
		Summary:  fmt.Sprintf("Generated %s resource", routeKind),
		Detail:   routeYAML,
	})

	// ReferenceGrant if cross-namespace
	if gwNamespace != "" && gwNamespace != ns {
		refGrantYAML := fmt.Sprintf(`# ReferenceGrant - Allows cross-namespace route reference
apiVersion: gateway.networking.k8s.io/v1beta1
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
    kind: Service`, ns, gwNamespace, ns, routeKind, ns)

		resources = append(resources, refGrantYAML)
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityWarning,
			Category:   types.CategoryRouting,
			Summary:    "Cross-namespace reference requires a ReferenceGrant",
			Detail:     refGrantYAML,
			Suggestion: "Apply this ReferenceGrant in the target service namespace to allow cross-namespace routing.",
		})
	}

	// Prerequisites warnings
	if protocol == "HTTPS" && tlsSecret == "" {
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityWarning,
			Category:   types.CategoryTLS,
			Summary:    "HTTPS requested but no TLS secret specified",
			Suggestion: "Create a TLS secret with your certificate and key, then reference it in the Gateway listener.",
		})
	}

	// Summary
	allYAML := strings.Join(resources, "\n---\n")
	findings = append(findings, types.DiagnosticFinding{
		Severity: types.SeverityInfo,
		Category: types.CategoryRouting,
		Summary:  fmt.Sprintf("Complete Gateway API configuration: %d resources to apply", len(resources)),
		Detail:   allYAML,
	})

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, "gateway-api"), nil
}
