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

var ingressGVR = schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"}

// --- list_ingresses ---

type ListIngressesTool struct{ BaseTool }

func (t *ListIngressesTool) Name() string        { return "list_ingresses" }
func (t *ListIngressesTool) Description() string  { return "List Ingress resources with hosts, paths, backends, and TLS configuration" }
func (t *ListIngressesTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace (empty for all namespaces)",
			},
		},
	}
}

func (t *ListIngressesTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	ns := getStringArg(args, "namespace", "")

	var list *unstructured.UnstructuredList
	var err error
	if ns == "" {
		list, err = t.Clients.Dynamic.Resource(ingressGVR).List(ctx, metav1.ListOptions{})
	} else {
		list, err = t.Clients.Dynamic.Resource(ingressGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list ingresses: %w", err)
	}

	findings := make([]types.DiagnosticFinding, 0, len(list.Items))
	for _, item := range list.Items {
		hosts, paths, hasTLS := summarizeIngressRules(&item)
		ingressClass, _, _ := unstructured.NestedString(item.Object, "spec", "ingressClassName")

		tlsStr := "none"
		if hasTLS {
			tlsStr = "enabled"
		}

		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryRouting,
			Resource: &types.ResourceRef{
				Kind:       "Ingress",
				Namespace:  item.GetNamespace(),
				Name:       item.GetName(),
				APIVersion: "networking.k8s.io/v1",
			},
			Summary: fmt.Sprintf("%s/%s hosts=[%s] paths=%d tls=%s class=%s",
				item.GetNamespace(), item.GetName(), strings.Join(hosts, ","), len(paths), tlsStr, ingressClass),
			Detail: fmt.Sprintf("hosts=%v paths=%v ingressClassName=%s tls=%v", hosts, paths, ingressClass, hasTLS),
		})
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, ""), nil
}

// --- get_ingress ---

type GetIngressTool struct{ BaseTool }

func (t *GetIngressTool) Name() string        { return "get_ingress" }
func (t *GetIngressTool) Description() string  { return "Get full Ingress spec with rules, TLS settings, status, and backend validation" }
func (t *GetIngressTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Ingress name",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace",
			},
		},
		"required": []string{"name", "namespace"},
	}
}

func (t *GetIngressTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	name := getStringArg(args, "name", "")
	ns := getStringArg(args, "namespace", "default")

	ing, err := t.Clients.Dynamic.Resource(ingressGVR).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ingress %s/%s: %w", ns, name, err)
	}

	ref := &types.ResourceRef{Kind: "Ingress", Namespace: ns, Name: name, APIVersion: "networking.k8s.io/v1"}
	findings := make([]types.DiagnosticFinding, 0, 6)

	ingressClass, _, _ := unstructured.NestedString(ing.Object, "spec", "ingressClassName")

	// Overview
	hosts, paths, hasTLS := summarizeIngressRules(ing)
	findings = append(findings, types.DiagnosticFinding{
		Severity: types.SeverityInfo,
		Category: types.CategoryRouting,
		Resource: ref,
		Summary:  fmt.Sprintf("%s/%s hosts=[%s] paths=%d tls=%v class=%s", ns, name, strings.Join(hosts, ","), len(paths), hasTLS, ingressClass),
		Detail:   fmt.Sprintf("ingressClassName=%s hosts=%v", ingressClass, hosts),
	})

	// TLS info
	tlsSlice, _, _ := unstructured.NestedSlice(ing.Object, "spec", "tls")
	for _, tls := range tlsSlice {
		if tm, ok := tls.(map[string]interface{}); ok {
			tlsHosts, _ := tm["hosts"].([]interface{})
			secretName, _ := tm["secretName"].(string)
			hostNames := make([]string, 0, len(tlsHosts))
			for _, h := range tlsHosts {
				if hs, ok := h.(string); ok {
					hostNames = append(hostNames, hs)
				}
			}
			findings = append(findings, types.DiagnosticFinding{
				Severity: types.SeverityInfo,
				Category: types.CategoryTLS,
				Resource: ref,
				Summary:  fmt.Sprintf("TLS: hosts=[%s] secret=%s", strings.Join(hostNames, ","), secretName),
				Detail:   fmt.Sprintf("tlsHosts=%v secretName=%s", hostNames, secretName),
			})
		}
	}

	// Rules with backend validation
	rules, _, _ := unstructured.NestedSlice(ing.Object, "spec", "rules")
	for _, rule := range rules {
		rm, ok := rule.(map[string]interface{})
		if !ok {
			continue
		}
		host, _ := rm["host"].(string)
		httpPaths, _, _ := unstructured.NestedSlice(rm, "http", "paths")
		for _, p := range httpPaths {
			pm, ok := p.(map[string]interface{})
			if !ok {
				continue
			}
			path, _ := pm["path"].(string)
			pathType, _ := pm["pathType"].(string)
			svcName, _, _ := unstructured.NestedString(pm, "backend", "service", "name")
			svcPort, _, _ := unstructured.NestedFloat64(pm, "backend", "service", "port", "number")
			svcPortName, _, _ := unstructured.NestedString(pm, "backend", "service", "port", "name")

			portStr := svcPortName
			if portStr == "" {
				portStr = fmt.Sprintf("%.0f", svcPort)
			}

			findings = append(findings, types.DiagnosticFinding{
				Severity: types.SeverityInfo,
				Category: types.CategoryRouting,
				Resource: ref,
				Summary:  fmt.Sprintf("rule: host=%s path=%s(%s) -> %s:%s", host, path, pathType, svcName, portStr),
				Detail:   fmt.Sprintf("host=%s path=%s pathType=%s backendService=%s backendPort=%s", host, path, pathType, svcName, portStr),
			})

			// Validate backend service exists
			if svcName != "" {
				_, svcErr := t.Clients.Dynamic.Resource(servicesGVR).Namespace(ns).Get(ctx, svcName, metav1.GetOptions{})
				if svcErr != nil {
					findings = append(findings, types.DiagnosticFinding{
						Severity:   types.SeverityWarning,
						Category:   types.CategoryRouting,
						Resource:   ref,
						Summary:    fmt.Sprintf("backend service %s/%s not found", ns, svcName),
						Detail:     fmt.Sprintf("referencedService=%s error=%v", svcName, svcErr),
						Suggestion: fmt.Sprintf("Create service %s in namespace %s, or fix the Ingress backend reference.", svcName, ns),
					})
				}
			}
		}
	}

	// Status - load balancer
	lbIngress, _, _ := unstructured.NestedSlice(ing.Object, "status", "loadBalancer", "ingress")
	if len(lbIngress) > 0 {
		lbAddrs := make([]string, 0, len(lbIngress))
		for _, lb := range lbIngress {
			if lm, ok := lb.(map[string]interface{}); ok {
				if ip, ok := lm["ip"].(string); ok && ip != "" {
					lbAddrs = append(lbAddrs, ip)
				}
				if hostname, ok := lm["hostname"].(string); ok && hostname != "" {
					lbAddrs = append(lbAddrs, hostname)
				}
			}
		}
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityOK,
			Category: types.CategoryRouting,
			Resource: ref,
			Summary:  fmt.Sprintf("load balancer: [%s]", strings.Join(lbAddrs, ", ")),
			Detail:   fmt.Sprintf("loadBalancerAddresses=%v", lbAddrs),
		})
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, ""), nil
}

// summarizeIngressRules extracts hosts, paths, and TLS presence from an Ingress object.
func summarizeIngressRules(ing *unstructured.Unstructured) (hosts []string, paths []string, hasTLS bool) {
	rules, _, _ := unstructured.NestedSlice(ing.Object, "spec", "rules")
	hostSet := make(map[string]struct{})
	for _, rule := range rules {
		if rm, ok := rule.(map[string]interface{}); ok {
			if host, ok := rm["host"].(string); ok && host != "" {
				hostSet[host] = struct{}{}
			}
			httpPaths, _, _ := unstructured.NestedSlice(rm, "http", "paths")
			for _, p := range httpPaths {
				if pm, ok := p.(map[string]interface{}); ok {
					if path, ok := pm["path"].(string); ok {
						paths = append(paths, path)
					}
				}
			}
		}
	}
	for h := range hostSet {
		hosts = append(hosts, h)
	}

	tlsSlice, _, _ := unstructured.NestedSlice(ing.Object, "spec", "tls")
	hasTLS = len(tlsSlice) > 0
	return
}
