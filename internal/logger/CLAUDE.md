# internal/logger

## Purpose
Provides a configured `*slog.Logger` for structured, levelled logging.

## Key Decisions
- **`log/slog` (stdlib, Go 1.21+)**: chosen to avoid an external logging
  dependency (zerolog, zap, etc.). slog is idiomatic Go and sufficient for
  our needs; switching later is easy because we pass `*slog.Logger` around
  rather than a package-level global.
- **JSON handler in production**: makes logs parseable by Datadog/Loki/CloudWatch
  without extra parsing configuration.
- **Text handler in debug**: human-readable for local development.

## Usage
```go
log := logger.New(cfg)
log.Info("server started", "port", cfg.Port)
log.Error("request failed", "err", err, "tag", playerTag)
```

## Gotchas
- The handler is selected based on `LOG_LEVEL == "debug"`. Any other level
  value selects the JSON handler (not just "info").
- There is no package-level default logger. Always construct via `New(cfg)`
  and pass `*slog.Logger` down; avoid `slog.SetDefault` to keep tests hermetic.
