# internal/config

## Purpose
Centralised configuration loading for the BrawlReport API. Reads environment
variables at startup, validates that required values are present, and returns
a typed `Config` struct. Callers receive a clear error and the process exits
early if required variables are absent.

## Environment Variables

| Variable | Required | Default | Notes |
|---|---|---|---|
| `BS_API_TOKENS` | **yes** | — | Comma-separated list of Brawl Stars API bearer tokens. At least one required. Tokens are rotated round-robin by the bsclient. |
| `BS_API_BASE_URL` | no | `https://api.brawlstars.com/v1` | Override for testing against a mock server. |
| `BS_API_RATE_PER_TOKEN` | no | `10.0` | Max requests-per-second per API token. |
| `MONGO_URI` | **yes** | — | Full MongoDB URI, e.g. `mongodb://localhost:27017` |
| `MONGO_DB` | no | `brawlreport` | Database name |
| `CLICKHOUSE_ADDR` | **yes** | — | `host:port`, native TCP port 9000 |
| `CLICKHOUSE_DB` | no | `brawlreport` | Database/schema name |
| `CLICKHOUSE_USER` | **yes** | — | ClickHouse username |
| `CLICKHOUSE_PASSWORD` | no | `""` | Empty is valid for passwordless setups |
| `LOG_LEVEL` | no | `info` | Accepted: `debug`, `info`, `warn`, `error`. |
| `PORT` | no | `8080` | HTTP listen port. |

## Key Decisions
- **godotenv soft-load**: `godotenv.Load()` is called with its return value
  discarded. A missing `.env` file is never an error — the right behaviour for
  production containers where env vars come from the platform. A present
  `.env` wins (local dev). Never fail on a missing `.env`.
- **Single `Config` struct**: All downstream packages receive `*Config` rather
  than reading `os.Getenv` themselves. This keeps env-var coupling in one place
  and makes testing straightforward.
- **Fail-fast on missing required vars** — missing `BS_API_TOKENS` or data-store
  variables must be an immediate startup error, not a silent zero value that
  causes obscure failures later.
- **No defaults for secrets**: `MONGO_URI`, `CLICKHOUSE_ADDR`, and
  `CLICKHOUSE_USER` have empty defaults and fail validation if unset. Database
  name and password have safe defaults so local dev works with minimal setup
  (matching the `docker-compose.yml` values).
- **Whitespace-tolerant token parsing** — `strings.TrimSpace` on each element
  so `token1, token2` (with spaces) works correctly.

## Local dev vs. prod
Copy `.env.example` to `.env` (git-ignored) and fill in values. The docker-
compose defaults match the `.env.example` defaults, so `docker-compose up` +
`go run ./cmd/migrate` works out of the box.

## Gotchas
- `BS_API_TOKENS` may contain blank elements after splitting (e.g. trailing
  comma). The loader skips empty strings so the token count is reliable.
- `BS_API_RATE_PER_TOKEN` must be a **positive** float; zero or negative values
  are rejected to prevent divide-by-zero in the rate limiter.
- Tests that call `Load()` must set the required store variables too (see
  `setStoreEnv` in `config_test.go`).

## Dependencies
- `github.com/joho/godotenv` — env file loader; only used here.
- Used by: `cmd/api`, `cmd/migrate`, any package needing config.
