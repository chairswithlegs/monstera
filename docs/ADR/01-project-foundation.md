# ADR 01 — Project Foundation

> **Status:** Draft v0.1 — Feb 24, 2026  
> **Addresses:** `design-prompts/01-project-foundation.md`

> **Implementation status (Feb 2026):** Several sections below are **out of date** relative to the current codebase. Out-of-date sections are annotated with **"Out of date:"** notes. The implementation has diverged in package layout (CLI under `internal/cli/`, router under `api/router/`), handler-centric router deps, auth middleware signatures, error-handling API (no logger arg; `WriteJSON` exported in `response.go`), health handler names, config extras, and missing routes (`/metrics`, `/admin`). Shutdown uses a 15s timeout and does not yet implement the full sequence (federation workers, SSE hub). `web/admin/` does not exist yet.

---

## Design Decisions (answered before authoring)

| Question | Decision |
|----------|----------|
| CLI sub-commands | **cobra** |
| Admin portal frontend | **HTMX + Go templates + Pico.css** (no build step; embedded via `go:embed`) — revised in ADR 10 |
| CORS policy | **Wildcard `*`** (public Mastodon-compatible API) |
| Config error handling | **`Load()` returns error** — collects all missing vars, logs them, exits 1 |

---

## 1. Package Layout

Every file to create, with a one-line responsibility statement.

> **Out of date:** Router lives in `internal/api/router/router.go` (package `router`), not `internal/api/router.go`. CLI lives in `cmd/monstera-fed/internal/cli/` (`serve.go`, `migrate.go`, `root.go`), not directly under `cmd/monstera-fed/`. `writeJSON` is implemented as exported `WriteJSON` in `internal/api/response.go` (with `WriteJRD`, `WriteActivityJSON`); `ERROR_HANDLING.md` is removed. `web/admin/` does not exist in the repo yet.

### `internal/config/`

| File | Responsibility |
|------|----------------|
| `config.go` | `Config` struct, `Load()`, `Validate()`, and typed env-var helpers |

### `internal/observability/`

| File | Responsibility |
|------|----------------|
| `logger.go` | `NewLogger()` factory, `RequestLogger` chi middleware, context key for `request_id` / `account_id` |
| `metrics.go` | `Metrics` struct with all Prometheus descriptors, `NewMetrics()`, `MetricsMiddleware` chi middleware |

### `internal/api/`

| File | Responsibility |
|------|----------------|
| `router.go` | `NewRouter()` — assembles chi router, registers all middleware and route groups |
| `errors.go` | `HandleError` mapper, `ErrorResponse` type, `writeJSON` helper, panic-recovery conventions (see `ERROR_HANDLING.md`) |
| `health.go` | `HealthChecker` struct, `Liveness` and `Readiness` handlers |
| `middleware/auth.go` | `RequireAuth`, `OptionalAuth`, `RequireAdmin` middleware and context helpers |

### `cmd/monstera-fed/`

> **Out of date:** In code, `main.go` only calls `cli.Execute()`; root and sub-commands are in `internal/cli/` (`root.go`, `serve.go`, `migrate.go`). There is no `buildRootCmd()` in main. Migrate also has `down-all` in addition to `up`/`down`.

| File | Responsibility |
|------|----------------|
| `main.go` | Cobra root command construction and `Execute()` call — nothing else |
| `serve.go` | `serve` sub-command: full startup wiring and graceful shutdown |
| `migrate.go` | `migrate up` / `migrate down` sub-commands via golang-migrate |

### `web/admin/`

> **Out of date:** `web/admin/` (templates + static) is not present in the repository; admin portal not yet implemented.

| Path | Responsibility |
|------|----------------|
| `web/admin/templates/` | Go `html/template` files (layouts, partials, pages) |
| `web/admin/static/` | Vendored static assets (htmx.min.js, Pico.css, custom CSS); embedded via `//go:embed` |

---

## 2. Go Signatures

### `internal/config/config.go`

> **Out of date:** Implemented config adds `FederationWorkerConcurrency` (int), `Version` (string), and helpers `SecretKeyBytes()` and `DeriveKey(purpose, length)` not in this ADR.

```go
package config

// Config holds all runtime configuration. Populated once at startup; treated as read-only
// after Load() returns. Passed by pointer through constructor injection — never a global.
type Config struct {
    // Core
    AppEnv         string // "development" | "production"
    AppPort        int    // default: 8080
    InstanceDomain string // required
    InstanceName   string // default: "Monstera-fed"
    LogLevel       string // "debug"|"info"|"warn"|"error", default: "info"

    // Database
    DatabaseURL          string // required
    DatabaseMaxOpenConns int    // default: 20
    DatabaseMaxIdleConns int    // default: 5

    // NATS
    NATSUrl       string // required
    NATSCredsFile string // optional

    // Cache
    CacheDriver   string // "memory"|"redis", default: "memory"
    CacheRedisURL string // required when CacheDriver == "redis"

    // Media
    MediaDriver     string // "local"|"s3", default: "local"
    MediaLocalPath  string // required when MediaDriver == "local"
    MediaBaseURL    string // required
    MediaS3Bucket   string // required when MediaDriver == "s3"
    MediaS3Region   string // required when MediaDriver == "s3"
    MediaS3Endpoint string // optional: override endpoint for MinIO/R2/B2
    MediaCDNBase    string // optional: CDN prefix for S3 URLs

    // Email
    EmailDriver       string // "noop"|"smtp", default: "noop"
    EmailFrom         string // required
    EmailFromName     string // default: "Monstera-fed"
    EmailSMTPHost     string // required when EmailDriver == "smtp"
    EmailSMTPPort     int    // default: 587
    EmailSMTPUsername string
    EmailSMTPPassword string

    // Security
    SecretKeyBase string // required; min 32 bytes of entropy
    MetricsToken  string // optional: if set, /metrics requires "Bearer <MetricsToken>"

    // Feature flags
    FederationEnabled bool  // default: true
    MaxStatusChars    int   // default: 500
    MediaMaxBytes     int64 // default: 10485760 (10 MB)
}

// Load reads all environment variables and populates a Config.
// It collects every validation error before returning — callers see all problems at once.
// Returns a non-nil *Config on success; a non-nil error on any problem.
func Load() (*Config, error)

// Validate checks cross-field constraints that cannot be expressed as individual field rules.
// Called automatically by Load(); exported for use in tests.
func (c *Config) Validate() error

// IsDevelopment is a convenience predicate used by the logger and other packages.
func (c *Config) IsDevelopment() bool
```

**Helper functions (unexported):**

```go
// envString returns the env var value, or defaultVal if unset.
func envString(key, defaultVal string) string

// envStringRequired returns the env var value, or appends an error to errs if unset.
func envStringRequired(key string, errs *[]string) string

// envInt parses an integer env var with a fallback default.
func envInt(key string, defaultVal int) int

// envInt64 parses an int64 env var with a fallback default.
func envInt64(key string, defaultVal int64) int64

// envBool parses a boolean env var ("true"/"false"/"1"/"0") with a fallback default.
func envBool(key string, defaultVal bool) bool
```

---

### `internal/observability/logger.go`

> **Out of date:** Request ID is a custom hex format (16 random bytes, hex-encoded with dashes), not UUID v4. The router also uses `chimw.RequestID` before `RequestLogger`; the effective ID in context/header comes from `RequestLogger`.

```go
package observability

// contextKey is an unexported type for context keys owned by this package.
type contextKey int

const (
    requestIDKey contextKey = iota
    accountIDKey
)

// NewLogger builds a *slog.Logger appropriate for the environment:
//   - development: slog.NewTextHandler (human-readable, colorised output to stderr)
//   - production:  slog.NewJSONHandler (machine-readable JSON to stderr)
//
// level must be one of "debug", "info", "warn", "error" (case-insensitive).
// An unrecognised level defaults to "info".
func NewLogger(env, level string) *slog.Logger

// RequestLogger returns a chi middleware that:
//  1. Generates a UUID v4 request_id and stores it in the context.
//  2. Wraps the ResponseWriter to capture the status code.
//  3. After the downstream handler completes, logs a structured entry with:
//     method, path, status, duration_ms, request_id, account_id (if present in context).
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler

// WithRequestID stores a request ID in the context.
func WithRequestID(ctx context.Context, id string) context.Context

// RequestIDFromContext retrieves the request ID from the context ("" if absent).
func RequestIDFromContext(ctx context.Context) string

// WithAccountID stores the authenticated account's ID in the context.
func WithAccountID(ctx context.Context, id string) context.Context

// AccountIDFromContext retrieves the account ID from the context ("" if absent).
func AccountIDFromContext(ctx context.Context) string
```

---

### `internal/observability/metrics.go`

```go
package observability

// Metrics holds every Prometheus collector used across the application.
// A single *Metrics is created at startup and injected wherever instrumentation is needed.
type Metrics struct {
    // HTTP layer
    HTTPRequestsTotal          *prometheus.CounterVec   // labels: method, path, status
    HTTPRequestDurationSeconds *prometheus.HistogramVec // labels: method, path

    // Federation worker
    FederationDeliveriesTotal          *prometheus.CounterVec // labels: result (success/failure/rejected)
    FederationDeliveryDurationSeconds  prometheus.Histogram   // no labels

    // SSE streaming
    ActiveSSEConnections *prometheus.GaugeVec // labels: stream

    // NATS publishing
    NATSPublishTotal *prometheus.CounterVec // labels: subject, result (ok/error)

    // Database
    DBQueryDurationSeconds *prometheus.HistogramVec // labels: query_name

    // Media
    MediaUploadBytesTotal *prometheus.CounterVec // labels: driver (local/s3)

    // Accounts
    AccountsTotal *prometheus.GaugeVec // labels: type (local/remote)
}

// NewMetrics registers all collectors against reg and returns the populated struct.
// Panics if registration fails (programming error, not a runtime error).
func NewMetrics(reg prometheus.Registerer) *Metrics

// MetricsMiddleware returns a chi middleware that records monstera-fed_http_requests_total
// and monstera-fed_http_request_duration_seconds.
//
// Path cardinality: chi.RouteContext(r.Context()).RoutePattern() is used to obtain the
// route template (e.g. "/api/v1/accounts/{id}") rather than the raw URL path, preventing
// a unique label value per account ID.
func MetricsMiddleware(m *Metrics) func(http.Handler) http.Handler
```

**Path cardinality note:** `chi.RouteContext(r.Context()).RoutePattern()` is populated *after* routing, so the middleware must record the pattern in a deferred call (or read it from the route context after `next.ServeHTTP` returns). This is the idiomatic chi approach and avoids any regex scrubbing.

---

### `internal/api/health.go`

> **Out of date:** Fields are exported (`DB`, `NATS`). Handler methods are named `GETLiveness` and `GETReadiness` (not `Liveness` / `Readiness`).

```go
package api

// HealthChecker holds the dependencies needed to execute readiness checks.
type HealthChecker struct {
    db   *pgxpool.Pool
    nats *nats.Conn
}

// NewHealthChecker constructs a HealthChecker. Both arguments are required.
func NewHealthChecker(db *pgxpool.Pool, nc *nats.Conn) *HealthChecker

// Liveness handles GET /healthz/live.
// Always returns 200 OK with body {"status":"ok"}.
// Used as the Kubernetes livenessProbe.
func (h *HealthChecker) Liveness(w http.ResponseWriter, r *http.Request)

// Readiness handles GET /healthz/ready.
// Pings PostgreSQL and NATS within a 2-second deadline.
// Returns 200 if both pass; 503 if either fails.
// Response body:
//
//	{
//	  "status": "ok"|"error",
//	  "checks": {
//	    "postgres": "ok"|"error",
//	    "nats":     "ok"|"error"
//	  }
//	}
//
// Used as the Kubernetes readinessProbe.
func (h *HealthChecker) Readiness(w http.ResponseWriter, r *http.Request)
```

---

### `internal/api/errors.go`

> **Out of date:** `HandleError` has signature `(w, r, err)` — no `logger` argument; default branch uses `slog.ErrorContext(r.Context(), ...)`. JSON writing is done via exported `WriteJSON` in `internal/api/response.go` (not unexported `writeJSON` here). `ERROR_HANDLING.md` is removed. API layer also defines its own sentinels (`ErrUnauthorized`, `ErrNotFound`, etc.) alongside domain errors.

```go
package api

// ErrorResponse is the Mastodon-compatible error body.
type ErrorResponse struct {
    Error string `json:"error"`
}

// HandleError maps domain/service errors to HTTP responses.
// See ERROR_HANDLING.md for the full mapping table and conventions.
func HandleError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error)

// writeJSON encodes v as JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any)
```

---

### `internal/api/middleware/auth.go`

> **Out of date:** `RequireAuth` and `OptionalAuth` take `(oauth *oauth.Server, accounts *service.AccountService)` instead of `(cache cache.Store, accounts AccountStore)`; token lookup is via OAuth server. `RequireAdmin` takes `(accounts *service.AccountService) func(http.Handler) http.Handler` to look up user role; it is not a simple wrapper of `next`.

```go
package middleware

// RequireAuth extracts the Bearer token from the Authorization header, looks it up
// in the cache (then DB on miss), and stores the resolved account in the context.
// Returns 401 {"error":"…"} if the token is missing, invalid, or revoked.
func RequireAuth(cache cache.Store, accounts AccountStore) func(http.Handler) http.Handler

// OptionalAuth behaves like RequireAuth but continues to the next handler even when
// no token is present or the token is invalid. Downstream handlers check for a nil
// account in context to determine if the request is authenticated.
func OptionalAuth(cache cache.Store, accounts AccountStore) func(http.Handler) http.Handler

// RequireAdmin checks that the authenticated account has role "admin" or "moderator".
// Must be chained after RequireAuth. Returns 403 if the role check fails.
func RequireAdmin(next http.Handler) http.Handler

// AccountFromContext retrieves the authenticated account from the context (nil if absent).
func AccountFromContext(ctx context.Context) *domain.Account

// WithAccount stores an account in the context.
func WithAccount(ctx context.Context, a *domain.Account) context.Context
```

---

### `internal/api/router.go`

> **Out of date:** Router lives in package `router` at `internal/api/router/router.go`. Function is `router.New(deps Deps)`, not `api.NewRouter(deps RouterDeps)`. `Deps` is handler-centric: no `Config`, `Logger`, `DB`, `NATS`, `Cache`, `MediaStore`, `Email`; it has `OAuthServer`, `AccountsService`, `Metrics`, `Health`, and concrete handlers (e.g. `Accounts`, `Statuses`, `Instance`, `WebFinger`, `Inbox`, …), not raw services.

```go
package api

// RouterDeps collects every dependency the router needs to construct handlers.
// Constructor injection: no global variables.
type RouterDeps struct {
    Config  *config.Config
    Logger  *slog.Logger
    Metrics *observability.Metrics
    DB      *pgxpool.Pool
    NATS    *nats.Conn

    // Infrastructure
    Cache      cache.Store
    MediaStore media.Store
    Email      email.Sender

    // Services (populated by serve.go wiring)
    AccountSvc    *service.AccountService
    StatusSvc     *service.StatusService
    TimelineSvc   *service.TimelineService
    FederationSvc *service.FederationService
    ModerationSvc *service.ModerationService
    // ... additional services

    // Handler helpers
    Health *HealthChecker
}

// NewRouter builds and returns the fully configured chi router.
// All middleware is applied and all routes are registered.
func NewRouter(deps RouterDeps) http.Handler
```

---

### `cmd/monstera-fed/main.go`

> **Out of date:** `main()` calls `cli.Execute()` and exits 1 on error; there is no `buildRootCmd()` in main — root is built in `internal/cli/root.go`.

```go
package main

func main() {
    rootCmd := buildRootCmd()
    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}
```

### `cmd/monstera-fed/serve.go`

```go
// serveCmd is the cobra command for starting the HTTP server.
// All startup wiring lives here; it calls newServer() which returns
// a server struct with a Run() method.
var serveCmd = &cobra.Command{
    Use:   "serve",
    Short: "Start the Monstera-fed HTTP server",
    RunE:  runServe,
}

func runServe(cmd *cobra.Command, args []string) error
```

### `cmd/monstera-fed/migrate.go`

> **Out of date:** Implemented in `internal/cli/migrate.go`. There is an additional `migrate down-all` sub-command (`migrateDownAllCmd` / `runMigrateDownAll`). Migrations are run via `store.RunUp` / `store.RunDown` / `store.RunDownAll` from `internal/store/migrate.go` (embed from `internal/store/migrations/`).

```go
var migrateCmd = &cobra.Command{
    Use:   "migrate",
    Short: "Database migration commands",
}

var migrateUpCmd = &cobra.Command{
    Use:   "up",
    Short: "Apply all pending migrations",
    RunE:  runMigrateUp,
}

var migrateDownCmd = &cobra.Command{
    Use:   "down",
    Short: "Roll back the most recent migration",
    RunE:  runMigrateDown,
}
```

---

## 3. Dependency Graph

> **Out of date:** Entry points are `cmd/monstera-fed/main.go` → `cli.Execute()` and `cmd/monstera-fed/internal/cli/serve.go` (and `migrate.go`). Router is built by `internal/api/router` (not `internal/api` directly with a single router.go).

```
cmd/monstera-fed/main.go
└── cmd/monstera-fed/serve.go
    ├── internal/config          (no internal imports)
    ├── internal/observability   → internal/config
    ├── internal/store/postgres  → internal/config
    ├── internal/nats            → internal/config
    ├── internal/cache/memory    (no internal imports)
    ├── internal/cache/redis     → internal/config
    ├── internal/media/local     → internal/config
    ├── internal/media/s3        → internal/config
    ├── internal/email/noop      (no internal imports)
    ├── internal/email/smtp      → internal/config
    ├── internal/domain          (no internal imports)
    ├── internal/service         → internal/domain, internal/store/postgres,
    │                              internal/cache, internal/media, internal/email,
    │                              internal/nats, internal/ap
    └── internal/api             → internal/config, internal/observability,
                                   internal/domain, internal/service,
                                   internal/store/postgres, internal/cache

cmd/monstera-fed/migrate.go
    ├── internal/config
    └── internal/store/postgres (migrations only)
```

**Key constraint:** `internal/domain` has zero internal imports. `internal/service` does not import `internal/api`. The dependency arrow always points inward toward `domain`.

---

## 4. Startup Sequence

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
12. **Build router** — `api.NewRouter(deps)` assembles chi with the full middleware stack and registers all routes. *(Out of date: code uses `router.New(deps)` from `internal/api/router` with handler-centric `Deps`.)*
13. **Start HTTP server** — `http.Server{Addr: ":PORT", Handler: router}`. Call `ListenAndServe` in a goroutine.
14. **Log ready** — structured `slog.Info("server started", "port", cfg.AppPort, "env", cfg.AppEnv)`.
15. **Block on signal** — `signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)`. When the context is cancelled, proceed to shutdown.

---

## 5. Shutdown Sequence

> **Out of date:** Implementation uses a **15-second** shutdown timeout (not 30s). Only HTTP drain and NATS close are explicitly done; there is no explicit “stop federation workers” or “close SSE hub” step (DB closes via defer when `runServe` returns).

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

## 6. Router Middleware Stack

> **Out of date:** CORS is implemented as custom `middleware.CORS` (sets `Access-Control-Allow-Origin: *` and handles OPTIONS), not `cors.Handler(cors.Options{...})`. `/metrics` is **not** registered in the router; `/admin/` routes are **not** registered (admin portal not implemented).

Middleware is applied from outermost to innermost. Order matters:

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

**Route groupings:**

```
/healthz/live       — public, no auth
/healthz/ready      — public, no auth
/metrics            — conditional: MetricsTokenAuth middleware if cfg.MetricsToken != ""

/.well-known/webfinger      — public
/.well-known/nodeinfo        — public
/nodeinfo/2.0                — public

/users/:username             — public (AP Actor)
/users/:username/outbox      — public
/users/:username/followers   — public
/users/:username/following   — public
/users/:username/inbox       — public (POST; HTTP Signature verified inside handler)
/inbox                       — public (shared inbox POST)

/oauth/authorize    — public (renders login/consent HTML)
/oauth/token        — public
/oauth/revoke       — public

/api/v1/apps        — public (register OAuth app)
/api/v2/instance    — public
/api/v1/custom_emojis — public

/api/v1/ (authenticated group)    — RequireAuth middleware
    /accounts/verify_credentials
    /accounts/update_credentials
    /accounts/:id
    /accounts/:id/statuses
    ... (all account, status, timeline, notification, media endpoints)

/api/v1/streaming/ (optional-auth group) — OptionalAuth middleware
    /health
    /user
    /public
    /public/local
    /hashtag

/admin/             — RequireAuth + RequireAdmin; server-rendered HTMX pages
/system/            — static file server (local media driver only)
```

---

## 7. Error Handling Conventions

**Summary:** Domain/store/service layers define sentinel errors in `internal/domain/errors.go` (e.g., `ErrNotFound`, `ErrConflict`, `ErrForbidden`) and wrap errors with context via `fmt.Errorf` with `%w`. No layer below the handler imports `net/http`. A central `HandleError` function in `internal/api/errors.go` maps domain errors to HTTP status codes and the Mastodon-compatible `{"error": "message"}` response shape.

> **Note:** This supersedes the `AppError` struct originally described in this section. `AppError` is removed in favour of domain sentinel errors + handler-layer mapping. See `ERROR_HANDLING.md` §"Compatibility with Existing ADRs" for the migration table.

> **Out of date:** `ERROR_HANDLING.md` has been removed. The API layer also defines sentinels in `api/errors.go` and uses both domain and API errors in `HandleError`. Recoverer calls `api.HandleError(w, r, api.ErrInternalServerError)` instead of `writeJSON(..., ErrorResponse{...})`; logging of request_id, panic, and stack is implemented as in the snippet below.

### Panic Recovery

The `Recoverer` middleware is wrapped to ensure panic details go to the logger, not the response:

```go
slog.Error("recovered from panic",
    "request_id", observability.RequestIDFromContext(r.Context()),
    "panic",       fmt.Sprintf("%v", recovered),
    "stack",       string(debug.Stack()),
)
writeJSON(w, http.StatusInternalServerError, api.ErrorResponse{Error: "Internal server error"})
```

The client only ever sees `{"error":"Internal server error"}`. The full stack trace is in the structured log for operator inspection.

---

## 8. Admin Portal: Go Embed Setup

> **Revised in ADR 10** — the admin portal uses HTMX + Go `html/template` + Pico.css instead of React + Vite. No Node.js build step is needed.

> **Out of date:** The `web/admin/` directory (templates + static) is not present in the repository; admin portal and embed setup are not yet implemented.

- `web/admin/templates/` — Go `html/template` files (layouts, partials, HTMX fragments)
- `web/admin/static/` — vendored static assets (htmx.min.js, Pico.css, custom CSS)
- The Go binary embeds both directories via `//go:embed web/admin/templates` and `//go:embed web/admin/static`
- Admin routes render templates server-side; HTMX handles partial page updates without full reloads

**Makefile targets:**

```makefile
.PHONY: build

build:
	CGO_ENABLED=0 go build -o bin/monstera-fed ./cmd/monstera-fed

docker-build:
	docker build -t monstera-fed:latest .
```

**Dockerfile:**

```dockerfile
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o monstera-fed ./cmd/monstera-fed

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /app/monstera-fed /monstera-fed
ENTRYPOINT ["/monstera-fed"]
```

The admin portal uses Go `html/template` + HTMX + Pico.css, with all assets embedded via `go:embed` — no Node.js build step is needed.

---

## 9. Open Questions

These require product input before final implementation decisions can be made:

| # | Question | Impact |
|---|----------|--------|
| ~~1~~ | ~~**Admin SPA framework**~~ — resolved in ADR 10: HTMX + Go templates + Pico.css. | N/A |
| ~~2~~ | ~~**OAuth token TTL**~~ — resolved: **non-expiring tokens** in Phase 1, matching Mastodon's behavior. Clients cache tokens indefinitely and don't support refresh flows. Revocation on password change or explicit logout covers the security case. | N/A |
| ~~3~~ | ~~**Invite generation by regular users**~~ — resolved: **admin/moderator only** in Phase 1. User-generated invites (with admin-configurable caps) deferred to Phase 2. | N/A |
| ~~4~~ | ~~**Migration on `serve` startup**~~ — resolved: **abort on failure**. Prevents partially-migrated pods from serving traffic. Kubernetes deployments should run migrations as an init container or Job before the Deployment rolls out. | N/A |
| ~~5~~ | ~~**`SECRET_KEY_BASE` uses**~~ — resolved: **HKDF-derived sub-keys** from a single `SECRET_KEY_BASE`, with a unique purpose string per use (`"monstera-fed-csrf"`, `"monstera-fed-email-token"`, `"monstera-fed-actor-private-key"`, `"monstera-fed-invite-token"`). Prevents cross-context key compromise. | N/A |

---

*End of ADR 01 — Project Foundation*
