package tools

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/isitobservable/k8s-networking-mcp/pkg/k8s"
)

var proxyContainerNames = []string{"istio-proxy", "envoy", "linkerd-proxy"}

var errorPatterns = regexp.MustCompile(`(?i)(error|warn|rate.?limit|429|503|connection refused|TLS|timeout|denied|RBAC|misconfigur|failed|upstream.?reset|no.?healthy|circuit.?break|overflow|rejected)`)

// --- get_proxy_logs ---

type GetProxyLogsTool struct{ BaseTool }

func (t *GetProxyLogsTool) Name() string        { return "get_proxy_logs" }
func (t *GetProxyLogsTool) Description() string  { return "Get logs from Envoy/proxy sidecars (auto-detects istio-proxy, envoy, linkerd-proxy containers)" }
func (t *GetProxyLogsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pod":       map[string]interface{}{"type": "string", "description": "Pod name"},
			"namespace": map[string]interface{}{"type": "string", "description": "Kubernetes namespace"},
			"container": map[string]interface{}{"type": "string", "description": "Container name (auto-detects proxy container if not specified)"},
			"tail":      map[string]interface{}{"type": "number", "description": "Number of lines from the end (default 100)"},
			"since":     map[string]interface{}{"type": "string", "description": "Duration to look back (e.g., 5m, 1h)"},
		},
		"required": []string{"pod", "namespace"},
	}
}

func (t *GetProxyLogsTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	podName := getStringArg(args, "pod", "")
	ns := getStringArg(args, "namespace", "default")
	container := getStringArg(args, "container", "")
	tail := getIntArg(args, "tail", 100)
	since := getStringArg(args, "since", "")

	if container == "" {
		pod, err := t.Clients.Clientset.CoreV1().Pods(ns).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get pod %s/%s: %w", ns, podName, err)
		}
		container = findProxyContainer(pod)
		if container == "" {
			return nil, fmt.Errorf("no proxy container found in pod %s/%s", ns, podName)
		}
	}

	logs, err := getPodLogs(ctx, t.Clients, ns, podName, container, int64(tail), since)
	if err != nil {
		return nil, err
	}

	return NewResponse(t.Cfg, t.Name(), map[string]interface{}{
		"pod":       podName,
		"namespace": ns,
		"container": container,
		"lineCount": len(strings.Split(strings.TrimSpace(logs), "\n")),
		"logs":      logs,
	}), nil
}

// --- get_gateway_logs ---

type GetGatewayLogsTool struct{ BaseTool }

func (t *GetGatewayLogsTool) Name() string        { return "get_gateway_logs" }
func (t *GetGatewayLogsTool) Description() string  { return "Get logs from Gateway controller pods and Gateway API provider pods" }
func (t *GetGatewayLogsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"gateway_name": map[string]interface{}{"type": "string", "description": "Gateway resource name (optional, discovers controller pods by labels)"},
			"namespace":    map[string]interface{}{"type": "string", "description": "Namespace to search in (default: all common namespaces)"},
			"tail":         map[string]interface{}{"type": "number", "description": "Number of lines from the end (default 100)"},
			"since":        map[string]interface{}{"type": "string", "description": "Duration to look back (e.g., 5m, 1h)"},
		},
	}
}

func (t *GetGatewayLogsTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	ns := getStringArg(args, "namespace", "")
	tail := getIntArg(args, "tail", 100)
	since := getStringArg(args, "since", "")

	// Search for gateway controller pods using known labels
	gatewayLabels := []struct {
		namespace string
		selector  string
		desc      string
	}{
		{"istio-system", "app=istiod", "Istio control plane"},
		{"istio-system", "istio=ingressgateway", "Istio ingress gateway"},
		{"envoy-gateway-system", "control-plane=envoy-gateway", "Envoy Gateway controller"},
		{"gateway-system", "app.kubernetes.io/name=gateway-api", "Gateway API controller"},
	}

	results := make([]map[string]interface{}, 0)

	for _, gl := range gatewayLabels {
		searchNs := gl.namespace
		if ns != "" {
			searchNs = ns
		}
		pods, err := t.Clients.Clientset.CoreV1().Pods(searchNs).List(ctx, metav1.ListOptions{
			LabelSelector: gl.selector,
		})
		if err != nil || len(pods.Items) == 0 {
			continue
		}

		for _, pod := range pods.Items {
			container := pod.Spec.Containers[0].Name
			logs, err := getPodLogs(ctx, t.Clients, pod.Namespace, pod.Name, container, int64(tail), since)
			if err != nil {
				continue
			}
			results = append(results, map[string]interface{}{
				"pod":         pod.Name,
				"namespace":   pod.Namespace,
				"container":   container,
				"description": gl.desc,
				"logs":        logs,
			})
		}
	}

	return NewResponse(t.Cfg, t.Name(), map[string]interface{}{
		"podCount": len(results),
		"results":  results,
	}), nil
}

// --- get_infra_logs ---

type GetInfraLogsTool struct{ BaseTool }

func (t *GetInfraLogsTool) Name() string        { return "get_infra_logs" }
func (t *GetInfraLogsTool) Description() string  { return "Get logs from kube-proxy, CoreDNS, or CNI pods" }
func (t *GetInfraLogsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"component": map[string]interface{}{
				"type":        "string",
				"description": "Infrastructure component: kube-proxy, coredns, or cni",
				"enum":        []string{"kube-proxy", "coredns", "cni"},
			},
			"namespace": map[string]interface{}{"type": "string", "description": "Namespace override (default: kube-system)"},
			"tail":      map[string]interface{}{"type": "number", "description": "Number of lines from the end (default 100)"},
			"since":     map[string]interface{}{"type": "string", "description": "Duration to look back (e.g., 5m, 1h)"},
		},
		"required": []string{"component"},
	}
}

func (t *GetInfraLogsTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	component := getStringArg(args, "component", "")
	ns := getStringArg(args, "namespace", "kube-system")
	tail := getIntArg(args, "tail", 100)
	since := getStringArg(args, "since", "")

	var labelSelector string
	switch component {
	case "kube-proxy":
		labelSelector = "k8s-app=kube-proxy"
	case "coredns":
		labelSelector = "k8s-app=kube-dns"
	case "cni":
		// Try multiple CNI label selectors
		cniSelectors := []string{
			"k8s-app=cilium",
			"k8s-app=calico-node",
			"app=flannel",
			"app=kube-proxy",
		}
		for _, sel := range cniSelectors {
			pods, err := t.Clients.Clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
				LabelSelector: sel,
				Limit:         1,
			})
			if err == nil && len(pods.Items) > 0 {
				labelSelector = sel
				break
			}
		}
		if labelSelector == "" {
			return nil, fmt.Errorf("no CNI pods found in namespace %s", ns)
		}
	default:
		return nil, fmt.Errorf("unsupported component: %s (use kube-proxy, coredns, or cni)", component)
	}

	pods, err := t.Clients.Clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list %s pods: %w", component, err)
	}
	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no %s pods found with selector %s in %s", component, labelSelector, ns)
	}

	results := make([]map[string]interface{}, 0, len(pods.Items))
	for _, pod := range pods.Items {
		container := pod.Spec.Containers[0].Name
		logs, err := getPodLogs(ctx, t.Clients, ns, pod.Name, container, int64(tail), since)
		if err != nil {
			continue
		}
		results = append(results, map[string]interface{}{
			"pod":       pod.Name,
			"namespace": ns,
			"container": container,
			"node":      pod.Spec.NodeName,
			"logs":      logs,
		})
	}

	return NewResponse(t.Cfg, t.Name(), map[string]interface{}{
		"component": component,
		"podCount":  len(results),
		"results":   results,
	}), nil
}

// --- analyze_log_errors ---

type AnalyzeLogErrorsTool struct{ BaseTool }

func (t *AnalyzeLogErrorsTool) Name() string        { return "analyze_log_errors" }
func (t *AnalyzeLogErrorsTool) Description() string  { return "Read logs and extract error/warning lines related to misconfig, rate limiting, connection issues, TLS errors" }
func (t *AnalyzeLogErrorsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pod":       map[string]interface{}{"type": "string", "description": "Pod name"},
			"namespace": map[string]interface{}{"type": "string", "description": "Kubernetes namespace"},
			"container": map[string]interface{}{"type": "string", "description": "Container name (optional, uses first container)"},
			"tail":      map[string]interface{}{"type": "number", "description": "Number of lines to analyze (default 500)"},
			"since":     map[string]interface{}{"type": "string", "description": "Duration to look back (e.g., 5m, 1h)"},
		},
		"required": []string{"pod", "namespace"},
	}
}

func (t *AnalyzeLogErrorsTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	podName := getStringArg(args, "pod", "")
	ns := getStringArg(args, "namespace", "default")
	container := getStringArg(args, "container", "")
	tail := getIntArg(args, "tail", 500)
	since := getStringArg(args, "since", "")

	if container == "" {
		pod, err := t.Clients.Clientset.CoreV1().Pods(ns).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get pod %s/%s: %w", ns, podName, err)
		}
		if len(pod.Spec.Containers) > 0 {
			// Prefer proxy container if found, otherwise first container
			container = findProxyContainer(pod)
			if container == "" {
				container = pod.Spec.Containers[0].Name
			}
		}
	}

	logs, err := getPodLogs(ctx, t.Clients, ns, podName, container, int64(tail), since)
	if err != nil {
		return nil, err
	}

	// Filter for error patterns
	errorLines := make([]string, 0)
	scanner := bufio.NewScanner(strings.NewReader(logs))
	for scanner.Scan() {
		line := scanner.Text()
		if errorPatterns.MatchString(line) {
			errorLines = append(errorLines, line)
		}
	}

	// Categorize errors
	categories := map[string]int{
		"connection_errors":  0,
		"tls_errors":        0,
		"rate_limiting":     0,
		"misconfig":         0,
		"rbac_denied":       0,
		"upstream_issues":   0,
		"timeout":           0,
		"other_errors":      0,
	}

	for _, line := range errorLines {
		lower := strings.ToLower(line)
		switch {
		case strings.Contains(lower, "connection refused") || strings.Contains(lower, "no healthy"):
			categories["connection_errors"]++
		case strings.Contains(lower, "tls"):
			categories["tls_errors"]++
		case strings.Contains(lower, "rate") || strings.Contains(lower, "429") || strings.Contains(lower, "overflow"):
			categories["rate_limiting"]++
		case strings.Contains(lower, "misconfigur") || strings.Contains(lower, "invalid"):
			categories["misconfig"]++
		case strings.Contains(lower, "rbac") || strings.Contains(lower, "denied") || strings.Contains(lower, "403"):
			categories["rbac_denied"]++
		case strings.Contains(lower, "upstream") || strings.Contains(lower, "503") || strings.Contains(lower, "circuit"):
			categories["upstream_issues"]++
		case strings.Contains(lower, "timeout"):
			categories["timeout"]++
		default:
			categories["other_errors"]++
		}
	}

	return NewResponse(t.Cfg, t.Name(), map[string]interface{}{
		"pod":            podName,
		"namespace":      ns,
		"container":      container,
		"totalLines":     tail,
		"errorLineCount": len(errorLines),
		"categories":     categories,
		"errorLines":     errorLines,
	}), nil
}

// Helper functions

func findProxyContainer(pod *corev1.Pod) string {
	for _, c := range pod.Spec.Containers {
		for _, proxyName := range proxyContainerNames {
			if c.Name == proxyName {
				return c.Name
			}
		}
	}
	return ""
}

func getPodLogs(ctx context.Context, clients *k8s.Clients, namespace, podName, container string, tailLines int64, since string) (string, error) {
	opts := &corev1.PodLogOptions{
		Container: container,
		TailLines: &tailLines,
	}

	if since != "" {
		dur, err := time.ParseDuration(since)
		if err == nil {
			sinceTime := metav1.NewTime(metav1.Now().Add(-dur))
			opts.SinceTime = &sinceTime
		}
	}

	req := clients.Clientset.CoreV1().Pods(namespace).GetLogs(podName, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get logs for %s/%s/%s: %w", namespace, podName, container, err)
	}
	defer stream.Close()

	buf := new(strings.Builder)
	_, err = io.Copy(buf, stream)
	if err != nil {
		return "", fmt.Errorf("failed to read log stream: %w", err)
	}

	return buf.String(), nil
}

