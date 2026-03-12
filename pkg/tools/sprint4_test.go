package tools

import (
	"testing"

	"github.com/isitobservable/k8s-networking-mcp/pkg/types"
)

// --- imageTag tests ---

func TestImageTag_WithTag(t *testing.T) {
	tests := []struct {
		image    string
		expected string
	}{
		{"docker.io/istio/proxyv2:1.22.0", "1.22.0"},
		{"proxyv2:1.22.0-distroless", "1.22.0-distroless"},
		{"registry.example.com/istio/proxyv2:latest", "latest"},
	}
	for _, tc := range tests {
		t.Run(tc.image, func(t *testing.T) {
			got := imageTag(tc.image)
			if got != tc.expected {
				t.Errorf("imageTag(%q) = %q, want %q", tc.image, got, tc.expected)
			}
		})
	}
}

func TestImageTag_NoTag(t *testing.T) {
	tests := []struct {
		image    string
		expected string
	}{
		{"docker.io/istio/proxyv2", ""},
		{"proxyv2", ""},
	}
	for _, tc := range tests {
		t.Run(tc.image, func(t *testing.T) {
			got := imageTag(tc.image)
			if got != tc.expected {
				t.Errorf("imageTag(%q) = %q, want %q", tc.image, got, tc.expected)
			}
		})
	}
}

func TestImageTag_WithDigest(t *testing.T) {
	got := imageTag("docker.io/istio/proxyv2@sha256:abc123")
	if got != "" {
		t.Errorf("imageTag with digest should return empty, got %q", got)
	}
}

// --- orAny tests ---

func TestOrAny_Empty(t *testing.T) {
	if got := orAny(""); got != "*" {
		t.Errorf("orAny(\"\") = %q, want \"*\"", got)
	}
}

func TestOrAny_NonEmpty(t *testing.T) {
	if got := orAny("GET"); got != "GET" {
		t.Errorf("orAny(\"GET\") = %q, want \"GET\"", got)
	}
}

// --- labelsSubsetOf tests (delegates to labelsMatch) ---

func TestLabelsSubsetOf_AllMatch(t *testing.T) {
	sub := map[string]string{"app": "web"}
	super := map[string]string{"app": "web", "version": "v1"}
	if !labelsSubsetOf(sub, super) {
		t.Error("expected subset match")
	}
}

func TestLabelsSubsetOf_Missing(t *testing.T) {
	sub := map[string]string{"app": "web", "env": "prod"}
	super := map[string]string{"app": "web"}
	if labelsSubsetOf(sub, super) {
		t.Error("expected no match (missing env)")
	}
}

func TestLabelsSubsetOf_Empty(t *testing.T) {
	if !labelsSubsetOf(map[string]string{}, map[string]string{"app": "web"}) {
		t.Error("empty subset should match anything")
	}
}

// --- countCiliumRules tests ---

func TestCountCiliumRules_Empty(t *testing.T) {
	counts := countCiliumRules(nil, nil)
	if counts.ingressRules != 0 || counts.egressRules != 0 || counts.l4PortRules != 0 || counts.l7Rules != 0 {
		t.Errorf("expected all zeros, got %+v", counts)
	}
}

func TestCountCiliumRules_WithRules(t *testing.T) {
	ingress := []interface{}{
		map[string]interface{}{
			"toPorts": []interface{}{
				map[string]interface{}{
					"ports": []interface{}{
						map[string]interface{}{"port": "80", "protocol": "TCP"},
					},
					"rules": map[string]interface{}{
						"http": []interface{}{
							map[string]interface{}{"method": "GET", "path": "/api"},
						},
					},
				},
			},
		},
	}
	egress := []interface{}{
		map[string]interface{}{
			"toPorts": []interface{}{
				map[string]interface{}{
					"ports": []interface{}{
						map[string]interface{}{"port": "443", "protocol": "TCP"},
						map[string]interface{}{"port": "8080", "protocol": "TCP"},
					},
				},
			},
		},
	}

	counts := countCiliumRules(ingress, egress)
	if counts.ingressRules != 1 {
		t.Errorf("expected 1 ingress rule, got %d", counts.ingressRules)
	}
	if counts.egressRules != 1 {
		t.Errorf("expected 1 egress rule, got %d", counts.egressRules)
	}
	if counts.l4PortRules != 3 {
		t.Errorf("expected 3 L4 port rules, got %d", counts.l4PortRules)
	}
	if counts.l7Rules != 1 {
		t.Errorf("expected 1 L7 rule, got %d", counts.l7Rules)
	}
}

// --- describeCiliumRule tests ---

func TestDescribeCiliumRule_L3Only(t *testing.T) {
	rule := map[string]interface{}{
		"fromEndpoints": []interface{}{
			map[string]interface{}{
				"matchLabels": map[string]interface{}{"app": "frontend"},
			},
		},
	}
	ref := &types.ResourceRef{Kind: "CiliumNetworkPolicy", Namespace: "ns", Name: "pol1"}
	findings := describeCiliumRule(rule, "ingress", 0, ref)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != types.SeverityInfo {
		t.Errorf("expected info severity, got %s", findings[0].Severity)
	}
}

func TestDescribeCiliumRule_L7HTTPWarning(t *testing.T) {
	rule := map[string]interface{}{
		"toPorts": []interface{}{
			map[string]interface{}{
				"ports": []interface{}{
					map[string]interface{}{"port": "80", "protocol": "TCP"},
				},
				"rules": map[string]interface{}{
					"http": []interface{}{
						map[string]interface{}{"method": "GET", "path": "/api/v1"},
					},
				},
			},
		},
	}
	ref := &types.ResourceRef{Kind: "CiliumNetworkPolicy", Namespace: "ns", Name: "pol1"}
	findings := describeCiliumRule(rule, "ingress", 0, ref)

	// Expect: 1 L4 info + 1 L7 HTTP warning
	warnCount := 0
	for _, f := range findings {
		if f.Severity == types.SeverityWarning {
			warnCount++
		}
	}
	if warnCount != 1 {
		t.Errorf("expected 1 warning for L7 HTTP restriction, got %d", warnCount)
	}
}

func TestDescribeCiliumRule_L7GRPCWarning(t *testing.T) {
	rule := map[string]interface{}{
		"toPorts": []interface{}{
			map[string]interface{}{
				"ports": []interface{}{
					map[string]interface{}{"port": "9090", "protocol": "TCP"},
				},
				"rules": map[string]interface{}{
					"grpc": []interface{}{
						map[string]interface{}{"service": "myservice.MyService", "method": "GetItem"},
					},
				},
			},
		},
	}
	ref := &types.ResourceRef{Kind: "CiliumNetworkPolicy", Namespace: "ns", Name: "pol1"}
	findings := describeCiliumRule(rule, "egress", 0, ref)

	warnCount := 0
	for _, f := range findings {
		if f.Severity == types.SeverityWarning {
			warnCount++
		}
	}
	if warnCount != 1 {
		t.Errorf("expected 1 warning for L7 gRPC restriction, got %d", warnCount)
	}
}

func TestDescribeCiliumRule_L7KafkaWarning(t *testing.T) {
	rule := map[string]interface{}{
		"toPorts": []interface{}{
			map[string]interface{}{
				"ports": []interface{}{
					map[string]interface{}{"port": "9092", "protocol": "TCP"},
				},
				"rules": map[string]interface{}{
					"kafka": []interface{}{
						map[string]interface{}{"topic": "orders", "role": "produce"},
					},
				},
			},
		},
	}
	ref := &types.ResourceRef{Kind: "CiliumNetworkPolicy", Namespace: "ns", Name: "pol1"}
	findings := describeCiliumRule(rule, "egress", 0, ref)

	warnCount := 0
	for _, f := range findings {
		if f.Severity == types.SeverityWarning {
			warnCount++
		}
	}
	if warnCount != 1 {
		t.Errorf("expected 1 warning for L7 Kafka restriction, got %d", warnCount)
	}
}

func TestDescribeCiliumRule_L7NoRestriction(t *testing.T) {
	// L7 rule with empty method/path → info, not warning
	rule := map[string]interface{}{
		"toPorts": []interface{}{
			map[string]interface{}{
				"ports": []interface{}{
					map[string]interface{}{"port": "80", "protocol": "TCP"},
				},
				"rules": map[string]interface{}{
					"http": []interface{}{
						map[string]interface{}{}, // no method or path
					},
				},
			},
		},
	}
	ref := &types.ResourceRef{Kind: "CiliumNetworkPolicy", Namespace: "ns", Name: "pol1"}
	findings := describeCiliumRule(rule, "ingress", 0, ref)

	for _, f := range findings {
		if f.Severity == types.SeverityWarning {
			t.Errorf("expected no warnings for unrestricted L7 rule, got warning: %s", f.Summary)
		}
	}
}

// --- ciliumEndpointSelectorStr tests ---

func TestCiliumEndpointSelectorStr_WithLabels(t *testing.T) {
	obj := map[string]interface{}{
		"spec": map[string]interface{}{
			"endpointSelector": map[string]interface{}{
				"matchLabels": map[string]interface{}{
					"app": "web",
				},
			},
		},
	}
	got := ciliumEndpointSelectorStr(obj)
	if got != "app=web" {
		t.Errorf("expected 'app=web', got %q", got)
	}
}

func TestCiliumEndpointSelectorStr_Empty(t *testing.T) {
	obj := map[string]interface{}{
		"spec": map[string]interface{}{},
	}
	got := ciliumEndpointSelectorStr(obj)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

// --- sidecarContainerNames tests ---

func TestSidecarContainerNames_IstioProxy(t *testing.T) {
	if !sidecarContainerNames["istio-proxy"] {
		t.Error("istio-proxy should be a sidecar")
	}
}

func TestSidecarContainerNames_LinkerdProxy(t *testing.T) {
	if !sidecarContainerNames["linkerd-proxy"] {
		t.Error("linkerd-proxy should be a sidecar")
	}
}

func TestSidecarContainerNames_CiliumAgentNotSidecar(t *testing.T) {
	if sidecarContainerNames["cilium-agent"] {
		t.Error("cilium-agent should NOT be a sidecar (runs as DaemonSet)")
	}
}

func TestSidecarContainerNames_UnknownNotSidecar(t *testing.T) {
	if sidecarContainerNames["my-container"] {
		t.Error("unknown container should not be a sidecar")
	}
}
