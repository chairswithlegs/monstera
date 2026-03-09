# Mastodon API remainder — implementation plan

**Single source of truth** for implementing the remainder of the Mastodon REST API in Monstera. This document merges the high-level plan (current state, specs, security/architecture) with the detailed subplans (order of operations, step-by-step implementation, and whole-plan review).

**References:** [docs/roadmap.md](roadmap.md), [docs/architecture/](architecture/), [Mastodon API methods](https://docs.joinmastodon.org/methods/), [Mastodon entities](https://docs.joinmastodon.org/entities/). When docs and Mastodon behavior differ, match Mastodon (“behavior wins”).

---

## Before you implement

- [ ] **Baseline green:** Run `make test` and `golangci-lint run`; fix any failures before changing code.
- [ ] **One section (or batch) at a time:** Do not start the next section until the current one is done and tests/lint pass.
- [ ] **Read the section:** Before coding, read the full section (Goal, Spec, Pattern, Files to read first, Convention, Steps, Acceptance).
- [ ] **Store convention:** When adding a Store method, add it to the Store interface, postgres implementation, and FakeStore in the same coherent change; then add callers (service, etc.).
- [ ] **Checkpoint after each batch:** After completing a batch (see §2.3), run `make test` and `golangci-lint run`; fix regressions before starting the next batch.

---

## Implementation workflow

1. **Pick a section** (or batch) from the Phase sequence (§2.2). Work in order.
2. **Read** that section’s Pattern, Files to read first, Convention, and Steps.
3. **Implement in layer order:** Store (and FakeStore) → service → handler(s) → router → tests. For sections with migrations, do migration first.
4. **Verify:** Run `make test` and `golangci-lint run` for the changed code paths. Ensure the section’s **Acceptance** criteria are met.
5. **Checkpoint (if you finished a batch):** Run the full test suite and linter; fix any regressions before the next batch.
6. **Repeat** for the next section (or batch).

Do not add Store methods without implementing them in both postgres and FakeStore. Do not skip tests. Follow existing project rules: `api-handler-patterns.mdc`, `error-handling.mdc`, `testing.mdc`, `store.mdc`.

---

## 1. Overview and current state

### 1.1 Implemented (from router and handlers)

- **Instance:** `GET /api/v1/instance`, `GET /api/v2/instance` (same handler; v1 response shape may need alignment).
- **Apps:** `POST /api/v1/apps`.
- **Accounts:** verify_credentials, GET by id, lookup, update_credentials, relationships, :id/statuses|followers|following, follow|unfollow|block|unblock|mute|unmute.
- **Statuses:** POST (422 for scheduled_at), GET :id, DELETE, reblog|unreblog, favourite|unfavourite, bookmark|unbookmark, context, favourited_by, reblogged_by.
- **Timelines:** home, public, tag/:hashtag, list/:id, favourites, bookmarks.
- **Media:** `POST /api/v2/media`, `PUT /api/v1/media/:id`.
- **Notifications:** GET list, GET :id, clear, dismiss. **Reports:** POST. **Follow requests:** GET, authorize, reject.
- **Lists:** full CRUD and list accounts. **Filters:** full CRUD. **Search:** `GET /api/v2/search`.
- **Streaming:** user, public, public/local, hashtag, health. **Custom emojis:** `GET /api/v1/custom_emojis` (stub).

### 1.2 Not implemented (gap)

- **Blocks/Mutes lists:** `GET /api/v1/blocks`, `GET /api/v1/mutes`.
- **Directory:** `GET /api/v1/directory`. **Markers:** `GET/POST /api/v1/markers`. **Preferences:** `GET /api/v1/preferences`.
- **Status pin/unpin:** `POST /api/v1/statuses/:id/pin`, `POST /api/v1/statuses/:id/unpin` (+ ActivityPub featured collection).
- **Status edit:** `PUT /api/v1/statuses/:id`, `GET .../history`, `GET .../source`.
- **Scheduled statuses:** GET/PUT/DELETE `/api/v1/scheduled_statuses` and `:id`; accept `scheduled_at` in POST statuses.
- **Polls:** schema + ingest; POST statuses with `poll`; `GET /api/v1/polls/:id`, `POST .../votes`.
- **Announcements:** GET, dismiss, reactions (optional). **Followed tags:** GET/POST/DELETE `/api/v1/followed_tags`.
- **Featured tags:** GET/POST/DELETE `/api/v1/featured_tags` + suggestions.
- **Conversation mute:** `POST /api/v1/statuses/:id/mute` and `unmute` (thread mute).
- **Optional later:** status card (link preview), translate, push subscription, trends, quotes/revoke.

---

## 2. Order of operations

### 2.1 Rationale

- **No new schema first** — Start with instance, blocks, mutes, preferences (existing tables/handlers only).
- **Read-only before write** — GET before POST/PUT/DELETE where it makes sense (e.g. markers).
- **Dependencies** — Pins and featured collection together; edit reuses status_edits; scheduled and polls need new tables/workers; conversation mute touches notification path.
- **Client impact** — Instance v1 and blocks/mutes unblock clients early; pins and edit are high-visibility; scheduled and polls are larger.

### 2.2 Phase sequence

| Phase | Section | Why this order |
|-------|---------|----------------|
| 1 | Instance API alignment (v1 vs v2) | No schema; unblocks client version checks. |
| 2 | Blocks list | Store has ListBlockedAccounts; expose + handler. |
| 3 | Mutes list | Same pattern; add ListMutedAccounts query. |
| 4 | Preferences | Map existing user columns; no new tables. |
| 5 | Markers | One new table; GET + POST. |
| 6 | Directory | One store query + handler. |
| 7 | Status pin/unpin + featured collection | account_pins table; ActivityPub featured; pinned in status. |
| 8 | Status edit, history, source | Reuse status_edits; Update(Note) federation. |
| 9 | Scheduled statuses | New table + worker; POST statuses with scheduled_at. |
| 10 | Polls | New tables; POST statuses with poll; GET poll, POST votes; AP. |
| 11 | Followed tags | New table; GET/POST/DELETE. |
| 12 | Featured tags | New table; GET/POST/DELETE + suggestions. |
| 13 | Conversation mute | New table; mute/unmute; notification filtering; status.muted. |
| 14 | Announcements | Optional; admin-driven. |

### 2.3 Implementation batching

- **Batch 1 (1–2 days):** Phases 1–4 — instance, blocks, mutes, preferences (no or minimal schema).
- **Batch 2 (~1 day):** Phases 5–6 — markers, directory.
- **Batch 3 (~2 days):** Phases 7–8 — pins + featured, edit/history/source.
- **Batch 4 (1–2 days):** Phase 9 — scheduled statuses.
- **Batch 5 (2–3 days):** Phase 10 — polls.
- **Batch 6 (~2 days):** Phases 11–13 — followed tags, featured tags, conversation mute.
- **Batch 7 (~1 day):** Phase 14 — announcements (optional).

**Total:** ~10–14 days focused implementation + buffer for integration/federation testing.

### 2.4 Checkpoints

After completing each batch below, run `make test` and `golangci-lint run`; fix any regressions before starting the next batch.

- **After Batch 1** (Phases 1–4: instance, blocks, mutes, preferences)
- **After Batch 2** (Phases 5–6: markers, directory)
- **After Batch 3** (Phases 7–8: pins + featured, edit/history/source)
- **After Batch 4** (Phase 9: scheduled statuses)
- **After Batch 5** (Phase 10: polls)
- **After Batch 6** (Phases 11–13: followed tags, featured tags, conversation mute)
- **After Batch 7** (Phase 14: announcements, optional)

---

## 3. Feature sections (detailed subplans)

### Section 1: Instance API alignment (v1 vs v2)

**Goal:** Return correct Mastodon v1 and v2 instance shapes so clients that check `version` and `urls.streaming_api` work.

**Spec:** [V1::Instance](https://docs.joinmastodon.org/entities/V1_Instance/) (uri, title, urls.streaming_api, stats, contact_account, rules); [Instance v2](https://docs.joinmastodon.org/entities/Instance/) (domain, configuration, registrations, contact).

**Pattern:** Branch response by route (v1 vs v2); no new Store methods unless stats require new queries. Handler-only change possible if stats already exist.

**Files to read first:** `internal/api/mastodon/instance.go`, `internal/api/router/router.go` (how instance routes are registered).

**Convention:** Add v1 response struct; branch in handler or register two handlers that call a shared helper with version; set version string (e.g. `4.1.0`).

**Steps:**

1. In `internal/api/mastodon/instance.go`, add v1 response struct: `uri`, `title`, `short_description`, `description`, `email`, `version`, `urls` (streaming_api), `stats` (user_count, status_count, domain_count), `languages`, `contact_account`, `rules`.
2. Ensure Store has `CountLocalAccounts`, `CountLocalStatuses`; add or use domain count (e.g. known_instances or distinct domains). Optionally cache stats with TTL.
3. Resolve contact account from instance setting or config; if none, return null for contact_account.
4. Branch by route: v1 routes return v1 entity, v2 return existing InstanceResponse with configuration.
5. Set version string (e.g. `4.1.0`) so clients don’t disable features.
6. Router: ensure `/api/v1/instance` and `/api/v2/instance` use the correct response builder.
7. **Tests:** Assert v1 keys (uri, urls, stats, version) and v2 keys (domain, configuration, registrations); version non-empty.

**Files:** `internal/api/mastodon/instance.go`, handler test. **Acceptance:** Clients get correct v1/v2 shape; version is plausible.

---

### Section 2: Blocks list

**Goal:** `GET /api/v1/blocks` returns paginated blocked accounts for the authenticated user.

**Spec:** [blocks](https://docs.joinmastodon.org/methods/blocks/) — `[]Account`; max_id, limit, since_id; auth + read:blocks or follow.

**Pattern:** Same as list endpoints (e.g. GET /api/v1/accounts/:id/followers): Store → service → handler → router → test. Cursor pagination + Link header.

**Files to read first:** `internal/store/store.go`, `internal/store/postgres/store_domain.go`, `internal/api/mastodon/accounts.go` (e.g. GETFollowers for list + Link pattern), `internal/api/router/router.go`.

**Convention:** Add Store method + postgres impl + FakeStore in the same change; then service (enforce limit cap, accountID = auth user); then handler (parse max_id/limit, build Link); then route (RequireAuth, read:blocks).

**Steps:**

1. **Store:** Add `ListBlockedAccounts(ctx, accountID string, maxID *string, limit int) ([]domain.Account, *string, error)` to `internal/store/store.go`; return next cursor for Link header.
2. **Postgres:** Implement using existing sqlc `ListBlockedAccounts`; add cursor pagination (e.g. by block id); return accounts + next cursor in `store_domain.go`.
3. **FakeStore:** Implement ListBlockedAccounts (blocks for accountID → target accounts; limit/cursor).
4. **Service:** ListBlockedAccounts(accountID, maxID, limit); enforce limit cap (80); accountID must be authenticated user (handler).
5. **Handler:** RequireAuth; parse max_id, limit (default 40, max 80); call service; build Link header; return []Account via apimodel.
6. **Router:** GET /api/v1/blocks, RequireAuth, read:blocks (or follow).
7. **Tests:** 401 without auth; 200 empty; 200 with blocks + Link pagination.

**Files:** store.go, store_domain.go, fakestore.go, account_service.go, accounts.go (or blocks.go), router, _test.go. **Acceptance:** GET /api/v1/blocks returns blocked accounts and pagination.

---

### Section 3: Mutes list

**Goal:** `GET /api/v1/mutes` returns paginated muted accounts.

**Spec:** [mutes](https://docs.joinmastodon.org/methods/mutes/) — `[]Account`; same pagination; read:mutes or follow.

**Pattern:** Same as Section 2 (blocks): Store (new query join mutes → accounts) → service → handler → router → test.

**Files to read first:** `internal/store/postgres/queries/mutes.sql`, `internal/store/postgres/queries/blocks.sql` (ListBlockedAccounts for join pattern), `internal/api/mastodon/accounts.go` (list handler pattern).

**Convention:** Add ListMutedAccounts SQL (join mutes to accounts); add to Store + postgres + FakeStore together; then service/handler/router; scope read:mutes.

**Steps:**

1. **SQL:** In `internal/store/postgres/queries/mutes.sql`, add `ListMutedAccounts :many`: join mutes → accounts on target_id where account_id = $1, order by mutes.id desc, limit/offset or cursor. Run sqlc.
2. **Store:** Add `ListMutedAccounts(ctx, accountID string, maxID *string, limit int) ([]domain.Account, *string, error)`; implement in postgres and FakeStore.
3. **Service + handler + router:** Same pattern as blocks; GET /api/v1/mutes, RequireAuth, read:mutes.
4. **Tests:** 401, 200 empty, 200 with mutes and pagination.

**Files:** mutes.sql, store, fakestore, service, handler, router, tests. **Acceptance:** GET /api/v1/mutes returns muted accounts and pagination.

---

### Section 4: Preferences

**Goal:** `GET /api/v1/preferences` returns posting/reading defaults.

**Spec:** [preferences](https://docs.joinmastodon.org/methods/preferences/) — posting:default:visibility, posting:default:sensitive, posting:default:language, reading:expand:media, reading:expand:spoilers.

**Pattern:** Handler-only (or minimal store): load user via existing GetUserByAccountID; map user columns to Mastodon preference keys; return JSON object.

**Files to read first:** `internal/domain/` (User type), `internal/store/migrations/000028_add_user_preferences.up.sql`, `internal/api/mastodon/accounts.go` (e.g. GETVerifyCredentials for auth + user load).

**Convention:** No new Store method if user already has default_privacy, default_sensitive, default_language; add columns only if Mastodon keys require them. RequireAuth, read:accounts.

**Steps:**

1. Map user columns (migration 028: default_privacy, default_sensitive, default_language) to Mastodon keys; add user columns for expand_media/expand_spoilers if needed, or use defaults.
2. Store: ensure user fetch returns preference fields.
3. Handler: RequireAuth; get user; build map/struct; return JSON.
4. Router: GET /api/v1/preferences, RequireAuth, read:accounts.
5. Tests: 401; 200 with required keys.

**Files:** Optional migration, domain user, store, handler (e.g. preferences.go), router, test. **Acceptance:** GET /api/v1/preferences returns key-value preferences.

---

### Section 5: Markers

**Goal:** GET and POST /api/v1/markers for home and notifications read positions.

**Spec:** [markers](https://docs.joinmastodon.org/methods/markers/) — GET ?timeline[]=home&timeline[]=notifications; POST home[last_read_id], notifications[last_read_id].

**Pattern:** New table → Store (GetMarkers, SetMarker) + FakeStore → service (allowlist timelines) → handler (GET/POST) → router → test.

**Files to read first:** `internal/store/store.go`, `internal/store/postgres/store_domain.go`, `internal/domain/` (add Marker type), `internal/testutil/fakestore.go` (for new methods).

**Convention:** Migration first; add Store interface + postgres + FakeStore together; allowlist timelines to ["home", "notifications"] in service; handler returns object keyed by timeline.

**Steps:**

1. **Migration:** `markers (account_id, timeline text, last_read_id text, version int default 0, updated_at timestamptz)`, PK (account_id, timeline).
2. **Store:** GetMarkers(ctx, accountID, timelines []string) (map[string]domain.Marker, error), SetMarker(ctx, accountID, timeline, lastReadID) error (upsert, increment version). Domain type Marker with LastReadID, Version, UpdatedAt.
3. **Postgres + FakeStore:** Implement both.
4. **Service:** Allowlist timelines ["home", "notifications"]; get/set for authenticated account only.
5. **Handler:** GET parse timeline[]; POST parse form/JSON; return object keyed by timeline.
6. **Router:** GET/POST /api/v1/markers, RequireAuth, read:statuses / write:statuses.
7. **Tests:** GET empty; POST then GET; invalid timeline handled.

**Files:** Migration, store, domain Marker, postgres + fakestore, service, handler (markers.go), router, tests. **Acceptance:** Clients can save/restore read positions.

---

### Section 6: Directory

**Goal:** GET /api/v1/directory returns discoverable/local accounts.

**Spec:** [directory](https://docs.joinmastodon.org/methods/directory/) — offset, limit (default 40, max 80), order (active|new), local (bool); public.

**Pattern:** Store query (ListDirectoryAccounts) → service (cap limit) → handler (parse query) → router. Optional migration for last_status_at if “active” order needed.

**Files to read first:** `internal/store/store.go`, `internal/store/postgres/queries/` (account list patterns), `internal/api/mastodon/accounts.go` (OptionalAuth list handler).

**Convention:** OptionalAuth (public endpoint); cap limit at 80 in service; order “active” by last status time, “new” by created_at; localOnly filter by domain IS NULL.

**Steps:**

1. **Store:** ListDirectoryAccounts(ctx, order, localOnly bool, offset, limit int) ([]domain.Account, error). "active" = by last status time (last_status_at or join); "new" = created_at. Filter local by domain IS NULL if no discoverable column.
2. **Schema:** Add last_status_at to accounts if needed for "active"; migration; update on status create.
3. **Service:** ListDirectory; cap limit 80.
4. **Handler:** Parse offset, limit, order, local; OptionalAuth; return []Account.
5. **Router:** GET /api/v1/directory, OptionalAuth.
6. **Tests:** order, limit cap, local filter.

**Files:** Store, optional migration, service, handler, router, tests. **Acceptance:** GET /api/v1/directory returns accounts per params.

---

### Section 7: Status pin/unpin + featured collection

**Goal:** Pin/unpin own statuses (max 5); ActivityPub featured returns pinned notes.

**Spec:** [statuses pin/unpin](https://docs.joinmastodon.org/methods/statuses/#pin); ActivityPub featured (stubbed in [collections.go](internal/api/activitypub/collections.go)).

**Pattern:** Migration (account_pins) → Store (+ FakeStore) → service (pin/unpin + visibility/max checks) → StatusesHandler (pin/unpin) + status response (pinned field) + ActivityPub GETFeatured.

**Files to read first:** `internal/api/activitypub/collections.go` (GETFeatured stub), `internal/api/mastodon/statuses.go` (handler pattern), `internal/service/status_service.go`, `internal/api/mastodon/apimodel/status.go` (pinned field).

**Convention:** Pin/unpin on StatusesHandler; return full Status; GETFeatured queries ListPinnedStatusIDs and converts statuses to Note orderedItems; set pinned: true in status API when viewer is owner and status in pin set.

**Steps:**

1. **Migration:** account_pins (account_id, status_id, created_at), unique (account_id, status_id); index account_id; max 5 enforced in app.
2. **Store:** CreateAccountPin, DeleteAccountPin, ListPinnedStatusIDs(accountID), CountAccountPins(accountID); postgres + FakeStore.
3. **Service:** Pin: verify owner, visibility public/unlisted, count < 5; create. Unpin: verify owner; delete.
4. **Status response:** Set pinned: true for owner when status in ListPinnedStatusIDs; batch-load for lists.
5. **Handlers:** POST statuses/:id/pin, POST statuses/:id/unpin; return full Status.
6. **ActivityPub:** GETFeatured: query pinned statuses; convert to Note; orderedItems + totalItems.
7. **Router:** pin/unpin with write:statuses.
8. **Tests:** pin/unpin success; 403 other’s status; 422 direct/max pins; featured returns pinned.

**Files:** Migration, store, service, status apimodel (pinned), handlers, activitypub collections, router, tests. **Acceptance:** Pin up to 5; featured shows them; pin/unpin return Status.

---

### Section 8: Status edit, history, source

**Goal:** PUT /api/v1/statuses/:id; GET history; GET source; federate Update(Note).

**Spec:** [PUT](https://docs.joinmastodon.org/methods/statuses/#update), [history](https://docs.joinmastodon.org/methods/statuses/#history), [source](https://docs.joinmastodon.org/methods/statuses/#source).

**Pattern:** Store (ensure ListStatusEdits, CreateStatusEdit, UpdateStatus on interface; add GetStatusSource if needed) → service (UpdateStatusFromAPI, GetStatusHistory, GetStatusSource) → handlers (PUT, GET history, GET source) → outbox (PublishUpdate(Note)).

**Files to read first:** `internal/store/postgres/generated/statuses.sql.go` (ListStatusEdits, CreateStatusEdit, UpdateStatus), `internal/service/status_service.go`, `internal/activitypub/outbox.go` (how Create is published), `internal/api/mastodon/statuses.go`.

**Convention:** Reuse existing status_edits; PUT requires owner, snapshots to CreateStatusEdit, re-renders content, diffs mentions/hashtags; history/source apply same visibility as GET status; add PublishUpdate(Note) to outbox.

**Steps:**

1. **Store:** Ensure ListStatusEdits, CreateStatusEdit, UpdateStatus on Store; add GetStatusSource(text, spoiler_text) if needed.
2. **Service:** UpdateStatusFromAPI: resolve status; require owner; CreateStatusEdit (current content); re-render; diff mentions/hashtags; UpdateStatus; PublishUpdate(Note); return status. GetStatusHistory/GetStatusSource: visibility check; return edits/source.
3. **Handlers:** PUT parse body; GET history (array of StatusEdit); GET source (id, text, spoiler_text).
4. **Federation:** Add PublishUpdate(Note) in outbox; inbox already handles Update(Note).
5. **Router:** PUT and GET history/source with write:statuses / read:statuses.
6. **Tests:** edit success, 403/404; history order; source fields; optional Update delivery test.

**Files:** Store, service (status_service.go), outbox, handlers, router, tests. **Acceptance:** Edit own status; history/source correct; Update(Note) sent.

---

### Section 9: Scheduled statuses

**Goal:** POST statuses with scheduled_at returns ScheduledStatus; worker publishes at time; GET/PUT/DELETE scheduled_statuses.

**Spec:** [scheduled_statuses](https://docs.joinmastodon.org/methods/scheduled_statuses/).

**Pattern:** Migration → Store (+ FakeStore) → service (Create, List, Get, Update, Delete, PublishScheduled) → worker (ticker/loop) → POST statuses branch (scheduled_at) → new scheduled_statuses handler → router → test.

**Files to read first:** `internal/api/mastodon/statuses.go` (POSTStatuses, where to branch on scheduled_at), `internal/service/status_service.go` (status creation path), `internal/store/store.go`, `cmd/server/` (where to run worker).

**Convention:** POST /api/v1/statuses with scheduled_at creates scheduled row and returns ScheduledStatus (remove 422); worker runs in same process (ticker) or separate; all scheduled_statuses endpoints verify account ownership (IDOR).

**Steps:**

1. **Migration:** scheduled_statuses (id, account_id, params jsonb, scheduled_at, created_at).
2. **Store:** CreateScheduledStatus, GetScheduledStatusByID, ListScheduledStatuses(accountID, maxID, sinceID, minID, limit), UpdateScheduledStatus, DeleteScheduledStatus; all filter by account_id.
3. **Service:** Create: scheduled_at > now; return ScheduledStatus. PublishScheduled(id): create status from params via existing path; delete scheduled row; publish.
4. **Worker:** Ticker or loop: select where scheduled_at <= now(); PublishScheduled each; atomic publish+delete where possible.
5. **POST statuses:** If scheduled_at present and valid, create scheduled and return ScheduledStatus; remove 422 for scheduled_at.
6. **Handlers:** GET list (pagination), GET :id, PUT :id, DELETE :id; RequireAuth; verify ownership.
7. **Router:** All four with read/write:statuses.
8. **Tests:** POST scheduled_at → ScheduledStatus; list/get/update/delete; worker test.

**Files:** Migration, store, service, worker, statuses handler, scheduled_statuses handler, router, tests. **Acceptance:** Scheduled posts stored and published; CRUD works.

---

### Section 10: Polls

**Goal:** Create status with poll; GET poll; POST vote; ingest/federate poll in Note.

**Spec:** [polls](https://docs.joinmastodon.org/methods/polls/), [Poll entity](https://docs.joinmastodon.org/entities/Poll/).

**Pattern:** Migrations (polls, poll_options, poll_votes) → Store (+ FakeStore) → service (CreatePoll, GetPoll, RecordVote) → status creation (attach poll) + poll handler (GET, POST votes) → ActivityPub (inbox/outbox poll in Note) → instance config (polls) → router → test.

**Files to read first:** `internal/service/status_service.go` (CreateStatus path), `internal/activitypub/inbox.go` (Create(Note) handling), `internal/activitypub/outbox.go` (Note building), `internal/api/mastodon/instance.go` (configuration).

**Convention:** Create poll and options when POST statuses includes poll; GET poll applies status visibility; POST votes validates not expired and choices; RecordVote idempotent per account/poll; include poll in Note for federation.

**Steps:**

1. **Migrations:** polls (id, status_id unique, expires_at, multiple, created_at); poll_options (id, poll_id, title, position); poll_votes (id, poll_id, account_id, option_id, created_at) with unique (poll_id, account_id) for single-choice.
2. **Store:** CreatePoll, GetPollByID, GetPollByStatusID, ListOptions, RecordVote, GetVoteCounts, HasVoted, GetOwnVotes.
3. **Create status:** If request has poll (options, expires_in, multiple), create poll+options; attach to status; return Status with embedded Poll.
4. **GET polls/:id:** Resolve poll; visibility check; load options and counts; if auth set voted, own_votes.
5. **POST polls/:id/votes:** Body choices[]; validate not expired and choices; RecordVote; return Poll.
6. **ActivityPub:** Inbox Create(Note) with poll → persist poll. Outbox: include poll in Note.
7. **Instance config:** v2 instance configuration.polls (max_options, etc.).
8. **Router + tests:** GET poll OptionalAuth; POST votes RequireAuth. Tests: create with poll; get; vote; 422 expired/already voted.

**Files:** Migrations, store, service, status creation, poll handler, activitypub, instance config, router, tests. **Acceptance:** Polls created, voted, federated.

---

### Section 11: Followed tags

**Goal:** GET/POST/DELETE /api/v1/followed_tags.

**Spec:** [followed_tags](https://docs.joinmastodon.org/methods/followed_tags/).

**Pattern:** Migration (account_followed_tags) → Store (+ FakeStore) → service (FollowTag, UnfollowTag, ListFollowedTags) → handler → router → test.

**Files to read first:** `internal/store/store.go` (GetOrCreateHashtag, hashtag usage), `internal/store/postgres/queries/`, `internal/api/mastodon/` (list handler + Tag entity).

**Convention:** Resolve tag by name via GetOrCreateHashtag; FollowTag/UnfollowTag add/remove row; list returns Tag entities with following: true; RequireAuth, read/write follows.

**Steps:**

1. **Migration:** account_followed_tags (account_id, tag_id, created_at), unique (account_id, tag_id).
2. **Store:** FollowTag, UnfollowTag, ListFollowedTags(accountID, maxID, limit); resolve tag by name via GetOrCreateHashtag.
3. **Service + handlers:** GET list; POST { name }; DELETE :id. Return Tag entity with following: true.
4. **Router:** RequireAuth, read/write follows. **Tests:** list, follow, unfollow.

**Files:** Migration, store, service, handler, router, tests. **Acceptance:** Users can follow/unfollow tags; list returns followed tags.

---

### Section 12: Featured tags

**Goal:** GET/POST/DELETE /api/v1/featured_tags and GET suggestions.

**Spec:** [featured_tags](https://docs.joinmastodon.org/methods/featured_tags/).

**Pattern:** Migration (account_featured_tags) → Store (+ FakeStore) → service (Create, Delete, List, Suggestions from status_hashtags) → handler (list, POST, DELETE, GET suggestions) → router → test.

**Files to read first:** `internal/store/store.go` (status_hashtags, hashtags), `internal/api/mastodon/` (Tag/FeaturedTag entity shape), Section 11 (followed tags) for similar CRUD.

**Convention:** Suggestions = tags from account’s status_hashtags with use counts; return FeaturedTag (id, name, url, statuses_count, last_status_at) and suggestions with history; read:accounts, write:accounts.

**Steps:**

1. **Migration:** account_featured_tags (id, account_id, tag_id, created_at), unique (account_id, tag_id).
2. **Store:** CreateFeaturedTag, DeleteFeaturedTag, ListFeaturedTags(accountID); suggestions from status_hashtags + counts.
3. **Service + handlers:** GET list; POST { name }; DELETE :id; GET featured_tags/suggestions. Return FeaturedTag and suggestions with history.
4. **Router:** read:accounts, write:accounts. **Tests:** CRUD and suggestions.

**Files:** Migration, store, service, handler, router, tests. **Acceptance:** Feature tags on profile; suggestions return used tags.

---

### Section 13: Conversation mute

**Goal:** POST statuses/:id/mute and unmute (thread); status.muted = true; no mention notifications for muted threads.

**Spec:** [statuses mute/unmute](https://docs.joinmastodon.org/methods/statuses/#mute).

**Pattern:** Migration (conversation_mutes) → Store (GetConversationRoot, Create/DeleteConversationMute, IsConversationMuted) → service (mute/unmute + skip mention notification when muted) → status builder (set muted) → StatusesHandler (mute/unmute) → router → test.

**Files to read first:** `internal/service/` (where mention notifications are created), `internal/api/mastodon/apimodel/status.go` (muted field), `internal/api/mastodon/statuses.go` (handler pattern).

**Convention:** conversation_id = root status id (walk in_reply_to_id to root); when creating mention notification, load root and skip if IsConversationMuted(mentionee, root); set status.muted in API when viewer has muted that conversation; write:mutes or write:statuses.

**Steps:**

1. **Migration:** conversation_mutes (account_id, conversation_id text, created_at). conversation_id = root status id (walk in_reply_to_id).
2. **Store:** CreateConversationMute, DeleteConversationMute, IsConversationMuted; GetConversationRoot(statusID).
3. **Service:** MuteConversation(accountID, statusID): root = GetConversationRoot; create mute. When creating mention notification: if IsConversationMuted(mentionee, root) skip.
4. **Status response:** Set muted = true when IsConversationMuted(viewer, root); batch-load for timelines.
5. **Handlers:** POST mute, POST unmute; return full Status.
6. **Router:** write:mutes or write:statuses. **Tests:** mute/unmute; no notification for muted thread.

**Files:** Migration, store, service (notification + status builder), handlers, router, tests. **Acceptance:** Thread mute; status.muted true; no mention notifications in muted thread.

---

### Section 14: Announcements (optional)

**Goal:** GET /api/v1/announcements; dismiss; reactions (optional).

**Spec:** [announcements](https://docs.joinmastodon.org/methods/announcements/).

**Pattern:** Migrations (announcements, announcement_reads, optional reactions) → Store (+ FakeStore) → service (List active, Dismiss, Add/RemoveReaction) → handler → admin API (create/update) → router → test.

**Files to read first:** `internal/api/monstera/` (admin API pattern), `internal/api/mastodon/` (list + auth handler), `internal/store/store.go`.

**Convention:** Optional feature; list returns active announcements for account; dismiss marks read; reactions optional; admin creates/updates via Monstera admin API; RequireAuth for Mastodon endpoints.

**Steps:**

1. **Migrations:** announcements (id, content, starts_at, ends_at, all_day, published_at, updated_at); announcement_reads (account_id, announcement_id); optional announcement_reactions.
2. **Store + service:** List active for account; Dismiss; AddReaction/RemoveReaction. Admin API to create/update announcements.
3. **Handlers:** GET list; POST :id/dismiss; PUT/DELETE :id/reactions/:name. RequireAuth.
4. **Router + tests:** Register; tests for list and dismiss.

**Files:** Migrations, store, service, handler, admin API, router, tests. **Acceptance:** Users see and dismiss announcements.

---

## 4. Testing strategy

- **Handlers:** For each new/changed endpoint: 200/201 + body shape; 401 without auth; 403 wrong scope or forbidden; 404; 422 validation. Table-driven; require/assert; fakes or mocks.
- **Pagination:** Test Link header and next/prev where applicable (blocks, mutes, directory, scheduled_statuses).
- **Entity shape:** Assert required fields and types for Status, Account, Poll, ScheduledStatus, Marker, Preferences.
- **Integration:** For store+service+federation flows, add integration tests (build tag) or unit tests with FakeStore.
- **Regression:** Before/after each feature run `make test` and `make test-integration`; no conditional skips (per testing rule).

---

## 5. Security and architecture refinements

**Security:**

- **Auth and scope:** RequireAuth for private data; correct scope (read:blocks, read:mutes, write:statuses, etc.). OptionalAuth for directory, instance, public timelines where behavior differs when logged in.
- **IDOR:** Resolve resource then check owner or visibility (only own blocks/mutes/markers/pins/scheduled; only owner can pin/edit).
- **Visibility:** Reuse service-layer visibility and block checks; return 404 when not allowed.
- **Input:** Validate and bound limit, offset, timeline names, poll options, scheduled_at; allowlists for enums.

**Architecture:**

- **Layers:** Handlers parse and call service; service holds business rules and calls store; store HTTP-agnostic. No net/http or status codes in service/store.
- **Errors:** Domain sentinels; wrap with %w; map to HTTP only in handlers via api.HandleError; no double logging.
- **Dependencies:** New store methods on Store interface; FakeStore updated; no new cycles (api → service → store/domain).

---

## 6. Docs and roadmap updates

- **docs/roadmap.md:** As each deferred item is implemented, shorten or remove that section and add “Implemented in …” or PR link.
- **docs/architecture/04-authentication-authorization.md:** Document new scopes (e.g. read:blocks, read:mutes) if added.
- **docs/architecture/01-high-level-system-architecture.md:** Add bullet if new worker (scheduled publisher) or subsystem is added.

---

## 7. Whole-plan review

**Consistency:** All sections follow handlers → service → store; auth/scope and IDOR called out; cursor pagination and Link header where needed; entity shapes match Mastodon docs.

**Dependencies and risks:** Phases 1–6 low-risk; 7–8 reuse or small new schema; 9–10 add workers and federation. Test Update(Note), poll in Note, and featured with a peer if possible. Scheduled worker: at-least-once (delete only after successful publish). Run full test suite before/after each section; no skips.

**Completion criteria:** All endpoints implemented with correct auth/scope; FakeStore and tests for new store methods; handler tests for 401/403/404/422 and success; instance v1/v2 and version set; roadmap and architecture docs updated; linter and tests pass; no new package cycles; internal/service does not import internal/api.
