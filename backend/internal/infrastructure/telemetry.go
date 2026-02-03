package infrastructure

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Telemetry handles all observability concerns: tracing and metrics
type Telemetry struct {
	TracerProvider     *sdktrace.TracerProvider
	MeterProvider      *sdkmetric.MeterProvider
	PrometheusExporter *prometheus.Exporter
	Tracer             trace.Tracer
	Meter              metric.Meter
	config             *TelemetryConfig
	logger             *zap.Logger
}

// TelemetryMetrics contains pre-created metrics for common operations
type TelemetryMetrics struct {
	HTTPRequestDuration metric.Float64Histogram
	HTTPRequestCount    metric.Int64Counter
	ActiveContests      metric.Int64UpDownCounter
	DBQueryDuration     metric.Float64Histogram
	ProblemsSolved      metric.Int64Counter
}

// NewTelemetry initializes OpenTelemetry with tracing and metrics
func NewTelemetry(ctx context.Context, config *TelemetryConfig, logger *zap.Logger) (*Telemetry, error) {
	if !config.Enabled {
		logger.Info("Telemetry disabled, using noop providers")
		return &Telemetry{
			Tracer: otel.Tracer(config.ServiceName),
			Meter:  otel.Meter(config.ServiceName),
			config: config,
			logger: logger,
		}, nil
	}

	// Create resource with service information
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(config.ServiceName),
			semconv.ServiceVersion(config.ServiceVersion),
			attribute.String("environment", "production"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Initialize trace exporter
	traceExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(config.OTLPEndpoint),
		otlptracehttp.WithInsecure(), // Use TLS in production
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Create tracer provider with batching for performance
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(traceExporter,
			sdktrace.WithBatchTimeout(5*time.Second),
			sdktrace.WithMaxExportBatchSize(512),
		),
		sdktrace.WithSampler(sdktrace.ParentBased(
			sdktrace.TraceIDRatioBased(0.1), // Sample 10% of traces
		)),
	)

	// Initialize Prometheus exporter for metrics
	promExporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create Prometheus exporter: %w", err)
	}

	// Create meter provider
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(promExporter),
	)

	// Set global providers
	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)

	logger.Info("Telemetry initialized",
		zap.String("service", config.ServiceName),
		zap.String("version", config.ServiceVersion),
		zap.String("otlp_endpoint", config.OTLPEndpoint),
	)

	return &Telemetry{
		TracerProvider:     tracerProvider,
		MeterProvider:      meterProvider,
		PrometheusExporter: promExporter,
		Tracer:             tracerProvider.Tracer(config.ServiceName),
		Meter:              meterProvider.Meter(config.ServiceName),
		config:             config,
		logger:             logger,
	}, nil
}

// CreateMetrics initializes all application metrics
func (t *Telemetry) CreateMetrics() (*TelemetryMetrics, error) {
	httpDuration, err := t.Meter.Float64Histogram(
		"http.request.duration",
		metric.WithDescription("HTTP request duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	httpCount, err := t.Meter.Int64Counter(
		"http.request.count",
		metric.WithDescription("Total number of HTTP requests"),
	)
	if err != nil {
		return nil, err
	}

	activeContests, err := t.Meter.Int64UpDownCounter(
		"contests.active",
		metric.WithDescription("Number of currently active contests"),
	)
	if err != nil {
		return nil, err
	}

	dbDuration, err := t.Meter.Float64Histogram(
		"db.query.duration",
		metric.WithDescription("Database query duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	problemsSolved, err := t.Meter.Int64Counter(
		"problems.solved",
		metric.WithDescription("Total number of problems solved"),
	)
	if err != nil {
		return nil, err
	}

	return &TelemetryMetrics{
		HTTPRequestDuration: httpDuration,
		HTTPRequestCount:    httpCount,
		ActiveContests:      activeContests,
		DBQueryDuration:     dbDuration,
		ProblemsSolved:      problemsSolved,
	}, nil
}

// Shutdown gracefully shuts down telemetry providers
func (t *Telemetry) Shutdown(ctx context.Context) error {
	if t.TracerProvider != nil {
		if err := t.TracerProvider.Shutdown(ctx); err != nil {
			t.logger.Error("Failed to shutdown tracer provider", zap.Error(err))
		}
	}
	if t.MeterProvider != nil {
		if err := t.MeterProvider.Shutdown(ctx); err != nil {
			t.logger.Error("Failed to shutdown meter provider", zap.Error(err))
		}
	}
	t.logger.Info("Telemetry shutdown complete")
	return nil
}

// SpanFromContext extracts the current span from context
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// StartSpan creates a new span with the given name
func (t *Telemetry) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return t.Tracer.Start(ctx, name, opts...)
}
