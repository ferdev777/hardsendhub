package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all application configuration.
type Config struct {
	// Server settings
	ServerPort string

	// Authentication
	AdminUsername string
	AdminPassword string
	JWTSecret     string
	JWTExpiry     time.Duration

	// Worker pool settings
	WorkerCount int
	MaxRetries  int
	RetryDelay  time.Duration

	// Circuit breaker settings
	CBFailureThreshold int
	CBRecoveryTimeout  time.Duration

	// AWS SES settings
	AWSRegion      string
	SESFromAddress string
	SESRateLimit   int // emails per second

	// Database
	DBPath string

	// File storage
	TempDir string
}

// Load reads configuration from .env file and environment variables with sensible defaults.
func Load() *Config {
	// Load .env file if it exists (does not override existing env vars)
	if err := godotenv.Load(); err != nil {
		log.Println("[Config] No .env file found, using environment variables and defaults")
	} else {
		log.Println("[Config] Loaded configuration from .env file")
	}

	return &Config{
		ServerPort:         getEnv("PORT", "8080"),
		AdminUsername:      getEnv("ADMIN_USERNAME", "hardsendvideodigital"),
		AdminPassword:      getEnv("ADMIN_PASSWORD", "modeloxvz91"),
		JWTSecret:          getEnv("JWT_SECRET", "hardsend-super-secret-key-2026-devrow"),
		JWTExpiry:          parseDuration(getEnv("JWT_EXPIRY", "24h")),
		WorkerCount:        parseInt(getEnv("WORKER_COUNT", "50")),
		MaxRetries:         parseInt(getEnv("MAX_RETRIES", "3")),
		RetryDelay:         parseDuration(getEnv("RETRY_DELAY", "60s")),
		CBFailureThreshold: parseInt(getEnv("CB_FAILURE_THRESHOLD", "5")),
		CBRecoveryTimeout:  parseDuration(getEnv("CB_RECOVERY_TIMEOUT", "300s")),
		AWSRegion:          getEnv("AWS_REGION", "us-east-1"),
		SESFromAddress:     getEnv("SES_FROM_ADDRESS", "noreply@devrow.com"),
		SESRateLimit:       parseInt(getEnv("SES_RATE_LIMIT", "14")),
		DBPath:             getEnv("DB_PATH", "./hardsend_metrics.db"),
		TempDir:            getEnv("TEMP_DIR", "./tmp"),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func parseInt(s string) int {
	val, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return val
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return time.Hour
	}
	return d
}
