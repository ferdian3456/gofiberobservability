package database

import (
	"context"
	"fmt"
	"time"

	"gofiberobservability/pkg/config"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

var pool *pgxpool.Pool

// InitDatabase initializes the PostgreSQL connection pool with OTEL tracing instrumentation.
func InitDatabase(ctx context.Context, cfg *config.Config, log *zap.Logger) error {
	pgxCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to parse database URL: %w", err)
	}

	// Production-grade pool settings
	pgxCfg.MaxConns = 25
	pgxCfg.MinConns = 5
	pgxCfg.MaxConnLifetime = 1 * time.Hour
	pgxCfg.MaxConnIdleTime = 30 * time.Minute
	pgxCfg.HealthCheckPeriod = 1 * time.Minute

	// OpenTelemetry instrumentation: auto-trace every SQL query
	pgxCfg.ConnConfig.Tracer = otelpgx.NewTracer(
		otelpgx.WithIncludeQueryParameters(),
	)

	pool, err = pgxpool.NewWithConfig(ctx, pgxCfg)
	if err != nil {
		return fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify connectivity
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	log.Info("PostgreSQL connection pool initialized",
		zap.String("host", pgxCfg.ConnConfig.Host),
		zap.Uint16("port", pgxCfg.ConnConfig.Port),
		zap.String("database", pgxCfg.ConnConfig.Database),
		zap.Int32("max_conns", pgxCfg.MaxConns),
		zap.Int32("min_conns", pgxCfg.MinConns),
	)

	return nil
}

// GetPool returns the database connection pool.
func GetPool() *pgxpool.Pool {
	return pool
}

// Close gracefully closes the connection pool.
func Close(log *zap.Logger) {
	if pool != nil {
		pool.Close()
		log.Info("PostgreSQL connection pool closed")
	}
}

// RunMigrations creates the initial schema if it does not exist.
func RunMigrations(ctx context.Context, log *zap.Logger) error {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		email VARCHAR(255) NOT NULL UNIQUE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	`

	if _, err := pool.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Info("Database migrations completed")
	return nil
}

// HealthCheck pings the database and returns an error if unhealthy.
func HealthCheck(ctx context.Context) error {
	return pool.Ping(ctx)
}
