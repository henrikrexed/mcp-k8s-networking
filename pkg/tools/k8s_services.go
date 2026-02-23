package tools

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var servicesGVR = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}
var endpointsGVR = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "endpoints"}
var podsGVR = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}

// --- list_services ---

type ListServicesTool struct{ BaseTool }

func (t *ListServicesTool) Name() string        { return "list_services" }
func (t *ListServicesTool) Description() string  { return "List Kubernetes services with type, clusterIP, ports, and selector" }
func (t *ListServicesTool) InputSchema() map[string]interface{} {
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

func (t *ListServicesTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	ns := getStringArg(args, "namespace", "")

	var list *unstructured.UnstructuredList
	var err error
	if ns == "" {
		list, err = t.Clients.Dynamic.Resource(servicesGVR).List(ctx, metav1.ListOptions{})
	} else {
		list, err = t.Clients.Dynamic.Resource(servicesGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	services := make([]map[string]interface{}, 0, len(list.Items))
	for _, item := range list.Items {
		spec, _, _ := unstructured.NestedMap(item.Object, "spec")
		svcType, _, _ := unstructured.NestedString(item.Object, "spec", "type")
		clusterIP, _, _ := unstructured.NestedString(item.Object, "spec", "clusterIP")
		selector, _, _ := unstructured.NestedStringMap(item.Object, "spec", "selector")
		ports, _, _ := unstructured.NestedSlice(spec, "ports")

		portSummary := make([]map[string]interface{}, 0)
		for _, p := range ports {
			if pm, ok := p.(map[string]interface{}); ok {
				portSummary = append(portSummary, map[string]interface{}{
					"name":       pm["name"],
					"port":       pm["port"],
					"targetPort": pm["targetPort"],
					"protocol":   pm["protocol"],
				})
			}
		}

		services = append(services, map[string]interface{}{
			"name":      item.GetName(),
			"namespace": item.GetNamespace(),
			"type":      svcType,
			"clusterIP": clusterIP,
			"ports":     portSummary,
			"selector":  selector,
		})
	}

	return NewResponse(t.Cfg, t.Name(), map[string]interface{}{
		"count":    len(services),
		"services": services,
	}), nil
}

// --- get_service ---

type GetServiceTool struct{ BaseTool }

func (t *GetServiceTool) Name() string        { return "get_service" }
func (t *GetServiceTool) Description() string  { return "Get detailed service info including endpoints and matching pod status" }
func (t *GetServiceTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Service name",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace",
			},
		},
		"required": []string{"name", "namespace"},
	}
}

func (t *GetServiceTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	name := getStringArg(args, "name", "")
	ns := getStringArg(args, "namespace", "default")

	svc, err := t.Clients.Dynamic.Resource(servicesGVR).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get service %s/%s: %w", ns, name, err)
	}

	spec, _, _ := unstructured.NestedMap(svc.Object, "spec")
	svcType, _, _ := unstructured.NestedString(svc.Object, "spec", "type")
	clusterIP, _, _ := unstructured.NestedString(svc.Object, "spec", "clusterIP")
	selector, _, _ := unstructured.NestedStringMap(svc.Object, "spec", "selector")
	ports, _, _ := unstructured.NestedSlice(spec, "ports")

	portSummary := make([]map[string]interface{}, 0)
	for _, p := range ports {
		if pm, ok := p.(map[string]interface{}); ok {
			portSummary = append(portSummary, map[string]interface{}{
				"name":       pm["name"],
				"port":       pm["port"],
				"targetPort": pm["targetPort"],
				"protocol":   pm["protocol"],
				"nodePort":   pm["nodePort"],
			})
		}
	}

	// Fetch endpoints
	ep, err := t.Clients.Dynamic.Resource(endpointsGVR).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
	var endpointData interface{}
	if err == nil {
		subsets, _, _ := unstructured.NestedSlice(ep.Object, "subsets")
		epInfo := make([]map[string]interface{}, 0)
		for _, s := range subsets {
			if sm, ok := s.(map[string]interface{}); ok {
				addresses, _ := sm["addresses"].([]interface{})
				notReady, _ := sm["notReadyAddresses"].([]interface{})
				epInfo = append(epInfo, map[string]interface{}{
					"readyAddresses":    len(addresses),
					"notReadyAddresses": len(notReady),
				})
			}
		}
		endpointData = epInfo
	}

	// Fetch matching pods
	var podData interface{}
	if len(selector) > 0 {
		labelSelector := ""
		for k, v := range selector {
			if labelSelector != "" {
				labelSelector += ","
			}
			labelSelector += k + "=" + v
		}
		podList, err := t.Clients.Dynamic.Resource(podsGVR).Namespace(ns).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err == nil {
			pods := make([]map[string]interface{}, 0, len(podList.Items))
			for _, pod := range podList.Items {
				phase, _, _ := unstructured.NestedString(pod.Object, "status", "phase")
				podIP, _, _ := unstructured.NestedString(pod.Object, "status", "podIP")
				pods = append(pods, map[string]interface{}{
					"name":   pod.GetName(),
					"phase":  phase,
					"podIP":  podIP,
				})
			}
			podData = pods
		}
	}

	return NewResponse(t.Cfg, t.Name(), map[string]interface{}{
		"name":      name,
		"namespace": ns,
		"type":      svcType,
		"clusterIP": clusterIP,
		"ports":     portSummary,
		"selector":  selector,
		"endpoints": endpointData,
		"pods":      podData,
	}), nil
}
