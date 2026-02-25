# Monstera-fed — Phase two features

> **Purpose:** Tracks features deferred from Phase 1 design that should be revisited in later phases.  
> **Last updated:** Feb 24, 2026 (added items 18–21 from ADR 10)

---

## 1. OAuth Out-of-Band (OOB) Code Display Flow

**Origin:** ADR 06, Open Question #3

Mastodon supports `urn:ietf:wg:oauth:2.0:oob` as a redirect URI for CLI tools. Instead of redirecting the browser to a callback URL, the server displays the authorization code on an HTML page for the user to copy-paste into their terminal.

**Current state:** The `isValidRedirectURI` helper already accepts `urn:ietf:wg:oauth:2.0:oob` as a valid URI. What's missing is the display page — when `AuthorizeSubmit` detects this URI, it should render a simple template showing the code rather than issuing an HTTP redirect.

**Why deferred:** Low priority for launch. The primary use case (mobile and desktop Mastodon clients) uses standard redirect URIs. CLI tools (toot, Mastodon.py scripts, admin utilities) are the main consumers of OOB, and their user base is small relative to GUI clients.

**Implementation when ready:**
- Add `internal/api/oauth/templates/oob.html` — a minimal page displaying the authorization code with a "copy" button.
- In `AuthorizeSubmit`, add a branch: if `redirectURI == "urn:ietf:wg:oauth:2.0:oob"`, render the OOB template instead of issuing `http.Redirect`.
- Estimated effort: ~1 template + ~10 lines of handler logic.

---

## 2. Configurable Token Expiry Policy

**Origin:** ADR 06, Open Question #1

Mastodon's default behaviour is non-expiring access tokens — clients cache the token indefinitely and never refresh. Monstera-fed follows this default. However, security-conscious operators may want tokens to expire after a configurable period (e.g. 90 days), forcing users to re-authenticate periodically.

**Current state:** The schema (`oauth_access_tokens.expires_at`) and the token lookup code (`LookupToken`) already support expiry. The `expires_at` column is nullable; NULL means non-expiring. When set, the token is rejected after the timestamp passes and the cache entry uses the shorter of the standard cache TTL and the remaining token lifetime.

**Why deferred:** Adding a configurable TTL is a policy decision with UX implications. Mastodon clients do not implement token refresh (there is no refresh_token grant in the Mastodon OAuth flow). If tokens expire, users are silently logged out and must re-authorize the app. This needs careful product thought: should it be a global setting, per-application, or per-role? Should there be a warning mechanism before expiry?

**Implementation when ready:**
- Add an `instance_settings` key: `oauth_token_ttl_seconds` (NULL or 0 = non-expiring).
- In `Server.issueToken`, read the setting and populate `expires_at` accordingly.
- Add admin portal UI to configure the value.
- Consider a "token expiring soon" notification mechanism (email or SSE event) so users aren't surprised by sudden logouts.
- Estimated effort: ~50 lines of service logic + admin UI work.

---

## 3. Shared Inbox Delivery Optimization

**Origin:** ADR 07, Open Question #1

Phase 1 deduplicates outbound federation deliveries by raw `inbox_url`. This works well because most Mastodon instances use the same shared inbox URL for all accounts. However, Pleroma/Akkoma instances often use per-user inboxes, meaning Monstera-fed sends N identical deliveries to the same server (one per follower) instead of a single delivery to the shared inbox.

**Current state:** The `accounts` table does not have a `shared_inbox_url` column. The `GetFollowerInboxURLs` query returns `a.inbox_url` and deduplicates in application code.

**Why deferred:** The current approach is functionally correct — every remote server receives the activity. The inefficiency only manifests with instances that use per-user inboxes, which are a minority. The cost is wasted outbound HTTP requests, not missed deliveries.

**Implementation when ready:**
- Add migration: `ALTER TABLE accounts ADD COLUMN shared_inbox_url TEXT`.
- Populate `shared_inbox_url` from the `endpoints.sharedInbox` field when ingesting remote Actor documents in `syncRemoteActor`.
- Update `GetFollowerInboxURLs`: `SELECT DISTINCT COALESCE(a.shared_inbox_url, a.inbox_url) FROM accounts a INNER JOIN follows f ON ...`
- Estimated effort: ~1 migration + ~20 lines across `inbox.go`, `outbox.go`, and `accounts.sql`.

---

## 4. Remote Media Lazy-Fetch

**Origin:** ADR 07, Open Question #3

Phase 1 stores `remote_url` on media attachments from incoming `Create{Note}` activities without fetching the media. Clients must proxy or link directly to the remote URL, which has privacy implications (the remote server sees the client's IP) and availability risks (the remote URL may go stale).

**Current state:** `media_attachments.remote_url` is populated; `media_attachments.url` and `storage_key` point to the remote URL. No local copy is stored.

**Why deferred:** Fetching remote media on ingest is I/O-intensive and increases the processing time for incoming activities. Mastodon fetches immediately, but Monstera-fed's Phase 1 prioritizes correctness and simplicity over media fidelity.

**Implementation when ready:**
- Add a NATS `MEDIA_FETCH` stream with subject `media.fetch.>`.
- When `InboxProcessor` stores a remote media reference, enqueue a `media.fetch.{attachmentID}` message.
- A `MediaFetchWorker` (same pattern as `FederationWorker`) pulls messages, downloads the media, stores it via `MediaStore`, and updates `media_attachments.url` and `storage_key`.
- Add a proxy endpoint (`GET /media/proxy/:id`) that triggers a synchronous fetch on cache miss as a fallback.
- Estimated effort: ~1 new NATS stream + ~150 lines of worker code + ~50 lines of proxy handler.

---

## 5. Async Inbox Processing

**Origin:** ADR 07, Open Question #2

Phase 1 processes incoming ActivityPub activities synchronously on the HTTP handler goroutine. Under high inbound traffic, this can slow inbox responses — particularly for `Create{Note}` activities that trigger remote account resolution (outbound HTTP fetch).

**Current state:** `InboxHandler` calls `InboxProcessor.Process` synchronously and returns 202 after processing completes.

**Why deferred:** Most instances will not experience meaningful latency from synchronous processing. The HTTP Signature verification (which is always synchronous) dominates request time. Moving to async processing adds complexity (error visibility, backpressure, ordering) without significant benefit at low-to-medium scale.

**Implementation when ready:**
- Option A: Bounded goroutine pool via `errgroup.SetLimit(50)`. The inbox handler submits work and returns 202 immediately. Simple but no durability — if the process crashes, in-flight activities are lost.
- Option B: Enqueue to a NATS `INBOX_PROCESSING` stream. A dedicated worker pool consumes and processes. Durable and observable but adds latency for the initial enqueue and a new stream to manage.
- Estimated effort: Option A ~30 lines; Option B ~100 lines + NATS stream config.

---

## 6. NodeInfo Active User Counts

**Origin:** ADR 07, Open Question #6

NodeInfo 2.0 supports optional `usage.users.activeMonth` and `usage.users.activeHalfyear` fields. Mastodon populates these; some aggregation sites (instances.social, fediverse.observer) use them to rank instances by activity.

**Current state:** Monstera-fed's NodeInfo response includes `usage.users.total` but not active user counts.

**Why deferred:** The counts are purely informational and do not affect federation. Computing them requires either a query against the `statuses` table (scanning for accounts with recent posts) or tracking a `last_active_at` timestamp on the `users` table. Neither is complex, but the `last_active_at` approach requires updating a row on every authenticated API request, which adds write load.

**Implementation when ready:**
- Add `last_active_at TIMESTAMPTZ` column to the `users` table.
- Update `last_active_at` on authenticated requests (debounced — at most once per 15 minutes per user, using the cache to track the last update time).
- Add queries: `CountActiveUsersMonth` and `CountActiveUsersHalfYear`.
- Populate the NodeInfo fields.
- Estimated effort: ~1 migration + ~40 lines of middleware/query code.

---

## 7. Pinned Posts (Featured Collection)

**Origin:** ADR 07, Design Decisions table

Phase 1 returns an empty `OrderedCollection` for the `featured` endpoint (`/users/{username}/collections/featured`). Mastodon clients and remote servers fetch this to display pinned posts on a user's profile.

**Current state:** The Actor document advertises a `featured` URL. The endpoint returns `{"totalItems": 0, "orderedItems": []}`. No database support for pinning.

**Why deferred:** Pinning requires a new join table (`account_pins` or a `pinned` boolean on `statuses`), REST API endpoints for pin/unpin, and updates to the featured collection handler. It's a self-contained feature with no impact on core federation.

**Implementation when ready:**
- Add `account_pins (account_id, status_id, created_at)` table with a unique constraint and a limit (e.g., max 5 pins per account).
- Add REST endpoints: `POST /api/v1/statuses/:id/pin`, `POST /api/v1/statuses/:id/unpin`.
- Update `FeaturedHandler` to query pinned statuses and return them as `Note` objects.
- Estimated effort: ~1 migration + ~80 lines of handler/service code.

---

## 8. Outbox totalItems Accuracy

**Origin:** ADR 07, Open Question #7

The AP outbox handler returns `totalItems` based on the first page query result count rather than a true count of all public statuses for the account.

**Current state:** `totalItems` shows the number of statuses returned in the first page (up to 20), not the actual total.

**Why deferred:** `totalItems` is informational. Remote servers use `next` page links to traverse the collection, not the count. No known Mastodon client or federation peer relies on `totalItems` being accurate.

**Implementation when ready:**
- Add `CountPublicAccountStatuses(ctx, accountID)` query using the existing `idx_statuses_account` index with an additional `visibility = 'public'` filter.
- Call it in the outbox handler's collection-root response.
- Estimated effort: ~1 query + ~5 lines of handler code.

---

## 9. Scheduled Statuses

**Origin:** ADR 08, Design Decisions table

Mastodon's `POST /api/v1/statuses` accepts a `scheduled_at` parameter that defers publication to a future time. The client receives a `ScheduledStatus` object and can manage it via `/api/v1/scheduled_statuses`.

**Current state:** Phase 1 rejects requests with `scheduled_at` (returns 422). Status creation is always immediate.

**Why deferred:** Scheduled statuses require a persistent job scheduler — either a NATS JetStream delayed delivery, a `scheduled_statuses` table with a polling worker, or a separate cron-like subsystem. The REST endpoints for managing scheduled statuses (`GET`, `PUT`, `DELETE /api/v1/scheduled_statuses/:id`) also need to be implemented. The feature is self-contained and has no impact on core federation or timeline rendering.

**Implementation when ready:**
- Add `scheduled_statuses` table: `(id, account_id, params JSONB, scheduled_at TIMESTAMPTZ, created_at)`.
- Add REST endpoints: `GET /api/v1/scheduled_statuses`, `GET/PUT/DELETE /api/v1/scheduled_statuses/:id`.
- Add a worker goroutine (or NATS delayed message) that polls for statuses past their `scheduled_at` and publishes them through the normal status creation flow.
- Remove the 422 rejection in `POST /api/v1/statuses` and route to the scheduled path when `scheduled_at` is present.
- Estimated effort: ~1 migration + ~150 lines of handler/service/worker code.

---

## 10. Markdown Rendering in Status Content

**Origin:** ADR 08, Design Decisions table

Phase 1 accepts plain text only for status content. Some Mastodon-compatible servers (Misskey, Calckey) and power users expect Markdown or basic formatting support in post composition.

**Current state:** `content.Render` strips all HTML, auto-links URLs/@mentions/#hashtags, and wraps in `<p>` tags. No Markdown parsing.

**Why deferred:** Markdown support requires integrating a Markdown parser (`github.com/yuin/goldmark`), defining which Markdown features are allowed (headers? tables? code blocks?), and ensuring the output is safe after rendering. The interaction between Markdown and @mention/#hashtag auto-linking needs careful handling (Markdown links should not be double-processed). This is a rendering-layer change with no schema or federation impact.

**Implementation when ready:**
- Add `goldmark` dependency with a restricted set of extensions (no raw HTML passthrough).
- In `content.Render`, detect whether the input contains Markdown syntax and route to the Markdown pipeline.
- Auto-linking of @mentions and #hashtags must run *before* Markdown rendering (or integrated as a goldmark extension) to avoid conflicts with Markdown link syntax.
- Add an `instance_settings` key `status_format` (`plain`|`markdown`) to allow admins to enable/disable.
- Estimated effort: ~100 lines of rendering code + goldmark config.

---

## 11. Status Editing via REST API

**Origin:** ADR 08, Design Decisions table

Mastodon 4.0+ supports editing published statuses via `PUT /api/v1/statuses/:id`. The edit history is stored and visible to users.

**Current state:** The database schema already supports editing — `status_edits` table exists (ADR 02, migration 000006), `statuses.edited_at` column exists, and `UpdateStatus` / `CreateStatusEdit` queries are defined. The federation layer handles incoming `Update{Note}` activities (ADR 07). What's missing is the REST API endpoint for local users to edit their own statuses.

**Why deferred:** The REST endpoint itself is straightforward, but editing has complex side effects: re-rendering content (new mentions/hashtags may be added or removed), updating `status_mentions` and `status_hashtags`, federating an `Update{Note}` activity to all followers, and sending new mention notifications without duplicating existing ones. These side effects need careful design.

**Implementation when ready:**
- Add `PUT /api/v1/statuses/:id` handler: accepts same body as `POST /api/v1/statuses` (minus `in_reply_to_id`, `visibility`, `scheduled_at`).
- Before updating: snapshot the current content into `status_edits`.
- Re-run `content.Render` on the new text; diff mentions/hashtags to determine additions and removals.
- Update `statuses` row (text, content, content_warning, sensitive, media_ids, edited_at).
- Federate `Update{Note}`.
- Add `GET /api/v1/statuses/:id/history` endpoint.
- Estimated effort: ~200 lines of handler/service code.

---

## 12. Conversation Muting

**Origin:** ADR 08, Design Decisions table

Mastodon allows users to mute individual conversations (threads) so they stop receiving notifications from replies. The `muted` field on Status responses indicates whether the authenticated user has muted that conversation.

**Current state:** `muted` is hardcoded to `false` on all Status responses. No `conversation_mutes` table exists.

**Why deferred:** Conversation muting requires a `conversation_mutes (account_id, conversation_id)` table, a concept of "conversation ID" (Mastodon uses the root status ID of a thread), and integration with the notification pipeline to suppress mention notifications for muted conversations. It's a self-contained feature.

**Implementation when ready:**
- Add `conversation_mutes (id, account_id, conversation_id TEXT, created_at)` table. `conversation_id` is the root status ID of the thread.
- Add `POST /api/v1/statuses/:id/mute` and `/unmute` endpoints.
- In the notification creation path, check `conversation_mutes` before creating `mention` notifications.
- In the batch status presenter, query `conversation_mutes` to set the `muted` field.
- Estimated effort: ~1 migration + ~80 lines of handler/service code.

---

## 13. Link Preview Cards

**Origin:** ADR 08, Design Decisions table

Mastodon generates Open Graph / oEmbed link preview cards for URLs in statuses. These appear as rich previews below the status content in clients.

**Current state:** `card` is returned as `null` on all Status responses.

**Why deferred:** Link preview generation requires fetching the target URL, parsing HTML for Open Graph meta tags (`og:title`, `og:description`, `og:image`), optionally fetching and storing the preview image, and caching the result. This is I/O-intensive and best handled as an async background job (NATS worker). The feature has no federation impact — cards are a client-facing enhancement.

**Implementation when ready:**
- Add `status_cards` table: `(status_id, url, title, description, image_url, type, provider_name, provider_url, blurhash, width, height)`.
- After status creation, enqueue a `card.fetch.{statusID}` NATS message.
- A `CardFetchWorker` fetches the first URL in the status, parses OG tags, stores the card, and pushes an SSE update event.
- Add the card to the Status presenter (query `status_cards` in the batch lookup).
- Estimated effort: ~1 migration + ~200 lines of worker/parser code.

---

## 14. Profile Field Verification

**Origin:** ADR 08, Open Question #3

Mastodon verifies profile metadata link fields by fetching the target URL and checking for a `rel="me"` backlink to the user's profile. Verified fields display a green checkmark in clients.

**Current state:** The `accounts.fields` JSONB schema includes a `verified_at` field per entry, but it is always `null`. No verification logic exists.

**Why deferred:** Verification requires outbound HTTP fetches to arbitrary URLs (privacy and performance considerations), HTML parsing for `rel="me"` links, and periodic re-verification (links may be removed). This is best implemented as a background job.

**Implementation when ready:**
- On `PATCH /api/v1/accounts/update_credentials` with `fields_attributes`, enqueue a verification job for each field that contains a URL.
- The worker fetches the URL, checks for `<a rel="me" href="https://{INSTANCE_DOMAIN}/@{username}">`, and sets `verified_at` on match.
- Re-verify periodically (e.g., weekly) to detect removed backlinks.
- Estimated effort: ~80 lines of worker code.

---

## 15. WebSocket Streaming Transport

**Origin:** ADR 09, Open Question #2

Mastodon's streaming server supports both SSE (`EventSource`) and WebSocket connections on the same streaming endpoint. Some clients prefer WebSocket for bidirectional framing and better mobile lifecycle management. The connection is upgraded via the standard `Upgrade: websocket` header.

**Current state:** Phase 1 supports SSE only. All major Mastodon clients (Ivory, Tusky, Mona, Ice Cubes, Elk) support SSE as a primary or fallback transport.

**Why deferred:** Adding WebSocket introduces a second transport with different framing (WebSocket messages vs. SSE `event:`/`data:` lines), connection lifecycle, and error handling. The Hub would need to abstract over both transport types. SSE covers the client ecosystem today.

**Implementation when ready:**
- Add `gorilla/websocket` (or `nhooyr.io/websocket`) dependency.
- In `ServeSSE`, detect `Upgrade: websocket` header and branch to a WebSocket handler.
- The WebSocket handler subscribes to the same Hub channel and writes JSON-framed messages instead of SSE frames.
- Keepalive uses WebSocket ping/pong frames instead of SSE comment lines.
- Estimated effort: ~150 lines of handler code + websocket dependency.

---

## 16. Multi-Stream SSE Subscriptions

**Origin:** ADR 09, Open Question #3

Mastodon supports subscribing to multiple streams on a single SSE connection via the `?stream=` query parameter (e.g., `?stream=user&stream=public`) and the `?list=` parameter for list timelines. This reduces the number of open connections per client.

**Current state:** Phase 1 requires one SSE connection per stream type. Clients open separate connections for `user`, `public`, and `hashtag` streams.

**Why deferred:** Multi-stream subscriptions require the Hub to multiplex events from multiple stream keys onto a single channel, tagging each event with its source stream so the client can distinguish them. The SSE frame format also changes (Mastodon wraps multi-stream events in an additional JSON envelope). This adds complexity to both the Hub and the HTTP handler for a moderate connection-count reduction.

**Implementation when ready:**
- Parse `?stream=` as a list of stream keys.
- Subscribe to each stream key via `hub.Subscribe` and merge the channels using a `select` loop or `reflect.Select` for dynamic channel count.
- Tag each `SSEEvent` with a `stream` field in the SSE `data` payload (Mastodon's multi-stream format).
- Estimated effort: ~80 lines of handler + Hub changes.

---

## 17. Hub-Side Mute/Block Filtering for Public Streams

**Origin:** ADR 09, Open Question #1

Mastodon's streaming server filters out posts from muted and blocked accounts before delivering events to authenticated clients on the public and hashtag streams. This means a user watching the public timeline via SSE never sees posts from accounts they've blocked.

**Current state:** The Hub performs pure fan-out — all subscribers to a stream key receive all events on that stream. No per-client filtering. Clients handle mute/block filtering locally using their cached block/mute lists.

**Why deferred:** Per-client filtering requires the Hub to load each connected user's mute and block lists (from the cache or DB), check every incoming event against them, and keep the lists up-to-date when the user mutes/blocks someone during an active SSE session. This significantly increases Hub complexity and memory usage.

**Implementation when ready:**
- On `Subscribe` for authenticated clients: load the user's muted and blocked account ID sets into an in-memory filter attached to the subscriber.
- In `fanOut`, before sending to each subscriber: check if the event's author is in the subscriber's block/mute set. Skip if blocked/muted.
- When a block/mute is created or removed, publish a control event (e.g., `stream.control.{accountID}`) that the Hub uses to update the in-memory filter.
- Estimated effort: ~120 lines of Hub logic + control event plumbing.

---

## 18. Two-Factor Authentication for Admin Portal

**Origin:** ADR 10, Design Decisions table

Admin and moderator accounts are high-value targets. TOTP-based 2FA (RFC 6238) would add a second factor to the admin login flow, significantly reducing the risk of compromised passwords.

**Current state:** Admin login is email + password only, protected by bcrypt (cost 12) and the `HttpOnly`/`SameSite=Strict` session cookie.

**Why deferred:** 2FA requires a new `totp_secrets` table (or column on `users`), a TOTP library (`github.com/pquerna/otp`), a setup flow with QR code generation, recovery codes, and changes to the login handler to prompt for TOTP after password verification. The security posture is acceptable for initial launch given the session cookie protections, but 2FA should be prioritised for any instance with multiple moderators.

**Implementation when ready:**
- Add `totp_secret TEXT` and `totp_enabled BOOLEAN DEFAULT FALSE` columns to `users` (or a dedicated table).
- Add admin portal pages: 2FA setup (QR code + verify), 2FA disable, recovery codes.
- Modify `LoginHandler` POST flow: after password verification, if `totp_enabled`, render a TOTP input form. Verify the 6-digit code before issuing the session.
- Generate 8 recovery codes at setup time, stored as bcrypt hashes.
- Estimated effort: ~1 migration + ~200 lines of handler code + TOTP library + 2 templates.

---

## 19. Federation Report Forwarding (`Flag` AP Activity)

**Origin:** ADR 10, Design Decisions table; SPEC §16

When a local user reports a remote account, the SPEC states that a copy of the report should be forwarded to the remote instance's moderators as an ActivityPub `Flag` activity. This enables cross-instance moderation cooperation.

**Current state:** Reports against remote accounts are stored locally and appear in the admin portal, but no `Flag` activity is sent to the remote instance.

**Why deferred:** The `Flag` activity type was not covered in ADR 07's supported activity vocabulary. Implementing it requires: building the `Flag` activity JSON (actor = instance actor, object = reported account + status URIs), delivering it to the remote instance's inbox, and handling incoming `Flag` activities in the inbox processor. The incoming side also needs a way to display "forwarded reports" in the admin portal — a report where the reporter is a remote instance, not a local user.

**Implementation when ready:**
- Add `Flag` to the outbox publisher: `PublishFlag(ctx, report, targetAccount)`.
- The `Flag` activity uses the instance actor (not the reporting user) as the `actor` to preserve reporter anonymity.
- Add `Flag` handling to the inbox processor: create a local report with `account_id` set to the instance actor.
- Add a "forwarded" badge to reports in the admin UI when the reporter is a remote instance.
- Estimated effort: ~80 lines of outbox code + ~40 lines of inbox code + template changes.

---

## 20. Admin Email Notifications on New Reports

**Origin:** ADR 10, Design Decisions table

Admins and moderators should optionally receive an email when a new report is filed, so they don't need to check the admin portal dashboard for pending reports.

**Current state:** Reports appear in the admin portal queue; there is no push notification mechanism for moderators.

**Why deferred:** This requires: a notification preference system (which moderators want email alerts?), a new email template, and logic in the report creation path to look up moderator emails and send notifications. The volume of reports on a new instance is low enough that periodic dashboard checks suffice.

**Implementation when ready:**
- Add an `instance_settings` key: `notify_moderators_on_report` (boolean, default false).
- When a report is created, if the setting is enabled, query all users with `role IN ('admin', 'moderator')` and send a notification email with report summary.
- Add a `report_notification.html/.txt` email template.
- Consider a per-moderator opt-out preference column on `users` for larger moderation teams.
- Estimated effort: ~1 email template + ~30 lines of service code + admin setting.

---

## 21. Moderator Access to Domain Block Management

**Origin:** ADR 10, Permission Matrix

Phase 1 restricts domain block creation and removal to admin-only. In larger instances with dedicated moderation teams, moderators may need to create domain blocks without escalating to an admin.

**Current state:** Domain block endpoints (`POST /admin/federation/domain-blocks`, `DELETE /admin/federation/domain-blocks/{domain}`) require `role = 'admin'`.

**Why deferred:** Domain blocks have significant federation impact — a `suspend` severity severs all existing follows with the target domain. Restricting this to admins in Phase 1 is the safer default. The permission change is a one-line code modification when ready.

**Implementation when ready:**
- Remove the `requireAdmin` check from the domain block create/delete handlers.
- Optionally: add a `moderator_can_manage_domain_blocks` instance setting for per-instance control.
- Estimated effort: ~2 lines of handler code (or ~10 lines with the setting).
