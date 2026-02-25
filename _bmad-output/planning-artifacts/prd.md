---
stepsCompleted: ['step-01-init', 'step-02-discovery', 'step-02b-vision', 'step-02c-executive-summary', 'step-03-success', 'step-04-journeys', 'step-05-domain', 'step-06-innovation', 'step-07-project-type', 'step-08-scoping', 'step-09-functional', 'step-10-nonfunctional', 'step-11-polish', 'step-e-01-discovery', 'step-e-02-review', 'step-e-03-edit']
inputDocuments: ['product-brief-mcp-k8s-networking-2026-02-22.md', 'prd-validation-report.md']
workflowType: 'prd'
documentCounts:
  briefs: 1
  research: 0
  brainstorming: 0
  projectDocs: 0
  projectContext: 0
classification:
  projectType: developer_tool
  domain: cloud-infrastructure-devops
  complexity: medium-high
  projectContext: greenfield
lastEdited: '2026-02-22'
editHistory:
  - date: '2026-02-22'
    source: 'prd-validation-report.md'
    changes: '17 changes from validation findings — 8 FR rewrites (FR2, FR10, FR15, FR17, FR18, FR35, FR36, FR37) adding measurable specifics and input/output contracts, 1 new FR (FR50 remediation), 7 NFR quantifications (NFR1, NFR7, NFR12-NFR14, NFR16-NFR18), Journey 3 post-MVP annotation, design guidance scope clarification'
---

# Product Requirements Document - mcp-k8s-networking

**Author:** Henrik.rexed
**Date:** 2026-02-22

## Executive Summary

mcp-k8s-networking is an in-cluster MCP server that makes AI agents the Kubernetes networking expert no team has time to be. Deployed as a workload on each cluster, it exposes the full K8s networking stack — routing, service mesh, Gateway API, TLS, kube-proxy, and CoreDNS — as structured, token-efficient diagnostic tools. It operates in two modes: **diagnostic mode**, where agents systematically troubleshoot networking issues across the full stack and across clusters; and **design mode**, where agents use codified skills to generate correct networking configurations (Gateways, HTTPRoutes, GRPCRoutes, mesh policies) tailored to the user's chosen provider (Envoy, Istio, kgateway, etc.).

The primary consumers are AI agents — not humans directly. Platform engineers, SREs, and developers interact through their agents, which query the MCP for diagnostics and configuration guidance. Output is compact by default with progressive detail on demand, optimized for minimal token consumption.

### What Makes This Special

1. **Diagnostic + Design in one tool**: No existing solution both troubleshoots networking issues AND helps create correct configurations. Cilium Hubble shows flows, Kiali visualizes meshes, kagent handles partial Istio diagnostics — none generate validated Gateway API manifests or guide configuration authoring.
2. **Full networking stack coverage**: Not mesh-specific or layer-specific. Covers routing, service mesh, Gateway API (including GAMMA and inference extensions), TLS, kube-proxy, and CoreDNS holistically.
3. **Living Gateway API knowledge**: The Gateway API ecosystem evolves rapidly. mcp-k8s-networking includes a process to track API changes, new CRDs, and new provider implementations — acting as the always-current expert.
4. **Agent-first, token-efficient**: Built for AI agent consumption from the ground up. Compact default output with detail-on-demand. Codified troubleshooting skills (skills.md) that agents execute as structured playbooks.
5. **Active diagnostic probing**: Deploys ephemeral pods for curl and network checks — going beyond passive observation to actively test connectivity and configurations.

## Project Classification

- **Project Type:** Developer tool — in-cluster MCP server workload exposing diagnostic and design tools via MCP protocol
- **Domain:** Cloud infrastructure / DevOps tooling (Kubernetes networking)
- **Complexity:** Medium-high — technical complexity from full networking stack coverage and rapidly evolving Gateway API ecosystem; no regulatory compliance requirements
- **Project Context:** Greenfield — open-source, community-focused (CNCF ecosystem)

## Success Criteria

### User Success

- **Correct, non-hallucinated output**: Generated manifests and diagnostic results are accurate and based on actual cluster state — not fabricated. Zero tolerance for hallucinated resources or configurations.
- **Time-saving diagnostics**: Platform engineers resolve networking issues in minutes instead of hours. The agent identifies root causes on the first diagnostic run across the full stack.
- **Reduced engineering dependency**: Developers design and deploy networking configurations without requiring platform team involvement. Correct manifests generated based on cluster's actual CRDs and installed providers.
- **Design guidance from cluster state**: The MCP introspects available CRDs in the cluster to provide accurate, context-aware design guidelines — no assumptions about what's installed.

### Business Success

- **CNCF project recognition**: Accepted as a CNCF project (sandbox or higher)
- **Community adoption**: Positive community feedback, active GitHub engagement (stars, issues, contributions)
- **Ecosystem credibility**: Recognized at KubeCon and within the cloud-native community as the go-to K8s networking diagnostic tool for AI agents

### Technical Success

- **Accurate cluster introspection**: Reliable discovery of installed CRDs, mesh providers, CNI plugins, and Gateway API implementations
- **Token-efficient output**: Compact default responses that minimize agent token consumption while providing actionable diagnostic data
- **Active probing reliability**: Ephemeral diagnostic pods deploy, execute, and clean up reliably without impacting cluster workloads

### Measurable Outcomes

- Diagnostic accuracy: Root cause correctly identified in networking issues (target: >90% accuracy)
- Manifest correctness: Generated configurations are valid and deployable without modification (target: zero hallucination)
- Community: Achieve CNCF project status within 12 months
- Adoption: Multiple organizations using in production within 12 months

## User Journeys

### Journey 1: Sara — "The App Is Not Responding" (Diagnostic Path)

**Opening Scene:** Sara gets paged at 2pm — a critical microservice is returning 503 errors. She's managing 4 clusters, and the affected service communicates across two of them. She opens her AI agent and describes the problem.

**Rising Action:** The agent queries the MCP server on the affected cluster. First call: compact diagnostic summary across the full networking stack. The MCP introspects services, endpoints, Istio mesh policies, Gateway API routes, CoreDNS resolution, and kube-proxy health. Within seconds, the agent reports: "Service endpoints are healthy. Istio AuthorizationPolicy `deny-external` is blocking traffic from the second cluster's gateway. The policy was updated 45 minutes ago."

**Climax:** Sara asks the agent to show the misconfigured policy. The agent requests detail-level output from the MCP, gets the full policy YAML, and highlights the specific rule that's too restrictive. The agent suggests a corrected policy that allows cross-cluster gateway traffic while maintaining the intended security posture.

**Resolution:** Sara applies the fix. Total time from page to resolution: 8 minutes. Previously, this would have taken 2-3 hours of manual kubectl debugging across clusters. She notes that the MCP's active probing confirmed connectivity was restored before she closed the incident.

**Requirements Revealed:** Full stack diagnostic tools, Istio policy analysis, compact/detail output modes, active connectivity probing, suggested fix generation.

---

### Journey 2: Marcus — "Expose My gRPC Service" (Design Guidance Path)

**Opening Scene:** Marcus has built a new gRPC payment service and needs to expose it through the cluster's networking stack. He knows his Go code but not the networking configs. He opens his coding agent, which has the MCP skills configured.

**Rising Action:** Marcus tells the agent: "I need to expose my gRPC payment-service on port 9090." The agent, using MCP skills, queries the cluster's available CRDs and discovers Istio is installed with Gateway API support. The agent asks Marcus: "Your cluster has Istio with Gateway API. Do you want to use an existing Gateway or create a new one?" Marcus picks the existing ingress gateway.

**Climax:** The agent generates the complete set of manifests — a GRPCRoute pointing to his service, the correct backend reference, and the necessary Istio DestinationRule for mTLS. All based on what's actually installed in the cluster, not generic templates. Marcus reviews the manifests — they reference his actual service name, port, and namespace.

**Resolution:** Marcus applies the manifests and his gRPC service is reachable. No ticket filed to the platform team. No trial-and-error. The MCP's CRD introspection ensured the generated configs match the cluster's actual setup — zero hallucination.

**Requirements Revealed:** CRD introspection, provider-aware manifest generation, design skills for Gateway API / GRPCRoute / mesh policies, interactive agent workflow.

---

### Journey 3: Priya — "Validate the Multi-Cluster Architecture" (Architecture Review Path) — *Post-MVP (Phase 2/3)*

**Opening Scene:** Priya has designed a new multi-cluster traffic routing architecture using Gateway API with Istio. Before rolling it out, she wants to validate that the existing cluster configurations can support her design — and that security policies and OpenTelemetry instrumentation are properly layered.

**Rising Action:** Priya asks her agent to audit the current networking state across both clusters. The agent queries each cluster's MCP server for a compact summary. Cluster A shows a healthy Gateway API setup with Istio. Cluster B reveals a TLS certificate mismatch on the cross-cluster gateway and missing OpenTelemetry sidecar injection on two critical services.

**Climax:** Priya drills into the detail on Cluster B. The agent retrieves full diagnostic data — the TLS cert was issued for the wrong domain, and the OTel sidecar injection annotation is missing from two deployment specs. The agent suggests the corrected cert configuration and the required annotations.

**Resolution:** Priya addresses the issues before rollout, preventing what would have been a silent traffic failure in production. She validates her architecture is correctly implemented across both clusters — networking, security, and observability aligned.

**Requirements Revealed:** Multi-cluster diagnostic queries, TLS certificate validation, integration awareness (OpenTelemetry), architecture-level validation, cross-cluster correlation.

---

### Journey 4: AI Agent — "Systematic Diagnostic Workflow" (Agent-as-Primary-Consumer)

**Opening Scene:** An AI agent receives a user request: "My checkout service can't reach the inventory API." The agent has access to the mcp-k8s-networking MCP server on the cluster.

**Rising Action:** The agent follows a systematic diagnostic workflow:
1. **Service discovery**: Calls MCP to check the inventory API service exists, has endpoints, and selectors match running pods. Result: compact summary — service healthy, 3 endpoints active.
2. **Connectivity probe**: Calls MCP to deploy an ephemeral pod and test connectivity from checkout namespace to inventory API. Result: connection refused on port 8080.
3. **Network policy check**: Calls MCP to inspect NetworkPolicies in both namespaces. Result: Calico NetworkPolicy in inventory namespace blocks ingress from checkout namespace.
4. **Root cause identified**: The agent correlates the findings — the NetworkPolicy is the blocker.

**Climax:** The agent generates a concise diagnostic report: root cause identified, evidence from each diagnostic step, and a suggested NetworkPolicy update that allows traffic from the checkout namespace while maintaining other restrictions.

**Resolution:** The agent presents the finding and fix to the user in plain language. Total MCP calls: 4. Total tokens consumed: minimal (compact mode throughout, detail only on the NetworkPolicy finding). The structured output format allowed the agent to reason efficiently without parsing walls of text.

**Requirements Revealed:** Structured tool responses, ephemeral pod probing, NetworkPolicy analysis (Calico), service/endpoint validation, token-efficient compact output, suggested remediation.

---

### Journey Requirements Summary

| Capability Area | Sara (Diagnostic) | Marcus (Design) | Priya (Architecture) | AI Agent (Systematic) |
|---|---|---|---|---|
| Service mesh diagnostics (Istio/Kuma/Linkerd) | ✓ | | ✓ | ✓ |
| Gateway API inspection & generation | ✓ | ✓ | ✓ | |
| CRD introspection | | ✓ | ✓ | |
| CNI / NetworkPolicy analysis | | | | ✓ |
| CoreDNS diagnostics | ✓ | | | ✓ |
| kube-proxy health | ✓ | | | ✓ |
| TLS certificate validation | | | ✓ | |
| Active probing (ephemeral pods) | ✓ | | | ✓ |
| Compact/detail output modes | ✓ | | ✓ | ✓ |
| Design guidance / manifest generation | | ✓ | | |
| Multi-cluster correlation | | | ✓ | |
| Suggested remediations | ✓ | | ✓ | ✓ |
| OTel / instrumentation awareness | | | ✓ | |

## Domain-Specific Requirements

### RBAC & Security Model

- **Cluster-wide read access** on all networking-related resources and CRDs from covered providers (see detailed ClusterRole spec in Developer Tool Specific Requirements)
- **Diagnostic pod deployment**: Create/delete permissions for ephemeral probing pods
- **Log and ConfigMap access**: Read access to pod logs and ConfigMaps (CoreDNS, kube-proxy, mesh configs) across namespaces
- **CRD discovery**: List and inspect all installed CRDs to determine cluster capabilities

### Cluster Stability & Safety

- **Ephemeral pod management**: Diagnostic pods must have resource limits enforced, TTL/timeout for automatic cleanup, and run in a dedicated namespace
- **Non-destructive by default**: The MCP reads and inspects — it does not modify existing resources. Manifest generation provides recommendations; the user/agent applies changes
- **Concurrent probe limits**: Cap on simultaneous diagnostic pods to prevent resource pressure on the cluster

### Kubernetes Compatibility

- **Minimum version**: Kubernetes 1.28+
- **Graceful degradation**: If Gateway API CRDs are not installed, the MCP detects the absence and the agent suggests installation — no errors, just guidance
- **CRD-aware operation**: The MCP adapts its available tools based on what's actually installed. No Istio? Istio diagnostic tools are unavailable but don't error. Cilium detected? Cilium-specific diagnostics activate.
- **API version handling**: Support for both stable and beta API versions of Gateway API and mesh CRDs as they evolve

### Open Source & CNCF Alignment

- **License**: Apache 2.0
- **CNCF sandbox target**: Project governance, contributor guidelines, and documentation standards aligned with CNCF sandbox acceptance criteria
- **Community-first development**: Public roadmap, open issue tracking, contribution-friendly architecture with clear extension points for adding new CNI/mesh provider support

## Innovation & Novel Patterns

### Detected Innovation Areas

1. **MCP-first K8s networking paradigm**: The first tool purpose-built for AI agents to diagnose and design Kubernetes networking. Not a human tool retrofitted for agents — agent consumption is the primary design constraint.
2. **Diagnostic + Design convergence**: No existing tool combines networking troubleshooting with configuration generation. mcp-k8s-networking does both, informed by the same cluster-state awareness.
3. **CRD introspection as dynamic intelligence**: Rather than hardcoding provider knowledge, the MCP discovers what's installed and adapts its capabilities accordingly. This makes it inherently extensible and provider-agnostic.
4. **Token-efficiency as a design principle**: Output format optimized for minimal AI agent token consumption — a differentiator that no competing tool considers because they target human consumers.

### Market Context & Competitive Landscape

- **kagent**: Closest competitor — partial Istio diagnostic support but weak on Gateway API and limited to single-mesh focus
- **Cilium Hubble / Kiali**: Observability and visualization tools — show traffic but don't diagnose or generate configurations
- **No MCP-first competitor exists**: The K8s networking MCP space is effectively greenfield. First-mover advantage in defining how agents interact with networking diagnostics.

### Validation Approach

- **Live cluster validation**: Test against a real cluster running Istio with Gateway API, using predefined misconfiguration scenarios on Gateway API resources
- **Scenario-based testing**: Multiple known misconfigurations deployed and validated — the MCP must correctly identify root causes and suggest fixes for each
- **Agent end-to-end test**: Full diagnostic workflow where an AI agent uses the MCP to diagnose a complex issue without human intervention
- **Community feedback loop**: Early access to CNCF community members for real-world validation and feedback

### Risk Mitigation

- **MCP-first commitment**: No CLI fallback — this keeps the product focused and prevents scope dilution. If the MCP protocol evolves, the tool evolves with it.
- **Networking focus with security expansion path**: Core scope is networking only. Security diagnostics (RBAC policies, pod security standards) is a natural adjacent expansion but not in MVP.
- **Provider-agnostic architecture**: CRD introspection ensures the tool doesn't break when providers release new versions or new providers emerge.

## Developer Tool Specific Requirements

### Project-Type Overview

mcp-k8s-networking is a Go-based MCP server deployed as an in-cluster workload, communicating with AI agents via SSE (Server-Sent Events) over HTTP using the JSON-RPC 2.0 MCP protocol. It uses client-go with dynamic client for CRD introspection and adapts its available tools dynamically based on what networking components are installed in the cluster.

### Technical Architecture Considerations

**Language & Runtime:**
- Go 1.22+ — aligns with K8s ecosystem conventions and client-go compatibility
- client-go with dynamic client for CRD discovery and resource inspection
- Distroless container base image for minimal attack surface

**MCP Protocol Implementation:**
- Transport: SSE (Server-Sent Events) over HTTP
- Protocol: JSON-RPC 2.0 over SSE per MCP specification
- SSE connection management: heartbeats, reconnection handling, graceful disconnect
- Structured JSON-RPC responses with compact/detail output modes

**CRD Discovery Mechanism:**
- On startup, discover which CRDs are installed using the API server's discovery endpoint
- Support detection of: `gateway.networking.k8s.io`, `networking.istio.io`, `security.istio.io`, `cilium.io`, `networking.k8s.io`, core v1 resources
- Re-discover periodically or on-demand to detect newly installed/removed CRDs
- Graceful handling when CRDs don't exist — tools for unavailable providers simply aren't registered

**Dynamic Tool Registry:**
- Tools registered dynamically based on discovered CRDs
- Not all clusters have Istio + Cilium + Gateway API — the registry adapts
- Tool registration/deregistration without server restart
- Clear tool categorization: diagnostic tools vs. design guidance tools

**Resource Access Strategy:**
- Architecture decision: informers (cached, real-time) vs. on-demand API calls
- Informers for frequently accessed resources (Services, Endpoints, Pods)
- On-demand dynamic client calls for CRD resources (less predictable, more varied)
- Namespace filtering: support both cluster-wide and namespace-scoped queries

**RBAC Requirements (ClusterRole — read-only):**
- `gateway.networking.k8s.io` — Gateway API resources (Gateways, HTTPRoutes, GRPCRoutes, ReferenceGrants)
- `networking.istio.io` — Istio networking (VirtualService, DestinationRule, Gateway, ServiceEntry)
- `security.istio.io` — Istio security (AuthorizationPolicy, PeerAuthentication, RequestAuthentication)
- `cilium.io` — Cilium resources (CiliumNetworkPolicy, CiliumClusterwideNetworkPolicy)
- `networking.k8s.io` — NetworkPolicy, Ingress
- Core v1 — Services, Endpoints, Pods, ConfigMaps (CoreDNS, kube-proxy config)
- Additional: create/delete pods in diagnostic namespace for active probing

### Installation & Distribution

**Helm Chart (primary):**
- Configurable RBAC scope, namespace targeting, resource limits
- Values for enabling/disabling provider-specific tool modules
- ServiceAccount, ClusterRole, ClusterRoleBinding bundled

**Raw YAML Manifests:**
- For users who prefer kubectl apply without Helm
- Single-file deployment option for quick setup

### Project Structure

```
k8s-networking-mcp/
├── cmd/
│   └── server/
│       └── main.go
├── pkg/
│   ├── mcp/              # MCP protocol implementation
│   │   ├── server.go     # SSE server, JSON-RPC handler
│   │   ├── types.go      # MCP protocol types
│   │   └── transport.go  # SSE transport
│   ├── tools/            # Tool implementations
│   │   ├── registry.go   # Dynamic tool registration
│   │   ├── gateway/      # Gateway API tools
│   │   ├── istio/        # Istio tools
│   │   ├── cilium/       # Cilium tools
│   │   └── k8s/          # Core K8s networking tools
│   ├── k8s/              # K8s client setup
│   │   ├── client.go     # Client factory
│   │   └── discovery.go  # CRD discovery
│   └── config/           # Configuration
│       └── config.go
├── deploy/
│   ├── helm/             # Helm chart
│   └── manifests/        # Raw YAML
├── Dockerfile
├── Makefile
├── go.mod
└── README.md
```

### Implementation Considerations

**Operational Requirements:**
- Structured logging (JSON format for cluster log aggregation)
- Optional `/metrics` endpoint for the MCP server itself (Prometheus-compatible)
- Graceful shutdown with proper SSE connection draining
- Health/readiness probes for K8s deployment

**Architecture Decisions to Resolve:**
1. Informers vs. on-demand API calls — hybrid approach recommended (informers for core resources, dynamic client for CRDs)
2. CRD re-discovery interval — configurable, default every 5 minutes or on tool invocation failure
3. SSE heartbeat interval and reconnection backoff strategy
4. Namespace filtering: default cluster-wide with optional namespace allowlist/denylist
5. Error handling: CRD not found returns informative "provider not installed" response, not an error
6. Caching TTL for diagnostic results to avoid redundant API calls within a diagnostic session

**Documentation Strategy:**
- README with quickstart (deploy in under 5 minutes)
- Architecture documentation for contributors
- Tool reference: each MCP tool with input/output schemas and examples
- Diagnostic scenario cookbook: common networking issues and how agents use the MCP to resolve them
- CNCF-aligned: CONTRIBUTING.md, CODE_OF_CONDUCT.md, GOVERNANCE.md

## Project Scoping & Phased Development

### MVP Strategy & Philosophy

**MVP Approach:** Problem-solving MVP — prove that an MCP server can deliver accurate, agent-consumable K8s networking diagnostics that save real time. Focus depth on the most complex and widely-adopted networking stack (Istio + Gateway API + kgateway) while providing basic coverage across the broader ecosystem.

**Resource Requirements:** Solo developer (Henrik.rexed) with deep K8s networking expertise. Go development, client-go, MCP protocol implementation.

### MVP Feature Set (Phase 1)

**Core User Journeys Supported:**
- Sara's diagnostic path (Journey 1) — full depth for Istio/Gateway API/kgateway
- AI Agent's systematic diagnostic workflow (Journey 4) — structured tool responses, compact/detail modes
- Marcus's design guidance path (Journey 2) — CRD-aware guidance for Istio/Gateway API/kgateway

**Deep Coverage (Tier 1 — full diagnostics + design guidance):**
- **Istio**: Configuration validation, policy diagnostics (AuthorizationPolicy, PeerAuthentication), traffic routing analysis (VirtualService, DestinationRule), sidecar injection checks, mTLS validation
- **Gateway API**: Gateway, HTTPRoute, GRPCRoute, ReferenceGrant inspection; misconfiguration detection; conformance validation against spec
- **kgateway**: Provider-specific diagnostics and configuration validation
- **CRD-aware design guidance**: Introspect installed CRDs, generate annotated YAML templates and contextual design guidance for Tier 1 providers based on user intent and cluster state (see FR35-FR37)

**Basic Coverage (Tier 2 — visibility + status):**
- **Kuma**: Detect installation, read policies, report mesh status
- **Linkerd**: Detect installation, read configuration, report health
- **Cilium**: NetworkPolicy inspection, basic connectivity status
- **Calico**: NetworkPolicy inspection, basic status
- **Flannel**: Detect installation, basic health check

**Core K8s Networking (full coverage):**
- **CoreDNS**: Configuration inspection, resolution diagnostics
- **kube-proxy**: Health checks, configuration validation
- **Services/Endpoints**: Selector matching, endpoint health, port validation
- **NetworkPolicies**: Policy analysis across namespaces

**Infrastructure:**
- Dynamic CRD discovery and tool registry
- MCP protocol (SSE + JSON-RPC 2.0)
- Compact/detail output modes
- Active probing (ephemeral pods for curl, DNS checks)
- Helm chart + raw YAML deployment
- Structured logging, health probes, graceful shutdown

### Post-MVP Features

**Phase 2 (Growth):**
- **Full design mode skills**: Complete manifest generation (Gateways, HTTPRoutes, GRPCRoutes, mesh policies) with provider-aware authoring
- **Deep Tier 2 coverage**: Promote Kuma, Linkerd, Cilium, Calico to full diagnostic depth
- **Troubleshooting playbooks (skills.md)**: Codified diagnostic workflows as agent-executable skills
- **TLS certificate diagnostics**: Deep cert chain validation, expiry detection, domain mismatch analysis
- **Optional /metrics endpoint**: Prometheus-compatible metrics for the MCP server itself

**Phase 3 (Expansion):**
- **Multi-cluster orchestration**: Cross-cluster diagnostic correlation and unified findings
- **Gateway API evolution tracking**: Automated process to track API changes, new CRDs, new provider implementations
- **Security expansion**: RBAC policy diagnostics, pod security standards validation
- **OpenTelemetry awareness**: Instrumentation validation (sidecar injection, collector configuration)
- **Comprehensive design assistant**: Generate full networking topologies from high-level intent
- **Support for emerging paradigms**: eBPF-native networking, ambient mesh, Gateway API inference extensions

### Risk Mitigation Strategy

**Technical Risks:**
- *Dynamic CRD discovery reliability* — Mitigate with comprehensive test scenarios on the live Istio + Gateway API cluster. This is the core architectural bet; validate early.
- *MCP protocol compliance* — Follow MCP spec strictly; test with multiple AI agent implementations (Claude, others)
- *client-go version compatibility* — Pin to stable client-go version aligned with K8s 1.28+

**Market Risks:**
- *MCP adoption pace* — MCP is the emerging standard but still early. Mitigate by validating with real community users at KubeCon and through blog posts.
- *kagent competition* — Mitigate by going deeper on Gateway API and full-stack coverage where kagent is weak.

**Resource Risks:**
- *Solo developer* — Focus MVP on Tier 1 providers only. Tier 2 basic coverage is low effort (CRD read + status). Community contributions can accelerate Tier 2 depth in Phase 2.
- *Scope creep* — The two-tier model provides a clear boundary. If time is tight, Tier 2 providers can ship as "experimental" with minimal diagnostics.

## Functional Requirements

### CRD Discovery & Cluster Introspection

- FR1: The MCP server can discover which networking CRDs are installed in the cluster on startup
- FR2: The MCP server can re-discover installed networking CRDs at a configurable interval (default: 5 minutes) and immediately upon a tool invocation that fails due to a missing CRD, reflecting changes in the tool registry within one re-discovery cycle
- FR3: The MCP server can report the complete list of detected networking providers and their versions to the agent
- FR4: The MCP server can adapt its available tools dynamically based on which CRDs are present without requiring restart
- FR5: The MCP server can detect the absence of Gateway API CRDs and inform the agent that Gateway API is not installed with a suggestion to install it

### Istio Diagnostics (Tier 1)

- FR6: The agent can request validation of Istio VirtualService and DestinationRule configurations
- FR7: The agent can request analysis of Istio AuthorizationPolicy and PeerAuthentication policies to identify overly restrictive or misconfigured rules
- FR8: The agent can request verification of Istio sidecar injection status across deployments in specified namespaces
- FR9: The agent can request mTLS validation between services to confirm encryption is properly configured
- FR10: The agent can request Istio traffic routing analysis that evaluates VirtualService route rules, DestinationRule subset definitions, and traffic splitting weights to detect: routes referencing non-existent subsets or services, weight allocations that do not sum to 100%, unreachable rules shadowed by higher-priority matches, and conflicts between routing and AuthorizationPolicy deny rules

### Gateway API Diagnostics (Tier 1)

- FR11: The agent can request inspection of Gateway resources and their status conditions
- FR12: The agent can request validation of HTTPRoute configurations including backend references, filters, and matching rules
- FR13: The agent can request validation of GRPCRoute configurations including backend references and method matching
- FR14: The agent can request ReferenceGrant inspection to identify cross-namespace reference issues
- FR15: The agent can request a misconfiguration scan across Gateway API resources that detects at minimum: routes referencing non-existent backend Services, routes attached to non-existent or non-matching Gateways, cross-namespace references missing required ReferenceGrants, Gateways with unresolved listener conflicts (port/protocol collisions), and routes with invalid filter configurations — each finding includes resource name, namespace, issue description, and suggested fix
- FR16: The agent can request conformance validation of Gateway API resources against the spec

### kgateway Diagnostics (Tier 1)

- FR17: The agent can request validation of kgateway provider-specific resources including GatewayParameters, RouteOption, and VirtualHostOption to detect: invalid or unresolvable upstream references, route option conflicts between overlapping VirtualHostOption resources, and GatewayParameters referencing non-existent Kubernetes resources
- FR18: The agent can request a health summary of the kgateway installation that reports: kgateway control plane pod status and readiness, translation status of kgateway resources (accepted, rejected, or errored as reported in resource status conditions), and data plane proxy health for Gateways managed by kgateway

### Core K8s Networking Diagnostics

- FR19: The agent can request Service and Endpoint validation including selector matching and port verification
- FR20: The agent can request CoreDNS configuration inspection and DNS resolution diagnostics
- FR21: The agent can request kube-proxy health checks and configuration validation
- FR22: The agent can request NetworkPolicy analysis across namespaces to identify blocking or permissive rules
- FR23: The agent can request Ingress resource inspection and validation

### Basic Provider Diagnostics (Tier 2)

- FR24: The agent can request basic Kuma mesh status and policy reporting
- FR25: The agent can request basic Linkerd configuration and health reporting
- FR26: The agent can request Cilium NetworkPolicy inspection and basic connectivity status
- FR27: The agent can request Calico NetworkPolicy inspection and basic status
- FR28: The agent can request Flannel installation detection and basic health reporting

### Active Diagnostic Probing

- FR29: The agent can request deployment of an ephemeral pod to test network connectivity between namespaces or services
- FR30: The agent can request deployment of an ephemeral pod to perform DNS resolution checks
- FR31: The agent can request deployment of an ephemeral pod to perform curl/HTTP checks against services
- FR32: The MCP server can enforce resource limits on ephemeral diagnostic pods
- FR33: The MCP server can automatically clean up ephemeral diagnostic pods after a configurable TTL
- FR34: The MCP server can limit concurrent diagnostic pods to prevent cluster resource pressure

### CRD-Aware Design Guidance

- FR35: Given a user intent (e.g., "expose service X on port Y via HTTPS"), the agent can request Gateway API design guidance that returns: the ordered list of required resources (Gateway, HTTPRoute/GRPCRoute, ReferenceGrant as needed) with annotated YAML templates populated with the user's service details, provider-specific annotations required by the installed Gateway API implementation, and warnings for missing prerequisites (e.g., missing TLS secret, missing ReferenceGrant) — all templates reference only API versions and CRDs confirmed present in the cluster
- FR36: Given a user intent (e.g., "enable mTLS between services", "route 80% traffic to v2"), the agent can request Istio configuration design guidance that returns: required Istio resources (PeerAuthentication, DestinationRule, VirtualService, AuthorizationPolicy as applicable) with annotated YAML templates, verification that referenced services, namespaces, and ports exist, and conflict warnings against existing Istio policies — all templates reference only Istio API versions installed in the cluster
- FR37: Given a user intent, the agent can request kgateway configuration design guidance that returns: required kgateway-specific resources (RouteOption, VirtualHostOption, GatewayParameters as applicable) with annotated YAML templates, verification that referenced upstreams and Gateways exist, and kgateway-specific best practices (e.g., delegation patterns) — all templates reference only kgateway CRD versions installed in the cluster
- FR38: The MCP server can provide contextual guidance that references only providers and API versions actually available in the cluster

### MCP Protocol & Agent Communication

- FR39: The MCP server can accept connections from AI agents via SSE (Server-Sent Events) over HTTP
- FR40: The MCP server can process JSON-RPC 2.0 requests per the MCP specification
- FR41: The MCP server can return compact diagnostic summaries by default for token-efficient agent consumption
- FR42: The agent can request detailed output for specific diagnostic findings when deeper analysis is needed
- FR43: The MCP server can register and expose available tools to agents based on discovered cluster capabilities
- FR44: The MCP server can maintain SSE connections with heartbeats and handle reconnection gracefully

### Diagnostic Remediation

- FR50: The agent can request suggested remediations for identified diagnostic issues across all diagnostic domains (Istio, Gateway API, kgateway, CoreDNS, NetworkPolicy, kube-proxy) — each suggestion includes the affected resource, a description of the corrective action, and when applicable an annotated YAML snippet showing the fix

### OpenTelemetry Self-Instrumentation

- FR-OTel-1: All MCP tool calls MUST produce spans following OTel MCP semantic conventions — each span carries `mcp.method.name`, `gen_ai.tool.name`, `gen_ai.operation.name="execute_tool"`, `mcp.protocol.version`, `mcp.session.id`, and `jsonrpc.request.id` as span attributes
- FR-OTel-2: Context propagation — the MCP server extracts `traceparent`/`tracestate` from MCP request `params._meta` to establish end-to-end traces spanning AI agent → MCP server → K8s API calls
- FR-OTel-3: GenAI metrics — the MCP server records `gen_ai.server.request.duration` histogram and `gen_ai.server.request.count` counter, both dimensioned by tool name and error type
- FR-OTel-4: Custom domain metrics — the MCP server records `mcp.findings.total` counter (by severity, analyzer/tool) and `mcp.errors.total` counter (by error code, tool name)
- FR-OTel-5: Structured logging via OTel log bridge — slog output is bridged to OTel Logs via `otelslog`, with automatic `trace_id`/`span_id` correlation on every log entry emitted within an active span context
- FR-OTel-6: Span attributes include `gen_ai.tool.call.arguments` (sanitized — no secrets, tokens, or certificate keys) and `gen_ai.tool.call.result` (truncated to 1024 characters) for diagnostic observability
- FR-OTel-7: Error spans set `error.type` attribute to the JSON-RPC error code (e.g., `PROVIDER_NOT_FOUND`, `INVALID_INPUT`) or `"tool_error"` for unclassified tool execution failures, and record the error as a span event
- FR-OTel-8: All three OTel signals (traces, metrics, logs) are exported via OTLP gRPC to the endpoint configured by `OTEL_EXPORTER_OTLP_ENDPOINT`, with noop providers when the endpoint is not set

### Deployment & Operations

- FR45: A cluster administrator can deploy the MCP server via Helm chart with configurable RBAC scope and resource limits
- FR46: A cluster administrator can deploy the MCP server via raw YAML manifests as an alternative to Helm
- FR47: The MCP server can expose health and readiness endpoints for Kubernetes probes
- FR48: The MCP server can produce structured JSON logs for cluster log aggregation
- FR49: The MCP server can perform graceful shutdown with proper SSE connection draining

## Non-Functional Requirements

### Performance

- NFR1: Compact diagnostic summary responses complete within 5 seconds for cached resources
- NFR2: Active probing operations (ephemeral pod deployment + check execution + cleanup) complete within 30 seconds
- NFR3: CRD discovery on startup completes within 10 seconds
- NFR4: Token efficiency: compact output mode produces responses under 500 tokens for standard diagnostic summaries

### Security

- NFR5: The MCP server operates with the minimum ClusterRole permissions required — read-only for networking resources, create/delete only for ephemeral diagnostic pods in a dedicated namespace
- NFR6: Ephemeral diagnostic pods run with restricted security context (no privileged access, no host networking)
- NFR7: The MCP server authenticates agent connections via Kubernetes ServiceAccount bearer tokens validated against the TokenReview API, rejecting unauthenticated or unauthorized connections before processing any tool invocation
- NFR8: The MCP server does not modify any existing cluster resources — read and report only (except ephemeral pod lifecycle)
- NFR9: Structured logs must not contain sensitive data (secrets, tokens, certificate private keys)

### Scalability

- NFR10: The MCP server supports 2 concurrent agent sessions for standard deployments
- NFR11: The MCP server supports up to 10 concurrent agent sessions for enterprise deployments
- NFR12: The MCP server supports horizontal scaling to at least 5 replicas behind a Kubernetes Service, with each replica operating independently (no shared state required between replicas)
- NFR13: Concurrent diagnostic pod limit defaults to 5 and is configurable between 1 and 20 to balance cluster resource usage with diagnostic throughput

### Reliability

- NFR14: The MCP server supports a minimum of 2 replicas for high availability, where failure of any single replica does not interrupt service for agents connected to other replicas
- NFR15: Individual replica failure does not disrupt other active agent sessions on different replicas
- NFR16: The MCP server recovers from K8s API server transient failures with exponential backoff retry (initial interval 1s, max interval 30s, max 5 retries) and returns a structured error to the agent if recovery fails
- NFR17: Orphaned ephemeral diagnostic pods are cleaned up automatically with a default TTL of 5 minutes (configurable between 1 and 30 minutes), even if the MCP server restarts
- NFR18: SSE connection loss triggers agent-side reconnection within 10 seconds, with the server retaining diagnostic session context for up to 5 minutes to allow seamless reconnection

### Integration

- NFR19: Full compliance with the MCP specification for tool registration, invocation, and response format
- NFR20: Compatible with Kubernetes 1.28+ API server
- NFR21: Supports both stable (v1) and beta API versions of Gateway API CRDs
- NFR22: Helm chart follows Helm best practices and passes `helm lint` validation
- NFR23: Container image published to a public registry (ghcr.io or Docker Hub) with semantic versioning tags
