package tools

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var istioGVRs = map[string]schema.GroupVersionResource{
	"VirtualService":      {Group: "networking.istio.io", Version: "v1beta1", Resource: "virtualservices"},
	"DestinationRule":     {Group: "networking.istio.io", Version: "v1beta1", Resource: "destinationrules"},
	"AuthorizationPolicy": {Group: "security.istio.io", Version: "v1beta1", Resource: "authorizationpolicies"},
	"PeerAuthentication":  {Group: "security.istio.io", Version: "v1beta1", Resource: "peerauthentications"},
}

// --- list_istio_resources ---

type ListIstioResourcesTool struct{ BaseTool }

func (t *ListIstioResourcesTool) Name() string        { return "list_istio_resources" }
func (t *ListIstioResourcesTool) Description() string  { return "List Istio resources (VirtualService, DestinationRule, AuthorizationPolicy, PeerAuthentication)" }
func (t *ListIstioResourcesTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"kind": map[string]interface{}{
				"type":        "string",
				"description": "Resource kind: VirtualService, DestinationRule, AuthorizationPolicy, PeerAuthentication",
				"enum":        []string{"VirtualService", "DestinationRule", "AuthorizationPolicy", "PeerAuthentication"},
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace (empty for all namespaces)",
			},
		},
		"required": []string{"kind"},
	}
}

func (t *ListIstioResourcesTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	kind := getStringArg(args, "kind", "")
	ns := getStringArg(args, "namespace", "")

	gvr, ok := istioGVRs[kind]
	if !ok {
		return nil, fmt.Errorf("unsupported Istio resource kind: %s", kind)
	}

	var list *unstructured.UnstructuredList
	var err error
	if ns == "" {
		list, err = t.Clients.Dynamic.Resource(gvr).List(ctx, metav1.ListOptions{})
	} else {
		list, err = t.Clients.Dynamic.Resource(gvr).Namespace(ns).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list %s: %w", kind, err)
	}

	resources := make([]map[string]interface{}, 0, len(list.Items))
	for _, item := range list.Items {
		spec, _, _ := unstructured.NestedMap(item.Object, "spec")
		summary := map[string]interface{}{
			"name":      item.GetName(),
			"namespace": item.GetNamespace(),
		}

		switch kind {
		case "VirtualService":
			hosts, _, _ := unstructured.NestedStringSlice(item.Object, "spec", "hosts")
			gateways, _, _ := unstructured.NestedStringSlice(item.Object, "spec", "gateways")
			http, _, _ := unstructured.NestedSlice(spec, "http")
			summary["hosts"] = hosts
			summary["gateways"] = gateways
			summary["httpRouteCount"] = len(http)
		case "DestinationRule":
			host, _, _ := unstructured.NestedString(item.Object, "spec", "host")
			subsets, _, _ := unstructured.NestedSlice(spec, "subsets")
			summary["host"] = host
			summary["subsetCount"] = len(subsets)
		case "AuthorizationPolicy":
			action, _, _ := unstructured.NestedString(item.Object, "spec", "action")
			rules, _, _ := unstructured.NestedSlice(spec, "rules")
			summary["action"] = action
			summary["ruleCount"] = len(rules)
		case "PeerAuthentication":
			mode, _, _ := unstructured.NestedString(item.Object, "spec", "mtls", "mode")
			summary["mtlsMode"] = mode
		}

		resources = append(resources, summary)
	}

	return NewResponse(t.Cfg, t.Name(), map[string]interface{}{
		"kind":      kind,
		"count":     len(resources),
		"resources": resources,
	}), nil
}

// --- get_istio_resource ---

type GetIstioResourceTool struct{ BaseTool }

func (t *GetIstioResourceTool) Name() string        { return "get_istio_resource" }
func (t *GetIstioResourceTool) Description() string  { return "Get full Istio resource detail with spec" }
func (t *GetIstioResourceTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"kind": map[string]interface{}{
				"type":        "string",
				"description": "Resource kind: VirtualService, DestinationRule, AuthorizationPolicy, PeerAuthentication",
				"enum":        []string{"VirtualService", "DestinationRule", "AuthorizationPolicy", "PeerAuthentication"},
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Resource name",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace",
			},
		},
		"required": []string{"kind", "name", "namespace"},
	}
}

func (t *GetIstioResourceTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	kind := getStringArg(args, "kind", "")
	name := getStringArg(args, "name", "")
	ns := getStringArg(args, "namespace", "default")

	gvr, ok := istioGVRs[kind]
	if !ok {
		return nil, fmt.Errorf("unsupported Istio resource kind: %s", kind)
	}

	resource, err := t.Clients.Dynamic.Resource(gvr).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get %s %s/%s: %w", kind, ns, name, err)
	}

	spec, _, _ := unstructured.NestedMap(resource.Object, "spec")
	status, _, _ := unstructured.NestedMap(resource.Object, "status")

	return NewResponse(t.Cfg, t.Name(), map[string]interface{}{
		"kind":      kind,
		"name":      name,
		"namespace": ns,
		"spec":      spec,
		"status":    status,
	}), nil
}

// --- check_sidecar_injection ---

type CheckSidecarInjectionTool struct{ BaseTool }

func (t *CheckSidecarInjectionTool) Name() string        { return "check_sidecar_injection" }
func (t *CheckSidecarInjectionTool) Description() string  { return "Check Istio sidecar injection status for all deployments in a namespace" }
func (t *CheckSidecarInjectionTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace to check",
			},
		},
		"required": []string{"namespace"},
	}
}

var deploymentsGVR = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

func (t *CheckSidecarInjectionTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	ns := getStringArg(args, "namespace", "default")

	// Check namespace label
	nsObj, err := t.Clients.Dynamic.Resource(schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}).Get(ctx, ns, metav1.GetOptions{})
	nsInjectionLabel := ""
	if err == nil {
		labels := nsObj.GetLabels()
		nsInjectionLabel = labels["istio-injection"]
		if nsInjectionLabel == "" {
			nsInjectionLabel = labels["istio.io/rev"]
		}
	}

	// List deployments
	depList, err := t.Clients.Dynamic.Resource(deploymentsGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments in %s: %w", ns, err)
	}

	deployments := make([]map[string]interface{}, 0, len(depList.Items))
	for _, dep := range depList.Items {
		annotations := dep.GetAnnotations()
		sidecarInject := annotations["sidecar.istio.io/inject"]

		// Check template annotations
		templateAnnotations, _, _ := unstructured.NestedStringMap(dep.Object, "spec", "template", "metadata", "annotations")
		if templateAnnotations["sidecar.istio.io/inject"] != "" {
			sidecarInject = templateAnnotations["sidecar.istio.io/inject"]
		}

		// Check if pods actually have istio-proxy container
		selector, _, _ := unstructured.NestedMap(dep.Object, "spec", "selector", "matchLabels")
		hasSidecar := false
		if len(selector) > 0 {
			labelSelector := ""
			for k, v := range selector {
				if labelSelector != "" {
					labelSelector += ","
				}
				if vs, ok := v.(string); ok {
					labelSelector += k + "=" + vs
				}
			}
			podList, podErr := t.Clients.Dynamic.Resource(podsGVR).Namespace(ns).List(ctx, metav1.ListOptions{
				LabelSelector: labelSelector,
				Limit:         1,
			})
			if podErr == nil && len(podList.Items) > 0 {
				containers, _, _ := unstructured.NestedSlice(podList.Items[0].Object, "spec", "containers")
				for _, c := range containers {
					if cm, ok := c.(map[string]interface{}); ok {
						if name, ok := cm["name"].(string); ok && name == "istio-proxy" {
							hasSidecar = true
							break
						}
					}
				}
			}
		}

		deployments = append(deployments, map[string]interface{}{
			"name":                  dep.GetName(),
			"sidecarInjectAnnotation": sidecarInject,
			"hasSidecar":            hasSidecar,
		})
	}

	return NewResponse(t.Cfg, t.Name(), map[string]interface{}{
		"namespace":           ns,
		"namespaceInjection":  nsInjectionLabel,
		"deploymentCount":     len(deployments),
		"deployments":         deployments,
	}), nil
}

// --- check_istio_mtls ---

type CheckIstioMTLSTool struct{ BaseTool }

func (t *CheckIstioMTLSTool) Name() string        { return "check_istio_mtls" }
func (t *CheckIstioMTLSTool) Description() string  { return "Check mTLS mode per namespace: PeerAuthentication policies and DestinationRule TLS settings" }
func (t *CheckIstioMTLSTool) InputSchema() map[string]interface{} {
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

func (t *CheckIstioMTLSTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	ns := getStringArg(args, "namespace", "")

	// Get PeerAuthentication policies
	var paList *unstructured.UnstructuredList
	var err error
	paGVR := istioGVRs["PeerAuthentication"]
	if ns == "" {
		paList, err = t.Clients.Dynamic.Resource(paGVR).List(ctx, metav1.ListOptions{})
	} else {
		paList, err = t.Clients.Dynamic.Resource(paGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list PeerAuthentication: %w", err)
	}

	paPolicies := make([]map[string]interface{}, 0, len(paList.Items))
	for _, item := range paList.Items {
		mode, _, _ := unstructured.NestedString(item.Object, "spec", "mtls", "mode")
		selector, _, _ := unstructured.NestedMap(item.Object, "spec", "selector")
		paPolicies = append(paPolicies, map[string]interface{}{
			"name":      item.GetName(),
			"namespace": item.GetNamespace(),
			"mtlsMode":  mode,
			"selector":  selector,
		})
	}

	// Get DestinationRule TLS settings
	var drList *unstructured.UnstructuredList
	drGVR := istioGVRs["DestinationRule"]
	if ns == "" {
		drList, err = t.Clients.Dynamic.Resource(drGVR).List(ctx, metav1.ListOptions{})
	} else {
		drList, err = t.Clients.Dynamic.Resource(drGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list DestinationRule: %w", err)
	}

	drPolicies := make([]map[string]interface{}, 0, len(drList.Items))
	for _, item := range drList.Items {
		host, _, _ := unstructured.NestedString(item.Object, "spec", "host")
		tlsMode, _, _ := unstructured.NestedString(item.Object, "spec", "trafficPolicy", "tls", "mode")
		drPolicies = append(drPolicies, map[string]interface{}{
			"name":      item.GetName(),
			"namespace": item.GetNamespace(),
			"host":      host,
			"tlsMode":   tlsMode,
		})
	}

	return NewResponse(t.Cfg, t.Name(), map[string]interface{}{
		"peerAuthentications": paPolicies,
		"destinationRules":    drPolicies,
	}), nil
}
