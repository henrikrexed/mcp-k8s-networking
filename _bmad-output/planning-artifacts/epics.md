---
stepsCompleted: ['step-01-validate-prerequisites', 'step-02-design-epics', 'step-03-create-stories', 'step-04-final-validation']
inputDocuments: ['prd.md', 'architecture.md']
---

# mcp-k8s-networking - Epic Breakdown

## Overview

This document provides the complete epic and story breakdown for mcp-k8s-networking, decomposing the requirements from the PRD and Architecture into implementable stories. The project has partial implementation — status is tracked per story.

## Requirements Inventory

### Functional Requirements

- FR1: The MCP server can discover which networking CRDs are installed in the cluster on startup
- FR2: The MCP server can re-discover installed networking CRDs at a configurable interval (default: 5 minutes) and immediately upon a tool invocation that fails due to a missing CRD, reflecting changes in the tool registry within one re-discovery cycle
- FR3: The MCP server can report the complete list of detected networking providers and their versions to the agent
- FR4: The MCP server can adapt its available tools dynamically based on which CRDs are present without requiring restart
- FR5: The MCP server can detect the absence of Gateway API CRDs and inform the agent that Gateway API is not installed with a suggestion to install it
- FR6: The agent can request validation of Istio VirtualService and DestinationRule configurations
- FR7: The agent can request analysis of Istio AuthorizationPolicy and PeerAuthentication policies to identify overly restrictive or misconfigured rules
- FR8: The agent can request verification of Istio sidecar injection status across deployments in specified namespaces
- FR9: The agent can request mTLS validation between services to confirm encryption is properly configured
- FR10: The agent can request Istio traffic routing analysis that evaluates VirtualService route rules, DestinationRule subset definitions, and traffic splitting weights
- FR11: The agent can request inspection of Gateway resources and their status conditions
- FR12: The agent can request validation of HTTPRoute configurations including backend references, filters, and matching rules
- FR13: The agent can request validation of GRPCRoute configurations including backend references and method matching
- FR14: The agent can request ReferenceGrant inspection to identify cross-namespace reference issues
- FR15: The agent can request a misconfiguration scan across Gateway API resources
- FR16: The agent can request conformance validation of Gateway API resources against the spec
- FR17: The agent can request validation of kgateway provider-specific resources including GatewayParameters, RouteOption, and VirtualHostOption
- FR18: The agent can request a health summary of the kgateway installation
- FR19: The agent can request Service and Endpoint validation including selector matching and port verification
- FR20: The agent can request CoreDNS configuration inspection and DNS resolution diagnostics
- FR21: The agent can request kube-proxy health checks and configuration validation
- FR22: The agent can request NetworkPolicy analysis across namespaces to identify blocking or permissive rules
- FR23: The agent can request Ingress resource inspection and validation
- FR24: The agent can request basic Kuma mesh status and policy reporting
- FR25: The agent can request basic Linkerd configuration and health reporting
- FR26: The agent can request Cilium NetworkPolicy inspection and basic connectivity status
- FR27: The agent can request Calico NetworkPolicy inspection and basic status
- FR28: The agent can request Flannel installation detection and basic health reporting
- FR29: The agent can request deployment of an ephemeral pod to test network connectivity between namespaces or services
- FR30: The agent can request deployment of an ephemeral pod to perform DNS resolution checks
- FR31: The agent can request deployment of an ephemeral pod to perform curl/HTTP checks against services
- FR32: The MCP server can enforce resource limits on ephemeral diagnostic pods
- FR33: The MCP server can automatically clean up ephemeral diagnostic pods after a configurable TTL
- FR34: The MCP server can limit concurrent diagnostic pods to prevent cluster resource pressure
- FR35: Given a user intent, the agent can request Gateway API design guidance with annotated YAML templates
- FR36: Given a user intent, the agent can request Istio configuration design guidance with annotated YAML templates
- FR37: Given a user intent, the agent can request kgateway configuration design guidance with annotated YAML templates
- FR38: The MCP server can provide contextual guidance that references only providers and API versions actually available in the cluster
- FR39: The MCP server can accept connections from AI agents via SSE/Streamable HTTP
- FR40: The MCP server can process JSON-RPC 2.0 requests per the MCP specification
- FR41: The MCP server can return compact diagnostic summaries by default for token-efficient agent consumption
- FR42: The agent can request detailed output for specific diagnostic findings when deeper analysis is needed
- FR43: The MCP server can register and expose available tools to agents based on discovered cluster capabilities
- FR44: The MCP server can maintain SSE connections with heartbeats and handle reconnection gracefully
- FR45: A cluster administrator can deploy the MCP server via Helm chart with configurable RBAC scope and resource limits
- FR46: A cluster administrator can deploy the MCP server via raw YAML manifests as an alternative to Helm
- FR47: The MCP server can expose health and readiness endpoints for Kubernetes probes
- FR48: The MCP server can produce structured JSON logs for cluster log aggregation
- FR49: The MCP server can perform graceful shutdown with proper SSE connection draining
- FR50: The agent can request suggested remediations for identified diagnostic issues

### NonFunctional Requirements

- NFR1: Compact diagnostic summary responses complete within 5 seconds for cached resources
- NFR2: Active probing operations complete within 30 seconds
- NFR3: CRD discovery on startup completes within 10 seconds
- NFR4: Token efficiency: compact output mode produces responses under 500 tokens
- NFR5: Minimum ClusterRole permissions — read-only for networking resources, create/delete for ephemeral pods
- NFR6: Ephemeral diagnostic pods run with restricted security context
- NFR7: Agent connections authenticated via K8s ServiceAccount bearer tokens (TokenReview API)
- NFR8: No modification of existing cluster resources (except ephemeral pod lifecycle)
- NFR9: Structured logs must not contain sensitive data
- NFR10: Support 2 concurrent agent sessions (standard)
- NFR11: Support up to 10 concurrent agent sessions (enterprise)
- NFR12: Horizontal scaling to at least 5 replicas
- NFR13: Concurrent diagnostic pod limit defaults to 5, configurable 1-20
- NFR14: Minimum 2 replicas for HA
- NFR15: Individual replica failure does not disrupt other sessions
- NFR16: K8s API server transient failure recovery with exponential backoff
- NFR17: Orphaned ephemeral pods cleaned up with configurable TTL (default 5 min)
- NFR18: SSE reconnection within 10 seconds with 5-minute session context retention
- NFR19: Full MCP specification compliance
- NFR20: Compatible with Kubernetes 1.28+
- NFR21: Supports stable and beta Gateway API CRD versions
- NFR22: Helm chart passes helm lint validation
- NFR23: Container image published to ghcr.io with semantic versioning

### Additional Requirements

- Architecture specifies official go-sdk (modelcontextprotocol/go-sdk) — current code uses mark3labs/mcp-go (needs migration)
- Architecture specifies CRD watch-based discovery — current code uses 60s polling (needs rework)
- Architecture specifies slog structured logging — current code uses fmt/log (needs migration)
- Architecture specifies DiagnosticFinding struct and MCPError codes — current code uses generic map[string]interface{} responses
- Architecture specifies Provider/DeepProvider tiered interfaces — current code uses flat tool structs
- Architecture specifies concern-based dispatcher — current code has direct tool-per-resource mapping
- New scope: Helm chart with Gateway API provider as configurable variable
- New scope: Expose MCP server via Gateway API HTTPRoute
- New scope: mkdocs documentation website with tool descriptions, setup guides, usage examples
- 20 code issues identified in codebase review requiring fixes
- New scope: Agent configuration skills/playbooks — codified multi-step workflows agents follow to configure networking routes
- New scope: OpenTelemetry instrumentation — MCP server produces spans for all tool invocations, K8s API calls, and probe lifecycles

### FR Coverage Map

| FR | Epic | Brief Description |
|---|---|---|
| FR1 | Epic 1 | CRD discovery on startup |
| FR2 | Epic 1 | CRD re-discovery (watch-based) |
| FR3 | Epic 1 | Report detected providers |
| FR4 | Epic 1 | Dynamic tool registry |
| FR5 | Epic 1 | Detect absent Gateway API CRDs |
| FR6 | Epic 4 | Istio VirtualService/DestinationRule validation |
| FR7 | Epic 4 | Istio AuthorizationPolicy/PeerAuthentication analysis |
| FR8 | Epic 4 | Istio sidecar injection verification |
| FR9 | Epic 4 | Istio mTLS validation |
| FR10 | Epic 4 | Istio traffic routing analysis |
| FR11 | Epic 3 | Gateway resource inspection |
| FR12 | Epic 3 | HTTPRoute validation |
| FR13 | Epic 3 | GRPCRoute validation |
| FR14 | Epic 3 | ReferenceGrant inspection |
| FR15 | Epic 3 | Gateway API misconfiguration scan |
| FR16 | Epic 3 | Gateway API conformance validation |
| FR17 | Epic 5 | kgateway resource validation |
| FR18 | Epic 5 | kgateway health summary |
| FR19 | Epic 1 | Service/Endpoint validation |
| FR20 | Epic 1 | CoreDNS diagnostics |
| FR21 | Epic 1 | kube-proxy health |
| FR22 | Epic 1 | NetworkPolicy analysis |
| FR23 | Epic 1 | Ingress inspection |
| FR24 | Epic 8 | Kuma basic status |
| FR25 | Epic 8 | Linkerd basic status |
| FR26 | Epic 8 | Cilium NetworkPolicy + status |
| FR27 | Epic 8 | Calico NetworkPolicy + status |
| FR28 | Epic 8 | Flannel detection + health |
| FR29 | Epic 6 | Connectivity probe (ephemeral pod) |
| FR30 | Epic 6 | DNS resolution probe |
| FR31 | Epic 6 | HTTP/curl probe |
| FR32 | Epic 6 | Probe resource limits |
| FR33 | Epic 6 | Probe TTL auto-cleanup |
| FR34 | Epic 6 | Concurrent probe limiting |
| FR35 | Epic 7 | Gateway API design guidance |
| FR36 | Epic 7 | Istio design guidance |
| FR37 | Epic 7 | kgateway design guidance |
| FR38 | Epic 7 | Provider-aware contextual guidance |
| FR39 | Epic 1 | SSE/Streamable HTTP connection |
| FR40 | Epic 1 | JSON-RPC 2.0 processing |
| FR41 | Epic 1 | Compact output mode |
| FR42 | Epic 1 | Detail output mode |
| FR43 | Epic 1 | Dynamic tool registration |
| FR44 | Epic 1 | SSE heartbeats + reconnection |
| FR45 | Epic 9 | Helm chart deployment |
| FR46 | Epic 9 | Raw YAML deployment |
| FR47 | Epic 1 | Health/readiness probes |
| FR48 | Epic 1 | Structured JSON logging |
| FR49 | Epic 1 | Graceful shutdown |
| FR50 | Epic 7 | Diagnostic remediation suggestions |

## Epic List

### Epic 1: MCP Server Foundation & Core K8s Diagnostics
Agents can connect to the MCP server and diagnose basic Kubernetes networking — services, endpoints, DNS resolution, NetworkPolicies, and Ingress resources. This is the minimum viable deployment that delivers immediate diagnostic value.
**FRs covered:** FR1, FR2, FR3, FR4, FR5, FR19, FR20, FR21, FR22, FR23, FR39, FR40, FR41, FR42, FR43, FR44, FR47, FR48, FR49

### Epic 2: Networking Log Collection & Error Analysis
Agents can retrieve and analyze logs from proxy sidecars, gateway controllers, and infrastructure components (kube-proxy, CoreDNS, CNI) to diagnose misconfigurations, rate limiting, and connection issues.
**FRs covered:** Extends FR19-21 diagnostic capability with log-based evidence

### Epic 3: Gateway API Diagnostics
Agents can inspect, validate, and diagnose Gateway API resources — Gateways, HTTPRoutes, GRPCRoutes, ReferenceGrants — and detect misconfigurations including orphaned routes, missing references, and listener conflicts.
**FRs covered:** FR11, FR12, FR13, FR14, FR15, FR16

### Epic 4: Istio Diagnostics
Agents can diagnose Istio mesh configuration — VirtualService, DestinationRule, AuthorizationPolicy, PeerAuthentication — including sidecar injection status, mTLS validation, and traffic routing analysis.
**FRs covered:** FR6, FR7, FR8, FR9, FR10

### Epic 5: kgateway Diagnostics
Agents can diagnose kgateway-specific resources (GatewayParameters, RouteOption, VirtualHostOption) and get a health summary of the kgateway installation.
**FRs covered:** FR17, FR18

### Epic 6: Active Diagnostic Probing
Agents can deploy ephemeral pods to actively test network connectivity, DNS resolution, and HTTP reachability between services — with resource limits, TTL cleanup, and concurrency controls.
**FRs covered:** FR29, FR30, FR31, FR32, FR33, FR34

### Epic 7: CRD-Aware Design Guidance & Remediation
Agents can request design guidance to generate provider-specific networking configurations (Gateway API, Istio, kgateway) and receive suggested remediations for identified diagnostic issues.
**FRs covered:** FR35, FR36, FR37, FR38, FR50

### Epic 8: Tier 2 Provider Support
Agents get basic visibility and diagnostics for additional mesh and CNI providers — Kuma, Linkerd, Cilium, Calico, and Flannel — with detection, status reporting, and policy inspection.
**FRs covered:** FR24, FR25, FR26, FR27, FR28

### Epic 9: Production Deployment — Helm Chart & Gateway API Exposure
Administrators can deploy the MCP server via a production-ready Helm chart with configurable RBAC, resource limits, and Gateway API provider selection. The MCP server is exposed through a Gateway API HTTPRoute.
**FRs covered:** FR45, FR46 + new scope (Helm chart with Gateway API provider variable, HTTPRoute exposure)

### Epic 10: Documentation Website
Users can access comprehensive mkdocs documentation including tool descriptions, setup guides, architecture overview, and usage examples for each diagnostic tool.
**FRs covered:** New scope (documentation website)

### Epic 11: Agent Skills for Networking Configuration
Agents can follow codified multi-step playbooks (skills) to guide users through configuring networking routes — exposing services via Gateway API, setting up mTLS, configuring traffic splitting, creating NetworkPolicies, and more. Skills are structured workflows that combine diagnostic checks with design guidance into interactive step-by-step flows.
**FRs covered:** New scope (extends FR35-37 design guidance into interactive agent workflows)
**Status:** Not started

### Epic 12: OpenTelemetry Instrumentation (GenAI + MCP Semantic Conventions)
The MCP server produces all three OTel signals (traces, metrics, logs) following GenAI and MCP semantic conventions. Every tool invocation produces spans with standardized attributes (gen_ai.tool.name, mcp.method.name, etc.), GenAI metrics track request performance, custom metrics track findings and errors, and structured logs are correlated with traces via the OTel log bridge.
**FRs covered:** FR-OTel-1 through FR-OTel-8
**Status:** In progress

---

## Epic 1: MCP Server Foundation & Core K8s Diagnostics

Agents can connect to the MCP server and diagnose basic Kubernetes networking — services, endpoints, DNS resolution, NetworkPolicies, and Ingress resources. This is the minimum viable deployment that delivers immediate diagnostic value.

### Story 1.1: Migrate MCP Server to Official go-sdk with Streamable HTTP

As an AI agent,
I want to connect to the MCP server via the official MCP protocol implementation,
So that I get reliable, spec-compliant communication with proper session management.

**Acceptance Criteria:**

**Given** the MCP server is running
**When** an agent initiates a connection via Streamable HTTP
**Then** the server accepts the connection and completes the MCP initialize/initialized handshake
**And** the server responds to `tools/list` with all registered tools
**And** the server responds to `tools/call` with structured JSON results

**Given** the server has tools registered
**When** tools are added or removed from the registry
**Then** the MCP server reflects the updated tool list via `tools/list` without restart

**Given** an active agent session
**When** the connection is idle
**Then** the server sends heartbeats to maintain the connection (FR44)

**Implementation Notes:**
- Replace mark3labs/mcp-go dependency with modelcontextprotocol/go-sdk
- Use `mcp.NewServer()` and Streamable HTTP transport
- Fix current bug: SyncTools() adds tools but never removes old ones
- Fix current bug: hardcoded `http://localhost` base URL
- Wire tool registry to go-sdk tool registration API

### Story 1.2: Implement CRD Watch-Based Discovery

As an AI agent,
I want the MCP server to detect installed networking CRDs in real-time,
So that diagnostic tools appear immediately when providers are installed and disappear when removed.

**Acceptance Criteria:**

**Given** the MCP server starts up
**When** initial CRD discovery runs
**Then** it detects all installed networking CRDs (Gateway API, Istio, Cilium, Calico, Linkerd, Kuma, Flannel) within 10 seconds (NFR3)
**And** it registers corresponding tools for detected CRDs (FR1, FR4)

**Given** the MCP server is running
**When** a new networking CRD is installed (e.g., `gateway.networking.k8s.io`)
**Then** the watch detects the change and registers the corresponding tools within one watch cycle (FR2)

**Given** the MCP server is running
**When** a networking CRD is removed from the cluster
**Then** the watch detects the removal and deregisters the corresponding tools (FR4)

**Given** Gateway API CRDs are not installed
**When** an agent calls `tools/list`
**Then** no Gateway API tools appear in the list
**And** the server can report that Gateway API is not installed if queried (FR5)

**Given** the MCP server is running
**When** the API server is temporarily unavailable
**Then** the watch reconnects with exponential backoff (NFR16)

**Implementation Notes:**
- Replace 60s polling with CRD watch on `apiextensions.k8s.io/v1`
- On startup: full discovery scan. After startup: watch for create/delete events
- Expose Features struct with HasGatewayAPI, HasIstio, HasCilium, etc.
- onChange callback triggers tool registration/deregistration

### Story 1.3: Implement Structured Logging and Configuration

As a platform engineer,
I want the MCP server to produce structured JSON logs and be configurable via environment variables,
So that I can integrate it with my cluster's log aggregation pipeline and tune its behavior.

**Acceptance Criteria:**

**Given** the MCP server is running
**When** any log message is emitted
**Then** it is formatted as structured JSON using Go's slog package (FR48)
**And** every log line includes `tool_name` and `session_id` fields when applicable
**And** no log line contains secrets, tokens, or certificate private keys (NFR9)

**Given** the following environment variables are set: CLUSTER_NAME, PORT, LOG_LEVEL, NAMESPACE, CACHE_TTL, TOOL_TIMEOUT, PROBE_NAMESPACE, PROBE_IMAGE, MAX_CONCURRENT_PROBES
**When** the server starts
**Then** it loads all configuration from environment variables with documented defaults

**Given** LOG_LEVEL is set to "debug"
**When** the server operates
**Then** debug-level log messages are visible in output

**Given** a required environment variable (CLUSTER_NAME) is not set
**When** the server starts
**Then** it exits with a clear error message indicating the missing configuration

**Implementation Notes:**
- Migrate from fmt.Printf/log to log/slog with JSON handler
- Add missing config vars: CACHE_TTL (30s), TOOL_TIMEOUT (10s), PROBE_NAMESPACE (mcp-diagnostics), PROBE_IMAGE, MAX_CONCURRENT_PROBES (5)
- Add config validation

### Story 1.4: Implement Standard Diagnostic Response Types

As an AI agent,
I want diagnostic results returned in a consistent structured format with compact/detail modes,
So that I can efficiently parse findings and minimize token consumption.

**Acceptance Criteria:**

**Given** any diagnostic tool is invoked
**When** it returns results
**Then** the response uses the `DiagnosticFinding` struct with fields: severity, category, resource, summary, detail, suggestion

**Given** a tool is invoked without `"detail": true`
**When** results are returned
**Then** only the `summary` field is populated (compact mode) and `detail`/`suggestion` are omitted (FR41, NFR4)

**Given** a tool is invoked with `"detail": true`
**When** results are returned
**Then** all fields including `detail` and `suggestion` are populated (FR42)

**Given** a tool encounters an expected error (provider not installed, CRD unavailable)
**When** the error is returned to the agent
**Then** it uses the `MCPError` struct with code, message, tool, and detail fields

**Given** any tool response
**When** returned to the agent
**Then** it includes `ClusterMetadata` with clusterName, timestamp, namespace, and provider

**Implementation Notes:**
- Define DiagnosticFinding, MCPError, ClusterMetadata, ResourceRef structs in pkg/dispatcher/types.go
- Define severity constants: critical, warning, info, ok
- Define error codes: PROVIDER_NOT_FOUND, CRD_NOT_AVAILABLE, INVALID_INPUT, INTERNAL_ERROR
- Refactor existing tools to use these types instead of map[string]interface{}

### Story 1.5: Service and Endpoint Diagnostics

As an AI agent,
I want to inspect Kubernetes Services and their Endpoints,
So that I can validate service health, selector matching, and port configuration.

**Acceptance Criteria:**

**Given** services exist in a namespace
**When** the agent calls `list_services` with a namespace parameter
**Then** it returns all services with type, clusterIP, ports, and selector information

**Given** a specific service exists
**When** the agent calls `get_service` with name and namespace
**Then** it returns the service detail, its endpoints (ready and not-ready counts), and matching pod status (FR19)

**Given** a service has a selector that matches no pods
**When** the agent calls `get_service`
**Then** it returns a warning-level DiagnosticFinding indicating no endpoints match the selector

**Given** `list_services` is called without a namespace
**When** the tool executes
**Then** it returns services across all namespaces (or the configured NAMESPACE if set)

**Implementation Notes:**
- Fix existing code: JSON number type assertions use int64 but should use float64
- Fix: empty namespace should list across all namespaces, not just "default"
- Refactor response format to use DiagnosticFinding

### Story 1.6: NetworkPolicy Analysis

As an AI agent,
I want to analyze NetworkPolicies across namespaces,
So that I can identify blocking or overly permissive rules affecting service connectivity.

**Acceptance Criteria:**

**Given** NetworkPolicies exist in a namespace
**When** the agent calls `list_networkpolicies` with a namespace
**Then** it returns all policies with podSelector, ingress rule count, and egress rule count (FR22)

**Given** a specific NetworkPolicy exists
**When** the agent calls `get_networkpolicy` with name and namespace
**Then** it returns the full policy with ingress and egress rule details including port, protocol, and peer selectors

**Given** a NetworkPolicy blocks all ingress for a pod selector
**When** the policy is analyzed
**Then** a warning-level DiagnosticFinding is returned indicating the restrictive rule

**Implementation Notes:**
- Refactor existing tools to use DiagnosticFinding response format
- Fix namespace handling for cluster-wide queries

### Story 1.7: DNS Resolution and kube-proxy Health Diagnostics

As an AI agent,
I want to check DNS resolution health and kube-proxy status,
So that I can diagnose fundamental networking infrastructure issues.

**Acceptance Criteria:**

**Given** a hostname and namespace
**When** the agent calls `check_dns_resolution`
**Then** it performs a DNS lookup and reports resolved addresses
**And** it checks the kube-dns service health in kube-system (FR20)

**Given** DNS resolution fails for a hostname
**When** the check completes
**Then** a critical-level DiagnosticFinding is returned with the failure details

**Given** the kube-proxy DaemonSet is running
**When** the agent calls `check_kube_proxy_health`
**Then** it reports kube-proxy pod status across nodes, configuration mode (iptables/IPVS), and any unhealthy pods (FR21)

**Implementation Notes:**
- Fix existing DNS tool to use DiagnosticFinding format
- Add kube-proxy health check (new tool) reading DaemonSet status and ConfigMap

### Story 1.8: Ingress Resource Inspection

As an AI agent,
I want to inspect Ingress resources,
So that I can validate Ingress configurations and identify routing issues.

**Acceptance Criteria:**

**Given** Ingress resources exist in a namespace
**When** the agent calls `list_ingresses` with a namespace
**Then** it returns all Ingress resources with hosts, paths, backends, and TLS configuration (FR23)

**Given** a specific Ingress resource
**When** the agent calls `get_ingress` with name and namespace
**Then** it returns the full Ingress spec with rules, TLS settings, and status (load balancer IP/hostname)

**Given** an Ingress references a backend service that does not exist
**When** the Ingress is inspected
**Then** a warning-level DiagnosticFinding is returned indicating the missing backend

**Implementation Notes:**
- New tools: list_ingresses, get_ingress
- Use dynamic client with networking.k8s.io/v1 Ingress GVR

### Story 1.9: Health Probes and Graceful Shutdown

As a platform engineer,
I want the MCP server to expose accurate health/readiness probes and shut down gracefully,
So that Kubernetes can manage the server lifecycle correctly.

**Acceptance Criteria:**

**Given** the MCP server has started and completed initial CRD discovery
**When** Kubernetes probes `/healthz`
**Then** it returns HTTP 200 (FR47)

**Given** the MCP server has started and completed initial CRD discovery and the K8s client is connected
**When** Kubernetes probes `/readyz`
**Then** it returns HTTP 200

**Given** the MCP server has NOT completed initial CRD discovery
**When** Kubernetes probes `/readyz`
**Then** it returns HTTP 503

**Given** the server receives a SIGTERM signal
**When** graceful shutdown begins
**Then** it stops accepting new connections, drains active sessions, cancels in-flight tool executions via context, and exits within a configurable timeout (FR49)

**Implementation Notes:**
- Split health check into /healthz (liveness) and /readyz (readiness with state checks)
- Add shutdown timeout (default 30s) to graceful shutdown context
- Fix current code: no timeout on shutdown context

---

## Epic 2: Networking Log Collection & Error Analysis

Agents can retrieve and analyze logs from proxy sidecars, gateway controllers, and infrastructure components to diagnose misconfigurations, rate limiting, and connection issues.

### Story 2.1: Proxy Sidecar Log Collection

As an AI agent,
I want to retrieve logs from Envoy/proxy sidecar containers,
So that I can diagnose proxy-level networking issues like connection failures and TLS errors.

**Acceptance Criteria:**

**Given** a pod has a proxy sidecar container (istio-proxy, envoy, or linkerd-proxy)
**When** the agent calls `get_proxy_logs` with pod and namespace
**Then** it auto-detects the proxy container and returns its logs

**Given** the agent specifies `tail` and `since` parameters
**When** `get_proxy_logs` executes
**Then** it returns only the specified number of recent lines within the time window

**Given** the log output exceeds 100KB
**When** logs are returned
**Then** the output is truncated with a message indicating truncation and total available lines

**Given** a pod has no proxy container
**When** `get_proxy_logs` is called
**Then** it returns an MCPError with code INVALID_INPUT

**Implementation Notes:**
- Fix existing code to add output size limits (max 100KB)
- Refactor to use DiagnosticFinding response format

### Story 2.2: Gateway Controller and Infrastructure Log Collection

As an AI agent,
I want to retrieve logs from gateway controllers and infrastructure components,
So that I can diagnose control plane and infrastructure-level networking issues.

**Acceptance Criteria:**

**Given** gateway controller pods are running (Istio, Envoy Gateway, etc.)
**When** the agent calls `get_gateway_logs`
**Then** it discovers controller pods by known labels and returns their logs

**Given** the agent specifies `component` as "kube-proxy", "coredns", or "cni"
**When** the agent calls `get_infra_logs`
**Then** it discovers the correct pods by label selectors and returns their logs with node information

**Given** a CNI is running (Cilium, Calico, or Flannel)
**When** `get_infra_logs` is called with component "cni"
**Then** it auto-discovers the CNI type and returns logs from the correct pods

**Given** log output exceeds 100KB per pod
**When** logs are returned
**Then** output is truncated per pod with truncation notice

**Implementation Notes:**
- Add per-pod output size limits
- Existing implementation covers most functionality — add size guards

### Story 2.3: Log Error Analysis and Categorization

As an AI agent,
I want to analyze pod logs and extract categorized error patterns,
So that I can quickly identify misconfigurations, rate limiting, and connection issues.

**Acceptance Criteria:**

**Given** a pod and namespace
**When** the agent calls `analyze_log_errors`
**Then** it reads the specified number of log lines (default 500) and extracts lines matching error patterns

**Given** error lines are found
**When** the analysis completes
**Then** errors are categorized into: connection_errors, tls_errors, rate_limiting, misconfig, rbac_denied, upstream_issues, timeout, other_errors

**Given** the analysis results
**When** returned to the agent
**Then** the response includes total lines scanned, error line count, category counts, and the actual error lines (capped at 50 lines)

**Given** no errors are found
**When** the analysis completes
**Then** an ok-level DiagnosticFinding is returned confirming no issues detected

**Implementation Notes:**
- Cap errorLines output to 50 lines to prevent oversized responses
- Refactor to use DiagnosticFinding format

---

## Epic 3: Gateway API Diagnostics

Agents can inspect, validate, and diagnose Gateway API resources — Gateways, HTTPRoutes, GRPCRoutes, ReferenceGrants — and detect misconfigurations.

### Story 3.1: Fix and Enhance Gateway and HTTPRoute Diagnostics

As an AI agent,
I want accurate Gateway and HTTPRoute inspection,
So that I can reliably diagnose Gateway API routing configurations.

**Acceptance Criteria:**

**Given** Gateway resources exist in a namespace
**When** the agent calls `list_gateways`
**Then** it returns all Gateways with listeners, status conditions, addresses, and attached route counts (FR11)

**Given** a specific Gateway
**When** the agent calls `get_gateway` with name and namespace
**Then** it returns full Gateway details including each listener's name, port, protocol, hostname, and attached routes list

**Given** HTTPRoute resources exist
**When** the agent calls `list_httproutes` or `get_httproute`
**Then** it returns correct parent refs, backend refs, rules, matches, and filters (FR12)

**Given** any Gateway API response
**When** numeric values are extracted from unstructured objects
**Then** they use float64 type assertions (fixing current int64 bug)

**Implementation Notes:**
- Fix attachedRoutes type assertion: JSON numbers from unstructured are float64, not int64
- Refactor to DiagnosticFinding response format
- Support both v1 and v1beta1 Gateway API versions (NFR21)

### Story 3.2: GRPCRoute Validation

As an AI agent,
I want to inspect GRPCRoute resources,
So that I can validate gRPC routing configurations and backend references.

**Acceptance Criteria:**

**Given** GRPCRoute resources exist
**When** the agent calls `list_grpcroutes` with an optional namespace
**Then** it returns all GRPCRoutes with parent refs, backend refs, and rule counts (FR13)

**Given** a specific GRPCRoute
**When** the agent calls `get_grpcroute` with name and namespace
**Then** it returns the full spec including method matching rules, backend refs, and status conditions

**Given** a GRPCRoute references a backend service that does not exist
**When** the route is inspected
**Then** a warning-level DiagnosticFinding is returned

**Implementation Notes:**
- New tools: list_grpcroutes, get_grpcroute
- GVR: gateway.networking.k8s.io/v1 grpcroutes

### Story 3.3: ReferenceGrant Inspection

As an AI agent,
I want to inspect ReferenceGrant resources,
So that I can identify cross-namespace reference issues blocking route resolution.

**Acceptance Criteria:**

**Given** ReferenceGrant resources exist
**When** the agent calls `list_referencegrants` with an optional namespace
**Then** it returns all ReferenceGrants with from/to resource specifications (FR14)

**Given** a specific ReferenceGrant
**When** the agent calls `get_referencegrant` with name and namespace
**Then** it returns the full spec including allowed from-namespaces, from-kinds, to-kinds, and to-names

**Given** an HTTPRoute references a backend in another namespace
**When** no matching ReferenceGrant exists
**Then** a warning-level DiagnosticFinding is returned indicating the missing grant

**Implementation Notes:**
- New tools: list_referencegrants, get_referencegrant
- GVR: gateway.networking.k8s.io/v1beta1 referencegrants

### Story 3.4: Gateway API Misconfiguration Scan

As an AI agent,
I want to run a comprehensive scan for Gateway API misconfigurations,
So that I can detect common issues across all Gateway API resources at once.

**Acceptance Criteria:**

**Given** Gateway API resources exist in a namespace (or cluster-wide)
**When** the agent calls `scan_gateway_misconfigs` with an optional namespace
**Then** it detects and reports (FR15):
- Routes referencing non-existent backend Services
- Routes attached to non-existent or non-matching Gateways
- Cross-namespace references missing required ReferenceGrants
- Gateways with unresolved listener conflicts (port/protocol collisions)
- Routes with invalid filter configurations

**Given** each detected misconfiguration
**When** included in the scan results
**Then** it includes resource name, namespace, issue description, and suggested fix as a DiagnosticFinding

**Given** no misconfigurations are detected
**When** the scan completes
**Then** an ok-level DiagnosticFinding confirms the Gateway API configuration is healthy

**Implementation Notes:**
- New tool: scan_gateway_misconfigs
- Cross-references Gateways, HTTPRoutes, GRPCRoutes, ReferenceGrants, and Services

### Story 3.5: Gateway API Conformance Validation

As an AI agent,
I want to validate Gateway API resources against the specification,
So that I can ensure configurations comply with the Gateway API standard.

**Acceptance Criteria:**

**Given** a Gateway API resource (Gateway, HTTPRoute, GRPCRoute)
**When** the agent calls `check_gateway_conformance` with the resource reference
**Then** it validates the resource against Gateway API spec requirements (FR16)
**And** reports any non-conformant fields as warning-level DiagnosticFindings

**Given** a resource uses features from an extended conformance profile
**When** validated
**Then** it reports the required conformance profile (core, extended) as informational

**Implementation Notes:**
- New tool: check_gateway_conformance
- Validates field constraints, required fields, and spec-defined enums

---

## Epic 4: Istio Diagnostics

Agents can diagnose Istio mesh configuration including sidecar injection, mTLS, policy analysis, and traffic routing.

### Story 4.1: Fix and Enhance Istio Resource Listing and Retrieval

As an AI agent,
I want accurate Istio resource listing and retrieval across API versions,
So that I can reliably inspect VirtualService, DestinationRule, AuthorizationPolicy, and PeerAuthentication resources.

**Acceptance Criteria:**

**Given** Istio resources exist in a namespace
**When** the agent calls `list_istio_resources` with a kind (virtualservices, destinationrules, authorizationpolicies, peerauthentications)
**Then** it returns all resources of that kind with key summary fields

**Given** a specific Istio resource
**When** the agent calls `get_istio_resource` with kind, name, and namespace
**Then** it returns the full spec and status

**Given** Istio is installed with v1 API versions (not just v1beta1)
**When** tools are registered
**Then** they dynamically detect and use the installed Istio API versions (not hardcoded v1beta1)

**Implementation Notes:**
- Fix hardcoded `v1beta1` API versions — auto-detect installed Istio API versions
- Refactor to DiagnosticFinding response format

### Story 4.2: Enhanced Sidecar Injection and mTLS Checks

As an AI agent,
I want to verify sidecar injection status and mTLS configuration,
So that I can confirm the mesh is properly configured for the target workloads.

**Acceptance Criteria:**

**Given** a namespace
**When** the agent calls `check_sidecar_injection`
**Then** it reports for each deployment: namespace label, deployment annotation, actual sidecar container presence, and injection status (injected, pending, missing) (FR8)

**Given** a deployment has the injection annotation but no sidecar container
**When** the check runs
**Then** a warning-level DiagnosticFinding is returned indicating injection is expected but missing

**Given** a namespace (or cluster-wide)
**When** the agent calls `check_istio_mtls`
**Then** it reports the effective mTLS mode per namespace based on PeerAuthentication policies and DestinationRule TLS settings (FR9)

**Given** a PeerAuthentication enforces STRICT mTLS but a DestinationRule disables TLS for a service
**When** the mTLS check runs
**Then** a critical-level DiagnosticFinding is returned indicating the conflict

**Implementation Notes:**
- Existing tools mostly work — refactor to DiagnosticFinding and add conflict detection

### Story 4.3: Istio Configuration Validation

As an AI agent,
I want to validate VirtualService and DestinationRule configurations,
So that I can detect misconfigurations before they cause routing failures.

**Acceptance Criteria:**

**Given** a VirtualService resource
**When** the agent calls `validate_istio_config` with kind=virtualservice
**Then** it checks for: routes referencing non-existent services, invalid match conditions, and conflicting route rules (FR6)

**Given** a DestinationRule resource
**When** validated
**Then** it checks for: subsets referencing labels that match no pods, TLS settings conflicts, and connection pool misconfigurations

**Given** validation findings exist
**When** returned to the agent
**Then** each finding is a DiagnosticFinding with severity, resource reference, and suggested fix

**Implementation Notes:**
- New tool: validate_istio_config
- Cross-references VirtualServices with Services/Endpoints for existence validation

### Story 4.4: Istio AuthorizationPolicy Analysis

As an AI agent,
I want to analyze AuthorizationPolicy resources,
So that I can identify overly restrictive or misconfigured access control rules.

**Acceptance Criteria:**

**Given** AuthorizationPolicy resources in a namespace
**When** the agent calls `analyze_istio_authpolicy` with namespace
**Then** it reports all policies with their action (ALLOW, DENY, CUSTOM), matched workloads, and rule summaries (FR7)

**Given** a DENY policy matches all traffic to a service
**When** analyzed
**Then** a warning-level DiagnosticFinding indicates the broad deny rule

**Given** multiple policies exist that could conflict
**When** analyzed
**Then** the tool identifies potential conflicts (e.g., ALLOW and DENY policies for the same workload)

**Implementation Notes:**
- New tool: analyze_istio_authpolicy
- Evaluates policy precedence and conflict detection

### Story 4.5: Istio Traffic Routing Analysis

As an AI agent,
I want to analyze Istio traffic routing end-to-end,
So that I can detect broken routes, weight misconfiguration, and shadowed rules.

**Acceptance Criteria:**

**Given** VirtualService and DestinationRule resources for a service
**When** the agent calls `analyze_istio_routing` with service name and namespace
**Then** it evaluates (FR10):
- Routes referencing non-existent subsets or services
- Weight allocations that do not sum to 100%
- Unreachable rules shadowed by higher-priority matches
- Conflicts between routing rules and AuthorizationPolicy deny rules

**Given** route weights for a VirtualService do not sum to 100%
**When** the analysis runs
**Then** a critical-level DiagnosticFinding is returned with the actual weight sum

**Given** a route rule is unreachable due to a more permissive rule above it
**When** the analysis runs
**Then** a warning-level DiagnosticFinding identifies the shadowed rule

**Implementation Notes:**
- New tool: analyze_istio_routing
- Cross-references VirtualService routes → DestinationRule subsets → Service endpoints

---

## Epic 5: kgateway Diagnostics

Agents can diagnose kgateway-specific resources and installation health.

### Story 5.1: kgateway Resource Validation

As an AI agent,
I want to validate kgateway-specific resources,
So that I can detect misconfigurations in GatewayParameters, RouteOption, and VirtualHostOption.

**Acceptance Criteria:**

**Given** kgateway CRDs are installed in the cluster
**When** the agent calls `list_kgateway_resources` with a kind
**Then** it returns all resources of that kind with key fields (FR17)

**Given** a kgateway resource (RouteOption, VirtualHostOption, or GatewayParameters)
**When** the agent calls `validate_kgateway_resource` with kind, name, and namespace
**Then** it detects:
- Invalid or unresolvable upstream references
- Route option conflicts between overlapping VirtualHostOption resources
- GatewayParameters referencing non-existent Kubernetes resources

**Given** kgateway CRDs are not installed
**When** any kgateway tool is called
**Then** the tool is not registered (CRD watch ensures this)

**Implementation Notes:**
- New tools: list_kgateway_resources, validate_kgateway_resource
- Requires CRD discovery to detect kgateway.dev CRDs

### Story 5.2: kgateway Installation Health Summary

As an AI agent,
I want to check the health of the kgateway installation,
So that I can verify the control plane and data plane are functioning correctly.

**Acceptance Criteria:**

**Given** kgateway is installed
**When** the agent calls `check_kgateway_health`
**Then** it reports (FR18):
- kgateway control plane pod status and readiness
- Translation status of kgateway resources (accepted, rejected, errored from status conditions)
- Data plane proxy health for Gateways managed by kgateway

**Given** the kgateway control plane has unhealthy pods
**When** the health check runs
**Then** a critical-level DiagnosticFinding is returned with pod details

**Implementation Notes:**
- New tool: check_kgateway_health
- Discover kgateway pods by labels, check status conditions on kgateway resources

---

## Epic 6: Active Diagnostic Probing

Agents can deploy ephemeral pods to actively test network connectivity, DNS resolution, and HTTP reachability.

### Story 6.1: Ephemeral Pod Manager

As a platform engineer,
I want the MCP server to manage ephemeral diagnostic pods safely,
So that active probes don't impact cluster stability.

**Acceptance Criteria:**

**Given** a probe request is made
**When** the probe manager creates a pod
**Then** the pod runs with restricted security context: runAsNonRoot, drop all capabilities, seccompProfile RuntimeDefault (NFR6)
**And** the pod has resource limits enforced from configuration (FR32)
**And** the pod is created in the configured PROBE_NAMESPACE

**Given** a probe completes (success or failure)
**When** the pod lifecycle ends
**Then** the pod is always deleted via defer, even on errors

**Given** the MCP server starts
**When** it initializes the probe manager
**Then** it cleans up any orphaned diagnostic pods (pods with TTL labels that have expired) (NFR17)

**Given** the probe manager receives a request when MAX_CONCURRENT_PROBES pods are already running
**When** the limit is reached
**Then** it returns an MCPError with code PROBE_LIMIT_REACHED (FR34)

**Implementation Notes:**
- New package: pkg/probes/ with manager.go, pod.go, cleanup.go, types.go
- Custom Alpine-based probe image with curl, dig, ncat
- Periodic orphan cleanup goroutine

### Story 6.2: Network Connectivity and DNS Probes

As an AI agent,
I want to test network connectivity and DNS resolution between services,
So that I can actively verify or rule out network-level issues.

**Acceptance Criteria:**

**Given** source and destination parameters (namespaces, service names, ports)
**When** the agent calls `probe_connectivity`
**Then** it deploys an ephemeral pod in the source namespace and tests TCP connectivity to the destination (FR29)
**And** completes within 30 seconds (NFR2)

**Given** a hostname and optional namespace
**When** the agent calls `probe_dns`
**Then** it deploys an ephemeral pod and runs dig/nslookup to test DNS resolution (FR30)
**And** reports resolved addresses, response time, and any DNS errors

**Given** the destination is unreachable
**When** the connectivity probe completes
**Then** a critical-level DiagnosticFinding is returned with the failure details (timeout, connection refused, etc.)

**Implementation Notes:**
- New tools: probe_connectivity, probe_dns
- Use probe manager from Story 6.1

### Story 6.3: HTTP Service Probes

As an AI agent,
I want to perform HTTP/HTTPS requests against services from within the cluster,
So that I can test application-layer reachability and response behavior.

**Acceptance Criteria:**

**Given** a target URL or service:port and optional HTTP method/headers
**When** the agent calls `probe_http`
**Then** it deploys an ephemeral pod and runs curl against the target (FR31)
**And** returns HTTP status code, response headers, response time, and body snippet (first 1KB)

**Given** the HTTP request fails (timeout, TLS error, connection refused)
**When** the probe completes
**Then** the specific failure reason is reported as a DiagnosticFinding

**Given** a TLS connection is made
**When** the probe completes
**Then** it reports certificate details (issuer, expiry, SANs) if available

**Implementation Notes:**
- New tool: probe_http
- Use probe manager from Story 6.1
- Curl command with timeout, follow redirects, verbose TLS output

---

## Epic 7: CRD-Aware Design Guidance & Remediation

Agents can request design guidance to generate provider-specific configurations and receive suggested remediations.

### Story 7.1: Gateway API Design Guidance

As an AI agent,
I want to generate Gateway API configurations based on user intent,
So that I can help users create correct networking manifests without trial and error.

**Acceptance Criteria:**

**Given** a user intent (e.g., "expose service X on port Y via HTTPS")
**When** the agent calls `design_gateway_api` with the intent parameters
**Then** it returns (FR35):
- Ordered list of required resources (Gateway, HTTPRoute/GRPCRoute, ReferenceGrant as needed)
- Annotated YAML templates populated with user's service details
- Provider-specific annotations required by the installed Gateway API implementation
- Warnings for missing prerequisites (missing TLS secret, missing ReferenceGrant)

**Given** the cluster has a specific Gateway API implementation (Istio, Envoy Gateway, kgateway)
**When** guidance is generated
**Then** templates reference only API versions and CRDs confirmed present in the cluster (FR38)

**Implementation Notes:**
- New tool: design_gateway_api
- Uses CRD discovery results to determine available API versions and provider

### Story 7.2: Istio Design Guidance

As an AI agent,
I want to generate Istio configurations based on user intent,
So that I can help users create correct mesh policies and routing configurations.

**Acceptance Criteria:**

**Given** a user intent (e.g., "enable mTLS between services", "route 80% traffic to v2")
**When** the agent calls `design_istio` with intent parameters
**Then** it returns (FR36):
- Required Istio resources (PeerAuthentication, DestinationRule, VirtualService, AuthorizationPolicy)
- Annotated YAML templates
- Verification that referenced services, namespaces, and ports exist
- Conflict warnings against existing Istio policies

**Given** templates are generated
**When** returned to the agent
**Then** they reference only Istio API versions installed in the cluster

**Implementation Notes:**
- New tool: design_istio
- Cross-references existing Istio resources to detect conflicts

### Story 7.3: kgateway Design Guidance

As an AI agent,
I want to generate kgateway configurations based on user intent,
So that I can help users create correct kgateway-specific resources.

**Acceptance Criteria:**

**Given** a user intent
**When** the agent calls `design_kgateway` with intent parameters
**Then** it returns (FR37):
- Required kgateway resources (RouteOption, VirtualHostOption, GatewayParameters)
- Annotated YAML templates
- Verification that referenced upstreams and Gateways exist
- kgateway-specific best practices (delegation patterns)

**Given** templates are generated
**When** returned to the agent
**Then** they reference only kgateway CRD versions installed in the cluster

**Implementation Notes:**
- New tool: design_kgateway

### Story 7.4: Diagnostic Remediation Suggestions

As an AI agent,
I want to receive suggested remediations for identified diagnostic issues,
So that I can provide actionable fix recommendations to users.

**Acceptance Criteria:**

**Given** an identified diagnostic issue (from any diagnostic domain)
**When** the agent calls `suggest_remediation` with the finding reference
**Then** it returns (FR50):
- Affected resource reference
- Description of the corrective action
- When applicable, an annotated YAML snippet showing the fix

**Given** remediations are requested for a Gateway API misconfiguration
**When** the suggestion is generated
**Then** it provides the correct YAML fix referencing the actual cluster state

**Given** remediations are requested for an Istio policy conflict
**When** the suggestion is generated
**Then** it provides the corrected policy YAML resolving the conflict

**Implementation Notes:**
- New tool: suggest_remediation
- Works with DiagnosticFinding output from all diagnostic tools

---

## Epic 8: Tier 2 Provider Support

Agents get basic visibility and diagnostics for additional mesh and CNI providers.

### Story 8.1: Kuma Basic Diagnostics

As an AI agent,
I want to detect Kuma installation and report mesh status,
So that I can provide basic Kuma visibility to users.

**Acceptance Criteria:**

**Given** Kuma CRDs are installed
**When** the agent calls `check_kuma_status`
**Then** it reports: Kuma control plane health, mesh count, policy count, and data plane proxy status (FR24)

**Given** Kuma is not installed
**When** Kuma tools are queried
**Then** no Kuma tools appear in `tools/list`

**Implementation Notes:**
- New provider: pkg/providers/kuma/
- Implements Provider interface (Tier 2 — no DeepProvider)

### Story 8.2: Linkerd Basic Diagnostics

As an AI agent,
I want to detect Linkerd installation and report configuration health,
So that I can provide basic Linkerd visibility to users.

**Acceptance Criteria:**

**Given** Linkerd CRDs are installed
**When** the agent calls `check_linkerd_status`
**Then** it reports: Linkerd control plane health, proxy injection status, and service profile count (FR25)

**Implementation Notes:**
- New provider: pkg/providers/linkerd/

### Story 8.3: Cilium NetworkPolicy and Connectivity Diagnostics

As an AI agent,
I want to inspect Cilium NetworkPolicies and connectivity status,
So that I can diagnose Cilium-specific networking configurations.

**Acceptance Criteria:**

**Given** Cilium CRDs are installed
**When** the agent calls `list_cilium_policies` with an optional namespace
**Then** it returns CiliumNetworkPolicy and CiliumClusterwideNetworkPolicy resources (FR26)

**Given** Cilium is running
**When** the agent calls `check_cilium_status`
**Then** it reports: Cilium agent health, endpoint count, and basic connectivity status

**Implementation Notes:**
- New provider: pkg/providers/cilium/

### Story 8.4: Calico NetworkPolicy Diagnostics

As an AI agent,
I want to inspect Calico NetworkPolicies,
So that I can diagnose Calico-specific networking configurations.

**Acceptance Criteria:**

**Given** Calico CRDs are installed
**When** the agent calls `list_calico_policies`
**Then** it returns Calico NetworkPolicy and GlobalNetworkPolicy resources (FR27)

**Given** Calico is running
**When** the agent calls `check_calico_status`
**Then** it reports: Calico node health and felix status

**Implementation Notes:**
- New provider: pkg/providers/calico/

### Story 8.5: Flannel Detection and Health

As an AI agent,
I want to detect Flannel installation and report basic health,
So that I can confirm the CNI is functioning.

**Acceptance Criteria:**

**Given** Flannel is installed (detected via kube-flannel DaemonSet)
**When** the agent calls `check_flannel_status`
**Then** it reports: Flannel DaemonSet health, pod status across nodes, and configuration mode (FR28)

**Implementation Notes:**
- New provider: pkg/providers/flannel/

---

## Epic 9: Production Deployment — Helm Chart & Gateway API Exposure

Administrators can deploy the MCP server via Helm and expose it through Gateway API.

### Story 9.1: Helm Chart Foundation

As a cluster administrator,
I want to deploy the MCP server using a Helm chart,
So that I can configure RBAC, resource limits, and feature flags declaratively.

**Acceptance Criteria:**

**Given** the Helm chart is installed with default values
**When** `helm install mcp-k8s-networking ./deploy/helm/mcp-k8s-networking` is run
**Then** it creates: Namespace, ServiceAccount, ClusterRole, ClusterRoleBinding, Deployment, Service, and ConfigMap (FR45)

**Given** the Helm chart
**When** `helm lint` is run
**Then** it passes with no errors (NFR22)

**Given** custom values are provided
**When** the chart is installed with overrides for replicas, resources, config.clusterName, config.logLevel, probe.maxConcurrent
**Then** the deployment reflects all overridden values

**Given** the chart values
**When** feature flags (enableIstio, enableGatewayAPI, etc.) are set to specific values
**Then** the ConfigMap and Deployment env vars reflect the feature flag settings

**Implementation Notes:**
- Create deploy/helm/mcp-k8s-networking/ with Chart.yaml, values.yaml, templates/, _helpers.tpl
- Include all RBAC resources, probe namespace, configmap
- Match architecture's Helm chart values specification

### Story 9.2: Gateway API HTTPRoute Exposure

As a cluster administrator,
I want to expose the MCP server through a Gateway API HTTPRoute,
So that AI agents can connect from outside the cluster via the configured Gateway API provider.

**Acceptance Criteria:**

**Given** the Helm chart values include `gatewayAPI.enabled: true` and `gatewayAPI.provider` (e.g., "istio", "envoy-gateway", "kgateway")
**When** the chart is installed
**Then** it creates an HTTPRoute targeting the MCP server Service with the correct parentRef for the selected provider

**Given** the `gatewayAPI.gatewayName` and `gatewayAPI.gatewayNamespace` are configured
**When** the HTTPRoute is created
**Then** it references the specified Gateway as parentRef

**Given** `gatewayAPI.hostname` is configured
**When** the HTTPRoute is created
**Then** it includes the hostname in the route matching rules

**Given** `gatewayAPI.enabled: false` (default)
**When** the chart is installed
**Then** no HTTPRoute or Gateway API resources are created

**Implementation Notes:**
- Add gatewayAPI section to values.yaml with provider, gatewayName, gatewayNamespace, hostname
- Create templates/httproute.yaml with conditional rendering
- Provider variable affects annotations and parentRef configuration

### Story 9.3: Update Raw YAML Manifests

As a cluster administrator,
I want to deploy the MCP server using raw YAML manifests,
So that I can use kubectl apply without Helm dependency.

**Acceptance Criteria:**

**Given** the raw manifests in deploy/manifests/install.yaml
**When** `kubectl apply -f deploy/manifests/install.yaml` is run
**Then** it creates all required resources: Namespace, ServiceAccount, ClusterRole, ClusterRoleBinding, Deployment, Service (FR46)

**Given** the existing deploy/ YAML files
**When** the manifests are updated
**Then** they match the Helm chart's default configuration for RBAC permissions, resource limits, and deployment spec

**Implementation Notes:**
- Consolidate existing deploy/*.yaml into deploy/manifests/install.yaml
- Add probe namespace creation
- Ensure RBAC matches Helm chart ClusterRole

---

## Epic 10: Documentation Website

Users can access comprehensive mkdocs documentation for the project.

### Story 10.1: mkdocs Site Setup and Structure

As a user,
I want a documentation website with clear navigation,
So that I can quickly find information about the project.

**Acceptance Criteria:**

**Given** the mkdocs project is configured
**When** `mkdocs build` is run
**Then** it generates a static site with navigation sections: Home, Getting Started, Tools Reference, Architecture, Contributing

**Given** the mkdocs.yml configuration
**When** the site is served locally via `mkdocs serve`
**Then** it renders with the Material for MkDocs theme and responsive layout

**Given** the documentation source
**When** changes are pushed to the main branch
**Then** a GitHub Actions workflow builds and deploys the site to GitHub Pages

**Implementation Notes:**
- Create docs/ directory with mkdocs.yml
- Use Material for MkDocs theme
- Add .github/workflows/docs.yml for GitHub Pages deployment

### Story 10.2: Tool Reference Documentation

As a user,
I want complete documentation for each MCP tool,
So that I can understand available diagnostics and how to use them.

**Acceptance Criteria:**

**Given** the Tools Reference section
**When** a user navigates to it
**Then** they find a page for each tool category: Core K8s, Gateway API, Istio, kgateway, Log Collection, Active Probing, Design Guidance

**Given** each tool page
**When** viewed
**Then** it includes: tool name, description, input schema (parameters with types and descriptions), example request, example response, and related tools

**Given** provider-specific tools (Gateway API, Istio, kgateway)
**When** documented
**Then** each notes that it requires the corresponding CRDs to be installed

**Implementation Notes:**
- Create docs/tools/ directory with one page per tool category
- Include JSON schema examples for input and output

### Story 10.3: Setup and Deployment Guide

As a user,
I want step-by-step setup instructions,
So that I can deploy the MCP server in under 5 minutes.

**Acceptance Criteria:**

**Given** the Getting Started section
**When** a user follows the guide
**Then** they can deploy the MCP server using either Helm or raw YAML manifests

**Given** the setup guide
**When** viewed
**Then** it covers: prerequisites (K8s 1.28+, Helm 3+), Helm installation with common value overrides, raw YAML installation, configuration reference (all env vars), verifying the deployment, and connecting an AI agent

**Given** the configuration reference page
**When** viewed
**Then** it documents all environment variables with types, defaults, and descriptions

**Implementation Notes:**
- Create docs/getting-started.md, docs/configuration.md
- Include copy-pasteable commands for quick deployment

### Story 10.4: Architecture and Contributing Guide

As a contributor,
I want architecture documentation and contribution guidelines,
So that I can understand the codebase and contribute effectively.

**Acceptance Criteria:**

**Given** the Architecture section
**When** viewed
**Then** it covers: high-level architecture diagram, component descriptions, data flow, provider interface for adding new providers, and design decisions

**Given** the Contributing section
**When** viewed
**Then** it covers: development setup, running tests, adding a new provider, code style conventions, and PR process

**Implementation Notes:**
- Create docs/architecture.md, docs/contributing.md
- Reference the architecture.md decisions document for accuracy

---

## Epic 11: Agent Skills for Networking Configuration

Agents can follow codified multi-step playbooks (skills) to guide users through configuring networking routes — exposing services via Gateway API, setting up mTLS, configuring traffic splitting, creating NetworkPolicies, and more. Skills combine diagnostic checks with design guidance into interactive step-by-step workflows.

### Story 11.1: Skills Framework and Registry

As an AI agent,
I want a structured skills framework that defines multi-step networking configuration workflows,
So that I can execute playbooks consistently and guide users through complex configurations.

**Acceptance Criteria:**

**Given** the MCP server starts
**When** skills are loaded
**Then** each skill is registered with: name, description, required inputs, step definitions, and prerequisite checks

**Given** an agent requests the list of available skills
**When** `list_skills` is called
**Then** it returns all registered skills with name, description, and required CRDs (only skills whose CRD prerequisites are met)

**Given** a skill requires specific CRDs (e.g., Gateway API)
**When** those CRDs are not installed
**Then** the skill does not appear in the available skills list

**Implementation Notes:**
- New package: pkg/skills/ with framework.go, registry.go, types.go
- Skill definition struct: name, description, steps[], prerequisites[], requiredCRDs[]
- Each step has: action (diagnose, check, generate, validate), tool reference, parameters, success/failure conditions
- Skills registered dynamically based on CRD availability (like tools)

### Story 11.2: Expose Service via Gateway API Skill

As an AI agent,
I want to follow a step-by-step playbook to expose a service via Gateway API,
So that I can guide users through the complete configuration without missing steps.

**Acceptance Criteria:**

**Given** the user wants to expose a service
**When** the agent calls `run_skill` with skill="expose_service_gateway_api" and parameters (service_name, namespace, port, protocol, hostname)
**Then** the skill executes these steps in order:
1. Verify the target service exists and has healthy endpoints
2. Detect installed Gateway API provider (Istio, Envoy Gateway, kgateway)
3. Check for an existing Gateway or recommend creating one
4. Generate the HTTPRoute/GRPCRoute YAML with provider-specific annotations
5. Check if cross-namespace ReferenceGrant is needed and generate it
6. Validate the generated configuration against the cluster state
7. Return the complete set of manifests with apply instructions

**Given** step 1 fails (service not found)
**When** the skill detects the failure
**Then** it returns a diagnostic finding and stops execution with a clear error

**Given** multiple Gateway API providers are available
**When** step 2 detects them
**Then** the skill reports available providers and asks the user to choose (via the agent)

**Implementation Notes:**
- New skill definition in pkg/skills/gateway_expose.go
- Orchestrates calls to: get_service, list_gateways, design_gateway_api, check_gateway_conformance

### Story 11.3: Configure Istio mTLS Skill

As an AI agent,
I want to follow a playbook to configure mTLS between services,
So that I can guide users through setting up service mesh encryption correctly.

**Acceptance Criteria:**

**Given** the user wants to enable mTLS between services
**When** the agent calls `run_skill` with skill="configure_istio_mtls" and parameters (namespace, mode)
**Then** the skill executes:
1. Verify Istio is installed and sidecar injection is enabled for the namespace
2. Check current mTLS status (PeerAuthentication policies)
3. Identify any conflicting DestinationRule TLS settings
4. Generate PeerAuthentication YAML for the desired mode (STRICT, PERMISSIVE)
5. Generate any necessary DestinationRule updates to align TLS settings
6. Validate the generated configuration doesn't conflict with existing policies
7. Return manifests with apply instructions and rollback guidance

**Given** sidecar injection is not enabled for the namespace
**When** the skill detects this
**Then** it returns a warning with instructions to enable injection before applying mTLS

**Implementation Notes:**
- New skill: pkg/skills/istio_mtls.go
- Orchestrates: check_sidecar_injection, check_istio_mtls, design_istio, validate_istio_config

### Story 11.4: Configure Traffic Splitting Skill

As an AI agent,
I want to follow a playbook to configure traffic splitting between service versions,
So that I can guide users through canary or blue-green deployments.

**Acceptance Criteria:**

**Given** the user wants to split traffic between service versions
**When** the agent calls `run_skill` with skill="configure_traffic_split" and parameters (service_name, namespace, versions[], weights[])
**Then** the skill executes:
1. Verify target service exists with healthy endpoints
2. Detect mesh provider (Istio, Linkerd, or use Gateway API HTTPRoute weights)
3. Verify the version deployments exist and have distinct labels
4. Generate the appropriate routing resource (VirtualService for Istio, HTTPRoute with weights for Gateway API)
5. Generate DestinationRule with subset definitions (for Istio)
6. Validate weights sum to 100%
7. Return manifests with apply instructions

**Given** weights don't sum to 100%
**When** the skill validates inputs
**Then** it returns an error before generating any manifests

**Implementation Notes:**
- New skill: pkg/skills/traffic_split.go
- Provider-aware: uses Istio VirtualService or Gateway API HTTPRoute weights

### Story 11.5: Create NetworkPolicy Skill

As an AI agent,
I want to follow a playbook to create NetworkPolicies for service isolation,
So that I can guide users through securing namespace-to-namespace communication.

**Acceptance Criteria:**

**Given** the user wants to restrict traffic to a service
**When** the agent calls `run_skill` with skill="create_network_policy" and parameters (target_service, namespace, allowed_sources[])
**Then** the skill executes:
1. Verify the target service and its pod selector
2. Check existing NetworkPolicies for conflicts
3. Detect CNI provider (Calico, Cilium, or standard K8s NetworkPolicy)
4. Generate the appropriate NetworkPolicy (standard K8s or provider-specific)
5. Validate the policy doesn't block required traffic (DNS, health checks)
6. Return the manifest with apply instructions and verification steps

**Given** the generated policy would block DNS traffic (port 53)
**When** the skill validates the policy
**Then** it automatically includes a DNS egress rule and warns the user

**Implementation Notes:**
- New skill: pkg/skills/network_policy.go
- Supports standard K8s NetworkPolicy and Cilium/Calico variants

---

## Epic 12: OpenTelemetry Instrumentation (GenAI + MCP Semantic Conventions)

The MCP server produces all three OTel signals (traces, metrics, logs) following GenAI and MCP semantic conventions. Every tool invocation produces spans with standardized attributes, GenAI metrics track request performance, custom domain metrics track diagnostic findings and errors, and structured logs are correlated with traces via the OTel log bridge. Platform engineers can observe MCP server behavior through any OTel-compatible backend.

**Requirements:** FR-OTel-1 through FR-OTel-8

### Story 12.1: Full Telemetry Package (TracerProvider + MeterProvider + LoggerProvider) — UPDATED

As a platform engineer,
I want the MCP server to initialize all three OTel signal providers (traces, metrics, logs),
So that I get complete observability via a single OTLP gRPC endpoint.

**Acceptance Criteria:**

**Given** the environment variable `OTEL_EXPORTER_OTLP_ENDPOINT` is set
**When** the MCP server starts
**Then** it initializes:
- TracerProvider with OTLP gRPC trace exporter
- MeterProvider with OTLP gRPC metric exporter (periodic reader, 30s interval)
- LoggerProvider with OTLP gRPC log exporter (batch processor)
- All three share a resource with `service.name="mcp-k8s-networking"`, `service.version`, and `k8s.cluster.name`

**Given** the environment variable `OTEL_EXPORTER_OTLP_ENDPOINT` is NOT set
**When** the MCP server starts
**Then** all signals are disabled (noop providers) and the server operates normally without errors

**Given** the server shuts down
**When** graceful shutdown executes
**Then** all three providers flush pending data before exiting

**Implementation Notes:**
- Update pkg/telemetry/telemetry.go to initialize TracerProvider, MeterProvider, and LoggerProvider
- Add dependencies: go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc, go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc, go.opentelemetry.io/otel/sdk/metric, go.opentelemetry.io/otel/sdk/log
- Return a single Shutdown function that flushes all three providers
- Satisfies: FR-OTel-8

**Status:** done (updated from traces-only to full 3-signal setup)

### Story 12.2: MCP Tool Call Instrumentation Middleware with GenAI/MCP Semconv Attributes — UPDATED

As a platform engineer,
I want every MCP tool call to produce a span following GenAI and MCP semantic conventions,
So that I can observe tool execution with standardized attributes in any OTel-compatible backend.

**Acceptance Criteria:**

**Given** an agent calls any MCP tool
**When** the tool executes
**Then** a span is created with:
- Name: `execute_tool {tool_name}`
- Attributes: `gen_ai.operation.name="execute_tool"`, `gen_ai.tool.name`, `mcp.method.name="tools/call"`, `mcp.protocol.version`, `mcp.session.id`, `jsonrpc.request.id`
- `gen_ai.tool.call.arguments` (sanitized JSON — no secrets/tokens)
- `gen_ai.tool.call.result` (truncated to 1024 chars)
- Status: OK on success, ERROR on failure

**Given** a tool invocation fails
**When** the span is recorded
**Then** the span status is ERROR, `error.type` is set to the MCPError code (e.g., `PROVIDER_NOT_FOUND`) or `"tool_error"`, and the error is recorded as a span event

**Given** a tool invocation completes (success or error)
**When** metrics are recorded
**Then** `gen_ai.server.request.duration` histogram and `gen_ai.server.request.count` counter are updated with dimensions `gen_ai.tool.name` and `error.type`

**Implementation Notes:**
- Wrap buildHandler in pkg/mcp/server.go with instrumentation middleware
- Use otel.Tracer("mcp-k8s-networking") for span creation
- Sanitize arguments before setting as span attribute (remove any keys containing "secret", "token", "key")
- Truncate result to 1024 characters for span attribute
- Satisfies: FR-OTel-1, FR-OTel-3, FR-OTel-6, FR-OTel-7

**Status:** done

### Story 12.3: Context Propagation (Extract from params._meta) — NEW

As a platform engineer,
I want the MCP server to extract W3C trace context from MCP request params._meta,
So that end-to-end traces flow from AI agent through MCP server to K8s API calls.

**Acceptance Criteria:**

**Given** an MCP request contains `params._meta.traceparent` and optionally `params._meta.tracestate`
**When** the tool handler processes the request
**Then** the extracted trace context becomes the parent of the tool invocation span

**Given** an MCP request does NOT contain trace context in `params._meta`
**When** the tool handler processes the request
**Then** a new trace is started (no error, no warning)

**Given** trace context is extracted from the incoming request
**When** the tool makes K8s API calls
**Then** the K8s API call spans are children of the tool invocation span (same trace)

**Implementation Notes:**
- Extract traceparent/tracestate from request params._meta map
- Use otel.GetTextMapPropagator().Extract(ctx, carrier) with MapCarrier
- This must happen before span creation in the middleware
- Satisfies: FR-OTel-2

**Status:** done

### Story 12.4: Custom Domain Metrics (Findings + Errors) — NEW

As a platform engineer,
I want custom metrics tracking diagnostic findings and errors,
So that I can monitor finding severity trends and error patterns.

**Acceptance Criteria:**

**Given** a tool invocation produces diagnostic findings
**When** the response is returned
**Then** `mcp.findings.total` counter is incremented for each finding, dimensioned by `severity` and `analyzer` (tool name)

**Given** a tool invocation produces an error
**When** the error is returned
**Then** `mcp.errors.total` counter is incremented, dimensioned by `error.code` and `gen_ai.tool.name`

**Implementation Notes:**
- Create metrics in pkg/telemetry/ using otel.Meter("mcp-k8s-networking")
- Record findings metrics in the middleware after inspecting ToolResult.Findings
- Record error metrics in the middleware error handling path
- Satisfies: FR-OTel-4

**Status:** done

### Story 12.5: slog → OTel Log Bridge with Trace Correlation — UPDATED

As a platform engineer,
I want slog output bridged to OTel Logs with automatic trace/span ID correlation,
So that I can correlate logs with traces in my observability platform.

**Acceptance Criteria:**

**Given** the OTel LoggerProvider is configured
**When** slog is initialized
**Then** it uses the otelslog bridge handler to export logs via OTLP and inject trace context

**Given** a tool invocation is in progress with an active span
**When** any slog log line is emitted during that invocation
**Then** the log record includes `trace_id` and `span_id` from the active span context

**Given** tracing is disabled (no OTEL_EXPORTER_OTLP_ENDPOINT)
**When** logs are emitted
**Then** slog uses a standard JSON handler without OTel bridging

**Implementation Notes:**
- Use go.opentelemetry.io/contrib/bridges/otelslog to bridge slog with OTel
- Create slog handler in pkg/telemetry/ that wraps otelslog when enabled
- Return the handler from Init so main.go can set it as default
- Satisfies: FR-OTel-5

**Status:** done

### Story 12.6: K8s API Call Spans

As a platform engineer,
I want K8s API calls to produce child spans,
So that I can identify slow API calls and diagnose latency issues in diagnostic tools.

**Acceptance Criteria:**

**Given** a tool makes a K8s API call (e.g., list pods, get service, get logs)
**When** the API call executes
**Then** a child span is created with:
- Name: `k8s.api/{verb}/{resource}` (e.g., `k8s.api/list/services`)
- Attributes: `k8s.namespace`, `k8s.resource.kind`, `k8s.resource.name`, `http.status_code`
- Duration: API call round-trip time

**Given** a K8s API call fails or times out
**When** the span is recorded
**Then** the span status is ERROR with the error details

**Implementation Notes:**
- Wrap K8s client methods in pkg/k8s/ or use client-go transport wrapper
- CRD watch events get individual spans

**Status:** backlog

### Story 12.7: Probe Lifecycle Spans

As a platform engineer,
I want ephemeral probe operations to produce spans,
So that I can observe probe deployment times, execution latency, and cleanup.

**Acceptance Criteria:**

**Given** a probe is requested (connectivity, DNS, or HTTP)
**When** the probe lifecycle executes
**Then** a parent span `probe/{probe_type}` is created with child spans for deploy, wait, execute, cleanup

**Given** a probe times out
**When** the parent span is recorded
**Then** it has status ERROR and a span event recording the timeout

**Implementation Notes:**
- Instrument pkg/probes/manager.go with spans for each lifecycle phase

**Status:** backlog
