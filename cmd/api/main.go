// Command api is the BrawlReport backend API server entrypoint.
// It loads configuration from environment variables, initialises structured
// logging, constructs the Brawl Stars API client, and starts the HTTP server.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/brawlreport/api/internal/api"
	"github.com/brawlreport/api/internal/bsclient"
	"github.com/brawlreport/api/internal/config"
	"github.com/brawlreport/api/internal/logger"
)

func main() {
	// 1. Load configuration — fail fast on missing required vars.
	cfg, err := config.Load()
	if err != nil {
		// We don't have a logger yet so write directly to stderr.
		slog.Error("configuration error", "err", err)
		os.Exit(1)
	}

	// 2. Initialise structured logger.
	log := logger.New(cfg)
	log.Info("starting BrawlReport API",
		"port", cfg.Port,
		"token_count", len(cfg.BSAPITokens),
		"rate_per_token", cfg.BSAPIRatePerToken,
		"log_level", cfg.LogLevel,
	)

	// 3. Construct the Brawl Stars API client.
	bsClient := bsclient.NewClient(cfg, log)
	_ = bsClient // available for future route handlers

	// 4. Build the HTTP server and register routes.
	srv := api.NewServer(log)

	// 5. Start server in a goroutine so we can handle shutdown signals.
	addr := ":" + cfg.Port
	errCh := make(chan error, 1)
	go func() {
		if err := srv.Start(addr); !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	// 6. Wait for a termination signal or a fatal server error.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Info("received shutdown signal", "signal", sig.String())
	case err := <-errCh:
		log.Error("server error", "err", err)
		os.Exit(1)
	}

	// 7. Graceful shutdown with a 15-second timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("shutdown error", "err", err)
		os.Exit(1)
	}
	log.Info("server stopped cleanly")
}
