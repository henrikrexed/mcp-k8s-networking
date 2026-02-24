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

var servicesGVR = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}
var endpointsGVR = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "endpoints"}
var podsGVR = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}

// --- list_services ---

type ListServicesTool struct{ BaseTool }

func (t *ListServicesTool) Name() string        { return "list_services" }
func (t *ListServicesTool) Description() string  { return "List Kubernetes services with type, clusterIP, ports, and selector" }
func (t *ListServicesTool) InputSchema() map[string]interface{} {
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

func (t *ListServicesTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	ns := getStringArg(args, "namespace", "")

	var list *unstructured.UnstructuredList
	var err error
	if ns == "" {
		list, err = t.Clients.Dynamic.Resource(servicesGVR).List(ctx, metav1.ListOptions{})
	} else {
		list, err = t.Clients.Dynamic.Resource(servicesGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	findings := make([]types.DiagnosticFinding, 0, len(list.Items))
	for _, item := range list.Items {
		spec, _, _ := unstructured.NestedMap(item.Object, "spec")
		svcType, _, _ := unstructured.NestedString(item.Object, "spec", "type")
		clusterIP, _, _ := unstructured.NestedString(item.Object, "spec", "clusterIP")
		selector, _, _ := unstructured.NestedStringMap(item.Object, "spec", "selector")
		ports, _, _ := unstructured.NestedSlice(spec, "ports")

		portNames := make([]string, 0, len(ports))
		for _, p := range ports {
			if pm, ok := p.(map[string]interface{}); ok {
				port := fmt.Sprintf("%v/%v", pm["port"], pm["protocol"])
				portNames = append(portNames, port)
			}
		}

		summary := fmt.Sprintf("%s/%s type=%s clusterIP=%s ports=[%s]",
			item.GetNamespace(), item.GetName(), svcType, clusterIP, strings.Join(portNames, ","))

		detail := fmt.Sprintf("selector=%v ports=%v", selector, portNames)

		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryRouting,
			Resource: &types.ResourceRef{
				Kind:      "Service",
				Namespace: item.GetNamespace(),
				Name:      item.GetName(),
			},
			Summary:  summary,
			Detail:   detail,
		})
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, ""), nil
}

// --- get_service ---

type GetServiceTool struct{ BaseTool }

func (t *GetServiceTool) Name() string        { return "get_service" }
func (t *GetServiceTool) Description() string  { return "Get detailed service info including endpoints and matching pod status" }
func (t *GetServiceTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Service name",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace",
			},
		},
		"required": []string{"name", "namespace"},
	}
}

func (t *GetServiceTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	name := getStringArg(args, "name", "")
	ns := getStringArg(args, "namespace", "default")

	svc, err := t.Clients.Dynamic.Resource(servicesGVR).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get service %s/%s: %w", ns, name, err)
	}

	spec, _, _ := unstructured.NestedMap(svc.Object, "spec")
	svcType, _, _ := unstructured.NestedString(svc.Object, "spec", "type")
	clusterIP, _, _ := unstructured.NestedString(svc.Object, "spec", "clusterIP")
	selector, _, _ := unstructured.NestedStringMap(svc.Object, "spec", "selector")
	ports, _, _ := unstructured.NestedSlice(spec, "ports")

	findings := make([]types.DiagnosticFinding, 0, 4)
	ref := &types.ResourceRef{Kind: "Service", Namespace: ns, Name: name}

	// Service info finding
	portNames := make([]string, 0, len(ports))
	for _, p := range ports {
		if pm, ok := p.(map[string]interface{}); ok {
			portNames = append(portNames, fmt.Sprintf("%v:%v/%v", pm["port"], pm["targetPort"], pm["protocol"]))
		}
	}
	findings = append(findings, types.DiagnosticFinding{
		Severity: types.SeverityInfo,
		Category: types.CategoryRouting,
		Resource: ref,
		Summary:  fmt.Sprintf("%s/%s type=%s clusterIP=%s ports=[%s]", ns, name, svcType, clusterIP, strings.Join(portNames, ",")),
		Detail:   fmt.Sprintf("selector=%v", selector),
	})

	// Endpoint finding
	ep, epErr := t.Clients.Dynamic.Resource(endpointsGVR).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
	if epErr == nil {
		subsets, _, _ := unstructured.NestedSlice(ep.Object, "subsets")
		readyCount, notReadyCount := 0, 0
		for _, s := range subsets {
			if sm, ok := s.(map[string]interface{}); ok {
				if addrs, ok := sm["addresses"].([]interface{}); ok {
					readyCount += len(addrs)
				}
				if addrs, ok := sm["notReadyAddresses"].([]interface{}); ok {
					notReadyCount += len(addrs)
				}
			}
		}

		severity := types.SeverityOK
		if readyCount == 0 {
			severity = types.SeverityWarning
		}
		findings = append(findings, types.DiagnosticFinding{
			Severity: severity,
			Category: types.CategoryRouting,
			Resource: &types.ResourceRef{Kind: "Endpoints", Namespace: ns, Name: name},
			Summary:  fmt.Sprintf("endpoints: %d ready, %d not-ready", readyCount, notReadyCount),
			Detail:   fmt.Sprintf("readyAddresses=%d notReadyAddresses=%d", readyCount, notReadyCount),
		})
	}

	// Matching pods finding
	if len(selector) > 0 {
		labelSelector := ""
		for k, v := range selector {
			if labelSelector != "" {
				labelSelector += ","
			}
			labelSelector += k + "=" + v
		}
		podList, podErr := t.Clients.Dynamic.Resource(podsGVR).Namespace(ns).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if podErr == nil {
			if len(podList.Items) == 0 {
				findings = append(findings, types.DiagnosticFinding{
					Severity:   types.SeverityWarning,
					Category:   types.CategoryRouting,
					Resource:   ref,
					Summary:    fmt.Sprintf("service %s/%s selector matches no pods", ns, name),
					Detail:     fmt.Sprintf("selector=%v matched 0 pods", selector),
					Suggestion: "Check that the selector labels match your pod template labels. Verify pods are running in the correct namespace.",
				})
			} else {
				podSummaries := make([]string, 0, len(podList.Items))
				for _, pod := range podList.Items {
					phase, _, _ := unstructured.NestedString(pod.Object, "status", "phase")
					podSummaries = append(podSummaries, fmt.Sprintf("%s(%s)", pod.GetName(), phase))
				}
				findings = append(findings, types.DiagnosticFinding{
					Severity: types.SeverityInfo,
					Category: types.CategoryRouting,
					Resource: ref,
					Summary:  fmt.Sprintf("%d matching pods: %s", len(podList.Items), strings.Join(podSummaries, ", ")),
					Detail:   fmt.Sprintf("selector=%v podCount=%d", selector, len(podList.Items)),
				})
			}
		}
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, ""), nil
}
