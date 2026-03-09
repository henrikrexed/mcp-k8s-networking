package types

import "strings"

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

// SeverityIcon returns a compact emoji for the severity level.
func SeverityIcon(severity string) string {
	switch severity {
	case SeverityCritical:
		return "❗"
	case SeverityWarning:
		return "⚠️"
	case SeverityOK:
		return "✅"
	default:
		return "ℹ️"
	}
}

// ToText renders a DiagnosticFinding as a compact single line.
func (f DiagnosticFinding) ToText() string {
	line := SeverityIcon(f.Severity) + " "
	if f.Resource != nil {
		line += f.Resource.Kind + " "
		if f.Resource.Namespace != "" {
			line += f.Resource.Namespace + "/"
		}
		line += f.Resource.Name + " | "
	}
	line += f.Summary
	if f.Detail != "" {
		line += " | " + f.Detail
	}
	if f.Suggestion != "" {
		line += " → " + f.Suggestion
	}
	return line
}

// FindingsToText renders a slice of findings as compact newline-separated text.
func FindingsToText(findings []DiagnosticFinding) string {
	if len(findings) == 0 {
		return "(no findings)"
	}
	lines := make([]string, len(findings))
	for i, f := range findings {
		lines[i] = f.ToText()
	}
	return strings.Join(lines, "\n")
}

// ToolResultToText renders a ToolResult as compact text.
func (tr *ToolResult) ToText() string {
	header := "cluster=" + tr.Metadata.ClusterName
	if tr.Metadata.Namespace != "" {
		header += " ns=" + tr.Metadata.Namespace
	}
	if tr.Metadata.Provider != "" {
		header += " provider=" + tr.Metadata.Provider
	}
	return header + "\n" + FindingsToText(tr.Findings)
}
