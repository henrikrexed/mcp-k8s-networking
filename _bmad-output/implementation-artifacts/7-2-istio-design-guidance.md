# Story 7.2: Istio Design Guidance

Status: done

## Story

As an AI agent,
I want to generate Istio configuration YAML based on user intent,
so that I can guide users through setting up mTLS (PeerAuthentication), traffic splitting (VirtualService + DestinationRule), and access control (AuthorizationPolicy).

## Acceptance Criteria

1. The design_istio tool detects user intent from parameters and free-text intent field to determine which resources to generate
2. mTLS intent generates PeerAuthentication with configurable mode (STRICT/PERMISSIVE/DISABLE) and checks for existing PA conflicts
3. Traffic split intent generates DestinationRule with version-based subsets and VirtualService with weighted routes
4. Weight validation warns when traffic split weights do not sum to 100%
5. Access control intent generates AuthorizationPolicy with namespace-based source rules
6. The tool is conditionally registered when Istio CRDs are detected

## Tasks / Subtasks

- [x] Task 1: Implement intent detection (AC: 1)
  - [x] Detect mTLS intent from mtls_mode parameter or "mtls"/"tls" in intent text
  - [x] Detect traffic split intent from traffic_split parameter or "traffic"/"canary"/"split" in intent text
  - [x] Detect auth policy intent from allowed_sources parameter or "restrict"/"authz"/"access" in intent text
- [x] Task 2: Implement PeerAuthentication generation (AC: 2)
  - [x] Check existing PeerAuthentication in namespace via `paV1GVR` from istio.go
  - [x] Add warning finding for each existing PA that may conflict
  - [x] Default mtls_mode to STRICT when intent detected but mode not specified
  - [x] Generate namespace-wide PA when no service_name, or service-scoped PA with selector matchLabels
  - [x] Use `peerAuthName()` helper for consistent naming
- [x] Task 3: Implement traffic splitting generation (AC: 3, 4)
  - [x] Parse traffic_split string "v1:80,v2:20" via `parseTrafficSplit()` helper
  - [x] Default to v1:80/v2:20 split when intent detected but no split provided
  - [x] Generate DestinationRule with version-labeled subsets
  - [x] Generate VirtualService with weighted route destinations
  - [x] Validate total weight and warn if not 100%
- [x] Task 4: Implement AuthorizationPolicy generation (AC: 5)
  - [x] Parse comma-separated allowed_sources into namespace-based source rules
  - [x] Default to "default" namespace when intent detected but no sources provided
  - [x] Generate ALLOW action policy with selector matching app label
- [x] Task 5: Implement fallback and combined output
  - [x] Show info finding when no resources generated, with parameter guidance
  - [x] Generate combined YAML summary with `---` separator when resources produced
- [x] Task 6: Register tool in main.go (AC: 6)
  - [x] Register inside `features.HasIstio` conditional block
  - [x] Include "design_istio" in istioToolNames for unregistration

## Dev Notes

### Key Design Decisions

- **Intent detection from text and parameters**: Both explicit parameters (mtls_mode, traffic_split, allowed_sources) and free-text intent field are checked. This allows agents to pass either structured parameters or natural language descriptions.
- **Existing PA conflict detection**: Before generating a new PeerAuthentication, the tool lists existing ones in the namespace via `paV1GVR` (from istio.go) and adds warning findings for potential conflicts. This prevents accidental overrides.
- **parseTrafficSplit helper**: Parses "subset:weight" comma-separated format into structured trafficEntry slices. Uses `fmt.Sscanf` for weight parsing with graceful skip on malformed entries.
- **Weight validation**: Total weight is summed and a warning finding is emitted if it does not equal 100%. This catches common misconfiguration.
- **peerAuthName helper**: Returns `{svcName}` for service-scoped or `{ns}-default` for namespace-wide PA, ensuring consistent naming.
- **AuthorizationPolicy defaults**: When intent is detected but no allowed_sources specified, defaults to allowing the "default" namespace, providing a working example for the agent to customize.

### Files Created

| File | Purpose |
|---|---|
| `pkg/tools/design_istio.go` | DesignIstioTool, peerAuthName, trafficEntry, parseTrafficSplit |

### Files Modified

| File | Action |
|---|---|
| `cmd/server/main.go` | Registered DesignIstioTool conditionally under HasIstio |

## Dev Agent Record

### Agent Model Used
claude-opus-4-6

### Completion Notes List
- Uses paV1GVR from istio.go for PeerAuthentication API access
- Three independent resource generators (PA, DR+VS, AP) can produce 0-4 resources per invocation
- CategoryTLS for mTLS findings, CategoryRouting for traffic split, CategoryPolicy for auth policy
- The tool does not apply resources; it only generates advisory YAML

### File List
- pkg/tools/design_istio.go
- cmd/server/main.go
