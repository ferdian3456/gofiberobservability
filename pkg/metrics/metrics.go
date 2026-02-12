package metrics

import (
	"context"
	"fmt"
	"time"

	"gofiberobservability/pkg/config"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	otplexemplar "go.opentelemetry.io/otel/sdk/metric/exemplar"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.uber.org/zap"
)

var (
	meterProvider *sdkmetric.MeterProvider
	meter         metric.Meter
)

// InitMetrics initializes the OpenTelemetry Metrics SDK with OTLP exporter
func InitMetrics(cfg *config.Config, log *zap.Logger) error {
	ctx := context.Background()

	// Create OTLP exporter
	exporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return fmt.Errorf("failed to create metrics exporter: %w", err)
	}

	// Create resource
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			"", // Use empty string to avoid SchemaURL conflict with resource.Default()
			semconv.ServiceNameKey.String(cfg.ServiceName),
			semconv.ServiceVersionKey.String(cfg.ServiceVersion),
			attribute.String("deployment.environment", cfg.ServiceEnvironment),
			attribute.String("build.commit", config.Commit),
			attribute.String("build.time", config.BuildTime),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	// Create MeterProvider with periodic exporting and trace-based exemplars
	meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter,
			sdkmetric.WithInterval(15*time.Second))),
		sdkmetric.WithExemplarFilter(otplexemplar.TraceBasedFilter),
	)

	// Set global MeterProvider
	otel.SetMeterProvider(meterProvider)

	// Create a meter for the application
	meter = meterProvider.Meter(cfg.ServiceName)

	// Register Go runtime metrics (Saturation)
	if err := runtime.Start(); err != nil {
		log.Error("Failed to start runtime metrics", zap.Error(err))
	}

	log.Info("OpenTelemetry metrics initialized",
		zap.String("otlp_endpoint", cfg.OTLPEndpoint),
		zap.String("service", cfg.ServiceName),
	)

	return nil
}

// GetMeter returns the initialized Meter
func GetMeter() metric.Meter {
	return meter
}

// Shutdown flushes and stops the MeterProvider
func Shutdown(ctx context.Context, log *zap.Logger) {
	if meterProvider == nil {
		return
	}

	if err := meterProvider.Shutdown(ctx); err != nil {
		log.Error("Error shutting down meter provider", zap.Error(err))
	} else {
		log.Info("Meter provider shut down successfully")
	}
}
