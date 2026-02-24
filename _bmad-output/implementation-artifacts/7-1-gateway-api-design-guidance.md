# Story 7.1: Gateway API Design Guidance

Status: done

## Story

As an AI agent,
I want to generate Gateway API configuration YAML based on user intent,
so that I can guide users through setting up Gateway, HTTPRoute/GRPCRoute, and ReferenceGrant resources correctly.

## Acceptance Criteria

1. The design_gateway_api tool generates annotated YAML for Gateway, HTTPRoute or GRPCRoute, and ReferenceGrant resources
2. When no existing Gateway is specified, the tool checks for existing gateways via listWithFallback and suggests using them
3. Cross-namespace route-to-gateway references automatically generate a ReferenceGrant
4. HTTPS protocol without a TLS secret produces a warning finding
5. The tool checks that the target service exists and warns if not found
6. The tool is conditionally registered when Gateway API CRDs are detected

## Tasks / Subtasks

- [x] Task 1: Implement DesignGatewayAPITool (AC: 1, 2, 3, 4, 5)
  - [x] Create struct embedding BaseTool
  - [x] Define InputSchema: service_name (required), namespace (required), port (required), intent, hostname, protocol (HTTP/HTTPS/GRPC), tls_secret, gateway_name, gateway_namespace
  - [x] Check target service exists via Dynamic client, add warning finding if not found
  - [x] When gateway_name is empty, list existing gateways via `listWithFallback(ctx, ..., gatewaysV1GVR, gatewaysV1B1GVR, "")` and use the first as default
  - [x] Generate Gateway YAML when no existing gateway found, with gatewayClassName placeholder comment
  - [x] Generate HTTPRoute (or GRPCRoute when protocol=GRPC) with parentRef pointing to the gateway
  - [x] Add hostname to listener and route when specified
  - [x] Add TLS configuration block to Gateway when protocol=HTTPS and tls_secret provided
  - [x] Detect cross-namespace reference (gwNamespace != ns) and generate ReferenceGrant with warning severity
  - [x] Add warning finding when HTTPS requested without tls_secret
  - [x] Generate combined YAML summary with `---` separator
- [x] Task 2: Register tool in main.go (AC: 6)
  - [x] Register inside `features.HasGatewayAPI` conditional block
  - [x] Include "design_gateway_api" in gatewayToolNames for unregistration

## Dev Notes

### Key Design Decisions

- **listWithFallback for gateway discovery**: Uses the existing `listWithFallback` helper that tries v1 first, then v1beta1, handling clusters at different Gateway API adoption stages. The `gatewaysV1GVR` and `gatewaysV1B1GVR` GVRs come from `gateway_api.go`.
- **Automatic ReferenceGrant generation**: When the route namespace differs from the gateway namespace, a `ReferenceGrant` in v1beta1 is automatically generated. This is the most common cross-namespace pitfall in Gateway API.
- **gatewayClassName placeholder**: The generated Gateway YAML includes `gatewayClassName: ""` with a comment instructing users to set their provider's class. This avoids assuming the provider.
- **Route kind selection**: Protocol "GRPC" generates a `GRPCRoute`, all others generate `HTTPRoute`. This covers the two most common route types in Gateway API v1.
- **Combined YAML output**: All resources are joined with `---` YAML document separator and included as a summary finding, making it easy for agents to present the complete configuration.
- **Service existence check**: Uses the dynamic client with `servicesGVR` to verify the backend service exists before generating routing configuration.

### Files Created

| File | Purpose |
|---|---|
| `pkg/tools/design_gateway.go` | DesignGatewayAPITool implementation |

### Files Modified

| File | Action |
|---|---|
| `cmd/server/main.go` | Registered DesignGatewayAPITool conditionally under HasGatewayAPI |

## Dev Agent Record

### Agent Model Used
claude-opus-4-6

### Completion Notes List
- Tool generates between 1-3 resources depending on context (Gateway + Route + ReferenceGrant)
- Uses existing gatewaysV1GVR/gatewaysV1B1GVR from gateway_api.go
- Findings use CategoryRouting for route/gateway resources and CategoryTLS for TLS warnings
- The tool does not apply resources; it only generates advisory YAML

### File List
- pkg/tools/design_gateway.go
- cmd/server/main.go
