package database

import (
	"context"
	"fmt"

	"gofiberobservability/pkg/config"

	redisotel "github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var rdb *redis.Client

// InitRedis initializes the Redis connection pool with OpenTelemetry instrumentation.
func InitRedis(ctx context.Context, cfg *config.Config, log *zap.Logger) error {
	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	rdb = redis.NewClient(opt)

	// Add OpenTelemetry instrumentation
	if err := redisotel.InstrumentTracing(rdb); err != nil {
		return fmt.Errorf("failed to instrument Redis tracing: %w", err)
	}
	if err := redisotel.InstrumentMetrics(rdb); err != nil {
		return fmt.Errorf("failed to instrument Redis metrics: %w", err)
	}

	// Verify connection
	if err := rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	log.Info("Redis initialized",
		zap.String("addr", opt.Addr),
		zap.Int("db", opt.DB),
	)

	return nil
}

// GetRedis returns the global Redis client.
func GetRedis() *redis.Client {
	return rdb
}

// CloseRedis closes the Redis connection.
func CloseRedis(log *zap.Logger) {
	if rdb != nil {
		if err := rdb.Close(); err != nil {
			log.Error("Failed to close Redis", zap.Error(err))
		} else {
			log.Info("Redis connection closed")
		}
	}
}

// RedisHealthCheck checks the health of the Redis connection.
func RedisHealthCheck(ctx context.Context) error {
	if rdb == nil {
		return fmt.Errorf("redis client not initialized")
	}
	return rdb.Ping(ctx).Err()
}
