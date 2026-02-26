# Monstera-fed — Error Handling Conventions

> How errors are defined, wrapped, and mapped to HTTP responses across layers.

---

## Principles

1. **Sentinel errors live in the package that owns the concept.** `domain.ErrNotFound` is defined in `internal/domain`, not in a handler or store package.
2. **Errors are wrapped with context as they propagate up.** Each layer adds what it knows — the function name, the entity, the ID — using `fmt.Errorf` with `%w`.
3. **No HTTP knowledge below the handler layer.** Store, service, cache, media, and email packages never import `net/http` or reference status codes. They return domain-level sentinel errors or wrapped errors.
4. **HTTP mapping happens once, at the handler layer.** A single `mapError` function in `internal/api` converts domain errors to HTTP responses. Handlers call it instead of scattering `if/else` chains.

---

## Layer Architecture

```
┌─────────────────────────────────────────────────────┐
│  Handler layer (internal/api/*)                     │
│  - Calls service methods                            │
│  - Matches returned errors via errors.Is / errors.As│
│  - Maps to HTTP status + JSON body                  │
├─────────────────────────────────────────────────────┤
│  Service layer (internal/service/*)                  │
│  - Orchestrates business logic                      │
│  - Returns domain sentinel errors for expected cases│
│  - Wraps unexpected errors with context              │
├─────────────────────────────────────────────────────┤
│  Store layer (internal/store/*)                      │
│  - Executes queries via pgx / sqlc                  │
│  - Translates pgx.ErrNoRows → domain.ErrNotFound   │
│  - Wraps other pgx errors with query context         │
├─────────────────────────────────────────────────────┤
│  Infrastructure (cache, media, email, nats)          │
│  - Defines package-level sentinel errors             │
│  - Wraps provider-specific errors                    │
└─────────────────────────────────────────────────────┘
```

---

## Sentinel Errors

### `internal/domain/errors.go`

The canonical set of business-level errors. These are the errors that service and store layers return for **expected** failure cases.

```go
package domain

import "errors"

var (
    ErrNotFound          = errors.New("not found")
    ErrConflict          = errors.New("conflict")
    ErrForbidden         = errors.New("forbidden")
    ErrUnauthorized      = errors.New("unauthorized")
    ErrValidation        = errors.New("validation error")
    ErrRateLimited       = errors.New("rate limited")
    ErrGone              = errors.New("gone")
    ErrUnprocessable     = errors.New("unprocessable entity")
    ErrAccountSuspended  = errors.New("account suspended")
)
```

These are intentionally terse. Context is added by the caller via wrapping.

### Infrastructure package errors

Each infrastructure package defines its own sentinel errors for conditions specific to that abstraction:

```go
// internal/cache/cache.go
var ErrCacheMiss = errors.New("cache miss")

// internal/media/store.go
var ErrMediaNotFound = errors.New("media: object not found")

// internal/email/sender.go
type ErrSendFailed struct {
    Provider string
    Err      error
}
```

Infrastructure errors that map to domain concepts should be translated at the store or service layer boundary, not passed through to handlers. For example, `cache.ErrCacheMiss` is handled internally by the service — it never reaches a handler.

---

## Error Wrapping

Use `fmt.Errorf` with `%w` to add context at each layer boundary. The wrapped chain should read like a stack trace when printed.

### Store layer

Translate database-specific errors into domain errors. Wrap unexpected errors with the query name.

```go
func (q *Queries) GetAccountByID(ctx context.Context, id string) (domain.Account, error) {
    row, err := q.db.GetAccountByID(ctx, id)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return domain.Account{}, domain.ErrNotFound
        }
        return domain.Account{}, fmt.Errorf("GetAccountByID(%s): %w", id, err)
    }
    return toAccount(row), nil
}
```

For unique constraint violations (e.g., duplicate username):

```go
var pgErr *pgconn.PgError
if errors.As(err, &pgErr) && pgErr.Code == "23505" {
    return domain.Account{}, fmt.Errorf("%w: username already taken", domain.ErrConflict)
}
```

### Service layer

Wrap domain errors with business context. Return sentinel errors for expected cases.

```go
func (s *AccountService) GetByID(ctx context.Context, id string) (domain.Account, error) {
    account, err := s.store.GetAccountByID(ctx, id)
    if err != nil {
        return domain.Account{}, fmt.Errorf("account %s: %w", id, err)
    }
    if account.Suspended {
        return domain.Account{}, fmt.Errorf("account %s: %w", id, domain.ErrAccountSuspended)
    }
    return account, nil
}
```

For input validation, return `ErrValidation` with a descriptive message:

```go
if len(input.Text) > maxChars {
    return domain.Status{}, fmt.Errorf("status text exceeds %d characters: %w", maxChars, domain.ErrValidation)
}
```

### What NOT to do

```go
// BAD: returning an HTTP-aware error from the service layer
return nil, &api.AppError{Status: 404, Message: "not found", Err: err}

// BAD: returning a bare error without context
return nil, err

// BAD: wrapping without %w (breaks errors.Is chain)
return nil, fmt.Errorf("account %s: %v", id, err)
```

---

## HTTP Error Mapping

### `internal/api/errors.go`

The handler layer maps domain errors to HTTP responses via a central function. This is the **only** place where `net/http` status codes appear in error handling.

```go
package api

import (
    "errors"
    "log/slog"
    "net/http"

    "github.com/yourorg/monstera-fed/internal/domain"
)

// ErrorResponse is the standard Mastodon-compatible error body.
type ErrorResponse struct {
    Error string `json:"error"`
}

// HandleError maps a service/domain error to an HTTP response.
// It logs unexpected errors and writes the appropriate status code and message.
func HandleError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) {
    switch {
    case errors.Is(err, domain.ErrNotFound):
        writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "Record not found"})

    case errors.Is(err, domain.ErrConflict):
        writeJSON(w, http.StatusConflict, ErrorResponse{Error: unwrapMessage(err)})

    case errors.Is(err, domain.ErrForbidden):
        writeJSON(w, http.StatusForbidden, ErrorResponse{Error: "Forbidden"})

    case errors.Is(err, domain.ErrUnauthorized):
        writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Unauthorized"})

    case errors.Is(err, domain.ErrValidation):
        writeJSON(w, http.StatusUnprocessableEntity, ErrorResponse{Error: unwrapMessage(err)})

    case errors.Is(err, domain.ErrUnprocessable):
        writeJSON(w, http.StatusUnprocessableEntity, ErrorResponse{Error: unwrapMessage(err)})

    case errors.Is(err, domain.ErrRateLimited):
        w.Header().Set("Retry-After", "900")
        writeJSON(w, http.StatusTooManyRequests, ErrorResponse{Error: "Rate limit exceeded"})

    case errors.Is(err, domain.ErrGone):
        writeJSON(w, http.StatusGone, ErrorResponse{Error: "Gone"})

    case errors.Is(err, domain.ErrAccountSuspended):
        writeJSON(w, http.StatusForbidden, ErrorResponse{Error: "Account suspended"})

    default:
        logger.ErrorContext(r.Context(), "unhandled error",
            "error", err,
            "path", r.URL.Path,
        )
        writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Internal server error"})
    }
}

// unwrapMessage extracts the outermost message from a wrapped error chain.
// For errors like `fmt.Errorf("username already taken: %w", domain.ErrConflict)`,
// this returns "username already taken: conflict" — a message safe to show clients
// because it was composed by our own code (not from pgx or other internals).
func unwrapMessage(err error) string {
    return err.Error()
}
```

### Handler usage

Handlers become clean — no error-mapping logic inline:

```go
func (h *AccountHandler) Get(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")

    account, err := h.accountSvc.GetByID(r.Context(), id)
    if err != nil {
        api.HandleError(w, r, h.logger, err)
        return
    }

    writeJSON(w, http.StatusOK, apimodel.Account(account))
}
```

### Overriding the default message

When a handler needs a more specific client message than the default mapping provides, it can write the response directly:

```go
account, err := h.accountSvc.GetByID(r.Context(), id)
if err != nil {
    if errors.Is(err, domain.ErrAccountSuspended) {
        writeJSON(w, http.StatusGone, api.ErrorResponse{Error: "This account has been suspended"})
        return
    }
    api.HandleError(w, r, h.logger, err)
    return
}
```

This keeps the override explicit and visible in the handler, rather than buried in a mapping table.

---

## ActivityPub Handlers

AP handlers (`internal/api/activitypub/`) follow the same pattern but with AP-specific response conventions:

- Inbox `POST` returns `202 Accepted` on success (no body) and uses `HandleError` for failures.
- Actor endpoints return `410 Gone` for suspended accounts (maps naturally from `domain.ErrGone`).
- Signature verification failures use `writeJSON` directly with `401` since they don't flow through the service layer.

---

## Admin Portal Handlers

Admin handlers (`internal/api/admin/`) render HTML templates rather than JSON. They use a parallel helper:

```go
func HandleAdminError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, tmpl *Templates, err error) {
    switch {
    case errors.Is(err, domain.ErrNotFound):
        tmpl.RenderError(w, http.StatusNotFound, "Not found")
    case errors.Is(err, domain.ErrForbidden):
        tmpl.RenderError(w, http.StatusForbidden, "Access denied")
    default:
        logger.ErrorContext(r.Context(), "admin error", "error", err)
        tmpl.RenderError(w, http.StatusInternalServerError, "Something went wrong")
    }
}
```

For HTMX requests (detected via the `HX-Request` header), error responses render as partial HTML fragments that swap into the page's error region, rather than full error pages.

---

## Logging

Errors are logged at the point where they're handled, not where they're created:

- **Handlers** log unexpected errors (the `default` branch in `HandleError`).
- **Service/store layers** do **not** log errors they return — logging and returning creates duplicate log entries for the same error.
- **Background workers** (federation delivery, token reaper) log errors at the point of final handling since there's no HTTP handler above them.

```go
// Good: handler logs once via HandleError
api.HandleError(w, r, h.logger, err)

// Bad: service logs AND returns (double-logged when handler also logs)
func (s *StatusService) Create(...) error {
    ...
    if err != nil {
        s.logger.Error("failed to create status", "error", err)  // don't do this
        return err
    }
}
```

---

## Compatibility with Existing ADRs

This document supersedes the `AppError` type defined in ADR 01 §7. The `AppError` struct is **removed**. Its responsibilities are replaced by:

| ADR 01 concept | Replacement |
|----------------|-------------|
| `AppError` struct | Domain sentinel errors + `HandleError` mapper |
| `writeAppError` | `HandleError` |
| `writeError` | `writeJSON` with `ErrorResponse` |
| `errInternal` sentinel | The `default` branch in `HandleError` |

The `{"error": "message"}` JSON response shape from ADR 01 §7 is preserved — it's the Mastodon-compatible format all clients expect.
