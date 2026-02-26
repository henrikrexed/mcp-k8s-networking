package tools

import (
	"context"
	"time"

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
