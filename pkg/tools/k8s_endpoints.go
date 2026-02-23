package tools

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// --- list_endpoints ---

type ListEndpointsTool struct{ BaseTool }

func (t *ListEndpointsTool) Name() string        { return "list_endpoints" }
func (t *ListEndpointsTool) Description() string  { return "List endpoints with ready/not-ready address counts" }
func (t *ListEndpointsTool) InputSchema() map[string]interface{} {
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

func (t *ListEndpointsTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	ns := getStringArg(args, "namespace", "")

	var list *unstructured.UnstructuredList
	var err error
	if ns == "" {
		list, err = t.Clients.Dynamic.Resource(endpointsGVR).List(ctx, metav1.ListOptions{})
	} else {
		list, err = t.Clients.Dynamic.Resource(endpointsGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list endpoints: %w", err)
	}

	endpoints := make([]map[string]interface{}, 0, len(list.Items))
	for _, item := range list.Items {
		subsets, _, _ := unstructured.NestedSlice(item.Object, "subsets")
		readyCount := 0
		notReadyCount := 0
		for _, s := range subsets {
			if sm, ok := s.(map[string]interface{}); ok {
				if addrs, ok := sm["addresses"].([]interface{}); ok {
					readyCount += len(addrs)
				}
				if addrs, ok := sm["notReadyAddresses"].([]interface{}); ok {
					notReadyCount += len(addrs)
				}
			}
		}

		endpoints = append(endpoints, map[string]interface{}{
			"name":              item.GetName(),
			"namespace":         item.GetNamespace(),
			"readyAddresses":    readyCount,
			"notReadyAddresses": notReadyCount,
		})
	}

	return NewResponse(t.Cfg, t.Name(), map[string]interface{}{
		"count":     len(endpoints),
		"endpoints": endpoints,
	}), nil
}
