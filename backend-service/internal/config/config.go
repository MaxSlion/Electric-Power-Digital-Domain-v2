package config

import (
	"fmt"
	"os"
)

// Config holds all configuration for the backend service
type Config struct {
	// HTTP Server
	HTTPAddr          string
	RateLimitRPS      int
	RequestTimeoutSec int

	// gRPC
	GRPCAlgoAddr   string
	GRPCResultAddr string

	// Database
	MySQLDSN string

	// Redis
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// Cache Keys
	SchemeCacheKey     string
	ProgressCacheKeyNS string

	// Feature Flags
	EnableSwagger bool
}

// Load reads configuration from environment variables with sensible defaults
func Load() Config {
	return Config{
		// HTTP
		HTTPAddr:          getEnv("HTTP_ADDR", ":8080"),
		RateLimitRPS:      getEnvInt("RATE_LIMIT_RPS", 100),
		RequestTimeoutSec: getEnvInt("REQUEST_TIMEOUT_SEC", 30),

		// gRPC
		GRPCAlgoAddr:   getEnv("ALGO_GRPC_ADDR", "127.0.0.1:50051"),
		GRPCResultAddr: getEnv("RESULT_GRPC_ADDR", ":9090"),

		// MySQL
		MySQLDSN: getEnv("MYSQL_DSN", "root:password@tcp(127.0.0.1:3306)/epdd_db?parseTime=true"),

		// Redis
		RedisAddr:     getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvInt("REDIS_DB", 0),

		// Cache
		SchemeCacheKey:     getEnv("SCHEME_CACHE_KEY", "sys:algo:schemes"),
		ProgressCacheKeyNS: getEnv("PROGRESS_KEY_NS", "job:progress:"),

		// Features
		EnableSwagger: getEnvBool("ENABLE_SWAGGER", true),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	var out int
	_, err := fmt.Sscanf(v, "%d", &out)
	if err != nil {
		return fallback
	}
	return out
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v == "true" || v == "1" || v == "yes"
}
