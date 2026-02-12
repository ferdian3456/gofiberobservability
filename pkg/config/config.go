package config

import (
	"os"
	"strconv"
	"time"
)

var (
	// Version is the application version, set at build time
	Version = "dev"
	// Commit is the git commit hash, set at build time
	Commit = "none"
	// BuildTime is the time when the binary was built, set at build time
	BuildTime = "unknown"
)

// Config holds the configuration for OpenTelemetry
type Config struct {
	// Service metadata
	ServiceName        string
	ServiceVersion     string
	ServiceEnvironment string

	// OTLP configuration
	OTLPEndpoint string
	OTLPInsecure bool

	// Batch processor configuration
	BatchTimeout       time.Duration
	BatchMaxQueueSize  int
	BatchExportTimeout time.Duration

	// Tracing configuration
	TracingEnabled   bool
	TraceSampleRate  float64 // 0.0 to 1.0 (0.1 = 10%, 1.0 = 100%)
	TraceExportBatch int

	// Server performance tuning
	Prefork bool

	// Database configuration
	DatabaseURL string
	RedisURL    string
}

// NewConfig creates a new configuration with defaults and environment overrides
func NewConfig() *Config {
	return &Config{
		ServiceName:        getEnv("OTEL_SERVICE_NAME", "gofiberobservability"),
		ServiceVersion:     Version, // Use the build-time version
		ServiceEnvironment: getEnv("OTEL_ENVIRONMENT", "development"),

		OTLPEndpoint: getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
		OTLPInsecure: getEnvBool("OTEL_EXPORTER_OTLP_INSECURE", true),

		BatchTimeout:       getEnvDuration("OTEL_BATCH_TIMEOUT", 10*time.Second),
		BatchMaxQueueSize:  getEnvInt("OTEL_BATCH_MAX_QUEUE_SIZE", 2048),
		BatchExportTimeout: getEnvDuration("OTEL_BATCH_EXPORT_TIMEOUT", 30*time.Second),

		// Tracing configuration
		TracingEnabled:   getEnvBool("OTEL_TRACING_ENABLED", true),
		TraceSampleRate:  getEnvFloat("OTEL_TRACE_SAMPLE_RATE", 1.0),
		TraceExportBatch: getEnvInt("OTEL_TRACE_EXPORT_BATCH", 512),

		// Server performance tuning
		Prefork: getEnvBool("FIBER_PREFORK", false),

		// Database configuration
		DatabaseURL: func() string {
			if url := os.Getenv("DATABASE_URL"); url != "" {
				return url
			}
			host := getEnv("DB_HOST", "localhost")
			port := getEnv("DB_PORT", "5432")
			user := getEnv("DB_USER", "appuser")
			pass := getEnv("DB_PASSWORD", "apppass")
			name := getEnv("DB_NAME", "appdb")
			return "postgres://" + user + ":" + pass + "@" + host + ":" + port + "/" + name + "?sslmode=disable"
		}(),
		RedisURL: func() string {
			if url := os.Getenv("REDIS_URL"); url != "" {
				return url
			}
			host := getEnv("REDIS_HOST", "localhost")
			port := getEnv("REDIS_PORT", "6379")
			return "redis://" + host + ":" + port + "/0"
		}(),
	}
}

// Helper functions for environment variable parsing
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			return parsed
		}
	}
	return defaultValue
}
