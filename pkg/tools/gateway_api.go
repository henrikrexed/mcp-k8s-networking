package tools

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var gatewaysGVR = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gateways"}
var httpRoutesGVR = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes"}

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

	var list *unstructured.UnstructuredList
	var err error
	if ns == "" {
		list, err = t.Clients.Dynamic.Resource(gatewaysGVR).List(ctx, metav1.ListOptions{})
	} else {
		list, err = t.Clients.Dynamic.Resource(gatewaysGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list gateways: %w", err)
	}

	gateways := make([]map[string]interface{}, 0, len(list.Items))
	for _, item := range list.Items {
		listeners, _, _ := unstructured.NestedSlice(item.Object, "spec", "listeners")
		conditions, _, _ := unstructured.NestedSlice(item.Object, "status", "conditions")

		// Count attached routes from listener status
		listenerStatuses, _, _ := unstructured.NestedSlice(item.Object, "status", "listeners")
		attachedRoutes := 0
		for _, ls := range listenerStatuses {
			if lsm, ok := ls.(map[string]interface{}); ok {
				if count, ok := lsm["attachedRoutes"].(int64); ok {
					attachedRoutes += int(count)
				}
			}
		}

		listenerSummary := make([]map[string]interface{}, 0)
		for _, l := range listeners {
			if lm, ok := l.(map[string]interface{}); ok {
				listenerSummary = append(listenerSummary, map[string]interface{}{
					"name":     lm["name"],
					"port":     lm["port"],
					"protocol": lm["protocol"],
					"hostname": lm["hostname"],
				})
			}
		}

		conditionSummary := extractConditions(conditions)

		gateways = append(gateways, map[string]interface{}{
			"name":           item.GetName(),
			"namespace":      item.GetNamespace(),
			"gatewayClass":   getNestedString(item.Object, "spec", "gatewayClassName"),
			"listeners":      listenerSummary,
			"conditions":     conditionSummary,
			"attachedRoutes": attachedRoutes,
		})
	}

	return NewResponse(t.Cfg, t.Name(), map[string]interface{}{
		"count":    len(gateways),
		"gateways": gateways,
	}), nil
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

	gw, err := t.Clients.Dynamic.Resource(gatewaysGVR).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway %s/%s: %w", ns, name, err)
	}

	spec, _, _ := unstructured.NestedMap(gw.Object, "spec")
	status, _, _ := unstructured.NestedMap(gw.Object, "status")
	conditions, _, _ := unstructured.NestedSlice(gw.Object, "status", "conditions")
	addresses, _, _ := unstructured.NestedSlice(gw.Object, "status", "addresses")

	// Find attached HTTPRoutes
	routeList, _ := t.Clients.Dynamic.Resource(httpRoutesGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	attachedRoutes := make([]map[string]interface{}, 0)
	if routeList != nil {
		for _, route := range routeList.Items {
			parentRefs, _, _ := unstructured.NestedSlice(route.Object, "spec", "parentRefs")
			for _, pr := range parentRefs {
				if prm, ok := pr.(map[string]interface{}); ok {
					refName, _ := prm["name"].(string)
					if refName == name {
						attachedRoutes = append(attachedRoutes, map[string]interface{}{
							"name":      route.GetName(),
							"namespace": route.GetNamespace(),
						})
						break
					}
				}
			}
		}
	}

	return NewResponse(t.Cfg, t.Name(), map[string]interface{}{
		"name":           name,
		"namespace":      ns,
		"spec":           spec,
		"conditions":     extractConditions(conditions),
		"addresses":      addresses,
		"attachedRoutes": attachedRoutes,
		"status":         status,
	}), nil
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

	var list *unstructured.UnstructuredList
	var err error
	if ns == "" {
		list, err = t.Clients.Dynamic.Resource(httpRoutesGVR).List(ctx, metav1.ListOptions{})
	} else {
		list, err = t.Clients.Dynamic.Resource(httpRoutesGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list httproutes: %w", err)
	}

	routes := make([]map[string]interface{}, 0, len(list.Items))
	for _, item := range list.Items {
		parentRefs, _, _ := unstructured.NestedSlice(item.Object, "spec", "parentRefs")
		rules, _, _ := unstructured.NestedSlice(item.Object, "spec", "rules")

		parentRefSummary := make([]map[string]interface{}, 0)
		for _, pr := range parentRefs {
			if prm, ok := pr.(map[string]interface{}); ok {
				parentRefSummary = append(parentRefSummary, map[string]interface{}{
					"name":        prm["name"],
					"namespace":   prm["namespace"],
					"sectionName": prm["sectionName"],
				})
			}
		}

		// Extract backend refs from rules
		backendRefs := make([]map[string]interface{}, 0)
		for _, r := range rules {
			if rm, ok := r.(map[string]interface{}); ok {
				if brs, ok := rm["backendRefs"].([]interface{}); ok {
					for _, br := range brs {
						if brm, ok := br.(map[string]interface{}); ok {
							backendRefs = append(backendRefs, map[string]interface{}{
								"name":   brm["name"],
								"port":   brm["port"],
								"weight": brm["weight"],
							})
						}
					}
				}
			}
		}

		routes = append(routes, map[string]interface{}{
			"name":        item.GetName(),
			"namespace":   item.GetNamespace(),
			"parentRefs":  parentRefSummary,
			"ruleCount":   len(rules),
			"backendRefs": backendRefs,
		})
	}

	return NewResponse(t.Cfg, t.Name(), map[string]interface{}{
		"count":  len(routes),
		"routes": routes,
	}), nil
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

	route, err := t.Clients.Dynamic.Resource(httpRoutesGVR).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get httproute %s/%s: %w", ns, name, err)
	}

	spec, _, _ := unstructured.NestedMap(route.Object, "spec")
	status, _, _ := unstructured.NestedMap(route.Object, "status")

	// Check backend service health
	rules, _, _ := unstructured.NestedSlice(route.Object, "spec", "rules")
	backendHealth := make([]map[string]interface{}, 0)
	for _, r := range rules {
		if rm, ok := r.(map[string]interface{}); ok {
			if brs, ok := rm["backendRefs"].([]interface{}); ok {
				for _, br := range brs {
					if brm, ok := br.(map[string]interface{}); ok {
						refName, _ := brm["name"].(string)
						refNs := ns
						if rns, ok := brm["namespace"].(string); ok && rns != "" {
							refNs = rns
						}
						health := map[string]interface{}{
							"name":      refName,
							"namespace": refNs,
						}
						// Check if service exists and has ready endpoints
						_, svcErr := t.Clients.Dynamic.Resource(servicesGVR).Namespace(refNs).Get(ctx, refName, metav1.GetOptions{})
						if svcErr != nil {
							health["exists"] = false
							health["error"] = svcErr.Error()
						} else {
							health["exists"] = true
							ep, epErr := t.Clients.Dynamic.Resource(endpointsGVR).Namespace(refNs).Get(ctx, refName, metav1.GetOptions{})
							if epErr == nil {
								subsets, _, _ := unstructured.NestedSlice(ep.Object, "subsets")
								readyCount := 0
								for _, s := range subsets {
									if sm, ok := s.(map[string]interface{}); ok {
										if addrs, ok := sm["addresses"].([]interface{}); ok {
											readyCount += len(addrs)
										}
									}
								}
								health["readyEndpoints"] = readyCount
							}
						}
						backendHealth = append(backendHealth, health)
					}
				}
			}
		}
	}

	return NewResponse(t.Cfg, t.Name(), map[string]interface{}{
		"name":          name,
		"namespace":     ns,
		"spec":          spec,
		"status":        status,
		"backendHealth": backendHealth,
	}), nil
}

// Helper functions

func extractConditions(conditions []interface{}) []map[string]interface{} {
	result := make([]map[string]interface{}, 0)
	for _, c := range conditions {
		if cm, ok := c.(map[string]interface{}); ok {
			result = append(result, map[string]interface{}{
				"type":    cm["type"],
				"status":  cm["status"],
				"reason":  cm["reason"],
				"message": cm["message"],
			})
		}
	}
	return result
}

func getNestedString(obj map[string]interface{}, fields ...string) string {
	val, _, _ := unstructured.NestedString(obj, fields...)
	return val
}
