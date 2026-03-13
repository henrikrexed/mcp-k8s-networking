# Tier 2 Provider Tools

These 8 tools are available when their respective provider CRDs are detected.

---

## Cilium

Requires: `cilium.io` CRDs

### list_cilium_policies

List Cilium NetworkPolicies and CiliumClusterwideNetworkPolicies with L3/L4/L7 rule counts and endpoint selector labels.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | No | Namespace (empty for all namespaces) |

**Example use cases:**

- Audit Cilium network policies across the cluster
- Find policies with L7 HTTP/gRPC/Kafka rules
- Review endpoint selector coverage

### get_cilium_policy

Get detailed CiliumNetworkPolicy view with L7 HTTP/gRPC/Kafka rules, L4 port rules, endpoint selector, and affected services.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | Yes | CiliumNetworkPolicy name |
| `namespace` | string | Yes | Kubernetes namespace |

**Example use cases:**

- Inspect L7 HTTP path/method restrictions
- Review gRPC service/method-level access control
- Understand which services are affected by a specific policy

### check_cilium_status

Check Cilium CNI installation health: DaemonSet status, node connectivity, and policy engine status.

**Parameters:** None.

**Example use cases:**

- Verify Cilium agents are running on all nodes
- Check node-to-node connectivity status
- Monitor policy enforcement engine health

---

## Calico

Requires: `crd.projectcalico.org` CRDs

### list_calico_policies

List Calico NetworkPolicies and GlobalNetworkPolicies.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | No | Namespace (empty for all namespaces) |

**Example use cases:**

- Audit Calico network policies and global policies
- Find policies by namespace scope

### check_calico_status

Check Calico node health and felix status.

**Parameters:** None.

**Example use cases:**

- Verify calico-node pods are running on all nodes
- Check kube-controllers health
- Monitor Felix agent status

---

## Kuma

Requires: `kuma.io` CRDs

### check_kuma_status

Check Kuma service mesh status including control plane health, mesh count, and data plane proxy status.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | No | Namespace to check (empty for cluster-wide) |

**Example use cases:**

- Verify the Kuma control plane is healthy
- Check data plane proxy injection across namespaces
- Monitor mesh configuration and proxy counts

---

## Linkerd

Requires: `linkerd.io` CRDs

### check_linkerd_status

Check Linkerd service mesh status including control plane health, proxy injection status, and service profile count.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | No | Namespace to check (empty for cluster-wide) |

**Example use cases:**

- Verify the Linkerd control plane is healthy
- Check proxy injection status across namespaces
- Review service profile counts for traffic management

---

## Flannel

Detected via: DaemonSet presence (no CRDs)

### check_flannel_status

Check Flannel CNI installation health including DaemonSet status and pod health across nodes.

**Parameters:** None.

**Example use cases:**

- Verify Flannel pods are running on all nodes
- Check for Flannel pod restarts or crashloops
- Review Flannel DaemonSet configuration
