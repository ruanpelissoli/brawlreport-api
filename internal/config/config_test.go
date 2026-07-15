package config

import (
	"os"
	"strings"
	"testing"
)

// setStoreEnv sets the required data-store variables so tests can exercise
// the Brawl Stars API side of Load() in isolation. t.Setenv restores the
// previous values automatically.
func setStoreEnv(t *testing.T) {
	t.Helper()
	t.Setenv("MONGO_URI", "mongodb://localhost:27017")
	t.Setenv("CLICKHOUSE_ADDR", "localhost:9000")
	t.Setenv("CLICKHOUSE_USER", "brawlreport")
}

func TestLoadMissingTokens(t *testing.T) {
	setStoreEnv(t)
	os.Unsetenv("BS_API_TOKENS")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when BS_API_TOKENS is not set, got nil")
	}
}

func TestLoadMissingStoreVars(t *testing.T) {
	t.Setenv("BS_API_TOKENS", "tok")
	os.Unsetenv("MONGO_URI")
	os.Unsetenv("CLICKHOUSE_ADDR")
	os.Unsetenv("CLICKHOUSE_USER")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when data-store variables are not set, got nil")
	}
	if !strings.Contains(err.Error(), "MONGO_URI") {
		t.Errorf("expected error to name the missing variables, got: %v", err)
	}
}

func TestLoadEmptyTokenList(t *testing.T) {
	setStoreEnv(t)
	os.Setenv("BS_API_TOKENS", "  ,  ,  ")
	defer os.Unsetenv("BS_API_TOKENS")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when token list contains only whitespace, got nil")
	}
}

func TestLoadSingleToken(t *testing.T) {
	setStoreEnv(t)
	os.Setenv("BS_API_TOKENS", "my-test-token")
	defer os.Unsetenv("BS_API_TOKENS")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.BSAPITokens) != 1 || cfg.BSAPITokens[0] != "my-test-token" {
		t.Errorf("expected [my-test-token], got %v", cfg.BSAPITokens)
	}
}

func TestLoadMultipleTokens(t *testing.T) {
	setStoreEnv(t)
	os.Setenv("BS_API_TOKENS", "token1, token2 , token3")
	defer os.Unsetenv("BS_API_TOKENS")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.BSAPITokens) != 3 {
		t.Errorf("expected 3 tokens, got %d: %v", len(cfg.BSAPITokens), cfg.BSAPITokens)
	}
	if cfg.BSAPITokens[0] != "token1" || cfg.BSAPITokens[1] != "token2" || cfg.BSAPITokens[2] != "token3" {
		t.Errorf("unexpected token values: %v", cfg.BSAPITokens)
	}
}

func TestLoadDefaults(t *testing.T) {
	setStoreEnv(t)
	os.Setenv("BS_API_TOKENS", "tok")
	defer os.Unsetenv("BS_API_TOKENS")
	os.Unsetenv("BS_API_BASE_URL")
	os.Unsetenv("LOG_LEVEL")
	os.Unsetenv("PORT")
	os.Unsetenv("BS_API_RATE_PER_TOKEN")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BSAPIBaseURL != "https://api.brawlstars.com/v1" {
		t.Errorf("unexpected base URL: %s", cfg.BSAPIBaseURL)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("unexpected log level: %s", cfg.LogLevel)
	}
	if cfg.Port != "8080" {
		t.Errorf("unexpected port: %s", cfg.Port)
	}
	if cfg.BSAPIRatePerToken != 10.0 {
		t.Errorf("unexpected rate: %f", cfg.BSAPIRatePerToken)
	}
}

func TestLoadInvalidRate(t *testing.T) {
	setStoreEnv(t)
	os.Setenv("BS_API_TOKENS", "tok")
	os.Setenv("BS_API_RATE_PER_TOKEN", "not-a-number")
	defer os.Unsetenv("BS_API_TOKENS")
	defer os.Unsetenv("BS_API_RATE_PER_TOKEN")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid rate, got nil")
	}
}

func TestLoadNegativeRate(t *testing.T) {
	setStoreEnv(t)
	os.Setenv("BS_API_TOKENS", "tok")
	os.Setenv("BS_API_RATE_PER_TOKEN", "-5")
	defer os.Unsetenv("BS_API_TOKENS")
	defer os.Unsetenv("BS_API_RATE_PER_TOKEN")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for negative rate, got nil")
	}
}
