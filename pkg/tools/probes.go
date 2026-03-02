package tools

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/isitobservable/k8s-networking-mcp/pkg/probes"
	"github.com/isitobservable/k8s-networking-mcp/pkg/types"
)

// validHostname matches DNS names, IPs, and K8s service FQDNs.
var validHostname = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// probeAllowedMethods whitelist for HTTP probe methods.
var probeAllowedMethods = map[string]bool{
	"GET": true, "POST": true, "HEAD": true, "PUT": true, "DELETE": true, "PATCH": true, "OPTIONS": true,
}

// validRecordType whitelist.
var validRecordTypes = map[string]bool{
	"A": true, "AAAA": true, "SRV": true, "CNAME": true, "MX": true, "TXT": true, "NS": true, "PTR": true,
}

// containsShellMeta returns true if the string contains shell metacharacters.
func containsShellMeta(s string) bool {
	return strings.ContainsAny(s, "'\"`;|&$(){}[]<>!\\#~")
}

// servicePort holds a resolved K8s Service port.
type servicePort struct {
	Name     string
	Port     int32
	Protocol string
}

// parseK8sServiceHost attempts to parse a hostname as a K8s service reference.
// Returns (serviceName, namespace, ok). fallbackNS is used when hostname is a bare name.
func parseK8sServiceHost(host, fallbackNS string) (name, namespace string, ok bool) {
	parts := strings.Split(host, ".")
	switch {
	case len(parts) >= 3 && parts[2] == "svc":
		return parts[0], parts[1], true
	case len(parts) == 2:
		return parts[0], parts[1], true
	case len(parts) == 1 && fallbackNS != "":
		return parts[0], fallbackNS, true
	default:
		return "", "", false
	}
}

// resolveServicePorts looks up a K8s Service by hostname and returns its ports.
func resolveServicePorts(ctx context.Context, bt *BaseTool, host, fallbackNS string) ([]servicePort, error) {
	svcName, namespace, ok := parseK8sServiceHost(host, fallbackNS)
	if !ok {
		return nil, fmt.Errorf("not a K8s service hostname: %s", host)
	}

	svc, err := bt.Clients.Clientset.CoreV1().Services(namespace).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get service %s/%s: %w", namespace, svcName, err)
	}

	ports := make([]servicePort, 0, len(svc.Spec.Ports))
	for _, p := range svc.Spec.Ports {
		proto := string(p.Protocol)
		if proto == "" {
			proto = "TCP"
		}
		ports = append(ports, servicePort{
			Name:     p.Name,
			Port:     p.Port,
			Protocol: proto,
		})
	}
	return ports, nil
}

// --- probe_connectivity ---

type ProbeConnectivityTool struct {
	BaseTool
	ProbeManager *probes.Manager
}

func (t *ProbeConnectivityTool) Name() string { return "probe_connectivity" }
func (t *ProbeConnectivityTool) Description() string {
	return "Deploy an ephemeral pod to test TCP network connectivity between namespaces or services. When target_port is omitted and target_host is a K8s service, the port is auto-resolved from the Service spec; if the service exposes multiple ports, all are tested."
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
				"description": "Target port to test connectivity on. Optional: when omitted, the port is auto-resolved from the K8s Service; if the service has multiple ports, all are tested.",
			},
			"timeout_seconds": map[string]interface{}{
				"type":        "integer",
				"description": "Probe timeout in seconds (default: 10, max: 30)",
			},
		},
		"required": []string{"target_host"},
	}
}

func (t *ProbeConnectivityTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	sourceNS := getStringArg(args, "source_namespace", t.Cfg.ProbeNamespace)
	targetHost := getStringArg(args, "target_host", "")
	timeoutSec := getIntArg(args, "timeout_seconds", 10)

	if targetHost == "" {
		return nil, &types.MCPError{
			Code:    types.ErrCodeInvalidInput,
			Tool:    t.Name(),
			Message: "target_host is required",
		}
	}
	if !validHostname.MatchString(targetHost) {
		return nil, &types.MCPError{
			Code:    types.ErrCodeInvalidInput,
			Tool:    t.Name(),
			Message: "target_host contains invalid characters",
		}
	}
	if timeoutSec > 30 {
		timeoutSec = 30
	}

	// Determine which port(s) to test
	targetPort := getIntArg(args, "target_port", 0)
	var ports []int
	if targetPort > 0 {
		ports = []int{targetPort}
	} else {
		resolved, err := resolveServicePorts(ctx, &t.BaseTool, targetHost, sourceNS)
		if err != nil || len(resolved) == 0 {
			slog.InfoContext(ctx, "service port resolution failed, defaulting to port 80", "host", targetHost, "error", err)
			ports = []int{80}
		} else {
			ports = make([]int, 0, len(resolved))
			for _, p := range resolved {
				ports = append(ports, int(p.Port))
			}
		}
	}

	allFindings := make([]types.DiagnosticFinding, 0, len(ports))
	for _, port := range ports {
		findings, err := t.probePort(ctx, sourceNS, targetHost, port, timeoutSec)
		if err != nil {
			return nil, err
		}
		allFindings = append(allFindings, findings...)
	}

	return NewToolResultResponse(t.Cfg, t.Name(), allFindings, sourceNS, ""), nil
}

func (t *ProbeConnectivityTool) probePort(ctx context.Context, sourceNS, targetHost string, targetPort, timeoutSec int) ([]types.DiagnosticFinding, error) {
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

	return findings, nil
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
	if !validHostname.MatchString(hostname) {
		return nil, &types.MCPError{
			Code:    types.ErrCodeInvalidInput,
			Tool:    t.Name(),
			Message: "hostname contains invalid characters",
		}
	}
	if !validRecordTypes[strings.ToUpper(recordType)] {
		recordType = "A"
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
	return "Deploy an ephemeral pod to perform HTTP/HTTPS requests against services from within the cluster. When the URL omits a port and the hostname is a K8s service, the port is auto-resolved from the Service spec; if the service exposes multiple ports, all are tested."
}
func (t *ProbeHTTPTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "Target URL (e.g., http://my-service.default.svc.cluster.local/health). When the port is omitted, it is auto-resolved from the K8s Service.",
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
	rawURL := getStringArg(args, "url", "")
	method := getStringArg(args, "method", "GET")
	headers := getStringArg(args, "headers", "")
	sourceNS := getStringArg(args, "source_namespace", t.Cfg.ProbeNamespace)
	timeoutSec := getIntArg(args, "timeout_seconds", 10)

	if rawURL == "" {
		return nil, &types.MCPError{
			Code:    types.ErrCodeInvalidInput,
			Tool:    t.Name(),
			Message: "url is required",
		}
	}
	if containsShellMeta(rawURL) {
		return nil, &types.MCPError{
			Code:    types.ErrCodeInvalidInput,
			Tool:    t.Name(),
			Message: "url contains invalid shell characters",
		}
	}
	method = strings.ToUpper(method)
	if !probeAllowedMethods[method] {
		method = "GET"
	}
	if timeoutSec > 30 {
		timeoutSec = 30
	}

	// Determine which URL(s) to test. If the URL has no port and the hostname
	// looks like a K8s service, resolve the port(s) from the Service spec.
	urls := []string{rawURL}
	u, parseErr := url.Parse(rawURL)
	if parseErr == nil && u.Host != "" && !strings.Contains(u.Host, ":") {
		resolved, resolveErr := resolveServicePorts(ctx, &t.BaseTool, u.Hostname(), sourceNS)
		if resolveErr != nil {
			slog.InfoContext(ctx, "service port resolution failed for HTTP probe, using URL as-is", "host", u.Hostname(), "error", resolveErr)
		} else if len(resolved) > 0 {
			urls = make([]string, 0, len(resolved))
			for _, p := range resolved {
				u2 := *u
				u2.Host = fmt.Sprintf("%s:%d", u.Hostname(), p.Port)
				urls = append(urls, u2.String())
			}
		}
	}

	allFindings := make([]types.DiagnosticFinding, 0, len(urls))
	for _, testURL := range urls {
		findings, err := t.probeURL(ctx, sourceNS, method, headers, timeoutSec, testURL)
		if err != nil {
			return nil, err
		}
		allFindings = append(allFindings, findings...)
	}

	return NewToolResultResponse(t.Cfg, t.Name(), allFindings, sourceNS, ""), nil
}

func (t *ProbeHTTPTool) probeURL(ctx context.Context, sourceNS, method, headers string, timeoutSec int, targetURL string) ([]types.DiagnosticFinding, error) {
	// Build curl command
	curlCmd := fmt.Sprintf("curl -s -o /tmp/body -w '%%{http_code}|%%{time_total}|%%{ssl_verify_result}' -X %s --max-time %d -L", method, timeoutSec)

	if headers != "" {
		for _, h := range strings.Split(headers, ";") {
			h = strings.TrimSpace(h)
			if h != "" && !containsShellMeta(h) {
				curlCmd += fmt.Sprintf(" -H '%s'", h)
			}
		}
	}

	curlCmd += fmt.Sprintf(" %s", targetURL)
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
		if code, err := strconv.Atoi(statusCode); err == nil {
			if code >= 500 {
				severity = types.SeverityCritical
			} else if code >= 400 {
				severity = types.SeverityWarning
			}
		}

		findings = append(findings, types.DiagnosticFinding{
			Severity: severity,
			Category: types.CategoryConnectivity,
			Summary:  fmt.Sprintf("HTTP %s %s returned %s in %s", method, targetURL, statusCode, responseTime),
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
			Summary:    fmt.Sprintf("HTTP %s %s failed (connection error or timeout)", method, targetURL),
			Detail:     detail,
			Suggestion: "Check that the target service is running, DNS resolves correctly, and there are no NetworkPolicies or mTLS requirements blocking the connection.",
		})
	}

	return findings, nil
}
