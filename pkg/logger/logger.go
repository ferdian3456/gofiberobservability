package logger

import (
	"context"
	"time"

	"gofiberobservability/pkg/config"

	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	loggerProvider *sdklog.LoggerProvider
	zapLogger      *zap.Logger
)

// InitLogger initializes Zap logger with OpenTelemetry OTLP gRPC exporter
func InitLogger(cfg *config.Config) error {
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

	// Configure OTLP gRPC exporter for logs
	exporterOptions := []otlploggrpc.Option{
		otlploggrpc.WithEndpoint(cfg.OTLPEndpoint),
	}

	if cfg.OTLPInsecure {
		exporterOptions = append(exporterOptions, otlploggrpc.WithInsecure())
	}

	exporter, err := otlploggrpc.New(ctx, exporterOptions...)
	if err != nil {
		return err
	}

	// Create batch processor for production efficiency
	processor := sdklog.NewBatchProcessor(
		exporter,
		sdklog.WithExportTimeout(cfg.BatchExportTimeout),
		sdklog.WithExportInterval(cfg.BatchTimeout),
		sdklog.WithMaxQueueSize(cfg.BatchMaxQueueSize),
	)

	// Create logger provider
	loggerProvider = sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(processor),
	)

	// Set as global logger provider
	global.SetLoggerProvider(loggerProvider)

	// 1. Create OTel Zap Core
	otelCore := otelzap.NewCore(cfg.ServiceName, otelzap.WithLoggerProvider(loggerProvider))

	// 2. Create Console Core (JSON)
	zapConfig := zap.NewProductionConfig()
	zapConfig.EncoderConfig.TimeKey = "time"
	zapConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Build a temporary logger to get the production console core
	tempLogger, err := zapConfig.Build()
	if err != nil {
		return err
	}
	consoleCore := tempLogger.Core()

	// 3. Combine cores using Tee
	core := zapcore.NewTee(consoleCore, otelCore)

	// 4. Create the final logger
	zapLogger = zap.New(core,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
		zap.Fields(
			zap.String("service", cfg.ServiceName),
			zap.String("version", cfg.ServiceVersion),
			zap.String("environment", cfg.ServiceEnvironment),
		),
	)

	zapLogger.Info("OpenTelemetry logger initialized",
		zap.String("service", cfg.ServiceName),
		zap.String("version", cfg.ServiceVersion),
		zap.String("environment", cfg.ServiceEnvironment),
		zap.String("otlp_endpoint", cfg.OTLPEndpoint),
	)

	return nil
}

// GetLogger returns the configured Zap logger
func GetLogger() *zap.Logger {
	if zapLogger == nil {
		// Fallback to default logger if not initialized
		logger, _ := zap.NewProduction()
		return logger
	}
	return zapLogger
}

// GetLoggerWithTraceContext returns logger with trace context fields
func GetLoggerWithTraceContext(ctx context.Context) *zap.Logger {
	logger := GetLogger()

	// Extract trace context from context
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		logger = logger.With(
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.String("span_id", span.SpanContext().SpanID().String()),
		)
	}

	return logger
}

// Shutdown gracefully shuts down the logger provider, flushing any pending logs
func Shutdown(ctx context.Context) error {
	if zapLogger != nil {
		_ = zapLogger.Sync()
	}

	if loggerProvider == nil {
		return nil
	}

	zapLogger.Info("Shutting down logger provider...")

	// Create a context with timeout for shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := loggerProvider.Shutdown(shutdownCtx); err != nil {
		zapLogger.Error("Error shutting down logger provider", zap.Error(err))
		return err
	}

	zapLogger.Info("Logger provider shut down successfully")
	return nil
}
