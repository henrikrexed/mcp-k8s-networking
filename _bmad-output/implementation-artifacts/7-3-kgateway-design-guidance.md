# Story 7.3: kgateway Design Guidance

Status: done

## Story

As an AI agent,
I want to generate kgateway-specific configuration YAML based on user intent,
so that I can guide users through setting up RouteOption, VirtualHostOption, and GatewayParameters resources.

## Acceptance Criteria

1. The design_kgateway tool generates annotated YAML for RouteOption, VirtualHostOption, and GatewayParameters resources
2. Intent detection from parameters and free-text determines which resources to generate (rate limiting, headers, timeouts for RouteOption; CORS for VirtualHostOption; gateway params for GatewayParameters)
3. The tool checks for existing RouteOption resources in the target namespace
4. Generated resources use the correct kgateway API group (gateway.kgateway.dev/v1alpha1) and targetRef patterns
5. The tool is conditionally registered when kgateway CRDs are detected

## Tasks / Subtasks

- [x] Task 1: Implement intent detection (AC: 2)
  - [x] Detect RouteOption intent from resource_type="routeoption" or "rate"/"header"/"timeout"/"retry" in intent
  - [x] Detect VirtualHostOption intent from resource_type="virtualhostoption" or "cors"/"virtualhost" in intent
  - [x] Detect GatewayParameters intent from resource_type="gatewayparameters" or gateway_name provided or "gateway param" in intent
  - [x] Default to RouteOption when route_name provided and no other type detected
- [x] Task 2: Implement existing resource discovery (AC: 3)
  - [x] List existing RouteOptions in namespace via `routeOptionGVR` from kgateway.go
  - [x] Add info finding showing count of existing RouteOptions
- [x] Task 3: Generate RouteOption YAML (AC: 1, 4)
  - [x] Use targetRefs pointing to HTTPRoute with Gateway API group
  - [x] Derive route name from route_name param, or service_name + "-route", or fallback "my-route"
  - [x] Include commented-out example options (timeout, retries) as guidance
- [x] Task 4: Generate VirtualHostOption YAML (AC: 1, 4)
  - [x] Use targetRefs pointing to Gateway
  - [x] Use gateway_name if provided, else fallback to "main-gateway"
  - [x] Include commented-out CORS example as guidance
- [x] Task 5: Generate GatewayParameters YAML (AC: 1, 4)
  - [x] Include kube deployment/service configuration with replicas and service type
  - [x] Include commented-out envoyContainer resource example
  - [x] Add suggestion about kgateway.dev/gateway-parameters annotation
- [x] Task 6: Implement fallback and combined output
  - [x] Show info finding with parameter guidance when no resources generated
  - [x] Generate combined YAML summary with `---` separator
- [x] Task 7: Register tool in main.go (AC: 5)
  - [x] Register inside `features.HasKgateway` conditional block
  - [x] Include "design_kgateway" in kgatewayToolNames for unregistration

## Dev Notes

### Key Design Decisions

- **kgateway API group**: All generated resources use `gateway.kgateway.dev/v1alpha1`, the correct API group for kgateway-specific CRDs. This differentiates from standard Gateway API resources.
- **GVRs from kgateway.go**: Uses `routeOptionGVR` for existing resource discovery. The `vhostOptionGVR` and `gatewayParamsGVR` GVRs are also defined in kgateway.go for future use.
- **Commented-out options as templates**: Rather than generating specific policy content (which varies wildly by use case), the tool generates the resource skeleton with commented-out examples. This gives agents and users a starting point.
- **targetRefs pattern**: Both RouteOption and VirtualHostOption use the `targetRefs` pattern pointing to Gateway API resources (HTTPRoute and Gateway respectively), following kgateway's policy attachment model.
- **Route name derivation chain**: route_name param > service_name + "-route" > "my-route" fallback. This provides sensible defaults while allowing explicit specification.
- **Inline anonymous function for gateway name**: VirtualHostOption uses an inline function to select gwName or "main-gateway" default, keeping the YAML template clean.

### Files Created

| File | Purpose |
|---|---|
| `pkg/tools/design_kgateway.go` | DesignKgatewayTool implementation |

### Files Modified

| File | Action |
|---|---|
| `cmd/server/main.go` | Registered DesignKgatewayTool conditionally under HasKgateway |

## Dev Agent Record

### Agent Model Used
claude-opus-4-6

### Completion Notes List
- Uses routeOptionGVR from kgateway.go for existing RouteOption discovery
- All findings use CategoryRouting
- The tool generates 0-3 resources per invocation depending on detected intent
- The tool does not apply resources; it only generates advisory YAML

### File List
- pkg/tools/design_kgateway.go
- cmd/server/main.go
