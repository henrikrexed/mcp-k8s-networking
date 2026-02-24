# Design Guidance Tools

Generate provider-specific networking configurations with annotated YAML templates.

## design_gateway_api

Generate Gateway API resources (Gateway, HTTPRoute, GRPCRoute, ReferenceGrant) based on user intent. Requires Gateway API CRDs.

## design_istio

Generate Istio resources (PeerAuthentication, VirtualService, DestinationRule, AuthorizationPolicy) based on user intent. Requires Istio CRDs.

## design_kgateway

Generate kgateway resources (RouteOption, VirtualHostOption, GatewayParameters) based on user intent. Requires kgateway CRDs.

## suggest_remediation

Suggest remediations for identified diagnostic issues with actionable YAML fixes. Always available.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `issue_type` | string | Yes | Issue type (see below) |
| `resource_kind` | string | No | Affected resource kind |
| `resource_name` | string | No | Affected resource name |
| `namespace` | string | No | Namespace |
| `additional_context` | string | No | Extra context |

**Supported issue types:** missing_endpoints, no_matching_pods, network_policy_blocking, dns_failure, mtls_conflict, route_misconfigured, missing_reference_grant, gateway_listener_conflict, sidecar_missing, weight_mismatch
