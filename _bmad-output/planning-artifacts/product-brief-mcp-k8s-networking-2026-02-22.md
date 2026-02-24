---
stepsCompleted: [1, 2, 3]
inputDocuments: ['user-provided-context-inline']
date: 2026-02-22
author: Henrik.rexed
---

# Product Brief: mcp-k8s-networking

<!-- Content will be appended sequentially through collaborative workflow steps -->

## Executive Summary

mcp-k8s-networking is an MCP server designed to bring expert-level Kubernetes networking diagnostics to AI agents. Deployed per cluster, it exposes the full K8s networking stack — routing, service mesh, Gateway API, TLS, kube-proxy, and CoreDNS — as structured, agent-consumable tools. Combined with codified troubleshooting skills (playbooks), it enables AI agents to perform end-to-end diagnostic reasoning across single and multi-cluster environments. The primary gap it fills: existing tools show network traffic and state, but none perform the actual diagnostic reasoning needed to identify root causes and suggest fixes. mcp-k8s-networking bridges that gap, serving platform engineers, SREs, and developers through AI agent interfaces.

---

## Core Vision

### Problem Statement

Kubernetes networking is one of the most complex and error-prone areas of cluster operations. When services fail to communicate — whether due to routing misconfigurations, broken mesh policies, TLS issues, or CoreDNS/kube-proxy failures — teams resort to manual detective work with kubectl and log analysis. This is slow, expertise-dependent, and especially painful in multi-cluster environments where no unified diagnostic view exists.

### Problem Impact

- **Silent service degradation**: Mesh misconfigurations can degrade service performance without obvious errors, making root cause detection extremely difficult
- **Long mean-time-to-resolution**: Manual debugging across networking layers and clusters can take hours or days
- **Expertise bottleneck**: Deep K8s networking knowledge is scarce; most teams lack the expertise to efficiently diagnose complex networking issues
- **Evolving complexity**: The Gateway API ecosystem (GAMMA initiative, inference extensions) is rapidly evolving, making it hard for teams to stay current on correct configurations

### Why Existing Solutions Fall Short

- **Cilium Hubble**: Visualizes network flows but does not perform diagnostic reasoning or suggest fixes
- **Kiali**: Provides Istio service mesh visualization but lacks cross-stack and multi-cluster diagnostic capabilities
- **kagent**: Offers partial diagnostic capabilities for Istio but falls short on Gateway API support and full networking stack coverage
- **Commercial APM tools**: Monitor network metrics but don't provide actionable, root-cause-level diagnostics consumable by AI agents
- **Common gap**: All existing tools show *what is happening* but none diagnose *why* or prescribe *what to fix*

### Proposed Solution

An MCP server deployed on each Kubernetes cluster that exposes the full networking stack as diagnostic tools for AI agents. Key components:

- **Per-cluster MCP server**: Inspects and exposes routing state, mesh configurations, Gateway API resources, TLS certificates, kube-proxy health, and CoreDNS status
- **Troubleshooting skills (skills.md)**: Codified diagnostic playbooks and predefined prompts that guide agents through systematic troubleshooting workflows
- **Agent-first output**: Structured responses designed for AI agent consumption, enabling automated diagnostic reasoning
- **Multi-cluster orchestration**: An agent layer that coordinates diagnostics across clusters, correlating findings to identify cross-cluster root causes
- **Living Gateway API knowledge**: Up-to-date expertise on Gateway API specs, GAMMA initiative, and inference extensions

### Key Differentiators

1. **Full networking stack coverage**: Not limited to a single layer — covers routing, service mesh, Gateway API, TLS, kube-proxy, and CoreDNS holistically
2. **Gateway API expertise**: Deep support for the evolving Gateway API ecosystem including GAMMA and inference extensions, where no competitor has strong coverage
3. **Agent-first design**: Built from the ground up for AI agent consumption via MCP, not retrofitted human dashboards
4. **Codified diagnostic intelligence**: Troubleshooting playbooks (skills) that encode senior network engineer expertise into repeatable, agent-executable workflows
5. **Multi-cluster native**: Designed for multi-cluster diagnostic orchestration from day one

## Target Users

### Primary Users

#### Persona 1: Sara — Platform Engineer / SRE (Primary)

**Context:** Sara manages multiple Kubernetes clusters for a mid-to-large organization. She has solid networking fundamentals but struggles to keep pace with the rapidly evolving ecosystem — Gateway API specs change, GAMMA introduces new patterns, and mesh configurations grow more complex with every release.

**Problem Experience:** When she gets the "app is not responding" alert, she starts manual debugging: check the service, then the mesh config, then DNS, then kube-proxy — working down the stack one layer at a time. This is time-consuming and error-prone, especially across multiple clusters.

**Workarounds:** kubectl commands, log tailing, mentally tracking which layers she's already checked, tribal knowledge from past incidents.

**Adoption Role:** Sara is the champion and installer. She has the cluster-level authority to deploy the MCP server, grant the required ClusterRole permissions (kubectl/API access, log access, pod deployment for active diagnostics), and connect it to AI agents. She is the trust-holder who approves a privileged infrastructure component on her clusters.

**Success Vision:** Her AI agent calls the MCP, gets a compact diagnostic summary in seconds across all networking layers, and drills into detail only where issues are detected — turning a hours-long investigation into a minutes-long resolution. The active diagnostic probing (deploying pods for curl and network checks) gives her answers that passive observability tools never could.

#### Persona 2: Marcus — Application Developer (Secondary)

**Context:** Marcus builds and deploys microservices on Kubernetes. He writes Helm charts and service manifests but has limited networking expertise. The networking stack is a black box to him.

**Problem Experience:** When building Helm charts, Marcus sometimes forgets critical networking configurations — a missing port definition, an incorrect service selector, or an incomplete network policy. He discovers these issues after deployment when his service can't communicate, then files a ticket and waits.

**Interaction Model:** Marcus never touches the MCP directly. He interacts through an AI agent (chatbot, CI/CD integration, or IDE copilot) that queries the MCP behind the scenes. The agent translates compact diagnostic results into plain-language guidance he can act on.

**Success Vision:** His AI agent catches misconfigurations and explains networking problems in terms he understands — without requiring deep networking expertise. The compact default output means fast, cheap agent interactions that don't overwhelm him with detail.

#### Persona 3: Priya — Cloud Architect (Secondary)

**Context:** Priya designs the networking topology, mesh policies, and Gateway API configurations across the organization's multi-cluster environment. She makes the high-level decisions about how traffic flows between services and clusters.

**Problem Experience:** Validating that her designs work correctly across clusters is difficult. When production behavior diverges from her design intent, diagnosing whether the issue is in architecture or implementation is manual and slow.

**Interaction Model:** Priya uses the MCP through an AI agent during design reviews and post-incident analysis. She leverages the progressive detail capability — starting with compact summaries across clusters, then drilling into full diagnostic data on specific Gateway API configurations or mesh policies that need validation.

**Success Vision:** The MCP agent validates her configurations against best practices and identifies exactly where implementation diverges from architecture — with the depth of data she needs for informed decisions.

### Secondary Stakeholders

- **AI Agent Platforms**: The primary technical consumers — AI agents that use the MCP tools to perform diagnostic reasoning. The compact-by-default, detail-on-demand output format is designed specifically for token-efficient agent consumption.
- **DevOps Team Leads**: Benefit from reduced MTTR and fewer escalations; may champion organizational adoption.

### User Journey

1. **Discovery:** Users find mcp-k8s-networking through conference talks, blog posts, and the open-source community (CNCF ecosystem)
2. **Onboarding:** Sara deploys the MCP server on a cluster, grants the required ClusterRole permissions and connects it to an AI agent. The "aha!" moment comes when the agent detects and explains a complex networking issue — using active diagnostic probing — that would have taken hours to diagnose manually
3. **Core Usage:** Day-to-day, the MCP serves as the networking diagnostic backbone. Agents query with compact summary calls for fast triage, drilling into full detail only when issues are detected — keeping token costs low and reasoning fast
4. **Success Moment:** The first time active probing catches a silent mesh degradation or a subtle Gateway API misconfiguration that passive observability tools missed entirely
5. **Long-term:** Adopted on a case-by-case basis initially; the ultimate success is becoming a default component in every cluster setup — as standard as monitoring or logging
