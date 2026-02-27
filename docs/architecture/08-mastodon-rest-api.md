# Mastodon REST API

This document describes the desired Mastodon-compatible REST handlers, response types, pagination, and content rendering. Build order is in [roadmap.md](../roadmap.md).

---

## Design decisions

| Question | Decision |
|----------|----------|
| Account counter columns | **Denormalized** — `followers_count`, `following_count`, `statuses_count` on `accounts`. Maintained via increment/decrement in service-layer transactions (same pattern as `statuses.replies_count`). |
| Status content input format | **Plain text only** in Phase 1. Strip all HTML from input, auto-link URLs / @mentions / #hashtags. Markdown deferred to Phase 2. |
| Viewer-relative fields | **Batch lookup** — after fetching a list of statuses, one query resolves `favourited`/`reblogged` for all status IDs; one query resolves relationship flags for all account IDs. |
| `acct` formatting | `username` for local accounts, `username@domain` for remote — standard Mastodon convention. |
| Idempotency key | Cache response under `idempotency:{accountID}:{key}` for 1 hour. Return cached response on duplicate. |
| Profile metadata fields | **Phase 1** — JSONB column `fields` on `accounts`. Max 4 fields, each `{name, value, verified_at}`. |
| Scheduled statuses | **Deferred** to Phase 2. Return 422 if `scheduled_at` is provided. |
| Conversation muting | **Deferred** to Phase 2. `muted` on Status response hardcoded to `false`. |
| Bookmarks / pinned | **Deferred** to Phase 2. Both fields hardcoded to `false`. |
| `card` (link preview) | **Deferred** to Phase 2. Return `null`. |
| `poll` | **Deferred** to Phase 2. Return `null`. |
| `in_reply_to_account_id` | **Denormalized** column on `statuses` — set at creation time to avoid a JOIN on every render. |
| Mentions storage | **`status_mentions` join table** — same pattern as `status_hashtags`. |
| User preferences | `default_privacy`, `default_sensitive`, `default_language` columns on `users` — returned in `source` on `verify_credentials`. |
| Content sanitization library | `github.com/microcosm-cc/bluemonday` for output sanitization; `mvdan.cc/xurls/v2` for URL detection. |

---

## File Layout

```
internal/api/mastodon/
├── accounts.go         — Account CRUD, follow/unfollow/block/mute, relationships
├── statuses.go         — Create/delete/boost/favourite, context, favourited_by, reblogged_by
├── timelines.go        — Home, public, hashtag
├── notifications.go    — List, clear, dismiss
├── media.go            — (already designed in IMPLEMENTATION 04)
├── search.go           — Accounts, hashtags (statuses empty in Phase 1)
├── instance.go         — Instance metadata, custom_emojis
├── streaming.go        — SSE (detailed in Prompt 09)
└── helpers.go          — Pagination helpers, common response writers

internal/api/mastodon/apimodel/
├── account.go          — domain → MastodonAccount
├── status.go           — domain → MastodonStatus
├── notification.go     — domain → MastodonNotification
└── relationship.go     — query result → MastodonRelationship

internal/service/
├── content.go          — Render: plain text → HTML, extract mentions + hashtags
└── search_service.go   — Account search, hashtag search, WebFinger resolve
```

---

## 1. Mastodon Response Types

### `Account`


**Presenter rules:**
- `acct`: `username` if `domain IS NULL`, else `username@domain`.
- `url`: `https://{INSTANCE_DOMAIN}/@{username}` for local; `ap_id` for remote.
- `avatar` / `header`: resolve via `avatar_media_id` / `header_media_id` → `MediaStore.URL()`. Fallback to a default placeholder URL if NULL.
- `avatar_static` = `avatar` (animated avatar support is Phase 2).
- `last_status_at`: only populated on full account lookups (not embedded in Status responses). Query: `SELECT created_at::date FROM statuses WHERE account_id = $1 AND deleted_at IS NULL ORDER BY id DESC LIMIT 1`.
- `source`: only included for `GET /api/v1/accounts/verify_credentials`. Populated from the `users` table preferences + raw `accounts.note` (plain text, pre-rendering).
- `fields`: deserialized from `accounts.fields` JSONB. Values are HTML-rendered (links auto-linked). `source.fields` returns raw values.
- `emojis`: empty array `[]` until custom emoji in Phase 2.

### `Status`
**Presenter rules:**
- `url`: `https://{INSTANCE_DOMAIN}/@{username}/{id}` for local statuses; `null` for remote.
- `spoiler_text`: `content_warning` column; empty string if NULL.
- `reblog`: if `reblog_of_id` is set, recursively render the original status. The outer status carries the booster's account; the inner `reblog` carries the original author and content.
- `media_attachments`: query `ListStatusAttachments` → map via `toMediaResponse` (IMPLEMENTATION 04).
- `mentions`: query `status_mentions` join table → render as `Mention` with account info.
- `tags`: query `GetStatusHashtags` → render as `Tag` with `url: https://{INSTANCE_DOMAIN}/tags/{name}`.
- `favourited` / `reblogged`: populated from the batch interaction query (see §4).

### `Instance` (v2)
Nested types include `InstanceConfig` with `statuses.max_characters`, `media_attachments.supported_mime_types`, `media_attachments.image_size_limit`, etc. All values sourced from `instance_settings` table (cached with 5-minute TTL per IMPLEMENTATION 03).

---

## 2. Pagination Design

### `PageParams`

Parsed from query string on every paginated endpoint:


**`PageParamsFromRequest(r *http.Request) PageParams`** — parses `max_id`, `min_id`, `since_id`, `limit` from query string. Clamps `limit` to `[1, 40]`, defaults to 20.

### Cursor behavior

| Parameter | SQL clause | ORDER BY | Post-processing |
|-----------|-----------|----------|-----------------|
| `max_id` | `id < $maxID` | `DESC` | none |
| `since_id` | `id > $sinceID` | `DESC` | none |
| `min_id` | `id > $minID` | `ASC` | reverse slice |
| none | (no bound) | `DESC` | none |

When `min_id` is set, the query uses `ORDER BY id ASC LIMIT $n` to fetch the N items immediately after the anchor, then reverses the slice so the response is newest-first. This gives clients the "fill forward from bottom" behavior they expect.

### `LinkHeader`

**`LinkHeader(requestURL string, items []T, idFunc func(T) string) string`** — builds the RFC 5988 `Link` header from the first and last item IDs in the result set:

```
Link: <https://social.example.com/api/v1/timelines/home?max_id=LAST_ID>; rel="next",
      <https://social.example.com/api/v1/timelines/home?min_id=FIRST_ID>; rel="prev"
```

If the result set is empty, no `Link` header is set. The helper preserves any existing query parameters (e.g. `local=true`) from the original request URL.

### SQL pattern

All paginated timeline queries support all three cursor parameters. Example shape (home timeline):


For `min_id`, the service layer calls a `*Forward` variant that uses `ORDER BY s.id ASC`, then reverses.

---

## 3. Content Rendering Pipeline

**File:** `internal/service/content.go`

### Interface


`MentionResolver` is a callback `func(username, domain string) *domain.Account` that the caller provides — typically a closure over `AccountStore` that does local + remote lookup.

### Pipeline steps

1. **Strip HTML** — run `bluemonday.StrictPolicy().Sanitize(text)` to remove any HTML tags from input. This is the Phase 1 plain-text contract.
2. **Extract @mentions** — regex `@([a-zA-Z0-9_]+)(?:@([a-zA-Z0-9\.\-]+))?`. For each match, call `MentionResolver`. If the account is found, record it as a `MentionRef` and replace the raw text with `<span class="h-card"><a href="{url}" class="u-url mention">@<span>{username}</span></a></span>`.
3. **Extract #hashtags** — regex `#([a-zA-Z0-9_]+)`. Normalize to lowercase. Replace with `<a href="https://{INSTANCE_DOMAIN}/tags/{name}" class="mention hashtag" rel="tag">#<span>{name}</span></a>`.
4. **Auto-link URLs** — use `xurls.Strict()` to detect bare URLs. Wrap each in `<a href="{url}" rel="nofollow noopener noreferrer" target="_blank">{display}</a>`. Display text is the URL with protocol stripped, truncated to 30 chars with `…` if longer.
5. **Paragraph wrapping** — split on `\n\n` for paragraph breaks (`<p>…</p>`), `\n` for line breaks (`<br>`).
6. **Output sanitization** — final pass through `bluemonday` with the allowed tag policy: `<p>`, `<br>`, `<a>` (with `href`, `rel`, `class`, `target`), `<span>` (with `class`). This catches any edge cases from the rendering steps.

### Mention + hashtag persistence

After `Render` returns, the status creation flow:
1. For each `MentionRef` with a resolved account: insert into `status_mentions(status_id, account_id)`.
2. For each tag name: call `GetOrCreateHashtag` → `AttachHashtagsToStatus`.
3. For each mention: create a `notification` of type `mention` for the target account (if local).

---

## 4. Viewer-Relative Batch Lookups

### Status interactions

After fetching a list of statuses, collect all status IDs into a single batch query:


The API model layer merges these flags into each `Status` response. If the viewer is unauthenticated, all flags default to `false` (skip the query entirely).

### Account relationships

For `GET /api/v1/accounts/relationships?id[]=...` and for enriching status responses with block/mute awareness:


### Batch mentions and tags

For a list of status IDs, fetch all mentions and tags in two queries:

The API model layer groups results by `status_id` and merges into each Status response.

### Presenter assembly flow

For any endpoint returning a list of statuses:

1. Fetch status rows (timeline query, account statuses, etc.).
2. Collect unique status IDs and account IDs.
3. Run batch queries in parallel: interactions, mentions, tags, attachments, accounts (if needed).
4. Build a lookup map for each batch result.
5. Assemble `[]Status` response objects, merging all data.

This keeps the query count fixed at O(1) per batch regardless of page size (typically 5-6 queries total for a 20-item timeline page).

---

## 5. Accounts Handlers

**File:** `internal/api/mastodon/accounts.go`

### Handler struct


### `GET /api/v1/accounts/verify_credentials`

- **Auth:** Required (`read:accounts`).
- **Logic:** Fetch `accounts` row for the authenticated user, fetch `users` row for preferences. Present as `Account` with `Source` populated.
- **Response:** 200 `Account` (with `source` field).

### `PATCH /api/v1/accounts/update_credentials`

- **Auth:** Required (`write:accounts`).
- **Content-Type:** `multipart/form-data`.
- **Accepted fields:** `display_name`, `note` (plain text → rendered HTML via `Render`), `avatar` (file), `header` (file), `locked`, `bot`, `source[privacy]`, `source[sensitive]`, `source[language]`, `fields_attributes[0][name]`, `fields_attributes[0][value]` (max 4 pairs).
- **Logic:** Parse form. Upload avatar/header via `MediaService.Upload` if provided. Update `accounts` row (display_name, note, note rendered, avatar_media_id, header_media_id, locked, bot, fields JSONB). Update `users` row (default_privacy, default_sensitive, default_language). Invalidate AP actor cache.
- **Response:** 200 `Account` (with `source`).
- **Errors:** 422 for validation failures (note too long, display_name too long, invalid field count).

### `GET /api/v1/accounts/:id`

- **Auth:** Optional.
- **Logic:** Fetch account by ID. Return 404 if not found or suspended (unless viewer is admin).
- **Response:** 200 `Account`.

### `GET /api/v1/accounts/:id/statuses`

- **Auth:** Optional.
- **Query params:** `max_id`, `min_id`, `since_id`, `limit` (pagination); `only_media` (bool); `exclude_replies` (bool, default true); `exclude_reblogs` (bool).
- **Logic:** Fetch statuses for the target account with filters. Visibility filtering: if viewer != account owner, exclude `private` and `direct` statuses; if viewer doesn't follow the account, also exclude `private`. Run batch API model assembly.
- **Response:** 200 `[]Status` with `Link` header.

### `GET /api/v1/accounts/:id/followers` and `/following`

- **Auth:** Optional.
- **Logic:** Paginated query via `GetFollowers` / `GetFollowing`. If the target account is `locked` and the viewer doesn't follow them, return an empty list (Mastodon convention for hiding social graph on locked accounts).
- **Response:** 200 `[]Account` with `Link` header.

### `POST /api/v1/accounts/:id/follow`

- **Auth:** Required (`write:follows`).
- **Logic:** Check for self-follow (400). Check for existing follow. Check for block in either direction (block takes precedence). Create follow with `state = 'pending'` if target is locked, `state = 'accepted'` otherwise. Update counter columns in transaction. Enqueue federation `Follow` activity. Create `follow` or `follow_request` notification.
- **Response:** 200 `Relationship`.

### `POST /api/v1/accounts/:id/unfollow`

- **Auth:** Required (`write:follows`).
- **Logic:** Delete follow row. Decrement counter columns. Enqueue federation `Undo{Follow}` activity.
- **Response:** 200 `Relationship`.

### `POST /api/v1/accounts/:id/block`

- **Auth:** Required (`write:blocks`).
- **Logic:** Create block row. If a follow exists in either direction, delete it and decrement counters. If a mute exists, delete it. Enqueue federation `Block` activity.
- **Response:** 200 `Relationship`.

### `POST /api/v1/accounts/:id/unblock`

- **Auth:** Required (`write:blocks`).
- **Logic:** Delete block row. Enqueue federation `Undo{Block}` activity.
- **Response:** 200 `Relationship`.

### `POST /api/v1/accounts/:id/mute`

- **Auth:** Required (`write:mutes`).
- **Request body:** optional `notifications` (bool, default true — controls `hide_notifications`).
- **Logic:** Upsert mute row.
- **Response:** 200 `Relationship`.

### `POST /api/v1/accounts/:id/unmute`

- **Auth:** Required (`write:mutes`).
- **Logic:** Delete mute row.
- **Response:** 200 `Relationship`.

### `GET /api/v1/accounts/relationships`

- **Auth:** Required (`read:follows`).
- **Query params:** `id[]` — one or more account IDs.
- **Logic:** Run `GetBatchRelationships` query. Map results to `[]Relationship`.
- **Response:** 200 `[]Relationship`.
- **Note:** chi route ordering — this must be registered before `/api/v1/accounts/:id` to avoid `relationships` being parsed as an `:id`.

---

## 6. Statuses Handlers

**File:** `internal/api/mastodon/statuses.go`

### `POST /api/v1/statuses` — Status Creation Flow

- **Auth:** Required (`write:statuses`).

**Request body** (JSON or form):

| Field | Type | Notes |
|-------|------|-------|
| `status` | string | Required (unless `media_ids` is non-empty) |
| `in_reply_to_id` | string | Optional |
| `media_ids` | []string | Optional, max 4 |
| `sensitive` | bool | Default false |
| `spoiler_text` | string | Optional content warning |
| `visibility` | string | `public`\|`unlisted`\|`private`\|`direct` |
| `language` | string | ISO 639-1, optional |
| `scheduled_at` | string | **Rejected** — return 422 in Phase 1 |

**Step-by-step flow:**

1. **Idempotency check** — if `Idempotency-Key` header is present, check cache for `idempotency:{accountID}:{key}`. On hit, return the cached response body and status code immediately.
2. **Validate visibility** — must be one of the four allowed values. Default to user's `default_privacy` if omitted.
3. **Validate character count** — strip HTML/URLs from `status` text, count characters. Enforce `MAX_STATUS_CHARS` (from config, default 500). URLs count as 23 characters regardless of length (Mastodon convention).
4. **Validate media** — if `media_ids` provided, verify each exists, belongs to the authenticated user, and is not already attached to another status. Max 4 attachments.
5. **Validate reply** — if `in_reply_to_id` is set, fetch the parent status. Store `in_reply_to_account_id` (denormalized). Verify the viewer isn't blocked by the parent author.
6. **Render content** — call `content.Render(text, mentionResolver)`. Receives rendered HTML, extracted mentions, extracted hashtags.
7. **Generate IDs** — `uid.New()` for the status ID. Compute `uri` and `ap_id` as `https://{INSTANCE_DOMAIN}/users/{username}/statuses/{id}`.
8. **Database transaction:**
   - Insert `statuses` row.
   - Attach media: update `media_attachments.status_id` for each `media_id`.
   - Persist mentions: insert into `status_mentions`.
   - Persist hashtags: `GetOrCreateHashtag` → `AttachHashtagsToStatus`.
   - Increment `accounts.statuses_count`.
   - If reply: increment parent's `replies_count`.
   - Create `mention` notifications for local mentioned accounts.
9. **Post-transaction:**
   - Enqueue federation `Create{Note}` via NATS.
   - Publish SSE events: `stream.public` (if public), `stream.public.local` (if public + local), `stream.user.{mentionedAccountID}` (for each local mention), `stream.hashtag.{tag}` (for each hashtag).
10. **Idempotency cache** — store the response under the idempotency key with 1-hour TTL.
11. **Response:** 200 `Status`.

### `GET /api/v1/statuses/:id`

- **Auth:** Optional.
- **Logic:** Fetch status. Enforce visibility: `direct` only visible to author and mentioned accounts; `private` only to author and followers. 404 if deleted or invisible.
- **Response:** 200 `Status`.

### `DELETE /api/v1/statuses/:id`

- **Auth:** Required (`write:statuses`).
- **Logic:** Verify ownership. Soft-delete (`deleted_at = NOW()`). Decrement counters. If reply, decrement parent's `replies_count`. Enqueue federation `Delete{Tombstone}`. Publish SSE `delete` event with the status ID.
- **Response:** 200 `Status` (the deleted status, with `text` field included — Mastodon convention for delete-and-redraft).

### `POST /api/v1/statuses/:id/reblog`

- **Auth:** Required (`write:statuses`).
- **Logic:** Check status exists and isn't private/direct (boosts are only valid for public/unlisted). Check for existing reblog by this user (409 if already reblogged). Create a new status with `reblog_of_id` pointing to the target. Increment `reblogs_count`. Create `reblog` notification. Enqueue federation `Announce`.
- **Response:** 200 `Status` (the new boost status, with nested `reblog`).

### `POST /api/v1/statuses/:id/unreblog`

- **Auth:** Required (`write:statuses`).
- **Logic:** Find the viewer's reblog status via `GetReblogByAccountAndTarget`. Soft-delete it. Decrement `reblogs_count`. Enqueue federation `Undo{Announce}`.
- **Response:** 200 `Status` (the original status, with updated counts).

### `POST /api/v1/statuses/:id/favourite` and `/unfavourite`

- **Auth:** Required (`write:favourites`).
- **Logic:** Create/delete `favourites` row. Increment/decrement `favourites_count`. Create `favourite` notification (on favourite). Enqueue federation `Like` / `Undo{Like}`.
- **Response:** 200 `Status`.

### `GET /api/v1/statuses/:id/context`

- **Auth:** Optional.
- **Logic:** Fetch ancestors via `GetStatusAncestors` (recursive CTE up the reply chain). Fetch descendants via `GetStatusDescendants` (recursive CTE down). Filter both lists for visibility (respect blocks, mutes, visibility levels). Run batch API model assembly for both lists.
- **Response:** 200 `{"ancestors": []Status, "descendants": []Status}`.

The recursive CTEs (already defined in IMPLEMENTATION 02) walk `in_reply_to_id` in both directions. Ancestors are ordered oldest-first (ASC); descendants are ordered oldest-first (ASC) for threading.

### `GET /api/v1/statuses/:id/favourited_by` and `/reblogged_by`

- **Auth:** Optional.
- **Logic:** Paginated list of accounts. `favourited_by` uses `GetStatusFavouritedBy`; `reblogged_by` queries statuses where `reblog_of_id = :id` and returns their account objects.
- **Response:** 200 `[]Account` with `Link` header.

---

## 7. Timeline Handlers

**File:** `internal/api/mastodon/timelines.go`

### `GET /api/v1/timelines/home`

- **Auth:** Required (`read:statuses`).
- **Query params:** Standard pagination.
- **Logic:** Call `GetHomeTimeline` (the UNION ALL query from IMPLEMENTATION 02 that combines own posts + followed accounts' posts). Filter out statuses from muted/blocked accounts in the service layer. Run batch API model assembly.
- **Response:** 200 `[]Status` with `Link` header.

**Caching strategy:** Cache the raw status ID list under `timeline:home:{accountID}` with a 60-second TTL (IMPLEMENTATION 03). On hit, fetch status rows by ID (cheap primary key lookup). On miss, run the full timeline query and cache the result.

### `GET /api/v1/timelines/public`

- **Auth:** Optional.
- **Query params:** Standard pagination + `local` (bool, default false).
- **Logic:** Call `GetPublicTimeline(localOnly, maxID, limit)`. When `local=true`, the partial index `idx_statuses_local_public` is used. Filter muted/blocked accounts if viewer is authenticated.
- **Response:** 200 `[]Status` with `Link` header.

### `GET /api/v1/timelines/tag/:hashtag`

- **Auth:** Optional.
- **Query params:** Standard pagination.
- **Logic:** Normalize tag to lowercase. Call `GetHashtagTimeline(name, maxID, limit)`.
- **Response:** 200 `[]Status` with `Link` header.

---

## 8. Notifications Handlers

**File:** `internal/api/mastodon/notifications.go`

### `GET /api/v1/notifications`

- **Auth:** Required (`read:notifications`).
- **Query params:** Standard pagination + `types[]` (include filter) + `exclude_types[]` (exclude filter).
- **Logic:** Filtered query:


For each notification, resolve `from_id` → Account and `status_id` → Status (using batch lookups). Muted accounts' notifications are excluded in the service layer (unless the mute has `hide_notifications = FALSE`).

- **Response:** 200 `[]Notification` with `Link` header.

### `GET /api/v1/notifications/:id`

- **Auth:** Required.
- **Logic:** Fetch single notification, verify ownership.
- **Response:** 200 `Notification`.

### `POST /api/v1/notifications/clear`

- **Auth:** Required (`write:notifications`).
- **Logic:** `DELETE FROM notifications WHERE account_id = $1`.
- **Response:** 200 `{}`.

### `POST /api/v1/notifications/:id/dismiss`

- **Auth:** Required (`write:notifications`).
- **Logic:** `DELETE FROM notifications WHERE id = $1 AND account_id = $2`.
- **Response:** 200 `{}`.

---

## 9. Search Handler

**File:** `internal/api/mastodon/search.go`

### `GET /api/v2/search`

- **Auth:** Optional (required for remote account resolution).
- **Query params:** `q` (required), `type` (`accounts`|`statuses`|`hashtags`, optional — search all if omitted), `resolve` (bool, attempt WebFinger if true), `limit` (default 5, max 40).

**Response:** 200


**Account search:**


Local accounts are ranked first. If `q` matches the `user@domain` pattern and `resolve=true`, attempt a WebFinger lookup → fetch the remote actor document → upsert the account → include in results.

**Hashtag search:**


**Statuses search:** Return empty array in Phase 1. Phase 2 adds `tsvector` full-text search.

---

## 10. Instance Handler

**File:** `internal/api/mastodon/instance.go`

### `GET /api/v2/instance`

- **Auth:** None.
- **Logic:** Read all values from `instance_settings` (cached 5 min). Assemble the `Instance` response. `version` follows the format `"0.1.0 (compatible; Monstera-fed)"` — Mastodon clients parse the version string to detect capabilities.
- **Configuration sub-object** includes:
  - `statuses.max_characters`: from `max_status_chars` setting.
  - `statuses.max_media_attachments`: 4.
  - `media_attachments.supported_mime_types`: keys of `media.AllowedContentTypes`.
  - `media_attachments.image_size_limit` / `video_size_limit`: from `media_max_bytes` setting.
- **Response:** 200 `Instance`.

### `GET /api/v1/custom_emojis`

- **Auth:** None.
- **Logic:** Return `[]` (empty array) in Phase 1. Phase 2 implements custom emoji CRUD.
- **Response:** 200 `[]`.

---

## 11. Streaming Handlers

SSE streaming endpoints (`/api/v1/streaming/*`) are detailed in Design Prompt 09. The route registrations are included in §13 below for completeness.

`GET /api/v1/streaming/health` returns 200 `OK` (plain text, no JSON) — a trivial handler used by clients to test streaming endpoint reachability before opening an SSE connection.

---

## 12. Schema Addenda

### New Migrations

| # | File prefix | Description |
|---|-------------|-------------|
| 25 | `000025_add_account_counters` | `ALTER TABLE accounts ADD COLUMN followers_count INT NOT NULL DEFAULT 0, ADD COLUMN following_count INT NOT NULL DEFAULT 0, ADD COLUMN statuses_count INT NOT NULL DEFAULT 0` |
| 26 | `000026_create_status_mentions` | `status_mentions (status_id TEXT REFERENCES statuses(id) ON DELETE CASCADE, account_id TEXT REFERENCES accounts(id), PRIMARY KEY (status_id, account_id))` + index on `account_id` |
| 27 | `000027_add_in_reply_to_account_id` | `ALTER TABLE statuses ADD COLUMN in_reply_to_account_id TEXT REFERENCES accounts(id)` |
| 28 | `000028_add_user_preferences` | `ALTER TABLE users ADD COLUMN default_privacy TEXT NOT NULL DEFAULT 'public', ADD COLUMN default_sensitive BOOLEAN NOT NULL DEFAULT FALSE, ADD COLUMN default_language TEXT NOT NULL DEFAULT ''` |
| 29 | `000029_add_account_fields` | `ALTER TABLE accounts ADD COLUMN fields JSONB` |

### New Queries

**`accounts.sql` additions:**
- `IncrementFollowersCount`, `DecrementFollowersCount`, `IncrementFollowingCount`, `DecrementFollowingCount`, `IncrementStatusesCount`, `DecrementStatusesCount` — all `:exec`, same pattern as status counter queries.
- `GetBatchRelationships` — see §4.
- `SearchAccounts` — see §9.

**New file `status_mentions.sql`:**
- `CreateStatusMention :exec` — insert `(status_id, account_id)`.
- `GetStatusMentions :many` — for a single status.
- `GetBatchStatusMentions :many` — for a list of status IDs (see §4).

**`statuses.sql` additions:**
- `GetBatchStatusInteractions` — see §4.
- `GetHomeTimelineForward :many` — `ORDER BY id ASC` variant for `min_id` pagination.
- `GetPublicTimelineForward :many` — same.
- `GetAccountStatusesFiltered :many` — supports `only_media`, `exclude_replies`, `exclude_reblogs` flags via conditional WHERE clauses.
- `GetRebloggedBy :many` — accounts that reblogged a given status.

**`media.sql` additions:**
- `GetBatchStatusAttachments :many` — see §4.

**`notifications.sql` additions:**
- `ListNotificationsFiltered :many` — see §8.

**`hashtags.sql` additions:**
- `SearchHashtagsByPrefix :many` — see §9.
- `GetBatchStatusHashtags :many` — see §4.

### Store interface additions

New sub-interfaces or additions to existing ones:

- `AccountStore` gains: counter increment/decrement methods, `SearchAccounts`, `GetBatchRelationships`.
- `StatusStore` gains: `GetBatchStatusInteractions`, forward-pagination variants, `GetAccountStatusesFiltered`, `GetRebloggedBy`.
- New `MentionStore` interface: `CreateStatusMention`, `GetStatusMentions`, `GetBatchStatusMentions`.
- `NotificationStore` gains: `ListNotificationsFiltered`.
- `HashtagStore` gains: `SearchHashtagsByPrefix`, `GetBatchStatusHashtags`.
- `AttachmentStore` gains: `GetBatchStatusAttachments`.

---

## 13. Route Registration


`mw` is shorthand for `middleware.RequiredScopes`.

**Note on OptionalAuth endpoints:** `GET /api/v1/accounts/:id`, `GET /api/v1/statuses/:id`, `GET /api/v1/statuses/:id/context`, `GET /api/v1/timelines/public`, `GET /api/v1/timelines/tag/:hashtag`, and `GET /api/v2/search` should ideally use OptionalAuth to serve both authenticated and anonymous requests. The route grouping above places them under RequireAuth for simplicity; an alternative is to use two groups or override auth per-route. The recommendation is to split the `/api/v1` group into `authed` and `optionalAuth` sub-groups, with the read-only public-facing endpoints in the optional group.

---

## 14. `go.mod` Additions

```
require (
    github.com/microcosm-cc/bluemonday   v1.x.x   // HTML sanitization
    mvdan.cc/xurls/v2                     v2.x.x   // URL detection
)
```
