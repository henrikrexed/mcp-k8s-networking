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
	"k8s.io/client-go/dynamic"

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

func (t *CheckSidecarInjectionTool) Name() string { return "check_sidecar_injection" }
func (t *CheckSidecarInjectionTool) Description() string {
	return "Check Istio sidecar injection status for all deployments in a namespace: namespace label, annotations, actual sidecar presence"
}
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

	var findings []types.DiagnosticFinding

	// Check namespace injection label
	nsInjectionLabel := ""
	nsInjectionEnabled := false
	nsObj, err := t.Clients.Dynamic.Resource(schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}).Get(ctx, ns, metav1.GetOptions{})
	if err != nil {
		return nil, &types.MCPError{
			Code:    types.ErrCodeInternalError,
			Tool:    t.Name(),
			Message: fmt.Sprintf("failed to get namespace %s", ns),
			Detail:  err.Error(),
		}
	}

	labels := nsObj.GetLabels()
	nsInjectionLabel = labels["istio-injection"]
	if nsInjectionLabel == "" {
		nsInjectionLabel = labels["istio.io/rev"]
	}
	nsInjectionEnabled = nsInjectionLabel == "enabled" || (nsInjectionLabel != "" && nsInjectionLabel != "disabled")

	if nsInjectionEnabled {
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryMesh,
			Resource: &types.ResourceRef{Kind: "Namespace", Name: ns},
			Summary:  fmt.Sprintf("Namespace %s has injection label=%s", ns, nsInjectionLabel),
		})
	} else {
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityWarning,
			Category:   types.CategoryMesh,
			Resource:   &types.ResourceRef{Kind: "Namespace", Name: ns},
			Summary:    fmt.Sprintf("Namespace %s does not have Istio injection enabled (label=%q)", ns, nsInjectionLabel),
			Suggestion: "Add label istio-injection=enabled or istio.io/rev=<tag> to enable sidecar injection",
		})
	}

	// List deployments
	depList, err := t.Clients.Dynamic.Resource(deploymentsGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, &types.MCPError{
			Code:    types.ErrCodeInternalError,
			Tool:    t.Name(),
			Message: fmt.Sprintf("failed to list deployments in %s", ns),
			Detail:  err.Error(),
		}
	}

	for _, dep := range depList.Items {
		depName := dep.GetName()
		depRef := &types.ResourceRef{
			Kind:       "Deployment",
			Namespace:  ns,
			Name:       depName,
			APIVersion: "apps/v1",
		}

		// Resolve effective injection annotation (template overrides deployment)
		annotations := dep.GetAnnotations()
		sidecarInject := annotations["sidecar.istio.io/inject"]
		templateAnnotations, _, _ := unstructured.NestedStringMap(dep.Object, "spec", "template", "metadata", "annotations")
		if templateAnnotations["sidecar.istio.io/inject"] != "" {
			sidecarInject = templateAnnotations["sidecar.istio.io/inject"]
		}

		// Check if pods actually have istio-proxy container
		hasSidecar := checkPodHasSidecar(ctx, t.Clients.Dynamic, ns, dep.Object)

		// Determine injection status
		injectionExpected := sidecarInject == "true" || (sidecarInject == "" && nsInjectionEnabled)

		var status string
		var severity string
		var suggestion string

		switch {
		case hasSidecar:
			status = "injected"
			severity = types.SeverityOK
		case injectionExpected && !hasSidecar:
			status = "pending"
			severity = types.SeverityWarning
			suggestion = "Sidecar injection is expected but not present; try restarting the deployment to trigger injection"
		default:
			status = "missing"
			if nsInjectionEnabled && sidecarInject == "false" {
				severity = types.SeverityInfo
				suggestion = "Injection is explicitly disabled via annotation sidecar.istio.io/inject=false"
			} else {
				severity = types.SeverityInfo
			}
		}

		detail := fmt.Sprintf("namespace-injection=%s annotation=%q sidecar-present=%v",
			nsInjectionLabel, sidecarInject, hasSidecar)

		findings = append(findings, types.DiagnosticFinding{
			Severity:   severity,
			Category:   types.CategoryMesh,
			Resource:   depRef,
			Summary:    fmt.Sprintf("Deployment %s/%s injection=%s", ns, depName, status),
			Detail:     detail,
			Suggestion: suggestion,
		})
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, "istio"), nil
}

// checkPodHasSidecar checks if a deployment's pods have the istio-proxy container.
func checkPodHasSidecar(ctx context.Context, client dynamic.Interface, ns string, depObj map[string]interface{}) bool {
	selector, _, _ := unstructured.NestedMap(depObj, "spec", "selector", "matchLabels")
	if len(selector) == 0 {
		return false
	}
	labelParts := make([]string, 0, len(selector))
	for k, v := range selector {
		if vs, ok := v.(string); ok {
			labelParts = append(labelParts, k+"="+vs)
		}
	}
	podList, podErr := client.Resource(podsGVR).Namespace(ns).List(ctx, metav1.ListOptions{
		LabelSelector: strings.Join(labelParts, ","),
		Limit:         1,
	})
	if podErr != nil || len(podList.Items) == 0 {
		return false
	}
	containers, _, _ := unstructured.NestedSlice(podList.Items[0].Object, "spec", "containers")
	for _, c := range containers {
		if cm, ok := c.(map[string]interface{}); ok {
			if name, ok := cm["name"].(string); ok && name == "istio-proxy" {
				return true
			}
		}
	}
	return false
}

// --- check_istio_mtls ---

type CheckIstioMTLSTool struct{ BaseTool }

func (t *CheckIstioMTLSTool) Name() string { return "check_istio_mtls" }
func (t *CheckIstioMTLSTool) Description() string {
	return "Check mTLS configuration: PeerAuthentication policies, DestinationRule TLS settings, and detect STRICT/DISABLE conflicts"
}
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
		return nil, &types.MCPError{
			Code:    types.ErrCodeCRDNotAvailable,
			Tool:    t.Name(),
			Message: "failed to list PeerAuthentication",
			Detail:  fmt.Sprintf("tried security.istio.io v1 and v1beta1: %v", err),
		}
	}

	// Get DestinationRule TLS settings (v1/v1beta1 fallback)
	drList, err := listWithFallback(ctx, t.Clients.Dynamic, drV1GVR, drV1B1GVR, ns)
	if err != nil {
		return nil, &types.MCPError{
			Code:    types.ErrCodeCRDNotAvailable,
			Tool:    t.Name(),
			Message: "failed to list DestinationRule",
			Detail:  fmt.Sprintf("tried networking.istio.io v1 and v1beta1: %v", err),
		}
	}

	var findings []types.DiagnosticFinding

	// Track namespace-wide PeerAuthentication modes for conflict detection
	// key: namespace -> effective mTLS mode
	nsMtlsModes := make(map[string]string)

	// PeerAuthentication findings
	for _, item := range paList.Items {
		paNs := item.GetNamespace()
		paName := item.GetName()
		mode, _, _ := unstructured.NestedString(item.Object, "spec", "mtls", "mode")
		if mode == "" {
			mode = "UNSET"
		}
		selector, _, _ := unstructured.NestedMap(item.Object, "spec", "selector", "matchLabels")

		ref := &types.ResourceRef{
			Kind:       "PeerAuthentication",
			Namespace:  paNs,
			Name:       paName,
			APIVersion: "security.istio.io",
		}

		scope := "namespace-wide"
		if len(selector) > 0 {
			labelParts := make([]string, 0, len(selector))
			for k, v := range selector {
				if vs, ok := v.(string); ok {
					labelParts = append(labelParts, fmt.Sprintf("%s=%s", k, vs))
				}
			}
			sort.Strings(labelParts)
			scope = fmt.Sprintf("selector={%s}", strings.Join(labelParts, ", "))
		} else {
			// Namespace-wide policy — track for conflict detection
			nsMtlsModes[paNs] = mode
		}

		// Mesh-wide policy (istio-system namespace, no selector)
		if paNs == "istio-system" && len(selector) == 0 {
			scope = "mesh-wide"
		}

		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryTLS,
			Resource: ref,
			Summary:  fmt.Sprintf("PeerAuthentication %s/%s mtls=%s (%s)", paNs, paName, mode, scope),
		})
	}

	// DestinationRule findings + conflict detection
	for _, item := range drList.Items {
		drNs := item.GetNamespace()
		drName := item.GetName()
		host, _, _ := unstructured.NestedString(item.Object, "spec", "host")
		tlsMode, _, _ := unstructured.NestedString(item.Object, "spec", "trafficPolicy", "tls", "mode")

		ref := &types.ResourceRef{
			Kind:       "DestinationRule",
			Namespace:  drNs,
			Name:       drName,
			APIVersion: "networking.istio.io",
		}

		if tlsMode == "" {
			// No explicit TLS mode — informational only
			findings = append(findings, types.DiagnosticFinding{
				Severity: types.SeverityInfo,
				Category: types.CategoryTLS,
				Resource: ref,
				Summary:  fmt.Sprintf("DestinationRule %s/%s host=%s tls=<not set>", drNs, drName, host),
			})
			continue
		}

		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryTLS,
			Resource: ref,
			Summary:  fmt.Sprintf("DestinationRule %s/%s host=%s tls=%s", drNs, drName, host, tlsMode),
		})

		// Conflict detection: PeerAuthentication STRICT vs DestinationRule DISABLE
		if tlsMode == "DISABLE" {
			// Check if any PeerAuthentication enforces STRICT for this namespace
			effectivePA := nsMtlsModes[drNs]
			// Also check mesh-wide policy
			if effectivePA == "" {
				effectivePA = nsMtlsModes["istio-system"]
			}

			if effectivePA == "STRICT" {
				findings = append(findings, types.DiagnosticFinding{
					Severity: types.SeverityCritical,
					Category: types.CategoryTLS,
					Resource: ref,
					Summary: fmt.Sprintf("CONFLICT: DestinationRule %s/%s disables TLS for host %s but PeerAuthentication enforces STRICT mTLS",
						drNs, drName, host),
					Detail: fmt.Sprintf("PeerAuthentication in namespace %s sets mtls=STRICT, but DestinationRule %s/%s sets trafficPolicy.tls.mode=DISABLE for host %s. "+
						"This will cause connection failures — clients will send plaintext but the server requires mTLS.",
						drNs, drNs, drName, host),
					Suggestion: "Either change the DestinationRule TLS mode to ISTIO_MUTUAL, or relax the PeerAuthentication to PERMISSIVE",
				})
			}
		}
	}

	if len(findings) == 0 {
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryTLS,
			Summary:  "No PeerAuthentication or DestinationRule TLS policies found; Istio defaults apply",
		})
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, "istio"), nil
}

// --- validate_istio_config ---

type ValidateIstioConfigTool struct{ BaseTool }

func (t *ValidateIstioConfigTool) Name() string { return "validate_istio_config" }
func (t *ValidateIstioConfigTool) Description() string {
	return "Validate Istio VirtualService and DestinationRule configurations: route destinations, subset cross-references, weight sums, TLS settings, and service existence"
}
func (t *ValidateIstioConfigTool) InputSchema() map[string]interface{} {
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

func (t *ValidateIstioConfigTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	ns := getStringArg(args, "namespace", "")

	// Fetch VirtualServices
	vsList, vsErr := listWithFallback(ctx, t.Clients.Dynamic, vsV1GVR, vsV1B1GVR, ns)
	if vsErr != nil {
		return nil, &types.MCPError{
			Code:    types.ErrCodeCRDNotAvailable,
			Tool:    t.Name(),
			Message: "failed to list VirtualService",
			Detail:  fmt.Sprintf("tried networking.istio.io v1 and v1beta1: %v", vsErr),
		}
	}

	// Fetch DestinationRules
	drList, drErr := listWithFallback(ctx, t.Clients.Dynamic, drV1GVR, drV1B1GVR, ns)
	if drErr != nil {
		return nil, &types.MCPError{
			Code:    types.ErrCodeCRDNotAvailable,
			Tool:    t.Name(),
			Message: "failed to list DestinationRule",
			Detail:  fmt.Sprintf("tried networking.istio.io v1 and v1beta1: %v", drErr),
		}
	}

	var findings []types.DiagnosticFinding

	// Validate each VirtualService
	for i := range vsList.Items {
		findings = append(findings, t.validateVirtualService(ctx, &vsList.Items[i], drList)...)
	}

	// Validate each DestinationRule
	for i := range drList.Items {
		findings = append(findings, t.validateDestinationRule(ctx, &drList.Items[i])...)
	}

	if len(findings) == 0 {
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityOK,
			Category: types.CategoryMesh,
			Summary:  "All VirtualService and DestinationRule configurations passed validation",
		})
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, "istio"), nil
}

// validateVirtualService checks a single VirtualService for misconfigurations.
func (t *ValidateIstioConfigTool) validateVirtualService(ctx context.Context, vs *unstructured.Unstructured, drList *unstructured.UnstructuredList) []types.DiagnosticFinding {
	vsNs := vs.GetNamespace()
	vsName := vs.GetName()
	ref := &types.ResourceRef{
		Kind:       "VirtualService",
		Namespace:  vsNs,
		Name:       vsName,
		APIVersion: "networking.istio.io",
	}

	var findings []types.DiagnosticFinding

	// Check hosts
	hosts, _, _ := unstructured.NestedStringSlice(vs.Object, "spec", "hosts")
	if len(hosts) == 0 {
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityWarning,
			Category:   types.CategoryMesh,
			Resource:   ref,
			Summary:    fmt.Sprintf("VirtualService %s/%s has no hosts defined", vsNs, vsName),
			Suggestion: "Add at least one host in spec.hosts",
		})
	}

	// Validate HTTP routes
	httpRoutes, _, _ := unstructured.NestedSlice(vs.Object, "spec", "http")
	for ri, route := range httpRoutes {
		routeMap, ok := route.(map[string]interface{})
		if !ok {
			continue
		}

		// Check catch-all route ordering
		matches, _, _ := unstructured.NestedSlice(routeMap, "match")
		if len(matches) == 0 && ri < len(httpRoutes)-1 {
			findings = append(findings, types.DiagnosticFinding{
				Severity: types.SeverityWarning,
				Category: types.CategoryMesh,
				Resource: ref,
				Summary:  fmt.Sprintf("VirtualService %s/%s http route[%d] is a catch-all but not the last route", vsNs, vsName, ri),
				Detail:   "Routes without match conditions match all requests. When placed before other routes, subsequent routes become unreachable.",
				Suggestion: "Move the catch-all route to the end of the route list",
			})
		}

		// Validate route destinations
		routeDests, _, _ := unstructured.NestedSlice(routeMap, "route")
		totalWeight := 0
		hasExplicitWeight := false

		for di, dest := range routeDests {
			destMap, ok := dest.(map[string]interface{})
			if !ok {
				continue
			}

			destHost, _, _ := unstructured.NestedString(destMap, "destination", "host")
			destSubset, _, _ := unstructured.NestedString(destMap, "destination", "subset")
			weight, weightFound, _ := unstructured.NestedFloat64(destMap, "weight")

			if weightFound {
				hasExplicitWeight = true
				totalWeight += int(weight)
			}

			if destHost == "" {
				findings = append(findings, types.DiagnosticFinding{
					Severity:   types.SeverityCritical,
					Category:   types.CategoryMesh,
					Resource:   ref,
					Summary:    fmt.Sprintf("VirtualService %s/%s http route[%d].route[%d] has no destination host", vsNs, vsName, ri, di),
					Suggestion: "Set destination.host to a valid service name",
				})
				continue
			}

			// Verify destination service exists
			svcNs, svcName := resolveIstioHost(destHost, vsNs)
			_, svcErr := t.Clients.Dynamic.Resource(servicesGVR).Namespace(svcNs).Get(ctx, svcName, metav1.GetOptions{})
			if svcErr != nil {
				findings = append(findings, types.DiagnosticFinding{
					Severity:   types.SeverityWarning,
					Category:   types.CategoryMesh,
					Resource:   ref,
					Summary:    fmt.Sprintf("VirtualService %s/%s route destination host %q may not exist as a Service in %s", vsNs, vsName, destHost, svcNs),
					Detail:     fmt.Sprintf("Service lookup failed: %v", svcErr),
					Suggestion: "Verify the destination host matches an existing Kubernetes Service",
				})
			}

			// Verify subset exists in DestinationRule
			if destSubset != "" && !subsetExists(drList, destHost, destSubset, vsNs) {
				findings = append(findings, types.DiagnosticFinding{
					Severity:   types.SeverityCritical,
					Category:   types.CategoryMesh,
					Resource:   ref,
					Summary:    fmt.Sprintf("VirtualService %s/%s references subset %q for host %q but no matching DestinationRule subset found", vsNs, vsName, destSubset, destHost),
					Suggestion: "Create a DestinationRule with a matching subset definition, or remove the subset reference",
				})
			}
		}

		// Validate weight sum
		if hasExplicitWeight && len(routeDests) > 1 && totalWeight != 100 {
			findings = append(findings, types.DiagnosticFinding{
				Severity:   types.SeverityWarning,
				Category:   types.CategoryMesh,
				Resource:   ref,
				Summary:    fmt.Sprintf("VirtualService %s/%s http route[%d] weight sum is %d (expected 100)", vsNs, vsName, ri, totalWeight),
				Suggestion: "Adjust route weights to sum to 100",
			})
		}
	}

	// Validate TCP routes
	tcpRoutes, _, _ := unstructured.NestedSlice(vs.Object, "spec", "tcp")
	for ri, route := range tcpRoutes {
		routeMap, ok := route.(map[string]interface{})
		if !ok {
			continue
		}
		routeDests, _, _ := unstructured.NestedSlice(routeMap, "route")
		for di, dest := range routeDests {
			destMap, ok := dest.(map[string]interface{})
			if !ok {
				continue
			}
			destHost, _, _ := unstructured.NestedString(destMap, "destination", "host")
			if destHost == "" {
				findings = append(findings, types.DiagnosticFinding{
					Severity:   types.SeverityCritical,
					Category:   types.CategoryMesh,
					Resource:   ref,
					Summary:    fmt.Sprintf("VirtualService %s/%s tcp route[%d].route[%d] has no destination host", vsNs, vsName, ri, di),
					Suggestion: "Set destination.host to a valid service name",
				})
			}
		}
	}

	return findings
}

// validateDestinationRule checks a single DestinationRule for misconfigurations.
func (t *ValidateIstioConfigTool) validateDestinationRule(ctx context.Context, dr *unstructured.Unstructured) []types.DiagnosticFinding {
	drNs := dr.GetNamespace()
	drName := dr.GetName()
	ref := &types.ResourceRef{
		Kind:       "DestinationRule",
		Namespace:  drNs,
		Name:       drName,
		APIVersion: "networking.istio.io",
	}

	var findings []types.DiagnosticFinding

	// Verify host service exists
	host, _, _ := unstructured.NestedString(dr.Object, "spec", "host")
	if host != "" {
		svcNs, svcName := resolveIstioHost(host, drNs)
		_, svcErr := t.Clients.Dynamic.Resource(servicesGVR).Namespace(svcNs).Get(ctx, svcName, metav1.GetOptions{})
		if svcErr != nil {
			findings = append(findings, types.DiagnosticFinding{
				Severity:   types.SeverityWarning,
				Category:   types.CategoryMesh,
				Resource:   ref,
				Summary:    fmt.Sprintf("DestinationRule %s/%s host %q may not exist as a Service in %s", drNs, drName, host, svcNs),
				Detail:     fmt.Sprintf("Service lookup failed: %v", svcErr),
				Suggestion: "Verify the host matches an existing Kubernetes Service",
			})
		}
	}

	// Validate subsets — check that labels match at least one pod
	subsets, _, _ := unstructured.NestedSlice(dr.Object, "spec", "subsets")
	for _, s := range subsets {
		sm, ok := s.(map[string]interface{})
		if !ok {
			continue
		}
		subsetName, _ := sm["name"].(string)
		labels, _, _ := unstructured.NestedStringMap(sm, "labels")
		if len(labels) == 0 {
			continue
		}

		// Build label selector
		labelParts := make([]string, 0, len(labels))
		for k, v := range labels {
			labelParts = append(labelParts, k+"="+v)
		}
		sort.Strings(labelParts)
		labelSelector := strings.Join(labelParts, ",")

		// Check if any pods match
		svcNs, _ := resolveIstioHost(host, drNs)
		podList, podErr := t.Clients.Dynamic.Resource(podsGVR).Namespace(svcNs).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
			Limit:         1,
		})
		if podErr == nil && len(podList.Items) == 0 {
			findings = append(findings, types.DiagnosticFinding{
				Severity:   types.SeverityWarning,
				Category:   types.CategoryMesh,
				Resource:   ref,
				Summary:    fmt.Sprintf("DestinationRule %s/%s subset %q labels {%s} match no pods in %s", drNs, drName, subsetName, labelSelector, svcNs),
				Suggestion: "Verify subset labels match the pod template labels of the target deployment",
			})
		}
	}

	// Validate TLS settings
	tlsMode, _, _ := unstructured.NestedString(dr.Object, "spec", "trafficPolicy", "tls", "mode")
	if tlsMode == "MUTUAL" {
		// MUTUAL mode requires client cert/key
		clientCert, _, _ := unstructured.NestedString(dr.Object, "spec", "trafficPolicy", "tls", "clientCertificate")
		privateKey, _, _ := unstructured.NestedString(dr.Object, "spec", "trafficPolicy", "tls", "privateKey")
		if clientCert == "" || privateKey == "" {
			findings = append(findings, types.DiagnosticFinding{
				Severity:   types.SeverityCritical,
				Category:   types.CategoryTLS,
				Resource:   ref,
				Summary:    fmt.Sprintf("DestinationRule %s/%s TLS mode is MUTUAL but missing client certificate or private key", drNs, drName),
				Detail:     fmt.Sprintf("clientCertificate=%q privateKey=%q", clientCert, privateKey),
				Suggestion: "Set trafficPolicy.tls.clientCertificate and trafficPolicy.tls.privateKey for MUTUAL TLS, or use ISTIO_MUTUAL to let Istio manage certificates",
			})
		}
	}

	// Validate connection pool settings
	maxConnections, connFound, _ := unstructured.NestedFloat64(dr.Object, "spec", "trafficPolicy", "connectionPool", "tcp", "maxConnections")
	if connFound && maxConnections <= 0 {
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityWarning,
			Category:   types.CategoryMesh,
			Resource:   ref,
			Summary:    fmt.Sprintf("DestinationRule %s/%s has tcp.maxConnections=%d (non-positive)", drNs, drName, int(maxConnections)),
			Suggestion: "Set a positive value for connectionPool.tcp.maxConnections",
		})
	}

	http1MaxPending, pendingFound, _ := unstructured.NestedFloat64(dr.Object, "spec", "trafficPolicy", "connectionPool", "http", "h2UpgradePolicy")
	_ = http1MaxPending
	_ = pendingFound

	return findings
}

// subsetExists checks whether a named subset is defined in any DestinationRule for the given host.
func subsetExists(drList *unstructured.UnstructuredList, host, subsetName, vsNamespace string) bool {
	_, hostSvc := resolveIstioHost(host, vsNamespace)
	for _, dr := range drList.Items {
		drHost, _, _ := unstructured.NestedString(dr.Object, "spec", "host")
		_, drSvc := resolveIstioHost(drHost, dr.GetNamespace())
		if drSvc != hostSvc {
			continue
		}
		subsets, _, _ := unstructured.NestedSlice(dr.Object, "spec", "subsets")
		for _, s := range subsets {
			if sm, ok := s.(map[string]interface{}); ok {
				if name, _ := sm["name"].(string); name == subsetName {
					return true
				}
			}
		}
	}
	return false
}

// resolveIstioHost parses an Istio host string and returns (namespace, serviceName).
// Supports formats: "svc", "svc.ns", "svc.ns.svc.cluster.local".
func resolveIstioHost(host, defaultNs string) (string, string) {
	// Remove trailing .svc.cluster.local
	host = strings.TrimSuffix(host, ".svc.cluster.local")

	parts := strings.SplitN(host, ".", 2)
	if len(parts) == 2 {
		return parts[1], parts[0]
	}
	return defaultNs, host
}

// --- analyze_istio_authpolicy ---

type AnalyzeIstioAuthPolicyTool struct{ BaseTool }

func (t *AnalyzeIstioAuthPolicyTool) Name() string { return "analyze_istio_authpolicy" }
func (t *AnalyzeIstioAuthPolicyTool) Description() string {
	return "Analyze Istio AuthorizationPolicy resources: report actions, matched workloads, rule summaries, broad deny detection, and ALLOW/DENY conflicts"
}
func (t *AnalyzeIstioAuthPolicyTool) InputSchema() map[string]interface{} {
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

func (t *AnalyzeIstioAuthPolicyTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	ns := getStringArg(args, "namespace", "")

	apList, err := listWithFallback(ctx, t.Clients.Dynamic, apV1GVR, apV1B1GVR, ns)
	if err != nil {
		return nil, &types.MCPError{
			Code:    types.ErrCodeCRDNotAvailable,
			Tool:    t.Name(),
			Message: "failed to list AuthorizationPolicy",
			Detail:  fmt.Sprintf("tried security.istio.io v1 and v1beta1: %v", err),
		}
	}

	var findings []types.DiagnosticFinding

	// Track policies by selector key for conflict detection.
	// key: sorted "label=value,..." string (empty string = namespace-wide)
	type policyEntry struct {
		action    string
		namespace string
		name      string
	}
	selectorPolicies := make(map[string][]policyEntry)

	for _, item := range apList.Items {
		apNs := item.GetNamespace()
		apName := item.GetName()
		ref := &types.ResourceRef{
			Kind:       "AuthorizationPolicy",
			Namespace:  apNs,
			Name:       apName,
			APIVersion: "security.istio.io",
		}

		action, _, _ := unstructured.NestedString(item.Object, "spec", "action")
		if action == "" {
			action = "ALLOW"
		}
		rules, _, _ := unstructured.NestedSlice(item.Object, "spec", "rules")
		selector, _, _ := unstructured.NestedMap(item.Object, "spec", "selector", "matchLabels")

		// Build selector key
		selectorKey := authPolicySelectorKey(selector)
		selectorPolicies[selectorKey] = append(selectorPolicies[selectorKey], policyEntry{
			action:    action,
			namespace: apNs,
			name:      apName,
		})

		// Build scope description
		scope := "namespace-wide"
		if len(selector) > 0 {
			labelParts := make([]string, 0, len(selector))
			for k, v := range selector {
				if vs, ok := v.(string); ok {
					labelParts = append(labelParts, fmt.Sprintf("%s=%s", k, vs))
				}
			}
			sort.Strings(labelParts)
			scope = fmt.Sprintf("selector={%s}", strings.Join(labelParts, ", "))
		}

		// Build rule summaries
		ruleSummaries := authPolicyRuleSummaries(rules)

		summary := fmt.Sprintf("AuthorizationPolicy %s/%s action=%s rules=%d (%s)", apNs, apName, action, len(rules), scope)
		detail := ""
		if len(ruleSummaries) > 0 {
			detail = strings.Join(ruleSummaries, "\n")
		}

		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryPolicy,
			Resource: ref,
			Summary:  summary,
			Detail:   detail,
		})

		// Detect broad DENY: DENY action with no rules = deny all traffic
		if action == "DENY" && len(rules) == 0 {
			findings = append(findings, types.DiagnosticFinding{
				Severity:   types.SeverityCritical,
				Category:   types.CategoryPolicy,
				Resource:   ref,
				Summary:    fmt.Sprintf("AuthorizationPolicy %s/%s is a blanket DENY with no rules — blocks ALL traffic (%s)", apNs, apName, scope),
				Detail:     "A DENY policy with no rules matches all requests. This will block all traffic to the targeted workloads.",
				Suggestion: "Add specific rules to narrow the deny scope, or remove this policy if it was created in error",
			})
		}

		// Detect broad DENY: DENY with rules that have no constraints
		if action == "DENY" && len(rules) > 0 {
			for ri, rule := range rules {
				ruleMap, ok := rule.(map[string]interface{})
				if !ok {
					continue
				}
				if isUnconstrainedRule(ruleMap) {
					findings = append(findings, types.DiagnosticFinding{
						Severity:   types.SeverityWarning,
						Category:   types.CategoryPolicy,
						Resource:   ref,
						Summary:    fmt.Sprintf("AuthorizationPolicy %s/%s DENY rule[%d] has no from/to/when constraints — matches all traffic", apNs, apName, ri),
						Suggestion: "Add from, to, or when conditions to narrow the deny rule scope",
					})
				}
			}
		}

		// Detect ALLOW with no rules — effectively denies all traffic (allowlist with empty list)
		if action == "ALLOW" && len(rules) == 0 {
			findings = append(findings, types.DiagnosticFinding{
				Severity:   types.SeverityWarning,
				Category:   types.CategoryPolicy,
				Resource:   ref,
				Summary:    fmt.Sprintf("AuthorizationPolicy %s/%s is ALLOW with no rules — effectively denies all traffic (%s)", apNs, apName, scope),
				Detail:     "An ALLOW policy with no rules means no requests are explicitly allowed. Combined with Istio's deny-by-default when any ALLOW policy exists, this blocks all traffic.",
				Suggestion: "Add rules to specify which traffic should be allowed, or remove this policy",
			})
		}
	}

	// Conflict detection: ALLOW and DENY policies targeting the same workload selector
	for selectorKey, policies := range selectorPolicies {
		if len(policies) < 2 {
			continue
		}
		hasAllow := false
		hasDeny := false
		var allowNames, denyNames []string
		for _, p := range policies {
			switch p.action {
			case "ALLOW":
				hasAllow = true
				allowNames = append(allowNames, p.namespace+"/"+p.name)
			case "DENY":
				hasDeny = true
				denyNames = append(denyNames, p.namespace+"/"+p.name)
			}
		}
		if hasAllow && hasDeny {
			sort.Strings(allowNames)
			sort.Strings(denyNames)
			selectorDesc := "namespace-wide"
			if selectorKey != "" {
				selectorDesc = fmt.Sprintf("selector={%s}", selectorKey)
			}
			findings = append(findings, types.DiagnosticFinding{
				Severity: types.SeverityWarning,
				Category: types.CategoryPolicy,
				Summary:  fmt.Sprintf("Conflicting ALLOW and DENY policies target the same workloads (%s)", selectorDesc),
				Detail: fmt.Sprintf("ALLOW policies: %s\nDENY policies: %s\n"+
					"When both ALLOW and DENY policies apply, DENY takes precedence. Ensure the ALLOW rules do not overlap with DENY rules, or traffic may be unexpectedly blocked.",
					strings.Join(allowNames, ", "), strings.Join(denyNames, ", ")),
				Suggestion: "Review policy rules to ensure ALLOW and DENY scopes don't unintentionally overlap",
			})
		}
	}

	if len(findings) == 0 {
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryPolicy,
			Summary:  "No AuthorizationPolicy resources found; Istio defaults apply (all traffic allowed)",
		})
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, "istio"), nil
}

// authPolicySelectorKey returns a deterministic string key for a selector map.
func authPolicySelectorKey(selector map[string]interface{}) string {
	if len(selector) == 0 {
		return ""
	}
	parts := make([]string, 0, len(selector))
	for k, v := range selector {
		if vs, ok := v.(string); ok {
			parts = append(parts, k+"="+vs)
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

// authPolicyRuleSummaries returns human-readable summaries for each rule in an AuthorizationPolicy.
func authPolicyRuleSummaries(rules []interface{}) []string {
	summaries := make([]string, 0, len(rules))
	for i, rule := range rules {
		ruleMap, ok := rule.(map[string]interface{})
		if !ok {
			continue
		}
		parts := []string{fmt.Sprintf("rule[%d]:", i)}

		// Summarize "from" sources
		if from, ok := ruleMap["from"].([]interface{}); ok && len(from) > 0 {
			var sources []string
			for _, f := range from {
				if fm, ok := f.(map[string]interface{}); ok {
					if source, ok := fm["source"].(map[string]interface{}); ok {
						for field, val := range source {
							if vals, ok := val.([]interface{}); ok {
								strs := make([]string, 0, len(vals))
								for _, v := range vals {
									if s, ok := v.(string); ok {
										strs = append(strs, s)
									}
								}
								sources = append(sources, fmt.Sprintf("%s=[%s]", field, strings.Join(strs, ",")))
							}
						}
					}
				}
			}
			sort.Strings(sources)
			parts = append(parts, "from{"+strings.Join(sources, " ")+"}")
		}

		// Summarize "to" operations
		if to, ok := ruleMap["to"].([]interface{}); ok && len(to) > 0 {
			var ops []string
			for _, t := range to {
				if tm, ok := t.(map[string]interface{}); ok {
					if operation, ok := tm["operation"].(map[string]interface{}); ok {
						for field, val := range operation {
							if vals, ok := val.([]interface{}); ok {
								strs := make([]string, 0, len(vals))
								for _, v := range vals {
									if s, ok := v.(string); ok {
										strs = append(strs, s)
									}
								}
								ops = append(ops, fmt.Sprintf("%s=[%s]", field, strings.Join(strs, ",")))
							}
						}
					}
				}
			}
			sort.Strings(ops)
			parts = append(parts, "to{"+strings.Join(ops, " ")+"}")
		}

		// Summarize "when" conditions
		if when, ok := ruleMap["when"].([]interface{}); ok && len(when) > 0 {
			parts = append(parts, fmt.Sprintf("when(%d conditions)", len(when)))
		}

		summaries = append(summaries, strings.Join(parts, " "))
	}
	return summaries
}

// --- analyze_istio_routing ---

type AnalyzeIstioRoutingTool struct{ BaseTool }

func (t *AnalyzeIstioRoutingTool) Name() string { return "analyze_istio_routing" }
func (t *AnalyzeIstioRoutingTool) Description() string {
	return "Analyze Istio traffic routing end-to-end for a service: VirtualService routes, DestinationRule subsets, service endpoints, weight sums, shadowed rules, and AuthorizationPolicy deny conflicts"
}
func (t *AnalyzeIstioRoutingTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"service": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes Service name to analyze routing for",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace",
			},
		},
		"required": []string{"service", "namespace"},
	}
}

func (t *AnalyzeIstioRoutingTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	svcName := getStringArg(args, "service", "")
	ns := getStringArg(args, "namespace", "default")

	if svcName == "" {
		return nil, &types.MCPError{
			Code:    types.ErrCodeInvalidInput,
			Tool:    t.Name(),
			Message: "service name is required",
		}
	}

	// Verify service exists and check endpoints
	svc, svcErr := t.Clients.Dynamic.Resource(servicesGVR).Namespace(ns).Get(ctx, svcName, metav1.GetOptions{})
	if svcErr != nil {
		return nil, &types.MCPError{
			Code:    types.ErrCodeInvalidInput,
			Tool:    t.Name(),
			Message: fmt.Sprintf("service %s/%s not found", ns, svcName),
			Detail:  svcErr.Error(),
		}
	}

	var findings []types.DiagnosticFinding

	svcRef := &types.ResourceRef{
		Kind:      "Service",
		Namespace: ns,
		Name:      svcName,
	}

	// Check endpoints
	ep, epErr := t.Clients.Dynamic.Resource(endpointsGVR).Namespace(ns).Get(ctx, svcName, metav1.GetOptions{})
	readyAddresses := 0
	if epErr == nil {
		subsets, _, _ := unstructured.NestedSlice(ep.Object, "subsets")
		for _, s := range subsets {
			if sm, ok := s.(map[string]interface{}); ok {
				if addrs, ok := sm["addresses"].([]interface{}); ok {
					readyAddresses += len(addrs)
				}
			}
		}
	}
	if readyAddresses == 0 {
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityWarning,
			Category:   types.CategoryRouting,
			Resource:   svcRef,
			Summary:    fmt.Sprintf("Service %s/%s has 0 ready endpoints", ns, svcName),
			Suggestion: "Check that pods matching the service selector are running and ready",
		})
	} else {
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryRouting,
			Resource: svcRef,
			Summary:  fmt.Sprintf("Service %s/%s has %d ready endpoint(s)", ns, svcName, readyAddresses),
		})
	}

	// Fetch VirtualServices in namespace
	vsList, vsErr := listWithFallback(ctx, t.Clients.Dynamic, vsV1GVR, vsV1B1GVR, ns)
	if vsErr != nil {
		return nil, &types.MCPError{
			Code:    types.ErrCodeCRDNotAvailable,
			Tool:    t.Name(),
			Message: "failed to list VirtualService",
			Detail:  fmt.Sprintf("tried networking.istio.io v1 and v1beta1: %v", vsErr),
		}
	}

	// Fetch DestinationRules in namespace
	drList, drErr := listWithFallback(ctx, t.Clients.Dynamic, drV1GVR, drV1B1GVR, ns)
	if drErr != nil {
		return nil, &types.MCPError{
			Code:    types.ErrCodeCRDNotAvailable,
			Tool:    t.Name(),
			Message: "failed to list DestinationRule",
			Detail:  fmt.Sprintf("tried networking.istio.io v1 and v1beta1: %v", drErr),
		}
	}

	// Find VirtualServices that reference this service
	matchingVS := filterVSForService(vsList, svcName, ns)
	if len(matchingVS) == 0 {
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryRouting,
			Resource: svcRef,
			Summary:  fmt.Sprintf("No VirtualService routes traffic to %s/%s — default Kubernetes routing applies", ns, svcName),
		})
	}

	// Find DestinationRule for this service
	var matchingDR *unstructured.Unstructured
	for i, dr := range drList.Items {
		drHost, _, _ := unstructured.NestedString(dr.Object, "spec", "host")
		_, drSvc := resolveIstioHost(drHost, dr.GetNamespace())
		if drSvc == svcName {
			matchingDR = &drList.Items[i]
			break
		}
	}

	// Collect defined subsets from the DestinationRule
	definedSubsets := make(map[string]bool)
	if matchingDR != nil {
		subsets, _, _ := unstructured.NestedSlice(matchingDR.Object, "spec", "subsets")
		for _, s := range subsets {
			if sm, ok := s.(map[string]interface{}); ok {
				if name, _ := sm["name"].(string); name != "" {
					definedSubsets[name] = true
				}
			}
		}
		drRef := &types.ResourceRef{
			Kind:       "DestinationRule",
			Namespace:  matchingDR.GetNamespace(),
			Name:       matchingDR.GetName(),
			APIVersion: "networking.istio.io",
		}
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryRouting,
			Resource: drRef,
			Summary:  fmt.Sprintf("DestinationRule %s/%s defines %d subset(s) for %s", matchingDR.GetNamespace(), matchingDR.GetName(), len(definedSubsets), svcName),
		})
	}

	// Analyze each matching VirtualService
	for _, vs := range matchingVS {
		vsRef := &types.ResourceRef{
			Kind:       "VirtualService",
			Namespace:  vs.GetNamespace(),
			Name:       vs.GetName(),
			APIVersion: "networking.istio.io",
		}

		httpRoutes, _, _ := unstructured.NestedSlice(vs.Object, "spec", "http")

		// Track match signatures to detect shadowed routes
		var seenCatchAll bool

		for ri, route := range httpRoutes {
			routeMap, ok := route.(map[string]interface{})
			if !ok {
				continue
			}

			matches, _, _ := unstructured.NestedSlice(routeMap, "match")
			isCatchAll := len(matches) == 0

			// Shadowed route detection: any route after a catch-all is unreachable
			if seenCatchAll {
				findings = append(findings, types.DiagnosticFinding{
					Severity:   types.SeverityWarning,
					Category:   types.CategoryRouting,
					Resource:   vsRef,
					Summary:    fmt.Sprintf("VirtualService %s/%s http route[%d] is unreachable — shadowed by a catch-all route above it", vs.GetNamespace(), vs.GetName(), ri),
					Detail:     "A previous route has no match conditions and matches all requests. This route will never be evaluated.",
					Suggestion: "Reorder routes so specific matches come before catch-all routes",
				})
			}

			// Detect prefix-shadowed routes: a broader prefix match before a narrower one
			if !isCatchAll && !seenCatchAll {
				findings = append(findings, t.detectShadowedMatches(vs, httpRoutes, ri, matches)...)
			}

			if isCatchAll {
				seenCatchAll = true
			}

			// Analyze route destinations
			routeDests, _, _ := unstructured.NestedSlice(routeMap, "route")
			totalWeight := 0
			hasExplicitWeight := false

			for di, dest := range routeDests {
				destMap, ok := dest.(map[string]interface{})
				if !ok {
					continue
				}

				destHost, _, _ := unstructured.NestedString(destMap, "destination", "host")
				destSubset, _, _ := unstructured.NestedString(destMap, "destination", "subset")
				weight, weightFound, _ := unstructured.NestedFloat64(destMap, "weight")

				if weightFound {
					hasExplicitWeight = true
					totalWeight += int(weight)
				}

				// Check if destination host resolves to our target service or another
				_, destSvc := resolveIstioHost(destHost, ns)
				if destSvc == svcName {
					// Check subset existence
					if destSubset != "" && !definedSubsets[destSubset] {
						findings = append(findings, types.DiagnosticFinding{
							Severity: types.SeverityCritical,
							Category: types.CategoryRouting,
							Resource: vsRef,
							Summary:  fmt.Sprintf("VirtualService %s/%s route[%d].route[%d] references non-existent subset %q for %s", vs.GetNamespace(), vs.GetName(), ri, di, destSubset, svcName),
							Detail: func() string {
								if matchingDR == nil {
									return fmt.Sprintf("No DestinationRule found for host %s — subset references cannot be resolved", svcName)
								}
								names := make([]string, 0, len(definedSubsets))
								for n := range definedSubsets {
									names = append(names, n)
								}
								sort.Strings(names)
								return fmt.Sprintf("Available subsets in DestinationRule: [%s]", strings.Join(names, ", "))
							}(),
							Suggestion: "Create the subset in the DestinationRule or correct the subset name",
						})
					}

					// If subset required but no DR exists
					if destSubset != "" && matchingDR == nil {
						findings = append(findings, types.DiagnosticFinding{
							Severity:   types.SeverityCritical,
							Category:   types.CategoryRouting,
							Resource:   vsRef,
							Summary:    fmt.Sprintf("VirtualService %s/%s route[%d].route[%d] references subset %q but no DestinationRule exists for %s", vs.GetNamespace(), vs.GetName(), ri, di, destSubset, svcName),
							Suggestion: "Create a DestinationRule with subset definitions for this service",
						})
					}
				}
			}

			// Weight sum validation
			if hasExplicitWeight && len(routeDests) > 1 && totalWeight != 100 {
				findings = append(findings, types.DiagnosticFinding{
					Severity:   types.SeverityCritical,
					Category:   types.CategoryRouting,
					Resource:   vsRef,
					Summary:    fmt.Sprintf("VirtualService %s/%s http route[%d] weight sum is %d (must be 100)", vs.GetNamespace(), vs.GetName(), ri, totalWeight),
					Suggestion: "Adjust route destination weights to sum to exactly 100",
				})
			}
		}
	}

	// Check for AuthorizationPolicy DENY conflicts
	findings = append(findings, t.checkAuthPolicyConflicts(ctx, svc, svcName, ns)...)

	if len(findings) == 0 {
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityOK,
			Category: types.CategoryRouting,
			Summary:  fmt.Sprintf("Routing analysis for %s/%s found no issues", ns, svcName),
		})
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, "istio"), nil
}

// filterVSForService returns VirtualServices whose HTTP or TCP route destinations reference the given service.
func filterVSForService(vsList *unstructured.UnstructuredList, svcName, ns string) []*unstructured.Unstructured {
	var result []*unstructured.Unstructured
	for i, vs := range vsList.Items {
		if vsReferencesService(&vs, svcName, ns) {
			result = append(result, &vsList.Items[i])
		}
	}
	return result
}

// vsReferencesService checks if a VirtualService has any route destination targeting the given service.
func vsReferencesService(vs *unstructured.Unstructured, svcName, ns string) bool {
	for _, routeType := range []string{"http", "tcp", "tls"} {
		routes, _, _ := unstructured.NestedSlice(vs.Object, "spec", routeType)
		for _, route := range routes {
			routeMap, ok := route.(map[string]interface{})
			if !ok {
				continue
			}
			dests, _, _ := unstructured.NestedSlice(routeMap, "route")
			for _, dest := range dests {
				destMap, ok := dest.(map[string]interface{})
				if !ok {
					continue
				}
				destHost, _, _ := unstructured.NestedString(destMap, "destination", "host")
				_, destSvc := resolveIstioHost(destHost, vs.GetNamespace())
				if destSvc == svcName {
					return true
				}
			}
		}
	}

	// Also check spec.hosts for the service name
	hosts, _, _ := unstructured.NestedStringSlice(vs.Object, "spec", "hosts")
	for _, h := range hosts {
		_, hostSvc := resolveIstioHost(h, vs.GetNamespace())
		if hostSvc == svcName {
			return true
		}
	}

	return false
}

// detectShadowedMatches checks if route[ri] is shadowed by a broader match in a preceding route.
func (t *AnalyzeIstioRoutingTool) detectShadowedMatches(vs *unstructured.Unstructured, httpRoutes []interface{}, ri int, currentMatches []interface{}) []types.DiagnosticFinding {
	var findings []types.DiagnosticFinding
	vsRef := &types.ResourceRef{
		Kind:       "VirtualService",
		Namespace:  vs.GetNamespace(),
		Name:       vs.GetName(),
		APIVersion: "networking.istio.io",
	}

	for pi := 0; pi < ri; pi++ {
		prevRoute, ok := httpRoutes[pi].(map[string]interface{})
		if !ok {
			continue
		}
		prevMatches, _, _ := unstructured.NestedSlice(prevRoute, "match")
		if len(prevMatches) == 0 {
			// Already handled by catch-all detection
			continue
		}

		// Check URI prefix shadowing: if a previous route has a shorter or equal prefix
		// that covers the current route's prefix
		for _, cm := range currentMatches {
			cmMap, ok := cm.(map[string]interface{})
			if !ok {
				continue
			}
			curPrefix := extractMatchPrefix(cmMap)
			if curPrefix == "" {
				continue
			}
			for _, pm := range prevMatches {
				pmMap, ok := pm.(map[string]interface{})
				if !ok {
					continue
				}
				prevPrefix := extractMatchPrefix(pmMap)
				if prevPrefix == "" {
					continue
				}
				if prevPrefix != curPrefix && strings.HasPrefix(curPrefix, prevPrefix) {
					findings = append(findings, types.DiagnosticFinding{
						Severity: types.SeverityWarning,
						Category: types.CategoryRouting,
						Resource: vsRef,
						Summary:  fmt.Sprintf("VirtualService %s/%s http route[%d] prefix %q may be shadowed by route[%d] prefix %q", vs.GetNamespace(), vs.GetName(), ri, curPrefix, pi, prevPrefix),
						Detail:   fmt.Sprintf("Route[%d] matches prefix %q which is a superset of route[%d] prefix %q. The broader route will match first.", pi, prevPrefix, ri, curPrefix),
						Suggestion: "Reorder routes so more specific prefixes come before broader ones",
					})
				}
			}
		}
	}

	return findings
}

// extractMatchPrefix extracts the URI prefix from an HTTP match condition.
func extractMatchPrefix(match map[string]interface{}) string {
	uri, ok := match["uri"].(map[string]interface{})
	if !ok {
		return ""
	}
	if prefix, ok := uri["prefix"].(string); ok {
		return prefix
	}
	return ""
}

// checkAuthPolicyConflicts checks if any DENY AuthorizationPolicy targets this service's workloads.
func (t *AnalyzeIstioRoutingTool) checkAuthPolicyConflicts(ctx context.Context, svc *unstructured.Unstructured, svcName, ns string) []types.DiagnosticFinding {
	apList, err := listWithFallback(ctx, t.Clients.Dynamic, apV1GVR, apV1B1GVR, ns)
	if err != nil {
		// Non-fatal — just skip AuthorizationPolicy analysis
		slog.Debug("analyze_istio_routing: skipping AuthorizationPolicy check", "error", err)
		return nil
	}

	// Get service selector to match against AP workload selectors
	svcSelector, _, _ := unstructured.NestedStringMap(svc.Object, "spec", "selector")
	if len(svcSelector) == 0 {
		return nil
	}

	var findings []types.DiagnosticFinding

	for _, ap := range apList.Items {
		action, _, _ := unstructured.NestedString(ap.Object, "spec", "action")
		if action != "DENY" {
			continue
		}

		apSelector, _, _ := unstructured.NestedMap(ap.Object, "spec", "selector", "matchLabels")

		// Namespace-wide DENY (no selector) affects all services
		if len(apSelector) == 0 {
			findings = append(findings, types.DiagnosticFinding{
				Severity: types.SeverityWarning,
				Category: types.CategoryRouting,
				Resource: &types.ResourceRef{
					Kind:       "AuthorizationPolicy",
					Namespace:  ap.GetNamespace(),
					Name:       ap.GetName(),
					APIVersion: "security.istio.io",
				},
				Summary: fmt.Sprintf("Namespace-wide DENY AuthorizationPolicy %s/%s may block traffic to %s", ap.GetNamespace(), ap.GetName(), svcName),
				Detail:  "This DENY policy has no workload selector and applies to all services in the namespace. Routed traffic may be denied.",
				Suggestion: "Verify the DENY policy rules don't overlap with traffic routed to this service",
			})
			continue
		}

		// Check if AP selector labels are a subset of the service's selector
		if selectorOverlaps(svcSelector, apSelector) {
			findings = append(findings, types.DiagnosticFinding{
				Severity: types.SeverityWarning,
				Category: types.CategoryRouting,
				Resource: &types.ResourceRef{
					Kind:       "AuthorizationPolicy",
					Namespace:  ap.GetNamespace(),
					Name:       ap.GetName(),
					APIVersion: "security.istio.io",
				},
				Summary: fmt.Sprintf("DENY AuthorizationPolicy %s/%s targets workloads that overlap with service %s", ap.GetNamespace(), ap.GetName(), svcName),
				Detail:  "The AuthorizationPolicy workload selector matches pods selected by this service. Routed traffic may be denied by this policy.",
				Suggestion: "Review the DENY rules to ensure they don't block expected traffic to this service",
			})
		}
	}

	return findings
}

// selectorOverlaps returns true if all labels in apSelector match labels in svcSelector.
func selectorOverlaps(svcSelector map[string]string, apSelector map[string]interface{}) bool {
	for k, v := range apSelector {
		vs, ok := v.(string)
		if !ok {
			continue
		}
		if svcSelector[k] != vs {
			return false
		}
	}
	return true
}

// isUnconstrainedRule returns true if a rule has no from, to, or when conditions.
func isUnconstrainedRule(rule map[string]interface{}) bool {
	from, hasFrom := rule["from"]
	to, hasTo := rule["to"]
	when, hasWhen := rule["when"]

	fromEmpty := !hasFrom || from == nil
	toEmpty := !hasTo || to == nil
	whenEmpty := !hasWhen || when == nil

	if !fromEmpty {
		if fromSlice, ok := from.([]interface{}); ok && len(fromSlice) == 0 {
			fromEmpty = true
		}
	}
	if !toEmpty {
		if toSlice, ok := to.([]interface{}); ok && len(toSlice) == 0 {
			toEmpty = true
		}
	}
	if !whenEmpty {
		if whenSlice, ok := when.([]interface{}); ok && len(whenSlice) == 0 {
			whenEmpty = true
		}
	}

	return fromEmpty && toEmpty && whenEmpty
}
