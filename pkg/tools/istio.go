package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/isitobservable/k8s-networking-mcp/pkg/types"
)

// Istio GVR pairs for v1/v1beta1 fallback.
var (
	vsV1GVR   = schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1", Resource: "virtualservices"}
	vsV1B1GVR = schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "virtualservices"}
	drV1GVR   = schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1", Resource: "destinationrules"}
	drV1B1GVR = schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "destinationrules"}
	apV1GVR   = schema.GroupVersionResource{Group: "security.istio.io", Version: "v1", Resource: "authorizationpolicies"}
	apV1B1GVR = schema.GroupVersionResource{Group: "security.istio.io", Version: "v1beta1", Resource: "authorizationpolicies"}
	paV1GVR   = schema.GroupVersionResource{Group: "security.istio.io", Version: "v1", Resource: "peerauthentications"}
	paV1B1GVR = schema.GroupVersionResource{Group: "security.istio.io", Version: "v1beta1", Resource: "peerauthentications"}
)

type istioGVRPair struct {
	v1     schema.GroupVersionResource
	v1beta1 schema.GroupVersionResource
	apiGroup string
}

var istioKindGVRs = map[string]istioGVRPair{
	"VirtualService":      {v1: vsV1GVR, v1beta1: vsV1B1GVR, apiGroup: "networking.istio.io"},
	"DestinationRule":     {v1: drV1GVR, v1beta1: drV1B1GVR, apiGroup: "networking.istio.io"},
	"AuthorizationPolicy": {v1: apV1GVR, v1beta1: apV1B1GVR, apiGroup: "security.istio.io"},
	"PeerAuthentication":  {v1: paV1GVR, v1beta1: paV1B1GVR, apiGroup: "security.istio.io"},
}

// --- list_istio_resources ---

type ListIstioResourcesTool struct{ BaseTool }

func (t *ListIstioResourcesTool) Name() string        { return "list_istio_resources" }
func (t *ListIstioResourcesTool) Description() string {
	return "List Istio resources (VirtualService, DestinationRule, AuthorizationPolicy, PeerAuthentication) with key summary fields"
}
func (t *ListIstioResourcesTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"kind": map[string]interface{}{
				"type":        "string",
				"description": "Resource kind: VirtualService, DestinationRule, AuthorizationPolicy, PeerAuthentication",
				"enum":        []string{"VirtualService", "DestinationRule", "AuthorizationPolicy", "PeerAuthentication"},
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace (empty for all namespaces)",
			},
		},
		"required": []string{"kind"},
	}
}

func (t *ListIstioResourcesTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	kind := getStringArg(args, "kind", "")
	ns := getStringArg(args, "namespace", "")

	pair, ok := istioKindGVRs[kind]
	if !ok {
		return nil, &types.MCPError{
			Code:    types.ErrCodeInvalidInput,
			Tool:    t.Name(),
			Message: fmt.Sprintf("unsupported Istio resource kind: %s", kind),
		}
	}

	list, err := listWithFallback(ctx, t.Clients.Dynamic, pair.v1, pair.v1beta1, ns)
	if err != nil {
		return nil, &types.MCPError{
			Code:    types.ErrCodeCRDNotAvailable,
			Tool:    t.Name(),
			Message: fmt.Sprintf("failed to list %s", kind),
			Detail:  fmt.Sprintf("tried %s v1 and v1beta1: %v", pair.apiGroup, err),
		}
	}

	findings := make([]types.DiagnosticFinding, 0, len(list.Items))
	for _, item := range list.Items {
		ref := &types.ResourceRef{
			Kind:       kind,
			Namespace:  item.GetNamespace(),
			Name:       item.GetName(),
			APIVersion: pair.apiGroup,
		}

		summary, detail := istioResourceSummary(kind, &item)

		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryMesh,
			Resource: ref,
			Summary:  summary,
			Detail:   detail,
		})
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, "istio"), nil
}

// --- get_istio_resource ---

type GetIstioResourceTool struct{ BaseTool }

func (t *GetIstioResourceTool) Name() string        { return "get_istio_resource" }
func (t *GetIstioResourceTool) Description() string {
	return "Get full Istio resource detail: spec, status, and validation messages"
}
func (t *GetIstioResourceTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"kind": map[string]interface{}{
				"type":        "string",
				"description": "Resource kind: VirtualService, DestinationRule, AuthorizationPolicy, PeerAuthentication",
				"enum":        []string{"VirtualService", "DestinationRule", "AuthorizationPolicy", "PeerAuthentication"},
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Resource name",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace",
			},
		},
		"required": []string{"kind", "name", "namespace"},
	}
}

func (t *GetIstioResourceTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	kind := getStringArg(args, "kind", "")
	name := getStringArg(args, "name", "")
	ns := getStringArg(args, "namespace", "default")

	pair, ok := istioKindGVRs[kind]
	if !ok {
		return nil, &types.MCPError{
			Code:    types.ErrCodeInvalidInput,
			Tool:    t.Name(),
			Message: fmt.Sprintf("unsupported Istio resource kind: %s", kind),
		}
	}

	resource, err := getWithFallback(ctx, t.Clients.Dynamic, pair.v1, pair.v1beta1, ns, name)
	if err != nil {
		return nil, &types.MCPError{
			Code:    types.ErrCodeCRDNotAvailable,
			Tool:    t.Name(),
			Message: fmt.Sprintf("failed to get %s %s/%s", kind, ns, name),
			Detail:  fmt.Sprintf("tried %s v1 and v1beta1: %v", pair.apiGroup, err),
		}
	}

	ref := &types.ResourceRef{
		Kind:       kind,
		Namespace:  ns,
		Name:       name,
		APIVersion: pair.apiGroup,
	}

	var findings []types.DiagnosticFinding

	// Main resource finding with summary and full spec in Detail
	summary, _ := istioResourceSummary(kind, resource)
	spec, _, _ := unstructured.NestedMap(resource.Object, "spec")
	specJSON, jsonErr := json.MarshalIndent(spec, "", "  ")
	if jsonErr != nil {
		slog.Warn("istio: failed to marshal spec to JSON", "kind", kind, "name", name, "error", jsonErr)
		specJSON = []byte(fmt.Sprintf("<failed to serialize spec: %v>", jsonErr))
	}

	findings = append(findings, types.DiagnosticFinding{
		Severity: types.SeverityInfo,
		Category: types.CategoryMesh,
		Resource: ref,
		Summary:  summary,
		Detail:   string(specJSON),
	})

	// Extract status validation messages
	status, _, _ := unstructured.NestedMap(resource.Object, "status")
	if status != nil {
		// Istio resources may have status.validationMessages or status.conditions
		if validationMsgs, ok := status["validationMessages"].([]interface{}); ok {
			for _, vm := range validationMsgs {
				if vmm, ok := vm.(map[string]interface{}); ok {
					docName, _ := vmm["documentationUrl"].(string)
					level, _ := vmm["level"].(string)
					msgType, _ := vmm["type"].(map[string]interface{})
					code := ""
					if msgType != nil {
						code, _ = msgType["code"].(string)
					}
					description, _ := vmm["description"].(string)

					severity := types.SeverityInfo
					switch level {
					case "ERROR":
						severity = types.SeverityCritical
					case "WARNING":
						severity = types.SeverityWarning
					}

					findings = append(findings, types.DiagnosticFinding{
						Severity:   severity,
						Category:   types.CategoryMesh,
						Resource:   ref,
						Summary:    fmt.Sprintf("Validation %s: %s", level, code),
						Detail:     description,
						Suggestion: docName,
					})
				}
			}
		}

		// Check status.conditions (some Istio versions use this)
		if conditions, ok := status["conditions"].([]interface{}); ok {
			for _, c := range conditions {
				cm, ok := c.(map[string]interface{})
				if !ok {
					continue
				}
				condStatus, _ := cm["status"].(string)
				condType, _ := cm["type"].(string)
				reason, _ := cm["reason"].(string)
				message, _ := cm["message"].(string)

				if condStatus == "False" {
					findings = append(findings, types.DiagnosticFinding{
						Severity: types.SeverityWarning,
						Category: types.CategoryMesh,
						Resource: ref,
						Summary:  fmt.Sprintf("Condition %s=%s reason=%s", condType, condStatus, reason),
						Detail:   message,
					})
				}
			}
		}
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, "istio"), nil
}

// istioResourceSummary returns a compact summary and optional detail string for an Istio resource.
func istioResourceSummary(kind string, item *unstructured.Unstructured) (string, string) {
	ns := item.GetNamespace()
	name := item.GetName()

	switch kind {
	case "VirtualService":
		hosts, _, _ := unstructured.NestedStringSlice(item.Object, "spec", "hosts")
		gateways, _, _ := unstructured.NestedStringSlice(item.Object, "spec", "gateways")
		httpRoutes, _, _ := unstructured.NestedSlice(item.Object, "spec", "http")
		tcpRoutes, _, _ := unstructured.NestedSlice(item.Object, "spec", "tcp")
		tlsRoutes, _, _ := unstructured.NestedSlice(item.Object, "spec", "tls")
		summary := fmt.Sprintf("%s/%s hosts=[%s] gateways=[%s] http=%d tcp=%d tls=%d",
			ns, name,
			strings.Join(hosts, ", "),
			strings.Join(gateways, ", "),
			len(httpRoutes), len(tcpRoutes), len(tlsRoutes))
		return summary, ""

	case "DestinationRule":
		host, _, _ := unstructured.NestedString(item.Object, "spec", "host")
		subsets, _, _ := unstructured.NestedSlice(item.Object, "spec", "subsets")
		tlsMode, _, _ := unstructured.NestedString(item.Object, "spec", "trafficPolicy", "tls", "mode")
		summary := fmt.Sprintf("%s/%s host=%s subsets=%d", ns, name, host, len(subsets))
		if tlsMode != "" {
			summary += fmt.Sprintf(" tls=%s", tlsMode)
		}
		// Build subset detail
		subsetParts := make([]string, 0, len(subsets))
		for _, s := range subsets {
			if sm, ok := s.(map[string]interface{}); ok {
				sName, _ := sm["name"].(string)
				labels, _, _ := unstructured.NestedStringMap(sm, "labels")
				labelParts := make([]string, 0, len(labels))
				for k, v := range labels {
					labelParts = append(labelParts, fmt.Sprintf("%s=%s", k, v))
				}
				sort.Strings(labelParts)
				subsetParts = append(subsetParts, fmt.Sprintf("%s {%s}", sName, strings.Join(labelParts, ", ")))
			}
		}
		detail := ""
		if len(subsetParts) > 0 {
			detail = "subsets: " + strings.Join(subsetParts, "; ")
		}
		return summary, detail

	case "AuthorizationPolicy":
		action, _, _ := unstructured.NestedString(item.Object, "spec", "action")
		rules, _, _ := unstructured.NestedSlice(item.Object, "spec", "rules")
		if action == "" {
			action = "ALLOW"
		}
		summary := fmt.Sprintf("%s/%s action=%s rules=%d", ns, name, action, len(rules))
		return summary, ""

	case "PeerAuthentication":
		mode, _, _ := unstructured.NestedString(item.Object, "spec", "mtls", "mode")
		if mode == "" {
			mode = "UNSET"
		}
		selector, _, _ := unstructured.NestedMap(item.Object, "spec", "selector", "matchLabels")
		summary := fmt.Sprintf("%s/%s mtls=%s", ns, name, mode)
		if len(selector) > 0 {
			labelParts := make([]string, 0, len(selector))
			for k, v := range selector {
				if vs, ok := v.(string); ok {
					labelParts = append(labelParts, fmt.Sprintf("%s=%s", k, vs))
				}
			}
			sort.Strings(labelParts)
			summary += fmt.Sprintf(" selector={%s}", strings.Join(labelParts, ", "))
		} else {
			summary += " (namespace-wide)"
		}
		return summary, ""
	}

	return fmt.Sprintf("%s/%s", ns, name), ""
}

// --- check_sidecar_injection ---

type CheckSidecarInjectionTool struct{ BaseTool }

func (t *CheckSidecarInjectionTool) Name() string        { return "check_sidecar_injection" }
func (t *CheckSidecarInjectionTool) Description() string  { return "Check Istio sidecar injection status for all deployments in a namespace" }
func (t *CheckSidecarInjectionTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace to check",
			},
		},
		"required": []string{"namespace"},
	}
}

var deploymentsGVR = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

func (t *CheckSidecarInjectionTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	ns := getStringArg(args, "namespace", "default")

	// Check namespace label
	nsObj, err := t.Clients.Dynamic.Resource(schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}).Get(ctx, ns, metav1.GetOptions{})
	nsInjectionLabel := ""
	if err == nil {
		labels := nsObj.GetLabels()
		nsInjectionLabel = labels["istio-injection"]
		if nsInjectionLabel == "" {
			nsInjectionLabel = labels["istio.io/rev"]
		}
	}

	// List deployments
	depList, err := t.Clients.Dynamic.Resource(deploymentsGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments in %s: %w", ns, err)
	}

	deployments := make([]map[string]interface{}, 0, len(depList.Items))
	for _, dep := range depList.Items {
		annotations := dep.GetAnnotations()
		sidecarInject := annotations["sidecar.istio.io/inject"]

		// Check template annotations
		templateAnnotations, _, _ := unstructured.NestedStringMap(dep.Object, "spec", "template", "metadata", "annotations")
		if templateAnnotations["sidecar.istio.io/inject"] != "" {
			sidecarInject = templateAnnotations["sidecar.istio.io/inject"]
		}

		// Check if pods actually have istio-proxy container
		selector, _, _ := unstructured.NestedMap(dep.Object, "spec", "selector", "matchLabels")
		hasSidecar := false
		if len(selector) > 0 {
			labelSelector := ""
			for k, v := range selector {
				if labelSelector != "" {
					labelSelector += ","
				}
				if vs, ok := v.(string); ok {
					labelSelector += k + "=" + vs
				}
			}
			podList, podErr := t.Clients.Dynamic.Resource(podsGVR).Namespace(ns).List(ctx, metav1.ListOptions{
				LabelSelector: labelSelector,
				Limit:         1,
			})
			if podErr == nil && len(podList.Items) > 0 {
				containers, _, _ := unstructured.NestedSlice(podList.Items[0].Object, "spec", "containers")
				for _, c := range containers {
					if cm, ok := c.(map[string]interface{}); ok {
						if name, ok := cm["name"].(string); ok && name == "istio-proxy" {
							hasSidecar = true
							break
						}
					}
				}
			}
		}

		deployments = append(deployments, map[string]interface{}{
			"name":                    dep.GetName(),
			"sidecarInjectAnnotation": sidecarInject,
			"hasSidecar":              hasSidecar,
		})
	}

	return NewResponse(t.Cfg, t.Name(), map[string]interface{}{
		"namespace":          ns,
		"namespaceInjection": nsInjectionLabel,
		"deploymentCount":    len(deployments),
		"deployments":        deployments,
	}), nil
}

// --- check_istio_mtls ---

type CheckIstioMTLSTool struct{ BaseTool }

func (t *CheckIstioMTLSTool) Name() string        { return "check_istio_mtls" }
func (t *CheckIstioMTLSTool) Description() string  { return "Check mTLS mode per namespace: PeerAuthentication policies and DestinationRule TLS settings" }
func (t *CheckIstioMTLSTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace (empty for all namespaces)",
			},
		},
	}
}

func (t *CheckIstioMTLSTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	ns := getStringArg(args, "namespace", "")

	// Get PeerAuthentication policies (v1/v1beta1 fallback)
	paList, err := listWithFallback(ctx, t.Clients.Dynamic, paV1GVR, paV1B1GVR, ns)
	if err != nil {
		return nil, fmt.Errorf("failed to list PeerAuthentication: %w", err)
	}

	paPolicies := make([]map[string]interface{}, 0, len(paList.Items))
	for _, item := range paList.Items {
		mode, _, _ := unstructured.NestedString(item.Object, "spec", "mtls", "mode")
		selector, _, _ := unstructured.NestedMap(item.Object, "spec", "selector")
		paPolicies = append(paPolicies, map[string]interface{}{
			"name":      item.GetName(),
			"namespace": item.GetNamespace(),
			"mtlsMode":  mode,
			"selector":  selector,
		})
	}

	// Get DestinationRule TLS settings (v1/v1beta1 fallback)
	drList, err := listWithFallback(ctx, t.Clients.Dynamic, drV1GVR, drV1B1GVR, ns)
	if err != nil {
		return nil, fmt.Errorf("failed to list DestinationRule: %w", err)
	}

	drPolicies := make([]map[string]interface{}, 0, len(drList.Items))
	for _, item := range drList.Items {
		host, _, _ := unstructured.NestedString(item.Object, "spec", "host")
		tlsMode, _, _ := unstructured.NestedString(item.Object, "spec", "trafficPolicy", "tls", "mode")
		drPolicies = append(drPolicies, map[string]interface{}{
			"name":      item.GetName(),
			"namespace": item.GetNamespace(),
			"host":      host,
			"tlsMode":   tlsMode,
		})
	}

	return NewResponse(t.Cfg, t.Name(), map[string]interface{}{
		"peerAuthentications": paPolicies,
		"destinationRules":    drPolicies,
	}), nil
}
