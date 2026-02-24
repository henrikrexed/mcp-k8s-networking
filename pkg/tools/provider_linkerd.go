package tools

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/isitobservable/k8s-networking-mcp/pkg/types"
)

var (
	linkerdSPGVR = schema.GroupVersionResource{Group: "linkerd.io", Version: "v1alpha2", Resource: "serviceprofiles"}
)

// --- check_linkerd_status ---

type CheckLinkerdStatusTool struct{ BaseTool }

func (t *CheckLinkerdStatusTool) Name() string { return "check_linkerd_status" }
func (t *CheckLinkerdStatusTool) Description() string {
	return "Check Linkerd service mesh status including control plane health, proxy injection status, and service profile count"
}
func (t *CheckLinkerdStatusTool) InputSchema() map[string]interface{} {
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

func (t *CheckLinkerdStatusTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	ns := getStringArg(args, "namespace", "")
	findings := make([]types.DiagnosticFinding, 0, 5)

	// Check control plane pods
	cpPods, err := t.Clients.Clientset.CoreV1().Pods("linkerd").List(ctx, metav1.ListOptions{
		LabelSelector: "linkerd.io/control-plane-component",
	})
	if err != nil {
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityWarning,
			Category:   types.CategoryMesh,
			Summary:    "Could not check Linkerd control plane pods",
			Detail:     err.Error(),
			Suggestion: "Verify Linkerd is installed in the linkerd namespace.",
		})
	} else {
		total := len(cpPods.Items)
		ready := 0
		components := make(map[string]bool)
		for _, pod := range cpPods.Items {
			component := pod.Labels["linkerd.io/control-plane-component"]
			isReady := true
			for _, cs := range pod.Status.ContainerStatuses {
				if !cs.Ready {
					isReady = false
				}
			}
			if isReady {
				ready++
			}
			components[component] = isReady
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
			Resource: &types.ResourceRef{Kind: "Namespace", Name: "linkerd"},
			Summary:  fmt.Sprintf("Linkerd control plane: %d/%d pods ready", ready, total),
			Detail:   fmt.Sprintf("components=%v", components),
		})
	}

	// Check proxy injection across namespaces
	namespaces, err := t.Clients.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err == nil {
		injected := 0
		for _, nsObj := range namespaces.Items {
			if nsObj.Annotations["linkerd.io/inject"] == "enabled" {
				injected++
			}
		}
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryMesh,
			Summary:  fmt.Sprintf("Linkerd proxy injection enabled in %d namespace(s)", injected),
		})
	}

	// Count service profiles
	if ns == "" {
		profiles, err := t.Clients.Dynamic.Resource(linkerdSPGVR).List(ctx, metav1.ListOptions{})
		if err == nil {
			findings = append(findings, types.DiagnosticFinding{
				Severity: types.SeverityInfo,
				Category: types.CategoryMesh,
				Summary:  fmt.Sprintf("Linkerd service profiles: %d", len(profiles.Items)),
			})
		}
	} else {
		profiles, err := t.Clients.Dynamic.Resource(linkerdSPGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
		if err == nil {
			findings = append(findings, types.DiagnosticFinding{
				Severity: types.SeverityInfo,
				Category: types.CategoryMesh,
				Summary:  fmt.Sprintf("Linkerd service profiles in %s: %d", ns, len(profiles.Items)),
			})
		}
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, "linkerd"), nil
}
