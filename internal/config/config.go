package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds runtime configuration loaded from the environment.
type Config struct {
	SupabaseDBURL         string
	SupabaseProjectURL    string
	SupabasePublishableKey string
	SupabaseJWTSecret     string
	FrontendOrigins       string
	WebhookURL            string
	ExpiryThresholdDays   int
	ScanIntervalHours     int
	WorkerCount           int
	Port                  string
}

// Load reads .env (if present) and required environment variables.
func Load() (*Config, error) {
	_ = godotenv.Load()

	supabaseDBURL := firstNonEmptyEnv("SUPABASE_DB_URL", "DATABASE_URL")
	supabaseProjectURL := firstNonEmptyEnv("SUPABASE_PROJECT_URL", "SUPABASE_URL", "NEXT_PUBLIC_SUPABASE_URL")
	supabasePublishableKey := firstNonEmptyEnv(
		"SUPABASE_PUBLISHABLE_KEY",
		"SUPABASE_ANON_KEY",
		"NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY",
		"NEXT_PUBLIC_SUPABASE_ANON_KEY",
	)

	cfg := &Config{
		SupabaseDBURL:          supabaseDBURL,
		SupabaseProjectURL:     supabaseProjectURL,
		SupabasePublishableKey: supabasePublishableKey,
		SupabaseJWTSecret:      os.Getenv("SUPABASE_JWT_SECRET"),
		FrontendOrigins:        getStringEnv("FRONTEND_ORIGINS", "*"),
		WebhookURL:             os.Getenv("WEBHOOK_URL"),
		ExpiryThresholdDays:    getIntEnv("EXPIRY_THRESHOLD_DAYS", 15),
		ScanIntervalHours:      getIntEnv("SCAN_INTERVAL_HOURS", 12),
		WorkerCount:            getIntEnv("WORKER_COUNT", 10),
		Port:                   getStringEnv("PORT", "8080"),
	}

	if cfg.SupabaseDBURL == "" {
		return nil, fmt.Errorf("SUPABASE_DB_URL is required")
	}
	if cfg.SupabaseJWTSecret == "" {
		return nil, fmt.Errorf("SUPABASE_JWT_SECRET is required")
	}
	if cfg.SupabaseProjectURL == "" {
		return nil, fmt.Errorf("SUPABASE_PROJECT_URL is required")
	}
	if cfg.SupabasePublishableKey == "" {
		return nil, fmt.Errorf("SUPABASE_PUBLISHABLE_KEY is required")
	}
	if cfg.WorkerCount < 1 {
		cfg.WorkerCount = 1
	}
	return cfg, nil
}

func getStringEnv(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

func getIntEnv(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return ""
}

// ScanInterval returns the configured scan interval as a duration.
func (c *Config) ScanInterval() time.Duration {
	return time.Duration(c.ScanIntervalHours) * time.Hour
}
