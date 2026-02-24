package tools

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/isitobservable/k8s-networking-mcp/pkg/types"
)

var (
	ciliumNPGVR  = schema.GroupVersionResource{Group: "cilium.io", Version: "v2", Resource: "ciliumnetworkpolicies"}
	ciliumCNPGVR = schema.GroupVersionResource{Group: "cilium.io", Version: "v2", Resource: "ciliumclusterwidenetworkpolicies"}
	ciliumEPGVR  = schema.GroupVersionResource{Group: "cilium.io", Version: "v2", Resource: "ciliumendpoints"}
)

// --- list_cilium_policies ---

type ListCiliumPoliciesTool struct{ BaseTool }

func (t *ListCiliumPoliciesTool) Name() string { return "list_cilium_policies" }
func (t *ListCiliumPoliciesTool) Description() string {
	return "List Cilium NetworkPolicies and CiliumClusterwideNetworkPolicies"
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
	var cnpList interface{}
	var err error
	if ns == "" {
		list, e := t.Clients.Dynamic.Resource(ciliumNPGVR).List(ctx, metav1.ListOptions{})
		err = e
		cnpList = list
		if e == nil {
			for _, item := range list.Items {
				findings = append(findings, types.DiagnosticFinding{
					Severity: types.SeverityInfo,
					Category: types.CategoryPolicy,
					Resource: &types.ResourceRef{Kind: "CiliumNetworkPolicy", Namespace: item.GetNamespace(), Name: item.GetName()},
					Summary:  fmt.Sprintf("CiliumNetworkPolicy %s/%s", item.GetNamespace(), item.GetName()),
				})
			}
		}
	} else {
		list, e := t.Clients.Dynamic.Resource(ciliumNPGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
		err = e
		cnpList = list
		if e == nil {
			for _, item := range list.Items {
				findings = append(findings, types.DiagnosticFinding{
					Severity: types.SeverityInfo,
					Category: types.CategoryPolicy,
					Resource: &types.ResourceRef{Kind: "CiliumNetworkPolicy", Namespace: item.GetNamespace(), Name: item.GetName()},
					Summary:  fmt.Sprintf("CiliumNetworkPolicy %s/%s", item.GetNamespace(), item.GetName()),
				})
			}
		}
	}
	_ = cnpList
	_ = err

	// CiliumClusterwideNetworkPolicies
	ccnpList, ccnpErr := t.Clients.Dynamic.Resource(ciliumCNPGVR).List(ctx, metav1.ListOptions{})
	if ccnpErr == nil {
		for _, item := range ccnpList.Items {
			findings = append(findings, types.DiagnosticFinding{
				Severity: types.SeverityInfo,
				Category: types.CategoryPolicy,
				Resource: &types.ResourceRef{Kind: "CiliumClusterwideNetworkPolicy", Name: item.GetName()},
				Summary:  fmt.Sprintf("CiliumClusterwideNetworkPolicy %s", item.GetName()),
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
	var endpoints interface{}
	if ns == "" {
		epList, e := t.Clients.Dynamic.Resource(ciliumEPGVR).List(ctx, metav1.ListOptions{})
		if e == nil {
			findings = append(findings, types.DiagnosticFinding{
				Severity: types.SeverityInfo,
				Category: types.CategoryMesh,
				Summary:  fmt.Sprintf("Cilium endpoints: %d cluster-wide", len(epList.Items)),
			})
		}
		endpoints = epList
	} else {
		epList, e := t.Clients.Dynamic.Resource(ciliumEPGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
		if e == nil {
			findings = append(findings, types.DiagnosticFinding{
				Severity: types.SeverityInfo,
				Category: types.CategoryMesh,
				Summary:  fmt.Sprintf("Cilium endpoints in %s: %d", ns, len(epList.Items)),
			})
		}
		endpoints = epList
	}
	_ = endpoints

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, "cilium"), nil
}
