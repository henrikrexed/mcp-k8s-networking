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
	kumaMeshGVR      = schema.GroupVersionResource{Group: "kuma.io", Version: "v1alpha1", Resource: "meshes"}
	kumaDataplaneGVR = schema.GroupVersionResource{Group: "kuma.io", Version: "v1alpha1", Resource: "dataplanes"}
)

// --- check_kuma_status ---

type CheckKumaStatusTool struct{ BaseTool }

func (t *CheckKumaStatusTool) Name() string { return "check_kuma_status" }
func (t *CheckKumaStatusTool) Description() string {
	return "Check Kuma service mesh status including control plane health, mesh count, and data plane proxy status"
}
func (t *CheckKumaStatusTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Namespace to check (empty for cluster-wide)",
			},
		},
	}
}

func (t *CheckKumaStatusTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	ns := getStringArg(args, "namespace", "")
	findings := make([]types.DiagnosticFinding, 0, 5)

	// Check control plane pods
	cpPods, err := t.Clients.Clientset.CoreV1().Pods("kuma-system").List(ctx, metav1.ListOptions{
		LabelSelector: "app=kuma-control-plane",
	})
	if err != nil {
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityWarning,
			Category:   types.CategoryMesh,
			Summary:    "Could not check Kuma control plane pods",
			Detail:     err.Error(),
			Suggestion: "Verify Kuma is installed in the kuma-system namespace.",
		})
	} else {
		ready := 0
		for _, pod := range cpPods.Items {
			for _, cs := range pod.Status.ContainerStatuses {
				if cs.Ready {
					ready++
				}
			}
		}
		severity := types.SeverityOK
		if ready == 0 {
			severity = types.SeverityCritical
		}
		findings = append(findings, types.DiagnosticFinding{
			Severity: severity,
			Category: types.CategoryMesh,
			Resource: &types.ResourceRef{Kind: "Deployment", Namespace: "kuma-system", Name: "kuma-control-plane"},
			Summary:  fmt.Sprintf("Kuma control plane: %d/%d pods ready", ready, len(cpPods.Items)),
		})
	}

	// Count meshes
	meshes, err := t.Clients.Dynamic.Resource(kumaMeshGVR).List(ctx, metav1.ListOptions{})
	if err == nil {
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryMesh,
			Summary:  fmt.Sprintf("Kuma meshes: %d", len(meshes.Items)),
			Detail:   meshNames(meshes),
		})
	}

	// Count dataplanes
	var dataplanes *unstructured.UnstructuredList
	if ns == "" {
		dataplanes, err = t.Clients.Dynamic.Resource(kumaDataplaneGVR).List(ctx, metav1.ListOptions{})
	} else {
		dataplanes, err = t.Clients.Dynamic.Resource(kumaDataplaneGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	}
	if err == nil {
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryMesh,
			Summary:  fmt.Sprintf("Kuma data plane proxies: %d", len(dataplanes.Items)),
		})
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, "kuma"), nil
}

func meshNames(list *unstructured.UnstructuredList) string {
	names := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		names = append(names, item.GetName())
	}
	return strings.Join(names, ", ")
}
