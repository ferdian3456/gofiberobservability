package middleware

import (
	"fmt"
	"runtime/debug"

	"github.com/gofiber/fiber/v3"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// RecoveryMiddleware returns a middleware that recovers from panics and records them in OpenTelemetry and Logs.
func RecoveryMiddleware(log *zap.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		defer func() {
			if r := recover(); r != nil {
				err, ok := r.(error)
				if !ok {
					err = fmt.Errorf("%v", r)
				}

				stack := debug.Stack()

				// Get trace span
				span := trace.SpanFromContext(c.Context())
				if span.IsRecording() {
					span.SetStatus(codes.Error, "panic recovered")
					span.RecordError(err, trace.WithStackTrace(true))
					span.SetAttributes(
						attribute.String("panic.error", err.Error()),
						attribute.String("panic.stack", string(stack)),
					)
				}

				log.Error("Panic recovered",
					zap.Error(err),
					zap.String("stack", string(stack)),
					zap.String("path", c.Path()),
					zap.String("method", c.Method()),
				)

				c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Internal Server Error",
				})
			}
		}()

		return c.Next()
	}
}
