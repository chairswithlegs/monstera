# Task tracking

At the start of each session, check TaskList for pending work items. If such a task exists, ask the user if you should begin working on it.

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
│   │   ├── mastodon/    # Mastodon REST handlers; sse/ subpackage for SSE streaming
│   │   ├── activitypub/ # ActivityPub HTTP endpoints (inbox, outbox, WebFinger)
│   │   ├── oauth/       # OAuth 2.0 endpoints
│   │   ├── monstera/    # Monstera-specific API endpoints
│   │   ├── middleware/  # HTTP middleware (auth, logging, etc.)
│   │   └── router/      # Route registration
│   ├── activitypub/     # ActivityPub logic (HTTP Signatures, federation delivery, federation subscriber)
│   ├── blocklist/       # Domain blocklist cache
│   ├── oauth/           # OAuth 2.0 flows
│   ├── media/           # Media processing; local/ and s3/ storage drivers
│   ├── cache/           # Cache interface + implementation
│   ├── email/           # Email interface + implementation
│   ├── natsutil/        # NATS JetStream integration
│   ├── events/          # Event and type definitions, outbox poller, notification subscriber
│   ├── scheduler/       # Background job scheduler; jobs/ for job definitions
│   ├── observability/   # Metrics and tracing setup
│   ├── config/          # 12-factor env-var configuration
│   ├── ssrf/            # SSRF protection for outbound HTTP
│   ├── uid/             # ULID generation
│   └── testutil/        # Shared test helpers
├── ui/                  # Next.js frontend (app router, shadcn/Tailwind)
└── docs/                # Architecture docs and ADRs
```

---

# Architecture rules

- `internal/service` never imports `internal/api`. Dependencies point inward toward `domain`.
- IDs are ULIDs via `internal/uid`.
- Config is 12-factor (env vars) via `internal/config`. When adding, removing, or changing any env var, update the Configuration section of `README.md` to match.
- Code in the "adapter" layer (e.g API handlers, workers) should not use the store directly, they should use services instead.
- Client implementations of system dependencies (Postgres, NATs, email) should always be abstracted behind an interface.

## Local/Remote Entity Safety

Follow these rules to prevent local/remote entity conflation bugs:

1. **Guard service mutations** — Methods that only apply to remote entities must call `requireRemote(st.Local, "MethodName")` at the top. Methods that only apply to local entities should call `requireLocal(st.Local, "MethodName")` when the locality is determined by a caller-supplied input; if the method itself explicitly sets locality (e.g. `Local: true` in a create path), the guard is unnecessary.
2. **`*Remote` naming convention** — Methods like `CreateRemote`, `DeleteRemote`, `CreateRemoteFollow` handle remote-originated operations. Both local and remote methods should emit domain events; it is up to consumers (e.g. the federation subscriber) to decide what actions to take based on locality.
3. **Federation subscriber locality checks** — Handlers must check payload locality (`Author.Domain == nil` or `payload.Local`) before federating. Never federate remote-originated events.
4. **Inbox handlers use `*Remote` variants** — Inbox handlers must never call generic service methods. Use `*Remote` variants to avoid triggering outbound federation.
5. **Check Domain, not InboxURL** — Use `account.Domain == nil` for local accounts and `status.Local` for local statuses. Never use `InboxURL == ""` or other proxy fields.
6. **Event payloads include locality** — Include `Local bool` where applicable so subscribers can filter correctly.

---

## Logging

Use the standard library `log/slog` **exported package-level functions** only. Do not create, inject, or store `*slog.Logger` instances.

- When `context.Context` is available: `slog.DebugContext`, `slog.InfoContext`, `slog.WarnContext`, `slog.ErrorContext`.
- Without context: `slog.Debug`, `slog.Info`, `slog.Warn`, `slog.Error`.
- Do not use `fmt.Printf`, `log.Println`, or inject loggers into structs.

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

Fix all linter findings and failing tests before considering the change done. Do not suppress findings or skip tests to make the build green.

---

## Testing Conventions

- Tests live alongside code (`*_test.go` in same package). Use **testify** `assert`/`require`.
- Integration tests (PostgreSQL, NATS) use `//go:build integration`; run with `go test -tags=integration ./path/...`.
- **require** for preconditions (stop on failure); **assert** for verifications (continue to see all failures).
- Table-driven tests for varying inputs; name each case in the struct.
- Call `t.Helper()` in any helper that calls `t.Fatal`/`t.Error`/`require`/`assert`.
- Mark tests with no shared mutable state as `t.Parallel()`.
- HTTP handler tests use `net/http/httptest`.
- Prefer hand-written fakes for simple interfaces; use testify mock when asserting call order or arguments.
- Do not use `t.Skip()` — all unit tests must run under `make test`; all integration tests must run under `make test-integration`.

---

## Error Handling Strategy

### Principles

- **Sentinels in the owning package.** Domain errors (`ErrNotFound`, `ErrConflict`, `ErrForbidden`, etc.) live in `internal/domain/errors.go`. Infra packages (cache, media, email) define their own sentinels.
- **Wrap with context using `%w`.** Each layer adds context: `fmt.Errorf("GetAccountByID(%s): %w", id, err)`. Never wrap with `%v` (breaks `errors.Is`).
- **No REST API types below handlers.** Store, service, cache, media, and email must not import `internal/api` or use HTTP status codes. They return domain/infra errors only. Services **may** import `net/http` for outbound HTTP calls (e.g. fetching URLs, card metadata) — the restriction prevents leaking the inbound REST API into business logic, not outbound HTTP in general.
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

Check the table above before outlining steps or writing code. If a command matches, invoke it early. Do not mention a command without using it.

---

## Subdirectory Rules

- `internal/CLAUDE.md` — interface/implementation pattern
- `internal/store/CLAUDE.md` — database store layer
- `internal/service/CLAUDE.md` — service layer
- `internal/api/CLAUDE.md` — API handler patterns
- `internal/api/mastodon/CLAUDE.md` — Mastodon REST API handlers
- `internal/activitypub/CLAUDE.md` — ActivityPub & federation
- `ui/CLAUDE.md` — Next.js development
