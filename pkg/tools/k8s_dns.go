package tools

import (
	"context"
	"fmt"
	"net"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// --- check_dns_resolution ---

type CheckDNSTool struct{ BaseTool }

func (t *CheckDNSTool) Name() string        { return "check_dns_resolution" }
func (t *CheckDNSTool) Description() string  { return "DNS lookup for a hostname plus kube-dns service health check" }
func (t *CheckDNSTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"hostname": map[string]interface{}{
				"type":        "string",
				"description": "Hostname to resolve (e.g., my-service.default.svc.cluster.local)",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Namespace context for short names",
			},
		},
		"required": []string{"hostname"},
	}
}

func (t *CheckDNSTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	hostname := getStringArg(args, "hostname", "")

	// DNS lookup
	ips, lookupErr := net.LookupHost(hostname)

	dnsResult := map[string]interface{}{
		"hostname":  hostname,
		"resolved":  lookupErr == nil,
		"addresses": ips,
	}
	if lookupErr != nil {
		dnsResult["error"] = lookupErr.Error()
	}

	// Check kube-dns service health
	kubeDNS, err := t.Clients.Dynamic.Resource(servicesGVR).Namespace("kube-system").Get(ctx, "kube-dns", metav1.GetOptions{})
	var dnsServiceHealth interface{}
	if err == nil {
		clusterIP, _, _ := unstructured.NestedString(kubeDNS.Object, "spec", "clusterIP")

		// Check kube-dns endpoints
		ep, epErr := t.Clients.Dynamic.Resource(endpointsGVR).Namespace("kube-system").Get(ctx, "kube-dns", metav1.GetOptions{})
		readyCount := 0
		if epErr == nil {
			subsets, _, _ := unstructured.NestedSlice(ep.Object, "subsets")
			for _, s := range subsets {
				if sm, ok := s.(map[string]interface{}); ok {
					if addrs, ok := sm["addresses"].([]interface{}); ok {
						readyCount += len(addrs)
					}
				}
			}
		}

		dnsServiceHealth = map[string]interface{}{
			"clusterIP":      clusterIP,
			"readyEndpoints": readyCount,
			"healthy":        readyCount > 0,
		}
	} else {
		dnsServiceHealth = map[string]interface{}{
			"error":   fmt.Sprintf("failed to get kube-dns service: %v", err),
			"healthy": false,
		}
	}

	return NewResponse(t.Cfg, t.Name(), map[string]interface{}{
		"dns":              dnsResult,
		"kubeDNSService":   dnsServiceHealth,
	}), nil
}
