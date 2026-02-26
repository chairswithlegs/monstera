# ADR 08 — Mastodon REST API Handlers

> **Status:** Draft v0.1 — Feb 24, 2026  
> **Addresses:** `design-prompts/08-mastodon-rest-api.md`

---

## Design Decisions

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
├── media.go            — (already designed in ADR 04)
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

```go
type Account struct {
    ID             string    `json:"id"`
    Username       string    `json:"username"`
    Acct           string    `json:"acct"`
    DisplayName    string    `json:"display_name"`
    Locked         bool      `json:"locked"`
    Bot            bool      `json:"bot"`
    CreatedAt      string    `json:"created_at"`       // ISO 8601
    Note           string    `json:"note"`              // HTML
    URL            string    `json:"url"`
    Avatar         string    `json:"avatar"`
    AvatarStatic   string    `json:"avatar_static"`
    Header         string    `json:"header"`
    HeaderStatic   string    `json:"header_static"`
    FollowersCount int       `json:"followers_count"`
    FollowingCount int       `json:"following_count"`
    StatusesCount  int       `json:"statuses_count"`
    LastStatusAt   *string   `json:"last_status_at"`    // "YYYY-MM-DD" or null
    Emojis         []any     `json:"emojis"`            // empty array Phase 1
    Fields         []Field   `json:"fields"`
    Source         *Source   `json:"source,omitempty"`  // only verify_credentials
}

type Field struct {
    Name       string  `json:"name"`
    Value      string  `json:"value"`       // HTML (links auto-linked)
    VerifiedAt *string `json:"verified_at"` // null in Phase 1
}

type Source struct {
    Note      string  `json:"note"`       // raw plain text
    Privacy   string  `json:"privacy"`    // default visibility
    Sensitive bool    `json:"sensitive"`
    Language  string  `json:"language"`
    Fields    []Field `json:"fields"`     // raw, not HTML-rendered
}
```

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

```go
type Status struct {
    ID                 string             `json:"id"`
    CreatedAt          string             `json:"created_at"`
    InReplyToID        *string            `json:"in_reply_to_id"`
    InReplyToAccountID *string            `json:"in_reply_to_account_id"`
    Sensitive          bool               `json:"sensitive"`
    SpoilerText        string             `json:"spoiler_text"`
    Visibility         string             `json:"visibility"`
    Language           *string            `json:"language"`
    URI                string             `json:"uri"`
    URL                *string            `json:"url"`
    RepliesCount       int                `json:"replies_count"`
    ReblogsCount       int                `json:"reblogs_count"`
    FavouritesCount    int                `json:"favourites_count"`
    Content            string             `json:"content"`          // HTML
    Reblog             *Status            `json:"reblog"`           // nested or null
    Account            Account            `json:"account"`
    MediaAttachments   []MediaAttachment  `json:"media_attachments"`
    Mentions           []Mention          `json:"mentions"`
    Tags               []Tag              `json:"tags"`
    Emojis             []any              `json:"emojis"`           // empty Phase 1
    Card               *any               `json:"card"`             // null Phase 1
    Poll               *any               `json:"poll"`             // null Phase 1
    Favourited         bool               `json:"favourited"`
    Reblogged          bool               `json:"reblogged"`
    Muted              bool               `json:"muted"`            // false Phase 1
    Bookmarked         bool               `json:"bookmarked"`       // false Phase 1
    Pinned             bool               `json:"pinned"`           // false Phase 1
}

type Mention struct {
    ID       string `json:"id"`
    Username string `json:"username"`
    Acct     string `json:"acct"`
    URL      string `json:"url"`
}

type Tag struct {
    Name string `json:"name"`
    URL  string `json:"url"`
}
```

**Presenter rules:**
- `url`: `https://{INSTANCE_DOMAIN}/@{username}/{id}` for local statuses; `null` for remote.
- `spoiler_text`: `content_warning` column; empty string if NULL.
- `reblog`: if `reblog_of_id` is set, recursively render the original status. The outer status carries the booster's account; the inner `reblog` carries the original author and content.
- `media_attachments`: query `ListStatusAttachments` → map via `toMediaResponse` (ADR 04).
- `mentions`: query `status_mentions` join table → render as `Mention` with account info.
- `tags`: query `GetStatusHashtags` → render as `Tag` with `url: https://{INSTANCE_DOMAIN}/tags/{name}`.
- `favourited` / `reblogged`: populated from the batch interaction query (see §4).

### `Notification`

```go
type Notification struct {
    ID        string   `json:"id"`
    Type      string   `json:"type"`
    CreatedAt string   `json:"created_at"`
    Account   Account  `json:"account"`    // the actor
    Status    *Status  `json:"status"`     // null for follow/follow_request
}
```

### `Relationship`

```go
type Relationship struct {
    ID                  string `json:"id"`
    Following           bool   `json:"following"`
    ShowingReblogs      bool   `json:"showing_reblogs"`      // true Phase 1
    Notifying           bool   `json:"notifying"`            // false Phase 1
    FollowedBy          bool   `json:"followed_by"`
    Blocking            bool   `json:"blocking"`
    BlockedBy           bool   `json:"blocked_by"`
    Muting              bool   `json:"muting"`
    MutingNotifications bool   `json:"muting_notifications"`
    Requested           bool   `json:"requested"`
    DomainBlocking      bool   `json:"domain_blocking"`
    Endorsed            bool   `json:"endorsed"`             // false Phase 1
    Note                string `json:"note"`                 // empty Phase 1
}
```

### `Instance` (v2)

```go
type Instance struct {
    Domain        string              `json:"domain"`
    Title         string              `json:"title"`
    Version       string              `json:"version"`
    SourceURL     string              `json:"source_url"`
    Description   string              `json:"description"`
    Languages     []string            `json:"languages"`
    Configuration InstanceConfig      `json:"configuration"`
    Registrations RegistrationConfig  `json:"registrations"`
    Contact       ContactConfig       `json:"contact"`
    Rules         []Rule              `json:"rules"`
    Thumbnail     *Thumbnail          `json:"thumbnail"`
}
```

Nested types include `InstanceConfig` with `statuses.max_characters`, `media_attachments.supported_mime_types`, `media_attachments.image_size_limit`, etc. All values sourced from `instance_settings` table (cached with 5-minute TTL per ADR 03).

---

## 2. Pagination Design

### `PageParams`

Parsed from query string on every paginated endpoint:

```go
type PageParams struct {
    MaxID   string // items older than this ID
    MinID   string // items immediately newer than this ID (bottom-anchored)
    SinceID string // items newer than this ID (top-anchored)
    Limit   int    // default 20, max 40
}
```

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

```sql
SELECT s.* FROM statuses s
WHERE s.deleted_at IS NULL
  AND s.account_id IN (
      SELECT target_id FROM follows WHERE account_id = $1 AND state = 'accepted'
      UNION ALL SELECT $1::text
  )
  AND ($2::text IS NULL OR s.id < $2)       -- max_id
  AND ($3::text IS NULL OR s.id > $3)       -- since_id / min_id
ORDER BY s.id DESC
LIMIT $4;
```

For `min_id`, the service layer calls a `*Forward` variant that uses `ORDER BY s.id ASC`, then reverses.

---

## 3. Content Rendering Pipeline

**File:** `internal/service/content.go`

### Interface

```go
type RenderResult struct {
    HTML     string
    Mentions []MentionRef  // {username, domain, accountID}
    Tags     []string      // normalized lowercase hashtag names
}

func Render(text string, resolve MentionResolver) (RenderResult, error)
```

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

```sql
-- name: GetBatchStatusInteractions :many
SELECT
    s.id AS status_id,
    EXISTS(SELECT 1 FROM favourites WHERE account_id = $1 AND status_id = s.id) AS favourited,
    EXISTS(SELECT 1 FROM statuses r WHERE r.account_id = $1 AND r.reblog_of_id = s.id
           AND r.deleted_at IS NULL) AS reblogged
FROM unnest($2::text[]) AS s(id);
```

The API model layer merges these flags into each `Status` response. If the viewer is unauthenticated, all flags default to `false` (skip the query entirely).

### Account relationships

For `GET /api/v1/accounts/relationships?id[]=...` and for enriching status responses with block/mute awareness:

```sql
-- name: GetBatchRelationships :many
SELECT
    a.id AS target_id,
    EXISTS(SELECT 1 FROM follows WHERE account_id = $1 AND target_id = a.id AND state = 'accepted') AS following,
    EXISTS(SELECT 1 FROM follows WHERE account_id = a.id AND target_id = $1 AND state = 'accepted') AS followed_by,
    EXISTS(SELECT 1 FROM blocks WHERE account_id = $1 AND target_id = a.id) AS blocking,
    EXISTS(SELECT 1 FROM blocks WHERE account_id = a.id AND target_id = $1) AS blocked_by,
    EXISTS(SELECT 1 FROM mutes WHERE account_id = $1 AND target_id = a.id) AS muting,
    COALESCE((SELECT hide_notifications FROM mutes WHERE account_id = $1 AND target_id = a.id), FALSE) AS muting_notifications,
    EXISTS(SELECT 1 FROM follows WHERE account_id = $1 AND target_id = a.id AND state = 'pending') AS requested,
    COALESCE(a.domain IS NOT NULL AND EXISTS(SELECT 1 FROM domain_blocks WHERE domain = a.domain), FALSE) AS domain_blocking
FROM unnest($2::text[]) WITH ORDINALITY AS input(id, ord)
JOIN accounts a ON a.id = input.id
ORDER BY input.ord;
```

### Batch mentions and tags

For a list of status IDs, fetch all mentions and tags in two queries:

```sql
-- name: GetBatchStatusMentions :many
SELECT sm.status_id, a.id, a.username, a.domain, a.ap_id
FROM status_mentions sm
JOIN accounts a ON a.id = sm.account_id
WHERE sm.status_id = ANY($1::text[]);

-- name: GetBatchStatusHashtags :many
SELECT sh.status_id, h.name
FROM status_hashtags sh
JOIN hashtags h ON h.id = sh.hashtag_id
WHERE sh.status_id = ANY($1::text[]);
```

The API model layer groups results by `status_id` and merges into each Status response.

### Batch media attachments

```sql
-- name: GetBatchStatusAttachments :many
SELECT * FROM media_attachments
WHERE status_id = ANY($1::text[])
ORDER BY status_id, id ASC;
```

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

```go
type AccountsHandler struct {
    accounts *service.AccountService
    logger   *slog.Logger
    domain   string  // INSTANCE_DOMAIN
}
```

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

The recursive CTEs (already defined in ADR 02) walk `in_reply_to_id` in both directions. Ancestors are ordered oldest-first (ASC); descendants are ordered oldest-first (ASC) for threading.

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
- **Logic:** Call `GetHomeTimeline` (the UNION ALL query from ADR 02 that combines own posts + followed accounts' posts). Filter out statuses from muted/blocked accounts in the service layer. Run batch API model assembly.
- **Response:** 200 `[]Status` with `Link` header.

**Caching strategy:** Cache the raw status ID list under `timeline:home:{accountID}` with a 60-second TTL (ADR 03). On hit, fetch status rows by ID (cheap primary key lookup). On miss, run the full timeline query and cache the result.

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

```sql
-- name: ListNotificationsFiltered :many
SELECT * FROM notifications
WHERE account_id = $1
  AND ($2::text IS NULL OR id < $2)
  AND ($3::text[] IS NULL OR type = ANY($3))
  AND ($4::text[] IS NULL OR NOT (type = ANY($4)))
ORDER BY id DESC
LIMIT $5;
```

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

```json
{
  "accounts": [/* Account */],
  "statuses": [],
  "hashtags": [/* Tag with history */]
}
```

**Account search:**

```sql
-- name: SearchAccounts :many
SELECT * FROM accounts
WHERE suspended = FALSE
  AND (username ILIKE '%' || $1 || '%' OR display_name ILIKE '%' || $1 || '%')
ORDER BY
    CASE WHEN domain IS NULL THEN 0 ELSE 1 END,
    username ASC
LIMIT $2;
```

Local accounts are ranked first. If `q` matches the `user@domain` pattern and `resolve=true`, attempt a WebFinger lookup → fetch the remote actor document → upsert the account → include in results.

**Hashtag search:**

```sql
-- name: SearchHashtagsByPrefix :many
SELECT * FROM hashtags WHERE name LIKE lower($1) || '%' ORDER BY name ASC LIMIT $2;
```

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

```go
// In NewRouter — public endpoints (no auth):
r.Get("/api/v2/instance", instanceHandler.GetInstance)
r.Get("/api/v1/custom_emojis", instanceHandler.CustomEmojis)

// Streaming — OptionalAuth (token can come via query param):
r.Route("/api/v1/streaming", func(r chi.Router) {
    r.Use(middleware.OptionalAuth(oauthServer, accountStore))
    r.Get("/health", streamingHandler.Health)
    r.Get("/user", streamingHandler.User)       // RequireAuth checked inside
    r.Get("/public", streamingHandler.Public)
    r.Get("/public/local", streamingHandler.PublicLocal)
    r.Get("/hashtag", streamingHandler.Hashtag)
})

// Authenticated Mastodon API:
r.Route("/api/v1", func(r chi.Router) {
    r.Use(middleware.RequireAuth(oauthServer, accountStore))

    // Accounts — static paths before parameterized
    r.With(mw("read:accounts")).Get("/accounts/verify_credentials", accounts.VerifyCredentials)
    r.With(mw("write:accounts")).Patch("/accounts/update_credentials", accounts.UpdateCredentials)
    r.With(mw("read:follows")).Get("/accounts/relationships", accounts.Relationships)

    // Accounts — parameterized
    r.Route("/accounts/{id}", func(r chi.Router) {
        r.With(mw("read:accounts")).Get("/", accounts.Get)
        r.With(mw("read:statuses")).Get("/statuses", accounts.Statuses)
        r.With(mw("read:accounts")).Get("/followers", accounts.Followers)
        r.With(mw("read:accounts")).Get("/following", accounts.Following)
        r.With(mw("write:follows")).Post("/follow", accounts.Follow)
        r.With(mw("write:follows")).Post("/unfollow", accounts.Unfollow)
        r.With(mw("write:blocks")).Post("/block", accounts.Block)
        r.With(mw("write:blocks")).Post("/unblock", accounts.Unblock)
        r.With(mw("write:mutes")).Post("/mute", accounts.Mute)
        r.With(mw("write:mutes")).Post("/unmute", accounts.Unmute)
    })

    // Statuses
    r.With(mw("write:statuses")).Post("/statuses", statuses.Create)
    r.Route("/statuses/{id}", func(r chi.Router) {
        r.With(mw("read:statuses")).Get("/", statuses.Get)
        r.With(mw("write:statuses")).Delete("/", statuses.Delete)
        r.With(mw("write:statuses")).Post("/reblog", statuses.Reblog)
        r.With(mw("write:statuses")).Post("/unreblog", statuses.Unreblog)
        r.With(mw("write:favourites")).Post("/favourite", statuses.Favourite)
        r.With(mw("write:favourites")).Post("/unfavourite", statuses.Unfavourite)
        r.With(mw("read:statuses")).Get("/context", statuses.Context)
        r.With(mw("read:statuses")).Get("/favourited_by", statuses.FavouritedBy)
        r.With(mw("read:statuses")).Get("/reblogged_by", statuses.RebloggedBy)
    })

    // Timelines
    r.With(mw("read:statuses")).Get("/timelines/home", timelines.Home)
    r.With(mw("read:statuses")).Get("/timelines/public", timelines.Public)
    r.With(mw("read:statuses")).Get("/timelines/tag/{hashtag}", timelines.Hashtag)

    // Notifications
    r.With(mw("read:notifications")).Get("/notifications", notifications.List)
    r.With(mw("read:notifications")).Get("/notifications/{id}", notifications.Get)
    r.With(mw("write:notifications")).Post("/notifications/clear", notifications.Clear)
    r.With(mw("write:notifications")).Post("/notifications/{id}/dismiss", notifications.Dismiss)

    // Media
    r.With(mw("write:media")).Get("/media/{id}", mediaHandler.Get)
    r.With(mw("write:media")).Put("/media/{id}", mediaHandler.UpdateDescription)
})

// v2 endpoints:
r.Route("/api/v2", func(r chi.Router) {
    r.Use(middleware.RequireAuth(oauthServer, accountStore))
    r.With(mw("write:media")).Post("/media", mediaHandler.Upload)
    r.With(mw("read:search")).Get("/search", search.Search)
})
```

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

---

## 15. Open Questions

| # | Question | Recommendation | Impact |
|---|----------|---------------|--------|
| 1 | **OptionalAuth route split** — several GET endpoints need to work both authenticated and unauthenticated (account lookup, public timelines, status detail). The current RequireAuth grouping blocks anonymous access. | Split `/api/v1` routes into two groups: one with RequireAuth (write operations + home timeline + notifications), one with OptionalAuth (read-only public endpoints). This matches Mastodon's behavior. | Medium — affects route registration; no service-layer impact. |
| 2 | **Home timeline cache invalidation** — the 60s TTL means a new post from a followed account may not appear for up to a minute. SSE-connected clients get it immediately, but REST-polling clients see staleness. | Accept the 60s staleness. Mastodon clients that support streaming get real-time updates; the REST fallback is for reconnection backfill. If needed, invalidation on new follow/unfollow (delete the cache entry) can be added. | Low — UX concern only for non-streaming clients. |
| 3 | **Profile field verification** — Mastodon verifies profile link fields by checking for a `rel="me"` backlink. Phase 1 always returns `verified_at: null`. | Defer verification to Phase 2. The JSONB schema supports `verified_at` already. | Low — cosmetic. |
| 4 | **Character counting for CJK** — Mastodon counts CJK characters as 1 character each (not by byte length). The character count validation should use `utf8.RuneCountInString` after stripping URLs and mentions. | Use rune count. URLs count as 23 regardless of actual length. | Low — straightforward to implement. |
| 5 | **`GET /api/v1/accounts/:id/statuses` filter complexity** — the combination of `only_media`, `exclude_replies`, `exclude_reblogs` flags generates many query variants. | Single query with conditional WHERE clauses via sqlc `CASE`/`AND` patterns. The partial indexes on `statuses` cover the hot paths. If sqlc cannot express the conditionals cleanly, fall back to 2-3 named query variants. | Low — implementation detail. |

---

*End of ADR 08 — Mastodon REST API Handlers*
