# cmd/migrate

## Purpose
Standalone CLI binary that bootstraps MongoDB and ClickHouse schemas for
BrawlReport. Designed to be run once before first use and safely re-run at
any time (idempotent).

## How to run

```bash
# 1. Start local infrastructure
docker-compose up -d

# 2. Copy environment template (first time only)
cp .env.example .env

# 3. Run the migrator
go run ./cmd/migrate
```

The command exits 0 on success and non-zero on any error.

## Idempotency guarantee

All schema operations in the underlying bootstrap functions use:
- MongoDB: `CreateCollection` (ignores `NamespaceExists` error 48) +
  `Indexes().CreateMany` (no-op for existing identical indexes)
- ClickHouse: `CREATE TABLE IF NOT EXISTS` for all DDL

Re-running `cmd/migrate` against a live database with existing schema is a
safe no-op — no data is affected.

## Retry behaviour

ClickHouse takes 5–15 s to fully initialise after `docker-compose up`. The
migrate command retries connection + bootstrap up to 12 times with exponential
backoff (starting at 2 s, capped at 30 s). This means
`docker-compose up -d && go run ./cmd/migrate` works from a cold start without
manual waiting.

## Order of operations

1. Load config (`internal/config.Load`)
2. Bootstrap MongoDB (connect → ping → create collections + indexes → disconnect)
3. Bootstrap ClickHouse (connect → ping → CREATE TABLE IF NOT EXISTS × 4 → close)

MongoDB is done first because it starts faster. If ClickHouse fails after
MongoDB succeeds, re-running is safe (MongoDB bootstrap is idempotent).

## Dependencies
- `internal/config` — loads env-based config
- `internal/store/mongo` — MongoDB client + bootstrap
- `internal/store/clickhouse` — ClickHouse client + bootstrap
