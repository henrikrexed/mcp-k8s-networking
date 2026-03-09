package tools

import (
	"context"
	"time"

	"encoding/json"
	"fmt"
	"strings"

	"github.com/isitobservable/k8s-networking-mcp/pkg/config"
	"github.com/isitobservable/k8s-networking-mcp/pkg/k8s"
	"github.com/isitobservable/k8s-networking-mcp/pkg/types"
)

type Tool interface {
	Name() string
	Description() string
	InputSchema() map[string]interface{}
	Run(ctx context.Context, args map[string]interface{}) (*StandardResponse, error)
}

type StandardResponse struct {
	Cluster   string      `json:"cluster"`
	Timestamp string      `json:"timestamp"`
	Tool      string      `json:"tool"`
	Data      interface{} `json:"data"`
}

func NewResponse(cfg *config.Config, toolName string, data interface{}) *StandardResponse {
	return &StandardResponse{
		Cluster:   cfg.ClusterName,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Tool:      toolName,
		Data:      data,
	}
}

type BaseTool struct {
	Cfg     *config.Config
	Clients *k8s.Clients
}

func getStringArg(args map[string]interface{}, key string, defaultVal string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return defaultVal
}

func getIntArg(args map[string]interface{}, key string, defaultVal int) int {
	if v, ok := args[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return defaultVal
}

// NewToolResultResponse creates a StandardResponse wrapping a ToolResult with auto-populated metadata.
func NewToolResultResponse(cfg *config.Config, toolName string, findings []types.DiagnosticFinding, namespace, provider string) *StandardResponse {
	return &StandardResponse{
		Cluster:   cfg.ClusterName,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Tool:      toolName,
		Data: &types.ToolResult{
			Findings: findings,
			Metadata: types.ClusterMetadata{
				ClusterName: cfg.ClusterName,
				Timestamp:   time.Now().UTC(),
				Namespace:   namespace,
				Provider:    provider,
			},
		},
	}
}

// ToText renders the StandardResponse as compact text for LLM consumption.
// If Data is a *types.ToolResult, uses the structured text formatter.
// Otherwise falls back to a simple key=value format.
func (r *StandardResponse) ToText() string {
	header := fmt.Sprintf("[%s] %s", r.Tool, r.Cluster)

	if tr, ok := r.Data.(*types.ToolResult); ok {
		return header + "\n" + tr.ToText()
	}

	// For non-ToolResult data (e.g. map responses), use compact key=value
	if m, ok := r.Data.(map[string]interface{}); ok {
		var parts []string
		for k, v := range m {
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}
		return header + " | " + strings.Join(parts, " | ")
	}

	// Fallback: marshal to JSON but keep it compact (no indent)
	b, err := json.Marshal(r.Data)
	if err != nil {
		return header + " | (error formatting data)"
	}
	return header + "\n" + string(b)
}
