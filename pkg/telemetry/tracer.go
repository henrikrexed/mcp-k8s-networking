package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// InitTracer initializes the OpenTelemetry TracerProvider.
// If OTEL_EXPORTER_OTLP_ENDPOINT is set, it creates an OTLP gRPC exporter.
// If not set, tracing is disabled (noop tracer) and the server operates normally.
// Returns a shutdown function that flushes pending spans.
func InitTracer(ctx context.Context, clusterName string) (func(context.Context) error, error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		slog.Info("telemetry: tracing disabled (OTEL_EXPORTER_OTLP_ENDPOINT not set)")
		return func(ctx context.Context) error { return nil }, nil
	}

	exporter, err := otlptracegrpc.New(ctx) // reads OTEL_EXPORTER_OTLP_ENDPOINT automatically
	if err != nil {
		return nil, fmt.Errorf("creating OTLP exporter: %w", err)
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("mcp-k8s-networking"),
			attribute.String("k8s.cluster.name", clusterName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("creating resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	slog.Info("telemetry: tracing enabled", "endpoint", endpoint)
	return tp.Shutdown, nil
}
