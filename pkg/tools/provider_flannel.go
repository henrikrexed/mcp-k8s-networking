package tools

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/isitobservable/k8s-networking-mcp/pkg/types"
)

// --- check_flannel_status ---

type CheckFlannelStatusTool struct{ BaseTool }

func (t *CheckFlannelStatusTool) Name() string { return "check_flannel_status" }
func (t *CheckFlannelStatusTool) Description() string {
	return "Check Flannel CNI installation health including DaemonSet status and pod health across nodes"
}
func (t *CheckFlannelStatusTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *CheckFlannelStatusTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	findings := make([]types.DiagnosticFinding, 0, 3)

	// Check kube-flannel DaemonSet pods
	// Flannel typically runs in kube-flannel or kube-system namespace
	type podCounts struct{ Ready, Total int }
	var flannelPods *podCounts

	for _, nsCandidate := range []string{"kube-flannel", "kube-system"} {
		pods, err := t.Clients.Clientset.CoreV1().Pods(nsCandidate).List(ctx, metav1.ListOptions{
			LabelSelector: "app=flannel",
		})
		if err == nil && len(pods.Items) > 0 {
			total := len(pods.Items)
			ready := 0
			nodeNames := make([]string, 0, total)
			for _, pod := range pods.Items {
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
				Category: types.CategoryConnectivity,
				Resource: &types.ResourceRef{Kind: "DaemonSet", Namespace: nsCandidate, Name: "kube-flannel-ds"},
				Summary:  fmt.Sprintf("Flannel pods: %d/%d ready in %s", ready, total, nsCandidate),
				Detail:   fmt.Sprintf("nodes=%s", strings.Join(nodeNames, ", ")),
			})

			flannelPods = &podCounts{Ready: ready, Total: total}
			break
		}
	}

	if flannelPods == nil {
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityWarning,
			Category:   types.CategoryConnectivity,
			Summary:    "Flannel DaemonSet not found",
			Suggestion: "Check if Flannel is installed (look for kube-flannel-ds DaemonSet in kube-flannel or kube-system namespace).",
		})
	}

	// Check for Flannel ConfigMap
	for _, nsCandidate := range []string{"kube-flannel", "kube-system"} {
		cm, err := t.Clients.Clientset.CoreV1().ConfigMaps(nsCandidate).Get(ctx, "kube-flannel-cfg", metav1.GetOptions{})
		if err == nil {
			netConf := cm.Data["net-conf.json"]
			if netConf != "" {
				findings = append(findings, types.DiagnosticFinding{
					Severity: types.SeverityInfo,
					Category: types.CategoryConnectivity,
					Resource: &types.ResourceRef{Kind: "ConfigMap", Namespace: nsCandidate, Name: "kube-flannel-cfg"},
					Summary:  "Flannel configuration found",
					Detail:   fmt.Sprintf("net-conf.json=%s", netConf),
				})
			}
			break
		}
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, "", "flannel"), nil
}
