package mongo

import (
	"context"
	"fmt"
	"log"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Bootstrap idempotently creates all BrawlReport MongoDB collections and indexes.
// It is safe to call multiple times — existing collections and indexes are left unchanged.
func Bootstrap(ctx context.Context, db *mongo.Database) error {
	steps := []struct {
		name string
		fn   func(context.Context, *mongo.Database) error
	}{
		{"players", bootstrapPlayers},
		{"crawl_frontier", bootstrapCrawlFrontier},
		{"api_cache", bootstrapAPICache},
		{"brawlers", bootstrapBrawlers},
		{"maps", bootstrapMaps},
	}

	for _, step := range steps {
		log.Printf("mongo bootstrap: ensuring collection %q...", step.name)
		if err := step.fn(ctx, db); err != nil {
			return fmt.Errorf("mongo bootstrap %s: %w", step.name, err)
		}
		log.Printf("mongo bootstrap: collection %q OK", step.name)
	}

	return nil
}

// ensureCollection creates the named collection if it does not already exist.
// MongoDB returns a NamespaceExists error when the collection is already present,
// which we treat as a no-op (idempotent).
func ensureCollection(ctx context.Context, db *mongo.Database, name string) error {
	err := db.CreateCollection(ctx, name)
	if err != nil {
		// NamespaceExists code 48 — collection already exists, that's fine
		if isNamespaceExistsError(err) {
			return nil
		}
		return err
	}
	return nil
}

// isNamespaceExistsError reports whether err is a MongoDB "namespace already exists" error.
func isNamespaceExistsError(err error) bool {
	if cmdErr, ok := err.(mongo.CommandError); ok {
		return cmdErr.Code == 48
	}
	return false
}

// bootstrapPlayers creates the players collection.
//
// Document shape (informational — MongoDB is schemaless):
//
//	{
//	  tag:        string  (e.g. "#ABC123") — Brawl Stars player tag, unique
//	  name:       string
//	  trophies:   int32
//	  ...         (full API profile JSON merged here)
//	  updated_at: datetime
//	}
//
// Indexes:
//   - Unique on tag (primary lookup key)
func bootstrapPlayers(ctx context.Context, db *mongo.Database) error {
	if err := ensureCollection(ctx, db, "players"); err != nil {
		return err
	}

	coll := db.Collection("players")
	_, err := coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "tag", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("tag_unique"),
		},
	})
	return err
}

// bootstrapCrawlFrontier creates the crawl_frontier collection.
//
// The frontier drives the crawler: it tracks which player tags still need to be
// fetched, when they were last crawled, and their priority.
//
// Document shape:
//
//	{
//	  tag:           string   — player tag
//	  status:        string   — "pending" | "in_progress" | "done" | "error"
//	  priority:      int32    — higher = crawl sooner (e.g. boosted for ranked/active players)
//	  next_crawl_at: datetime — earliest time at which this tag should be re-crawled
//	  last_crawled:  datetime
//	  error_count:   int32
//	}
//
// Indexes:
//   - Unique on tag
//   - Compound { priority: -1, next_crawl_at: 1 } — supports the "pick next work item" query
//   - { status: 1 } — supports filtering by status
func bootstrapCrawlFrontier(ctx context.Context, db *mongo.Database) error {
	if err := ensureCollection(ctx, db, "crawl_frontier"); err != nil {
		return err
	}

	coll := db.Collection("crawl_frontier")
	_, err := coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "tag", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("tag_unique"),
		},
		{
			Keys: bson.D{
				{Key: "priority", Value: -1},
				{Key: "next_crawl_at", Value: 1},
			},
			Options: options.Index().SetName("priority_next_crawl"),
		},
		{
			Keys:    bson.D{{Key: "status", Value: 1}},
			Options: options.Index().SetName("status"),
		},
	})
	return err
}

// bootstrapAPICache creates the api_cache collection.
//
// Stores raw API responses to avoid hammering the upstream API.
// Documents expire automatically via a MongoDB TTL index on expires_at.
// Note: the TTL background thread runs every ~60 s, so briefly expired docs
// may still be returned — callers should check expires_at if strict freshness matters.
//
// Document shape:
//
//	{
//	  cache_key:  string   — e.g. "battlelog:#ABC123"
//	  body:       string   — raw JSON response body
//	  status:     int32    — HTTP status code
//	  fetched_at: datetime
//	  expires_at: datetime — TTL index causes MongoDB to delete after this time
//	}
//
// Indexes:
//   - Unique on cache_key
//   - TTL on expires_at
func bootstrapAPICache(ctx context.Context, db *mongo.Database) error {
	if err := ensureCollection(ctx, db, "api_cache"); err != nil {
		return err
	}

	coll := db.Collection("api_cache")
	ttlSeconds := int32(0) // expire at the expires_at time itself
	_, err := coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "cache_key", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("cache_key_unique"),
		},
		{
			Keys:    bson.D{{Key: "expires_at", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(ttlSeconds).SetName("expires_at_ttl"),
		},
	})
	return err
}

// bootstrapBrawlers creates the brawlers collection.
//
// Stores the canonical brawler roster from GET /brawlers.
// This collection is fully replaced each time the metadata refresher runs.
//
// Document shape mirrors the Brawl Stars API /brawlers response:
//
//	{
//	  id:    int32   — Brawl Stars brawler id (unique)
//	  name:  string
//	  ...   (starPowers, gadgets, gears arrays as returned by the API)
//	}
//
// Indexes:
//   - Unique on id (primary lookup; roster grows over time)
func bootstrapBrawlers(ctx context.Context, db *mongo.Database) error {
	if err := ensureCollection(ctx, db, "brawlers"); err != nil {
		return err
	}

	coll := db.Collection("brawlers")
	_, err := coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "id", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("id_unique"),
		},
	})
	return err
}

// bootstrapMaps creates the maps collection.
//
// Stores event/map metadata observed from crawled battle logs and the events rotation.
// Maps are upserted by the crawler as new events are encountered.
//
// Document shape:
//
//	{
//	  id:   int32  — Brawl Stars event id (unique)
//	  name: string — map name
//	  mode: string — e.g. "gemGrab", "brawlBall"
//	}
//
// Indexes:
//   - Unique on id
//   - { mode: 1 } — supports filtering maps by game mode
func bootstrapMaps(ctx context.Context, db *mongo.Database) error {
	if err := ensureCollection(ctx, db, "maps"); err != nil {
		return err
	}

	coll := db.Collection("maps")
	_, err := coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "id", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("id_unique"),
		},
		{
			Keys:    bson.D{{Key: "mode", Value: 1}},
			Options: options.Index().SetName("mode"),
		},
	})
	return err
}
