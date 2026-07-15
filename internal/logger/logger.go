// Package logger initialises a structured logger for the service.
// It uses the stdlib log/slog package (Go 1.21+) — zero external dependencies.
package logger

import (
	"log/slog"
	"os"
	"strings"

	"github.com/brawlreport/api/internal/config"
)

// New creates and returns a *slog.Logger configured from cfg.
// In production (LOG_LEVEL != "debug") it uses a JSON handler so log
// aggregators can parse structured fields. In debug mode it uses the
// human-readable text handler.
func New(cfg *config.Config) *slog.Logger {
	var level slog.Level
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if strings.ToLower(cfg.LogLevel) == "debug" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}
