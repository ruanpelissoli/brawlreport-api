package clickhouse

import (
	"context"
	"fmt"
	"log"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Bootstrap idempotently creates all BrawlReport ClickHouse tables.
// All DDL uses CREATE TABLE IF NOT EXISTS — safe to call multiple times.
func Bootstrap(ctx context.Context, conn driver.Conn) error {
	tables := []struct {
		name string
		ddl  string
	}{
		{"battles", ddlBattles},
		{"brawler_win_rates", ddlBrawlerWinRates},
		{"matchup_matrix", ddlMatchupMatrix},
		{"daily_snapshots", ddlDailySnapshots},
	}

	for _, t := range tables {
		log.Printf("clickhouse bootstrap: ensuring table %q...", t.name)
		if err := conn.Exec(ctx, t.ddl); err != nil {
			return fmt.Errorf("clickhouse bootstrap %s: %w", t.name, err)
		}
		log.Printf("clickhouse bootstrap: table %q OK", t.name)
	}

	return nil
}

// ddlBattles defines the canonical, append-only battles table.
//
// Design notes:
//   - ENGINE = ReplacingMergeTree() deduplicates by ORDER BY key during background
//     merges. This is an eventual-consistency safety net; queries needing exact
//     dedup must use SELECT ... FINAL.
//   - dedup_hash is computed by the Go ingest layer BEFORE insertion (see CLAUDE.md
//     for the exact algorithm). It is the primary dedup signal.
//   - Partitioned by YYYYMM so old partitions can be dropped wholesale by the TTL
//     engine; 12-month TTL is enforced at the partition level.
//   - team1/team2 brawler arrays are sorted ascending so matchup queries can do
//     set operations without caring about team ordering within the array.
//   - participant_tags carries ALL player tags (both teams) sorted; used by the
//     crawler to expand the crawl frontier without a separate join.
const ddlBattles = `
CREATE TABLE IF NOT EXISTS battles (
    battle_time       DateTime,
    event_id          UInt32,
    event_mode        LowCardinality(String),
    map_name          LowCardinality(String),
    battle_type       LowCardinality(String),
    battle_result     LowCardinality(String),
    duration_secs     UInt16,
    trophy_change     Int16,
    team1_brawler_ids Array(UInt32),
    team2_brawler_ids Array(UInt32),
    participant_tags  Array(String),
    dedup_hash        String,
    ingested_at       DateTime DEFAULT now()
)
ENGINE = ReplacingMergeTree()
PARTITION BY toYYYYMM(battle_time)
ORDER BY (toDate(battle_time), dedup_hash)
TTL battle_time + INTERVAL 12 MONTH
SETTINGS index_granularity = 8192
`

// ddlBrawlerWinRates defines the brawler win/pick rate aggregate table.
// Populated by the aggregation pipeline; queried by the API for brawler pages.
// Using ReplacingMergeTree(updated_at) so re-runs of the aggregation pipeline
// replace stale rows with fresh ones (eventual dedup on updated_at).
const ddlBrawlerWinRates = `
CREATE TABLE IF NOT EXISTS brawler_win_rates (
    period_date  Date,
    brawler_id   UInt32,
    map_name     LowCardinality(String),
    event_mode   LowCardinality(String),
    battle_type  LowCardinality(String),
    wins         UInt64,
    battles      UInt64,
    picks        UInt64,
    updated_at   DateTime DEFAULT now()
)
ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY toYYYYMM(period_date)
ORDER BY (period_date, brawler_id, map_name, event_mode, battle_type)
TTL period_date + INTERVAL 12 MONTH
`

// ddlMatchupMatrix defines the brawler-vs-brawler matchup table.
// Rows represent "brawler_id win rate when facing enemy_brawler_id on map_name".
// The aggregation pipeline writes one row per (brawler, enemy, map, mode, period).
// The 100+ battle sample gate is enforced at read time by the API, not here.
const ddlMatchupMatrix = `
CREATE TABLE IF NOT EXISTS matchup_matrix (
    period_date      Date,
    brawler_id       UInt32,
    enemy_brawler_id UInt32,
    map_name         LowCardinality(String),
    event_mode       LowCardinality(String),
    wins             UInt64,
    battles          UInt64,
    updated_at       DateTime DEFAULT now()
)
ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY toYYYYMM(period_date)
ORDER BY (period_date, brawler_id, enemy_brawler_id, map_name, event_mode)
TTL period_date + INTERVAL 12 MONTH
`

// ddlDailySnapshots defines the daily global brawler metrics table.
// One row per (snapshot_date, brawler_id). Powers 3–6 month trend lines on the
// meta dashboard and brawler pages. Written once per day by the snapshot job.
const ddlDailySnapshots = `
CREATE TABLE IF NOT EXISTS daily_snapshots (
    snapshot_date Date,
    brawler_id    UInt32,
    win_rate      Float32,
    pick_rate     Float32,
    battle_count  UInt64,
    updated_at    DateTime DEFAULT now()
)
ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY toYYYYMM(snapshot_date)
ORDER BY (snapshot_date, brawler_id)
TTL snapshot_date + INTERVAL 12 MONTH
`
