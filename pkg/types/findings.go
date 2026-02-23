package types

// Severity levels for diagnostic findings.
const (
	SeverityCritical = "critical"
	SeverityWarning  = "warning"
	SeverityInfo     = "info"
	SeverityOK       = "ok"
)

// Category constants for diagnostic findings.
const (
	CategoryRouting      = "routing"
	CategoryDNS          = "dns"
	CategoryTLS          = "tls"
	CategoryPolicy       = "policy"
	CategoryMesh         = "mesh"
	CategoryConnectivity = "connectivity"
	CategoryLogs         = "logs"
)

// DiagnosticFinding represents a single diagnostic result.
type DiagnosticFinding struct {
	Severity   string       `json:"severity"`
	Category   string       `json:"category"`
	Resource   *ResourceRef `json:"resource,omitempty"`
	Summary    string       `json:"summary"`
	Detail     string       `json:"detail,omitempty"`
	Suggestion string       `json:"suggestion,omitempty"`
}

// ResourceRef identifies a Kubernetes resource.
type ResourceRef struct {
	Kind       string `json:"kind"`
	Namespace  string `json:"namespace,omitempty"`
	Name       string `json:"name"`
	APIVersion string `json:"apiVersion,omitempty"`
}

// FilterFindings returns a copy of findings with Detail and Suggestion stripped when detail is false.
func FilterFindings(findings []DiagnosticFinding, detail bool) []DiagnosticFinding {
	if detail {
		return findings
	}
	filtered := make([]DiagnosticFinding, len(findings))
	for i, f := range findings {
		filtered[i] = DiagnosticFinding{
			Severity: f.Severity,
			Category: f.Category,
			Resource: f.Resource,
			Summary:  f.Summary,
		}
	}
	return filtered
}
