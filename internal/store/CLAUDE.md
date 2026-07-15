# internal/store

## Purpose
Top-level store package grouping both data store implementations for BrawlReport.
The two sub-packages (`mongo` and `clickhouse`) correspond to the two workloads
identified in the Architecture Decision doc.

## Two-store strategy

| Store | Package | Workload |
|---|---|---|
| MongoDB | `store/mongo` | Operational / document data — player profiles, crawl frontier, API cache, metadata |
| ClickHouse | `store/clickhouse` | Analytical — canonical battle records, aggregate/matchup tables |

**Decision rule:** if the access pattern is a point-lookup, a document upsert,
or involves semi-structured schema-evolving data → MongoDB. If it involves
range scans, aggregations, or time-series over battle data → ClickHouse.

## Connection lifecycle

Both stores expose a `New(ctx, ...)` constructor that:
1. Opens the connection with a timeout.
2. Pings to verify connectivity before returning.
3. Returns a client/connection the caller is responsible for closing.

Pattern for `cmd/migrate` and future server entrypoints:
```go
mongoClient, err := mongo.New(ctx, cfg.MongoURI)
// handle err
defer mongoClient.Disconnect(context.Background())

chConn, err := clickhouse.New(ctx, cfg)
// handle err
defer chConn.Close()
```

## Schema bootstrap
Both packages expose an idempotent `Bootstrap(ctx, ...)` function. Call it once
on startup (or via `cmd/migrate`) to ensure all collections/tables/indexes exist.
Re-running on a live store with existing schema is safe — all DDL is no-op if
already present.

## Dependencies
- `store/mongo` depends on: `internal/config`, `go.mongodb.org/mongo-driver/v2`
- `store/clickhouse` depends on: `internal/config`, `github.com/ClickHouse/clickhouse-go/v2`
- Used by: `cmd/migrate`, future crawler and API server packages
