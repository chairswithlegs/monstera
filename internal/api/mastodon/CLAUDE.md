# Mastodon REST API Handlers

Design doc: `docs/architecture/01-high-level-system-architecture.md`

## Conventions

- All error responses use `api.HandleError(w, r, err)`.
- Response shape: `{"error": "message"}` for errors. Mastodon clients depend on this exact format.
- Pagination: cursor-based with `max_id`, `min_id`, `since_id` query params. Return `Link` header per RFC 5988.
- Content input: plain text only in Phase 1. Strip HTML, auto-link URLs/@mentions/#hashtags.
- HTML sanitization: `github.com/microcosm-cc/bluemonday`.
- API model types and conversion functions in `internal/api/mastodon/apimodel/` map domain types to Mastodon JSON response types.
- Viewer-relative fields (`favourited`, `reblogged`) resolved via batch lookup after fetching the main entities.
- `acct` field: `username` for local, `username@domain` for remote.
- Request body validation: use `api.DecodeAndValidateJSON` for JSON-only endpoints or `parseXxxRequest` helpers for mixed JSON/form. See `internal/api/CLAUDE.md` for details.
- SSE streaming: the `sse/` subpackage contains the SSE hub, NATS subscriber, and event formatting. The hub manages client connections; the subscriber consumes domain events from the `DOMAIN_EVENTS` NATS stream (via the `domain-events-sse` consumer) and publishes to the hub.
- Handler file decomposition: large handlers are split into `{resource}_{concern}.go` files (e.g. `statuses_actions.go` for reblog/favourite/bookmark/pin, `statuses_context.go` for thread context, `accounts_relationships.go` for follow/block/mute).
