# Agent Skills

Skills are multi-step playbooks that guide agents through complex networking configuration tasks. Two tools manage the skills system; the available skills depend on which CRDs are installed.

---

## list_skills

List available networking configuration skills (multi-step guided workflows).

**Parameters:** None.

**Example use cases:**

- Discover which networking skills are available for the current cluster
- Check skill availability after installing new CRDs

---

## run_skill

Execute a networking configuration skill (multi-step guided workflow). Use `list_skills` to see available skills and their parameters.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `skill_name` | string | Yes | Name of the skill to execute (from `list_skills`) |
| `arguments` | object | No | Skill-specific arguments (see skill parameters from `list_skills`) |

**Example use cases:**

- Run a guided workflow to expose a service via Gateway API
- Execute step-by-step mTLS configuration
- Follow a playbook for traffic splitting setup

---

## Available Skills

### expose_service_gateway_api

Step-by-step workflow to expose a service via Gateway API HTTPRoute.

**Requires:** Gateway API CRDs

### configure_istio_mtls

Step-by-step workflow to configure mTLS between services.

**Requires:** Istio CRDs

### configure_traffic_split

Step-by-step workflow to configure traffic splitting (canary/blue-green).

**Requires:** Istio CRDs or Gateway API CRDs

### create_network_policy

Step-by-step workflow to create NetworkPolicies for service isolation.

**Requires:** Always available (uses standard K8s or provider-specific policies)
