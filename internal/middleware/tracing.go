package middleware

import (
	"github.com/gofiber/fiber/v3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TracingMiddleware creates a custom OpenTelemetry tracing middleware for Fiber v3
func TracingMiddleware(serviceName string) fiber.Handler {
	tracer := otel.Tracer("gofiber-v3-tracing")
	propagator := otel.GetTextMapPropagator()

	return func(c fiber.Ctx) error {
		// Extract context from headers (propagation)
		ctx := propagator.Extract(c.Context(), propagation.HeaderCarrier(c.GetReqHeaders()))

		// Start span
		spanName := c.Method() + " " + c.Path()
		if route := c.Route(); route != nil {
			spanName = c.Method() + " " + route.Path
		}

		ctx, span := tracer.Start(ctx, spanName,
			trace.WithAttributes(
				attribute.String("service.name", serviceName),
				attribute.String("http.method", c.Method()),
				attribute.String("http.url", c.OriginalURL()),
				attribute.String("http.target", c.Path()),
				attribute.String("http.client_ip", c.IP()),
				attribute.String("http.user_agent", c.Get("User-Agent")),
			),
			trace.WithSpanKind(trace.SpanKindServer),
		)
		defer span.End()

		// Update context with span
		// Fiber v3 handles context differently, we can use c.SetContext
		c.SetContext(ctx)

		// Process request
		err := c.Next()

		// Recording response attributes
		statusCode := c.Response().StatusCode()
		span.SetAttributes(attribute.Int("http.status_code", statusCode))

		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}

		// Inject trace context into response headers
		propagator.Inject(ctx, propagation.HeaderCarrier(c.GetRespHeaders()))

		return err
	}
}
