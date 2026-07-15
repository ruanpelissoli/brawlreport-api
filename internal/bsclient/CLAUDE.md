# internal/bsclient

## Purpose
Typed, rate-limited HTTP client for the official Brawl Stars API. This is the
only package that talks to `api.brawlstars.com`. Everything else (crawler,
serving API) calls through this client.

## Key Decisions

### Tag encoding
All player/club tags must be **uppercased** and `#` encoded as `%23` before
insertion into URL paths. `EncodeTag(tag)` centralises this. Never pass a raw
tag directly to a URL — tests will catch it.

### Token rotation (round-robin with per-token backoff)
`TokenPool` holds one `rate.Limiter` per API key. On 429/503 the token that
received the error is marked unavailable for a cooldown window; the next
`Acquire()` skips it and tries the next available token. This way a single
rate-limited key doesn't stall the whole fleet.

### Pointer fields for optional battle data
`Battle.Result`, `Battle.TrophyChange`, and `Battle.Rank` are `*string` / `*int`
rather than value types. This distinguishes "field absent in JSON" (`nil`) from
"field present but zero" (`0`). Without pointers a missing `trophyChange` would
decode as `0`, silently masking the absence.

### 403 IP vs. bad-token disambiguation
Supercell returns the same HTTP 403 for an invalid token and for a request
from a non-whitelisted IP. We surface `APIError.Message` (the raw Supercell
response string) so operators can tell which it is. Common strings seen from
Supercell: `"Invalid authorization"` (bad token) vs `"IP address is not..."`.

## Gotchas
- **Friendly battles have no `trophyChange`** (`nil`, not `0`). Downstream stats
  pipelines must filter by `battle.Type != "friendly"` before aggregating win
  rates or trophy deltas.
- **Battlelog shape varies by mode**: 3v3 uses `Teams [][]BattlePlayer` + `Result`;
  solo Showdown uses `Players []BattlePlayer` + `Rank`; duo Showdown uses
  `Teams` + `Rank`. Both `Teams` and `Players` may be nil simultaneously for
  some event types.
- **Backoff sleeps are real**: tests use a high rate limit (100 req/s) and a
  mock server. In production the default is 10 req/s/token; adjust via
  `BS_API_RATE_PER_TOKEN`.
- **`/events/rotation` is a bare array** (not a `{ "items": [...] }` envelope).
  `EventRotation` is typed as `[]RotationSlot` accordingly.

## Dependencies
- `golang.org/x/time/rate` — token-bucket rate limiter
- `internal/config` — for base URL, tokens, rate
- `internal/logger` — for structured log output

## What depends on this
- Future: `internal/crawler` (battlelog harvesting)
- Future: `internal/api` handlers that proxy/aggregate API responses
