// Package config loads and validates environment-based configuration.
// It reads from environment variables only — no config files — so the
// service is 12-factor compliant and easy to run in containers.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
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
}

// Load reads configuration from environment variables and returns a Config.
// It returns an error if any required variable is missing or invalid.
func Load() (*Config, error) {
	cfg := &Config{
		BSAPIBaseURL:      getEnvOrDefault("BS_API_BASE_URL", "https://api.brawlstars.com/v1"),
		LogLevel:          getEnvOrDefault("LOG_LEVEL", "info"),
		Port:              getEnvOrDefault("PORT", "8080"),
		BSAPIRatePerToken: 10.0,
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

	return cfg, nil
}

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
