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
