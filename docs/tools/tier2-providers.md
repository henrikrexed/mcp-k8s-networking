# Tier 2 Provider Tools

These tools are available when their respective provider CRDs are detected.

## Kuma

### check_kuma_status

Control plane health, mesh count, and data plane proxy status.

Requires: `kuma.io` CRDs

## Linkerd

### check_linkerd_status

Control plane health, proxy injection status, and service profile count.

Requires: `linkerd.io` CRDs

## Cilium

### list_cilium_policies

List CiliumNetworkPolicy and CiliumClusterwideNetworkPolicy resources.

### check_cilium_status

Cilium agent health, endpoint count, and connectivity status.

Requires: `cilium.io` CRDs

## Calico

### list_calico_policies

List Calico NetworkPolicy and GlobalNetworkPolicy resources.

### check_calico_status

Calico node health and kube-controllers status.

Requires: `crd.projectcalico.org` CRDs

## Flannel

### check_flannel_status

Flannel DaemonSet health, pod status across nodes, and configuration.

Detected via: DaemonSet presence (no CRDs)
