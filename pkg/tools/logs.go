package tools

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/isitobservable/k8s-networking-mcp/pkg/k8s"
	"github.com/isitobservable/k8s-networking-mcp/pkg/types"
)

const maxLogBytes = 102400 // 100KB

var proxyContainerNames = []string{"istio-proxy", "envoy", "linkerd-proxy"}

var errorPatterns = regexp.MustCompile(`(?i)(error|warn|rate.?limit|429|503|connection refused|TLS|timeout|denied|RBAC|misconfigur|failed|upstream.?reset|no.?healthy|circuit.?break|overflow|rejected)`)

// logResult holds the result of a log fetch with truncation metadata.
type logResult struct {
	logs          string
	truncated     bool
	returnedLines int
}

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
			return nil, &types.MCPError{
				Code:    types.ErrCodeInvalidInput,
				Tool:    t.Name(),
				Message: fmt.Sprintf("no proxy sidecar container found in pod %s/%s", ns, podName),
				Detail:  fmt.Sprintf("looked for containers named: %s", strings.Join(proxyContainerNames, ", ")),
			}
		}
	}

	result, err := getPodLogs(ctx, t.Clients, ns, podName, container, int64(tail), since)
	if err != nil {
		return nil, err
	}

	findings := []types.DiagnosticFinding{
		{
			Severity: types.SeverityInfo,
			Category: types.CategoryLogs,
			Resource: &types.ResourceRef{
				Kind:      "Pod",
				Namespace: ns,
				Name:      podName,
			},
			Summary: fmt.Sprintf("Retrieved %d log lines from %s/%s container %s", result.returnedLines, ns, podName, container),
			Detail:  result.logs,
		},
	}

	if result.truncated {
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityWarning,
			Category:   types.CategoryLogs,
			Summary:    fmt.Sprintf("Log output truncated at 100KB limit for %s/%s container %s", ns, podName, container),
			Suggestion: "Use a smaller --tail value or narrower --since window to avoid truncation",
		})
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, ""), nil
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

	var findings []types.DiagnosticFinding

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
			lr, err := getPodLogs(ctx, t.Clients, pod.Namespace, pod.Name, container, int64(tail), since)
			if err != nil {
				continue
			}
			findings = append(findings, types.DiagnosticFinding{
				Severity: types.SeverityInfo,
				Category: types.CategoryLogs,
				Resource: &types.ResourceRef{
					Kind:      "Pod",
					Namespace: pod.Namespace,
					Name:      pod.Name,
				},
				Summary: fmt.Sprintf("Retrieved %d log lines from %s/%s container %s (%s)", lr.returnedLines, pod.Namespace, pod.Name, container, gl.desc),
				Detail:  lr.logs,
			})
			if lr.truncated {
				findings = append(findings, types.DiagnosticFinding{
					Severity:   types.SeverityWarning,
					Category:   types.CategoryLogs,
					Summary:    fmt.Sprintf("Log output truncated at 100KB limit for %s/%s container %s", pod.Namespace, pod.Name, container),
					Suggestion: "Use a smaller --tail value or narrower --since window to avoid truncation",
				})
			}
		}
	}

	if len(findings) == 0 {
		return nil, &types.MCPError{
			Code:    types.ErrCodeProviderNotFound,
			Tool:    t.Name(),
			Message: "no gateway controller pods found",
			Detail:  "searched for Istio, Envoy Gateway, and Gateway API controller pods by known labels",
		}
	}

	responseNs := ns
	if responseNs == "" {
		responseNs = "all"
	}
	return NewToolResultResponse(t.Cfg, t.Name(), findings, responseNs, ""), nil
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
			return nil, &types.MCPError{
				Code:    types.ErrCodeProviderNotFound,
				Tool:    t.Name(),
				Message: fmt.Sprintf("no CNI pods found in namespace %s", ns),
				Detail:  "searched for Cilium, Calico, Flannel, and kube-proxy pods by known labels",
			}
		}
	default:
		return nil, &types.MCPError{
			Code:    types.ErrCodeInvalidInput,
			Tool:    t.Name(),
			Message: fmt.Sprintf("unsupported component: %s", component),
			Detail:  "supported components: kube-proxy, coredns, cni",
		}
	}

	pods, err := t.Clients.Clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list %s pods: %w", component, err)
	}
	if len(pods.Items) == 0 {
		return nil, &types.MCPError{
			Code:    types.ErrCodeProviderNotFound,
			Tool:    t.Name(),
			Message: fmt.Sprintf("no %s pods found with selector %s in %s", component, labelSelector, ns),
		}
	}

	var findings []types.DiagnosticFinding
	for _, pod := range pods.Items {
		container := pod.Spec.Containers[0].Name
		lr, err := getPodLogs(ctx, t.Clients, ns, pod.Name, container, int64(tail), since)
		if err != nil {
			continue
		}
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryLogs,
			Resource: &types.ResourceRef{
				Kind:      "Pod",
				Namespace: ns,
				Name:      pod.Name,
			},
			Summary: fmt.Sprintf("Retrieved %d log lines from %s/%s container %s (node %s, component %s)", lr.returnedLines, ns, pod.Name, container, pod.Spec.NodeName, component),
			Detail:  lr.logs,
		})
		if lr.truncated {
			findings = append(findings, types.DiagnosticFinding{
				Severity:   types.SeverityWarning,
				Category:   types.CategoryLogs,
				Summary:    fmt.Sprintf("Log output truncated at 100KB limit for %s/%s container %s", ns, pod.Name, container),
				Suggestion: "Use a smaller --tail value or narrower --since window to avoid truncation",
			})
		}
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, ""), nil
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

const maxErrorLines = 50

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

	lr, err := getPodLogs(ctx, t.Clients, ns, podName, container, int64(tail), since)
	if err != nil {
		return nil, err
	}

	// Filter for error patterns and categorize
	type categorizedLines struct {
		category string
		lines    []string
	}
	categoryMap := map[string]*categorizedLines{
		"connection_errors": {category: "connection_errors"},
		"tls_errors":        {category: "tls_errors"},
		"rate_limiting":     {category: "rate_limiting"},
		"misconfig":         {category: "misconfig"},
		"rbac_denied":       {category: "rbac_denied"},
		"upstream_issues":   {category: "upstream_issues"},
		"timeout":           {category: "timeout"},
		"other_errors":      {category: "other_errors"},
	}

	totalErrorLines := 0
	scanner := bufio.NewScanner(strings.NewReader(lr.logs))
	for scanner.Scan() {
		line := scanner.Text()
		if !errorPatterns.MatchString(line) {
			continue
		}
		totalErrorLines++

		lower := strings.ToLower(line)
		var cat string
		switch {
		case strings.Contains(lower, "connection refused") || strings.Contains(lower, "no healthy"):
			cat = "connection_errors"
		case strings.Contains(lower, "tls"):
			cat = "tls_errors"
		case strings.Contains(lower, "rate") || strings.Contains(lower, "429") || strings.Contains(lower, "overflow"):
			cat = "rate_limiting"
		case strings.Contains(lower, "misconfigur") || strings.Contains(lower, "invalid"):
			cat = "misconfig"
		case strings.Contains(lower, "rbac") || strings.Contains(lower, "denied") || strings.Contains(lower, "403"):
			cat = "rbac_denied"
		case strings.Contains(lower, "upstream") || strings.Contains(lower, "503") || strings.Contains(lower, "circuit"):
			cat = "upstream_issues"
		case strings.Contains(lower, "timeout"):
			cat = "timeout"
		default:
			cat = "other_errors"
		}
		categoryMap[cat].lines = append(categoryMap[cat].lines, line)
	}

	podRef := &types.ResourceRef{
		Kind:      "Pod",
		Namespace: ns,
		Name:      podName,
	}

	// No errors found — return ok finding
	if totalErrorLines == 0 {
		findings := []types.DiagnosticFinding{
			{
				Severity: types.SeverityOK,
				Category: types.CategoryLogs,
				Resource: podRef,
				Summary:  fmt.Sprintf("No error patterns found in %d log lines from %s/%s container %s", lr.returnedLines, ns, podName, container),
			},
		}
		return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, ""), nil
	}

	// Build summary counts string and findings per non-zero category
	var findings []types.DiagnosticFinding
	var countParts []string

	// Stable iteration order for categories
	categoryOrder := []string{
		"connection_errors", "tls_errors", "rate_limiting", "misconfig",
		"rbac_denied", "upstream_issues", "timeout", "other_errors",
	}
	for _, catName := range categoryOrder {
		cl := categoryMap[catName]
		if len(cl.lines) == 0 {
			continue
		}
		countParts = append(countParts, fmt.Sprintf("%s=%d", catName, len(cl.lines)))

		// Cap lines in detail
		detail := cl.lines
		if len(detail) > maxErrorLines {
			detail = detail[:maxErrorLines]
		}

		severity := types.SeverityWarning
		if catName == "other_errors" {
			severity = types.SeverityInfo
		}

		findings = append(findings, types.DiagnosticFinding{
			Severity: severity,
			Category: types.CategoryLogs,
			Resource: podRef,
			Summary:  fmt.Sprintf("%d %s lines in %s/%s container %s", len(cl.lines), catName, ns, podName, container),
			Detail:   strings.Join(detail, "\n"),
		})
	}

	// Prepend an overall summary finding
	summaryFinding := types.DiagnosticFinding{
		Severity: types.SeverityWarning,
		Category: types.CategoryLogs,
		Resource: podRef,
		Summary:  fmt.Sprintf("Found %d error lines in %d log lines from %s/%s container %s: %s", totalErrorLines, lr.returnedLines, ns, podName, container, strings.Join(countParts, ", ")),
	}
	findings = append([]types.DiagnosticFinding{summaryFinding}, findings...)

	if lr.truncated {
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityWarning,
			Category:   types.CategoryLogs,
			Summary:    fmt.Sprintf("Log input was truncated at 100KB limit for %s/%s container %s — error counts may be incomplete", ns, podName, container),
			Suggestion: "Use a smaller --tail value or narrower --since window to get complete analysis",
		})
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, ""), nil
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

func getPodLogs(ctx context.Context, clients *k8s.Clients, namespace, podName, container string, tailLines int64, since string) (*logResult, error) {
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
		return nil, fmt.Errorf("failed to get logs for %s/%s/%s: %w", namespace, podName, container, err)
	}
	defer stream.Close()

	// Read up to maxLogBytes+1 to detect truncation
	data, err := io.ReadAll(io.LimitReader(stream, maxLogBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read log stream: %w", err)
	}

	truncated := len(data) > maxLogBytes
	if truncated {
		data = data[:maxLogBytes]
	}

	logs := string(data)
	lineCount := bytes.Count(data, []byte("\n"))
	// Account for a final line without trailing newline
	if len(data) > 0 && data[len(data)-1] != '\n' {
		lineCount++
	}

	return &logResult{
		logs:          logs,
		truncated:     truncated,
		returnedLines: lineCount,
	}, nil
}
