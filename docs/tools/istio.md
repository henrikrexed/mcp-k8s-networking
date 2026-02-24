# Istio Tools

Available when Istio CRDs (`networking.istio.io`, `security.istio.io`) are detected.

## list_istio_resources / get_istio_resource

List or get Istio resources by kind: virtualservices, destinationrules, authorizationpolicies, peerauthentications.

## check_sidecar_injection

Verify sidecar injection status across deployments in a namespace.

## check_istio_mtls

Check effective mTLS mode per namespace based on PeerAuthentication and DestinationRule policies.

## validate_istio_config

Validate VirtualService and DestinationRule configurations for common misconfigurations.

## analyze_istio_authpolicy

Analyze AuthorizationPolicy resources for overly restrictive or conflicting rules.

## analyze_istio_routing

End-to-end traffic routing analysis: broken routes, weight mismatches, shadowed rules.

## design_istio

Generate Istio configuration based on user intent (mTLS, traffic splitting, access control).

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | Yes | Target namespace |
| `service_name` | string | No | Target service |
| `mtls_mode` | string | No | STRICT, PERMISSIVE, or DISABLE |
| `traffic_split` | string | No | Split as "v1:80,v2:20" |
| `allowed_sources` | string | No | Comma-separated source namespaces |
