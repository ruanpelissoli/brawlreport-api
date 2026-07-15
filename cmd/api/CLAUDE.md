# cmd/api

## Purpose
Entrypoint for the BrawlReport backend HTTP server. Wires together the config,
logger, Brawl Stars API client, and HTTP server, then blocks until a termination
signal.

## Wiring Order
1. `config.Load()` — reads env vars, fails fast on missing `BS_API_TOKENS`
2. `logger.New(cfg)` — structured JSON logger (or text in debug mode)
3. `bsclient.NewClient(cfg, log)` — rate-limited Brawl Stars API client
4. `api.NewServer(log)` — HTTP mux with `/health` and future routes registered
5. `srv.Start(":PORT")` in a goroutine
6. Block on `SIGINT` / `SIGTERM` → graceful shutdown (15s timeout)

## Key Decisions
- **Graceful shutdown**: `signal.Notify` + `srv.Shutdown(ctx)` ensures in-flight
  requests complete before the process exits. The 15-second timeout prevents
  indefinite hangs.
- **`bsClient` is constructed here** so future route handlers can receive it
  via closure or dependency injection into `api.NewServer`. Currently unused in
  routes (`_ = bsClient`) — remove the blank assignment when the first handler
  is added.
- **Fatal errors go to stderr before logger is ready**: if `config.Load()` fails,
  we call `slog.Error` (the default text handler goes to stderr) and `os.Exit(1)`.

## Gotchas
- `http.ErrServerClosed` is the expected error from `ListenAndServe` after a
  clean shutdown — it must NOT be treated as a fatal error.
- The server goroutine writes to `errCh` only for unexpected errors; the main
  goroutine drains `errCh` in the select so there's no goroutine leak.
