# internal/api

## Purpose
HTTP server and route handlers for the BrawlReport backend. Uses only stdlib
`net/http` — no router framework dependency — keeping the binary lean and the
dependency graph minimal.

## Structure
- `server.go` — `Server` struct wrapping `http.Server` with a `ServeMux`.
  Configures read/write/idle timeouts and exposes `Start`, `Shutdown`.
- `health.go` — `GET /health` handler returning `{"status":"ok"}` with 200.
- `health_test.go` — tests both the handler directly and its mux registration.

## /health Contract
- Method: `GET`
- Path: `/health`
- Response: `200 application/json` `{"status":"ok"}`
- No auth required — used by load balancers and Kubernetes readiness probes.

## Adding New Routes
Register handlers on the `mux` inside `NewServer()`:
```go
mux.HandleFunc("GET /players/{tag}", playersHandler)
```
Go 1.22+ method+path patterns (`"GET /path"`) are used throughout.

## Key Decisions
- **Timeouts**: read=5s, write=10s, idle=60s. Prevents slowloris attacks and
  resource leaks from stalled connections.
- **No framework**: the routing needs are simple. A framework would add an
  external dependency, import path complexity, and middleware patterns that
  are overkill for a handful of endpoints.

## Gotchas
- Pattern matching uses Go 1.22 enhanced ServeMux syntax (`"METHOD /path"`).
  The minimum Go version in go.mod must remain ≥ 1.22 for this to work.
- `Server.Addr()` is a convenience method — it reads `srv.Addr` which is only
  set inside `Start()`. Do not call `Addr()` before `Start()`.
