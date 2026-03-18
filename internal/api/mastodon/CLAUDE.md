# Mastodon REST API Handlers

Design doc: `docs/architecture/01-high-level-system-architecture.md`

## Conventions

- All error responses use `api.HandleError(w, r, err)`.
- Pagination: cursor-based with `max_id`, `min_id`, `since_id` query params. Return `Link` header per RFC 5988.
- Content input: plain text only. Strip HTML, auto-link URLs/@mentions/#hashtags. Sanitize with `github.com/microcosm-cc/bluemonday`.
- API model types and conversion functions in `internal/api/mastodon/apimodel/` map domain types to Mastodon JSON response types.
- Viewer-relative fields (`favourited`, `reblogged`) resolved via batch lookup after fetching the main entities.
- Request body validation: use `api.DecodeAndValidateJSON` for JSON-only endpoints or `parseXxxRequest` helpers for mixed JSON/form. See `internal/api/CLAUDE.md` for details.
- SSE streaming: the `sse/` subpackage contains the SSE hub, NATS subscriber, and event formatting. The hub manages client connections; the subscriber consumes domain events from the `DOMAIN_EVENTS` NATS stream (via the `domain-events-sse` consumer) and publishes to the hub.
