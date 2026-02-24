# Story 11.1: Skills Framework and Registry

Status: done

## Story

As an AI agent,
I want access to multi-step guided workflows (skills) that produce configuration manifests,
so that I can help users configure networking resources with step-by-step validation and YAML generation.

## Acceptance Criteria

1. A `pkg/skills/` package exists with types for skill definitions, steps, and results
2. The `Skill` interface requires `Definition() SkillDefinition` and `Execute(ctx, args) (*SkillResult, error)`
3. The `Registry` provides thread-safe `Register`, `Unregister`, `Get`, and `List` operations using `sync.RWMutex`
4. `SyncWithFeatures` registers/unregisters skills based on discovered CRD features (Gateway API, Istio, etc.)
5. MCP wrapper tools `list_skills` and `run_skill` exist in `pkg/tools/skills.go` for agent interaction
6. `run_skill` validates skill_name with `MCPError` for missing/invalid input
7. The skills registry is created in `cmd/server/main.go` and `SyncWithFeatures` is called in the onChange callback

## Tasks / Subtasks

- [x] Create pkg/skills/types.go with StepAction constants (diagnose, check, generate, validate), Step struct, StepResult struct, SkillResult struct, SkillDefinition struct, SkillParam struct
- [x] Create pkg/skills/registry.go with Skill interface, Registry struct (map + RWMutex), NewRegistry(), Register(), Unregister(), Get(), List(), SyncWithFeatures(), skillBase struct (cfg + clients)
- [x] Implement SyncWithFeatures to register ExposeServiceSkill (Gateway API), ConfigureMTLSSkill (Istio), TrafficSplitSkill (Istio or Gateway API), NetworkPolicySkill (always)
- [x] Create pkg/tools/skills.go with ListSkillsTool (returns skill definitions with count) and RunSkillTool (executes named skill with arguments)
- [x] Implement RunSkillTool with MCPError validation for missing skill_name and JSON argument parsing (string or map)
- [x] Update cmd/server/main.go to create skills.NewRegistry(), register ListSkillsTool and RunSkillTool, call SyncWithFeatures in onChange

## Dev Notes

### Type System

The types are designed for structured multi-step workflow output:

- **StepAction**: Enum of `diagnose`, `check`, `generate`, `validate` describing what a step does
- **Step**: Named step with description and action type
- **StepResult**: Outcome of a step with status (`passed`, `failed`, `warning`, `skipped`), DiagnosticFindings, and optional output text
- **SkillResult**: Complete execution result with skill name, overall status (`completed`, `failed`, `partial`), steps array, generated manifests, and summary
- **SkillDefinition**: Metadata for listing: name, description, required CRDs, parameters
- **SkillParam**: Parameter metadata: name, type, required flag, description

### Registry Thread Safety

The Registry uses `sync.RWMutex` for concurrent access: `RLock` for Get/List (multiple readers), `Lock` for Register/Unregister (exclusive writer). This is critical because SyncWithFeatures is called from the CRD watch callback goroutine while List/Get are called from MCP request handlers.

### SyncWithFeatures Logic

```
Gateway API detected    -> register ExposeServiceSkill
Gateway API removed     -> unregister "expose_service_gateway_api"
Istio detected          -> register ConfigureMTLSSkill
Istio removed           -> unregister "configure_istio_mtls"
Istio OR Gateway API    -> register TrafficSplitSkill
Neither                 -> unregister "configure_traffic_split"
Always                  -> register NetworkPolicySkill (with Cilium/Calico flags)
```

### skillBase Pattern

The `skillBase` struct provides shared dependencies (`cfg *config.Config`, `clients *k8s.Clients`) to all skill implementations, avoiding repeated fields in each skill struct.

### MCP Tool Wrappers

- **list_skills**: No parameters, returns array of SkillDefinition with count
- **run_skill**: Requires `skill_name` (string), optional `arguments` (object or JSON string). Returns SkillResult via NewResponse.

### RunSkillTool Error Handling

The run_skill tool handles three error cases:
1. Missing skill_name: returns MCPError with ErrCodeInvalidInput
2. Skill not found: returns a response listing available skills (not an error, helps agent self-correct)
3. Invalid arguments JSON: returns MCPError with ErrCodeInvalidInput

## File List

| File | Action |
|---|---|
| `pkg/skills/types.go` | Created |
| `pkg/skills/registry.go` | Created |
| `pkg/tools/skills.go` | Created |
| `cmd/server/main.go` | Modified (skills registry creation, tool registration, SyncWithFeatures in onChange) |
