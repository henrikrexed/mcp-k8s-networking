# Story 9.3: Update Raw YAML Manifests

Status: done

## Story

As a platform engineer who prefers not to use Helm,
I want a consolidated raw YAML manifest file for mcp-k8s-networking,
so that I can deploy with a single `kubectl apply -f` command.

## Acceptance Criteria

1. A single `deploy/manifests/install.yaml` file contains all resources needed for deployment
2. The manifest creates both the `mcp-k8s-networking` namespace and the `mcp-diagnostics` probe namespace
3. The ClusterRole RBAC rules match the Helm chart ClusterRole (all provider API groups, core K8s, probe pod permissions)
4. The Deployment includes all environment variables (CLUSTER_NAME, PORT, LOG_LEVEL, PROBE_NAMESPACE, PROBE_IMAGE, MAX_CONCURRENT_PROBES)
5. The Deployment includes liveness/readiness health probes on the health port
6. The Deployment includes security context (runAsNonRoot, readOnlyRootFilesystem, allowPrivilegeEscalation: false, seccompProfile: RuntimeDefault)
7. The Service exposes port 8080 as ClusterIP

## Tasks / Subtasks

- [x] Create deploy/manifests/install.yaml with document separator comments
- [x] Add Namespace resources for mcp-k8s-networking and mcp-diagnostics (with purpose label)
- [x] Add ServiceAccount in mcp-k8s-networking namespace
- [x] Add ClusterRole matching Helm chart RBAC (core K8s, CRD discovery, Gateway API, Istio, kgateway, Cilium, Calico, Kuma, Linkerd, ephemeral probes)
- [x] Add ClusterRoleBinding binding to ServiceAccount
- [x] Add Deployment with container image, dual ports (mcp 8080, health 8081), all env vars, health probes, resource requests/limits, security context
- [x] Add Service (ClusterIP on port 8080)
- [x] Add comment at top noting CLUSTER_NAME must be changed before applying

## Dev Notes

### Resource Order

The manifest is ordered for `kubectl apply -f` compatibility: Namespaces first, then ServiceAccount, ClusterRole, ClusterRoleBinding, Deployment, Service. Each resource is separated by `---`.

### CLUSTER_NAME Placeholder

The Deployment includes `CLUSTER_NAME: "my-cluster"` with a comment `# CHANGE THIS to your cluster name`. Users must edit this before applying. The Helm chart solves this with `required` validation, but raw YAML requires manual editing.

### RBAC Parity

The ClusterRole rules exactly match the Helm chart ClusterRole to ensure identical permissions regardless of installation method. All provider API groups have read-only access, and probe pod create/delete is included.

### Differences from Helm Chart

- No templating or conditional rendering
- OTEL env vars not included (add manually if needed)
- Gateway API HTTPRoute not included (separate concern)
- Fixed values instead of configurable defaults

## File List

| File | Action |
|---|---|
| `deploy/manifests/install.yaml` | Created |
