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
