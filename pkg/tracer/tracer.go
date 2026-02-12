package tracer

import (
	"context"
	"time"

	"gofiberobservability/pkg/config"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.uber.org/zap"
)

var (
	tracerProvider *sdktrace.TracerProvider
)

// InitTracer initializes the OpenTelemetry tracer with OTLP gRPC exporter
func InitTracer(cfg *config.Config, logger *zap.Logger) error {
	if !cfg.TracingEnabled {
		logger.Info("Tracing is disabled")
		return nil
	}

	ctx := context.Background()

	// Create resource with service metadata
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.ServiceEnvironment),
			attribute.String("build.commit", config.Commit),
			attribute.String("build.time", config.BuildTime),
		),
		resource.WithHost(),
		resource.WithOS(),
		resource.WithProcess(),
	)
	if err != nil {
		return err
	}

	// Configure OTLP gRPC exporter for traces
	exporterOptions := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
	}

	if cfg.OTLPInsecure {
		exporterOptions = append(exporterOptions, otlptracegrpc.WithInsecure())
	}

	exporter, err := otlptracegrpc.New(ctx, exporterOptions...)
	if err != nil {
		return err
	}

	// Create batch span processor
	batchProcessor := sdktrace.NewBatchSpanProcessor(
		exporter,
		sdktrace.WithMaxQueueSize(cfg.TraceExportBatch),
		sdktrace.WithBatchTimeout(cfg.BatchTimeout),
		sdktrace.WithExportTimeout(cfg.BatchExportTimeout),
	)

	// Create tracer provider with sampling
	sampler := sdktrace.ParentBased(
		sdktrace.TraceIDRatioBased(cfg.TraceSampleRate),
	)

	tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sampler),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(batchProcessor),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tracerProvider)

	// Set global propagator for trace context propagation
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	logger.Info("OpenTelemetry tracer initialized",
		zap.String("service", cfg.ServiceName),
		zap.String("version", cfg.ServiceVersion),
		zap.String("environment", cfg.ServiceEnvironment),
		zap.String("otlp_endpoint", cfg.OTLPEndpoint),
		zap.Float64("sample_rate", cfg.TraceSampleRate),
	)

	return nil
}

// Shutdown gracefully shuts down the tracer provider, flushing any pending spans
func Shutdown(ctx context.Context, logger *zap.Logger) error {
	if tracerProvider == nil {
		return nil
	}

	logger.Info("Shutting down tracer provider...")

	// Create a context with timeout for shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := tracerProvider.Shutdown(shutdownCtx); err != nil {
		logger.Error("Error shutting down tracer provider", zap.Error(err))
		return err
	}

	logger.Info("Tracer provider shut down successfully")
	return nil
}

// GetTracerProvider returns the configured tracer provider
func GetTracerProvider() *sdktrace.TracerProvider {
	return tracerProvider
}
