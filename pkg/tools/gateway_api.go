package tools

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/isitobservable/k8s-networking-mcp/pkg/types"
)

var (
	gatewaysV1GVR    = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gateways"}
	gatewaysV1B1GVR  = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1beta1", Resource: "gateways"}
	httpRoutesV1GVR  = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes"}
	httpRoutesV1B1GVR = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1beta1", Resource: "httproutes"}
)

// listWithFallback tries listing with the v1 GVR first, falling back to v1beta1.
func listWithFallback(ctx context.Context, client dynamic.Interface, v1, v1beta1 schema.GroupVersionResource, ns string) (*unstructured.UnstructuredList, error) {
	var ri dynamic.ResourceInterface
	if ns == "" {
		ri = client.Resource(v1)
	} else {
		ri = client.Resource(v1).Namespace(ns)
	}
	list, err := ri.List(ctx, metav1.ListOptions{})
	if err == nil {
		return list, nil
	}
	// Fallback to v1beta1
	if ns == "" {
		ri = client.Resource(v1beta1)
	} else {
		ri = client.Resource(v1beta1).Namespace(ns)
	}
	return ri.List(ctx, metav1.ListOptions{})
}

// getWithFallback tries getting with the v1 GVR first, falling back to v1beta1.
func getWithFallback(ctx context.Context, client dynamic.Interface, v1, v1beta1 schema.GroupVersionResource, ns, name string) (*unstructured.Unstructured, error) {
	obj, err := client.Resource(v1).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return obj, nil
	}
	return client.Resource(v1beta1).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
}

// --- list_gateways ---

type ListGatewaysTool struct{ BaseTool }

func (t *ListGatewaysTool) Name() string        { return "list_gateways" }
func (t *ListGatewaysTool) Description() string  { return "List Gateway API gateways with listeners, status conditions, and attached route count" }
func (t *ListGatewaysTool) InputSchema() map[string]interface{} {
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

func (t *ListGatewaysTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	ns := getStringArg(args, "namespace", "")

	list, err := listWithFallback(ctx, t.Clients.Dynamic, gatewaysV1GVR, gatewaysV1B1GVR, ns)
	if err != nil {
		return nil, &types.MCPError{
			Code:    types.ErrCodeCRDNotAvailable,
			Tool:    t.Name(),
			Message: "failed to list gateways",
			Detail:  fmt.Sprintf("tried gateway.networking.k8s.io/v1 and v1beta1: %v", err),
		}
	}

	findings := make([]types.DiagnosticFinding, 0, len(list.Items))
	for _, item := range list.Items {
		listeners, _, _ := unstructured.NestedSlice(item.Object, "spec", "listeners")
		conditions, _, _ := unstructured.NestedSlice(item.Object, "status", "conditions")
		gatewayClass := getNestedString(item.Object, "spec", "gatewayClassName")

		// Count attached routes from listener status (float64 from JSON)
		listenerStatuses, _, _ := unstructured.NestedSlice(item.Object, "status", "listeners")
		attachedRoutes := 0
		for _, ls := range listenerStatuses {
			if lsm, ok := ls.(map[string]interface{}); ok {
				if count, ok := lsm["attachedRoutes"].(float64); ok {
					attachedRoutes += int(count)
				}
			}
		}

		// Build listener summary strings
		listenerParts := make([]string, 0, len(listeners))
		for _, l := range listeners {
			if lm, ok := l.(map[string]interface{}); ok {
				port := fmt.Sprintf("%v", lm["port"])
				protocol, _ := lm["protocol"].(string)
				name, _ := lm["name"].(string)
				hostname, _ := lm["hostname"].(string)
				part := fmt.Sprintf("%s:%s/%s", name, port, protocol)
				if hostname != "" {
					part += fmt.Sprintf(" hostname=%s", hostname)
				}
				listenerParts = append(listenerParts, part)
			}
		}

		// Build condition summary for detail
		condDetail := formatConditions(conditions)

		summary := fmt.Sprintf("%s/%s class=%s listeners=[%s] attachedRoutes=%d",
			item.GetNamespace(), item.GetName(), gatewayClass,
			strings.Join(listenerParts, ", "), attachedRoutes)

		finding := types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryRouting,
			Resource: &types.ResourceRef{
				Kind:       "Gateway",
				Namespace:  item.GetNamespace(),
				Name:       item.GetName(),
				APIVersion: "gateway.networking.k8s.io",
			},
			Summary: summary,
			Detail:  condDetail,
		}

		// Elevate severity if any condition is not healthy
		if hasUnhealthyCondition(conditions) {
			finding.Severity = types.SeverityWarning
		}

		findings = append(findings, finding)
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, "gateway-api"), nil
}

// --- get_gateway ---

type GetGatewayTool struct{ BaseTool }

func (t *GetGatewayTool) Name() string        { return "get_gateway" }
func (t *GetGatewayTool) Description() string  { return "Get full Gateway detail: listeners, addresses, conditions, and attached routes" }
func (t *GetGatewayTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Gateway name",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace",
			},
		},
		"required": []string{"name", "namespace"},
	}
}

func (t *GetGatewayTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	name := getStringArg(args, "name", "")
	ns := getStringArg(args, "namespace", "default")

	gw, err := getWithFallback(ctx, t.Clients.Dynamic, gatewaysV1GVR, gatewaysV1B1GVR, ns, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway %s/%s: %w", ns, name, err)
	}

	gatewayClass := getNestedString(gw.Object, "spec", "gatewayClassName")
	listeners, _, _ := unstructured.NestedSlice(gw.Object, "spec", "listeners")
	conditions, _, _ := unstructured.NestedSlice(gw.Object, "status", "conditions")
	addresses, _, _ := unstructured.NestedSlice(gw.Object, "status", "addresses")
	listenerStatuses, _, _ := unstructured.NestedSlice(gw.Object, "status", "listeners")

	gwRef := &types.ResourceRef{
		Kind:       "Gateway",
		Namespace:  ns,
		Name:       name,
		APIVersion: "gateway.networking.k8s.io",
	}

	var findings []types.DiagnosticFinding

	// Address summary
	addrParts := make([]string, 0, len(addresses))
	for _, a := range addresses {
		if am, ok := a.(map[string]interface{}); ok {
			addrType, _ := am["type"].(string)
			addrValue, _ := am["value"].(string)
			addrParts = append(addrParts, fmt.Sprintf("%s=%s", addrType, addrValue))
		}
	}

	// Main gateway finding
	mainSummary := fmt.Sprintf("Gateway %s/%s class=%s addresses=[%s]",
		ns, name, gatewayClass, strings.Join(addrParts, ", "))
	findings = append(findings, types.DiagnosticFinding{
		Severity: types.SeverityInfo,
		Category: types.CategoryRouting,
		Resource: gwRef,
		Summary:  mainSummary,
		Detail:   formatConditions(conditions),
	})

	// Per-listener findings with attached route count from status
	listenerStatusMap := make(map[string]map[string]interface{})
	for _, ls := range listenerStatuses {
		if lsm, ok := ls.(map[string]interface{}); ok {
			if lsName, ok := lsm["name"].(string); ok {
				listenerStatusMap[lsName] = lsm
			}
		}
	}

	for _, l := range listeners {
		lm, ok := l.(map[string]interface{})
		if !ok {
			continue
		}
		lName, _ := lm["name"].(string)
		port := fmt.Sprintf("%v", lm["port"])
		protocol, _ := lm["protocol"].(string)
		hostname, _ := lm["hostname"].(string)

		attached := 0
		var listenerConditions []interface{}
		if lsm, ok := listenerStatusMap[lName]; ok {
			if count, ok := lsm["attachedRoutes"].(float64); ok {
				attached = int(count)
			}
			if conds, ok := lsm["conditions"].([]interface{}); ok {
				listenerConditions = conds
			}
		}

		lSummary := fmt.Sprintf("Listener %s port=%s protocol=%s attachedRoutes=%d", lName, port, protocol, attached)
		if hostname != "" {
			lSummary += fmt.Sprintf(" hostname=%s", hostname)
		}

		severity := types.SeverityInfo
		if hasUnhealthyCondition(listenerConditions) {
			severity = types.SeverityWarning
		}

		findings = append(findings, types.DiagnosticFinding{
			Severity: severity,
			Category: types.CategoryRouting,
			Resource: gwRef,
			Summary:  lSummary,
			Detail:   formatConditions(listenerConditions),
		})
	}

	// Condition warnings
	for _, c := range conditions {
		cm, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		status, _ := cm["status"].(string)
		condType, _ := cm["type"].(string)
		reason, _ := cm["reason"].(string)
		message, _ := cm["message"].(string)

		if status == "False" && (condType == "Accepted" || condType == "Programmed") {
			findings = append(findings, types.DiagnosticFinding{
				Severity:   types.SeverityWarning,
				Category:   types.CategoryRouting,
				Resource:   gwRef,
				Summary:    fmt.Sprintf("Gateway condition %s=%s reason=%s", condType, status, reason),
				Detail:     message,
				Suggestion: fmt.Sprintf("Check gateway class %q and listener configuration", gatewayClass),
			})
		}
	}

	// Find attached HTTPRoutes
	routeList, _ := listWithFallback(ctx, t.Clients.Dynamic, httpRoutesV1GVR, httpRoutesV1B1GVR, ns)
	if routeList != nil {
		for _, route := range routeList.Items {
			parentRefs, _, _ := unstructured.NestedSlice(route.Object, "spec", "parentRefs")
			for _, pr := range parentRefs {
				if prm, ok := pr.(map[string]interface{}); ok {
					refName, _ := prm["name"].(string)
					if refName == name {
						findings = append(findings, types.DiagnosticFinding{
							Severity: types.SeverityInfo,
							Category: types.CategoryRouting,
							Resource: &types.ResourceRef{
								Kind:       "HTTPRoute",
								Namespace:  route.GetNamespace(),
								Name:       route.GetName(),
								APIVersion: "gateway.networking.k8s.io",
							},
							Summary: fmt.Sprintf("HTTPRoute %s/%s attached to gateway %s", route.GetNamespace(), route.GetName(), name),
						})
						break
					}
				}
			}
		}
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, "gateway-api"), nil
}

// --- list_httproutes ---

type ListHTTPRoutesTool struct{ BaseTool }

func (t *ListHTTPRoutesTool) Name() string        { return "list_httproutes" }
func (t *ListHTTPRoutesTool) Description() string  { return "List HTTPRoutes with parent refs, backend refs, and rule count" }
func (t *ListHTTPRoutesTool) InputSchema() map[string]interface{} {
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

func (t *ListHTTPRoutesTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	ns := getStringArg(args, "namespace", "")

	list, err := listWithFallback(ctx, t.Clients.Dynamic, httpRoutesV1GVR, httpRoutesV1B1GVR, ns)
	if err != nil {
		return nil, &types.MCPError{
			Code:    types.ErrCodeCRDNotAvailable,
			Tool:    t.Name(),
			Message: "failed to list httproutes",
			Detail:  fmt.Sprintf("tried gateway.networking.k8s.io/v1 and v1beta1: %v", err),
		}
	}

	findings := make([]types.DiagnosticFinding, 0, len(list.Items))
	for _, item := range list.Items {
		parentRefs, _, _ := unstructured.NestedSlice(item.Object, "spec", "parentRefs")
		rules, _, _ := unstructured.NestedSlice(item.Object, "spec", "rules")

		parentRefParts := make([]string, 0, len(parentRefs))
		for _, pr := range parentRefs {
			if prm, ok := pr.(map[string]interface{}); ok {
				refName, _ := prm["name"].(string)
				refNs, _ := prm["namespace"].(string)
				section, _ := prm["sectionName"].(string)
				part := refName
				if refNs != "" {
					part = refNs + "/" + part
				}
				if section != "" {
					part += "/" + section
				}
				parentRefParts = append(parentRefParts, part)
			}
		}

		// Extract backend refs from rules
		backendRefParts := make([]string, 0)
		for _, r := range rules {
			if rm, ok := r.(map[string]interface{}); ok {
				if brs, ok := rm["backendRefs"].([]interface{}); ok {
					for _, br := range brs {
						if brm, ok := br.(map[string]interface{}); ok {
							brName, _ := brm["name"].(string)
							brPort := fmt.Sprintf("%v", brm["port"])
							backendRefParts = append(backendRefParts, fmt.Sprintf("%s:%s", brName, brPort))
						}
					}
				}
			}
		}

		summary := fmt.Sprintf("%s/%s parents=[%s] rules=%d backends=[%s]",
			item.GetNamespace(), item.GetName(),
			strings.Join(parentRefParts, ", "),
			len(rules),
			strings.Join(backendRefParts, ", "))

		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryRouting,
			Resource: &types.ResourceRef{
				Kind:       "HTTPRoute",
				Namespace:  item.GetNamespace(),
				Name:       item.GetName(),
				APIVersion: "gateway.networking.k8s.io",
			},
			Summary: summary,
		})
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, "gateway-api"), nil
}

// --- get_httproute ---

type GetHTTPRouteTool struct{ BaseTool }

func (t *GetHTTPRouteTool) Name() string        { return "get_httproute" }
func (t *GetHTTPRouteTool) Description() string  { return "Get full HTTPRoute: rules, matches, filters, backend refs with health" }
func (t *GetHTTPRouteTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "HTTPRoute name",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace",
			},
		},
		"required": []string{"name", "namespace"},
	}
}

func (t *GetHTTPRouteTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	name := getStringArg(args, "name", "")
	ns := getStringArg(args, "namespace", "default")

	route, err := getWithFallback(ctx, t.Clients.Dynamic, httpRoutesV1GVR, httpRoutesV1B1GVR, ns, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get httproute %s/%s: %w", ns, name, err)
	}

	routeRef := &types.ResourceRef{
		Kind:       "HTTPRoute",
		Namespace:  ns,
		Name:       name,
		APIVersion: "gateway.networking.k8s.io",
	}

	parentRefs, _, _ := unstructured.NestedSlice(route.Object, "spec", "parentRefs")
	rules, _, _ := unstructured.NestedSlice(route.Object, "spec", "rules")

	var findings []types.DiagnosticFinding

	// Parent refs summary
	parentRefParts := make([]string, 0, len(parentRefs))
	for _, pr := range parentRefs {
		if prm, ok := pr.(map[string]interface{}); ok {
			refName, _ := prm["name"].(string)
			refNs, _ := prm["namespace"].(string)
			section, _ := prm["sectionName"].(string)
			part := refName
			if refNs != "" {
				part = refNs + "/" + part
			}
			if section != "" {
				part += "/" + section
			}
			parentRefParts = append(parentRefParts, part)
		}
	}

	// Main route finding
	findings = append(findings, types.DiagnosticFinding{
		Severity: types.SeverityInfo,
		Category: types.CategoryRouting,
		Resource: routeRef,
		Summary:  fmt.Sprintf("HTTPRoute %s/%s parents=[%s] rules=%d", ns, name, strings.Join(parentRefParts, ", "), len(rules)),
	})

	// Per-rule findings with matches, filters, and backend refs
	for i, r := range rules {
		rm, ok := r.(map[string]interface{})
		if !ok {
			continue
		}

		// Extract matches
		matchParts := make([]string, 0)
		if matches, ok := rm["matches"].([]interface{}); ok {
			for _, m := range matches {
				if mm, ok := m.(map[string]interface{}); ok {
					if pathMatch, ok := mm["path"].(map[string]interface{}); ok {
						matchType, _ := pathMatch["type"].(string)
						matchValue, _ := pathMatch["value"].(string)
						matchParts = append(matchParts, fmt.Sprintf("path(%s=%s)", matchType, matchValue))
					}
					if headers, ok := mm["headers"].([]interface{}); ok {
						for _, h := range headers {
							if hm, ok := h.(map[string]interface{}); ok {
								hName, _ := hm["name"].(string)
								matchParts = append(matchParts, fmt.Sprintf("header(%s)", hName))
							}
						}
					}
					if method, ok := mm["method"].(string); ok {
						matchParts = append(matchParts, fmt.Sprintf("method(%s)", method))
					}
				}
			}
		}

		// Extract filters
		filterParts := make([]string, 0)
		if filters, ok := rm["filters"].([]interface{}); ok {
			for _, f := range filters {
				if fm, ok := f.(map[string]interface{}); ok {
					fType, _ := fm["type"].(string)
					filterParts = append(filterParts, fType)
				}
			}
		}

		// Extract backend refs
		backendParts := make([]string, 0)
		if brs, ok := rm["backendRefs"].([]interface{}); ok {
			for _, br := range brs {
				if brm, ok := br.(map[string]interface{}); ok {
					brName, _ := brm["name"].(string)
					brPort := fmt.Sprintf("%v", brm["port"])
					weight := ""
					if w, ok := brm["weight"].(float64); ok {
						weight = fmt.Sprintf(" weight=%d", int(w))
					}
					backendParts = append(backendParts, fmt.Sprintf("%s:%s%s", brName, brPort, weight))
				}
			}
		}

		ruleSummary := fmt.Sprintf("Rule %d: matches=[%s] filters=[%s] backends=[%s]",
			i, strings.Join(matchParts, ", "), strings.Join(filterParts, ", "), strings.Join(backendParts, ", "))

		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryRouting,
			Resource: routeRef,
			Summary:  ruleSummary,
		})
	}

	// Check backend service health
	for _, r := range rules {
		rm, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		brs, ok := rm["backendRefs"].([]interface{})
		if !ok {
			continue
		}
		for _, br := range brs {
			brm, ok := br.(map[string]interface{})
			if !ok {
				continue
			}
			refName, _ := brm["name"].(string)
			refNs := ns
			if rns, ok := brm["namespace"].(string); ok && rns != "" {
				refNs = rns
			}

			_, svcErr := t.Clients.Dynamic.Resource(servicesGVR).Namespace(refNs).Get(ctx, refName, metav1.GetOptions{})
			if svcErr != nil {
				findings = append(findings, types.DiagnosticFinding{
					Severity:   types.SeverityWarning,
					Category:   types.CategoryRouting,
					Resource:   routeRef,
					Summary:    fmt.Sprintf("Backend service %s/%s not found", refNs, refName),
					Detail:     svcErr.Error(),
					Suggestion: "Verify the backend service name and namespace are correct",
				})
				continue
			}

			ep, epErr := t.Clients.Dynamic.Resource(endpointsGVR).Namespace(refNs).Get(ctx, refName, metav1.GetOptions{})
			if epErr != nil {
				continue
			}
			subsets, _, _ := unstructured.NestedSlice(ep.Object, "subsets")
			readyCount := 0
			for _, s := range subsets {
				if sm, ok := s.(map[string]interface{}); ok {
					if addrs, ok := sm["addresses"].([]interface{}); ok {
						readyCount += len(addrs)
					}
				}
			}
			if readyCount == 0 {
				findings = append(findings, types.DiagnosticFinding{
					Severity:   types.SeverityWarning,
					Category:   types.CategoryRouting,
					Resource:   routeRef,
					Summary:    fmt.Sprintf("Backend service %s/%s has 0 ready endpoints", refNs, refName),
					Suggestion: "Check that pods backing this service are running and passing readiness probes",
				})
			} else {
				findings = append(findings, types.DiagnosticFinding{
					Severity: types.SeverityOK,
					Category: types.CategoryRouting,
					Resource: routeRef,
					Summary:  fmt.Sprintf("Backend service %s/%s has %d ready endpoints", refNs, refName, readyCount),
				})
			}
		}
	}

	// Route status conditions (from parent statuses)
	parentStatuses, _, _ := unstructured.NestedSlice(route.Object, "status", "parents")
	for _, ps := range parentStatuses {
		psm, ok := ps.(map[string]interface{})
		if !ok {
			continue
		}
		parentName, _ := psm["parentRef"].(map[string]interface{})
		pName := ""
		if parentName != nil {
			pName, _ = parentName["name"].(string)
		}
		if conds, ok := psm["conditions"].([]interface{}); ok {
			for _, c := range conds {
				cm, ok := c.(map[string]interface{})
				if !ok {
					continue
				}
				status, _ := cm["status"].(string)
				condType, _ := cm["type"].(string)
				reason, _ := cm["reason"].(string)
				message, _ := cm["message"].(string)

				if status == "False" {
					findings = append(findings, types.DiagnosticFinding{
						Severity:   types.SeverityWarning,
						Category:   types.CategoryRouting,
						Resource:   routeRef,
						Summary:    fmt.Sprintf("Route condition %s=%s for parent %s reason=%s", condType, status, pName, reason),
						Detail:     message,
						Suggestion: "Check that the parent gateway and listener accept this route",
					})
				}
			}
		}
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, "gateway-api"), nil
}

// Helper functions

func formatConditions(conditions []interface{}) string {
	parts := make([]string, 0, len(conditions))
	for _, c := range conditions {
		if cm, ok := c.(map[string]interface{}); ok {
			condType, _ := cm["type"].(string)
			status, _ := cm["status"].(string)
			reason, _ := cm["reason"].(string)
			message, _ := cm["message"].(string)
			part := fmt.Sprintf("%s=%s reason=%s", condType, status, reason)
			if message != "" {
				part += fmt.Sprintf(" message=%q", message)
			}
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, "\n")
}

func hasUnhealthyCondition(conditions []interface{}) bool {
	for _, c := range conditions {
		if cm, ok := c.(map[string]interface{}); ok {
			status, _ := cm["status"].(string)
			condType, _ := cm["type"].(string)
			if status == "False" && (condType == "Accepted" || condType == "Programmed" || condType == "Ready" || condType == "ResolvedRefs") {
				return true
			}
		}
	}
	return false
}

func getNestedString(obj map[string]interface{}, fields ...string) string {
	val, _, _ := unstructured.NestedString(obj, fields...)
	return val
}
