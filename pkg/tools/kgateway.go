package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/isitobservable/k8s-networking-mcp/pkg/types"
)

// kgateway GVR definitions.
var (
	gatewayParamsGVR = schema.GroupVersionResource{Group: "kgateway.dev", Version: "v1alpha1", Resource: "gatewayparameters"}
	routeOptionGVR   = schema.GroupVersionResource{Group: "gateway.kgateway.dev", Version: "v1alpha1", Resource: "routeoptions"}
	vhostOptionGVR   = schema.GroupVersionResource{Group: "gateway.kgateway.dev", Version: "v1alpha1", Resource: "virtualhostoptions"}
)

type kgatewayKindInfo struct {
	gvr      schema.GroupVersionResource
	apiGroup string
}

var kgatewayKindGVRs = map[string]kgatewayKindInfo{
	"GatewayParameters":  {gvr: gatewayParamsGVR, apiGroup: "kgateway.dev"},
	"RouteOption":        {gvr: routeOptionGVR, apiGroup: "gateway.kgateway.dev"},
	"VirtualHostOption":  {gvr: vhostOptionGVR, apiGroup: "gateway.kgateway.dev"},
}

// --- list_kgateway_resources ---

type ListKgatewayResourcesTool struct{ BaseTool }

func (t *ListKgatewayResourcesTool) Name() string { return "list_kgateway_resources" }
func (t *ListKgatewayResourcesTool) Description() string {
	return "List kgateway resources (GatewayParameters, RouteOption, VirtualHostOption) with key summary fields"
}
func (t *ListKgatewayResourcesTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"kind": map[string]interface{}{
				"type":        "string",
				"description": "Resource kind: GatewayParameters, RouteOption, VirtualHostOption",
				"enum":        []string{"GatewayParameters", "RouteOption", "VirtualHostOption"},
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace (empty for all namespaces)",
			},
		},
		"required": []string{"kind"},
	}
}

func (t *ListKgatewayResourcesTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	kind := getStringArg(args, "kind", "")
	ns := getStringArg(args, "namespace", "")

	info, ok := kgatewayKindGVRs[kind]
	if !ok {
		return nil, &types.MCPError{
			Code:    types.ErrCodeInvalidInput,
			Tool:    t.Name(),
			Message: fmt.Sprintf("unsupported kgateway resource kind: %s", kind),
		}
	}

	var list *unstructured.UnstructuredList
	var err error
	if ns == "" {
		list, err = t.Clients.Dynamic.Resource(info.gvr).List(ctx, metav1.ListOptions{})
	} else {
		list, err = t.Clients.Dynamic.Resource(info.gvr).Namespace(ns).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, &types.MCPError{
			Code:    types.ErrCodeCRDNotAvailable,
			Tool:    t.Name(),
			Message: fmt.Sprintf("failed to list %s", kind),
			Detail:  fmt.Sprintf("%s: %v", info.apiGroup, err),
		}
	}

	findings := make([]types.DiagnosticFinding, 0, len(list.Items))
	for _, item := range list.Items {
		ref := &types.ResourceRef{
			Kind:       kind,
			Namespace:  item.GetNamespace(),
			Name:       item.GetName(),
			APIVersion: info.apiGroup,
		}

		summary, detail := kgatewayResourceSummary(kind, &item)

		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryMesh,
			Resource: ref,
			Summary:  summary,
			Detail:   detail,
		})
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, "kgateway"), nil
}

// kgatewayResourceSummary returns a compact summary and optional detail for a kgateway resource.
func kgatewayResourceSummary(kind string, item *unstructured.Unstructured) (string, string) {
	ns := item.GetNamespace()
	name := item.GetName()

	switch kind {
	case "GatewayParameters":
		// GatewayParameters defines infrastructure configuration for a Gateway
		kube, _, _ := unstructured.NestedMap(item.Object, "spec", "kube")
		selfManaged, _, _ := unstructured.NestedBool(item.Object, "spec", "selfManaged")
		summary := fmt.Sprintf("%s/%s", ns, name)
		if selfManaged {
			summary += " selfManaged=true"
		}
		if kube != nil {
			if deployment, ok := kube["deployment"].(map[string]interface{}); ok {
				if replicas, ok := deployment["replicas"].(float64); ok {
					summary += fmt.Sprintf(" replicas=%d", int(replicas))
				}
			}
			if svcType, _, _ := unstructured.NestedString(kube, "service", "type"); svcType != "" {
				summary += fmt.Sprintf(" serviceType=%s", svcType)
			}
		}
		return summary, ""

	case "RouteOption":
		// RouteOption attaches options to HTTPRoute rules
		targetRef := kgatewayTargetRefSummary(item)
		summary := fmt.Sprintf("%s/%s", ns, name)
		if targetRef != "" {
			summary += " " + targetRef
		}

		// Summarize option types present
		options, _, _ := unstructured.NestedMap(item.Object, "spec", "options")
		if len(options) > 0 {
			optionKeys := make([]string, 0, len(options))
			for k := range options {
				optionKeys = append(optionKeys, k)
			}
			summary += fmt.Sprintf(" options=[%s]", strings.Join(optionKeys, ", "))
		}
		return summary, ""

	case "VirtualHostOption":
		// VirtualHostOption attaches options at the virtual host level
		targetRef := kgatewayTargetRefSummary(item)
		summary := fmt.Sprintf("%s/%s", ns, name)
		if targetRef != "" {
			summary += " " + targetRef
		}

		options, _, _ := unstructured.NestedMap(item.Object, "spec", "options")
		if len(options) > 0 {
			optionKeys := make([]string, 0, len(options))
			for k := range options {
				optionKeys = append(optionKeys, k)
			}
			summary += fmt.Sprintf(" options=[%s]", strings.Join(optionKeys, ", "))
		}
		return summary, ""
	}

	return fmt.Sprintf("%s/%s", ns, name), ""
}

// kgatewayTargetRefSummary extracts a targetRef or targetRefs summary from a kgateway resource.
func kgatewayTargetRefSummary(item *unstructured.Unstructured) string {
	// Single targetRef
	targetRef, _, _ := unstructured.NestedMap(item.Object, "spec", "targetRef")
	if targetRef != nil {
		group, _ := targetRef["group"].(string)
		kind, _ := targetRef["kind"].(string)
		name, _ := targetRef["name"].(string)
		ns, _ := targetRef["namespace"].(string)
		ref := fmt.Sprintf("targetRef=%s/%s", kind, name)
		if group != "" {
			ref = fmt.Sprintf("targetRef=%s.%s/%s", kind, group, name)
		}
		if ns != "" {
			ref += fmt.Sprintf(" (ns=%s)", ns)
		}
		return ref
	}

	// Multiple targetRefs
	targetRefs, _, _ := unstructured.NestedSlice(item.Object, "spec", "targetRefs")
	if len(targetRefs) > 0 {
		return fmt.Sprintf("targetRefs=%d", len(targetRefs))
	}

	return ""
}

// --- validate_kgateway_resource ---

type ValidateKgatewayResourceTool struct{ BaseTool }

func (t *ValidateKgatewayResourceTool) Name() string { return "validate_kgateway_resource" }
func (t *ValidateKgatewayResourceTool) Description() string {
	return "Validate kgateway resources: upstream references, route option conflicts, GatewayParameters references, and status conditions"
}
func (t *ValidateKgatewayResourceTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"kind": map[string]interface{}{
				"type":        "string",
				"description": "Resource kind: GatewayParameters, RouteOption, VirtualHostOption",
				"enum":        []string{"GatewayParameters", "RouteOption", "VirtualHostOption"},
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

func (t *ValidateKgatewayResourceTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	kind := getStringArg(args, "kind", "")
	name := getStringArg(args, "name", "")
	ns := getStringArg(args, "namespace", "default")

	info, ok := kgatewayKindGVRs[kind]
	if !ok {
		return nil, &types.MCPError{
			Code:    types.ErrCodeInvalidInput,
			Tool:    t.Name(),
			Message: fmt.Sprintf("unsupported kgateway resource kind: %s", kind),
		}
	}

	resource, err := t.Clients.Dynamic.Resource(info.gvr).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, &types.MCPError{
			Code:    types.ErrCodeInvalidInput,
			Tool:    t.Name(),
			Message: fmt.Sprintf("failed to get %s %s/%s", kind, ns, name),
			Detail:  err.Error(),
		}
	}

	ref := &types.ResourceRef{
		Kind:       kind,
		Namespace:  ns,
		Name:       name,
		APIVersion: info.apiGroup,
	}

	var findings []types.DiagnosticFinding

	// Resource summary with spec in detail
	summary, _ := kgatewayResourceSummary(kind, resource)
	spec, _, _ := unstructured.NestedMap(resource.Object, "spec")
	specJSON, jsonErr := json.MarshalIndent(spec, "", "  ")
	if jsonErr != nil {
		slog.Warn("kgateway: failed to marshal spec to JSON", "kind", kind, "name", name, "error", jsonErr)
		specJSON = []byte(fmt.Sprintf("<failed to serialize spec: %v>", jsonErr))
	}

	findings = append(findings, types.DiagnosticFinding{
		Severity: types.SeverityInfo,
		Category: types.CategoryMesh,
		Resource: ref,
		Summary:  summary,
		Detail:   string(specJSON),
	})

	// Check status conditions
	findings = append(findings, kgatewayStatusFindings(resource, ref)...)

	// Kind-specific validation
	switch kind {
	case "GatewayParameters":
		findings = append(findings, t.validateGatewayParameters(ctx, resource, ref)...)
	case "RouteOption":
		findings = append(findings, t.validateRouteOption(ctx, resource, ref, ns)...)
	case "VirtualHostOption":
		findings = append(findings, t.validateVirtualHostOption(ctx, resource, ref, ns)...)
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, "kgateway"), nil
}

// kgatewayStatusFindings extracts findings from status.conditions on a kgateway resource.
func kgatewayStatusFindings(resource *unstructured.Unstructured, ref *types.ResourceRef) []types.DiagnosticFinding {
	var findings []types.DiagnosticFinding

	conditions, _, _ := unstructured.NestedSlice(resource.Object, "status", "conditions")
	for _, c := range conditions {
		cm, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		condType, _ := cm["type"].(string)
		condStatus, _ := cm["status"].(string)
		reason, _ := cm["reason"].(string)
		message, _ := cm["message"].(string)

		if condStatus == "False" {
			findings = append(findings, types.DiagnosticFinding{
				Severity: types.SeverityWarning,
				Category: types.CategoryMesh,
				Resource: ref,
				Summary:  fmt.Sprintf("Condition %s=%s reason=%s", condType, condStatus, reason),
				Detail:   message,
			})
		}

		// Check for rejected/errored status
		if condType == "Accepted" && condStatus == "False" {
			findings = append(findings, types.DiagnosticFinding{
				Severity:   types.SeverityCritical,
				Category:   types.CategoryMesh,
				Resource:   ref,
				Summary:    fmt.Sprintf("Resource not accepted: reason=%s", reason),
				Detail:     message,
				Suggestion: "Review the resource configuration and check kgateway controller logs for details",
			})
		}
	}

	return findings
}

// validateGatewayParameters checks GatewayParameters for misconfigurations.
func (t *ValidateKgatewayResourceTool) validateGatewayParameters(ctx context.Context, resource *unstructured.Unstructured, ref *types.ResourceRef) []types.DiagnosticFinding {
	var findings []types.DiagnosticFinding

	// Check if referenced by any Gateway
	gatewayAPIGVR := schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gateways"}
	gateways, err := t.Clients.Dynamic.Resource(gatewayAPIGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Debug("kgateway: skipping Gateway reference check", "error", err)
	} else {
		referenced := false
		for _, gw := range gateways.Items {
			// kgateway uses parametersRef on Gateway infrastructure
			infraParams, _, _ := unstructured.NestedMap(gw.Object, "spec", "infrastructure", "parametersRef")
			if infraParams != nil {
				refGroup, _ := infraParams["group"].(string)
				refName, _ := infraParams["name"].(string)
				if refGroup == "kgateway.dev" && refName == resource.GetName() {
					referenced = true
					break
				}
			}
			// Also check annotations for parametersRef
			annotations := gw.GetAnnotations()
			if annotations["kgateway.dev/gateway-parameters-name"] == resource.GetName() {
				referenced = true
				break
			}
		}
		if !referenced {
			findings = append(findings, types.DiagnosticFinding{
				Severity:   types.SeverityWarning,
				Category:   types.CategoryMesh,
				Resource:   ref,
				Summary:    fmt.Sprintf("GatewayParameters %s/%s is not referenced by any Gateway", resource.GetNamespace(), resource.GetName()),
				Suggestion: "Reference this GatewayParameters from a Gateway's infrastructure.parametersRef or via annotation",
			})
		}
	}

	// Validate kube deployment settings
	replicas, replicasFound, _ := unstructured.NestedFloat64(resource.Object, "spec", "kube", "deployment", "replicas")
	if replicasFound && replicas <= 0 {
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityWarning,
			Category:   types.CategoryMesh,
			Resource:   ref,
			Summary:    fmt.Sprintf("GatewayParameters %s/%s has deployment.replicas=%d (non-positive)", resource.GetNamespace(), resource.GetName(), int(replicas)),
			Suggestion: "Set a positive replica count for the Gateway deployment",
		})
	}

	// Check envoy image reference
	envoyImage, _, _ := unstructured.NestedString(resource.Object, "spec", "kube", "envoyContainer", "image", "uri")
	if envoyImage != "" && !strings.Contains(envoyImage, ":") && !strings.Contains(envoyImage, "@") {
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityWarning,
			Category:   types.CategoryMesh,
			Resource:   ref,
			Summary:    fmt.Sprintf("GatewayParameters %s/%s envoy image %q has no tag or digest", resource.GetNamespace(), resource.GetName(), envoyImage),
			Suggestion: "Pin the envoy image to a specific tag or digest for reproducibility",
		})
	}

	// Validate service account reference
	saName, _, _ := unstructured.NestedString(resource.Object, "spec", "kube", "serviceAccount", "name")
	if saName != "" {
		saGVR := schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}
		_, saErr := t.Clients.Dynamic.Resource(saGVR).Namespace(resource.GetNamespace()).Get(ctx, saName, metav1.GetOptions{})
		if saErr != nil {
			findings = append(findings, types.DiagnosticFinding{
				Severity:   types.SeverityWarning,
				Category:   types.CategoryMesh,
				Resource:   ref,
				Summary:    fmt.Sprintf("GatewayParameters %s/%s references ServiceAccount %q which may not exist", resource.GetNamespace(), resource.GetName(), saName),
				Detail:     fmt.Sprintf("ServiceAccount lookup failed: %v", saErr),
				Suggestion: "Create the ServiceAccount or correct the reference",
			})
		}
	}

	return findings
}

// validateRouteOption checks a RouteOption for misconfigurations.
func (t *ValidateKgatewayResourceTool) validateRouteOption(ctx context.Context, resource *unstructured.Unstructured, ref *types.ResourceRef, ns string) []types.DiagnosticFinding {
	var findings []types.DiagnosticFinding

	// Validate targetRef exists
	findings = append(findings, t.validateKgatewayTargetRef(ctx, resource, ref, ns)...)

	// Check for upstream references in options
	findings = append(findings, t.validateUpstreamRefs(ctx, resource, ref, ns)...)

	return findings
}

// validateVirtualHostOption checks a VirtualHostOption for misconfigurations.
func (t *ValidateKgatewayResourceTool) validateVirtualHostOption(ctx context.Context, resource *unstructured.Unstructured, ref *types.ResourceRef, ns string) []types.DiagnosticFinding {
	var findings []types.DiagnosticFinding

	// Validate targetRef exists
	findings = append(findings, t.validateKgatewayTargetRef(ctx, resource, ref, ns)...)

	// Check for upstream references in options
	findings = append(findings, t.validateUpstreamRefs(ctx, resource, ref, ns)...)

	// Check for conflicts with other VirtualHostOptions targeting the same Gateway/listener
	findings = append(findings, t.detectVHostOptionConflicts(ctx, resource, ref, ns)...)

	return findings
}

// validateKgatewayTargetRef verifies that a targetRef points to an existing resource.
func (t *ValidateKgatewayResourceTool) validateKgatewayTargetRef(ctx context.Context, resource *unstructured.Unstructured, ref *types.ResourceRef, ns string) []types.DiagnosticFinding {
	var findings []types.DiagnosticFinding

	targetRef, _, _ := unstructured.NestedMap(resource.Object, "spec", "targetRef")
	if targetRef == nil {
		return findings
	}

	group, _ := targetRef["group"].(string)
	kind, _ := targetRef["kind"].(string)
	name, _ := targetRef["name"].(string)
	targetNs, _ := targetRef["namespace"].(string)
	if targetNs == "" {
		targetNs = ns
	}

	if name == "" {
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityWarning,
			Category:   types.CategoryMesh,
			Resource:   ref,
			Summary:    fmt.Sprintf("%s %s/%s targetRef has no name", ref.Kind, ns, resource.GetName()),
			Suggestion: "Set targetRef.name to the target resource name",
		})
		return findings
	}

	// Resolve GVR for the target
	targetGVR, ok := resolveTargetRefGVR(group, kind)
	if !ok {
		// Unknown target kind — informational only
		return findings
	}

	_, err := t.Clients.Dynamic.Resource(targetGVR).Namespace(targetNs).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityWarning,
			Category:   types.CategoryMesh,
			Resource:   ref,
			Summary:    fmt.Sprintf("%s %s/%s targetRef %s/%s not found in %s", ref.Kind, ns, resource.GetName(), kind, name, targetNs),
			Detail:     fmt.Sprintf("Lookup failed: %v", err),
			Suggestion: "Verify the targetRef points to an existing resource",
		})
	}

	return findings
}

// resolveTargetRefGVR maps a targetRef group/kind to a GVR for lookup.
func resolveTargetRefGVR(group, kind string) (schema.GroupVersionResource, bool) {
	switch {
	case group == "gateway.networking.k8s.io" && kind == "Gateway":
		return schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gateways"}, true
	case group == "gateway.networking.k8s.io" && kind == "HTTPRoute":
		return schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes"}, true
	case group == "" && kind == "Service":
		return servicesGVR, true
	}
	return schema.GroupVersionResource{}, false
}

// validateUpstreamRefs checks if any upstream references in options resolve to existing services.
func (t *ValidateKgatewayResourceTool) validateUpstreamRefs(ctx context.Context, resource *unstructured.Unstructured, ref *types.ResourceRef, ns string) []types.DiagnosticFinding {
	var findings []types.DiagnosticFinding

	options, _, _ := unstructured.NestedMap(resource.Object, "spec", "options")
	if options == nil {
		return findings
	}

	// Check extauth upstream refs
	findings = append(findings, t.checkNestedUpstreamRef(ctx, options, ref, ns, "extauth", "spec.options.extauth")...)

	// Check ratelimit upstream refs
	findings = append(findings, t.checkNestedUpstreamRef(ctx, options, ref, ns, "rateLimitConfigs", "spec.options.rateLimitConfigs")...)

	return findings
}

// checkNestedUpstreamRef looks for upstream references within an options sub-field.
func (t *ValidateKgatewayResourceTool) checkNestedUpstreamRef(ctx context.Context, options map[string]interface{}, ref *types.ResourceRef, ns, fieldName, path string) []types.DiagnosticFinding {
	var findings []types.DiagnosticFinding

	field, ok := options[fieldName]
	if !ok || field == nil {
		return findings
	}

	// Walk the structure looking for upstream references (name/namespace pairs)
	upstreamRefs := extractUpstreamRefs(field, path)
	for _, ur := range upstreamRefs {
		upNs := ur.namespace
		if upNs == "" {
			upNs = ns
		}
		_, svcErr := t.Clients.Dynamic.Resource(servicesGVR).Namespace(upNs).Get(ctx, ur.name, metav1.GetOptions{})
		if svcErr != nil {
			findings = append(findings, types.DiagnosticFinding{
				Severity:   types.SeverityWarning,
				Category:   types.CategoryMesh,
				Resource:   ref,
				Summary:    fmt.Sprintf("Upstream reference %s/%s in %s may not exist", upNs, ur.name, ur.path),
				Detail:     fmt.Sprintf("Service lookup failed: %v", svcErr),
				Suggestion: "Verify the upstream reference points to an existing Service",
			})
		}
	}

	return findings
}

type upstreamRef struct {
	name      string
	namespace string
	path      string
}

// extractUpstreamRefs recursively searches for upstream reference patterns in nested maps.
func extractUpstreamRefs(obj interface{}, path string) []upstreamRef {
	var refs []upstreamRef

	switch v := obj.(type) {
	case map[string]interface{}:
		// Check if this map has name/namespace pattern (upstream ref)
		if name, ok := v["name"].(string); ok {
			if _, hasNs := v["namespace"]; hasNs || len(v) <= 3 {
				nsVal, _ := v["namespace"].(string)
				refs = append(refs, upstreamRef{name: name, namespace: nsVal, path: path})
			}
		}
		// Recurse into upstream or upstreamRef fields
		for _, key := range []string{"upstream", "upstreamRef", "serverRef"} {
			if sub, ok := v[key]; ok {
				refs = append(refs, extractUpstreamRefs(sub, path+"."+key)...)
			}
		}
	case []interface{}:
		for i, item := range v {
			refs = append(refs, extractUpstreamRefs(item, fmt.Sprintf("%s[%d]", path, i))...)
		}
	}

	return refs
}

// detectVHostOptionConflicts checks if multiple VirtualHostOptions target the same Gateway/listener.
func (t *ValidateKgatewayResourceTool) detectVHostOptionConflicts(ctx context.Context, resource *unstructured.Unstructured, ref *types.ResourceRef, ns string) []types.DiagnosticFinding {
	var findings []types.DiagnosticFinding

	// Get our targetRef
	ourTargetRef, _, _ := unstructured.NestedMap(resource.Object, "spec", "targetRef")
	if ourTargetRef == nil {
		return findings
	}

	ourTargetKey := kgatewayTargetKey(ourTargetRef, ns)
	if ourTargetKey == "" {
		return findings
	}

	// List all VirtualHostOptions in the namespace
	vhoList, err := t.Clients.Dynamic.Resource(vhostOptionGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return findings
	}

	var conflictNames []string
	for _, vho := range vhoList.Items {
		if vho.GetName() == resource.GetName() {
			continue
		}
		otherTargetRef, _, _ := unstructured.NestedMap(vho.Object, "spec", "targetRef")
		if otherTargetRef == nil {
			continue
		}
		otherKey := kgatewayTargetKey(otherTargetRef, ns)
		if otherKey == ourTargetKey {
			conflictNames = append(conflictNames, vho.GetName())
		}
	}

	if len(conflictNames) > 0 {
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityWarning,
			Category: types.CategoryMesh,
			Resource: ref,
			Summary:  fmt.Sprintf("VirtualHostOption %s/%s targets the same resource as: %s", ns, resource.GetName(), strings.Join(conflictNames, ", ")),
			Detail:   "Multiple VirtualHostOptions targeting the same Gateway/listener may have conflicting options. kgateway merges them by priority, which can produce unexpected behavior.",
			Suggestion: "Review option precedence or consolidate into a single VirtualHostOption",
		})
	}

	return findings
}

// kgatewayTargetKey returns a deterministic key for a targetRef to detect overlaps.
func kgatewayTargetKey(targetRef map[string]interface{}, defaultNs string) string {
	group, _ := targetRef["group"].(string)
	kind, _ := targetRef["kind"].(string)
	name, _ := targetRef["name"].(string)
	ns, _ := targetRef["namespace"].(string)
	if ns == "" {
		ns = defaultNs
	}
	if name == "" {
		return ""
	}
	sectionName, _ := targetRef["sectionName"].(string)
	key := fmt.Sprintf("%s/%s/%s/%s", group, kind, ns, name)
	if sectionName != "" {
		key += "/" + sectionName
	}
	return key
}

// --- check_kgateway_health ---

type CheckKgatewayHealthTool struct{ BaseTool }

func (t *CheckKgatewayHealthTool) Name() string { return "check_kgateway_health" }
func (t *CheckKgatewayHealthTool) Description() string {
	return "Check kgateway installation health: control plane pod status, resource translation status, and data plane proxy health for kgateway-managed Gateways"
}
func (t *CheckKgatewayHealthTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Namespace where kgateway is installed (default: kgateway-system)",
			},
		},
	}
}

func (t *CheckKgatewayHealthTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	ns := getStringArg(args, "namespace", "kgateway-system")

	var findings []types.DiagnosticFinding

	// 1. Control plane pod health
	findings = append(findings, t.checkControlPlanePods(ctx, ns)...)

	// 2. Translation status of kgateway resources
	findings = append(findings, t.checkResourceTranslationStatus(ctx)...)

	// 3. Data plane proxy health for kgateway-managed Gateways
	findings = append(findings, t.checkDataPlaneHealth(ctx)...)

	if len(findings) == 0 {
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityOK,
			Category: types.CategoryMesh,
			Summary:  "kgateway installation health check passed — no issues detected",
		})
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, "kgateway"), nil
}

// checkControlPlanePods checks kgateway control plane pods for readiness.
func (t *CheckKgatewayHealthTool) checkControlPlanePods(ctx context.Context, ns string) []types.DiagnosticFinding {
	var findings []types.DiagnosticFinding

	// kgateway control plane pods are typically labelled app.kubernetes.io/name=kgateway or app=kgateway
	for _, labelSelector := range []string{"app.kubernetes.io/name=kgateway", "app=kgateway"} {
		podList, err := t.Clients.Dynamic.Resource(podsGVR).Namespace(ns).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			continue
		}
		if len(podList.Items) == 0 {
			continue
		}

		for _, pod := range podList.Items {
			findings = append(findings, evaluatePodHealth(&pod, "control-plane")...)
		}
		return findings
	}

	// Also try deployment-based discovery
	deploymentsGVR := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	depList, err := t.Clients.Dynamic.Resource(deploymentsGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, dep := range depList.Items {
			depName := dep.GetName()
			if strings.Contains(depName, "kgateway") || strings.Contains(depName, "gloo") {
				// Check the deployment's pods via matchLabels
				selector, _, _ := unstructured.NestedMap(dep.Object, "spec", "selector", "matchLabels")
				if len(selector) == 0 {
					continue
				}
				labelParts := make([]string, 0, len(selector))
				for k, v := range selector {
					if vs, ok := v.(string); ok {
						labelParts = append(labelParts, k+"="+vs)
					}
				}
				podList, podErr := t.Clients.Dynamic.Resource(podsGVR).Namespace(ns).List(ctx, metav1.ListOptions{
					LabelSelector: strings.Join(labelParts, ","),
				})
				if podErr != nil || len(podList.Items) == 0 {
					findings = append(findings, types.DiagnosticFinding{
						Severity:   types.SeverityCritical,
						Category:   types.CategoryMesh,
						Resource:   &types.ResourceRef{Kind: "Deployment", Namespace: ns, Name: depName, APIVersion: "apps/v1"},
						Summary:    fmt.Sprintf("kgateway Deployment %s/%s has no running pods", ns, depName),
						Suggestion: "Check deployment status and events for scheduling or image pull issues",
					})
					continue
				}
				for _, pod := range podList.Items {
					findings = append(findings, evaluatePodHealth(&pod, "control-plane")...)
				}
			}
		}
	}

	if len(findings) == 0 {
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityWarning,
			Category:   types.CategoryMesh,
			Summary:    fmt.Sprintf("No kgateway control plane pods found in namespace %s", ns),
			Suggestion: "Verify kgateway is installed and the correct namespace is specified",
		})
	}

	return findings
}

// evaluatePodHealth returns findings for a single pod's health status.
func evaluatePodHealth(pod *unstructured.Unstructured, role string) []types.DiagnosticFinding {
	var findings []types.DiagnosticFinding
	podName := pod.GetName()
	podNs := pod.GetNamespace()
	ref := &types.ResourceRef{Kind: "Pod", Namespace: podNs, Name: podName}

	phase, _, _ := unstructured.NestedString(pod.Object, "status", "phase")

	// Check container statuses
	containerStatuses, _, _ := unstructured.NestedSlice(pod.Object, "status", "containerStatuses")
	allReady := true
	restartCount := 0
	var notReadyContainers []string

	for _, cs := range containerStatuses {
		csMap, ok := cs.(map[string]interface{})
		if !ok {
			continue
		}
		cName, _ := csMap["name"].(string)
		ready, _, _ := unstructured.NestedBool(csMap, "ready")
		restarts, _, _ := unstructured.NestedFloat64(csMap, "restartCount")
		restartCount += int(restarts)

		if !ready {
			allReady = false
			notReadyContainers = append(notReadyContainers, cName)
		}

		// Check for waiting state with error reason
		if waiting, ok := csMap["state"].(map[string]interface{}); ok {
			if w, ok := waiting["waiting"].(map[string]interface{}); ok {
				reason, _ := w["reason"].(string)
				message, _ := w["message"].(string)
				if reason == "CrashLoopBackOff" || reason == "ImagePullBackOff" || reason == "ErrImagePull" {
					findings = append(findings, types.DiagnosticFinding{
						Severity:   types.SeverityCritical,
						Category:   types.CategoryMesh,
						Resource:   ref,
						Summary:    fmt.Sprintf("Pod %s/%s container %s is %s", podNs, podName, cName, reason),
						Detail:     message,
						Suggestion: "Check container logs and events for the root cause",
					})
				}
			}
		}
	}

	if phase == "Running" && allReady {
		severity := types.SeverityOK
		summary := fmt.Sprintf("Pod %s/%s (%s) is Running and ready", podNs, podName, role)
		detail := ""
		if restartCount > 0 {
			detail = fmt.Sprintf("Total container restarts: %d", restartCount)
		}
		if restartCount > 5 {
			severity = types.SeverityWarning
			summary = fmt.Sprintf("Pod %s/%s (%s) is Running but has %d restarts", podNs, podName, role, restartCount)
		}
		findings = append(findings, types.DiagnosticFinding{
			Severity: severity,
			Category: types.CategoryMesh,
			Resource: ref,
			Summary:  summary,
			Detail:   detail,
		})
	} else if phase == "Running" && !allReady {
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityWarning,
			Category:   types.CategoryMesh,
			Resource:   ref,
			Summary:    fmt.Sprintf("Pod %s/%s (%s) is Running but not all containers are ready", podNs, podName, role),
			Detail:     fmt.Sprintf("Not ready containers: %s", strings.Join(notReadyContainers, ", ")),
			Suggestion: "Check container readiness probes and logs",
		})
	} else {
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityCritical,
			Category:   types.CategoryMesh,
			Resource:   ref,
			Summary:    fmt.Sprintf("Pod %s/%s (%s) phase=%s", podNs, podName, role, phase),
			Suggestion: "Check pod events and logs for scheduling or startup issues",
		})
	}

	return findings
}

// checkResourceTranslationStatus checks status conditions on kgateway-managed resources.
func (t *CheckKgatewayHealthTool) checkResourceTranslationStatus(ctx context.Context) []types.DiagnosticFinding {
	var findings []types.DiagnosticFinding

	// Check each kgateway resource type
	for kind, info := range kgatewayKindGVRs {
		list, err := t.Clients.Dynamic.Resource(info.gvr).List(ctx, metav1.ListOptions{})
		if err != nil {
			slog.Debug("kgateway health: skipping resource type", "kind", kind, "error", err)
			continue
		}

		accepted := 0
		rejected := 0
		errored := 0

		for _, item := range list.Items {
			conditions, _, _ := unstructured.NestedSlice(item.Object, "status", "conditions")
			state := classifyResourceStatus(conditions)
			switch state {
			case "accepted":
				accepted++
			case "rejected":
				rejected++
				findings = append(findings, types.DiagnosticFinding{
					Severity: types.SeverityCritical,
					Category: types.CategoryMesh,
					Resource: &types.ResourceRef{
						Kind:       kind,
						Namespace:  item.GetNamespace(),
						Name:       item.GetName(),
						APIVersion: info.apiGroup,
					},
					Summary:    fmt.Sprintf("%s %s/%s is rejected by kgateway", kind, item.GetNamespace(), item.GetName()),
					Detail:     extractConditionMessage(conditions, "Accepted"),
					Suggestion: "Check the resource configuration — the kgateway controller could not translate it",
				})
			case "errored":
				errored++
				findings = append(findings, types.DiagnosticFinding{
					Severity: types.SeverityWarning,
					Category: types.CategoryMesh,
					Resource: &types.ResourceRef{
						Kind:       kind,
						Namespace:  item.GetNamespace(),
						Name:       item.GetName(),
						APIVersion: info.apiGroup,
					},
					Summary: fmt.Sprintf("%s %s/%s has error conditions", kind, item.GetNamespace(), item.GetName()),
					Detail:  extractConditionMessage(conditions, ""),
				})
			}
		}

		total := len(list.Items)
		if total > 0 {
			severity := types.SeverityInfo
			if rejected > 0 || errored > 0 {
				severity = types.SeverityWarning
			}
			findings = append(findings, types.DiagnosticFinding{
				Severity: severity,
				Category: types.CategoryMesh,
				Summary:  fmt.Sprintf("%s resources: %d total, %d accepted, %d rejected, %d errored", kind, total, accepted, rejected, errored),
			})
		}
	}

	return findings
}

// classifyResourceStatus determines the translation state from status conditions.
func classifyResourceStatus(conditions []interface{}) string {
	for _, c := range conditions {
		cm, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		condType, _ := cm["type"].(string)
		condStatus, _ := cm["status"].(string)
		reason, _ := cm["reason"].(string)

		if condType == "Accepted" {
			if condStatus == "True" {
				return "accepted"
			}
			return "rejected"
		}
		// Check for error-related conditions
		if condStatus == "False" && (strings.Contains(reason, "Error") || strings.Contains(reason, "Invalid")) {
			return "errored"
		}
	}
	return "unknown"
}

// extractConditionMessage returns the message from a specific condition type, or all False conditions.
func extractConditionMessage(conditions []interface{}, condType string) string {
	var messages []string
	for _, c := range conditions {
		cm, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		ct, _ := cm["type"].(string)
		status, _ := cm["status"].(string)
		reason, _ := cm["reason"].(string)
		message, _ := cm["message"].(string)

		if condType != "" && ct == condType {
			return fmt.Sprintf("reason=%s: %s", reason, message)
		}
		if condType == "" && status == "False" {
			messages = append(messages, fmt.Sprintf("%s: reason=%s: %s", ct, reason, message))
		}
	}
	return strings.Join(messages, "; ")
}

// checkDataPlaneHealth checks Gateways managed by kgateway for proxy health.
func (t *CheckKgatewayHealthTool) checkDataPlaneHealth(ctx context.Context) []types.DiagnosticFinding {
	var findings []types.DiagnosticFinding

	gatewayAPIGVR := schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gateways"}
	gateways, err := t.Clients.Dynamic.Resource(gatewayAPIGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Debug("kgateway health: skipping Gateway data plane check", "error", err)
		return findings
	}

	for _, gw := range gateways.Items {
		// Check if this Gateway is managed by kgateway
		if !isKgatewayManaged(&gw) {
			continue
		}

		gwNs := gw.GetNamespace()
		gwName := gw.GetName()
		gwRef := &types.ResourceRef{
			Kind:       "Gateway",
			Namespace:  gwNs,
			Name:       gwName,
			APIVersion: "gateway.networking.k8s.io",
		}

		// Check Gateway status conditions
		conditions, _, _ := unstructured.NestedSlice(gw.Object, "status", "conditions")
		hasProgrammed := false
		for _, c := range conditions {
			cm, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			condType, _ := cm["type"].(string)
			condStatus, _ := cm["status"].(string)
			reason, _ := cm["reason"].(string)
			message, _ := cm["message"].(string)

			if condType == "Programmed" {
				hasProgrammed = true
				if condStatus == "True" {
					findings = append(findings, types.DiagnosticFinding{
						Severity: types.SeverityOK,
						Category: types.CategoryMesh,
						Resource: gwRef,
						Summary:  fmt.Sprintf("Gateway %s/%s (kgateway) is Programmed", gwNs, gwName),
					})
				} else {
					findings = append(findings, types.DiagnosticFinding{
						Severity:   types.SeverityCritical,
						Category:   types.CategoryMesh,
						Resource:   gwRef,
						Summary:    fmt.Sprintf("Gateway %s/%s (kgateway) is NOT Programmed: reason=%s", gwNs, gwName, reason),
						Detail:     message,
						Suggestion: "Check kgateway controller logs and Gateway resource configuration",
					})
				}
			}

			if condType == "Accepted" && condStatus == "False" {
				findings = append(findings, types.DiagnosticFinding{
					Severity:   types.SeverityCritical,
					Category:   types.CategoryMesh,
					Resource:   gwRef,
					Summary:    fmt.Sprintf("Gateway %s/%s (kgateway) is NOT Accepted: reason=%s", gwNs, gwName, reason),
					Detail:     message,
					Suggestion: "Review Gateway configuration and check if the GatewayClass is correct",
				})
			}
		}

		if !hasProgrammed {
			findings = append(findings, types.DiagnosticFinding{
				Severity:   types.SeverityWarning,
				Category:   types.CategoryMesh,
				Resource:   gwRef,
				Summary:    fmt.Sprintf("Gateway %s/%s (kgateway) has no Programmed condition", gwNs, gwName),
				Suggestion: "The Gateway may still be provisioning or the kgateway controller may not be processing it",
			})
		}

		// Check data plane proxy pods for this Gateway
		proxyLabels := fmt.Sprintf("gateway.networking.k8s.io/gateway-name=%s", gwName)
		proxyPods, podErr := t.Clients.Dynamic.Resource(podsGVR).Namespace(gwNs).List(ctx, metav1.ListOptions{
			LabelSelector: proxyLabels,
		})
		if podErr == nil {
			if len(proxyPods.Items) == 0 {
				findings = append(findings, types.DiagnosticFinding{
					Severity:   types.SeverityWarning,
					Category:   types.CategoryMesh,
					Resource:   gwRef,
					Summary:    fmt.Sprintf("Gateway %s/%s (kgateway) has no data plane proxy pods", gwNs, gwName),
					Suggestion: "Check if kgateway has provisioned the proxy deployment for this Gateway",
				})
			} else {
				for _, pod := range proxyPods.Items {
					findings = append(findings, evaluatePodHealth(&pod, "data-plane")...)
				}
			}
		}
	}

	return findings
}

// isKgatewayManaged checks if a Gateway is managed by kgateway via gatewayClassName or annotations.
func isKgatewayManaged(gw *unstructured.Unstructured) bool {
	// Check gatewayClassName
	className, _, _ := unstructured.NestedString(gw.Object, "spec", "gatewayClassName")
	if strings.Contains(strings.ToLower(className), "kgateway") || strings.Contains(strings.ToLower(className), "gloo") {
		return true
	}

	// Check annotations
	annotations := gw.GetAnnotations()
	for k := range annotations {
		if strings.Contains(k, "kgateway.dev") {
			return true
		}
	}

	// Check infrastructure parametersRef
	infraParams, _, _ := unstructured.NestedMap(gw.Object, "spec", "infrastructure", "parametersRef")
	if infraParams != nil {
		refGroup, _ := infraParams["group"].(string)
		if refGroup == "kgateway.dev" {
			return true
		}
	}

	return false
}
