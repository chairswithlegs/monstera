# Mastodon API: stubbed and deferred endpoints

This document lists Mastodon API endpoints that are **stubbed** (handler exists and returns minimal/empty data so clients do not 404) or **deferred** (not implemented; no route or documented as optional/later). It is derived from the implementation plan, roadmap, and current router/handlers.

---

## Stubbed endpoints

These endpoints are registered and return a valid response shape (typically an empty array) so that clients (e.g. Elk, Tusky, Ivory) can load pages without errors. The underlying features are not fully implemented.

| Method | Endpoint | Response | Reason |
|--------|----------|----------|--------|
| GET | `/api/v1/custom_emojis` | `[]` | Custom emoji support not implemented; plan lists as stub in §1.1. Returns empty list so clients do not break. |
| GET | `/api/v1/trends/statuses` | `[]` | Trending statuses require a trending algorithm and possibly new schema; marked "optional later" in plan §1.2. Stubbed so Explore-style views work. |
| GET | `/api/v1/trends/tags` | `[]` | Trending hashtags not implemented; same rationale as trends/statuses. Stubbed for client compatibility. |
| GET | `/api/v1/trends/links` | `[]` | Trending links not implemented; same rationale as other trends. Stubbed for client compatibility. |
| GET | `/api/v2/suggestions` | `[]` | Account suggestions (who to follow) not implemented. Stubbed so clients that call this (e.g. Elk) do not 404. |

**References:** Plan §1.1 (custom_emojis stub), §1.2 (optional later: trends); router and handlers: `InstanceHandler.GETCustomEmojis`, `TrendsHandler` (statuses/tags/links), `SuggestionsHandler.GETSuggestions`.

---

## Deferred endpoints (no handler / 404)

These Mastodon API endpoints are **not** registered. Requests return 404. They are documented as deferred or "optional later" in the plan or roadmap.

| Method | Endpoint | Reason |
|--------|----------|--------|
| GET | `/api/v1/push/subscription` | Web Push subscriptions not implemented. Plan §1.2: "optional later … push subscription". Roadmap: "Push notification subscriptions (Web Push)" — medium effort; requires VAPID, subscription storage, and push delivery. |
| POST | `/api/v1/push/subscription` | Same as above. |
| PUT | `/api/v1/push/subscription` | Same as above. |
| DELETE | `/api/v1/push/subscription` | Same as above. |

**Note:** Mastodon documents that GET returns the current subscription or 404 when none exists. Returning 404 for GET is therefore valid when push is unsupported; some clients may still request it and need to handle 404.

---

## Deferred behavior (not separate endpoints)

These are not standalone endpoints but missing or minimal behavior in existing endpoints or entities.

| Item | Where it appears | Reason |
|------|------------------|--------|
| **Translate** | Mastodon has e.g. POST translate or similar | Plan §1.2: "optional later … translate". Not implemented; no stub. |

---

## Recently implemented (previously listed here)

These items were previously listed as stubbed or deferred but are now fully implemented.

| Item | Notes |
|------|-------|
| **Conversations** (`GET /api/v1/conversations`) | Full implementation via `ConversationService.ListConversations` with pagination. Also includes `DELETE /api/v1/conversations/:id` and `POST /api/v1/conversations/:id/read`. |
| **Quotes** | `quoted_status_id` on status creation, `GET /api/v1/statuses/:id/quotes`, quote revoke, and interaction policy. |
| **Status card (link preview)** | `card` field on Status entity populated via async background job (`fetch-status-cards`, 1-min interval). OG/meta tag parsing via `CardService`; stored in `status_cards` table. |

---

## Summary

- **Stubbed (5 endpoints):** `custom_emojis`, `trends/statuses`, `trends/tags`, `trends/links`, `suggestions` (v2). All return empty arrays so clients can load UI without 404s.
- **Deferred (4 push endpoints):** Full push subscription CRUD; no routes; 404 is acceptable for GET when push is unsupported.
- **Deferred behavior:** Translate — documented as optional/later; no stub endpoints added.

For the full implementation plan and phase order, see [mastodon-api-remainder-implementation-plan.md](mastodon-api-remainder-implementation-plan.md). For roadmap context, see [roadmap.md](roadmap.md).
