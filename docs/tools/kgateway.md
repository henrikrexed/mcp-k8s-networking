# kgateway Tools

Available when kgateway CRDs (`kgateway.dev`) are detected.

## list_kgateway_resources

List kgateway resources by kind: GatewayParameters, RouteOption, VirtualHostOption.

## validate_kgateway_resource

Validate kgateway resources for invalid upstream references and option conflicts.

## check_kgateway_health

Health summary of kgateway installation: control plane, translation status, data plane.

## design_kgateway

Generate kgateway configuration based on user intent.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | Yes | Target namespace |
| `resource_type` | string | No | routeoption, virtualhostoption, or gatewayparameters |
| `route_name` | string | No | HTTPRoute to attach to |
| `gateway_name` | string | No | Gateway to configure |
