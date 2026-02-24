package types

import "time"

// ClusterMetadata provides context for every tool response.
type ClusterMetadata struct {
	ClusterName string    `json:"clusterName"`
	Timestamp   time.Time `json:"timestamp"`
	Namespace   string    `json:"namespace,omitempty"`
	Provider    string    `json:"provider,omitempty"`
}

// ToolResult is the standard response envelope for all diagnostic tools.
type ToolResult struct {
	Findings []DiagnosticFinding `json:"findings"`
	Metadata ClusterMetadata     `json:"metadata"`
	IsError  bool                `json:"isError,omitempty"`
}
