# internal/config

## Purpose
Loads and validates all environment-based configuration for the service.
Returns a typed `Config` struct; callers receive a clear error and the process
exits early if required variables are absent.

## Environment Variables

| Variable | Required | Default | Notes |
|---|---|---|---|
| `BS_API_TOKENS` | **yes** | — | Comma-separated list of Brawl Stars API bearer tokens. At least one required. Tokens are rotated round-robin by the bsclient. |
| `BS_API_BASE_URL` | no | `https://api.brawlstars.com/v1` | Override for testing against a mock server. |
| `BS_API_RATE_PER_TOKEN` | no | `10.0` | Max requests-per-second per API token. |
| `LOG_LEVEL` | no | `info` | Accepted: `debug`, `info`, `warn`, `error`. |
| `PORT` | no | `8080` | HTTP listen port. |

## Key Decisions
- **No external config library** — pure `os.Getenv`. Avoids a dependency for
  something stdlib handles fine; keeps the binary lean.
- **Fail-fast on missing tokens** — a missing `BS_API_TOKENS` must be an
  immediate startup error, not a silent zero-token slice that causes obscure
  panics later.
- **Whitespace-tolerant token parsing** — `strings.TrimSpace` on each element
  so `token1, token2` (with spaces) works correctly.

## Gotchas
- `BS_API_TOKENS` may contain blank elements after splitting (e.g. trailing
  comma). The loader skips empty strings so the token count is reliable.
- `BS_API_RATE_PER_TOKEN` must be a **positive** float; zero or negative values
  are rejected to prevent divide-by-zero in the rate limiter.
