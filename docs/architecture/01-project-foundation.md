# Project foundation

This document describes the desired architecture for CLI, configuration, router, observability, and startup/shutdown. Build order and validation steps are in [roadmap.md](../roadmap.md).

---

## Design decisions

| Question | Decision |
|----------|----------|
| CLI sub-commands | **cobra** |
| Admin portal frontend | **HTMX + Go templates + Pico.css** (no build step; embedded via `go:embed`) |
| CORS policy | **Wildcard `*`** (public Mastodon-compatible API) |
| Config error handling | **`Load()` returns error** — collects all missing vars, logs them, exits 1 |


---

## Startup Sequence

`runServe` in `cmd/monstera-fed/serve.go` executes these steps in order:

1. **Load config** — `config.Load()`. Log all errors and `os.Exit(1)` if any.
2. **Init logger** — `observability.NewLogger(cfg.AppEnv, cfg.LogLevel)`. All subsequent steps log through this logger.
3. **Init metrics** — `observability.NewMetrics(prometheus.NewRegistry())`.
4. **Open DB pool** — `pgxpool.New(ctx, cfg.DatabaseURL)` with `MaxConns` and `MinConns` from config. Ping to confirm connectivity. Exit 1 on failure.
5. **Run migrations** — `golang-migrate` applies any pending `.sql` files from `internal/store/migrations/`. Exit 1 if migrations fail (prevents a partially-migrated pod from starting).
6. **Connect to NATS** — `nats.Connect(cfg.NATSUrl, opts...)`. Apply `cfg.NATSCredsFile` if set. Exit 1 on failure.
7. **Build cache store** — switch on `cfg.CacheDriver`: instantiate `cache/memory` or `cache/redis`.
8. **Build media store** — switch on `cfg.MediaDriver`: instantiate `media/local` or `media/s3`.
9. **Build email sender** — switch on `cfg.EmailDriver`: instantiate `email/noop` or `email/smtp`.
10. **Build services** — construct all `service.*` structs via constructor injection, passing the above dependencies.
11. **Build health checker** — `api.NewHealthChecker(dbPool, natsConn)`.
12. **Build router** — Router assembles chi with the full middleware stack and registers all routes.
13. **Start HTTP server** — `http.Server{Addr: ":PORT", Handler: router}`. Call `ListenAndServe` in a goroutine.
14. **Log ready** — structured `slog.Info("server started", "port", cfg.AppPort, "env", cfg.AppEnv)`.
15. **Block on signal** — `signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)`. When the context is cancelled, proceed to shutdown.

---

## Shutdown Sequence


Ordered to prevent data loss. Each step has a timeout budget drawn from the 30-second global shutdown deadline.

1. **Cancel the signal context** — triggers the shutdown path in `runServe`.
2. **HTTP drain** — `server.Shutdown(shutdownCtx)` with 30s deadline. Stops accepting new connections; waits for in-flight request handlers to return. SSE long-poll handlers are signalled to close.
3. **Stop federation workers** — send cancellation to the federation worker goroutine pool. Wait for any in-flight `POST` delivery attempts to finish or time out.
4. **Close SSE hub** — close all active per-account event channels. Clients receive the stream EOF and will reconnect.
5. **Drain NATS** — `nc.Drain()`. Flushes any pending publishes; waits for active subscriptions to finish processing. Safer than `nc.Close()` alone.
6. **Close DB pool** — `dbPool.Close()`. All service-layer DB operations have stopped by this point.
7. **Log shutdown complete** — `slog.Info("shutdown complete", "elapsed_ms", ...)`.
8. **Exit 0**.

**Why this order matters:**
- HTTP must drain before services stop, or in-flight requests will read from closed dependencies.
- Federation workers must stop before NATS drains, or they may try to publish to a drained connection.
- NATS drains before DB closes, because federation workers and SSE may write to DB on receipt of a NATS message.
- DB closes last — it is the final source of truth and may still be written to by any pending service operation.

---

## Router middleware stack

Order matters:

```
chi.NewRouter()
│
├── middleware.RequestID          — assigns X-Request-Id header + context value
├── middleware.RealIP             — trusts X-Real-IP / X-Forwarded-For
├── observability.RequestLogger   — logs after response; uses request_id + account_id from context
├── observability.MetricsMiddleware — records HTTP counters/histograms; reads chi route pattern
├── middleware.Recoverer          — catches panics; logs stack trace; writes generic 500
├── cors.Handler(cors.Options{AllowedOrigins: []string{"*"}})
└── middleware.Timeout(30 * time.Second)
```

