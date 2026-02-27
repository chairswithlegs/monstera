# Open questions

This document collects **unresolved** product/architecture questions. It intentionally avoids roadmap/task lists; see `docs/roadmap.md` for build order and execution planning.

## How to use

- **When making a design change**: add/update the relevant entry here first.
- **When answering a question**: mark it resolved and link to the commit/PR (or to the architecture doc section that now codifies the decision).

## Questions

### API surface & auth

| Area | Question | Why it matters |
|------|----------|----------------|
| Routing | Should we split Mastodon routes into **RequireAuth** vs **OptionalAuth** groups so public read endpoints work anonymously (status detail, account lookup, public timelines)? | Affects client compatibility and handler grouping; no service-layer impact. |
| Timelines | Do we accept **home timeline cache staleness** (e.g. 60s TTL), or add invalidation on follow/unfollow and/or new posts? | Trade-off between freshness and complexity/write fan-out. |
| Accounts | Should we implement **profile field verification** (`verified_at`) by checking for `rel="me"` backlinks? | Affects client UX; requires outbound HTTP + HTML parsing + background work. |
| Statuses | What is the canonical **character counting** rule (CJK rune count + URL counting convention)? | Impacts post validation and client expectations. |
| Store/queries | How should we manage query complexity for `GET /api/v1/accounts/:id/statuses` filters (`only_media`, `exclude_replies`, `exclude_reblogs`)? | Affects sqlc query design and index usage. |

### Data model & migrations

| Area | Question | Why it matters |
|------|----------|----------------|
| Hashtags | Should we extract/index hashtags for **remote statuses** as well as local ones? | Better federated hashtag timelines vs extra ingest overhead. |
| OAuth | Should we add a partial index for hot-path token lookup (e.g. `WHERE revoked_at IS NULL`) even though `token` is already unique-indexed? | Small perf win; adds a migration and ongoing index maintenance. |

### Federation & ActivityPub

| Area | Question | Why it matters |
|------|----------|----------------|
| Delivery | Should we persist `shared_inbox_url` and coalesce deliveries to shared inboxes (instead of deduping only by inbox URL string)? | Reduces outbound HTTP volume for servers with per-user inboxes. |
| Inbox | Should inbox processing remain synchronous, or move to a bounded async pool / durable queue? | Latency vs complexity/observability/backpressure. |
| Media | Should remote media be lazy-fetched or fetched on ingest and stored locally (local filesystem/S3)? | Privacy (remote sees client IP), durability, and storage cost. |
| Cleanup | On repeated `410 Gone` responses, should we clean up follows / mark domains as gone? | Reduces repeated failed delivery attempts; needs careful semantics. |
| NodeInfo | Should we publish `usage.users.activeMonth` / `activeHalfyear`? If yes, via query or last-activity tracking? | Operator expectations and ecosystem ranking sites; adds write/query load. |
| Outbox | Should `totalItems` be accurate (true count) vs “best-effort” first-page count? | Mostly informational; can add an extra query. |
| Emojis | Should we ingest **remote custom emojis** from incoming activities so clients render them? | Impacts fidelity of federated content; adds ingestion logic/storage. |

### Streaming (SSE) & NATS

| Area | Question | Why it matters |
|------|----------|----------------|
| Filtering | Should the server do **mute/block filtering** for authenticated clients on public/hashtag streams, or rely on client-side filtering? | Compatibility vs Hub complexity/memory. |
| Transport | Do we need **WebSocket** streaming support, or is SSE-only sufficient? | Some clients may prefer WS; doubles transport surface area. |
| Multiplexing | Should we support multi-stream subscriptions on a single connection (`?stream=user&stream=public`)? | Reduces connections; adds multiplexing complexity. |
| Fan-out strategy | Do we keep “fan-out at publish time” to `stream.user.{id}`, or switch to Hub-side follower filtering for large instances? | Changes NATS message volume vs Hub workload; impacts scale characteristics. |
| Control events | Do we implement `filters_changed` and similar control events once filters land? | Client UX; requires coordination with filter subsystem. |

### Email

| Area | Question | Why it matters |
|------|----------|----------------|
| Deliverability | Should we include `List-Unsubscribe` / `List-Unsubscribe-Post` headers on account-lifecycle emails? | Gmail/Yahoo deliverability requirements vs applicability for transactional email. |
| Tokens | If a user requests a new confirmation email, do we invalidate prior tokens or allow multiple active tokens? | Security posture vs user experience; Mastodon-like behavior allows multiples. |
| Reaping | Where do we run periodic deletion of expired email tokens (startup, in-process ticker, external cron)? | Operational simplicity vs keeping app pods stateless/simple. |
| SMTP init | Should SMTP initialization verify connectivity (fail fast), optionally behind a config flag? | Startup reliability vs surfacing email misconfiguration early. |
| Drivers | Do we need vendor-specific HTTP email drivers (vs SMTP-only) for some deployments? | Port egress constraints; increases maintenance surface. |

### Admin portal

| Area | Question | Why it matters |
|------|----------|----------------|
| Dashboard | Should the UI show “last updated” timestamps for cached stats? | UX polish; helps operators interpret cached numbers. |

