package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/isitobservable/k8s-networking-mcp/pkg/probes"
	"github.com/isitobservable/k8s-networking-mcp/pkg/types"
)

// --- probe_connectivity ---

type ProbeConnectivityTool struct {
	BaseTool
	ProbeManager *probes.Manager
}

func (t *ProbeConnectivityTool) Name() string { return "probe_connectivity" }
func (t *ProbeConnectivityTool) Description() string {
	return "Deploy an ephemeral pod to test TCP network connectivity between namespaces or services"
}
func (t *ProbeConnectivityTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"source_namespace": map[string]interface{}{
				"type":        "string",
				"description": "Namespace to deploy the probe pod in (source of connectivity test)",
			},
			"target_host": map[string]interface{}{
				"type":        "string",
				"description": "Target hostname or IP (e.g., my-service.target-ns.svc.cluster.local)",
			},
			"target_port": map[string]interface{}{
				"type":        "integer",
				"description": "Target port to test connectivity on",
			},
			"timeout_seconds": map[string]interface{}{
				"type":        "integer",
				"description": "Probe timeout in seconds (default: 10, max: 30)",
			},
		},
		"required": []string{"target_host", "target_port"},
	}
}

func (t *ProbeConnectivityTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	sourceNS := getStringArg(args, "source_namespace", t.Cfg.ProbeNamespace)
	targetHost := getStringArg(args, "target_host", "")
	targetPort := getIntArg(args, "target_port", 80)
	timeoutSec := getIntArg(args, "timeout_seconds", 10)

	if targetHost == "" {
		return nil, &types.MCPError{
			Code:    types.ErrCodeInvalidInput,
			Tool:    t.Name(),
			Message: "target_host is required",
		}
	}
	if timeoutSec > 30 {
		timeoutSec = 30
	}

	req := probes.ProbeRequest{
		Type:      probes.ProbeTypeConnectivity,
		Namespace: sourceNS,
		Command: []string{
			"sh", "-c",
			fmt.Sprintf("nc -z -w %d %s %d && echo 'CONNECTION_SUCCESS' || echo 'CONNECTION_FAILED'", timeoutSec, targetHost, targetPort),
		},
	}

	result, err := t.ProbeManager.Execute(ctx, req)
	if err != nil {
		return nil, err
	}

	findings := make([]types.DiagnosticFinding, 0, 1)

	if result.Success && strings.Contains(result.Output, "CONNECTION_SUCCESS") {
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityOK,
			Category: types.CategoryConnectivity,
			Summary:  fmt.Sprintf("TCP connectivity from %s to %s:%d succeeded", sourceNS, targetHost, targetPort),
			Detail:   fmt.Sprintf("output=%s duration=%s", strings.TrimSpace(result.Output), result.Duration),
		})
	} else {
		detail := strings.TrimSpace(result.Output)
		if result.Error != "" {
			detail = result.Error + "; " + detail
		}
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityCritical,
			Category:   types.CategoryConnectivity,
			Summary:    fmt.Sprintf("TCP connectivity from %s to %s:%d failed", sourceNS, targetHost, targetPort),
			Detail:     detail,
			Suggestion: "Check NetworkPolicies, service endpoints, DNS resolution, and firewall rules between the source and destination namespaces.",
		})
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, sourceNS, ""), nil
}

// --- probe_dns ---

type ProbeDNSTool struct {
	BaseTool
	ProbeManager *probes.Manager
}

func (t *ProbeDNSTool) Name() string { return "probe_dns" }
func (t *ProbeDNSTool) Description() string {
	return "Deploy an ephemeral pod to test DNS resolution from within the cluster"
}
func (t *ProbeDNSTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"hostname": map[string]interface{}{
				"type":        "string",
				"description": "Hostname to resolve (e.g., my-service.default.svc.cluster.local)",
			},
			"source_namespace": map[string]interface{}{
				"type":        "string",
				"description": "Namespace to deploy the probe pod in",
			},
			"record_type": map[string]interface{}{
				"type":        "string",
				"description": "DNS record type to query (A, AAAA, SRV, CNAME). Default: A",
			},
		},
		"required": []string{"hostname"},
	}
}

func (t *ProbeDNSTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	hostname := getStringArg(args, "hostname", "")
	sourceNS := getStringArg(args, "source_namespace", t.Cfg.ProbeNamespace)
	recordType := getStringArg(args, "record_type", "A")

	if hostname == "" {
		return nil, &types.MCPError{
			Code:    types.ErrCodeInvalidInput,
			Tool:    t.Name(),
			Message: "hostname is required",
		}
	}

	req := probes.ProbeRequest{
		Type:      probes.ProbeTypeDNS,
		Namespace: sourceNS,
		Command: []string{
			"sh", "-c",
			fmt.Sprintf("nslookup -type=%s %s 2>&1; echo EXIT_CODE=$?", recordType, hostname),
		},
	}

	result, err := t.ProbeManager.Execute(ctx, req)
	if err != nil {
		return nil, err
	}

	findings := make([]types.DiagnosticFinding, 0, 1)
	output := strings.TrimSpace(result.Output)

	if result.Success && !strings.Contains(output, "** server can't find") && !strings.Contains(output, "NXDOMAIN") {
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityOK,
			Category: types.CategoryDNS,
			Summary:  fmt.Sprintf("DNS resolution for %s (%s) succeeded", hostname, recordType),
			Detail:   fmt.Sprintf("output=%s duration=%s", output, result.Duration),
		})
	} else {
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityCritical,
			Category:   types.CategoryDNS,
			Summary:    fmt.Sprintf("DNS resolution for %s (%s) failed", hostname, recordType),
			Detail:     output,
			Suggestion: "Check CoreDNS pods are running, verify the service exists in the expected namespace, and check NetworkPolicies are not blocking DNS (port 53).",
		})
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, sourceNS, ""), nil
}

// --- probe_http ---

type ProbeHTTPTool struct {
	BaseTool
	ProbeManager *probes.Manager
}

func (t *ProbeHTTPTool) Name() string { return "probe_http" }
func (t *ProbeHTTPTool) Description() string {
	return "Deploy an ephemeral pod to perform HTTP/HTTPS requests against services from within the cluster"
}
func (t *ProbeHTTPTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "Target URL (e.g., http://my-service.default.svc.cluster.local:8080/health)",
			},
			"method": map[string]interface{}{
				"type":        "string",
				"description": "HTTP method (GET, POST, HEAD). Default: GET",
			},
			"headers": map[string]interface{}{
				"type":        "string",
				"description": "Additional headers as 'Key: Value' pairs separated by semicolons",
			},
			"source_namespace": map[string]interface{}{
				"type":        "string",
				"description": "Namespace to deploy the probe pod in",
			},
			"timeout_seconds": map[string]interface{}{
				"type":        "integer",
				"description": "Request timeout in seconds (default: 10, max: 30)",
			},
		},
		"required": []string{"url"},
	}
}

func (t *ProbeHTTPTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	url := getStringArg(args, "url", "")
	method := getStringArg(args, "method", "GET")
	headers := getStringArg(args, "headers", "")
	sourceNS := getStringArg(args, "source_namespace", t.Cfg.ProbeNamespace)
	timeoutSec := getIntArg(args, "timeout_seconds", 10)

	if url == "" {
		return nil, &types.MCPError{
			Code:    types.ErrCodeInvalidInput,
			Tool:    t.Name(),
			Message: "url is required",
		}
	}
	if timeoutSec > 30 {
		timeoutSec = 30
	}

	// Build curl command
	curlCmd := fmt.Sprintf("curl -s -o /tmp/body -w '%%{http_code}|%%{time_total}|%%{ssl_verify_result}' -X %s --max-time %d -L", method, timeoutSec)

	if headers != "" {
		for _, h := range strings.Split(headers, ";") {
			h = strings.TrimSpace(h)
			if h != "" {
				curlCmd += fmt.Sprintf(" -H '%s'", h)
			}
		}
	}

	curlCmd += fmt.Sprintf(" '%s'", url)
	curlCmd += " 2>&1; echo; echo '---BODY---'; head -c 1024 /tmp/body 2>/dev/null || true"

	req := probes.ProbeRequest{
		Type:      probes.ProbeTypeHTTP,
		Namespace: sourceNS,
		Command:   []string{"sh", "-c", curlCmd},
	}

	result, err := t.ProbeManager.Execute(ctx, req)
	if err != nil {
		return nil, err
	}

	findings := make([]types.DiagnosticFinding, 0, 1)
	output := strings.TrimSpace(result.Output)

	// Parse curl output: status_code|time_total|ssl_verify
	parts := strings.SplitN(output, "\n", 2)
	statusLine := parts[0]
	bodySnippet := ""
	if idx := strings.Index(output, "---BODY---"); idx >= 0 {
		bodySnippet = strings.TrimSpace(output[idx+len("---BODY---"):])
		if len(bodySnippet) > 1024 {
			bodySnippet = bodySnippet[:1024] + "...(truncated)"
		}
	}

	curlParts := strings.SplitN(statusLine, "|", 3)
	statusCode := "000"
	responseTime := "unknown"
	if len(curlParts) >= 2 {
		statusCode = curlParts[0]
		responseTime = curlParts[1] + "s"
	}

	if result.Success && statusCode != "000" {
		severity := types.SeverityOK
		if statusCode >= "400" {
			severity = types.SeverityWarning
		}
		if statusCode >= "500" {
			severity = types.SeverityCritical
		}

		findings = append(findings, types.DiagnosticFinding{
			Severity: severity,
			Category: types.CategoryConnectivity,
			Summary:  fmt.Sprintf("HTTP %s %s returned %s in %s", method, url, statusCode, responseTime),
			Detail:   fmt.Sprintf("status=%s response_time=%s body_snippet=%s", statusCode, responseTime, bodySnippet),
		})
	} else {
		detail := output
		if result.Error != "" {
			detail = result.Error + "; " + detail
		}
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityCritical,
			Category:   types.CategoryConnectivity,
			Summary:    fmt.Sprintf("HTTP %s %s failed (connection error or timeout)", method, url),
			Detail:     detail,
			Suggestion: "Check that the target service is running, DNS resolves correctly, and there are no NetworkPolicies or mTLS requirements blocking the connection.",
		})
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, sourceNS, ""), nil
}
