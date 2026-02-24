package tools

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/isitobservable/k8s-networking-mcp/pkg/types"
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

	findings := make([]types.DiagnosticFinding, 0, len(list.Items))
	for _, item := range list.Items {
		podSelector, _, _ := unstructured.NestedMap(item.Object, "spec", "podSelector")
		ingress, _, _ := unstructured.NestedSlice(item.Object, "spec", "ingress")
		egress, _, _ := unstructured.NestedSlice(item.Object, "spec", "egress")
		policyTypes, _, _ := unstructured.NestedStringSlice(item.Object, "spec", "policyTypes")

		severity := types.SeverityInfo
		suggestion := ""

		// Detect block-all-ingress: has Ingress policyType but 0 ingress rules
		hasIngressType := false
		for _, pt := range policyTypes {
			if pt == "Ingress" {
				hasIngressType = true
			}
		}
		if hasIngressType && len(ingress) == 0 {
			severity = types.SeverityWarning
			suggestion = "This policy blocks ALL ingress traffic for selected pods. Verify this is intentional."
		}

		selectorLabels, _, _ := unstructured.NestedStringMap(podSelector, "matchLabels")
		selectorStr := formatSelector(selectorLabels)

		findings = append(findings, types.DiagnosticFinding{
			Severity: severity,
			Category: types.CategoryPolicy,
			Resource: &types.ResourceRef{
				Kind:       "NetworkPolicy",
				Namespace:  item.GetNamespace(),
				Name:       item.GetName(),
				APIVersion: "networking.k8s.io/v1",
			},
			Summary:    fmt.Sprintf("%s/%s podSelector={%s} types=%v ingress=%d egress=%d", item.GetNamespace(), item.GetName(), selectorStr, policyTypes, len(ingress), len(egress)),
			Detail:     fmt.Sprintf("podSelector=%v policyTypes=%v ingressRules=%d egressRules=%d", podSelector, policyTypes, len(ingress), len(egress)),
			Suggestion: suggestion,
		})
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, ""), nil
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

	ref := &types.ResourceRef{Kind: "NetworkPolicy", Namespace: ns, Name: name, APIVersion: "networking.k8s.io/v1"}
	findings := make([]types.DiagnosticFinding, 0, 4)

	podSelector, _, _ := unstructured.NestedMap(np.Object, "spec", "podSelector")
	policyTypes, _, _ := unstructured.NestedStringSlice(np.Object, "spec", "policyTypes")
	ingress, _, _ := unstructured.NestedSlice(np.Object, "spec", "ingress")
	egress, _, _ := unstructured.NestedSlice(np.Object, "spec", "egress")

	selectorLabels, _, _ := unstructured.NestedStringMap(podSelector, "matchLabels")
	selectorStr := formatSelector(selectorLabels)

	// Overview finding
	findings = append(findings, types.DiagnosticFinding{
		Severity: types.SeverityInfo,
		Category: types.CategoryPolicy,
		Resource: ref,
		Summary:  fmt.Sprintf("%s/%s podSelector={%s} types=%v", ns, name, selectorStr, policyTypes),
		Detail:   fmt.Sprintf("podSelector=%v policyTypes=%v", podSelector, policyTypes),
	})

	// Ingress rules
	for i, rule := range ingress {
		rm, ok := rule.(map[string]interface{})
		if !ok {
			continue
		}
		ruleSummary := describeNetpolRule(rm, "ingress", i)
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryPolicy,
			Resource: ref,
			Summary:  ruleSummary,
			Detail:   fmt.Sprintf("ingressRule[%d]=%v", i, rm),
		})
	}

	// Egress rules
	for i, rule := range egress {
		rm, ok := rule.(map[string]interface{})
		if !ok {
			continue
		}
		ruleSummary := describeNetpolRule(rm, "egress", i)
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryPolicy,
			Resource: ref,
			Summary:  ruleSummary,
			Detail:   fmt.Sprintf("egressRule[%d]=%v", i, rm),
		})
	}

	// Warn if block-all-ingress
	hasIngressType := false
	for _, pt := range policyTypes {
		if pt == "Ingress" {
			hasIngressType = true
		}
	}
	if hasIngressType && len(ingress) == 0 {
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityWarning,
			Category:   types.CategoryPolicy,
			Resource:   ref,
			Summary:    "policy blocks ALL ingress traffic for selected pods",
			Suggestion: "This policy has policyType=Ingress but no ingress rules, which blocks all incoming traffic. Add ingress rules to allow specific traffic.",
		})
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, ""), nil
}

func formatSelector(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	parts := make([]string, 0, len(labels))
	for k, v := range labels {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, ",")
}

func describeNetpolRule(rule map[string]interface{}, direction string, index int) string {
	parts := []string{fmt.Sprintf("%s rule[%d]:", direction, index)}

	// Ports
	if portsSlice, ok := rule["ports"].([]interface{}); ok && len(portsSlice) > 0 {
		portDescs := make([]string, 0, len(portsSlice))
		for _, p := range portsSlice {
			if pm, ok := p.(map[string]interface{}); ok {
				portDescs = append(portDescs, fmt.Sprintf("%v/%v", pm["port"], pm["protocol"]))
			}
		}
		parts = append(parts, fmt.Sprintf("ports=[%s]", strings.Join(portDescs, ",")))
	}

	// Peers (from/to)
	peerKey := "from"
	if direction == "egress" {
		peerKey = "to"
	}
	if peers, ok := rule[peerKey].([]interface{}); ok && len(peers) > 0 {
		peerDescs := make([]string, 0, len(peers))
		for _, peer := range peers {
			if pm, ok := peer.(map[string]interface{}); ok {
				if _, hasPod := pm["podSelector"]; hasPod {
					peerDescs = append(peerDescs, "podSelector")
				}
				if _, hasNS := pm["namespaceSelector"]; hasNS {
					peerDescs = append(peerDescs, "namespaceSelector")
				}
				if _, hasIP := pm["ipBlock"]; hasIP {
					peerDescs = append(peerDescs, "ipBlock")
				}
			}
		}
		parts = append(parts, fmt.Sprintf("%s=[%s]", peerKey, strings.Join(peerDescs, ",")))
	}

	return strings.Join(parts, " ")
}
