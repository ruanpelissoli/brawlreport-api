# internal/store/mongo

## Purpose
MongoDB client and idempotent schema bootstrap for BrawlReport's operational
data layer. Handles player profiles, crawl frontier state, raw API response
caching, and refreshable reference metadata (brawlers, maps).

## Collections

| Collection | Role |
|---|---|
| `players` | Full player profiles as returned by the Brawl Stars API, merged/upserted on each crawl |
| `crawl_frontier` | Work queue for the crawler — tracks crawl status, priority, and next-crawl scheduling |
| `api_cache` | Raw API response bodies with TTL expiry — avoids redundant upstream calls |
| `brawlers` | Brawler roster from `GET /brawlers` — refreshed by the metadata job |
| `maps` | Event/map metadata upserted as events are observed in battle logs |

## Index rationale

**`crawl_frontier` compound `{ priority: -1, next_crawl_at: 1 }`**  
The crawler's "pick next item" query is:
```
{ status: "pending", next_crawl_at: { $lte: now } }
sort: { priority: -1, next_crawl_at: 1 }
```
The compound index supports both the sort and covers the next_crawl_at range
filter without a separate index scan.

**`api_cache` TTL index on `expires_at`**  
MongoDB's background TTL thread removes expired documents automatically
(~60 s granularity). Callers should still check `expires_at` if they need
sub-minute freshness guarantees.

## Idempotency
`Bootstrap` uses `ensureCollection` (ignores NamespaceExists error code 48) and
`CreateMany` (indexes are no-ops if a matching index already exists). Re-running
Bootstrap on a live database is safe.

## Gotchas
- MongoDB TTL background thread runs every ~60 s; expired cache entries may linger.
- `api_cache.body` stores raw JSON strings (not parsed BSON) to allow zero-
  transformation caching. Validate the cached HTTP status before using the body.
- Brawler count grows with each Supercell update. The `brawlers` collection is
  designed to be fully replaced by the metadata refresher (delete-all + bulk insert).
