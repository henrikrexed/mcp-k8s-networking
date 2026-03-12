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

var (
	ciliumNPGVR  = schema.GroupVersionResource{Group: "cilium.io", Version: "v2", Resource: "ciliumnetworkpolicies"}
	ciliumCNPGVR = schema.GroupVersionResource{Group: "cilium.io", Version: "v2", Resource: "ciliumclusterwidenetworkpolicies"}
	ciliumEPGVR  = schema.GroupVersionResource{Group: "cilium.io", Version: "v2", Resource: "ciliumendpoints"}
)

// ciliumRuleCounts holds L3/L4/L7 counts for a policy spec.
type ciliumRuleCounts struct {
	ingressRules int
	egressRules  int
	l4PortRules  int
	l7Rules      int
}

// countCiliumRules walks ingress/egress slices and counts L4/L7 rules.
func countCiliumRules(ingress, egress []interface{}) ciliumRuleCounts {
	counts := ciliumRuleCounts{
		ingressRules: len(ingress),
		egressRules:  len(egress),
	}
	for _, rules := range [][]interface{}{ingress, egress} {
		for _, rule := range rules {
			rm, ok := rule.(map[string]interface{})
			if !ok {
				continue
			}
			toPorts, _, _ := unstructured.NestedSlice(rm, "toPorts")
			for _, tp := range toPorts {
				tpm, ok := tp.(map[string]interface{})
				if !ok {
					continue
				}
				if ports, ok := tpm["ports"].([]interface{}); ok && len(ports) > 0 {
					counts.l4PortRules += len(ports)
				}
				rules, _, _ := unstructured.NestedMap(tpm, "rules")
				if len(rules) > 0 {
					counts.l7Rules++
				}
			}
		}
	}
	return counts
}

// ciliumEndpointSelectorStr returns a compact label string from a CiliumNetworkPolicy endpointSelector.
func ciliumEndpointSelectorStr(obj map[string]interface{}) string {
	labels, _, _ := unstructured.NestedStringMap(obj, "spec", "endpointSelector", "matchLabels")
	return formatSelector(labels)
}

// --- list_cilium_policies ---

type ListCiliumPoliciesTool struct{ BaseTool }

func (t *ListCiliumPoliciesTool) Name() string { return "list_cilium_policies" }
func (t *ListCiliumPoliciesTool) Description() string {
	return "List Cilium NetworkPolicies and CiliumClusterwideNetworkPolicies with L3/L4/L7 rule counts and endpoint selector labels"
}
func (t *ListCiliumPoliciesTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Namespace (empty for all namespaces)",
			},
		},
	}
}

func (t *ListCiliumPoliciesTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	ns := getStringArg(args, "namespace", "")
	findings := make([]types.DiagnosticFinding, 0, 10)

	// CiliumNetworkPolicies
	var cnpList *unstructured.UnstructuredList
	var err error
	if ns == "" {
		cnpList, err = t.Clients.Dynamic.Resource(ciliumNPGVR).List(ctx, metav1.ListOptions{})
	} else {
		cnpList, err = t.Clients.Dynamic.Resource(ciliumNPGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	}
	if err == nil {
		for _, item := range cnpList.Items {
			ingress, _, _ := unstructured.NestedSlice(item.Object, "spec", "ingress")
			egress, _, _ := unstructured.NestedSlice(item.Object, "spec", "egress")
			counts := countCiliumRules(ingress, egress)
			selectorStr := ciliumEndpointSelectorStr(item.Object)

			summary := fmt.Sprintf("CiliumNetworkPolicy %s/%s endpointSelector={%s} ingress=%d egress=%d l4Ports=%d l7=%d",
				item.GetNamespace(), item.GetName(), selectorStr,
				counts.ingressRules, counts.egressRules, counts.l4PortRules, counts.l7Rules)

			findings = append(findings, types.DiagnosticFinding{
				Severity: types.SeverityInfo,
				Category: types.CategoryPolicy,
				Resource: &types.ResourceRef{
					Kind:       "CiliumNetworkPolicy",
					Namespace:  item.GetNamespace(),
					Name:       item.GetName(),
					APIVersion: "cilium.io/v2",
				},
				Summary: summary,
			})
		}
	}

	// CiliumClusterwideNetworkPolicies
	ccnpList, ccnpErr := t.Clients.Dynamic.Resource(ciliumCNPGVR).List(ctx, metav1.ListOptions{})
	if ccnpErr == nil {
		for _, item := range ccnpList.Items {
			ingress, _, _ := unstructured.NestedSlice(item.Object, "spec", "ingress")
			egress, _, _ := unstructured.NestedSlice(item.Object, "spec", "egress")
			counts := countCiliumRules(ingress, egress)
			selectorStr := ciliumEndpointSelectorStr(item.Object)

			summary := fmt.Sprintf("CiliumClusterwideNetworkPolicy %s endpointSelector={%s} ingress=%d egress=%d l4Ports=%d l7=%d",
				item.GetName(), selectorStr,
				counts.ingressRules, counts.egressRules, counts.l4PortRules, counts.l7Rules)

			findings = append(findings, types.DiagnosticFinding{
				Severity: types.SeverityInfo,
				Category: types.CategoryPolicy,
				Resource: &types.ResourceRef{
					Kind:       "CiliumClusterwideNetworkPolicy",
					Name:       item.GetName(),
					APIVersion: "cilium.io/v2",
				},
				Summary: summary,
			})
		}
	}

	if len(findings) == 0 {
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryPolicy,
			Summary:  "No Cilium network policies found",
		})
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, "cilium"), nil
}

// --- get_cilium_policy ---

type GetCiliumPolicyTool struct{ BaseTool }

func (t *GetCiliumPolicyTool) Name() string { return "get_cilium_policy" }
func (t *GetCiliumPolicyTool) Description() string {
	return "Get detailed CiliumNetworkPolicy view with L7 HTTP/gRPC/Kafka rules, L4 port rules, endpoint selector, and affected services"
}
func (t *GetCiliumPolicyTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "CiliumNetworkPolicy name",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace",
			},
		},
		"required": []string{"name", "namespace"},
	}
}

func (t *GetCiliumPolicyTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	name := getStringArg(args, "name", "")
	ns := getStringArg(args, "namespace", "default")

	policy, err := t.Clients.Dynamic.Resource(ciliumNPGVR).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get CiliumNetworkPolicy %s/%s: %w", ns, name, err)
	}

	ref := &types.ResourceRef{
		Kind:       "CiliumNetworkPolicy",
		Namespace:  ns,
		Name:       name,
		APIVersion: "cilium.io/v2",
	}
	findings := make([]types.DiagnosticFinding, 0, 8)

	// --- Overview finding: endpoint selector + rule counts ---
	selectorLabels, _, _ := unstructured.NestedStringMap(policy.Object, "spec", "endpointSelector", "matchLabels")
	selectorStr := formatSelector(selectorLabels)

	ingress, _, _ := unstructured.NestedSlice(policy.Object, "spec", "ingress")
	egress, _, _ := unstructured.NestedSlice(policy.Object, "spec", "egress")
	counts := countCiliumRules(ingress, egress)

	findings = append(findings, types.DiagnosticFinding{
		Severity: types.SeverityInfo,
		Category: types.CategoryPolicy,
		Resource: ref,
		Summary: fmt.Sprintf("%s/%s endpointSelector={%s} ingress=%d egress=%d l4Ports=%d l7=%d",
			ns, name, selectorStr, counts.ingressRules, counts.egressRules, counts.l4PortRules, counts.l7Rules),
		Detail: fmt.Sprintf("endpointSelector.matchLabels=%v", selectorLabels),
	})

	// --- Walk ingress rules ---
	for i, rule := range ingress {
		rm, ok := rule.(map[string]interface{})
		if !ok {
			continue
		}
		ruleFindings := describeCiliumRule(rm, "ingress", i, ref)
		findings = append(findings, ruleFindings...)
	}

	// --- Walk egress rules ---
	for i, rule := range egress {
		rm, ok := rule.(map[string]interface{})
		if !ok {
			continue
		}
		ruleFindings := describeCiliumRule(rm, "egress", i, ref)
		findings = append(findings, ruleFindings...)
	}

	// --- Cross-reference: find services whose pod selectors overlap ---
	if len(selectorLabels) > 0 {
		affectedSvcs := findServicesMatchingLabels(ctx, t, ns, selectorLabels)
		if len(affectedSvcs) > 0 {
			findings = append(findings, types.DiagnosticFinding{
				Severity: types.SeverityInfo,
				Category: types.CategoryPolicy,
				Resource: ref,
				Summary:  fmt.Sprintf("Affected services: %s", strings.Join(affectedSvcs, ", ")),
				Detail:   "Services whose pod selector labels overlap with the policy endpointSelector",
			})
		}
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, "cilium"), nil
}

// describeCiliumRule returns DiagnosticFindings for a single ingress/egress rule entry.
func describeCiliumRule(rule map[string]interface{}, direction string, index int, ref *types.ResourceRef) []types.DiagnosticFinding {
	findings := make([]types.DiagnosticFinding, 0, 4)

	// --- L3: endpoint selectors (fromEndpoints / toEndpoints) ---
	peerKey := "fromEndpoints"
	if direction == "egress" {
		peerKey = "toEndpoints"
	}
	if endpoints, ok := rule[peerKey].([]interface{}); ok && len(endpoints) > 0 {
		epDescs := make([]string, 0, len(endpoints))
		for _, ep := range endpoints {
			if epm, ok := ep.(map[string]interface{}); ok {
				if ml, ok := epm["matchLabels"].(map[string]interface{}); ok && len(ml) > 0 {
					parts := make([]string, 0, len(ml))
					for k, v := range ml {
						parts = append(parts, fmt.Sprintf("%s=%v", k, v))
					}
					epDescs = append(epDescs, "{"+strings.Join(parts, ",")+"}")
				} else {
					epDescs = append(epDescs, "{}")
				}
			}
		}
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryPolicy,
			Resource: ref,
			Summary:  fmt.Sprintf("%s rule[%d] L3 %s: %s", direction, index, peerKey, strings.Join(epDescs, "; ")),
		})
	}

	// --- L4/L7: toPorts ---
	toPorts, _, _ := unstructured.NestedSlice(rule, "toPorts")
	for pi, tp := range toPorts {
		tpm, ok := tp.(map[string]interface{})
		if !ok {
			continue
		}

		// Collect port/protocol entries
		portDescs := make([]string, 0)
		if ports, ok := tpm["ports"].([]interface{}); ok {
			for _, p := range ports {
				if pm, ok := p.(map[string]interface{}); ok {
					proto := fmt.Sprintf("%v", pm["protocol"])
					port := fmt.Sprintf("%v", pm["port"])
					portDescs = append(portDescs, fmt.Sprintf("%s/%s", port, proto))
				}
			}
		}

		portStr := strings.Join(portDescs, ",")
		if portStr == "" {
			portStr = "any"
		}

		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryPolicy,
			Resource: ref,
			Summary:  fmt.Sprintf("%s rule[%d] L4 toPorts[%d]: ports=[%s]", direction, index, pi, portStr),
		})

		// L7 rules (conditional enrichment — only when L7 rules exist)
		l7rules, _, _ := unstructured.NestedMap(tpm, "rules")
		if len(l7rules) == 0 {
			continue
		}

		// HTTP rules
		if httpRules, ok := l7rules["http"].([]interface{}); ok && len(httpRules) > 0 {
			for hi, hr := range httpRules {
				hrm, ok := hr.(map[string]interface{})
				if !ok {
					continue
				}
				method, _ := hrm["method"].(string)
				path, _ := hrm["path"].(string)

				summary := fmt.Sprintf("%s rule[%d] L7 HTTP[%d]: method=%s path=%s",
					direction, index, hi, orAny(method), orAny(path))

				severity := types.SeverityInfo
				suggestion := ""
				if method != "" || path != "" {
					severity = types.SeverityWarning
					suggestion = fmt.Sprintf("L7 HTTP rule restricts traffic to method=%s path=%s; ensure clients comply or requests will be dropped.", orAny(method), orAny(path))
				}

				findings = append(findings, types.DiagnosticFinding{
					Severity:   severity,
					Category:   types.CategoryPolicy,
					Resource:   ref,
					Summary:    summary,
					Suggestion: suggestion,
				})
			}
		}

		// gRPC rules
		if grpcRules, ok := l7rules["grpc"].([]interface{}); ok && len(grpcRules) > 0 {
			for gi, gr := range grpcRules {
				grm, ok := gr.(map[string]interface{})
				if !ok {
					continue
				}
				svc, _ := grm["service"].(string)
				method, _ := grm["method"].(string)

				summary := fmt.Sprintf("%s rule[%d] L7 gRPC[%d]: service=%s method=%s",
					direction, index, gi, orAny(svc), orAny(method))

				severity := types.SeverityInfo
				suggestion := ""
				if svc != "" || method != "" {
					severity = types.SeverityWarning
					suggestion = fmt.Sprintf("L7 gRPC rule restricts traffic to service=%s method=%s; ensure clients comply or requests will be dropped.", orAny(svc), orAny(method))
				}

				findings = append(findings, types.DiagnosticFinding{
					Severity:   severity,
					Category:   types.CategoryPolicy,
					Resource:   ref,
					Summary:    summary,
					Suggestion: suggestion,
				})
			}
		}

		// Kafka rules
		if kafkaRules, ok := l7rules["kafka"].([]interface{}); ok && len(kafkaRules) > 0 {
			for ki, kr := range kafkaRules {
				krm, ok := kr.(map[string]interface{})
				if !ok {
					continue
				}
				topic, _ := krm["topic"].(string)
				role, _ := krm["role"].(string)

				summary := fmt.Sprintf("%s rule[%d] L7 Kafka[%d]: topic=%s role=%s",
					direction, index, ki, orAny(topic), orAny(role))

				severity := types.SeverityInfo
				suggestion := ""
				if topic != "" || role != "" {
					severity = types.SeverityWarning
					suggestion = fmt.Sprintf("L7 Kafka rule restricts traffic to topic=%s role=%s; ensure clients comply or requests will be dropped.", orAny(topic), orAny(role))
				}

				findings = append(findings, types.DiagnosticFinding{
					Severity:   severity,
					Category:   types.CategoryPolicy,
					Resource:   ref,
					Summary:    summary,
					Suggestion: suggestion,
				})
			}
		}
	}

	return findings
}

// findServicesMatchingLabels lists services in ns and returns names whose pod selector
// labels are a superset of (or equal to) the given label set.
func findServicesMatchingLabels(ctx context.Context, t *GetCiliumPolicyTool, ns string, policyLabels map[string]string) []string {
	svcList, err := t.Clients.Dynamic.Resource(servicesGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}

	matched := make([]string, 0)
	for _, svc := range svcList.Items {
		selector, _, _ := unstructured.NestedStringMap(svc.Object, "spec", "selector")
		if len(selector) == 0 {
			continue
		}
		// Check if every policy label exists in the service selector
		if labelsSubsetOf(policyLabels, selector) {
			matched = append(matched, fmt.Sprintf("%s/%s", svc.GetNamespace(), svc.GetName()))
		}
	}
	return matched
}

// labelsSubsetOf returns true if every key/value in sub is present in super.
func labelsSubsetOf(sub, super map[string]string) bool {
	for k, v := range sub {
		if sv, ok := super[k]; !ok || sv != v {
			return false
		}
	}
	return true
}

// orAny returns s if non-empty, otherwise "*".
func orAny(s string) string {
	if s == "" {
		return "*"
	}
	return s
}

// --- check_cilium_status ---

type CheckCiliumStatusTool struct{ BaseTool }

func (t *CheckCiliumStatusTool) Name() string { return "check_cilium_status" }
func (t *CheckCiliumStatusTool) Description() string {
	return "Check Cilium agent health, endpoint count, and basic connectivity status"
}
func (t *CheckCiliumStatusTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Namespace to check endpoints in (empty for all)",
			},
		},
	}
}

func (t *CheckCiliumStatusTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	ns := getStringArg(args, "namespace", "")
	findings := make([]types.DiagnosticFinding, 0, 5)

	// Check Cilium agent pods
	agentPods, err := t.Clients.Clientset.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{
		LabelSelector: "k8s-app=cilium",
	})
	if err != nil {
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityWarning,
			Category:   types.CategoryMesh,
			Summary:    "Could not check Cilium agent pods",
			Detail:     err.Error(),
			Suggestion: "Verify Cilium is installed in the kube-system namespace.",
		})
	} else {
		total := len(agentPods.Items)
		ready := 0
		nodeNames := make([]string, 0, total)
		for _, pod := range agentPods.Items {
			isReady := true
			for _, cs := range pod.Status.ContainerStatuses {
				if !cs.Ready {
					isReady = false
				}
			}
			if isReady {
				ready++
			}
			nodeNames = append(nodeNames, pod.Spec.NodeName)
		}
		severity := types.SeverityOK
		if ready < total {
			severity = types.SeverityWarning
		}
		if ready == 0 {
			severity = types.SeverityCritical
		}
		findings = append(findings, types.DiagnosticFinding{
			Severity: severity,
			Category: types.CategoryMesh,
			Summary:  fmt.Sprintf("Cilium agents: %d/%d ready", ready, total),
			Detail:   fmt.Sprintf("nodes=%s", strings.Join(nodeNames, ", ")),
		})
	}

	// Count Cilium endpoints
	if ns == "" {
		epList, err := t.Clients.Dynamic.Resource(ciliumEPGVR).List(ctx, metav1.ListOptions{})
		if err == nil {
			findings = append(findings, types.DiagnosticFinding{
				Severity: types.SeverityInfo,
				Category: types.CategoryMesh,
				Summary:  fmt.Sprintf("Cilium endpoints: %d cluster-wide", len(epList.Items)),
			})
		}
	} else {
		epList, err := t.Clients.Dynamic.Resource(ciliumEPGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
		if err == nil {
			findings = append(findings, types.DiagnosticFinding{
				Severity: types.SeverityInfo,
				Category: types.CategoryMesh,
				Summary:  fmt.Sprintf("Cilium endpoints in %s: %d", ns, len(epList.Items)),
			})
		}
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, "cilium"), nil
}
