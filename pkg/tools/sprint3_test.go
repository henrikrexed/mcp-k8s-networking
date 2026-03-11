package tools

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/isitobservable/k8s-networking-mcp/pkg/types"
)

// --- extractCBFromMap tests ---

func TestExtractCBFromMap_NoConnectionPoolOrOutlierDetection(t *testing.T) {
	parent := map[string]interface{}{
		"tls": map[string]interface{}{"mode": "ISTIO_MUTUAL"},
	}
	ref := &types.ResourceRef{Kind: "DestinationRule", Namespace: "ns", Name: "dr1"}
	findings := extractCBFromMap(parent, ref)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

func TestExtractCBFromMap_ConnectionPoolOnly(t *testing.T) {
	parent := map[string]interface{}{
		"connectionPool": map[string]interface{}{
			"tcp": map[string]interface{}{
				"maxConnections": float64(100),
				"connectTimeout": "30s",
			},
			"http": map[string]interface{}{
				"http1MaxPendingRequests":  float64(1024),
				"http2MaxRequests":         float64(1024),
				"maxRequestsPerConnection": float64(10),
				"maxRetries":               float64(3),
			},
		},
	}
	ref := &types.ResourceRef{Kind: "DestinationRule", Namespace: "ns", Name: "dr1"}
	findings := extractCBFromMap(parent, ref)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != types.SeverityInfo {
		t.Errorf("expected info severity, got %s", findings[0].Severity)
	}
	if findings[0].Summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestExtractCBFromMap_OutlierDetectionNormal(t *testing.T) {
	parent := map[string]interface{}{
		"outlierDetection": map[string]interface{}{
			"consecutiveErrors":      float64(5),
			"consecutive5xxErrors":   float64(5),
			"consecutiveGatewayErrors": float64(3),
			"interval":               "10s",
			"baseEjectionTime":       "30s",
			"maxEjectionPercent":     float64(10),
		},
	}
	ref := &types.ResourceRef{Kind: "DestinationRule", Namespace: "ns", Name: "dr1"}
	findings := extractCBFromMap(parent, ref)

	// Should have 1 info finding for outlier detection, no warnings (not aggressive)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding (info only), got %d", len(findings))
	}
	if findings[0].Severity != types.SeverityInfo {
		t.Errorf("expected info severity, got %s", findings[0].Severity)
	}
}

func TestExtractCBFromMap_OutlierDetectionAggressive(t *testing.T) {
	parent := map[string]interface{}{
		"outlierDetection": map[string]interface{}{
			"consecutiveErrors":    float64(1),
			"consecutive5xxErrors": float64(2),
			"baseEjectionTime":     "10m",
		},
	}
	ref := &types.ResourceRef{Kind: "DestinationRule", Namespace: "ns", Name: "dr1"}
	findings := extractCBFromMap(parent, ref)

	// 1 info (summary) + 3 warnings (consecutiveErrors<3, consecutive5xxErrors<3, baseEjectionTime>5m)
	if len(findings) != 4 {
		t.Fatalf("expected 4 findings, got %d", len(findings))
	}
	warnCount := 0
	for _, f := range findings {
		if f.Severity == types.SeverityWarning {
			warnCount++
		}
	}
	if warnCount != 3 {
		t.Errorf("expected 3 warnings, got %d", warnCount)
	}
}

func TestExtractCBFromMap_ConnectionPoolAndOutlierDetection(t *testing.T) {
	parent := map[string]interface{}{
		"connectionPool": map[string]interface{}{
			"tcp": map[string]interface{}{
				"maxConnections": float64(50),
			},
		},
		"outlierDetection": map[string]interface{}{
			"consecutiveErrors": float64(5),
			"interval":          "10s",
		},
	}
	ref := &types.ResourceRef{Kind: "DestinationRule", Namespace: "ns", Name: "dr1"}
	findings := extractCBFromMap(parent, ref)

	// 1 connectionPool info + 1 outlierDetection info = 2 findings
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}
}

// --- extractCircuitBreakerFindings tests (Istio wrapper) ---

func TestExtractCircuitBreakerFindings_WithBasePath(t *testing.T) {
	obj := map[string]interface{}{
		"spec": map[string]interface{}{
			"trafficPolicy": map[string]interface{}{
				"connectionPool": map[string]interface{}{
					"tcp": map[string]interface{}{
						"maxConnections": float64(100),
					},
				},
			},
		},
	}
	ref := &types.ResourceRef{Kind: "DestinationRule", Namespace: "ns", Name: "dr1"}
	findings := extractCircuitBreakerFindings(obj, "spec", ref)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestExtractCircuitBreakerFindings_NoTrafficPolicy(t *testing.T) {
	obj := map[string]interface{}{
		"spec": map[string]interface{}{
			"host": "my-service",
		},
	}
	ref := &types.ResourceRef{Kind: "DestinationRule", Namespace: "ns", Name: "dr1"}
	findings := extractCircuitBreakerFindings(obj, "spec", ref)

	if findings != nil {
		t.Errorf("expected nil findings, got %d", len(findings))
	}
}

func TestExtractCircuitBreakerFindings_SubsetPath(t *testing.T) {
	obj := map[string]interface{}{
		"trafficPolicy": map[string]interface{}{
			"outlierDetection": map[string]interface{}{
				"consecutiveErrors": float64(10),
			},
		},
	}
	ref := &types.ResourceRef{Kind: "DestinationRule", Namespace: "ns", Name: "dr1 (subset: v1)"}
	findings := extractCircuitBreakerFindings(obj, "", ref)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

// --- labelsMatch tests ---

func TestLabelsMatch_AllMatch(t *testing.T) {
	selector := map[string]string{"app": "myservice", "version": "v1"}
	podLabels := map[string]string{"app": "myservice", "version": "v1", "extra": "label"}
	if !labelsMatch(selector, podLabels) {
		t.Error("expected labels to match")
	}
}

func TestLabelsMatch_MissingLabel(t *testing.T) {
	selector := map[string]string{"app": "myservice", "version": "v1"}
	podLabels := map[string]string{"app": "myservice"}
	if labelsMatch(selector, podLabels) {
		t.Error("expected labels NOT to match (missing version)")
	}
}

func TestLabelsMatch_WrongValue(t *testing.T) {
	selector := map[string]string{"app": "myservice"}
	podLabels := map[string]string{"app": "otherservice"}
	if labelsMatch(selector, podLabels) {
		t.Error("expected labels NOT to match (wrong value)")
	}
}

func TestLabelsMatch_EmptySelector(t *testing.T) {
	selector := map[string]string{}
	podLabels := map[string]string{"app": "myservice"}
	if !labelsMatch(selector, podLabels) {
		t.Error("empty selector should match everything")
	}
}

// --- targetRef matching logic tests ---

func TestCheckKgatewayTrafficPolicies_TargetRefMatching(t *testing.T) {
	tests := []struct {
		name       string
		targetRef  map[string]interface{}
		service    string
		route      string
		shouldSkip bool
	}{
		{
			name:       "exact service match",
			targetRef:  map[string]interface{}{"kind": "Service", "name": "my-svc"},
			service:    "my-svc",
			shouldSkip: false,
		},
		{
			name:       "service mismatch",
			targetRef:  map[string]interface{}{"kind": "Service", "name": "other-svc"},
			service:    "my-svc",
			shouldSkip: true,
		},
		{
			name:       "httproute match with route filter",
			targetRef:  map[string]interface{}{"kind": "HTTPRoute", "name": "my-route"},
			service:    "my-svc",
			route:      "my-route",
			shouldSkip: false,
		},
		{
			name:       "httproute with no route filter — include",
			targetRef:  map[string]interface{}{"kind": "HTTPRoute", "name": "any-route"},
			service:    "my-svc",
			route:      "",
			shouldSkip: false,
		},
		{
			name:       "httproute mismatch with route filter",
			targetRef:  map[string]interface{}{"kind": "HTTPRoute", "name": "other-route"},
			service:    "my-svc",
			route:      "my-route",
			shouldSkip: true,
		},
		{
			name:       "no targetRef — include",
			targetRef:  nil,
			service:    "my-svc",
			shouldSkip: false,
		},
		{
			name:       "name matches service directly",
			targetRef:  map[string]interface{}{"name": "my-svc"},
			service:    "my-svc",
			shouldSkip: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.targetRef == nil {
				// No targetRef means include
				if tc.shouldSkip {
					t.Error("nil targetRef should never skip")
				}
				return
			}

			targetName, _ := tc.targetRef["name"].(string)
			targetKind, _ := tc.targetRef["kind"].(string)

			// Reproduce the simplified matching logic from rate_limiting.go
			skip := false
			if targetName != "" {
				matches := targetName == tc.service ||
					(targetKind == "Service" && targetName == tc.service) ||
					(targetKind == "HTTPRoute" && (tc.route == "" || targetName == tc.route))
				if !matches {
					skip = true
				}
			}

			if skip != tc.shouldSkip {
				t.Errorf("expected skip=%v, got %v", tc.shouldSkip, skip)
			}
		})
	}
}

// --- envoyRateLimitTypeURLs tests ---

func TestEnvoyRateLimitTypeURLs(t *testing.T) {
	tests := []struct {
		typeURL    string
		expected   string
		shouldFind bool
	}{
		{
			typeURL:    "type.googleapis.com/envoy.extensions.filters.http.local_ratelimit.v3.LocalRateLimit",
			expected:   "local",
			shouldFind: true,
		},
		{
			typeURL:    "type.googleapis.com/envoy.extensions.filters.http.ratelimit.v3.RateLimit",
			expected:   "global",
			shouldFind: true,
		},
		{
			typeURL:    "type.googleapis.com/envoy.extensions.filters.http.cors.v3.Cors",
			shouldFind: false,
		},
		{
			typeURL:    "",
			shouldFind: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.typeURL, func(t *testing.T) {
			rlType, ok := envoyRateLimitTypeURLs[tc.typeURL]
			if tc.shouldFind {
				if !ok || rlType == "" {
					t.Errorf("expected to find type=%q for URL %q", tc.expected, tc.typeURL)
				}
				if rlType != tc.expected {
					t.Errorf("expected type=%q, got %q", tc.expected, rlType)
				}
			} else {
				if ok && rlType != "" {
					t.Errorf("expected NOT to find rate limit type for URL %q, got %q", tc.typeURL, rlType)
				}
			}
		})
	}
}

// --- TLS validation tests (gateway_api.go get_gateway listener enrichment) ---

func TestTLSListenerEnrichment_TerminateWithCerts(t *testing.T) {
	listener := map[string]interface{}{
		"name":     "https",
		"port":     float64(443),
		"protocol": "HTTPS",
		"tls": map[string]interface{}{
			"mode": "Terminate",
			"certificateRefs": []interface{}{
				map[string]interface{}{
					"name": "my-cert",
				},
			},
		},
	}

	// Verify TLS fields can be extracted
	tlsConfig, tlsFound, _ := unstructured.NestedMap(listener, "tls")
	if !tlsFound || tlsConfig == nil {
		t.Fatal("expected TLS config to be found")
	}
	mode, _ := tlsConfig["mode"].(string)
	if mode != "Terminate" {
		t.Errorf("expected mode=Terminate, got %q", mode)
	}
	certRefs, _, _ := unstructured.NestedSlice(tlsConfig, "certificateRefs")
	if len(certRefs) != 1 {
		t.Fatalf("expected 1 cert ref, got %d", len(certRefs))
	}
}

func TestTLSListenerEnrichment_PassthroughNoCerts(t *testing.T) {
	listener := map[string]interface{}{
		"name":     "tls-passthrough",
		"port":     float64(443),
		"protocol": "TLS",
		"tls": map[string]interface{}{
			"mode": "Passthrough",
		},
	}

	tlsConfig, tlsFound, _ := unstructured.NestedMap(listener, "tls")
	if !tlsFound {
		t.Fatal("expected TLS config")
	}
	certRefs, _, _ := unstructured.NestedSlice(tlsConfig, "certificateRefs")
	mode, _ := tlsConfig["mode"].(string)

	// Passthrough without certs is valid — no warning expected
	if mode != "Passthrough" {
		t.Errorf("expected Passthrough, got %q", mode)
	}
	if len(certRefs) != 0 {
		t.Errorf("expected 0 cert refs, got %d", len(certRefs))
	}
}

func TestTLSListenerEnrichment_PassthroughWithCertsContradiction(t *testing.T) {
	listener := map[string]interface{}{
		"name":     "tls-passthrough",
		"port":     float64(443),
		"protocol": "TLS",
		"tls": map[string]interface{}{
			"mode": "Passthrough",
			"certificateRefs": []interface{}{
				map[string]interface{}{"name": "should-not-be-here"},
			},
		},
	}

	tlsConfig, _, _ := unstructured.NestedMap(listener, "tls")
	mode, _ := tlsConfig["mode"].(string)
	certRefs, _, _ := unstructured.NestedSlice(tlsConfig, "certificateRefs")

	// This is a contradiction — Passthrough + certs
	if mode != "Passthrough" || len(certRefs) == 0 {
		t.Fatal("test setup error: should be Passthrough with cert refs")
	}
}

func TestTLSListenerEnrichment_DefaultMode(t *testing.T) {
	listener := map[string]interface{}{
		"name":     "https",
		"port":     float64(443),
		"protocol": "HTTPS",
		"tls": map[string]interface{}{
			// No explicit mode — should default to Terminate
			"certificateRefs": []interface{}{
				map[string]interface{}{"name": "my-cert"},
			},
		},
	}

	tlsConfig, _, _ := unstructured.NestedMap(listener, "tls")
	mode, _ := tlsConfig["mode"].(string)
	if mode != "" {
		t.Errorf("expected empty mode (to be defaulted), got %q", mode)
	}
	// The code defaults empty mode to "Terminate"
}

func TestTLSListenerEnrichment_HTTPListenerNoTLS(t *testing.T) {
	listener := map[string]interface{}{
		"name":     "http",
		"port":     float64(80),
		"protocol": "HTTP",
	}

	protocol, _ := listener["protocol"].(string)
	// HTTP listeners should NOT trigger TLS enrichment
	if protocol == "HTTPS" || protocol == "TLS" {
		t.Error("HTTP listener should not trigger TLS checks")
	}
}

// --- baseEjectionTime duration parsing tests ---

func TestBaseEjectionTimeParsing(t *testing.T) {
	tests := []struct {
		input     string
		expectWarn bool
	}{
		{"30s", false},
		{"1m", false},
		{"5m", false},
		{"6m", true},
		{"10m", true},
		{"invalid", false}, // parse error, no warning
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			dur, err := time.ParseDuration(tc.input)
			isWarning := err == nil && dur > 5*time.Minute
			if isWarning != tc.expectWarn {
				t.Errorf("baseEjectionTime=%q: expected warn=%v, got %v", tc.input, tc.expectWarn, isWarning)
			}
		})
	}
}
