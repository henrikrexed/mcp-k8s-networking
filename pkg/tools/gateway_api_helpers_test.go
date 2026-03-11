package tools

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/isitobservable/k8s-networking-mcp/pkg/types"
)

// --- extractRouteStatusSuffix tests ---

func TestExtractRouteStatusSuffix_AllAccepted(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"parents": []interface{}{
				map[string]interface{}{
					"parentRef":      map[string]interface{}{"name": "gw"},
					"controllerName": "example.io/controller",
					"conditions": []interface{}{
						map[string]interface{}{"type": "Accepted", "status": "True", "reason": "Accepted"},
						map[string]interface{}{"type": "ResolvedRefs", "status": "True", "reason": "ResolvedRefs"},
					},
				},
			},
		},
	}
	suffix, hasProblems := extractRouteStatusSuffix(obj)
	if hasProblems {
		t.Errorf("expected no problems, got suffix=%q", suffix)
	}
}

func TestExtractRouteStatusSuffix_NotAccepted(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"parents": []interface{}{
				map[string]interface{}{
					"parentRef":      map[string]interface{}{"name": "gw"},
					"controllerName": "example.io/controller",
					"conditions": []interface{}{
						map[string]interface{}{"type": "Accepted", "status": "False", "reason": "NoMatchingListenerHostname"},
					},
				},
			},
		},
	}
	suffix, hasProblems := extractRouteStatusSuffix(obj)
	if !hasProblems {
		t.Fatal("expected problems")
	}
	if suffix != "⚠ NOT_ACCEPTED(NoMatchingListenerHostname) via example.io/controller" {
		t.Errorf("unexpected suffix: %q", suffix)
	}
}

func TestExtractRouteStatusSuffix_UnresolvedRefs(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"parents": []interface{}{
				map[string]interface{}{
					"parentRef":      map[string]interface{}{"name": "gw"},
					"controllerName": "test-controller",
					"conditions": []interface{}{
						map[string]interface{}{"type": "ResolvedRefs", "status": "False", "reason": "BackendNotFound"},
					},
				},
			},
		},
	}
	suffix, hasProblems := extractRouteStatusSuffix(obj)
	if !hasProblems {
		t.Fatal("expected problems")
	}
	if suffix != "⚠ UNRESOLVED_REFS(BackendNotFound) via test-controller" {
		t.Errorf("unexpected suffix: %q", suffix)
	}
}

func TestExtractRouteStatusSuffix_EmptyControllerAndParentName(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"parents": []interface{}{
				map[string]interface{}{
					"parentRef": map[string]interface{}{},
					"conditions": []interface{}{
						map[string]interface{}{"type": "Accepted", "status": "False", "reason": "BadRef"},
					},
				},
			},
		},
	}
	suffix, hasProblems := extractRouteStatusSuffix(obj)
	if !hasProblems {
		t.Fatal("expected problems")
	}
	// M1 fix: should not contain "via " with empty string
	if suffix != "⚠ NOT_ACCEPTED(BadRef)" {
		t.Errorf("unexpected suffix: %q", suffix)
	}
}

func TestExtractRouteStatusSuffix_NoParentStatuses(t *testing.T) {
	obj := map[string]interface{}{}
	suffix, hasProblems := extractRouteStatusSuffix(obj)
	if hasProblems {
		t.Errorf("expected no problems for empty obj, got suffix=%q", suffix)
	}
}

func TestExtractRouteStatusSuffix_MultipleProblems(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"parents": []interface{}{
				map[string]interface{}{
					"parentRef":      map[string]interface{}{"name": "gw1"},
					"controllerName": "ctrl-a",
					"conditions": []interface{}{
						map[string]interface{}{"type": "Accepted", "status": "False", "reason": "R1"},
					},
				},
				map[string]interface{}{
					"parentRef":      map[string]interface{}{"name": "gw2"},
					"controllerName": "ctrl-b",
					"conditions": []interface{}{
						map[string]interface{}{"type": "ResolvedRefs", "status": "False", "reason": "R2"},
					},
				},
			},
		},
	}
	suffix, hasProblems := extractRouteStatusSuffix(obj)
	if !hasProblems {
		t.Fatal("expected problems")
	}
	if suffix != "⚠ NOT_ACCEPTED(R1) via ctrl-a ⚠ UNRESOLVED_REFS(R2) via ctrl-b" {
		t.Errorf("unexpected suffix: %q", suffix)
	}
}

// --- validateCrossNamespaceRef tests ---

func TestValidateCrossNamespaceRef_SameNamespace(t *testing.T) {
	finding := validateCrossNamespaceRef("ns1", "HTTPRoute", "ns1", "svc", nil, &types.ResourceRef{})
	if finding != nil {
		t.Error("expected nil for same-namespace ref")
	}
}

func TestValidateCrossNamespaceRef_CrossNsWithGrant(t *testing.T) {
	grants := []refGrantEntry{
		{fromNs: "ns1", fromKind: "HTTPRoute", toNs: "ns2"},
	}
	finding := validateCrossNamespaceRef("ns1", "HTTPRoute", "ns2", "svc", grants, &types.ResourceRef{Name: "route1"})
	if finding != nil {
		t.Error("expected nil when ReferenceGrant exists")
	}
}

func TestValidateCrossNamespaceRef_CrossNsWithoutGrant(t *testing.T) {
	ref := &types.ResourceRef{Kind: "HTTPRoute", Namespace: "ns1", Name: "route1"}
	finding := validateCrossNamespaceRef("ns1", "HTTPRoute", "ns2", "svc", nil, ref)
	if finding == nil {
		t.Fatal("expected warning finding for missing grant")
	}
	if finding.Severity != types.SeverityWarning {
		t.Errorf("expected warning severity, got %s", finding.Severity)
	}
	if finding.Category != types.CategoryPolicy {
		t.Errorf("expected policy category, got %s", finding.Category)
	}
}

func TestValidateCrossNamespaceRef_WildcardKind(t *testing.T) {
	grants := []refGrantEntry{
		{fromNs: "ns1", fromKind: "", toNs: "ns2"}, // empty kind = wildcard
	}
	finding := validateCrossNamespaceRef("ns1", "GRPCRoute", "ns2", "svc", grants, &types.ResourceRef{Name: "route1"})
	if finding != nil {
		t.Error("expected nil when wildcard kind grant exists")
	}
}

// --- checkRuleWeights tests ---

func makeBackendRefs(backends ...map[string]interface{}) map[string]interface{} {
	brs := make([]interface{}, len(backends))
	for i, b := range backends {
		brs[i] = b
	}
	return map[string]interface{}{"backendRefs": brs}
}

func TestCheckRuleWeights_SingleBackend(t *testing.T) {
	rm := makeBackendRefs(map[string]interface{}{"name": "svc1", "weight": float64(100)})
	findings := checkRuleWeights(rm, "ns", 0, &types.ResourceRef{}, nil)
	if len(findings) != 0 {
		t.Errorf("expected no findings for single backend, got %d", len(findings))
	}
}

func TestCheckRuleWeights_SumTo100(t *testing.T) {
	rm := makeBackendRefs(
		map[string]interface{}{"name": "svc1", "weight": float64(70)},
		map[string]interface{}{"name": "svc2", "weight": float64(30)},
	)
	findings := checkRuleWeights(rm, "ns", 0, &types.ResourceRef{}, nil)
	for _, f := range findings {
		if f.Severity == types.SeverityWarning && f.Summary != "" {
			// No weight sum warning expected
			if contains(f.Summary, "weights sum to") {
				t.Errorf("unexpected weight sum warning: %s", f.Summary)
			}
		}
	}
}

func TestCheckRuleWeights_SumNot100(t *testing.T) {
	rm := makeBackendRefs(
		map[string]interface{}{"name": "svc1", "weight": float64(50)},
		map[string]interface{}{"name": "svc2", "weight": float64(30)},
	)
	findings := checkRuleWeights(rm, "ns", 0, &types.ResourceRef{}, nil)
	found := false
	for _, f := range findings {
		if f.Severity == types.SeverityWarning && contains(f.Summary, "weights sum to 80") {
			found = true
		}
	}
	if !found {
		t.Error("expected weight sum warning for sum=80")
	}
}

func TestCheckRuleWeights_DefaultWeight1(t *testing.T) {
	// H3: backend without explicit weight gets default weight=1
	rm := makeBackendRefs(
		map[string]interface{}{"name": "svc1", "weight": float64(99)},
		map[string]interface{}{"name": "svc2"}, // no weight → default 1
	)
	findings := checkRuleWeights(rm, "ns", 0, &types.ResourceRef{}, nil)
	found := false
	for _, f := range findings {
		if f.Severity == types.SeverityWarning && contains(f.Summary, "weights sum to 100") {
			found = true
		}
	}
	if found {
		t.Error("did not expect weight sum warning for 99+1=100")
	}
}

func TestCheckRuleWeights_NoExplicitWeights(t *testing.T) {
	rm := makeBackendRefs(
		map[string]interface{}{"name": "svc1"},
		map[string]interface{}{"name": "svc2"},
	)
	findings := checkRuleWeights(rm, "ns", 0, &types.ResourceRef{}, nil)
	if len(findings) != 0 {
		t.Errorf("expected no findings when no explicit weights, got %d", len(findings))
	}
}

func TestCheckRuleWeights_ZeroEndpointsWarning(t *testing.T) {
	rm := makeBackendRefs(
		map[string]interface{}{"name": "svc1", "weight": float64(50)},
		map[string]interface{}{"name": "svc2", "weight": float64(50)},
	)
	epHealth := map[string]backendEndpointHealth{
		"ns/svc1": {readyCount: 3, found: true},
		"ns/svc2": {readyCount: 0, found: true},
	}
	findings := checkRuleWeights(rm, "ns", 0, &types.ResourceRef{}, epHealth)
	found := false
	for _, f := range findings {
		if f.Severity == types.SeverityWarning && contains(f.Summary, "svc2 has weight 50%") {
			found = true
		}
	}
	if !found {
		t.Error("expected zero-endpoints warning for svc2")
	}
}

// --- checkRuleTimeouts tests ---

func TestCheckRuleTimeouts_NoTimeouts(t *testing.T) {
	rm := map[string]interface{}{}
	findings := checkRuleTimeouts(rm, 0, &types.ResourceRef{})
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestCheckRuleTimeouts_RequestTimeoutOnly(t *testing.T) {
	rm := map[string]interface{}{
		"timeouts": map[string]interface{}{
			"request": "10s",
		},
	}
	findings := checkRuleTimeouts(rm, 0, &types.ResourceRef{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != types.SeverityInfo {
		t.Errorf("expected info severity, got %s", findings[0].Severity)
	}
	if findings[0].Suggestion != "" {
		t.Errorf("expected empty suggestion for valid config, got %q", findings[0].Suggestion)
	}
}

func TestCheckRuleTimeouts_BackendExceedsRequest(t *testing.T) {
	rm := map[string]interface{}{
		"timeouts": map[string]interface{}{
			"request":        "5s",
			"backendRequest": "10s",
		},
	}
	findings := checkRuleTimeouts(rm, 0, &types.ResourceRef{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != types.SeverityWarning {
		t.Errorf("expected warning severity, got %s", findings[0].Severity)
	}
	if findings[0].Suggestion == "" {
		t.Error("expected non-empty suggestion for misconfigured timeouts")
	}
}

func TestCheckRuleTimeouts_BackendBelowRequest(t *testing.T) {
	rm := map[string]interface{}{
		"timeouts": map[string]interface{}{
			"request":        "30s",
			"backendRequest": "10s",
		},
	}
	findings := checkRuleTimeouts(rm, 0, &types.ResourceRef{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != types.SeverityInfo {
		t.Errorf("expected info severity for valid config, got %s", findings[0].Severity)
	}
}

// --- fetchRecentEvents tests ---

func TestFetchRecentEvents_NilClientset(t *testing.T) {
	result := fetchRecentEvents(context.Background(), nil, "ns", "Gateway", "gw")
	if result != nil {
		t.Error("expected nil for nil clientset")
	}
}

func TestFetchRecentEvents_RecentEvents(t *testing.T) {
	clientset := fake.NewSimpleClientset(
		&corev1.Event{
			ObjectMeta: metav1.ObjectMeta{Name: "evt1", Namespace: "ns"},
			InvolvedObject: corev1.ObjectReference{
				Name:      "gw",
				Kind:      "Gateway",
				Namespace: "ns",
			},
			Type:           "Warning",
			Reason:         "Unhealthy",
			Message:        "Listener not ready",
			LastTimestamp:   metav1.NewTime(time.Now().Add(-10 * time.Minute)),
		},
	)
	result := fetchRecentEvents(context.Background(), clientset, "ns", "Gateway", "gw")
	if len(result) != 1 {
		t.Fatalf("expected 1 event, got %d", len(result))
	}
	if !contains(result[0], "Unhealthy") {
		t.Errorf("expected event to contain 'Unhealthy', got %q", result[0])
	}
}

func TestFetchRecentEvents_OldEventsFiltered(t *testing.T) {
	clientset := fake.NewSimpleClientset(
		&corev1.Event{
			ObjectMeta: metav1.ObjectMeta{Name: "old-evt", Namespace: "ns"},
			InvolvedObject: corev1.ObjectReference{
				Name:      "gw",
				Kind:      "Gateway",
				Namespace: "ns",
			},
			Type:          "Warning",
			Reason:        "Old",
			Message:       "Old event",
			LastTimestamp:  metav1.NewTime(time.Now().Add(-2 * time.Hour)),
		},
	)
	result := fetchRecentEvents(context.Background(), clientset, "ns", "Gateway", "gw")
	if len(result) != 0 {
		t.Errorf("expected 0 events (old filtered), got %d", len(result))
	}
}

func TestFetchRecentEvents_MaxFiveEvents(t *testing.T) {
	var events []corev1.Event
	for i := 0; i < 8; i++ {
		events = append(events, corev1.Event{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "evt" + string(rune('0'+i)),
				Namespace: "ns",
			},
			InvolvedObject: corev1.ObjectReference{
				Name:      "gw",
				Kind:      "Gateway",
				Namespace: "ns",
			},
			Type:         "Warning",
			Reason:       "Test",
			Message:      "msg",
			LastTimestamp: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
		})
	}
	clientset := fake.NewSimpleClientset()
	for i := range events {
		_, _ = clientset.CoreV1().Events("ns").Create(context.Background(), &events[i], metav1.CreateOptions{})
	}
	result := fetchRecentEvents(context.Background(), clientset, "ns", "Gateway", "gw")
	if len(result) > 5 {
		t.Errorf("expected at most 5 events, got %d", len(result))
	}
}

// --- buildRefGrants tests ---

func TestBuildRefGrants_NilList(t *testing.T) {
	result := buildRefGrants(nil)
	if result != nil {
		t.Error("expected nil for nil list")
	}
}

func TestBuildRefGrants_ExtractsEntries(t *testing.T) {
	list := &unstructured.UnstructuredList{
		Items: []unstructured.Unstructured{
			{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "grant1",
						"namespace": "target-ns",
					},
					"spec": map[string]interface{}{
						"from": []interface{}{
							map[string]interface{}{
								"namespace": "source-ns",
								"kind":      "HTTPRoute",
							},
						},
					},
				},
			},
		},
	}
	result := buildRefGrants(list)
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	if result[0].fromNs != "source-ns" || result[0].fromKind != "HTTPRoute" || result[0].toNs != "target-ns" {
		t.Errorf("unexpected entry: %+v", result[0])
	}
}

// --- hasRefGrant tests ---

func TestHasRefGrant_Match(t *testing.T) {
	grants := []refGrantEntry{{fromNs: "ns1", fromKind: "HTTPRoute", toNs: "ns2"}}
	if !hasRefGrant(grants, "ns1", "HTTPRoute", "ns2") {
		t.Error("expected match")
	}
}

func TestHasRefGrant_NoMatch(t *testing.T) {
	grants := []refGrantEntry{{fromNs: "ns1", fromKind: "HTTPRoute", toNs: "ns2"}}
	if hasRefGrant(grants, "ns1", "GRPCRoute", "ns2") {
		t.Error("expected no match for different kind")
	}
}

func TestHasRefGrant_WildcardKind(t *testing.T) {
	grants := []refGrantEntry{{fromNs: "ns1", fromKind: "", toNs: "ns2"}}
	if !hasRefGrant(grants, "ns1", "GRPCRoute", "ns2") {
		t.Error("expected match with wildcard kind")
	}
}

func TestHasRefGrant_Empty(t *testing.T) {
	if hasRefGrant(nil, "ns1", "HTTPRoute", "ns2") {
		t.Error("expected no match with nil grants")
	}
}

// helper
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
