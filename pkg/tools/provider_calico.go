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
	calicoNPGVR  = schema.GroupVersionResource{Group: "crd.projectcalico.org", Version: "v1", Resource: "networkpolicies"}
	calicoGNPGVR = schema.GroupVersionResource{Group: "crd.projectcalico.org", Version: "v1", Resource: "globalnetworkpolicies"}
)

// --- list_calico_policies ---

type ListCalicoPoliciesTool struct{ BaseTool }

func (t *ListCalicoPoliciesTool) Name() string { return "list_calico_policies" }
func (t *ListCalicoPoliciesTool) Description() string {
	return "List Calico NetworkPolicies and GlobalNetworkPolicies"
}
func (t *ListCalicoPoliciesTool) InputSchema() map[string]interface{} {
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

func (t *ListCalicoPoliciesTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	ns := getStringArg(args, "namespace", "")
	findings := make([]types.DiagnosticFinding, 0, 10)

	// Calico NetworkPolicies
	if ns == "" {
		list, err := t.Clients.Dynamic.Resource(calicoNPGVR).List(ctx, metav1.ListOptions{})
		if err == nil {
			for _, item := range list.Items {
				findings = append(findings, types.DiagnosticFinding{
					Severity: types.SeverityInfo,
					Category: types.CategoryPolicy,
					Resource: &types.ResourceRef{Kind: "CalicoNetworkPolicy", Namespace: item.GetNamespace(), Name: item.GetName(), APIVersion: "crd.projectcalico.org/v1"},
					Summary:  fmt.Sprintf("Calico NetworkPolicy %s/%s", item.GetNamespace(), item.GetName()),
				})
			}
		}
	} else {
		list, err := t.Clients.Dynamic.Resource(calicoNPGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
		if err == nil {
			for _, item := range list.Items {
				findings = append(findings, types.DiagnosticFinding{
					Severity: types.SeverityInfo,
					Category: types.CategoryPolicy,
					Resource: &types.ResourceRef{Kind: "CalicoNetworkPolicy", Namespace: item.GetNamespace(), Name: item.GetName(), APIVersion: "crd.projectcalico.org/v1"},
					Summary:  fmt.Sprintf("Calico NetworkPolicy %s/%s", item.GetNamespace(), item.GetName()),
				})
			}
		}
	}

	// GlobalNetworkPolicies
	gnpList, err := t.Clients.Dynamic.Resource(calicoGNPGVR).List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, item := range gnpList.Items {
			findings = append(findings, types.DiagnosticFinding{
				Severity: types.SeverityInfo,
				Category: types.CategoryPolicy,
				Resource: &types.ResourceRef{Kind: "GlobalNetworkPolicy", Name: item.GetName(), APIVersion: "crd.projectcalico.org/v1"},
				Summary:  fmt.Sprintf("Calico GlobalNetworkPolicy %s", item.GetName()),
			})
		}
	}

	if len(findings) == 0 {
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryPolicy,
			Summary:  "No Calico network policies found",
		})
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, "calico"), nil
}

// --- check_calico_status ---

type CheckCalicoStatusTool struct{ BaseTool }

func (t *CheckCalicoStatusTool) Name() string { return "check_calico_status" }
func (t *CheckCalicoStatusTool) Description() string {
	return "Check Calico node health and felix status"
}
func (t *CheckCalicoStatusTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *CheckCalicoStatusTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	findings := make([]types.DiagnosticFinding, 0, 5)

	// Check calico-node DaemonSet pods
	calicoNodes, err := t.Clients.Clientset.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{
		LabelSelector: "k8s-app=calico-node",
	})
	if err != nil {
		// Try calico-system namespace
		calicoNodes, err = t.Clients.Clientset.CoreV1().Pods("calico-system").List(ctx, metav1.ListOptions{
			LabelSelector: "k8s-app=calico-node",
		})
	}

	if err != nil {
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityWarning,
			Category:   types.CategoryMesh,
			Summary:    "Could not check Calico node pods",
			Detail:     err.Error(),
			Suggestion: "Verify Calico is installed (check kube-system or calico-system namespace).",
		})
	} else {
		total := len(calicoNodes.Items)
		ready := 0
		nodeNames := make([]string, 0, total)
		for _, pod := range calicoNodes.Items {
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
		if ready == 0 && total > 0 {
			severity = types.SeverityCritical
		}
		findings = append(findings, types.DiagnosticFinding{
			Severity: severity,
			Category: types.CategoryMesh,
			Summary:  fmt.Sprintf("Calico nodes: %d/%d ready", ready, total),
			Detail:   fmt.Sprintf("nodes=%s", strings.Join(nodeNames, ", ")),
		})
	}

	// Check calico-kube-controllers
	controllers, err := t.Clients.Clientset.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{
		LabelSelector: "k8s-app=calico-kube-controllers",
	})
	if err != nil {
		controllers, err = t.Clients.Clientset.CoreV1().Pods("calico-system").List(ctx, metav1.ListOptions{
			LabelSelector: "k8s-app=calico-kube-controllers",
		})
	}
	if err == nil {
		ready := 0
		for _, pod := range controllers.Items {
			for _, cs := range pod.Status.ContainerStatuses {
				if cs.Ready {
					ready++
				}
			}
		}
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryMesh,
			Summary:  fmt.Sprintf("Calico kube-controllers: %d/%d ready", ready, len(controllers.Items)),
		})
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, "", "calico"), nil
}
