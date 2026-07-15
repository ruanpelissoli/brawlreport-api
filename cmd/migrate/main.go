// Command migrate bootstraps MongoDB and ClickHouse schemas for BrawlReport.
// It is idempotent: running it multiple times on a live database is safe.
//
// Usage:
//
//	go run ./cmd/migrate
//
// The command loads configuration from environment variables (with optional .env
// file support), connects to both stores with exponential-backoff retries, then
// runs the schema bootstrap for each. Exits non-zero on any error.
package main

import (
	"context"
	"errors"
	"log"
	"math"
	"os"
	"time"

	"github.com/brawlreport/api/internal/config"
	clickhousestore "github.com/brawlreport/api/internal/store/clickhouse"
	mongostore "github.com/brawlreport/api/internal/store/mongo"
)

const (
	maxRetries      = 12
	initialBackoff  = 2 * time.Second
	maxBackoff      = 30 * time.Second
)

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("migrate: starting schema bootstrap")

	ctx := context.Background()

	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Printf("migrate: config error: %v", err)
		os.Exit(1)
	}

	// Bootstrap MongoDB
	if err := bootstrapMongo(ctx, cfg); err != nil {
		log.Printf("migrate: mongo bootstrap failed: %v", err)
		os.Exit(1)
	}

	// Bootstrap ClickHouse
	if err := bootstrapClickHouse(ctx, cfg); err != nil {
		log.Printf("migrate: clickhouse bootstrap failed: %v", err)
		os.Exit(1)
	}

	log.Println("migrate: all schemas bootstrapped successfully")
}

// bootstrapMongo connects to MongoDB with retry and runs the schema bootstrap.
func bootstrapMongo(ctx context.Context, cfg *config.Config) error {
	log.Println("migrate: connecting to MongoDB...")

	client, err := withRetry(ctx, maxRetries, "MongoDB", func() error {
		c, err := mongostore.New(ctx, cfg.MongoURI)
		if err != nil {
			return err
		}
		defer func() {
			// only disconnect if we're about to retry — on success we need it
		}()

		db := c.Database(cfg.MongoDB)
		if err := mongostore.Bootstrap(ctx, db); err != nil {
			_ = c.Disconnect(ctx)
			return err
		}

		_ = c.Disconnect(ctx)
		return nil
	})

	// client is unused (the closure manages its own lifecycle)
	_ = client
	return err
}

// bootstrapClickHouse connects to ClickHouse with retry and runs the schema bootstrap.
// ClickHouse can take 5–15 s to become ready after docker-compose start, so we
// retry aggressively with exponential backoff.
func bootstrapClickHouse(ctx context.Context, cfg *config.Config) error {
	log.Println("migrate: connecting to ClickHouse...")

	_, err := withRetry(ctx, maxRetries, "ClickHouse", func() error {
		conn, err := clickhousestore.New(ctx, cfg)
		if err != nil {
			return err
		}
		defer conn.Close()

		return clickhousestore.Bootstrap(ctx, conn)
	})

	return err
}

// withRetry executes fn up to maxAttempts times, sleeping with exponential
// backoff between failures. It logs each retry attempt.
func withRetry(ctx context.Context, maxAttempts int, name string, fn func() error) (interface{}, error) {
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, errors.New("context cancelled: " + err.Error())
		}

		lastErr = fn()
		if lastErr == nil {
			return nil, nil
		}

		if attempt == maxAttempts {
			break
		}

		// Exponential backoff: 2s, 4s, 8s, ... capped at maxBackoff
		backoff := time.Duration(math.Min(
			float64(initialBackoff)*math.Pow(2, float64(attempt-1)),
			float64(maxBackoff),
		))
		log.Printf("migrate: %s attempt %d/%d failed (%v); retrying in %s...",
			name, attempt, maxAttempts, lastErr, backoff)

		select {
		case <-ctx.Done():
			return nil, errors.New("context cancelled during retry")
		case <-time.After(backoff):
		}
	}

	return nil, errors.New(name + " bootstrap failed after " +
		intToString(maxAttempts) + " attempts: " + lastErr.Error())
}

// intToString converts an int to its decimal string representation.
func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}
