# Gateway API Tools

Available when Gateway API CRDs (`gateway.networking.k8s.io`) are detected.

## list_gateways / get_gateway

Inspect Gateway resources including listeners, status conditions, and attached routes.

## list_httproutes / get_httproute

Inspect HTTPRoute resources including parent refs, backend refs, rules, and filters.

## list_grpcroutes / get_grpcroute

Inspect GRPCRoute resources including method matching and backend references.

## list_referencegrants / get_referencegrant

Inspect ReferenceGrant resources for cross-namespace reference allowances.

## scan_gateway_misconfigs

Comprehensive scan for Gateway API misconfigurations: orphaned routes, missing backends, listener conflicts, missing ReferenceGrants.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | No | Namespace (empty for cluster-wide) |

## check_gateway_conformance

Validate Gateway API resources against the specification for conformance.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | Yes | Resource name |
| `namespace` | string | Yes | Namespace |
| `kind` | string | Yes | Resource kind (Gateway, HTTPRoute, GRPCRoute) |

## design_gateway_api

Generate Gateway API configuration based on user intent with annotated YAML templates.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `service_name` | string | Yes | Target service |
| `namespace` | string | Yes | Target namespace |
| `port` | integer | Yes | Service port |
| `hostname` | string | No | Route hostname |
| `protocol` | string | No | HTTP, HTTPS, or GRPC |
| `tls_secret` | string | No | TLS secret name |
| `gateway_name` | string | No | Existing Gateway name |
