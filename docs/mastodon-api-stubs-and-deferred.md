# Mastodon API: stubbed and deferred endpoints

This document lists Mastodon API endpoints that are **stubbed** (handler exists and returns minimal/empty data so clients do not 404) or **deferred** (not implemented; no route or documented as optional/later).

---

## Stubbed endpoints

These endpoints are registered and return a valid response shape (typically an empty array) so that clients (e.g. Elk, Tusky, Ivory) can load pages without errors. The underlying features are not fully implemented.

| Method | Endpoint | Response | Reason |
|--------|----------|----------|--------|
| GET | `/api/v1/custom_emojis` | `[]` | Custom emoji support not implemented; plan lists as stub in §1.1. Returns empty list so clients do not break. |
| GET | `/api/v1/trends/links` | `[]` | Trending links not implemented; same rationale as other trends. Stubbed for client compatibility. |
| GET | `/api/v2/suggestions` | `[]` | Account suggestions (who to follow) not implemented. Stubbed so clients that call this (e.g. Elk) do not 404. |

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

| Item | Where it appears |
|------|------------------|
| **Translate** | Mastodon has e.g. POST translate or similar |
