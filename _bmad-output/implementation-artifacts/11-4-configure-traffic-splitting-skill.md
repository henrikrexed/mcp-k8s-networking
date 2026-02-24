# Story 11.4: Configure Traffic Splitting Skill

Status: done

## Story

As an AI agent,
I want a guided workflow to configure traffic splitting between service versions,
so that I can generate canary or blue-green deployment manifests using either Istio or Gateway API.

## Acceptance Criteria

1. The `configure_traffic_split` skill is registered when Istio or Gateway API CRDs are detected
2. Input validation verifies version/weight count match and fails early on mismatch
3. Weight validation confirms weights sum to exactly 100% and fails early if not
4. When Istio is available, generates both a VirtualService (with weighted routes) and a DestinationRule (with subset definitions)
5. When only Gateway API is available, generates an HTTPRoute with weighted backendRefs
6. Istio is preferred over Gateway API when both are available
7. The `parseWeights` helper function parses comma-separated weight strings with error handling

## Tasks / Subtasks

- [x] Create pkg/skills/traffic_split.go with TrafficSplitSkill struct (embeds skillBase, has hasIstio and hasGatewayAPI flags)
- [x] Implement Definition() returning skill name, description, required CRDs note, and 4 parameters (service_name, namespace, versions, weights)
- [x] Implement Execute() with validation and generation workflow:
  - Step 0 (validate_inputs): Check version count matches weight count; fail if mismatched
  - Step 1 (verify_service): GET service via dynamic client; fail if not found
  - Step 2 (validate_weights): Sum all weights and verify total is 100; fail if not
  - Step 3 (generate_manifests): Branch on hasIstio vs hasGatewayAPI to generate provider-specific manifests
  - Step 4 (complete): Summary with manifest count and concatenated output
- [x] Implement Istio manifest generation: DestinationRule with subset per version (label: version), VirtualService with weighted route per version
- [x] Implement Gateway API manifest generation: HTTPRoute with weighted backendRefs (service-name-version pattern, default port 80)
- [x] Implement parseWeights helper using fmt.Sscanf for each comma-separated value

## Dev Notes

### Provider Selection

The skill is registered with `hasIstio` and `hasGatewayAPI` boolean flags set from the Features struct in SyncWithFeatures. During execution, Istio is preferred when both are available (checked first in the if/else chain).

### Input Validation

Two validation steps run before manifest generation:
1. **Version/weight count match**: `len(versions) != len(weights)` -> immediate failure with critical finding
2. **Weight sum check**: Sum all weights, must equal 100 -> immediate failure with specific message

Both validations return early with `result.Status = "failed"`, preventing manifest generation with invalid inputs.

### Istio Manifest Generation

For Istio, two manifests are generated:

**DestinationRule**: Defines subsets for each version with `version` label selector
```yaml
subsets:
- name: v1
  labels:
    version: v1
- name: v2
  labels:
    version: v2
```

**VirtualService**: Routes traffic to subsets with specified weights
```yaml
http:
- route:
  - destination:
      host: my-service
      subset: v1
    weight: 80
  - destination:
      host: my-service
      subset: v2
    weight: 20
```

### Gateway API Manifest Generation

For Gateway API, a single HTTPRoute is generated with weighted backendRefs:
```yaml
rules:
- backendRefs:
  - name: my-service-v1
    port: 80
    weight: 80
  - name: my-service-v2
    port: 80
    weight: 20
```

Note: Gateway API weighted routing requires separate Service resources per version (e.g., `my-service-v1`, `my-service-v2`), unlike Istio which uses subset labels on a single service. A suggestion finding notes this requirement.

### parseWeights Implementation

The `parseWeights` function splits on commas, trims whitespace, and uses `fmt.Sscanf` to parse integers. Invalid values are silently skipped (returning 0 parsed count), which allows partial parsing and downstream validation to catch issues via the weight sum check.

```go
func parseWeights(s string) []int {
    parts := strings.Split(s, ",")
    weights := make([]int, 0, len(parts))
    for _, p := range parts {
        w := 0
        if n, _ := fmt.Sscanf(strings.TrimSpace(p), "%d", &w); n > 0 {
            weights = append(weights, w)
        }
    }
    return weights
}
```

## File List

| File | Action |
|---|---|
| `pkg/skills/traffic_split.go` | Created |
