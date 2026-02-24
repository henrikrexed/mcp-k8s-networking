package types

import "fmt"

// Error code constants for agent-facing errors.
const (
	ErrCodeProviderNotFound  = "PROVIDER_NOT_FOUND"
	ErrCodeCRDNotAvailable   = "CRD_NOT_AVAILABLE"
	ErrCodeInvalidInput      = "INVALID_INPUT"
	ErrCodeInternalError     = "INTERNAL_ERROR"
	ErrCodeProbeTimeout      = "PROBE_TIMEOUT"
	ErrCodeProbeLimitReached = "PROBE_LIMIT_REACHED"
	ErrCodeAuthFailed        = "AUTH_FAILED"
)

// MCPError represents a structured error returned to AI agents.
type MCPError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Tool    string `json:"tool"`
	Detail  string `json:"detail,omitempty"`
}

func (e *MCPError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("[%s] %s: %s (%s)", e.Code, e.Tool, e.Message, e.Detail)
	}
	return fmt.Sprintf("[%s] %s: %s", e.Code, e.Tool, e.Message)
}
