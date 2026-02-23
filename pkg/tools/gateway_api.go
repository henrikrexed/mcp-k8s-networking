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
	gatewaysV1GVR     = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gateways"}
	gatewaysV1B1GVR   = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1beta1", Resource: "gateways"}
	httpRoutesV1GVR   = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes"}
	httpRoutesV1B1GVR = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1beta1", Resource: "httproutes"}
	grpcRoutesV1GVR   = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "grpcroutes"}
	grpcRoutesV1B1GVR = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1beta1", Resource: "grpcroutes"}
	refGrantsV1B1GVR  = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1beta1", Resource: "referencegrants"}
	refGrantsV1GVR    = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "referencegrants"}
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

// --- list_grpcroutes ---

type ListGRPCRoutesTool struct{ BaseTool }

func (t *ListGRPCRoutesTool) Name() string        { return "list_grpcroutes" }
func (t *ListGRPCRoutesTool) Description() string  { return "List GRPCRoutes with parent refs, backend refs, and rule counts" }
func (t *ListGRPCRoutesTool) InputSchema() map[string]interface{} {
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

func (t *ListGRPCRoutesTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	ns := getStringArg(args, "namespace", "")

	list, err := listWithFallback(ctx, t.Clients.Dynamic, grpcRoutesV1GVR, grpcRoutesV1B1GVR, ns)
	if err != nil {
		return nil, &types.MCPError{
			Code:    types.ErrCodeCRDNotAvailable,
			Tool:    t.Name(),
			Message: "failed to list grpcroutes",
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
				Kind:       "GRPCRoute",
				Namespace:  item.GetNamespace(),
				Name:       item.GetName(),
				APIVersion: "gateway.networking.k8s.io",
			},
			Summary: summary,
		})
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, "gateway-api"), nil
}

// --- get_grpcroute ---

type GetGRPCRouteTool struct{ BaseTool }

func (t *GetGRPCRouteTool) Name() string        { return "get_grpcroute" }
func (t *GetGRPCRouteTool) Description() string  { return "Get full GRPCRoute: method matching rules, backend refs with health, and status conditions" }
func (t *GetGRPCRouteTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "GRPCRoute name",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace",
			},
		},
		"required": []string{"name", "namespace"},
	}
}

func (t *GetGRPCRouteTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	name := getStringArg(args, "name", "")
	ns := getStringArg(args, "namespace", "default")

	route, err := getWithFallback(ctx, t.Clients.Dynamic, grpcRoutesV1GVR, grpcRoutesV1B1GVR, ns, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get grpcroute %s/%s: %w", ns, name, err)
	}

	routeRef := &types.ResourceRef{
		Kind:       "GRPCRoute",
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
		Summary:  fmt.Sprintf("GRPCRoute %s/%s parents=[%s] rules=%d", ns, name, strings.Join(parentRefParts, ", "), len(rules)),
	})

	// Per-rule findings with method matches and backend refs
	for i, r := range rules {
		rm, ok := r.(map[string]interface{})
		if !ok {
			continue
		}

		// Extract gRPC method matches
		matchParts := make([]string, 0)
		if matches, ok := rm["matches"].([]interface{}); ok {
			for _, m := range matches {
				if mm, ok := m.(map[string]interface{}); ok {
					if method, ok := mm["method"].(map[string]interface{}); ok {
						svc, _ := method["service"].(string)
						meth, _ := method["method"].(string)
						matchType, _ := method["type"].(string)
						if matchType == "" {
							matchType = "Exact"
						}
						matchParts = append(matchParts, fmt.Sprintf("%s(%s/%s)", matchType, svc, meth))
					}
					if headers, ok := mm["headers"].([]interface{}); ok {
						for _, h := range headers {
							if hm, ok := h.(map[string]interface{}); ok {
								hName, _ := hm["name"].(string)
								matchParts = append(matchParts, fmt.Sprintf("header(%s)", hName))
							}
						}
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
						Suggestion: "Check that the parent gateway and listener accept this GRPCRoute",
					})
				}
			}
		}
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, "gateway-api"), nil
}

// --- list_referencegrants ---

type ListReferenceGrantsTool struct{ BaseTool }

func (t *ListReferenceGrantsTool) Name() string        { return "list_referencegrants" }
func (t *ListReferenceGrantsTool) Description() string  { return "List ReferenceGrants with from/to resource specifications for cross-namespace reference validation" }
func (t *ListReferenceGrantsTool) InputSchema() map[string]interface{} {
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

func (t *ListReferenceGrantsTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	ns := getStringArg(args, "namespace", "")

	list, err := listWithFallback(ctx, t.Clients.Dynamic, refGrantsV1GVR, refGrantsV1B1GVR, ns)
	if err != nil {
		return nil, &types.MCPError{
			Code:    types.ErrCodeCRDNotAvailable,
			Tool:    t.Name(),
			Message: "failed to list referencegrants",
			Detail:  fmt.Sprintf("tried gateway.networking.k8s.io/v1 and v1beta1: %v", err),
		}
	}

	findings := make([]types.DiagnosticFinding, 0, len(list.Items))
	for _, item := range list.Items {
		fromRefs, _, _ := unstructured.NestedSlice(item.Object, "spec", "from")
		toRefs, _, _ := unstructured.NestedSlice(item.Object, "spec", "to")

		fromParts := make([]string, 0, len(fromRefs))
		for _, f := range fromRefs {
			if fm, ok := f.(map[string]interface{}); ok {
				group, _ := fm["group"].(string)
				kind, _ := fm["kind"].(string)
				namespace, _ := fm["namespace"].(string)
				fromParts = append(fromParts, fmt.Sprintf("%s/%s from ns=%s", group, kind, namespace))
			}
		}

		toParts := make([]string, 0, len(toRefs))
		for _, t := range toRefs {
			if tm, ok := t.(map[string]interface{}); ok {
				group, _ := tm["group"].(string)
				kind, _ := tm["kind"].(string)
				name, _ := tm["name"].(string)
				part := fmt.Sprintf("%s/%s", group, kind)
				if name != "" {
					part += fmt.Sprintf(" name=%s", name)
				}
				toParts = append(toParts, part)
			}
		}

		summary := fmt.Sprintf("%s/%s from=[%s] to=[%s]",
			item.GetNamespace(), item.GetName(),
			strings.Join(fromParts, "; "),
			strings.Join(toParts, "; "))

		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryPolicy,
			Resource: &types.ResourceRef{
				Kind:       "ReferenceGrant",
				Namespace:  item.GetNamespace(),
				Name:       item.GetName(),
				APIVersion: "gateway.networking.k8s.io",
			},
			Summary: summary,
		})
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, "gateway-api"), nil
}

// --- get_referencegrant ---

type GetReferenceGrantTool struct{ BaseTool }

func (t *GetReferenceGrantTool) Name() string        { return "get_referencegrant" }
func (t *GetReferenceGrantTool) Description() string  { return "Get full ReferenceGrant spec: allowed from-namespaces, from-kinds, to-kinds, to-names, and cross-namespace validation" }
func (t *GetReferenceGrantTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "ReferenceGrant name",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace (the namespace that grants access)",
			},
		},
		"required": []string{"name", "namespace"},
	}
}

func (t *GetReferenceGrantTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	name := getStringArg(args, "name", "")
	ns := getStringArg(args, "namespace", "default")

	grant, err := getWithFallback(ctx, t.Clients.Dynamic, refGrantsV1GVR, refGrantsV1B1GVR, ns, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get referencegrant %s/%s: %w", ns, name, err)
	}

	grantRef := &types.ResourceRef{
		Kind:       "ReferenceGrant",
		Namespace:  ns,
		Name:       name,
		APIVersion: "gateway.networking.k8s.io",
	}

	fromRefs, _, _ := unstructured.NestedSlice(grant.Object, "spec", "from")
	toRefs, _, _ := unstructured.NestedSlice(grant.Object, "spec", "to")

	var findings []types.DiagnosticFinding

	// Build detailed from/to descriptions
	fromParts := make([]string, 0, len(fromRefs))
	for _, f := range fromRefs {
		if fm, ok := f.(map[string]interface{}); ok {
			group, _ := fm["group"].(string)
			kind, _ := fm["kind"].(string)
			namespace, _ := fm["namespace"].(string)
			if group == "" {
				group = "core"
			}
			fromParts = append(fromParts, fmt.Sprintf("%s/%s from namespace %s", group, kind, namespace))
		}
	}

	toParts := make([]string, 0, len(toRefs))
	for _, tr := range toRefs {
		if tm, ok := tr.(map[string]interface{}); ok {
			group, _ := tm["group"].(string)
			kind, _ := tm["kind"].(string)
			toName, _ := tm["name"].(string)
			if group == "" {
				group = "core"
			}
			part := fmt.Sprintf("%s/%s", group, kind)
			if toName != "" {
				part += fmt.Sprintf(" name=%s", toName)
			}
			toParts = append(toParts, part)
		}
	}

	// Main grant finding
	findings = append(findings, types.DiagnosticFinding{
		Severity: types.SeverityInfo,
		Category: types.CategoryPolicy,
		Resource: grantRef,
		Summary:  fmt.Sprintf("ReferenceGrant %s/%s allows %d source(s) to reference %d target(s) in namespace %s", ns, name, len(fromRefs), len(toRefs), ns),
		Detail:   fmt.Sprintf("from:\n  %s\nto:\n  %s", strings.Join(fromParts, "\n  "), strings.Join(toParts, "\n  ")),
	})

	// Check for HTTPRoutes that reference backends in this namespace from allowed source namespaces
	// and identify any cross-namespace references that are NOT covered by this grant
	allowedFromNamespaces := make(map[string]bool)
	for _, f := range fromRefs {
		if fm, ok := f.(map[string]interface{}); ok {
			fromNs, _ := fm["namespace"].(string)
			fromKind, _ := fm["kind"].(string)
			if fromKind == "HTTPRoute" || fromKind == "GRPCRoute" {
				allowedFromNamespaces[fromNs] = true
			}
		}
	}

	// Find HTTPRoutes that reference backends in this grant's namespace
	routeList, _ := listWithFallback(ctx, t.Clients.Dynamic, httpRoutesV1GVR, httpRoutesV1B1GVR, "")
	if routeList != nil {
		for _, route := range routeList.Items {
			routeNs := route.GetNamespace()
			if routeNs == ns {
				continue // same namespace, no grant needed
			}
			rules, _, _ := unstructured.NestedSlice(route.Object, "spec", "rules")
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
					refNs, _ := brm["namespace"].(string)
					refName, _ := brm["name"].(string)
					if refNs != ns {
						continue // backend not in this grant's namespace
					}
					if allowedFromNamespaces[routeNs] {
						findings = append(findings, types.DiagnosticFinding{
							Severity: types.SeverityOK,
							Category: types.CategoryPolicy,
							Resource: &types.ResourceRef{
								Kind:       "HTTPRoute",
								Namespace:  routeNs,
								Name:       route.GetName(),
								APIVersion: "gateway.networking.k8s.io",
							},
							Summary: fmt.Sprintf("HTTPRoute %s/%s cross-namespace ref to %s/%s is allowed by ReferenceGrant %s/%s", routeNs, route.GetName(), ns, refName, ns, name),
						})
					} else {
						findings = append(findings, types.DiagnosticFinding{
							Severity:   types.SeverityWarning,
							Category:   types.CategoryPolicy,
							Resource: &types.ResourceRef{
								Kind:       "HTTPRoute",
								Namespace:  routeNs,
								Name:       route.GetName(),
								APIVersion: "gateway.networking.k8s.io",
							},
							Summary:    fmt.Sprintf("HTTPRoute %s/%s references backend %s/%s but namespace %s is not allowed by ReferenceGrant %s/%s", routeNs, route.GetName(), ns, refName, routeNs, ns, name),
							Suggestion: fmt.Sprintf("Add namespace %s to the ReferenceGrant 'from' list, or create a new ReferenceGrant in namespace %s", routeNs, ns),
						})
					}
				}
			}
		}
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, "gateway-api"), nil
}

// --- scan_gateway_misconfigs ---

type ScanGatewayMisconfigsTool struct{ BaseTool }

func (t *ScanGatewayMisconfigsTool) Name() string { return "scan_gateway_misconfigs" }
func (t *ScanGatewayMisconfigsTool) Description() string {
	return "Scan for Gateway API misconfigurations: missing backends, orphaned routes, missing ReferenceGrants, listener conflicts"
}
func (t *ScanGatewayMisconfigsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace (empty for cluster-wide scan)",
			},
		},
	}
}

func (t *ScanGatewayMisconfigsTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	ns := getStringArg(args, "namespace", "")

	// Fetch all resources
	gwList, _ := listWithFallback(ctx, t.Clients.Dynamic, gatewaysV1GVR, gatewaysV1B1GVR, ns)
	httpRouteList, _ := listWithFallback(ctx, t.Clients.Dynamic, httpRoutesV1GVR, httpRoutesV1B1GVR, ns)
	grpcRouteList, _ := listWithFallback(ctx, t.Clients.Dynamic, grpcRoutesV1GVR, grpcRoutesV1B1GVR, ns)
	refGrantList, _ := listWithFallback(ctx, t.Clients.Dynamic, refGrantsV1GVR, refGrantsV1B1GVR, ns)

	// Build lookup maps
	// gatewaysByKey: "namespace/name" -> gateway listeners
	type listenerInfo struct {
		name     string
		port     float64
		protocol string
	}
	type gatewayInfo struct {
		listeners []listenerInfo
	}
	gatewaysByKey := make(map[string]*gatewayInfo)
	if gwList != nil {
		for _, gw := range gwList.Items {
			key := gw.GetNamespace() + "/" + gw.GetName()
			listeners, _, _ := unstructured.NestedSlice(gw.Object, "spec", "listeners")
			info := &gatewayInfo{}
			for _, l := range listeners {
				if lm, ok := l.(map[string]interface{}); ok {
					lName, _ := lm["name"].(string)
					port, _ := lm["port"].(float64)
					protocol, _ := lm["protocol"].(string)
					info.listeners = append(info.listeners, listenerInfo{name: lName, port: port, protocol: protocol})
				}
			}
			gatewaysByKey[key] = info
		}
	}

	// refGrants: build set of allowed cross-namespace refs
	// key: "fromNamespace/fromKind -> toNamespace" = true
	type refGrantEntry struct {
		fromNs   string
		fromKind string
		toNs     string
	}
	var refGrants []refGrantEntry
	if refGrantList != nil {
		for _, rg := range refGrantList.Items {
			toNs := rg.GetNamespace()
			fromRefs, _, _ := unstructured.NestedSlice(rg.Object, "spec", "from")
			for _, f := range fromRefs {
				if fm, ok := f.(map[string]interface{}); ok {
					fromNs, _ := fm["namespace"].(string)
					fromKind, _ := fm["kind"].(string)
					refGrants = append(refGrants, refGrantEntry{fromNs: fromNs, fromKind: fromKind, toNs: toNs})
				}
			}
		}
	}

	hasRefGrant := func(fromNs, fromKind, toNs string) bool {
		for _, rg := range refGrants {
			if rg.fromNs == fromNs && rg.toNs == toNs && (rg.fromKind == fromKind || rg.fromKind == "") {
				return true
			}
		}
		return false
	}

	var findings []types.DiagnosticFinding

	// --- Check 1: Gateway listener conflicts (port/protocol collisions) ---
	if gwList != nil {
		for _, gw := range gwList.Items {
			gwKey := gw.GetNamespace() + "/" + gw.GetName()
			info := gatewaysByKey[gwKey]
			seen := make(map[string]string) // "port" -> first listener name
			for _, l := range info.listeners {
				portKey := fmt.Sprintf("%v/%s", l.port, l.protocol)
				if prev, exists := seen[portKey]; exists {
					findings = append(findings, types.DiagnosticFinding{
						Severity: types.SeverityWarning,
						Category: types.CategoryRouting,
						Resource: &types.ResourceRef{
							Kind:       "Gateway",
							Namespace:  gw.GetNamespace(),
							Name:       gw.GetName(),
							APIVersion: "gateway.networking.k8s.io",
						},
						Summary:    fmt.Sprintf("Gateway %s has listener conflict: %s and %s both use port %v/%s", gwKey, prev, l.name, l.port, l.protocol),
						Suggestion: "Use different ports or merge listeners with the same port/protocol",
					})
				} else {
					seen[portKey] = l.name
				}
			}
		}
	}

	// Helper to scan routes for misconfigs
	type routeInfo struct {
		kind      string
		name      string
		namespace string
		obj       map[string]interface{}
	}
	var allRoutes []routeInfo
	if httpRouteList != nil {
		for _, r := range httpRouteList.Items {
			allRoutes = append(allRoutes, routeInfo{kind: "HTTPRoute", name: r.GetName(), namespace: r.GetNamespace(), obj: r.Object})
		}
	}
	if grpcRouteList != nil {
		for _, r := range grpcRouteList.Items {
			allRoutes = append(allRoutes, routeInfo{kind: "GRPCRoute", name: r.GetName(), namespace: r.GetNamespace(), obj: r.Object})
		}
	}

	for _, route := range allRoutes {
		routeRef := &types.ResourceRef{
			Kind:       route.kind,
			Namespace:  route.namespace,
			Name:       route.name,
			APIVersion: "gateway.networking.k8s.io",
		}

		// --- Check 2: Routes attached to non-existent or non-matching Gateways ---
		parentRefs, _, _ := unstructured.NestedSlice(route.obj, "spec", "parentRefs")
		for _, pr := range parentRefs {
			prm, ok := pr.(map[string]interface{})
			if !ok {
				continue
			}
			refName, _ := prm["name"].(string)
			refNs, _ := prm["namespace"].(string)
			if refNs == "" {
				refNs = route.namespace
			}
			gwKey := refNs + "/" + refName
			if _, exists := gatewaysByKey[gwKey]; !exists {
				findings = append(findings, types.DiagnosticFinding{
					Severity:   types.SeverityWarning,
					Category:   types.CategoryRouting,
					Resource:   routeRef,
					Summary:    fmt.Sprintf("%s %s/%s references non-existent gateway %s", route.kind, route.namespace, route.name, gwKey),
					Suggestion: fmt.Sprintf("Create gateway %s or update the parentRef to an existing gateway", gwKey),
				})
			} else if sectionName, ok := prm["sectionName"].(string); ok && sectionName != "" {
				gwInfo := gatewaysByKey[gwKey]
				found := false
				for _, l := range gwInfo.listeners {
					if l.name == sectionName {
						found = true
						break
					}
				}
				if !found {
					findings = append(findings, types.DiagnosticFinding{
						Severity:   types.SeverityWarning,
						Category:   types.CategoryRouting,
						Resource:   routeRef,
						Summary:    fmt.Sprintf("%s %s/%s references non-existent listener %q on gateway %s", route.kind, route.namespace, route.name, sectionName, gwKey),
						Suggestion: fmt.Sprintf("Check listener names on gateway %s", gwKey),
					})
				}
			}
		}

		// --- Check 3 & 4: Backend service existence and cross-namespace ReferenceGrants ---
		rules, _, _ := unstructured.NestedSlice(route.obj, "spec", "rules")
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
				refNs, _ := brm["namespace"].(string)
				if refNs == "" {
					refNs = route.namespace
				}

				// Check 3: Non-existent backend services
				_, svcErr := t.Clients.Dynamic.Resource(servicesGVR).Namespace(refNs).Get(ctx, refName, metav1.GetOptions{})
				if svcErr != nil {
					findings = append(findings, types.DiagnosticFinding{
						Severity:   types.SeverityWarning,
						Category:   types.CategoryRouting,
						Resource:   routeRef,
						Summary:    fmt.Sprintf("%s %s/%s references non-existent backend service %s/%s", route.kind, route.namespace, route.name, refNs, refName),
						Suggestion: "Create the backend service or update the backendRef",
					})
				}

				// Check 4: Cross-namespace references missing ReferenceGrants
				if refNs != route.namespace {
					if !hasRefGrant(route.namespace, route.kind, refNs) {
						findings = append(findings, types.DiagnosticFinding{
							Severity:   types.SeverityWarning,
							Category:   types.CategoryPolicy,
							Resource:   routeRef,
							Summary:    fmt.Sprintf("%s %s/%s references backend %s/%s across namespaces but no ReferenceGrant allows this", route.kind, route.namespace, route.name, refNs, refName),
							Suggestion: fmt.Sprintf("Create a ReferenceGrant in namespace %s allowing %s from namespace %s", refNs, route.kind, route.namespace),
						})
					}
				}
			}

			// --- Check 5: Invalid filter configurations ---
			if filters, ok := rm["filters"].([]interface{}); ok {
				for _, f := range filters {
					fm, ok := f.(map[string]interface{})
					if !ok {
						continue
					}
					fType, _ := fm["type"].(string)
					switch fType {
					case "RequestRedirect":
						if _, ok := fm["requestRedirect"]; !ok {
							findings = append(findings, types.DiagnosticFinding{
								Severity:   types.SeverityWarning,
								Category:   types.CategoryRouting,
								Resource:   routeRef,
								Summary:    fmt.Sprintf("%s %s/%s has RequestRedirect filter with missing requestRedirect config", route.kind, route.namespace, route.name),
								Suggestion: "Add requestRedirect configuration to the filter",
							})
						}
					case "URLRewrite":
						if _, ok := fm["urlRewrite"]; !ok {
							findings = append(findings, types.DiagnosticFinding{
								Severity:   types.SeverityWarning,
								Category:   types.CategoryRouting,
								Resource:   routeRef,
								Summary:    fmt.Sprintf("%s %s/%s has URLRewrite filter with missing urlRewrite config", route.kind, route.namespace, route.name),
								Suggestion: "Add urlRewrite configuration to the filter",
							})
						}
					case "RequestHeaderModifier":
						if _, ok := fm["requestHeaderModifier"]; !ok {
							findings = append(findings, types.DiagnosticFinding{
								Severity:   types.SeverityWarning,
								Category:   types.CategoryRouting,
								Resource:   routeRef,
								Summary:    fmt.Sprintf("%s %s/%s has RequestHeaderModifier filter with missing requestHeaderModifier config", route.kind, route.namespace, route.name),
								Suggestion: "Add requestHeaderModifier configuration to the filter",
							})
						}
					case "ResponseHeaderModifier":
						if _, ok := fm["responseHeaderModifier"]; !ok {
							findings = append(findings, types.DiagnosticFinding{
								Severity:   types.SeverityWarning,
								Category:   types.CategoryRouting,
								Resource:   routeRef,
								Summary:    fmt.Sprintf("%s %s/%s has ResponseHeaderModifier filter with missing responseHeaderModifier config", route.kind, route.namespace, route.name),
								Suggestion: "Add responseHeaderModifier configuration to the filter",
							})
						}
					case "RequestMirror":
						if _, ok := fm["requestMirror"]; !ok {
							findings = append(findings, types.DiagnosticFinding{
								Severity:   types.SeverityWarning,
								Category:   types.CategoryRouting,
								Resource:   routeRef,
								Summary:    fmt.Sprintf("%s %s/%s has RequestMirror filter with missing requestMirror config", route.kind, route.namespace, route.name),
								Suggestion: "Add requestMirror configuration to the filter",
							})
						}
					case "ExtensionRef":
						if _, ok := fm["extensionRef"]; !ok {
							findings = append(findings, types.DiagnosticFinding{
								Severity:   types.SeverityWarning,
								Category:   types.CategoryRouting,
								Resource:   routeRef,
								Summary:    fmt.Sprintf("%s %s/%s has ExtensionRef filter with missing extensionRef config", route.kind, route.namespace, route.name),
								Suggestion: "Add extensionRef configuration to the filter",
							})
						}
					}
				}
			}
		}
	}

	if len(findings) == 0 {
		responseNs := ns
		if responseNs == "" {
			responseNs = "all"
		}
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityOK,
			Category: types.CategoryRouting,
			Summary:  fmt.Sprintf("No Gateway API misconfigurations detected in namespace %s", responseNs),
		})
	}

	responseNs := ns
	if responseNs == "" {
		responseNs = "all"
	}
	return NewToolResultResponse(t.Cfg, t.Name(), findings, responseNs, "gateway-api"), nil
}

// --- check_gateway_conformance ---

type CheckGatewayConformanceTool struct{ BaseTool }

func (t *CheckGatewayConformanceTool) Name() string { return "check_gateway_conformance" }
func (t *CheckGatewayConformanceTool) Description() string {
	return "Validate Gateway API resources (Gateway, HTTPRoute, GRPCRoute) against the specification and report non-conformant fields"
}
func (t *CheckGatewayConformanceTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"kind": map[string]interface{}{
				"type":        "string",
				"description": "Resource kind: Gateway, HTTPRoute, or GRPCRoute",
				"enum":        []string{"Gateway", "HTTPRoute", "GRPCRoute"},
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

// Valid enum values per the Gateway API specification.
var (
	validListenerProtocols = map[string]bool{"HTTP": true, "HTTPS": true, "TLS": true, "TCP": true, "UDP": true}
	validTLSModes          = map[string]bool{"Terminate": true, "Passthrough": true}
	validHTTPMethods       = map[string]bool{"GET": true, "HEAD": true, "POST": true, "PUT": true, "DELETE": true, "CONNECT": true, "OPTIONS": true, "TRACE": true, "PATCH": true}
	validPathMatchTypes    = map[string]bool{"Exact": true, "PathPrefix": true, "RegularExpression": true}
	validHeaderMatchTypes  = map[string]bool{"Exact": true, "RegularExpression": true}
	validQueryMatchTypes   = map[string]bool{"Exact": true, "RegularExpression": true}
	validHTTPFilterTypes   = map[string]bool{"RequestHeaderModifier": true, "ResponseHeaderModifier": true, "RequestMirror": true, "RequestRedirect": true, "URLRewrite": true, "ExtensionRef": true}
	validGRPCMethodTypes   = map[string]bool{"Exact": true, "RegularExpression": true}
	validGRPCFilterTypes   = map[string]bool{"RequestHeaderModifier": true, "ResponseHeaderModifier": true, "RequestMirror": true, "ExtensionRef": true}

	// Extended conformance features (not in core profile).
	extendedProtocols      = map[string]bool{"TLS": true, "TCP": true, "UDP": true}
	extendedTLSModes       = map[string]bool{"Passthrough": true}
	extendedPathMatchTypes = map[string]bool{"RegularExpression": true}
	extendedHTTPFilters    = map[string]bool{"URLRewrite": true, "ResponseHeaderModifier": true, "RequestMirror": true}
	extendedGRPCMethods    = map[string]bool{"RegularExpression": true}
)

func (t *CheckGatewayConformanceTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	kind := getStringArg(args, "kind", "")
	name := getStringArg(args, "name", "")
	ns := getStringArg(args, "namespace", "default")

	if kind == "" || name == "" {
		return nil, &types.MCPError{
			Code:    types.ErrCodeInvalidInput,
			Tool:    t.Name(),
			Message: "kind and name are required",
		}
	}

	var findings []types.DiagnosticFinding

	switch kind {
	case "Gateway":
		findings = t.validateGateway(ctx, ns, name)
	case "HTTPRoute":
		findings = t.validateHTTPRoute(ctx, ns, name)
	case "GRPCRoute":
		findings = t.validateGRPCRoute(ctx, ns, name)
	default:
		return nil, &types.MCPError{
			Code:    types.ErrCodeInvalidInput,
			Tool:    t.Name(),
			Message: fmt.Sprintf("unsupported kind %q; must be Gateway, HTTPRoute, or GRPCRoute", kind),
		}
	}

	if len(findings) == 0 {
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityOK,
			Category: types.CategoryRouting,
			Resource: &types.ResourceRef{
				Kind:       kind,
				Namespace:  ns,
				Name:       name,
				APIVersion: "gateway.networking.k8s.io",
			},
			Summary: fmt.Sprintf("%s %s/%s is conformant with the Gateway API specification (core profile)", kind, ns, name),
		})
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, "gateway-api"), nil
}

func (t *CheckGatewayConformanceTool) validateGateway(ctx context.Context, ns, name string) []types.DiagnosticFinding {
	gw, err := getWithFallback(ctx, t.Clients.Dynamic, gatewaysV1GVR, gatewaysV1B1GVR, ns, name)
	if err != nil {
		return []types.DiagnosticFinding{{
			Severity: types.SeverityWarning,
			Category: types.CategoryRouting,
			Resource: &types.ResourceRef{Kind: "Gateway", Namespace: ns, Name: name, APIVersion: "gateway.networking.k8s.io"},
			Summary:  fmt.Sprintf("Gateway %s/%s not found: %v", ns, name, err),
		}}
	}

	ref := &types.ResourceRef{Kind: "Gateway", Namespace: ns, Name: name, APIVersion: "gateway.networking.k8s.io"}
	var findings []types.DiagnosticFinding
	extendedFeatures := make(map[string]bool)

	// Validate gatewayClassName (required)
	gatewayClass := getNestedString(gw.Object, "spec", "gatewayClassName")
	if gatewayClass == "" {
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityWarning,
			Category:   types.CategoryRouting,
			Resource:   ref,
			Summary:    "spec.gatewayClassName is required but missing",
			Suggestion: "Set spec.gatewayClassName to a valid GatewayClass name",
		})
	}

	// Validate listeners
	listeners, _, _ := unstructured.NestedSlice(gw.Object, "spec", "listeners")
	if len(listeners) == 0 {
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityWarning,
			Category:   types.CategoryRouting,
			Resource:   ref,
			Summary:    "spec.listeners is required but empty or missing",
			Suggestion: "Add at least one listener to the Gateway",
		})
	}

	for i, l := range listeners {
		lm, ok := l.(map[string]interface{})
		if !ok {
			continue
		}

		lName, _ := lm["name"].(string)
		prefix := fmt.Sprintf("spec.listeners[%d]", i)
		if lName != "" {
			prefix = fmt.Sprintf("spec.listeners[%d] (%s)", i, lName)
		}

		// name is required
		if lName == "" {
			findings = append(findings, types.DiagnosticFinding{
				Severity:   types.SeverityWarning,
				Category:   types.CategoryRouting,
				Resource:   ref,
				Summary:    fmt.Sprintf("%s: name is required but missing", prefix),
				Suggestion: "Set a unique name for each listener",
			})
		}

		// protocol validation
		protocol, _ := lm["protocol"].(string)
		if protocol == "" {
			findings = append(findings, types.DiagnosticFinding{
				Severity:   types.SeverityWarning,
				Category:   types.CategoryRouting,
				Resource:   ref,
				Summary:    fmt.Sprintf("%s: protocol is required but missing", prefix),
				Suggestion: "Set protocol to one of: HTTP, HTTPS, TLS, TCP, UDP",
			})
		} else if !validListenerProtocols[protocol] {
			findings = append(findings, types.DiagnosticFinding{
				Severity:   types.SeverityWarning,
				Category:   types.CategoryRouting,
				Resource:   ref,
				Summary:    fmt.Sprintf("%s: invalid protocol %q", prefix, protocol),
				Detail:     "Valid protocols: HTTP, HTTPS, TLS, TCP, UDP",
				Suggestion: "Use a valid Gateway API protocol value",
			})
		} else if extendedProtocols[protocol] {
			extendedFeatures["protocol "+protocol] = true
		}

		// port validation (1-65535)
		port, portOk := lm["port"].(float64)
		if !portOk {
			findings = append(findings, types.DiagnosticFinding{
				Severity:   types.SeverityWarning,
				Category:   types.CategoryRouting,
				Resource:   ref,
				Summary:    fmt.Sprintf("%s: port is required but missing or invalid", prefix),
				Suggestion: "Set port to a value between 1 and 65535",
			})
		} else if port < 1 || port > 65535 {
			findings = append(findings, types.DiagnosticFinding{
				Severity:   types.SeverityWarning,
				Category:   types.CategoryRouting,
				Resource:   ref,
				Summary:    fmt.Sprintf("%s: port %d is out of range (1-65535)", prefix, int(port)),
				Suggestion: "Set port to a value between 1 and 65535",
			})
		}

		// TLS validation for HTTPS and TLS protocols
		if protocol == "HTTPS" || protocol == "TLS" {
			tls, tlsFound, _ := unstructured.NestedMap(lm, "tls")
			if !tlsFound {
				findings = append(findings, types.DiagnosticFinding{
					Severity:   types.SeverityWarning,
					Category:   types.CategoryRouting,
					Resource:   ref,
					Summary:    fmt.Sprintf("%s: tls configuration is required for %s protocol", prefix, protocol),
					Suggestion: "Add tls configuration with certificateRefs",
				})
			} else {
				// TLS mode validation
				mode, _ := tls["mode"].(string)
				if mode != "" {
					if !validTLSModes[mode] {
						findings = append(findings, types.DiagnosticFinding{
							Severity:   types.SeverityWarning,
							Category:   types.CategoryRouting,
							Resource:   ref,
							Summary:    fmt.Sprintf("%s: invalid TLS mode %q", prefix, mode),
							Detail:     "Valid TLS modes: Terminate, Passthrough",
							Suggestion: "Use a valid TLS mode",
						})
					} else if extendedTLSModes[mode] {
						extendedFeatures["TLS mode "+mode] = true
					}
				}

				// certificateRefs required for Terminate mode (or default mode)
				if mode == "" || mode == "Terminate" {
					certRefs, _, _ := unstructured.NestedSlice(tls, "certificateRefs")
					if len(certRefs) == 0 {
						findings = append(findings, types.DiagnosticFinding{
							Severity:   types.SeverityWarning,
							Category:   types.CategoryRouting,
							Resource:   ref,
							Summary:    fmt.Sprintf("%s: tls.certificateRefs is required for TLS Terminate mode", prefix),
							Suggestion: "Add at least one certificateRef pointing to a TLS Secret",
						})
					}
				}
			}
		}
	}

	// Report extended profile if any extended features detected
	if len(extendedFeatures) > 0 {
		featureList := make([]string, 0, len(extendedFeatures))
		for f := range extendedFeatures {
			featureList = append(featureList, f)
		}
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryRouting,
			Resource: ref,
			Summary:  fmt.Sprintf("Gateway uses extended conformance features: %s", strings.Join(featureList, ", ")),
			Detail:   "These features require the implementation to support the extended conformance profile",
		})
	}

	return findings
}

func (t *CheckGatewayConformanceTool) validateHTTPRoute(ctx context.Context, ns, name string) []types.DiagnosticFinding {
	route, err := getWithFallback(ctx, t.Clients.Dynamic, httpRoutesV1GVR, httpRoutesV1B1GVR, ns, name)
	if err != nil {
		return []types.DiagnosticFinding{{
			Severity: types.SeverityWarning,
			Category: types.CategoryRouting,
			Resource: &types.ResourceRef{Kind: "HTTPRoute", Namespace: ns, Name: name, APIVersion: "gateway.networking.k8s.io"},
			Summary:  fmt.Sprintf("HTTPRoute %s/%s not found: %v", ns, name, err),
		}}
	}

	ref := &types.ResourceRef{Kind: "HTTPRoute", Namespace: ns, Name: name, APIVersion: "gateway.networking.k8s.io"}
	var findings []types.DiagnosticFinding
	extendedFeatures := make(map[string]bool)

	// parentRefs required
	parentRefs, _, _ := unstructured.NestedSlice(route.Object, "spec", "parentRefs")
	if len(parentRefs) == 0 {
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityWarning,
			Category:   types.CategoryRouting,
			Resource:   ref,
			Summary:    "spec.parentRefs is required but empty or missing",
			Suggestion: "Add at least one parentRef pointing to a Gateway",
		})
	}

	// Validate rules
	rules, _, _ := unstructured.NestedSlice(route.Object, "spec", "rules")
	for i, r := range rules {
		rm, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		prefix := fmt.Sprintf("spec.rules[%d]", i)

		// Validate matches
		if matches, ok := rm["matches"].([]interface{}); ok {
			for j, m := range matches {
				mm, ok := m.(map[string]interface{})
				if !ok {
					continue
				}
				mPrefix := fmt.Sprintf("%s.matches[%d]", prefix, j)

				// Path match type
				if pathMatch, ok := mm["path"].(map[string]interface{}); ok {
					matchType, _ := pathMatch["type"].(string)
					if matchType != "" && !validPathMatchTypes[matchType] {
						findings = append(findings, types.DiagnosticFinding{
							Severity:   types.SeverityWarning,
							Category:   types.CategoryRouting,
							Resource:   ref,
							Summary:    fmt.Sprintf("%s.path.type %q is not a valid PathMatchType", mPrefix, matchType),
							Detail:     "Valid values: Exact, PathPrefix, RegularExpression",
							Suggestion: "Use a valid PathMatchType",
						})
					} else if extendedPathMatchTypes[matchType] {
						extendedFeatures["RegularExpression path match"] = true
					}
					// PathPrefix must start with /
					if matchType == "PathPrefix" || matchType == "" {
						value, _ := pathMatch["value"].(string)
						if value != "" && !strings.HasPrefix(value, "/") {
							findings = append(findings, types.DiagnosticFinding{
								Severity:   types.SeverityWarning,
								Category:   types.CategoryRouting,
								Resource:   ref,
								Summary:    fmt.Sprintf("%s.path.value %q must start with '/' for PathPrefix match", mPrefix, value),
								Suggestion: "Prefix the path value with /",
							})
						}
					}
				}

				// Header match types
				if headers, ok := mm["headers"].([]interface{}); ok {
					for k, h := range headers {
						if hm, ok := h.(map[string]interface{}); ok {
							hType, _ := hm["type"].(string)
							if hType != "" && !validHeaderMatchTypes[hType] {
								findings = append(findings, types.DiagnosticFinding{
									Severity:   types.SeverityWarning,
									Category:   types.CategoryRouting,
									Resource:   ref,
									Summary:    fmt.Sprintf("%s.headers[%d].type %q is not a valid HeaderMatchType", mPrefix, k, hType),
									Detail:     "Valid values: Exact, RegularExpression",
									Suggestion: "Use a valid HeaderMatchType",
								})
							}
						}
					}
				}

				// Query param match types
				if queryParams, ok := mm["queryParams"].([]interface{}); ok {
					for k, q := range queryParams {
						if qm, ok := q.(map[string]interface{}); ok {
							qType, _ := qm["type"].(string)
							if qType != "" && !validQueryMatchTypes[qType] {
								findings = append(findings, types.DiagnosticFinding{
									Severity:   types.SeverityWarning,
									Category:   types.CategoryRouting,
									Resource:   ref,
									Summary:    fmt.Sprintf("%s.queryParams[%d].type %q is not a valid QueryParamMatchType", mPrefix, k, qType),
									Detail:     "Valid values: Exact, RegularExpression",
									Suggestion: "Use a valid QueryParamMatchType",
								})
							}
						}
					}
				}

				// HTTP method
				if method, ok := mm["method"].(string); ok {
					if !validHTTPMethods[method] {
						findings = append(findings, types.DiagnosticFinding{
							Severity:   types.SeverityWarning,
							Category:   types.CategoryRouting,
							Resource:   ref,
							Summary:    fmt.Sprintf("%s.method %q is not a valid HTTP method", mPrefix, method),
							Detail:     "Valid values: GET, HEAD, POST, PUT, DELETE, CONNECT, OPTIONS, TRACE, PATCH",
							Suggestion: "Use a valid HTTP method",
						})
					}
				}
			}
		}

		// Validate filters
		if filters, ok := rm["filters"].([]interface{}); ok {
			for j, f := range filters {
				if fm, ok := f.(map[string]interface{}); ok {
					fType, _ := fm["type"].(string)
					fPrefix := fmt.Sprintf("%s.filters[%d]", prefix, j)
					if fType != "" && !validHTTPFilterTypes[fType] {
						findings = append(findings, types.DiagnosticFinding{
							Severity:   types.SeverityWarning,
							Category:   types.CategoryRouting,
							Resource:   ref,
							Summary:    fmt.Sprintf("%s.type %q is not a valid HTTPRouteFilterType", fPrefix, fType),
							Detail:     "Valid values: RequestHeaderModifier, ResponseHeaderModifier, RequestMirror, RequestRedirect, URLRewrite, ExtensionRef",
							Suggestion: "Use a valid HTTPRouteFilterType",
						})
					} else if extendedHTTPFilters[fType] {
						extendedFeatures["filter "+fType] = true
					}
				}
			}
		}

		// Validate backendRefs — port is required for Service backends
		if brs, ok := rm["backendRefs"].([]interface{}); ok {
			for j, br := range brs {
				if brm, ok := br.(map[string]interface{}); ok {
					brKind, _ := brm["kind"].(string)
					if brKind == "" || brKind == "Service" {
						if _, hasPort := brm["port"]; !hasPort {
							brName, _ := brm["name"].(string)
							findings = append(findings, types.DiagnosticFinding{
								Severity:   types.SeverityWarning,
								Category:   types.CategoryRouting,
								Resource:   ref,
								Summary:    fmt.Sprintf("%s.backendRefs[%d]: port is required for Service backend %q", prefix, j, brName),
								Suggestion: "Add a port field to the backendRef",
							})
						}
					}
				}
			}
		}
	}

	// Report extended profile
	if len(extendedFeatures) > 0 {
		featureList := make([]string, 0, len(extendedFeatures))
		for f := range extendedFeatures {
			featureList = append(featureList, f)
		}
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryRouting,
			Resource: ref,
			Summary:  fmt.Sprintf("HTTPRoute uses extended conformance features: %s", strings.Join(featureList, ", ")),
			Detail:   "These features require the implementation to support the extended conformance profile",
		})
	}

	return findings
}

func (t *CheckGatewayConformanceTool) validateGRPCRoute(ctx context.Context, ns, name string) []types.DiagnosticFinding {
	route, err := getWithFallback(ctx, t.Clients.Dynamic, grpcRoutesV1GVR, grpcRoutesV1B1GVR, ns, name)
	if err != nil {
		return []types.DiagnosticFinding{{
			Severity: types.SeverityWarning,
			Category: types.CategoryRouting,
			Resource: &types.ResourceRef{Kind: "GRPCRoute", Namespace: ns, Name: name, APIVersion: "gateway.networking.k8s.io"},
			Summary:  fmt.Sprintf("GRPCRoute %s/%s not found: %v", ns, name, err),
		}}
	}

	ref := &types.ResourceRef{Kind: "GRPCRoute", Namespace: ns, Name: name, APIVersion: "gateway.networking.k8s.io"}
	var findings []types.DiagnosticFinding
	extendedFeatures := make(map[string]bool)

	// parentRefs required
	parentRefs, _, _ := unstructured.NestedSlice(route.Object, "spec", "parentRefs")
	if len(parentRefs) == 0 {
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityWarning,
			Category:   types.CategoryRouting,
			Resource:   ref,
			Summary:    "spec.parentRefs is required but empty or missing",
			Suggestion: "Add at least one parentRef pointing to a Gateway",
		})
	}

	// Validate rules
	rules, _, _ := unstructured.NestedSlice(route.Object, "spec", "rules")
	for i, r := range rules {
		rm, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		prefix := fmt.Sprintf("spec.rules[%d]", i)

		// Validate gRPC method matches
		if matches, ok := rm["matches"].([]interface{}); ok {
			for j, m := range matches {
				mm, ok := m.(map[string]interface{})
				if !ok {
					continue
				}
				mPrefix := fmt.Sprintf("%s.matches[%d]", prefix, j)

				if method, ok := mm["method"].(map[string]interface{}); ok {
					matchType, _ := method["type"].(string)
					svc, _ := method["service"].(string)
					meth, _ := method["method"].(string)

					// type validation
					if matchType != "" && !validGRPCMethodTypes[matchType] {
						findings = append(findings, types.DiagnosticFinding{
							Severity:   types.SeverityWarning,
							Category:   types.CategoryRouting,
							Resource:   ref,
							Summary:    fmt.Sprintf("%s.method.type %q is not a valid GRPCMethodMatchType", mPrefix, matchType),
							Detail:     "Valid values: Exact, RegularExpression",
							Suggestion: "Use a valid GRPCMethodMatchType",
						})
					} else if extendedGRPCMethods[matchType] {
						extendedFeatures["RegularExpression gRPC method match"] = true
					}

					// At least service or method must be specified
					if svc == "" && meth == "" {
						findings = append(findings, types.DiagnosticFinding{
							Severity:   types.SeverityWarning,
							Category:   types.CategoryRouting,
							Resource:   ref,
							Summary:    fmt.Sprintf("%s.method: at least one of service or method must be specified", mPrefix),
							Suggestion: "Set service, method, or both in the gRPC method match",
						})
					}
				}

				// Header match types
				if headers, ok := mm["headers"].([]interface{}); ok {
					for k, h := range headers {
						if hm, ok := h.(map[string]interface{}); ok {
							hType, _ := hm["type"].(string)
							if hType != "" && !validHeaderMatchTypes[hType] {
								findings = append(findings, types.DiagnosticFinding{
									Severity:   types.SeverityWarning,
									Category:   types.CategoryRouting,
									Resource:   ref,
									Summary:    fmt.Sprintf("%s.headers[%d].type %q is not a valid HeaderMatchType", mPrefix, k, hType),
									Detail:     "Valid values: Exact, RegularExpression",
									Suggestion: "Use a valid HeaderMatchType",
								})
							}
						}
					}
				}
			}
		}

		// Validate filters
		if filters, ok := rm["filters"].([]interface{}); ok {
			for j, f := range filters {
				if fm, ok := f.(map[string]interface{}); ok {
					fType, _ := fm["type"].(string)
					fPrefix := fmt.Sprintf("%s.filters[%d]", prefix, j)
					if fType != "" && !validGRPCFilterTypes[fType] {
						findings = append(findings, types.DiagnosticFinding{
							Severity:   types.SeverityWarning,
							Category:   types.CategoryRouting,
							Resource:   ref,
							Summary:    fmt.Sprintf("%s.type %q is not a valid GRPCRouteFilterType", fPrefix, fType),
							Detail:     "Valid values: RequestHeaderModifier, ResponseHeaderModifier, RequestMirror, ExtensionRef",
							Suggestion: "Use a valid GRPCRouteFilterType",
						})
					}
				}
			}
		}

		// Validate backendRefs — port required for Service
		if brs, ok := rm["backendRefs"].([]interface{}); ok {
			for j, br := range brs {
				if brm, ok := br.(map[string]interface{}); ok {
					brKind, _ := brm["kind"].(string)
					if brKind == "" || brKind == "Service" {
						if _, hasPort := brm["port"]; !hasPort {
							brName, _ := brm["name"].(string)
							findings = append(findings, types.DiagnosticFinding{
								Severity:   types.SeverityWarning,
								Category:   types.CategoryRouting,
								Resource:   ref,
								Summary:    fmt.Sprintf("%s.backendRefs[%d]: port is required for Service backend %q", prefix, j, brName),
								Suggestion: "Add a port field to the backendRef",
							})
						}
					}
				}
			}
		}
	}

	// Report extended profile
	if len(extendedFeatures) > 0 {
		featureList := make([]string, 0, len(extendedFeatures))
		for f := range extendedFeatures {
			featureList = append(featureList, f)
		}
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryRouting,
			Resource: ref,
			Summary:  fmt.Sprintf("GRPCRoute uses extended conformance features: %s", strings.Join(featureList, ", ")),
			Detail:   "These features require the implementation to support the extended conformance profile",
		})
	}

	return findings
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
