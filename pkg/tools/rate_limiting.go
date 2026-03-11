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

// GVR definitions for rate limit policy sources.
var (
	trafficPolicyGVR = schema.GroupVersionResource{Group: "gateway.kgateway.dev", Version: "v1alpha1", Resource: "trafficpolicies"}
	envoyFilterV1A1  = schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1alpha3", Resource: "envoyfilters"}
)

// --- check_rate_limit_policies ---

type CheckRateLimitPoliciesTool struct{ BaseTool }

func (t *CheckRateLimitPoliciesTool) Name() string { return "check_rate_limit_policies" }
func (t *CheckRateLimitPoliciesTool) Description() string {
	return "Discover rate limiting policies (kgateway TrafficPolicy, Istio EnvoyFilter) affecting a service or route"
}
func (t *CheckRateLimitPoliciesTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"service": map[string]interface{}{
				"type":        "string",
				"description": "Service name to check for rate limit policies",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace (empty for all namespaces)",
			},
			"route": map[string]interface{}{
				"type":        "string",
				"description": "Optional route name to scope the search",
			},
		},
		"required": []string{"service", "namespace"},
	}
}

func (t *CheckRateLimitPoliciesTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	service := getStringArg(args, "service", "")
	ns := getStringArg(args, "namespace", "")
	route := getStringArg(args, "route", "")

	if service == "" {
		return nil, &types.MCPError{
			Code:    types.ErrCodeInvalidInput,
			Tool:    t.Name(),
			Message: "service is required",
		}
	}

	var findings []types.DiagnosticFinding

	// 1. Check kgateway TrafficPolicy resources with rateLimit config
	findings = append(findings, t.checkKgatewayTrafficPolicies(ctx, ns, service, route)...)

	// 2. Check Istio EnvoyFilter resources with rate limit configuration
	findings = append(findings, t.checkIstioEnvoyFilters(ctx, ns, service)...)

	if len(findings) == 0 {
		responseNs := ns
		if responseNs == "" {
			responseNs = "all"
		}
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryPolicy,
			Summary:  fmt.Sprintf("No rate limit policies found for service %s in namespace %s", service, responseNs),
		})
	}

	responseNs := ns
	if responseNs == "" {
		responseNs = "all"
	}
	return NewToolResultResponse(t.Cfg, t.Name(), findings, responseNs, "multi"), nil
}

func (t *CheckRateLimitPoliciesTool) checkKgatewayTrafficPolicies(ctx context.Context, ns, service, route string) []types.DiagnosticFinding {
	var list *unstructured.UnstructuredList
	var err error
	if ns == "" {
		list, err = t.Clients.Dynamic.Resource(trafficPolicyGVR).List(ctx, metav1.ListOptions{})
	} else {
		list, err = t.Clients.Dynamic.Resource(trafficPolicyGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil // CRD not installed or no access — silent
	}

	var findings []types.DiagnosticFinding
	for _, tp := range list.Items {
		tpName := tp.GetName()
		tpNs := tp.GetNamespace()

		ref := &types.ResourceRef{
			Kind:       "TrafficPolicy",
			Namespace:  tpNs,
			Name:       tpName,
			APIVersion: "gateway.kgateway.dev/v1alpha1",
		}

		// Check if this policy targets the service or route
		targetRef, _, _ := unstructured.NestedMap(tp.Object, "spec", "targetRef")
		if targetRef != nil {
			targetName, _ := targetRef["name"].(string)
			targetKind, _ := targetRef["kind"].(string)
			// Match on service name or route name
			if targetName != "" && targetName != service && (route == "" || targetName != route) {
				if targetKind != "Service" || targetName != service {
					if targetKind != "HTTPRoute" || (route != "" && targetName != route) {
						continue
					}
				}
			}
		}

		// Extract rateLimit configuration
		rateLimit, rlFound, _ := unstructured.NestedMap(tp.Object, "spec", "rateLimit")
		if !rlFound || rateLimit == nil {
			continue
		}

		// Determine rate limit type and details
		rlType := "local"
		if descriptors, ok := rateLimit["descriptors"].([]interface{}); ok && len(descriptors) > 0 {
			rlType = "global"
		}

		// Extract limit values
		var limitParts []string
		if local, ok := rateLimit["local"].(map[string]interface{}); ok {
			if tokenBucket, ok := local["tokenBucket"].(map[string]interface{}); ok {
				maxTokens, _ := tokenBucket["maxTokens"].(float64)
				tokensPerFill, _ := tokenBucket["tokensPerFill"].(float64)
				fillInterval, _ := tokenBucket["fillInterval"].(string)
				if maxTokens > 0 {
					limitParts = append(limitParts, fmt.Sprintf("maxTokens=%d", int(maxTokens)))
				}
				if tokensPerFill > 0 {
					limitParts = append(limitParts, fmt.Sprintf("tokensPerFill=%d", int(tokensPerFill)))
				}
				if fillInterval != "" {
					limitParts = append(limitParts, fmt.Sprintf("fillInterval=%s", fillInterval))
				}
			}
		}

		// Extract global rate limit config
		if rlType == "global" {
			if rls, ok := rateLimit["rls"].(map[string]interface{}); ok {
				domain, _ := rls["domain"].(string)
				if domain != "" {
					limitParts = append(limitParts, fmt.Sprintf("domain=%s", domain))
				}
			}
		}

		// Build scope description
		scope := "unknown"
		if targetRef != nil {
			targetKind, _ := targetRef["kind"].(string)
			targetName, _ := targetRef["name"].(string)
			if targetKind != "" && targetName != "" {
				scope = fmt.Sprintf("%s/%s", targetKind, targetName)
			}
		}

		summary := fmt.Sprintf("TrafficPolicy %s/%s: type=%s scope=%s", tpNs, tpName, rlType, scope)
		if len(limitParts) > 0 {
			summary += fmt.Sprintf(" limits={%s}", strings.Join(limitParts, ", "))
		}

		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryPolicy,
			Resource: ref,
			Summary:  summary,
		})
	}

	return findings
}

func (t *CheckRateLimitPoliciesTool) checkIstioEnvoyFilters(ctx context.Context, ns, service string) []types.DiagnosticFinding {
	var list *unstructured.UnstructuredList
	var err error
	if ns == "" {
		list, err = t.Clients.Dynamic.Resource(envoyFilterV1A1).List(ctx, metav1.ListOptions{})
	} else {
		list, err = t.Clients.Dynamic.Resource(envoyFilterV1A1).Namespace(ns).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil // CRD not installed or no access — silent
	}

	var findings []types.DiagnosticFinding
	for _, ef := range list.Items {
		efName := ef.GetName()
		efNs := ef.GetNamespace()

		ref := &types.ResourceRef{
			Kind:       "EnvoyFilter",
			Namespace:  efNs,
			Name:       efName,
			APIVersion: "networking.istio.io/v1alpha3",
		}

		// Check configPatches for rate limit filters
		configPatches, _, _ := unstructured.NestedSlice(ef.Object, "spec", "configPatches")
		hasRateLimit := false
		rlFilterType := ""

		for _, cp := range configPatches {
			cpm, ok := cp.(map[string]interface{})
			if !ok {
				continue
			}
			patch, _, _ := unstructured.NestedMap(cpm, "patch")
			if patch == nil {
				continue
			}
			value, _, _ := unstructured.NestedMap(patch, "value")
			if value == nil {
				continue
			}

			filterName, _ := value["name"].(string)
			if filterName == "envoy.filters.http.local_ratelimit" {
				hasRateLimit = true
				rlFilterType = "local"
			} else if filterName == "envoy.filters.http.ratelimit" {
				hasRateLimit = true
				rlFilterType = "global"
			}

			// Also check typed_config for rate limit
			typedConfig, _, _ := unstructured.NestedMap(value, "typed_config")
			if typedConfig != nil {
				typeURL, _ := typedConfig["@type"].(string)
				if strings.Contains(typeURL, "local_rate_limit") {
					hasRateLimit = true
					rlFilterType = "local"
				} else if strings.Contains(typeURL, "rate_limit") && !strings.Contains(typeURL, "local") {
					hasRateLimit = true
					rlFilterType = "global"
				}
			}
		}

		if !hasRateLimit {
			continue
		}

		// Check workload selector for service relevance
		scope := "all workloads"
		workloadSelector, _, _ := unstructured.NestedMap(ef.Object, "spec", "workloadSelector")
		if workloadSelector != nil {
			labels, _, _ := unstructured.NestedStringMap(workloadSelector, "labels")
			if len(labels) > 0 {
				var labelParts []string
				for k, v := range labels {
					labelParts = append(labelParts, fmt.Sprintf("%s=%s", k, v))
				}
				scope = fmt.Sprintf("workloads matching {%s}", strings.Join(labelParts, ", "))
			}
		}

		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryPolicy,
			Resource: ref,
			Summary:  fmt.Sprintf("EnvoyFilter %s/%s: type=%s scope=%s", efNs, efName, rlFilterType, scope),
		})
	}

	return findings
}
