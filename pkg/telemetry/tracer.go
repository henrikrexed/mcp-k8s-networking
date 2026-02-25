package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Providers holds references to the initialized OTel SDK providers.
type Providers struct {
	TracerProvider *sdktrace.TracerProvider
	MeterProvider  *sdkmetric.MeterProvider
	LoggerProvider *sdklog.LoggerProvider
	Enabled        bool
}

// InitResult contains the telemetry initialization outputs.
type InitResult struct {
	Shutdown   func(context.Context) error
	SlogHandler slog.Handler
	Providers  *Providers
}

// Init initializes all three OTel signal providers (traces, metrics, logs).
// If OTEL_EXPORTER_OTLP_ENDPOINT is set, it creates OTLP gRPC exporters for all signals.
// If not set, all signals are disabled (noop providers) and the server operates normally.
// Returns an InitResult with a shutdown function, an slog handler, and provider references.
func Init(ctx context.Context, clusterName string) (*InitResult, error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		slog.Info("telemetry: disabled (OTEL_EXPORTER_OTLP_ENDPOINT not set)")
		return &InitResult{
			Shutdown:    func(ctx context.Context) error { return nil },
			SlogHandler: slog.NewJSONHandler(os.Stdout, nil),
			Providers:   &Providers{Enabled: false},
		}, nil
	}

	res, err := buildResource(clusterName)
	if err != nil {
		return nil, fmt.Errorf("creating OTel resource: %w", err)
	}

	// Initialize TracerProvider
	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP trace exporter: %w", err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Initialize MeterProvider
	metricExporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP metric exporter: %w", err)
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter,
			sdkmetric.WithInterval(30*time.Second),
		)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	// Initialize LoggerProvider
	logExporter, err := otlploggrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP log exporter: %w", err)
	}
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
		sdklog.WithResource(res),
	)

	// Create slog handler bridged to OTel logs
	slogHandler := otelslog.NewHandler("mcp-k8s-networking", otelslog.WithLoggerProvider(lp))

	slog.Info("telemetry: enabled (traces + metrics + logs)", "endpoint", endpoint)

	providers := &Providers{
		TracerProvider: tp,
		MeterProvider:  mp,
		LoggerProvider: lp,
		Enabled:        true,
	}

	shutdown := func(ctx context.Context) error {
		var errs []error
		if err := tp.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("tracer shutdown: %w", err))
		}
		if err := mp.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("meter shutdown: %w", err))
		}
		if err := lp.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("logger shutdown: %w", err))
		}
		if len(errs) > 0 {
			return fmt.Errorf("telemetry shutdown errors: %v", errs)
		}
		return nil
	}

	return &InitResult{
		Shutdown:    shutdown,
		SlogHandler: slogHandler,
		Providers:   providers,
	}, nil
}

func buildResource(clusterName string) (*resource.Resource, error) {
	return resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("mcp-k8s-networking"),
			semconv.ServiceVersion("1.0.0"),
			attribute.String("k8s.cluster.name", clusterName),
		),
	)
}

// InitTracer is a backward-compatible wrapper that initializes only the tracer.
// Deprecated: Use Init() instead for full 3-signal telemetry.
func InitTracer(ctx context.Context, clusterName string) (func(context.Context) error, error) {
	result, err := Init(ctx, clusterName)
	if err != nil {
		return nil, err
	}
	return result.Shutdown, nil
}
