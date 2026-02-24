# Agent Skills

Skills are multi-step playbooks that guide agents through complex networking configuration tasks.

## list_skills

List available skills based on installed CRDs.

## run_skill

Execute a skill with parameters.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `skill` | string | Yes | Skill name |
| `parameters` | object | Yes | Skill-specific parameters |

## Available Skills

### expose_service_gateway_api

Step-by-step workflow to expose a service via Gateway API HTTPRoute.

Requires: Gateway API CRDs

### configure_istio_mtls

Step-by-step workflow to configure mTLS between services.

Requires: Istio CRDs

### configure_traffic_split

Step-by-step workflow to configure traffic splitting (canary/blue-green).

Requires: Istio CRDs or Gateway API CRDs

### create_network_policy

Step-by-step workflow to create NetworkPolicies for service isolation.

Always available (uses standard K8s or provider-specific policies).
