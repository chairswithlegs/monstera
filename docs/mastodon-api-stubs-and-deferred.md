# Mastodon API: stubbed and deferred endpoints

This document lists Mastodon API endpoints that are **stubbed** (handler exists and returns minimal/empty data so clients do not 404) or **deferred** (not implemented; no route or documented as optional/later).

---

## Stubbed endpoints

These endpoints are registered and return a valid response shape (typically an empty array) so that clients (e.g. Elk, Tusky, Ivory) can load pages without errors. The underlying features are not fully implemented.

| Method | Endpoint | Response | Reason |
|--------|----------|----------|--------|
| GET | `/api/v1/custom_emojis` | `[]` | Custom emoji support not implemented; plan lists as stub in §1.1. Returns empty list so clients do not break. |
| GET | `/api/v2/suggestions` | `[]` | Account suggestions (who to follow) not implemented. Stubbed so clients that call this (e.g. Elk) do not 404. |

---

## Deferred behavior (not separate endpoints)

These are not standalone endpoints but missing or minimal behavior in existing endpoints or entities.

| Item | Where it appears |
|------|------------------|
| **Translate** | Mastodon has e.g. POST translate or similar |
