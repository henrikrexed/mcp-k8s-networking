package tools

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/isitobservable/k8s-networking-mcp/pkg/types"
)

// sidecarContainerNames are the well-known mesh sidecar container names injected into workload pods.
// Note: cilium-agent runs as a DaemonSet pod, not as a sidecar, so it is not included here.
var sidecarContainerNames = map[string]bool{
	"istio-proxy":   true,
	"linkerd-proxy": true,
}

// initContainerNames are the well-known mesh init container names to check for failures.
var meshInitContainerNames = map[string]bool{
	"istio-init":   true,
	"linkerd-init": true,
}

// --- check_dataplane_health ---

type CheckDataplaneHealthTool struct{ BaseTool }

func (t *CheckDataplaneHealthTool) Name() string { return "check_dataplane_health" }
func (t *CheckDataplaneHealthTool) Description() string {
	return "Check data plane health for all pods in a namespace by inspecting mesh sidecar containers (istio-proxy, cilium-agent, linkerd-proxy). Reports sidecar readiness, restart counts, init container failures, and Istio version skew without requiring exec access."
}
func (t *CheckDataplaneHealthTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Namespace to inspect (required)",
			},
		},
		"required": []string{"namespace"},
	}
}

func (t *CheckDataplaneHealthTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	ns := getStringArg(args, "namespace", "")
	if ns == "" {
		return nil, &types.MCPError{
			Code:    types.ErrCodeInvalidInput,
			Tool:    t.Name(),
			Message: "namespace is required",
		}
	}

	pods, err := t.Clients.Clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods in namespace %s: %w", ns, err)
	}

	// Fetch the istiod control plane version once (best-effort).
	istiodTag := t.fetchIstiodImageTag(ctx)

	findings := make([]types.DiagnosticFinding, 0, len(pods.Items))

	for i := range pods.Items {
		pod := &pods.Items[i]
		podRef := &types.ResourceRef{
			Kind:      "Pod",
			Namespace: pod.Namespace,
			Name:      pod.Name,
		}

		// Build a map of container statuses by name for quick lookup.
		csMap := make(map[string]struct {
			ready        bool
			restartCount int32
		})
		for _, cs := range pod.Status.ContainerStatuses {
			csMap[cs.Name] = struct {
				ready        bool
				restartCount int32
			}{ready: cs.Ready, restartCount: cs.RestartCount}
		}
		// Also include init container statuses (for native sidecars and mesh init containers).
		type initCSInfo struct {
			ready        bool
			restartCount int32
			exitCode     int32
			reason       string
			terminated   bool
		}
		initCSMap := make(map[string]initCSInfo)
		for _, ics := range pod.Status.InitContainerStatuses {
			info := initCSInfo{
				ready:        ics.Ready,
				restartCount: ics.RestartCount,
			}
			if ics.State.Terminated != nil {
				info.terminated = true
				info.exitCode = ics.State.Terminated.ExitCode
				info.reason = ics.State.Terminated.Reason
			}
			initCSMap[ics.Name] = info
		}

		// Collect all containers (regular + init) to find sidecars.
		// K8s 1.28+ native sidecars appear in InitContainers with restartPolicy=Always,
		// but we detect them by name in both container lists.
		foundSidecar := false

		// Check regular containers.
		for _, c := range pod.Spec.Containers {
			if !sidecarContainerNames[c.Name] {
				continue
			}
			foundSidecar = true
			cs, hasStatus := csMap[c.Name]

			// Sidecar not ready.
			if hasStatus && !cs.ready {
				findings = append(findings, types.DiagnosticFinding{
					Severity:   types.SeverityWarning,
					Category:   types.CategoryMesh,
					Resource:   podRef,
					Summary:    fmt.Sprintf("Sidecar %q is not ready in pod %s/%s", c.Name, pod.Namespace, pod.Name),
					Detail:     fmt.Sprintf("container=%s ready=false", c.Name),
					Suggestion: "Check the sidecar container logs and ensure the control plane is reachable.",
				})
			}

			// Restart count warning.
			if hasStatus && cs.restartCount > 0 {
				findings = append(findings, types.DiagnosticFinding{
					Severity:   types.SeverityWarning,
					Category:   types.CategoryMesh,
					Resource:   podRef,
					Summary:    fmt.Sprintf("Sidecar %q has restarted %d time(s) in pod %s/%s", c.Name, cs.restartCount, pod.Namespace, pod.Name),
					Detail:     fmt.Sprintf("container=%s restartCount=%d", c.Name, cs.restartCount),
					Suggestion: "Investigate sidecar crash logs for OOM or configuration errors.",
				})
			}

			// Istio version skew check.
			if c.Name == "istio-proxy" && istiodTag != "" {
				proxyTag := imageTag(c.Image)
				if proxyTag != "" && proxyTag != istiodTag {
					findings = append(findings, types.DiagnosticFinding{
						Severity:   types.SeverityWarning,
						Category:   types.CategoryMesh,
						Resource:   podRef,
						Summary:    fmt.Sprintf("Istio version skew detected in pod %s/%s: proxy=%s, control-plane=%s", pod.Namespace, pod.Name, proxyTag, istiodTag),
						Detail:     fmt.Sprintf("proxy_image=%s istiod_tag=%s", c.Image, istiodTag),
						Suggestion: "Restart the pod to pick up the updated istio-proxy sidecar image matching the control plane version.",
					})
				}
			}
		}

		// Check init containers for sidecar presence (K8s 1.28+ native sidecars) and failures.
		for _, ic := range pod.Spec.InitContainers {
			if sidecarContainerNames[ic.Name] {
				foundSidecar = true
				// Native sidecar in init containers: check its status via map lookup.
				if ics, ok := initCSMap[ic.Name]; ok {
					if !ics.ready {
						findings = append(findings, types.DiagnosticFinding{
							Severity:   types.SeverityWarning,
							Category:   types.CategoryMesh,
							Resource:   podRef,
							Summary:    fmt.Sprintf("Native sidecar %q (init container) is not ready in pod %s/%s", ic.Name, pod.Namespace, pod.Name),
							Detail:     fmt.Sprintf("container=%s ready=false", ic.Name),
							Suggestion: "Check the sidecar container logs and ensure the control plane is reachable.",
						})
					}
					if ics.restartCount > 0 {
						findings = append(findings, types.DiagnosticFinding{
							Severity:   types.SeverityWarning,
							Category:   types.CategoryMesh,
							Resource:   podRef,
							Summary:    fmt.Sprintf("Native sidecar %q (init container) has restarted %d time(s) in pod %s/%s", ic.Name, ics.restartCount, pod.Namespace, pod.Name),
							Detail:     fmt.Sprintf("container=%s restartCount=%d", ic.Name, ics.restartCount),
							Suggestion: "Investigate sidecar crash logs for OOM or configuration errors.",
						})
					}
				}
			}

			// Check mesh init containers for non-zero exit codes.
			if meshInitContainerNames[ic.Name] {
				if status, ok := initCSMap[ic.Name]; ok && status.terminated && status.exitCode != 0 {
					findings = append(findings, types.DiagnosticFinding{
						Severity:   types.SeverityWarning,
						Category:   types.CategoryMesh,
						Resource:   podRef,
						Summary:    fmt.Sprintf("Init container %q exited with code %d in pod %s/%s", ic.Name, status.exitCode, pod.Namespace, pod.Name),
						Detail:     fmt.Sprintf("container=%s exitCode=%d reason=%s", ic.Name, status.exitCode, status.reason),
						Suggestion: "Check init container logs; the mesh init container may have failed to configure iptables rules.",
					})
				}
			}
		}

		// If no sidecar was detected, emit an info finding.
		if !foundSidecar {
			findings = append(findings, types.DiagnosticFinding{
				Severity: types.SeverityInfo,
				Category: types.CategoryMesh,
				Resource: podRef,
				Summary:  fmt.Sprintf("No mesh sidecar detected in pod %s/%s", pod.Namespace, pod.Name),
				Detail:   "No istio-proxy or linkerd-proxy sidecar container found.",
			})
		}
	}

	if len(pods.Items) == 0 {
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryMesh,
			Summary:  fmt.Sprintf("No pods found in namespace %s", ns),
		})
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, ""), nil
}

// fetchIstiodImageTag attempts to retrieve the istiod image tag from the istiod
// Deployment in the istio-system namespace. Returns an empty string on any error.
func (t *CheckDataplaneHealthTool) fetchIstiodImageTag(ctx context.Context) string {
	deploy, err := t.Clients.Clientset.AppsV1().Deployments("istio-system").Get(ctx, "istiod", metav1.GetOptions{})
	if err != nil {
		return ""
	}

	// Try the revision label first.
	if rev, ok := deploy.Labels["istio.io/rev"]; ok && rev != "" {
		return rev
	}

	// Fall back to extracting the tag from the istiod container image.
	for _, c := range deploy.Spec.Template.Spec.Containers {
		if strings.Contains(c.Image, "istiod") || strings.Contains(c.Image, "pilot") {
			if tag := imageTag(c.Image); tag != "" {
				return tag
			}
		}
	}
	return ""
}

// imageTag extracts the tag portion from a container image reference.
// Returns an empty string if no tag is present or the image uses a digest.
func imageTag(image string) string {
	// Strip digest if present.
	if idx := strings.Index(image, "@"); idx >= 0 {
		return ""
	}
	parts := strings.SplitN(image, ":", 2)
	if len(parts) == 2 && parts[1] != "" {
		return parts[1]
	}
	return ""
}
