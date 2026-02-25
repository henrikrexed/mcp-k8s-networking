package telemetry

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// WithAttrs returns a metric.MeasurementOption from attribute key-value pairs.
func WithAttrs(attrs ...attribute.KeyValue) metric.MeasurementOption {
	return metric.WithAttributes(attrs...)
}

// Meters holds pre-created OTel metric instruments for MCP server instrumentation.
type Meters struct {
	// GenAI semantic convention metrics
	RequestDuration metric.Float64Histogram
	RequestCount    metric.Int64Counter

	// Custom domain metrics
	FindingsTotal metric.Int64Counter
	ErrorsTotal   metric.Int64Counter
}

// NewMeters creates all OTel metric instruments for MCP server instrumentation.
func NewMeters() (*Meters, error) {
	meter := otel.Meter("mcp-k8s-networking")

	requestDuration, err := meter.Float64Histogram(
		"gen_ai.server.request.duration",
		metric.WithDescription("Duration of MCP tool call execution in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	requestCount, err := meter.Int64Counter(
		"gen_ai.server.request.count",
		metric.WithDescription("Number of MCP tool call requests"),
	)
	if err != nil {
		return nil, err
	}

	findingsTotal, err := meter.Int64Counter(
		"mcp.findings.total",
		metric.WithDescription("Total diagnostic findings emitted"),
	)
	if err != nil {
		return nil, err
	}

	errorsTotal, err := meter.Int64Counter(
		"mcp.errors.total",
		metric.WithDescription("Total tool execution errors"),
	)
	if err != nil {
		return nil, err
	}

	return &Meters{
		RequestDuration: requestDuration,
		RequestCount:    requestCount,
		FindingsTotal:   findingsTotal,
		ErrorsTotal:     errorsTotal,
	}, nil
}
