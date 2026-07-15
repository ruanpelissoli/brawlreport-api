# internal/store/clickhouse

## Purpose
ClickHouse client and idempotent schema bootstrap for BrawlReport's analytical
data layer. Houses canonical battle records and pre-computed aggregate tables
that power the API's win-rate, matchup-matrix, and trend endpoints.

## Dedup key specification

The `battles` table uses `dedup_hash` as its primary dedup signal. The hash is
computed in the Go ingest layer **before** insertion using:

```
dedup_hash = hex( SHA256(
    battleTime_UTC_RFC3339          // e.g. "2026-01-15T14:30:00Z"
    + "|"
    + sorted_participant_tags_csv   // all tags from both teams, sorted, comma-joined
    + "|"
    + event_id_string               // decimal string
    + "|"
    + map_name                      // exact string from the API
))
```

**Why this set of fields?**
The same battle appears in every participant's battlelog. The combination of
time + all participants + event/map is globally unique per real match. Sorting
tags makes the hash order-independent (team A sees the same tags as team B).

## Immutability contract

The `battles` table is **append-only**. Never UPDATE or DELETE rows. The Brawl
Stars battlelog is a historical record; mutations would corrupt analytics.

If a wrong row is ingested (e.g. a schema bug), the remedy is:
1. Drop the affected partition (`ALTER TABLE battles DROP PARTITION 'YYYYMM'`).
2. Re-ingest from the raw source.

## SELECT FINAL guidance

`ReplacingMergeTree` deduplicates by `ORDER BY` key **eventually** (on
background merge). Until a merge runs, duplicate rows with the same `dedup_hash`
may exist. Two strategies:

- **`SELECT ... FINAL`** — forces ClickHouse to deduplicate at query time.
  Slower (reads more data) but returns exact results. Use for small result sets
  or correctness-critical reads.
- **Accept eventual duplicates** — for large aggregations (e.g. `COUNT(*)` over
  a month) the variance from un-merged rows is tiny and the query is much faster.
  Document the trade-off in the calling code.

## TTL enforcement

ClickHouse enforces TTL during background merges and explicit `OPTIMIZE TABLE`
calls. Rows are not deleted the instant they expire — the TTL acts as a
partition-level guarantee over a slightly longer horizon. The 12-month TTL is
the cost-control mechanism for the 12-month data-retention requirement.

## Aggregate tables

| Table | Populated by | Used by |
|---|---|---|
| `brawler_win_rates` | Aggregation pipeline | Brawler pages, meta dashboard |
| `matchup_matrix` | Aggregation pipeline | Counter-pick heatmap |
| `daily_snapshots` | Snapshot job (once/day) | Trend lines (3–6 mo) |

All aggregate tables use `ReplacingMergeTree(updated_at)` so the pipeline can
upsert updated stats by re-inserting with a newer `updated_at`.

## Gotchas
- team1/team2 brawler id arrays are sorted ascending before insertion — this
  makes matchup queries symmetric and avoids double-counting.
- The 100+ battle sample gate is enforced at **read time** by the API layer
  (`WHERE battles >= 100`), not by the schema.
- `battle_type` must never mix `friendly` battles into competitive aggregates —
  the aggregation pipeline filters by `battle_type != 'friendly'`.
