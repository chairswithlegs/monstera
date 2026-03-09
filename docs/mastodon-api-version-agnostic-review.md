# Mastodon API routes: version-agnostic review

This document reviews which Mastodon API endpoints in Monstera are **version-agnostic** (same request/response contract and handler logic regardless of `/api/v1` vs `/api/v2`) versus **version-specific**. It supports client compatibility (e.g. Elk may call v1 or v2 for the same feature) and avoids duplicate handler logic.

**Definitions:**
- **Version-agnostic**: One handler; same request/response shape for v1 and v2; no branching on URL or version. Safe to expose on both paths if the Mastodon spec allows.
- **Version-specific**: Different response shapes per version (e.g. instance v1 vs v2) or the spec defines the endpoint for only one version.

---

## Currently exposed on both v1 and v2

| Method | Path (both versions) | Handler | Version-agnostic? |
|--------|----------------------|---------|--------------------|
| POST   | `/api/v1/media`, `/api/v2/media` | `MediaHandler.POSTMedia` | **Yes** — same multipart form (`file`, `description`), same Media entity response. No version branching. |
| GET    | `/api/v1/search`, `/api/v2/search` | `SearchHandler.GETSearch` | **Yes** — same query params and response shape (`accounts`, `statuses`, `hashtags`). Exposed on both to avoid 404s when clients use v1 path. |
| GET    | `/api/v1/suggestions`, `/api/v2/suggestions` | `SuggestionsHandler.GETSuggestions` | **Yes** — same response (`[]`). Exposed on both to avoid 404s when clients use v1 path. |

All three are registered under both v1 and v2; handlers are version-agnostic and documented as serving both.

---

## Version-agnostic but v1-only (by Mastodon spec)

| Method | Route(s) | Handler | Notes |
|--------|----------|---------|--------|
| PUT    | `/api/v1/media/{id}` only | `MediaHandler.PUTMedia` | **Agnostic.** Same form (description, focus) and Media response. Mastodon only defines PUT media under v1; no v2 equivalent in spec. Keep as v1-only. |

---

## Version-specific (intentional)

| Method | Path | Handler | Why version-specific |
|--------|------|---------|----------------------|
| GET    | `/api/v1/instance` | `InstanceHandler.GETInstanceV1` | v1 entity: `uri`, `title`, `urls.streaming_api`, `stats`, `contact_account`, `rules`, etc. |
| GET    | `/api/v2/instance` | `InstanceHandler.GETInstance` | v2 entity: `domain`, `configuration` (with `urls.streaming`, statuses/media/polls limits), `registrations`, `contact`. Different JSON shape; clients (e.g. Elk) use v2 for `configuration.urls.streaming`. |

Instance is the only endpoint that **must** be version-specific: Mastodon’s v1 and v2 instance responses have different structures. Two handlers (and two response types) are correct.

---

## v1-only routes (Mastodon spec)

These exist only under `/api/v1` in both the Mastodon spec and Monstera. No v2 variant exists in the spec; handlers are agnostic of “version” (they don’t read the path).

- Apps: `POST /api/v1/apps`
- Accounts: verify_credentials, lookup, relationships, :id/statuses|followers|following, follow|unfollow|block|unblock|mute|unmute, etc.
- Statuses: POST, GET :id, context, reblog, favourite, bookmark, pin, edit, delete, etc.
- Timelines: home, public, tag, list, favourites, bookmarks
- Media: `PUT /api/v1/media/:id` (POST is on both v1 and v2)
- Notifications, reports, follow_requests, lists, filters, markers, preferences, featured_tags, followed_tags, announcements, conversations, trends, custom_emojis, polls, scheduled_statuses, directory

No change needed for “version agnostic” — they are v1-only by spec.

---

## v2-only routes (Mastodon spec)

| Method | Path | Handler | Version-agnostic? |
|--------|------|---------|--------------------|
| GET    | `/api/v2/instance` | `InstanceHandler.GETInstance` | No — v2 shape only. |
| GET    | `/api/v2/search` | `SearchHandler.GETSearch` | Yes — could also serve `/api/v1/search` if we add it. |
| POST   | `/api/v2/media` | `MediaHandler.POSTMedia` | Yes — also exposed on v1. |
| GET    | `/api/v2/suggestions` | `SuggestionsHandler.GETSuggestions` | Yes — implementation is agnostic. |

---

## Summary

| Category | Endpoints | Status |
|----------|-----------|--------|
| **Exposed on both v1 and v2** | POST /media, GET /search, GET /suggestions | All three wired on both paths to avoid client 404s. |
| **Version-specific by design** | GET /instance (v1 vs v2) | Two handlers; no change. |
| **v1-only by spec** | PUT /media/:id, all other v1 routes | No v2 equivalent in spec; keep as-is. |

**Conclusion:** The only endpoint that must differ by version is **GET /instance**. **POST /media**, **GET /search**, and **GET /suggestions** are version-agnostic and are exposed on both v1 and v2 so clients that use either path succeed.
