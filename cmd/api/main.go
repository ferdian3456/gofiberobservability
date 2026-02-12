package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gofiberobservability/internal/handler"
	"gofiberobservability/internal/middleware"
	"gofiberobservability/pkg/config"
	"gofiberobservability/pkg/database"
	"gofiberobservability/pkg/logger"
	"gofiberobservability/pkg/metrics"
	"gofiberobservability/pkg/tracer"

	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
)

func main() {
	// Initialize configuration
	cfg := config.NewConfig()

	// Initialize OpenTelemetry logger
	if err := logger.InitLogger(cfg); err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}
	defer logger.Shutdown(context.Background())

	log := logger.GetLogger()

	// Initialize OpenTelemetry tracer
	if err := tracer.InitTracer(cfg, log); err != nil {
		log.Fatal("Failed to initialize tracer", zap.Error(err))
	}
	defer tracer.Shutdown(context.Background(), log)

	// Initialize OpenTelemetry metrics
	if err := metrics.InitMetrics(cfg, log); err != nil {
		log.Fatal("Failed to initialize metrics", zap.Error(err))
	}
	defer metrics.Shutdown(context.Background(), log)

	// Initialize PostgreSQL database
	dbCtx, dbCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer dbCancel()

	if err := database.InitDatabase(dbCtx, cfg, log); err != nil {
		log.Fatal("Failed to initialize database", zap.Error(err))
	}
	defer database.Close(log)

	// Initialize Redis
	if err := database.InitRedis(dbCtx, cfg, log); err != nil {
		log.Fatal("Failed to initialize Redis", zap.Error(err))
	}
	defer database.CloseRedis(log)

	// Run migrations
	if err := database.RunMigrations(dbCtx, log); err != nil {
		log.Fatal("Failed to run database migrations", zap.Error(err))
	}

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName: cfg.ServiceName,
		ErrorHandler: func(c fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}

			log := logger.GetLoggerWithTraceContext(c.Context())
			log.Error("Request error",
				zap.Error(err),
				zap.Int("status_code", code),
				zap.String("path", c.Path()),
			)

			return c.Status(code).JSON(fiber.Map{
				"error": err.Error(),
			})
		},
	})

	// Register middleware (order matters!)
	app.Use(middleware.RecoveryMiddleware(log))

	// Add tracing middleware if tracing is enabled
	if cfg.TracingEnabled {
		app.Use(middleware.TracingMiddleware(cfg.ServiceName))
	}

	app.Use(middleware.LoggingMiddleware())

	// Favicon handler to stay silent in logs
	app.Get("/favicon.ico", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	// Routes
	app.Get("/", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Hello, World!",
			"service": cfg.ServiceName,
			"version": cfg.ServiceVersion,
		})
	})

	// Test route for panics
	app.Get("/debug/panic", func(c fiber.Ctx) error {
		panic("THIS IS A TEST PANIC")
	})

	// Test route for errors
	app.Get("/debug/error", func(c fiber.Ctx) error {
		return fiber.NewError(fiber.StatusBadRequest, "This is a deliberate error")
	})

	// Health check
	app.Get("/health", handler.HealthCheck())

	// User CRUD (backed by PostgreSQL)
	app.Get("/api/users", handler.ListUsers(cfg.ServiceName))
	app.Post("/api/users", handler.CreateUser(cfg.ServiceName))
	app.Get("/api/users/:id", handler.GetUser(cfg.ServiceName))
	app.Delete("/api/users/:id", handler.DeleteUser(cfg.ServiceName))

	// Error simulation endpoint
	app.Get("/api/error", func(c fiber.Ctx) error {
		ctx := c.Context()
		log := logger.GetLoggerWithTraceContext(ctx)
		log.Error("Simulated error endpoint called")
		return fiber.NewError(fiber.StatusInternalServerError, "This is a simulated error")
	})

	app.Get("/api/panic", func(c fiber.Ctx) error {
		panic("This is a simulated panic!")
	})

	// Graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server in a goroutine
	go func() {
		log.Info("Starting server", zap.String("port", "3002"), zap.Bool("prefork", cfg.Prefork))
		if err := app.Listen(":3002", fiber.ListenConfig{EnablePrefork: cfg.Prefork}); err != nil {
			log.Error("Server error", zap.Error(err))
			cancel()
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		log.Info("Shutting down server...")
	case <-ctx.Done():
		log.Info("Context cancelled")
	}

	// Shutdown server
	log.Info("Shutting down server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		log.Error("Server shutdown error", zap.Error(err))
	}

	log.Info("Server shutdown complete")

	// Logger, tracer, metrics, and database will be shut down by defer statements
}
