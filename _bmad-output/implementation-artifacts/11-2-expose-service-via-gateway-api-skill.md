# Story 11.2: Expose Service via Gateway API Skill

Status: done

## Story

As an AI agent,
I want a guided workflow to expose a Kubernetes service via Gateway API,
so that I can generate correct HTTPRoute/GRPCRoute and Gateway manifests with step-by-step validation.

## Acceptance Criteria

1. The `expose_service_gateway_api` skill is registered when Gateway API CRDs are detected
2. Step 1 verifies the target service exists and fails early if not found
3. Step 2 confirms Gateway API CRD availability
4. Step 3 checks for existing Gateways in the cluster; if none found, generates a Gateway manifest with placeholder gatewayClassName
5. Step 4 generates an HTTPRoute (or GRPCRoute for GRPC protocol) with parentRef, optional hostname, and backendRef
6. Step 5 checks if cross-namespace references require a ReferenceGrant and generates one if needed
7. Step 6 provides a summary with all generated manifests concatenated
8. Helper functions `getArg` and `getIntArgSkill` provide safe argument extraction with defaults

## Tasks / Subtasks

- [x] Create pkg/skills/gateway_expose.go with ExposeServiceSkill struct (embeds skillBase)
- [x] Implement Definition() returning skill name, description, required CRDs (gateway.networking.k8s.io), and 5 parameters (service_name, namespace, port, hostname, protocol)
- [x] Implement Execute() with 6-step workflow:
  - Step 1 (verify_service): Dynamic client GET on service, fail with critical finding if not found
  - Step 2 (detect_provider): Confirm Gateway API CRDs detected
  - Step 3 (check_gateway): List existing Gateways; use first found or generate Gateway manifest with protocol-appropriate listener port (80 for HTTP, 443 for HTTPS)
  - Step 4 (generate_route): Generate HTTPRoute or GRPCRoute with parentRef (conditional cross-namespace), optional hostname, backendRef with port
  - Step 5 (check_reference_grant): If gateway namespace differs from service namespace, generate ReferenceGrant; otherwise skip
  - Step 6 (complete): Summary with manifest count and concatenated output
- [x] Implement getArg helper for string argument extraction with default value
- [x] Implement getIntArgSkill helper for integer argument extraction with float64/int type switching

## Dev Notes

### 6-Step Workflow

| Step | Name | Action | Fail Behavior |
|---|---|---|---|
| 1 | verify_service | GET service via dynamic client | Hard fail, return result with status "failed" |
| 2 | detect_provider | Confirm Gateway API available | Always passes (skill only registered when CRDs present) |
| 3 | check_gateway | List gateways, use existing or generate | Warning if none found, generates Gateway manifest |
| 4 | generate_route | Create HTTPRoute/GRPCRoute YAML | Always succeeds |
| 5 | check_reference_grant | Cross-namespace check | Skipped if same namespace, warning + manifest if cross-namespace |
| 6 | complete | Summary | Always passes |

### Protocol-Aware Route Generation

The skill uses the `protocol` parameter (default: HTTP) to determine:
- **HTTP/HTTPS**: Generates `HTTPRoute` kind
- **GRPC**: Generates `GRPCRoute` kind
- Gateway listener port: 80 for HTTP, 443 for HTTPS

### Cross-Namespace ReferenceGrant

When the gateway is in a different namespace than the service, Gateway API requires a ReferenceGrant in the service's namespace to allow the route to reference the backend service. The skill detects this condition and generates the appropriate ReferenceGrant manifest.

### GVR Definitions

Two package-level GVR variables are defined for dynamic client access:
```go
var svcGVR = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}
var gwGVR = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gateways"}
```

### Helper Functions

- `getArg(args, key, default)`: Extracts string argument, returns default if missing or empty
- `getIntArgSkill(args, key, default)`: Extracts integer argument, handles JSON number (float64) and Go int types

## File List

| File | Action |
|---|---|
| `pkg/skills/gateway_expose.go` | Created |
