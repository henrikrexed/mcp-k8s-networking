package tools

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/isitobservable/k8s-networking-mcp/pkg/types"
)

// --- design_istio ---

type DesignIstioTool struct{ BaseTool }

func (t *DesignIstioTool) Name() string { return "design_istio" }
func (t *DesignIstioTool) Description() string {
	return "Generate Istio configuration guidance with annotated YAML templates based on user intent"
}
func (t *DesignIstioTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"intent": map[string]interface{}{
				"type":        "string",
				"description": "What the user wants to achieve (e.g., 'enable mTLS', 'route 80% to v2', 'restrict access')",
			},
			"service_name": map[string]interface{}{
				"type":        "string",
				"description": "Target service name",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Target namespace",
			},
			"mtls_mode": map[string]interface{}{
				"type":        "string",
				"description": "mTLS mode: STRICT, PERMISSIVE, or DISABLE",
			},
			"traffic_split": map[string]interface{}{
				"type":        "string",
				"description": "Traffic split definition as 'subset1:weight1,subset2:weight2' (e.g., 'v1:80,v2:20')",
			},
			"allowed_sources": map[string]interface{}{
				"type":        "string",
				"description": "Comma-separated list of allowed source namespaces or principals for AuthorizationPolicy",
			},
		},
		"required": []string{"namespace"},
	}
}

func (t *DesignIstioTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	intent := getStringArg(args, "intent", "")
	svcName := getStringArg(args, "service_name", "")
	ns := getStringArg(args, "namespace", "default")
	mtlsMode := strings.ToUpper(getStringArg(args, "mtls_mode", ""))
	trafficSplit := getStringArg(args, "traffic_split", "")
	allowedSources := getStringArg(args, "allowed_sources", "")

	findings := make([]types.DiagnosticFinding, 0, 8)
	resources := make([]string, 0, 4)

	// Detect intent from parameters
	wantMTLS := mtlsMode != "" || strings.Contains(strings.ToLower(intent), "mtls") || strings.Contains(strings.ToLower(intent), "tls")
	wantTrafficSplit := trafficSplit != "" || strings.Contains(strings.ToLower(intent), "traffic") || strings.Contains(strings.ToLower(intent), "canary") || strings.Contains(strings.ToLower(intent), "split")
	wantAuthPolicy := allowedSources != "" || strings.Contains(strings.ToLower(intent), "restrict") || strings.Contains(strings.ToLower(intent), "authz") || strings.Contains(strings.ToLower(intent), "access")

	// Check for existing PeerAuthentication conflicts
	if wantMTLS {
		existingPA, err := t.Clients.Dynamic.Resource(paV1GVR).Namespace(ns).List(ctx, metav1.ListOptions{})
		if err == nil && len(existingPA.Items) > 0 {
			for _, pa := range existingPA.Items {
				findings = append(findings, types.DiagnosticFinding{
					Severity:   types.SeverityWarning,
					Category:   types.CategoryTLS,
					Resource:   &types.ResourceRef{Kind: "PeerAuthentication", Namespace: pa.GetNamespace(), Name: pa.GetName()},
					Summary:    fmt.Sprintf("Existing PeerAuthentication %s/%s may conflict", pa.GetNamespace(), pa.GetName()),
					Suggestion: "Review and potentially update this existing policy to avoid conflicts.",
				})
			}
		}

		if mtlsMode == "" {
			mtlsMode = "STRICT"
		}

		paYAML := fmt.Sprintf(`# PeerAuthentication - Configures mTLS mode
apiVersion: security.istio.io/v1
kind: PeerAuthentication
metadata:
  name: %s-mtls
  namespace: %s
spec:
  mtls:
    mode: %s`, peerAuthName(svcName, ns), ns, mtlsMode)

		if svcName != "" {
			paYAML = fmt.Sprintf(`# PeerAuthentication - Configures mTLS for service %s
apiVersion: security.istio.io/v1
kind: PeerAuthentication
metadata:
  name: %s-mtls
  namespace: %s
spec:
  selector:
    matchLabels:
      app: %s
  mtls:
    mode: %s`, svcName, svcName, ns, svcName, mtlsMode)
		}

		resources = append(resources, paYAML)
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryTLS,
			Summary:  fmt.Sprintf("Generated PeerAuthentication with mode %s", mtlsMode),
			Detail:   paYAML,
		})
	}

	// Traffic splitting
	if wantTrafficSplit && svcName != "" {
		splits := parseTrafficSplit(trafficSplit)
		if len(splits) == 0 {
			splits = []trafficEntry{{subset: "v1", weight: 80}, {subset: "v2", weight: 20}}
		}

		// DestinationRule with subsets
		subsetYAML := ""
		for _, s := range splits {
			subsetYAML += fmt.Sprintf(`
  - name: %s
    labels:
      version: %s`, s.subset, s.subset)
		}

		drYAML := fmt.Sprintf(`# DestinationRule - Defines traffic subsets for %s
apiVersion: networking.istio.io/v1
kind: DestinationRule
metadata:
  name: %s
  namespace: %s
spec:
  host: %s
  subsets:%s`, svcName, svcName, ns, svcName, subsetYAML)

		resources = append(resources, drYAML)
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryRouting,
			Summary:  fmt.Sprintf("Generated DestinationRule with %d subsets", len(splits)),
			Detail:   drYAML,
		})

		// VirtualService with routes
		routeYAML := ""
		totalWeight := 0
		for _, s := range splits {
			routeYAML += fmt.Sprintf(`
    - destination:
        host: %s
        subset: %s
      weight: %d`, svcName, s.subset, s.weight)
			totalWeight += s.weight
		}

		vsYAML := fmt.Sprintf(`# VirtualService - Splits traffic between subsets
apiVersion: networking.istio.io/v1
kind: VirtualService
metadata:
  name: %s
  namespace: %s
spec:
  hosts:
  - %s
  http:
  - route:%s`, svcName, ns, svcName, routeYAML)

		resources = append(resources, vsYAML)
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryRouting,
			Summary:  fmt.Sprintf("Generated VirtualService with traffic split (total weight: %d%%)", totalWeight),
			Detail:   vsYAML,
		})

		if totalWeight != 100 {
			findings = append(findings, types.DiagnosticFinding{
				Severity:   types.SeverityWarning,
				Category:   types.CategoryRouting,
				Summary:    fmt.Sprintf("Traffic weights sum to %d%%, expected 100%%", totalWeight),
				Suggestion: "Adjust the weights to ensure they sum to exactly 100%.",
			})
		}
	}

	// AuthorizationPolicy
	if wantAuthPolicy && svcName != "" {
		sources := strings.Split(allowedSources, ",")
		rulesYAML := ""
		for _, src := range sources {
			src = strings.TrimSpace(src)
			if src == "" {
				continue
			}
			rulesYAML += fmt.Sprintf(`
    - from:
      - source:
          namespaces:
          - "%s"`, src)
		}

		if rulesYAML == "" {
			rulesYAML = `
    - from:
      - source:
          namespaces:
          - "default"`
		}

		apYAML := fmt.Sprintf(`# AuthorizationPolicy - Restricts access to %s
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: %s-allow
  namespace: %s
spec:
  selector:
    matchLabels:
      app: %s
  action: ALLOW
  rules:%s`, svcName, svcName, ns, svcName, rulesYAML)

		resources = append(resources, apYAML)
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryPolicy,
			Summary:  "Generated AuthorizationPolicy",
			Detail:   apYAML,
		})
	}

	if len(resources) == 0 {
		findings = append(findings, types.DiagnosticFinding{
			Severity:   types.SeverityInfo,
			Category:   types.CategoryMesh,
			Summary:    "No specific Istio resources generated",
			Suggestion: "Provide more specific parameters: mtls_mode for mTLS config, traffic_split for canary deployments, or allowed_sources for access control.",
		})
	} else {
		allYAML := strings.Join(resources, "\n---\n")
		findings = append(findings, types.DiagnosticFinding{
			Severity: types.SeverityInfo,
			Category: types.CategoryMesh,
			Summary:  fmt.Sprintf("Complete Istio configuration: %d resources to apply", len(resources)),
			Detail:   allYAML,
		})
	}

	return NewToolResultResponse(t.Cfg, t.Name(), findings, ns, "istio"), nil
}

func peerAuthName(svcName, ns string) string {
	if svcName != "" {
		return svcName
	}
	return ns + "-default"
}

type trafficEntry struct {
	subset string
	weight int
}

func parseTrafficSplit(split string) []trafficEntry {
	if split == "" {
		return nil
	}
	var entries []trafficEntry
	for _, part := range strings.Split(split, ",") {
		parts := strings.SplitN(strings.TrimSpace(part), ":", 2)
		if len(parts) == 2 {
			weight := 0
			fmt.Sscanf(parts[1], "%d", &weight)
			entries = append(entries, trafficEntry{subset: parts[0], weight: weight})
		}
	}
	return entries
}
