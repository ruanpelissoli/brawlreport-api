// Package config loads and validates environment-based configuration.
// A .env file is soft-loaded first (missing file is not an error) so local
// development works out of the box; in production, environment variables are
// expected to be set by the platform.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration for the service.
type Config struct {
	// BSAPIBaseURL is the base URL for the Brawl Stars API.
	// Default: https://api.brawlstars.com/v1
	BSAPIBaseURL string

	// BSAPITokens is the list of API tokens for the Brawl Stars API.
	// At least one token is required. Tokens are rotated round-robin.
	BSAPITokens []string

	// BSAPIRatePerToken is the maximum requests per second per token.
	// Default: 10
	BSAPIRatePerToken float64

	// LogLevel controls log verbosity: "debug", "info", "warn", "error".
	// Default: info
	LogLevel string

	// Port is the HTTP port to listen on.
	// Default: 8080
	Port string

	// MongoDB
	MongoURI string
	MongoDB  string

	// ClickHouse
	ClickHouseAddr     string
	ClickHouseDB       string
	ClickHouseUser     string
	ClickHousePassword string
}

// Load reads configuration from environment variables and returns a Config.
// It attempts to load a .env file first (soft fail — missing file is OK).
// It returns an error if any required variable is missing or invalid.
func Load() (*Config, error) {
	// Soft-load .env: present in local dev, absent in prod containers — both are valid.
	_ = godotenv.Load()

	cfg := &Config{
		BSAPIBaseURL:       getEnvOrDefault("BS_API_BASE_URL", "https://api.brawlstars.com/v1"),
		LogLevel:           getEnvOrDefault("LOG_LEVEL", "info"),
		Port:               getEnvOrDefault("PORT", "8080"),
		BSAPIRatePerToken:  10.0,
		MongoURI:           getEnvOrDefault("MONGO_URI", ""),
		MongoDB:            getEnvOrDefault("MONGO_DB", "brawlreport"),
		ClickHouseAddr:     getEnvOrDefault("CLICKHOUSE_ADDR", ""),
		ClickHouseDB:       getEnvOrDefault("CLICKHOUSE_DB", "brawlreport"),
		ClickHouseUser:     getEnvOrDefault("CLICKHOUSE_USER", ""),
		ClickHousePassword: getEnvOrDefault("CLICKHOUSE_PASSWORD", ""),
	}

	// Parse required BS_API_TOKENS — comma-separated list of API keys.
	rawTokens := os.Getenv("BS_API_TOKENS")
	if rawTokens == "" {
		return nil, errors.New("BS_API_TOKENS environment variable is required but not set; " +
			"set it to a comma-separated list of Brawl Stars API tokens from https://developer.brawlstars.com")
	}
	for _, t := range strings.Split(rawTokens, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			cfg.BSAPITokens = append(cfg.BSAPITokens, t)
		}
	}
	if len(cfg.BSAPITokens) == 0 {
		return nil, errors.New("BS_API_TOKENS contains no valid tokens after parsing; " +
			"provide at least one non-empty token")
	}

	// Optional rate-per-token override.
	if raw := os.Getenv("BS_API_RATE_PER_TOKEN"); raw != "" {
		rate, err := strconv.ParseFloat(raw, 64)
		if err != nil || rate <= 0 {
			return nil, fmt.Errorf("BS_API_RATE_PER_TOKEN must be a positive number, got %q", raw)
		}
		cfg.BSAPIRatePerToken = rate
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	return cfg, nil
}

// validate checks that all required data-store fields are set.
func (c *Config) validate() error {
	var missing []string

	if c.MongoURI == "" {
		missing = append(missing, "MONGO_URI")
	}
	if c.ClickHouseAddr == "" {
		missing = append(missing, "CLICKHOUSE_ADDR")
	}
	if c.ClickHouseUser == "" {
		missing = append(missing, "CLICKHOUSE_USER")
	}

	if len(missing) > 0 {
		return errors.New("missing required environment variables: " + strings.Join(missing, ", "))
	}

	return nil
}

// getEnvOrDefault returns the value of the named environment variable,
// falling back to defaultVal if unset or empty.
func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
