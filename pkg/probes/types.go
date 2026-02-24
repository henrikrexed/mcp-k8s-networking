package probes

import "time"

// ProbeType identifies the kind of active probe.
type ProbeType string

const (
	ProbeTypeConnectivity ProbeType = "connectivity"
	ProbeTypeDNS          ProbeType = "dns"
	ProbeTypeHTTP         ProbeType = "http"
)

// ProbeRequest defines the parameters for launching an ephemeral probe pod.
type ProbeRequest struct {
	Type      ProbeType
	Namespace string // source namespace where the probe pod runs
	Command   []string
	Timeout   time.Duration
}

// ProbeResult holds the outcome of a probe execution.
type ProbeResult struct {
	Success  bool
	Output   string
	ExitCode int
	Duration time.Duration
	Error    string
}

const (
	// LabelManagedBy is the label key identifying pods managed by the probe manager.
	LabelManagedBy = "app.kubernetes.io/managed-by"
	// LabelManagedByValue is the label value for probe pods.
	LabelManagedByValue = "mcp-k8s-networking"
	// LabelProbeType is the label key for the probe type.
	LabelProbeType = "mcp-probe-type"
	// AnnotationCreatedAt records the pod creation timestamp for TTL cleanup.
	AnnotationCreatedAt = "mcp-k8s-networking/created-at"
)
