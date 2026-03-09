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

// FindingsToText renders findings as a compact markdown table.
func FindingsToText(findings []DiagnosticFinding) string {
	if len(findings) == 0 {
		return "(no findings)"
	}

	var sb strings.Builder
	sb.WriteString("| St | Resource | Summary | Detail |\n")
	sb.WriteString("|----|----------|---------|--------|\n")
	for _, f := range findings {
		res := "-"
		if f.Resource != nil {
			res = f.Resource.Kind
			if f.Resource.Namespace != "" {
				res += " " + f.Resource.Namespace + "/" + f.Resource.Name
			} else {
				res += " " + f.Resource.Name
			}
		}
		detail := f.Detail
		if f.Suggestion != "" {
			if detail != "" {
				detail += " → "
			}
			detail += f.Suggestion
		}

		// Escape pipes in content
		res = strings.ReplaceAll(res, "|", "\\|")
		summary := strings.ReplaceAll(f.Summary, "|", "\\|")
		detail = strings.ReplaceAll(detail, "|", "\\|")
		// Replace newlines
		summary = strings.ReplaceAll(summary, "\n", " ")
		detail = strings.ReplaceAll(detail, "\n", " ")

		sb.WriteString("| " + SeverityIcon(f.Severity) + " | " + res + " | " + summary + " | " + detail + " |\n")
	}
	return sb.String()
}
