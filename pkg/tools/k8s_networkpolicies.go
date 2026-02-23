package tools

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var networkPoliciesGVR = schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"}

// --- list_networkpolicies ---

type ListNetworkPoliciesTool struct{ BaseTool }

func (t *ListNetworkPoliciesTool) Name() string        { return "list_networkpolicies" }
func (t *ListNetworkPoliciesTool) Description() string  { return "List NetworkPolicies with podSelector and rule counts" }
func (t *ListNetworkPoliciesTool) InputSchema() map[string]interface{} {
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

func (t *ListNetworkPoliciesTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	ns := getStringArg(args, "namespace", "")

	var list *unstructured.UnstructuredList
	var err error
	if ns == "" {
		list, err = t.Clients.Dynamic.Resource(networkPoliciesGVR).List(ctx, metav1.ListOptions{})
	} else {
		list, err = t.Clients.Dynamic.Resource(networkPoliciesGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list network policies: %w", err)
	}

	policies := make([]map[string]interface{}, 0, len(list.Items))
	for _, item := range list.Items {
		podSelector, _, _ := unstructured.NestedMap(item.Object, "spec", "podSelector")
		ingress, _, _ := unstructured.NestedSlice(item.Object, "spec", "ingress")
		egress, _, _ := unstructured.NestedSlice(item.Object, "spec", "egress")
		policyTypes, _, _ := unstructured.NestedStringSlice(item.Object, "spec", "policyTypes")

		policies = append(policies, map[string]interface{}{
			"name":         item.GetName(),
			"namespace":    item.GetNamespace(),
			"podSelector":  podSelector,
			"policyTypes":  policyTypes,
			"ingressRules": len(ingress),
			"egressRules":  len(egress),
		})
	}

	return NewResponse(t.Cfg, t.Name(), map[string]interface{}{
		"count":    len(policies),
		"policies": policies,
	}), nil
}

// --- get_networkpolicy ---

type GetNetworkPolicyTool struct{ BaseTool }

func (t *GetNetworkPolicyTool) Name() string        { return "get_networkpolicy" }
func (t *GetNetworkPolicyTool) Description() string  { return "Get full NetworkPolicy with ingress/egress rule details" }
func (t *GetNetworkPolicyTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "NetworkPolicy name",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace",
			},
		},
		"required": []string{"name", "namespace"},
	}
}

func (t *GetNetworkPolicyTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	name := getStringArg(args, "name", "")
	ns := getStringArg(args, "namespace", "default")

	np, err := t.Clients.Dynamic.Resource(networkPoliciesGVR).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get network policy %s/%s: %w", ns, name, err)
	}

	spec, _, _ := unstructured.NestedMap(np.Object, "spec")

	return NewResponse(t.Cfg, t.Name(), map[string]interface{}{
		"name":      name,
		"namespace": ns,
		"spec":      spec,
	}), nil
}
