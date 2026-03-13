package k8s

import (
	"fmt"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

var k8sTracer = otel.Tracer("mcp-k8s-networking.k8s")

// tracingRoundTripper wraps an http.RoundTripper to create OTel spans for K8s API calls.
type tracingRoundTripper struct {
	base http.RoundTripper
}

func newTracingTransport(base http.RoundTripper) http.RoundTripper {
	return &tracingRoundTripper{base: base}
}

func (t *tracingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	verb, resource, namespace, name := parseK8sURL(req.Method, req.URL.Path)

	spanName := fmt.Sprintf("k8s.api/%s/%s", verb, resource)

	attrs := []attribute.KeyValue{
		semconv.HTTPRequestMethodKey.String(req.Method),
		attribute.String("k8s.resource.kind", resource),
		attribute.String("k8s.api.verb", verb),
	}
	if namespace != "" {
		attrs = append(attrs, attribute.String("k8s.namespace", namespace))
	}
	if name != "" {
		attrs = append(attrs, attribute.String("k8s.resource.name", name))
	}

	ctx, span := k8sTracer.Start(req.Context(), spanName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attrs...),
	)
	defer span.End()

	resp, err := t.base.RoundTrip(req.WithContext(ctx))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return resp, err
	}

	span.SetAttributes(semconv.HTTPResponseStatusCode(resp.StatusCode))
	if resp.StatusCode >= 400 {
		span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", resp.StatusCode))
	}

	return resp, nil
}

// parseK8sURL extracts verb, resource, namespace, and name from a K8s API URL path.
//
// Core API:  /api/v1/namespaces/{ns}/{resource}[/{name}[/{subresource}]]
// Group API: /apis/{group}/{version}/namespaces/{ns}/{resource}[/{name}[/{subresource}]]
// Cluster:   /api/v1/{resource}[/{name}]
// Cluster:   /apis/{group}/{version}/{resource}[/{name}]
func parseK8sURL(method, path string) (verb, resource, namespace, name string) {
	resource = "unknown"

	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) < 2 {
		verb = httpMethodToVerb(method, false)
		return
	}

	var idx int
	if parts[0] == "api" {
		// /api/v1/...
		idx = 2 // skip "api" and version
	} else if parts[0] == "apis" {
		// /apis/{group}/{version}/...
		idx = 3 // skip "apis", group, and version
	} else {
		verb = httpMethodToVerb(method, false)
		return
	}

	if idx >= len(parts) {
		verb = httpMethodToVerb(method, false)
		return
	}

	// Check for namespaced resources
	if parts[idx] == "namespaces" && idx+2 < len(parts) {
		namespace = parts[idx+1]
		idx += 2
	}

	if idx < len(parts) {
		resource = parts[idx]
		idx++
	}

	if idx < len(parts) {
		name = parts[idx]
	}

	verb = httpMethodToVerb(method, name != "")
	return
}

func httpMethodToVerb(method string, hasName bool) string {
	switch method {
	case http.MethodGet:
		if hasName {
			return "get"
		}
		return "list"
	case http.MethodPost:
		return "create"
	case http.MethodPut:
		return "update"
	case http.MethodPatch:
		return "patch"
	case http.MethodDelete:
		if hasName {
			return "delete"
		}
		return "deletecollection"
	default:
		return strings.ToLower(method)
	}
}
