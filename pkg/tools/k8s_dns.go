package tools

import (
	"context"
	"fmt"
	"net"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/isitobservable/k8s-networking-mcp/pkg/types"
)

var daemonsetsGVR = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"}
var configmapsGVR = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}

// --- check_dns_resolution ---

type CheckDNSTool struct{ BaseTool }

func (t *CheckDNSTool) Name() string        { return "check_dns_resolution" }
func (t *CheckDNSTool) Description() string  { return "DNS lookup for a hostname plus kube-dns service health check" }
func (t *CheckDNSTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"hostname": map[string]interface{}{
				"type":        "string",
				"description": "Hostname to resolve (e.g., my-service.default.svc.cluster.local)",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Namespace context for short names",
			},
		},
		"required": []string{"hostname"},
	}
}

func (t *CheckDNSTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	hostname := getStringArg(args, "hostname", "")

	findings := make([]types.DiagnosticFinding, 0, 2)

	// DNS lookup
	ips, lookupErr := net.LookupHost(hostname)

	if lookupErr != nil {
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityCritical,
			Category:   types.CategoryDNS,
			Summary:    fmt.Sprintf("DNS lookup failed for %s: %v", hostname, lookupErr),
			Detail:     fmt.Sprintf("hostname=%s error=%v", hostname, lookupErr),
			Suggestion: "Verify the hostname is correct and kube-dns is healthy. For cluster services, use FQDN format: <service>.<namespace>.svc.cluster.local",
		})
	} else {
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityOK,
			Category: types.CategoryDNS,
			Summary:  fmt.Sprintf("DNS resolved %s -> [%s]", hostname, strings.Join(ips, ", ")),
			Detail:   fmt.Sprintf("hostname=%s addresses=%v", hostname, ips),
		})
	}

	// Check kube-dns service health
	kubeDNS, err := t.Clients.Dynamic.Resource(servicesGVR).Namespace("kube-system").Get(ctx, "kube-dns", metav1.GetOptions{})
	if err == nil {
		clusterIP, _, _ := unstructured.NestedString(kubeDNS.Object, "spec", "clusterIP")

		ep, epErr := t.Clients.Dynamic.Resource(endpointsGVR).Namespace("kube-system").Get(ctx, "kube-dns", metav1.GetOptions{})
		readyCount := 0
		if epErr == nil {
			subsets, _, _ := unstructured.NestedSlice(ep.Object, "subsets")
			for _, s := range subsets {
				if sm, ok := s.(map[string]interface{}); ok {
					if addrs, ok := sm["addresses"].([]interface{}); ok {
						readyCount += len(addrs)
					}
				}
			}
		}

		if readyCount > 0 {
			findings = append(findings, types.DiagnosticFinding{
				Severity: types.SeverityOK,
				Category: types.CategoryDNS,
				Resource: &types.ResourceRef{Kind: "Service", Namespace: "kube-system", Name: "kube-dns"},
				Summary:  fmt.Sprintf("kube-dns healthy: %d ready endpoints, clusterIP=%s", readyCount, clusterIP),
				Detail:   fmt.Sprintf("clusterIP=%s readyEndpoints=%d", clusterIP, readyCount),
			})
		} else {
			findings = append(findings, types.DiagnosticFinding{
				Severity:   types.SeverityCritical,
				Category:   types.CategoryDNS,
				Resource:   &types.ResourceRef{Kind: "Service", Namespace: "kube-system", Name: "kube-dns"},
				Summary:    "kube-dns has 0 ready endpoints",
				Detail:     fmt.Sprintf("clusterIP=%s readyEndpoints=0", clusterIP),
				Suggestion: "Check kube-dns pods in kube-system namespace. Run: kubectl get pods -n kube-system -l k8s-app=kube-dns",
			})
		}
	} else {
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityCritical,
			Category:   types.CategoryDNS,
			Summary:    fmt.Sprintf("kube-dns service not found: %v", err),
			Suggestion: "Verify CoreDNS is deployed in the cluster",
		})
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, "", ""), nil
}

// --- check_kube_proxy_health ---

type CheckKubeProxyHealthTool struct{ BaseTool }

func (t *CheckKubeProxyHealthTool) Name() string        { return "check_kube_proxy_health" }
func (t *CheckKubeProxyHealthTool) Description() string  { return "Check kube-proxy DaemonSet health: pod status across nodes, configuration mode (iptables/IPVS), unhealthy pods" }
func (t *CheckKubeProxyHealthTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *CheckKubeProxyHealthTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	findings := make([]types.DiagnosticFinding, 0, 4)

	// Check kube-proxy DaemonSet
	ds, err := t.Clients.Dynamic.Resource(daemonsetsGVR).Namespace("kube-system").Get(ctx, "kube-proxy", metav1.GetOptions{})
	if err != nil {
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityWarning,
			Category:   types.CategoryConnectivity,
			Summary:    fmt.Sprintf("kube-proxy DaemonSet not found: %v", err),
			Suggestion: "kube-proxy may not be deployed as a DaemonSet (e.g., running as a static pod or replaced by a CNI like Cilium).",
		})
		return NewToolResultResponse(t.Cfg, t.Name(), findings, "kube-system", ""), nil
	}

	desired, _, _ := unstructured.NestedInt64(ds.Object, "status", "desiredNumberScheduled")
	ready, _, _ := unstructured.NestedInt64(ds.Object, "status", "numberReady")
	available, _, _ := unstructured.NestedInt64(ds.Object, "status", "numberAvailable")
	unavailable, _, _ := unstructured.NestedInt64(ds.Object, "status", "numberUnavailable")

	severity := types.SeverityOK
	if unavailable > 0 {
		severity = types.SeverityWarning
	}
	if ready == 0 && desired > 0 {
		severity = types.SeverityCritical
	}

	findings = append(findings, types.DiagnosticFinding{
		Severity: severity,
		Category: types.CategoryConnectivity,
		Resource: &types.ResourceRef{Kind: "DaemonSet", Namespace: "kube-system", Name: "kube-proxy", APIVersion: "apps/v1"},
		Summary:  fmt.Sprintf("kube-proxy: desired=%d ready=%d available=%d unavailable=%d", desired, ready, available, unavailable),
		Detail:   fmt.Sprintf("desiredNumberScheduled=%d numberReady=%d numberAvailable=%d numberUnavailable=%d", desired, ready, available, unavailable),
	})

	// Check kube-proxy ConfigMap for mode
	cm, err := t.Clients.Dynamic.Resource(configmapsGVR).Namespace("kube-system").Get(ctx, "kube-proxy", metav1.GetOptions{})
	if err == nil {
		configData, _, _ := unstructured.NestedString(cm.Object, "data", "config.conf")
		if configData == "" {
			configData, _, _ = unstructured.NestedString(cm.Object, "data", "kubeconfig.conf")
		}

		mode := "iptables"
		if strings.Contains(configData, "mode: ipvs") || strings.Contains(configData, "mode: \"ipvs\"") {
			mode = "ipvs"
		} else if strings.Contains(configData, "mode: nftables") || strings.Contains(configData, "mode: \"nftables\"") {
			mode = "nftables"
		}

		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryConnectivity,
			Resource: &types.ResourceRef{Kind: "ConfigMap", Namespace: "kube-system", Name: "kube-proxy"},
			Summary:  fmt.Sprintf("kube-proxy mode: %s", mode),
			Detail:   fmt.Sprintf("proxyMode=%s", mode),
		})
	}

	// List kube-proxy pods to find unhealthy ones
	podList, err := t.Clients.Dynamic.Resource(podsGVR).Namespace("kube-system").List(ctx, metav1.ListOptions{
		LabelSelector: "k8s-app=kube-proxy",
	})
	if err == nil {
		for _, pod := range podList.Items {
			phase, _, _ := unstructured.NestedString(pod.Object, "status", "phase")
			node, _, _ := unstructured.NestedString(pod.Object, "spec", "nodeName")

			if phase != "Running" {
				findings = append(findings, types.DiagnosticFinding{
					Severity:   types.SeverityWarning,
					Category:   types.CategoryConnectivity,
					Resource:   &types.ResourceRef{Kind: "Pod", Namespace: "kube-system", Name: pod.GetName()},
					Summary:    fmt.Sprintf("kube-proxy pod %s on node %s is %s", pod.GetName(), node, phase),
					Detail:     fmt.Sprintf("phase=%s node=%s", phase, node),
					Suggestion: "Check pod events and logs for this kube-proxy instance.",
				})
			}
		}
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, "kube-system", ""), nil
}
