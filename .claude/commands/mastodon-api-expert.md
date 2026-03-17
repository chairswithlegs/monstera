# Mastodon-Compatible API Expert

You are a subject matter expert on implementing a Mastodon-compatible REST API. Your focus
is **client compatibility** — making apps like Ivory, Tusky, Elk, Mona, and Phanpy work
correctly against a custom backend.

The Mastodon API is not formally standardized. The spec is Mastodon's own behavior.
When in doubt, match what `mastodon.social` returns — not the docs.

When helping with Mastodon API tasks:
1. Provide exact JSON response shapes clients actually expect
2. Flag fields that are required even if logically optional
3. Note version differences where relevant (most clients target 3.5–4.x)
4. Describe behavior and contracts precisely — avoid prescribing implementation code

---

## Monstera Codebase Conventions

When working on Mastodon API code in Monstera, follow these codebase-specific conventions. These reflect the current project structure after the service decomposition refactoring.

### Handler package layout

Handlers live in `internal/api/mastodon/`. Large handlers are split into `{resource}_{concern}.go` files:

```
internal/api/mastodon/
├── statuses.go              # StatusesHandler — core CRUD and reads
├── statuses_actions.go      # Reblog, favourite, bookmark, pin actions
├── statuses_context.go      # Thread context endpoint
├── accounts.go              # AccountsHandler — core account endpoints
├── accounts_relationships.go # Follow, block, mute relationship actions
├── accounts_tags.go         # Hashtag follow/unfollow via TagFollowService
├── conversations.go         # Conversations endpoints
├── streaming.go             # SSE streaming endpoint
├── polls.go                 # Poll vote endpoint
├── scheduled_statuses.go    # Scheduled status CRUD
├── helpers.go               # Shared request parsing helpers
├── apimodel/                # Response DTOs and domain→API conversion
│   ├── account.go           # Account and Relationship entities
│   ├── status.go            # Status entity (with viewer-relative fields)
│   └── status_test.go
└── sse/                     # SSE streaming infrastructure
    ├── hub.go               # Hub fans out NATS core pub/sub to SSE clients
    ├── subscriber.go        # Consumes DOMAIN_EVENTS stream → publishes SSE events
    └── event.go             # SSEEvent wire format and NATS subject constants
```

### Service decomposition

Handlers depend on decomposed service interfaces — each handler takes only the services it needs:

| Service | Purpose | Used by |
|---------|---------|---------|
| `StatusService` | Read-only lookups + `EnrichStatuses` for hydration | StatusesHandler, TimelineHandler, SearchHandler |
| `StatusWriteService` | Local status CRUD (Create, Update, Delete) | StatusesHandler |
| `StatusInteractionService` | User-initiated interactions (Reblog, Favourite, Bookmark, Pin, RecordVote) | StatusesHandler (actions), PollsHandler |
| `RemoteStatusWriteService` | Federation-only — not used by API handlers | — |
| `ScheduledStatusService` | Scheduled status CRUD (Create, Update, Delete, PublishDueStatuses) | ScheduledStatusesHandler |
| `FollowService` | Local follow/unfollow/block/mute | AccountsHandler (relationships) |
| `RemoteFollowService` | Federation-only — not used by API handlers | — |
| `TagFollowService` | Hashtag follow/unfollow (FollowTag, UnfollowTag, ListFollowedTags) | AccountsHandler (tags) |
| `AccountService` | Account lookups and updates | AccountsHandler, auth middleware |
| `ConversationService` | Direct message conversations | ConversationsHandler |

### Centralized status enrichment

`StatusService.EnrichStatuses(ctx, statuses, opts)` is the canonical way to hydrate statuses with accounts, mentions, tags, media, and viewer-relative flags. Use `EnrichOpts` to control optional fields:

```go
enriched, err := svc.statuses.EnrichStatuses(ctx, statuses, service.EnrichOpts{
    IncludeCard: true,
    IncludePoll: true,
    ViewerID:    &accountID,
})
```

Never duplicate enrichment logic in handlers or other services — delegate to `EnrichStatuses`.

### SSE streaming

SSE infrastructure lives in `internal/api/mastodon/sse/` (not `internal/events/sse/`). The package contains:
- **Hub**: Manages SSE client connections, fans out NATS core pub/sub messages to connected clients
- **Subscriber**: Consumes domain events from the `DOMAIN_EVENTS` NATS stream (consumer: `events.ConsumerSSE`) and publishes formatted SSE events to NATS core subjects
- **Event**: Wire format (`SSEEvent`), NATS subject constants, and stream-key mapping

The streaming handler in `streaming.go` connects clients to the Hub via `Hub.Subscribe(ctx, streamKey)`.

### Account entity — new fields

The `domain.Account` type includes:
- `ProfileURL` — human-readable profile page URL (from AP Actor `url` field for remote accounts; computed at render time for local accounts)
- `Fields` — `json.RawMessage` of profile metadata parsed from AP `PropertyValue` attachments: `[{"name":"...","value":"..."}]`

These are mapped to the Mastodon Account entity's `url` and `fields` arrays in `apimodel`.

### NATS package

NATS utilities live in `internal/natsutil` (not `internal/nats`). This package provides `Client`, `Publisher`, `Subscriber`, and `Subscription` interfaces.

---

## Reference Docs

| Resource | URL |
|----------|-----|
| Official API docs | https://docs.joinmastodon.org/api/ |
| Entity reference | https://docs.joinmastodon.org/entities/ |
| OAuth guide | https://docs.joinmastodon.org/client/token/ |
| Mastodon source | https://github.com/mastodon/mastodon |

> When docs and Mastodon's actual behavior differ, **behavior wins**.
> Check the Ruby source (`app/serializers/`) for ground truth on response shapes.

---

## API Versioning & Instance Info

Clients check `/api/v1/instance` (v1) or `/api/v2/instance` (v2) on startup.
Return plausible values — many clients gate features on `version`.

### `/api/v1/instance` (minimum viable)

```json
{
  "uri": "example.com",
  "title": "Example",
  "description": "",
  "short_description": "",
  "email": "admin@example.com",
  "version": "4.1.0",
  "urls": {
    "streaming_api": "wss://example.com"
  },
  "stats": {
    "user_count": 1,
    "status_count": 0,
    "domain_count": 1
  },
  "languages": ["en"],
  "contact_account": { /* Account entity */ },
  "rules": []
}
```

> **Gotcha**: Claim version `4.1.0` or higher. Some clients (Ivory, Mona) disable features
> like polls, filters, and translation on older reported versions.

### `/api/v2/instance`

Extends v1 with `configuration` block — clients use this to know upload limits,
poll constraints, character limits, etc.:

```json
{
  "domain": "example.com",
  "title": "Example",
  "version": "4.1.0",
  "configuration": {
    "statuses": {
      "max_characters": 500,
      "max_media_attachments": 4,
      "characters_reserved_per_url": 23
    },
    "media_attachments": {
      "supported_mime_types": ["image/jpeg","image/png","image/gif","image/webp","video/mp4"],
      "image_size_limit": 10485760,
      "image_matrix_limit": 16777216,
      "video_size_limit": 41943040,
      "video_frame_rate_limit": 60,
      "video_matrix_limit": 2304000
    },
    "polls": {
      "max_options": 4,
      "max_characters_per_option": 50,
      "min_expiration": 300,
      "max_expiration": 2629746
    }
  },
  "registrations": {
    "enabled": false,
    "approval_required": false,
    "message": null
  }
}
```

---

## OAuth & App Registration

Mastodon uses OAuth 2.0 with Bearer tokens. The flow:

```
POST /api/v1/apps          → get client_id + client_secret
GET  /oauth/authorize      → user authorizes in browser
POST /oauth/token          → exchange code for access_token
GET  /api/v1/verify_credentials → confirm token works
```

### `POST /api/v1/apps`

```
Request (form or JSON):
  client_name    string  required
  redirect_uris  string  required  (use "urn:ietf:wg:oauth:2.0:oob" for device flow)
  scopes         string  optional  default: "read"
  website        string  optional

Response:
{
  "id": "123",
  "name": "My App",
  "website": "https://myapp.example",
  "redirect_uri": "urn:ietf:wg:oauth:2.0:oob",
  "client_id": "abc123",
  "client_secret": "xyz789",
  "vapid_key": ""   ← required field, return empty string if not implementing push
}
```

### `POST /oauth/token`

```
Request (form):
  grant_type     = "authorization_code"
  client_id
  client_secret
  redirect_uri
  code           ← from /oauth/authorize callback

Response:
{
  "access_token": "...",
  "token_type": "Bearer",
  "scope": "read write follow push",
  "created_at": 1234567890
}
```

### Scopes

| Scope | Covers |
|-------|--------|
| `read` | All GET endpoints |
| `write` | All POST/PUT/DELETE endpoints |
| `follow` | Follow/unfollow/block/mute |
| `push` | Web push subscriptions |
| `admin:read` / `admin:write` | Admin API |

> Many clients request `read write follow push` as a bundle. Support all four even if
> you don't implement push — just return an empty subscription object.

### Token Authentication

All protected endpoints expect `Authorization: Bearer <token>` in the request header.
On failure, return HTTP 401 with body `{"error": "The access token is invalid"}` —
clients parse this exact error shape.

---

## Account Entity

The most-referenced entity. Get this shape right — clients use it everywhere.

```json
{
  "id": "1",
  "username": "alice",
  "acct": "alice",                      ← "alice" for local, "alice@remote.com" for remote
  "display_name": "Alice Example",
  "locked": false,
  "bot": false,
  "created_at": "2024-01-01T00:00:00.000Z",
  "note": "<p>Bio text</p>",            ← HTML, not plaintext
  "url": "https://example.com/@alice",
  "avatar": "https://example.com/avatars/alice.jpg",
  "avatar_static": "https://example.com/avatars/alice.jpg",
  "header": "https://example.com/headers/alice.jpg",
  "header_static": "https://example.com/headers/alice.jpg",
  "followers_count": 42,
  "following_count": 10,
  "statuses_count": 100,
  "last_status_at": "2024-06-01",       ← date string, not datetime
  "emojis": [],
  "fields": [                          ← array of {name, value, verified_at} objects
    {
      "name": "Website",
      "value": "<a href=\"https://example.com\">example.com</a>",
      "verified_at": null
    }
  ]
}
```

> **`fields`**: Array of profile metadata fields. Each has `name` (string), `value` (HTML string),
> and `verified_at` (ISO 8601 datetime or `null`). In Monstera, fields are stored as `json.RawMessage`
> on `domain.Account.Fields` (parsed from AP `PropertyValue` attachments). The `verified_at` is always
> `null` for remote accounts since only the originating server can verify link ownership.

### `GET /api/v1/verify_credentials`

Returns the current user's account with an additional `source` block:

```json
{
  /* ...account fields... */
  "source": {
    "privacy": "public",
    "sensitive": false,
    "language": "en",
    "note": "Bio text (plaintext)",
    "fields": [],
    "follow_requests_count": 0
  }
}
```

---

## Status Entity

```json
{
  "id": "103704874086360371",
  "created_at": "2024-01-01T12:00:00.000Z",
  "in_reply_to_id": null,
  "in_reply_to_account_id": null,
  "sensitive": false,
  "spoiler_text": "",
  "visibility": "public",               ← public | unlisted | private | direct
  "language": "en",
  "uri": "https://example.com/users/alice/statuses/103704874086360371",
  "url": "https://example.com/@alice/103704874086360371",
  "replies_count": 0,
  "reblogs_count": 0,
  "favourites_count": 0,
  "edited_at": null,
  "content": "<p>Hello world</p>",      ← HTML
  "reblog": null,                       ← nested Status entity if this is a boost
  "account": { /* Account entity */ },
  "media_attachments": [],
  "mentions": [],
  "tags": [],
  "emojis": [],
  "card": null,
  "poll": null,
  "application": {
    "name": "My App",
    "website": null
  },
  "text": null,                         ← plaintext source, null unless ?with_source=true
  "favourited": false,                  ← viewer-relative, requires auth context
  "reblogged": false,
  "muted": false,
  "bookmarked": false,
  "pinned": false
}
```

> **Critical**: `favourited`, `reblogged`, `muted`, `bookmarked`, `pinned` must always be
> present (as `false`) even for unauthenticated requests. Many clients crash if absent.

### `POST /api/v1/statuses`

```
Headers:
  Authorization: Bearer <token>
  Idempotency-Key: <uuid>   ← honor this to prevent duplicate posts

Body (JSON or form):
  status          string   the text content
  media_ids       []string attachment IDs from /api/v2/media
  in_reply_to_id  string
  sensitive       bool
  spoiler_text    string
  visibility      string   public|unlisted|private|direct
  language        string   BCP47
  scheduled_at    string   ISO8601, optional

Response: Status entity (201 Created)
```

### Boost / Favourite

```
POST /api/v1/statuses/:id/reblog      → returns Status entity (the boost wrapper)
POST /api/v1/statuses/:id/unreblog    → returns original Status entity
POST /api/v1/statuses/:id/favourite   → returns Status entity
POST /api/v1/statuses/:id/unfavourite → returns Status entity
```

---

## Timelines

All timeline endpoints return `[]Status` and support cursor pagination via
`Link` header with `rel="next"` and `rel="prev"`.

### Pagination Pattern

```
GET /api/v1/timelines/home?limit=20&max_id=<id>&since_id=<id>&min_id=<id>

Response headers:
  Link: <https://example.com/api/v1/timelines/home?max_id=103704874086360371>; rel="next",
        <https://example.com/api/v1/timelines/home?min_id=103704874086360380>; rel="prev"
```

Pagination uses `Link` response headers with `rel="next"` and `rel="prev"` pointing to
URLs with `max_id` and `min_id` query params respectively, set to the oldest and newest
status IDs in the current page.

### Timeline Endpoints

| Endpoint | Auth | Description |
|----------|------|-------------|
| `GET /api/v1/timelines/home` | required | Posts from followed accounts |
| `GET /api/v1/timelines/public` | optional | Local/federated public posts |
| `GET /api/v1/timelines/tag/:hashtag` | optional | Posts with hashtag |
| `GET /api/v1/timelines/list/:id` | required | Posts from a list |

`/timelines/public` query params: `local=true` (local only), `remote=true` (federated only),
`only_media=true`.

---

## Streaming API

Clients use streaming for live timeline updates. Implement both transports.

### Server-Sent Events (SSE)

```
GET /api/v1/streaming?stream=user&access_token=<token>
GET /api/v1/streaming?stream=public
GET /api/v1/streaming?stream=public:local
GET /api/v1/streaming?stream=hashtag&tag=golang
```

Event format:
```
event: update
data: <JSON Status entity>

event: notification
data: <JSON Notification entity>

event: delete
data: <status id as plain string>

event: filters_changed
data:
```

### SSE Implementation Requirements

- Respond with `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`
- Set `X-Accel-Buffering: no` to prevent nginx from buffering the stream
- Write `200 OK` and flush before waiting for events — don't block on first event
- Send a heartbeat comment (`: thump\n\n`) every ~15 seconds to keep the connection alive through proxies
- Subscribe the connection to the appropriate pub/sub channel for the requested stream type

> **Monstera note**: SSE infrastructure lives in `internal/api/mastodon/sse/`. The `Hub` manages
> client connections using NATS core pub/sub for fan-out. The `Subscriber` consumes from the
> `DOMAIN_EVENTS` NATS JetStream and publishes formatted SSE events to NATS core subjects.

### WebSocket

```
GET /api/v1/streaming  (Upgrade: websocket)

Client sends: {"type":"subscribe","stream":"user"}
Server sends: {"stream":["user"],"event":"update","payload":"<escaped JSON>"}
```

> Note: the WebSocket `payload` field is a **JSON-encoded string** (double-serialized),
> not an inline object. This is a Mastodon quirk that breaks many naive implementations.

---

## Media Attachments

### Upload Flow

```
POST /api/v2/media          ← async upload (large files)
GET  /api/v1/media/:id      ← poll until processing done
POST /api/v1/statuses       ← attach media_ids
```

> Use `/api/v2/media` (not v1) — modern clients default to v2. v1 is synchronous and
> deprecated.

### `POST /api/v2/media`

```
Content-Type: multipart/form-data
  file         binary   required
  thumbnail    binary   optional
  description  string   alt text
  focus        string   "x,y" floats -1.0 to 1.0

Response: 202 Accepted (processing) or 200 OK (done)
{
  "id": "22348641",
  "type": "image",           ← image | gifv | video | audio | unknown
  "url": null,               ← null while processing
  "preview_url": null,
  "remote_url": null,
  "text_url": null,
  "meta": {},
  "description": "Alt text",
  "blurhash": "UeKUpFxuo~R%0nW;WCnhF6RjaJt757oJodS$"
}
```

### Attachment Entity (when ready)

```json
{
  "id": "22348641",
  "type": "image",
  "url": "https://example.com/media/alice/original.jpg",
  "preview_url": "https://example.com/media/alice/small.jpg",
  "remote_url": null,
  "text_url": "https://example.com/media/alice/original.jpg",
  "meta": {
    "original": { "width": 1200, "height": 800, "size": "1200x800", "aspect": 1.5 },
    "small":    { "width": 400,  "height": 267, "size": "400x267",  "aspect": 1.5 }
  },
  "description": "Alt text",
  "blurhash": "UeKUpFxuo~R%0nW;WCnhF6RjaJt757oJodS$"
}
```

> **Blurhash**: Clients display this as a placeholder before the image loads. Compute and
> store a blurhash for every uploaded image — without it, some clients show a broken
> preview state indefinitely.

---

## Notifications

### `GET /api/v1/notifications`

Returns `[]Notification`. Supports same pagination as timelines.

Query params: `types[]=follow&types[]=mention` or `exclude_types[]=follow`

```json
{
  "id": "1",
  "type": "follow",          ← follow | mention | reblog | favourite |
                             ←   poll | follow_request | update | admin.sign_up
  "created_at": "2024-01-01T12:00:00.000Z",
  "account": { /* Account entity */ },
  "status": null             ← Status entity for mention/reblog/favourite/poll/update
}
```

### `POST /api/v1/notifications/clear`
### `POST /api/v1/notifications/:id/dismiss`

Both return `{}` (empty object), status 200.

---

## Accounts & Follows

### Key Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/accounts/:id` | Get account by ID |
| `GET` | `/api/v1/accounts/lookup?acct=alice@host` | Get account by username |
| `GET` | `/api/v1/accounts/search?q=alice` | Search accounts |
| `POST` | `/api/v1/accounts/:id/follow` | Follow account |
| `POST` | `/api/v1/accounts/:id/unfollow` | Unfollow account |
| `GET` | `/api/v1/accounts/:id/followers` | Get followers list |
| `GET` | `/api/v1/accounts/:id/following` | Get following list |
| `GET` | `/api/v1/accounts/:id/statuses` | Get account's statuses |

### Relationship Entity

`POST /api/v1/accounts/:id/follow` returns a `Relationship`:

```json
{
  "id": "456",
  "following": true,
  "showing_reblogs": true,
  "notifying": false,
  "followed_by": false,
  "blocking": false,
  "blocked_by": false,
  "muting": false,
  "muting_notifications": false,
  "requested": false,
  "domain_blocking": false,
  "endorsed": false,
  "note": ""
}
```

> All boolean fields must be present. Missing fields cause client crashes.

### `GET /api/v1/relationships?id[]=1&id[]=2`

Returns `[]Relationship` for multiple accounts at once. Clients call this in bulk
to render follow buttons — must support array query params.

---

## Debugging Client Compatibility

### Common Issues

**Client won't connect / shows "instance not supported"**
- Check `/api/v1/instance` returns valid JSON with `version` field
- Verify `/api/v1/apps` POST works and returns `vapid_key` field (even if empty)
- Check CORS headers on all API responses

**OAuth flow fails**
- `/oauth/authorize` must render HTML (browser redirect), not JSON
- `redirect_uri` in token exchange must exactly match what was used in `/apps`
- Return `token_type: "Bearer"` (capital B)

**Timeline loads but statuses missing viewer-relative fields**
- `favourited`, `reblogged`, `bookmarked`, `pinned`, `muted` must always be present
- For unauthenticated requests, return `false` for all

**Streaming disconnects immediately**
- Check nginx/proxy isn't buffering — set `X-Accel-Buffering: no`
- Send a heartbeat comment (`: thump`) every 15s
- Return `200 OK` before first event, not after

**Media uploads fail**
- Implement `/api/v2/media` not just `/api/v1/media`
- Accept `multipart/form-data` with field name `file`
- Return `202 Accepted` while processing, poll endpoint returns `200` when done

**WebSocket payload parsing errors**
- The `payload` field in WS events is a JSON string, not an object
- Clients call `JSON.parse(event.payload)` — double-encode it on the server

### CORS Requirements

All API endpoints must include these response headers:

```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS, PATCH
Access-Control-Allow-Headers: Authorization, Content-Type, Idempotency-Key
```

`OPTIONS` preflight requests should return `204 No Content` with the above headers and no body.

### Testing Against Real Clients

| Client | Platform | Good for testing |
|--------|----------|-----------------|
| Elk | Web (open source) | Easy to inspect network requests |
| Phanpy | Web (open source) | Good streaming test |
| Tusky | Android | Thorough OAuth + notification test |
| Ivory | iOS | Strictest about entity shapes |

> **Tip**: Run Elk locally (`npx elk`) pointed at your server — you can inspect every
> API call in DevTools with full request/response detail.

---

## Reference Implementations

- **Mastodon source** (`app/serializers/`) — ground truth when docs and behavior diverge
- **Elk** (open source web client) — easiest to debug against; inspect every API call in browser DevTools
