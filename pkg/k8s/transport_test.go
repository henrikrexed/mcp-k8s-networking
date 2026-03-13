package k8s

import (
	"fmt"
	"net/http"
	"testing"
)

func TestParseK8sURL(t *testing.T) {
	tests := []struct {
		name      string
		method    string
		path      string
		wantVerb  string
		wantRes   string
		wantNS    string
		wantName  string
	}{
		{
			name:     "list namespaced pods",
			method:   http.MethodGet,
			path:     "/api/v1/namespaces/default/pods",
			wantVerb: "list", wantRes: "pods", wantNS: "default",
		},
		{
			name:     "get specific pod",
			method:   http.MethodGet,
			path:     "/api/v1/namespaces/kube-system/pods/coredns-abc123",
			wantVerb: "get", wantRes: "pods", wantNS: "kube-system", wantName: "coredns-abc123",
		},
		{
			name:     "list cluster-scoped nodes",
			method:   http.MethodGet,
			path:     "/api/v1/nodes",
			wantVerb: "list", wantRes: "nodes",
		},
		{
			name:     "get specific node",
			method:   http.MethodGet,
			path:     "/api/v1/nodes/worker-1",
			wantVerb: "get", wantRes: "nodes", wantName: "worker-1",
		},
		{
			name:     "list gateway API resources",
			method:   http.MethodGet,
			path:     "/apis/gateway.networking.k8s.io/v1/namespaces/default/httproutes",
			wantVerb: "list", wantRes: "httproutes", wantNS: "default",
		},
		{
			name:     "get specific httproute",
			method:   http.MethodGet,
			path:     "/apis/gateway.networking.k8s.io/v1/namespaces/prod/httproutes/my-route",
			wantVerb: "get", wantRes: "httproutes", wantNS: "prod", wantName: "my-route",
		},
		{
			name:     "create pod",
			method:   http.MethodPost,
			path:     "/api/v1/namespaces/default/pods",
			wantVerb: "create", wantRes: "pods", wantNS: "default",
		},
		{
			name:     "delete pod",
			method:   http.MethodDelete,
			path:     "/api/v1/namespaces/default/pods/my-pod",
			wantVerb: "delete", wantRes: "pods", wantNS: "default", wantName: "my-pod",
		},
		{
			name:     "list services across all namespaces",
			method:   http.MethodGet,
			path:     "/api/v1/services",
			wantVerb: "list", wantRes: "services",
		},
		{
			name:     "list istio virtualservices",
			method:   http.MethodGet,
			path:     "/apis/networking.istio.io/v1/namespaces/default/virtualservices",
			wantVerb: "list", wantRes: "virtualservices", wantNS: "default",
		},
		{
			name:     "cluster-scoped gateway",
			method:   http.MethodGet,
			path:     "/apis/gateway.networking.k8s.io/v1/gateways",
			wantVerb: "list", wantRes: "gateways",
		},
		{
			name:     "pod logs subresource",
			method:   http.MethodGet,
			path:     "/api/v1/namespaces/default/pods/my-pod/log",
			wantVerb: "get", wantRes: "pods", wantNS: "default", wantName: "my-pod",
		},
		{
			name:     "short path",
			method:   http.MethodGet,
			path:     "/api",
			wantVerb: "list", wantRes: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verb, resource, ns, name := parseK8sURL(tt.method, tt.path)
			if verb != tt.wantVerb {
				t.Errorf("verb = %q, want %q", verb, tt.wantVerb)
			}
			if resource != tt.wantRes {
				t.Errorf("resource = %q, want %q", resource, tt.wantRes)
			}
			if ns != tt.wantNS {
				t.Errorf("namespace = %q, want %q", ns, tt.wantNS)
			}
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
		})
	}
}

func TestHttpMethodToVerb(t *testing.T) {
	tests := []struct {
		method  string
		hasName bool
		want    string
	}{
		{http.MethodGet, false, "list"},
		{http.MethodGet, true, "get"},
		{http.MethodPost, false, "create"},
		{http.MethodPut, true, "update"},
		{http.MethodPatch, true, "patch"},
		{http.MethodDelete, true, "delete"},
		{http.MethodDelete, false, "deletecollection"},
	}

	for _, tt := range tests {
		t.Run(tt.method+"_hasName_"+fmt.Sprintf("%v", tt.want), func(t *testing.T) {
			got := httpMethodToVerb(tt.method, tt.hasName)
			if got != tt.want {
				t.Errorf("httpMethodToVerb(%q, %v) = %q, want %q", tt.method, tt.hasName, got, tt.want)
			}
		})
	}
}
