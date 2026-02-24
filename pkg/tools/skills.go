package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/isitobservable/k8s-networking-mcp/pkg/skills"
)

// ListSkillsTool exposes the skills registry as an MCP tool.
type ListSkillsTool struct {
	BaseTool
	Registry *skills.Registry
}

func (t *ListSkillsTool) Name() string { return "list_skills" }
func (t *ListSkillsTool) Description() string {
	return "List available networking configuration skills (multi-step guided workflows)"
}
func (t *ListSkillsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *ListSkillsTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	defs := t.Registry.List()

	type skillInfo struct {
		Name         string              `json:"name"`
		Description  string              `json:"description"`
		RequiredCRDs []string            `json:"requiredCRDs,omitempty"`
		Parameters   []skills.SkillParam `json:"parameters"`
	}

	items := make([]skillInfo, 0, len(defs))
	for _, d := range defs {
		items = append(items, skillInfo{
			Name:         d.Name,
			Description:  d.Description,
			RequiredCRDs: d.RequiredCRDs,
			Parameters:   d.Parameters,
		})
	}

	return NewResponse(t.Cfg, "list_skills", map[string]interface{}{
		"skills": items,
		"count":  len(items),
	}), nil
}

// RunSkillTool executes a named skill with provided arguments.
type RunSkillTool struct {
	BaseTool
	Registry *skills.Registry
}

func (t *RunSkillTool) Name() string { return "run_skill" }
func (t *RunSkillTool) Description() string {
	return "Execute a networking configuration skill (multi-step guided workflow). Use list_skills to see available skills and their parameters."
}
func (t *RunSkillTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"skill_name": map[string]interface{}{
				"type":        "string",
				"description": "Name of the skill to execute (from list_skills)",
			},
			"arguments": map[string]interface{}{
				"type":        "object",
				"description": "Skill-specific arguments (see skill parameters from list_skills)",
			},
		},
		"required": []string{"skill_name"},
	}
}

func (t *RunSkillTool) Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error) {
	skillName := getStringArg(args, "skill_name", "")
	if skillName == "" {
		return nil, fmt.Errorf("skill_name is required")
	}

	skill, ok := t.Registry.Get(skillName)
	if !ok {
		available := t.Registry.List()
		names := make([]string, 0, len(available))
		for _, d := range available {
			names = append(names, d.Name)
		}
		return NewResponse(t.Cfg, "run_skill", map[string]interface{}{
			"error":            fmt.Sprintf("skill %q not found", skillName),
			"available_skills": names,
		}), nil
	}

	// Extract skill arguments
	skillArgs := make(map[string]interface{})
	if a, ok := args["arguments"]; ok {
		switch v := a.(type) {
		case map[string]interface{}:
			skillArgs = v
		case string:
			// Try to parse JSON string
			if err := json.Unmarshal([]byte(v), &skillArgs); err != nil {
				return nil, fmt.Errorf("invalid arguments: %w", err)
			}
		}
	}

	result, err := skill.Execute(ctx, skillArgs)
	if err != nil {
		return nil, fmt.Errorf("skill execution failed: %w", err)
	}

	return NewResponse(t.Cfg, "run_skill", result), nil
}
