# Monstera Project

Monstera is a self-hosted **ActivityPub server** written in Go 1.26 that exposes a **Mastodon-compatible REST API**. Any Mastodon client (Ivory, Tusky, Elk, Mona, etc.) connects without modification.

## Features

- **Mastodon API** — accounts, statuses, timelines, notifications, media, search, streaming (SSE)
- **ActivityPub federation** — follow and interact with users on other instances (Mastodon, Pleroma, etc.)
- **OAuth 2.0** — Authorization Code + PKCE for client apps
- **Next.js UI** — web interface for users, moderation, and instance settings
- **Horizontally scalable** — stateless API pods; PostgreSQL + NATS JetStream for federation delivery and SSE fan-out

## Stack

| Component | Role |
|-----------|------|
| Go 1.26 | Server |
| PostgreSQL 16+ | Primary data store (sqlc-generated queries) |
| NATS JetStream | Federation delivery queue, SSE pub/sub |
| S3 / local disk | Media storage |
| Next.js | Frontend UI |

## Project layout

```
monstera/
├── cmd/
│   ├── server/          # Main HTTP server binary
│   └── loadtest/        # Load testing utility
├── internal/
│   ├── domain/          # Domain types and error sentinels (ErrNotFound, etc.)
│   ├── store/           # Storage interface; postgres/ has the sqlc implementation
│   ├── service/         # Business logic — depends on store, never on api
│   ├── api/
│   │   ├── mastodon/    # Mastodon REST handlers
│   │   ├── activitypub/ # ActivityPub HTTP endpoints (inbox, outbox, WebFinger)
│   │   ├── oauth/       # OAuth 2.0 endpoints
│   │   ├── monstera/    # Monstera-specific API endpoints
│   │   ├── middleware/  # HTTP middleware (auth, logging, etc.)
│   │   └── router/      # Route registration
│   ├── activitypub/     # ActivityPub logic (HTTP Signatures, federation delivery, federation subscriber)
│   ├── outbox/          # Transactional outbox poller and DOMAIN_EVENTS stream config
│   ├── oauth/           # OAuth 2.0 flows
│   ├── media/           # Media processing; local/ and s3/ storage drivers
│   ├── cache/           # Cache interface + implementation
│   ├── email/           # Email interface + implementation
│   ├── nats/            # NATS JetStream integration
│   ├── events/          # Event and type definitions, SSE implementation
│   ├── scheduler/       # Background job scheduler; jobs/ for job definitions
│   ├── observability/   # Metrics and tracing setup
│   ├── config/          # 12-factor env-var configuration
│   ├── uid/             # ULID generation
│   └── testutil/        # Shared test helpers
├── ui/                  # Next.js frontend (app router, shadcn/Tailwind)
└── docs/                # Architecture docs and ADRs
```

---

# Architecture rules

- `internal/service` never imports `internal/api`. Dependencies point inward toward `domain`.
- IDs are ULIDs via `internal/uid`.
- Config is 12-factor (env vars) via `internal/config`.
- Code in the "adapter" layer (e.g API handlers, workers) should not use the store directly, they should use services instead.
- Client implementations of system dependencies (Postgres, NATs, email) should always be abstracted behind an interface.

---

# Code style

- Use `require` for preconditions, `assert` for verifications in tests.
- Use `t.Helper()` in all test helpers.
- No comments that just narrate what the code does.
- Structured logging via `slog` — no `fmt.Printf` or `log.Println`.

---

## Logging

Use the standard library `log/slog` and its **exported package-level functions**. Do not create or pass around `*slog.Logger` instances; the default logger is configured once at application startup. Do not store logger on structs.

### Use Context methods when context is available

Prefer `*Context()` variants so log output can be tied to request/trace context:

- `slog.DebugContext(ctx, msg, ...)`
- `slog.InfoContext(ctx, msg, ...)`
- `slog.WarnContext(ctx, msg, ...)`
- `slog.ErrorContext(ctx, msg, ...)`

When no `context.Context` is in scope, use `slog.Debug`, `slog.Info`, `slog.Warn`, `slog.Error`.

### Do not

- Construct or inject `*slog.Logger` (or interfaces wrapping it) into handlers, services, or stores.
- Store logger on structs.
- Use `fmt.Printf`, `log.Println`, or other ad-hoc logging.

### Examples

```go
// ❌ BAD — passing a logger
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    h.logger.InfoContext(r.Context(), "request received")
}

// ✅ GOOD — package-level slog with context
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    slog.InfoContext(r.Context(), "request received")
}
```

```go
// ❌ BAD — no context when context is available
func process(ctx context.Context, id string) {
    slog.Info("processing", slog.String("id", id))
}

// ✅ GOOD
func process(ctx context.Context, id string) {
    slog.InfoContext(ctx, "processing", slog.String("id", id))
}
```

---

## Lint, Test, and Format After Code Changes

After editing code, run the linter and unit tests before considering the change done.

If the changes touch any integration points, run the integration tests as well.

### Commands

1. **Linter** (from repo root):
   ```bash
   golangci-lint run
   ```
   Fix any reported issues (errcheck, wrapcheck, goimports, etc.) before moving on.

2. **Unit tests**:
   ```bash
   make test
   ```
   Or `go test ./...` if `make test` cannot be used. Ensure all tests pass.

3. **Integration tests**:
   ```
   make test-integration
   ```

4. **Format**:
   ```bash
   go fmt ./...
   ```

### When to run

- After implementing a feature or fix.
- After refactoring or touching multiple files.
- Before committing or marking a task complete.

### If something fails

- **Linter**: Address each finding; do not leave suppressions or TODOs without a clear reason.
- **Tests**: Fix failing tests or update expectations; do not skip or disable tests to make the build green.

---

## Testing Conventions

### Tools

- **go test** (stdlib) — test runner.
- **testify** (v1.10.0) — `assert`, `require`, HTTP helpers, mocking.
- **golangci-lint** — linter; see lint section above for usage.

### Organisation

- Tests live alongside code (`*_test.go` in same package).
- Integration tests (PostgreSQL, NATS) use `//go:build integration` and live in the same package for white-box access. Run with: `go test -tags=integration ./path/...`.

### Conventions

- **require** for preconditions (stop on failure); **assert** for verifications (continue so you see all failures).
- **Table-driven tests** for varying inputs; name each case in the struct (e.g. `name: "plain text"`) — it appears in failure output.
- Call **t.Helper()** in any helper that calls `t.Fatal`/`t.Error`/`require`/`assert` so failures point to the caller.
- Mark tests with no shared mutable state as **t.Parallel()**.
- **HTTP handler tests** use `net/http/httptest` and testify assertions.
- Never log errors in test helpers — use `require`/`assert` so failures report via `t`.

### Mocking

- Prefer **hand-written fakes** for simple interfaces (`store.Store`, `cache.Store`, etc.).
- Use testify **mock** when you need to assert call order or arguments (e.g. "Send called once with this subject").

### No Conditional Skips

When `make test` is run, **all** unit tests must execute. Do not add conditional skips.

When `make test-integration` is run, **all** integration tests must execute. Do not add conditional skips.

### Do not

- Use `t.Skip()` or `t.Skipf()` when a dependency (e.g. DB, NATS, MinIO) is unavailable or env vars are unset.
- Use `t.Skip()` based on environment detection so that CI or local `make test-integration` skips tests.

---

## Error Handling Strategy

### Principles

- **Sentinels in the owning package.** Domain errors (`ErrNotFound`, `ErrConflict`, `ErrForbidden`, etc.) live in `internal/domain/errors.go`. Infra packages (cache, media, email) define their own sentinels.
- **Wrap with context using `%w`.** Each layer adds context: `fmt.Errorf("GetAccountByID(%s): %w", id, err)`. Never wrap with `%v` (breaks `errors.Is`).
- **No HTTP below handlers.** Store, service, cache, media, and email never import `net/http` or use status codes. They return domain/infra errors only.
- **Map to HTTP once.** Handlers use `api.HandleError(w, r, err)`. Match via `errors.Is(err, domain.ErrNotFound)` etc.

### Handler pattern

```go
var body MyRequest
if err := api.DecodeAndValidateJSON(r, &body); err != nil {
    api.HandleError(w, r, err)
    return
}
result, err := h.svc.Do(r.Context(), body)
if err != nil {
    api.HandleError(w, r, err)
    return
}
api.WriteJSON(w, http.StatusOK, result)
```

### Store / service

- **Store:** Translate `pgx.ErrNoRows` → `domain.ErrNotFound`. For unique violations use `errors.As` + `pgErr.Code == "23505"` → `domain.ErrConflict`. Wrap other errors with query/ID context.
- **Service:** Return domain sentinels for expected cases; wrap with business context (e.g. `"account %s: %w"`, id, err). Do **not** log before returning — handlers log in `HandleError`'s default branch.

### Do not

- Return API/HTTP-aware types from service or store.
- Use `%v` when wrapping (use `%w`).
- Log in service/store and then return the same error (double logging).
- Use raw `json.NewDecoder(r.Body).Decode` in handlers (use `api.DecodeJSONBody` or `api.DecodeAndValidateJSON`).
- Return `NewUnprocessableError` for JSON decode failures (use `NewBadRequestError`; 400 = malformed request, 422 = semantically invalid).

---

### Other rules

- Follow existing project rules in this file and in the relevant subdirectory CLAUDE.md files.
- When in doubt, match the plan's **Convention** and **Steps** for that section.

---

## Skills and Commands

Available custom commands (invoke with `/command-name`):

| Command | When to use |
|---------|-------------|
| `/activitypub-expert` | ActivityPub, federation, inbox/outbox, HTTP Signatures, WebFinger, fediverse interop |
| `/mastodon-api-expert` | Mastodon REST API, client compatibility, OAuth flows, timelines, streaming |
| `/code-reviewer` | Review changes for architecture, quality, test adequacy; runs linter and tests |
| `/system-architect` | Architecture review, coupling, layers, codebase structure |
| `/security-analyst` | Security review of APIs, federation, auth, visibility, access control |
| `/ui-designer` | React/shadcn/Tailwind UI code and reviews |
| `/repository-documentation` | Create or update documentation |
| `/vercel-react-best-practices` | React/Next.js performance optimization |

### How to apply

1. **Review available commands first.** Before outlining steps or writing code, check the table above for any that match the task.
2. **Use matching commands.** If a command's description matches the work (e.g. "Use when implementing…"), read and follow its instructions as part of the plan.
3. **Invoke commands early.** Apply relevant commands at the start of the task, not only when stuck.

### Examples

- **Federation / inbox / signatures** → `/activitypub-expert`
- **Mastodon client compatibility / OAuth / timelines** → `/mastodon-api-expert`
- **Docs or README** → `/repository-documentation`
- **Coupling, layers, structure** → `/system-architect`

Do not mention a command without using it: if it is relevant, apply it.

---

## Subdirectory Rules

Additional conventions are loaded automatically when working in these directories:

- `internal/CLAUDE.md` — interface/implementation pattern
- `internal/store/CLAUDE.md` — database store layer
- `internal/service/CLAUDE.md` — service layer
- `internal/api/CLAUDE.md` — API handler patterns
- `internal/api/mastodon/CLAUDE.md` — Mastodon REST API handlers
- `internal/activitypub/CLAUDE.md` — ActivityPub & federation
- `ui/CLAUDE.md` — Next.js development
