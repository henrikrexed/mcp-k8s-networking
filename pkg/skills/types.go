package skills

import (
	"github.com/isitobservable/k8s-networking-mcp/pkg/types"
)

// StepAction defines what a skill step does.
type StepAction string

const (
	ActionDiagnose StepAction = "diagnose"
	ActionCheck    StepAction = "check"
	ActionGenerate StepAction = "generate"
	ActionValidate StepAction = "validate"
)

// Step represents a single step in a skill playbook.
type Step struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Action      StepAction `json:"action"`
}

// StepResult holds the outcome of executing a skill step.
type StepResult struct {
	StepName string                   `json:"stepName"`
	Status   string                   `json:"status"` // "passed", "failed", "warning", "skipped"
	Findings []types.DiagnosticFinding `json:"findings,omitempty"`
	Output   string                   `json:"output,omitempty"`
}

// SkillResult is the complete result of executing a skill.
type SkillResult struct {
	SkillName   string       `json:"skillName"`
	Status      string       `json:"status"` // "completed", "failed", "partial"
	Steps       []StepResult `json:"steps"`
	Manifests   []string     `json:"manifests,omitempty"`
	Summary     string       `json:"summary"`
}

// SkillDefinition describes a skill for listing.
type SkillDefinition struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	RequiredCRDs []string `json:"requiredCRDs,omitempty"`
	Parameters   []SkillParam `json:"parameters"`
}

// SkillParam describes a skill input parameter.
type SkillParam struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}
