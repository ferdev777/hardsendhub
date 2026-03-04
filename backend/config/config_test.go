package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	// Unset all env vars to test defaults
	os.Unsetenv("PORT")
	os.Unsetenv("WORKER_COUNT")
	os.Unsetenv("DAILY_LIMIT")
	os.Unsetenv("RESEND_RATE_LIMIT")

	cfg := Load()

	if cfg.ServerPort != "8080" {
		t.Errorf("Default ServerPort = %q, want 8080", cfg.ServerPort)
	}
	if cfg.WorkerCount != 1 {
		t.Errorf("Default WorkerCount = %d, want 1", cfg.WorkerCount)
	}
	if cfg.DailyLimit != 0 {
		t.Errorf("Default DailyLimit = %d, want 0 (must be set from frontend)", cfg.DailyLimit)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("Default MaxRetries = %d, want 3", cfg.MaxRetries)
	}
	if cfg.RetryDelay != 60*time.Second {
		t.Errorf("Default RetryDelay = %v, want 60s", cfg.RetryDelay)
	}
	if cfg.DBPath != "./hardsend_metrics.db" {
		t.Errorf("Default DBPath = %q, want ./hardsend_metrics.db", cfg.DBPath)
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	os.Setenv("PORT", "9090")
	os.Setenv("WORKER_COUNT", "5")
	os.Setenv("DAILY_LIMIT", "3000")
	os.Setenv("RESEND_RATE_LIMIT", "10")
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("WORKER_COUNT")
		os.Unsetenv("DAILY_LIMIT")
		os.Unsetenv("RESEND_RATE_LIMIT")
	}()

	cfg := Load()

	if cfg.ServerPort != "9090" {
		t.Errorf("ServerPort = %q, want 9090", cfg.ServerPort)
	}
	if cfg.WorkerCount != 5 {
		t.Errorf("WorkerCount = %d, want 5", cfg.WorkerCount)
	}
	if cfg.DailyLimit != 3000 {
		t.Errorf("DailyLimit = %d, want 3000", cfg.DailyLimit)
	}
	if cfg.ResendRateLimit != 10 {
		t.Errorf("ResendRateLimit = %d, want 10", cfg.ResendRateLimit)
	}
}

func TestGetEnv(t *testing.T) {
	os.Setenv("TEST_KEY_HARDSEND", "test_value")
	defer os.Unsetenv("TEST_KEY_HARDSEND")

	if got := getEnv("TEST_KEY_HARDSEND", "fallback"); got != "test_value" {
		t.Errorf("getEnv() = %q, want test_value", got)
	}
	if got := getEnv("NONEXISTENT_KEY_XYZ", "default"); got != "default" {
		t.Errorf("getEnv() = %q, want default", got)
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"42", 42},
		{"0", 0},
		{"1500", 1500},
		{"invalid", 0},
		{"", 0},
	}
	for _, tt := range tests {
		if got := parseInt(tt.input); got != tt.want {
			t.Errorf("parseInt(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"60s", 60 * time.Second},
		{"5m", 5 * time.Minute},
		{"1h", time.Hour},
		{"300s", 300 * time.Second},
		{"invalid", time.Hour}, // default on error
	}
	for _, tt := range tests {
		if got := parseDuration(tt.input); got != tt.want {
			t.Errorf("parseDuration(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
