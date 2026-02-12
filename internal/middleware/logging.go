package middleware

import (
	"time"

	"gofiberobservability/pkg/logger"
	"gofiberobservability/pkg/metrics"

	"github.com/gofiber/fiber/v3"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

// LoggingMiddleware logs incoming requests and outgoing responses with OpenTelemetry trace correlation and metrics
func LoggingMiddleware() fiber.Handler {
	// Initialize metrics for the middleware
	meter := metrics.GetMeter()
	requestCount, _ := meter.Int64Counter("http.requests_total",
		metric.WithDescription("Total number of HTTP requests"),
		metric.WithUnit("{request}"),
	)
	requestDuration, _ := meter.Float64Histogram("http.request.duration_ms",
		metric.WithDescription("HTTP request duration in milliseconds"),
		metric.WithUnit("ms"),
	)
	requestSize, _ := meter.Int64Histogram("http.request.size_bytes",
		metric.WithDescription("HTTP request body size in bytes"),
		metric.WithUnit("By"),
	)
	responseSize, _ := meter.Int64Histogram("http.response.size_bytes",
		metric.WithDescription("HTTP response body size in bytes"),
		metric.WithUnit("By"),
	)

	return func(c fiber.Ctx) error {
		start := time.Now()

		// Get logger with trace context
		log := logger.GetLoggerWithTraceContext(c.Context())

		// Log incoming request
		log.Info("Incoming request",
			zap.String("http.method", c.Method()),
			zap.String("http.route", c.Route().Path),
			zap.String("http.path", c.Path()),
			zap.String("http.user_agent", c.Get("User-Agent")),
			zap.String("http.client_ip", c.IP()),
		)

		// Process request
		err := c.Next()

		// Calculate duration
		duration := time.Since(start)
		statusCode := c.Response().StatusCode()

		// If there's an error and the status code is still 200, it means the error handler
		// hasn't run yet. We should try to determine the intended status code.
		if err != nil && statusCode == fiber.StatusOK {
			statusCode = fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				statusCode = e.Code
			}
		}

		// Performance Optimization: Pass attributes directly to avoid slice allocations where possible
		// Note: OTEL SDKs are optimized for this pattern
		method := c.Method()
		route := c.Route().Path

		// Record traffic and errors
		requestCount.Add(c.Context(), 1, metric.WithAttributes(
			attribute.String("http.method", method),
			attribute.String("http.route", route),
			attribute.Int("http.status_code", statusCode),
		))

		// Record latency
		requestDuration.Record(c.Context(), float64(duration.Milliseconds()), metric.WithAttributes(
			attribute.String("http.method", method),
			attribute.String("http.route", route),
			attribute.Int("http.status_code", statusCode),
		))

		// Record sizes
		reqSize := int64(len(c.Request().Body()))
		respSize := int64(len(c.Response().Body()))

		requestSize.Record(c.Context(), reqSize, metric.WithAttributes(
			attribute.String("http.method", method),
			attribute.String("http.route", route),
		))
		responseSize.Record(c.Context(), respSize, metric.WithAttributes(
			attribute.String("http.method", method),
			attribute.String("http.route", route),
		))

		// Log response (Optimized zap fields)
		log.Info("Request completed",
			zap.String("http.method", method),
			zap.String("http.route", route),
			zap.Int("http.status_code", statusCode),
			zap.Int64("http.request.duration_ms", duration.Milliseconds()),
		)

		// Log error if present
		if err != nil {
			log.Error("Request error",
				zap.String("http.method", method),
				zap.String("http.path", c.Path()),
				zap.Error(err),
			)
		}

		return err
	}
}
