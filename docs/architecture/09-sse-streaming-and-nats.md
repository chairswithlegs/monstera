# SSE streaming and NATS

This document describes the desired SSE streaming hub, NATS client/streams, and event publisher. Build order is in [roadmap.md](../roadmap.md).

---

## Design decisions

| Question | Decision |
|----------|----------|
| NATS client library API | **`github.com/nats-io/nats.go/jetstream`** (newer API) for JetStream operations — consistent with IMPLEMENTATION 07. Core `nats.Conn` used for connection management and core pub/sub. |
| `EnsureStreams` location | **`internal/nats/streams.go`** — consolidated. All JetStream stream definitions (FEDERATION, FEDERATION_DLQ, and any future streams) live in one file. Refactors IMPLEMENTATION 07's per-package `EnsureStreams`. |
| FEDERATION stream MaxAge | **72 hours** — work queue messages older than 3 days are stale and can be dropped. |
| FEDERATION_DLQ MaxAge | **30 days** — admins need time to inspect and re-queue failed deliveries. |
| SSE follower fan-out strategy | **Fan-out at publish time** — publish to `stream.user.{followerID}` for each follower. Simple, correct, and appropriate for Monstera-fed's self-hosted scale target. Hub-side follow-list filtering documented as a Phase 2 optimization. |
| SSE transport | **Server-Sent Events only** — no WebSocket support in Phase 1. Mastodon clients handle SSE natively. WebSocket is a Phase 2 consideration. |
| NATS delivery semantics for SSE | **Core pub/sub (at-most-once)** — no JetStream for SSE fan-out. Missed events are backfilled via REST on reconnect (standard Mastodon client behavior). |
| Hub channel buffer size | **16 events** per SSE connection. On buffer full, drop oldest event (non-blocking send) and log warning. |
| Keepalive interval | **30 seconds** — write `:keepalive\n\n` comment frame. Prevents proxy/LB idle timeouts. |
| SSE auth token source | **Query parameter `access_token`** as primary, `Authorization: Bearer` header as fallback — Mastodon streaming convention. EventSource API does not support custom headers. Implemented as a `StreamingTokenFromQuery` middleware that copies the query param into the `Authorization` header before `OptionalAuth` runs (Approach A — same pattern as `middleware.RealIP`). |
| Delete event fan-out | **Full fan-out** — delete events are published to the same NATS subjects as the original post (public channels, per-follower, per-hashtag). NATS core pub/sub handles the volume trivially, and this preserves Mastodon-compatible behavior where clients remove deleted posts from their UI immediately. |

---

## 1. `internal/nats/client.go` — NATS Connection Setup

### Package Layout

| File | Responsibility |
|------|----------------|
| `client.go` | `Client` struct, `New` constructor with connection options, `Ping` health check |
| `streams.go` | `EnsureStreams` — all JetStream stream and consumer definitions |

### Connection Options

`New` applies the following `nats.Option` values:

| Option | Value | Rationale |
|--------|-------|-----------|
| `nats.MaxReconnects(-1)` | Unlimited | NATS should reconnect indefinitely; the readiness probe will mark the pod unhealthy if connectivity is lost. |
| `nats.ReconnectWait(2 * time.Second)` | 2s between attempts | Avoids thundering herd on NATS restart. |
| `nats.ReconnectBufSize(16 * 1024 * 1024)` | 16 MB | Buffer pending publishes during reconnect. |
| `nats.DisconnectErrHandler(fn)` | Log at `slog.Warn` | `"nats: disconnected"` with error detail. |
| `nats.ReconnectHandler(fn)` | Log at `slog.Info` | `"nats: reconnected"` with server URL. |
| `nats.ErrorHandler(fn)` | Log at `slog.Error` | Catches async errors (slow consumers, etc.). |
| `nats.UserCredentials(path)` | Conditional | Only applied when `cfg.NATSCredsFile != ""`. |

### JetStream Context

After connecting, `New` obtains the `jetstream.JetStream` handle:


This replaces the older `nc.JetStream()` call and aligns with the API used in IMPLEMENTATION 07.

### `Ping() error`

Calls `c.Conn.FlushTimeout(2 * time.Second)`. Returns an error if the connection is not healthy. Used by `HealthChecker.Readiness` (IMPLEMENTATION 01).

### `Close()`

Calls `c.Conn.Drain()` — flushes pending publishes and waits for active subscriptions to finish. Preferred over `c.Conn.Close()` for graceful shutdown (see IMPLEMENTATION 01, §5 shutdown sequence).

---

## 2. `internal/nats/streams.go` — JetStream Stream Definitions

All JetStream stream and consumer configurations are consolidated here. Called once at startup by `cmd/monstera-fed/serve.go`, after the NATS client connects but before the HTTP server starts.


### FEDERATION Stream

| Field | Value | Rationale |
|-------|-------|-----------|
| Name | `FEDERATION` | |
| Subjects | `["federation.deliver.>"]` | Wildcard captures activity type suffix |
| Retention | `WorkQueuePolicy` | Message deleted after acknowledgement |
| Storage | `FileStorage` | Durable across NATS restarts |
| MaxAge | `72 * time.Hour` | 3-day expiry; stale deliveries are abandoned |
| MaxMsgSize | `4 * 1024 * 1024` (4 MB) | AP activities with embedded attachments can be large |
| Replicas | `1` | Single-node NATS for most deployments; bump for clustered NATS |
| Discard | `DiscardOld` | On limit, drop oldest unprocessed message |

### FEDERATION_DLQ Stream

| Field | Value | Rationale |
|-------|-------|-----------|
| Name | `FEDERATION_DLQ` | |
| Subjects | `["federation.dlq.>"]` | |
| Retention | `LimitsPolicy` | Retained for admin inspection |
| Storage | `FileStorage` | |
| MaxAge | `30 * 24 * time.Hour` | 30-day retention; auto-purged |

### Federation Worker Durable Consumer

`EnsureStreams` also creates the durable pull consumer for the federation worker:

| Field | Value |
|-------|-------|
| Durable | `"federation-worker"` |
| AckPolicy | `AckExplicitPolicy` |
| MaxAckPending | `50` |
| AckWait | `60 * time.Second` |
| MaxDeliver | `5` |
| BackOff | `[]time.Duration{0, 5*time.Minute, 30*time.Minute, 2*time.Hour, 12*time.Hour}` |

### Error Handling

`EnsureStreams` uses `js.CreateOrUpdateStream` (idempotent). If the stream already exists with a compatible config, it updates in place. If the config is incompatible (e.g., subject change on an existing stream), the error propagates and the server aborts startup — this is intentional to prevent silent config drift.

### Refactor Note (IMPLEMENTATION 07)

This consolidation moves stream definitions out of `internal/nats/federation/producer.go`. The `Producer` in that package retains `EnqueueDelivery` and `EnqueueDLQ` but no longer owns `EnsureStreams`. The `FederationWorker` now receives the pre-created consumer rather than creating it — `EnsureStreams` returns the consumer config, and `FederationWorker.Start` calls `js.CreateOrUpdateConsumer` as before.

---

## 3. `SSEEvent` Wire Format

The `SSEEvent` struct is the NATS message payload for all SSE-related pub/sub. It is serialized as JSON, published to NATS core subjects, and deserialized by the Hub on each replica.


### Field Semantics

| Field | Values | Notes |
|-------|--------|-------|
| `Stream` | `"user"`, `"public"`, `"public:local"`, `"hashtag:{tag}"` | Tells the Hub which SSE connection channels to fan out to. A single NATS message may be published to multiple subjects (e.g., a public post goes to both `events.public` and `events.public.local`), but each message carries ONE stream value. |
| `Event` | `"update"`, `"notification"`, `"delete"`, `"filters_changed"` | Maps directly to the SSE `event:` field. |
| `Data` | JSON string | For `update`: the full Mastodon `Status` JSON. For `notification`: the full `Notification` JSON. For `delete`: the status ID as a plain string (not JSON-wrapped). For `filters_changed`: empty string. |

The `Data` field is a **pre-serialized JSON string**, not a nested object. The publisher serializes the domain object once; the Hub writes it directly to the SSE response without re-serialization. This avoids a decode-then-re-encode cycle on every replica.

---

## 4. `internal/nats/streaming/publisher.go` — SSE Event Publisher

The Publisher is called by service-layer code (via the `EventBus` interface — see §5) after committing database writes. It determines which NATS subjects to publish to based on the event type and post visibility, serializes the payload, and fires NATS core pub/sub messages.

### Types and Constructor

`FollowerStore` is a narrow interface to avoid importing the full store package:

Only **local** follower IDs are needed — remote followers don't have SSE connections. This keeps the query efficient (filters on `domain IS NULL` + `state = 'accepted'`).

### Subject Routing by Visibility

#### `PublishUpdate` — new status

| Visibility | NATS Subjects Published To |
|------------|---------------------------|
| `public` | `events.public` + `events.public.local` (if Local) + `stream.user.{followerID}` for each local follower + `events.hashtag.{tag}` for each hashtag |
| `unlisted` | `stream.user.{followerID}` for each local follower + `events.hashtag.{tag}` for each hashtag |
| `private` | `stream.user.{followerID}` for each local follower |
| `direct` | `stream.user.{mentionedAccountID}` for each locally-mentioned account (extracted from `status.Mentions`) |

For `public` and `unlisted`, the `SSEEvent.Stream` value differs per subject:
- Messages to `events.public` carry `Stream: "public"`.
- Messages to `events.public.local` carry `Stream: "public:local"`.
- Messages to `stream.user.*` carry `Stream: "user"`.
- Messages to `events.hashtag.*` carry `Stream: "hashtag:{tag}"`.

This means the publisher constructs and publishes **multiple SSEEvent messages per status** — one per NATS subject. The JSON `Data` payload is serialized once and reused across all messages (only `Stream` differs).

#### `PublishNotification` — new notification

Always publishes to a single subject: `stream.user.{accountID}` with `Stream: "user"`, `Event: "notification"`.

#### `PublishDelete` — status deletion

Full fan-out, matching the original post's routing:

| Visibility | NATS Subjects Published To |
|------------|---------------------------|
| `public` | `events.public` + `events.public.local` (if Local) + `stream.user.{followerID}` for each local follower + `events.hashtag.{tag}` for each hashtag |
| `unlisted` | `stream.user.{followerID}` for each local follower + `events.hashtag.{tag}` for each hashtag |
| `private` | `stream.user.{followerID}` for each local follower |
| `direct` | `stream.user.{mentionedAccountID}` — requires the caller to pass mentioned account IDs (see note below) |

The `Data` field for delete events is the **status ID as a plain string** (not JSON-wrapped), per Mastodon protocol: `Data: "12345"`.

**Direct message deletes:** The `PublishOpts` struct does not carry mentioned account IDs. For `direct` visibility deletes, the caller (service layer) must query `status_mentions` before the soft-delete and pass the mentioned IDs. This is handled by adding `MentionedAccountIDs []string` to `PublishOpts`:


### Metrics

Every NATS publish call increments `monstera-fed_nats_publish_total{subject, result}`:
- `result="ok"` on success.
- `result="error"` on failure (logged at `slog.Error`; error is **not** propagated to the caller — SSE fan-out failures should not fail the user's request).

### Error Handling

NATS core pub/sub is fire-and-forget. If the connection is down, `nc.Publish` returns an error, which the publisher logs and increments the error metric. The caller (service layer) does **not** receive this error — the DB write has already committed, and failing the user request because SSE fan-out failed would be wrong. This matches the at-most-once semantics of the SSE path.

---

## 5. `internal/service/events.go` — EventBus Interface

The service layer publishes events through an `EventBus` interface. This keeps services testable (no NATS dependency) and provides a clean boundary between business logic and infrastructure.


### Relationship to `ap.EventPublisher`

IMPLEMENTATION 07 defined a separate `EventPublisher` interface in `internal/ap/inbox.go`:
This is a **lower-level** interface used by the `InboxProcessor` when processing incoming federation activities. It takes pre-serialized JSON payloads because the inbox already has the raw AP JSON and converts it to Mastodon-format JSON internally.

Both interfaces coexist. The `streaming.Publisher` implements both:

| Interface | Used by | Input format |
|-----------|---------|-------------|
| `service.EventBus` | Service layer (local user actions) | Domain objects (`*mastodon.Status`, etc.) — publisher serializes to JSON |
| `ap.EventPublisher` | Inbox processor (remote activities) | Pre-serialized `json.RawMessage` — publisher wraps in `SSEEvent` and publishes directly |

This avoids double-serialization in the federation path and keeps the service layer working with typed domain objects.

### Service-Layer Usage Pattern

Services call `EventBus` **after committing the database transaction**, never inside it. Example flow in `StatusService.Create`:

1. Begin transaction.
2. Insert status, mentions, hashtags, update counters.
3. Commit transaction.
4. Call `eventBus.PublishUpdate(ctx, presentedStatus, opts)` — fire-and-forget.
5. Call `outbox.Publish(ctx, ...)` — enqueue federation delivery.

If step 4 fails (NATS down), the status is still created. Connected SSE clients miss the real-time event but see it on their next REST poll or reconnect. Federation delivery (step 5) is independent and uses JetStream (durable).

---

## 6. `internal/nats/streaming/hub.go` — SSE Hub

The Hub runs as a long-lived goroutine on each replica. It subscribes to NATS core pub/sub subjects, maintains a registry of connected SSE clients, and fans out incoming events to the appropriate per-connection channels.

### Subscription Lifecycle

The Hub manages two categories of NATS subscriptions:

**Always-on (subscribed at startup via `Start`):**

| NATS Subject | Stream Key |
|-------------|------------|
| `events.public` | `public` |
| `events.public.local` | `public:local` |

These are cheap — every replica needs them regardless of connected clients, and they carry only public timeline events.

**On-demand (subscribed when the first client connects, unsubscribed when the last disconnects):**

| NATS Subject Pattern | Stream Key | Example |
|---------------------|------------|---------|
| `stream.user.{accountID}` | `user:{accountID}` | `user:abc123` |
| `events.hashtag.{tag}` | `hashtag:{tag}` | `hashtag:golang` |

On-demand subscriptions use reference counting via `managedSub.refCount`. This prevents a replica with no connected users from receiving per-user NATS traffic, and avoids accumulating subscriptions for hashtags nobody is watching.

### `Start(ctx context.Context) error`

Called once at startup as a goroutine. Steps:

1. Subscribe to `events.public` and `events.public.local` with a shared message handler.
2. Block until `ctx` is cancelled (shutdown signal).
3. On cancellation, close all subscriber channels and drain NATS subscriptions.

The NATS message handler for each subscription deserializes the `SSEEvent` payload and calls the internal `fanOut` method.

### `Subscribe(streamKey string) (<-chan SSEEvent, func())`

Called by the HTTP handler when a new SSE client connects. Steps:

1. Create a buffered channel: `make(chan SSEEvent, 16)`.
2. Acquire `mu` write lock.
3. Append the subscriber to `subscribers[streamKey]`.
4. If this is the first subscriber for an on-demand stream key, create the NATS subscription (map stream key → NATS subject: `user:abc123` → `stream.user.abc123`, `hashtag:golang` → `events.hashtag.golang`).
5. If it's an existing on-demand subscription, increment `refCount`.
6. Increment `monstera-fed_active_sse_connections{stream=streamKey}` gauge.
7. Return the channel and a cancel function.

The **cancel function** (returned to the caller, invoked on client disconnect):

1. Acquire `mu` write lock.
2. Remove this subscriber from `subscribers[streamKey]`.
3. Close the subscriber's channel.
4. For on-demand subscriptions: decrement `refCount`. If zero, call `sub.Unsubscribe()` and remove from `natsSubs`.
5. Decrement `monstera-fed_active_sse_connections{stream=streamKey}` gauge.

### `fanOut(streamKey string, event SSEEvent)`

Called by the NATS message handler whenever a message arrives on a subscribed subject.

1. Acquire `mu` read lock.
2. Iterate over `subscribers[streamKey]`.
3. For each subscriber, attempt a non-blocking send on the channel:


The non-blocking send with `default` ensures a slow client cannot back-pressure the fan-out loop or block delivery to other subscribers. Dropped events are acceptable under the at-most-once contract — clients backfill via REST.

### Stream Key ↔ NATS Subject Mapping

The Hub uses deterministic mapping between stream keys (used internally and in the subscriber map) and NATS subjects:

| Stream Key Format | NATS Subject |
|-------------------|-------------|
| `public` | `events.public` |
| `public:local` | `events.public.local` |
| `user:{accountID}` | `stream.user.{accountID}` |
| `hashtag:{tag}` | `events.hashtag.{tag}` |

Helper functions `streamKeyToSubject(key string) string` and `subjectToStreamKey(subject string) string` handle conversion. The colon delimiter in stream keys avoids ambiguity with the dot-delimited NATS subject namespace.

### Shutdown

When `Start`'s context is cancelled:

1. Unsubscribe all NATS subscriptions (both always-on and on-demand).
2. Acquire `mu` write lock.
3. Close every subscriber channel. The HTTP handlers see the closed channel and return, ending the SSE connection gracefully.
4. Clear the `subscribers` and `natsSubs` maps.

This integrates with the shutdown sequence from IMPLEMENTATION 01, §5 — the Hub is shut down after the HTTP server stops accepting new connections but before NATS is drained.

---

## 7. `internal/api/mastodon/streaming.go` — SSE HTTP Handlers

### `StreamingTokenFromQuery` Middleware
Runs before `OptionalAuth` on the `/api/v1/streaming` route group. Copies the `access_token` query parameter into the `Authorization` header so the existing auth middleware can resolve it.

Scoped strictly to streaming routes — query-param auth is not enabled on REST endpoints (tokens in URLs appear in access logs and browser history).

### Route Registration

Updates the streaming route group from IMPLEMENTATION 08, §13:


### `ServeSSE` — Reusable SSE Helper

All streaming endpoints delegate to this shared method after determining the stream key:


**Steps:**

1. **Assert `http.Flusher`** — the `ResponseWriter` must implement `http.Flusher`. If not (e.g., certain test harnesses), return `501 Not Implemented`.

2. **Set headers:**

| Header | Value | Purpose |
|--------|-------|---------|
| `Content-Type` | `text/event-stream` | SSE content type |
| `Cache-Control` | `no-cache` | Prevent caching |
| `Connection` | `keep-alive` | Long-lived connection |
| `X-Accel-Buffering` | `no` | Disable NGINX response buffering |

3. **Write initial comment** — `fmt.Fprintf(w, ":)\n\n")` and flush. This sends data immediately, confirming to the client that the SSE connection is established and triggering the `EventSource.onopen` callback. The `:)` is a convention from Mastodon's streaming server.

4. **Subscribe** — call `hub.Subscribe(streamKey)` to get the event channel and cancel function. Defer the cancel function.

5. **Event loop:**


The `r.Context().Done()` channel fires when the client closes the connection (TCP FIN/RST), which chi/net/http detects and cancels the request context.

### Endpoint Handlers

#### `GET /api/v1/streaming/health`


Returns `200 OK` with plain text body `OK`. No JSON. Used by clients to probe streaming endpoint reachability before opening an SSE connection.

#### `GET /api/v1/streaming/user`


- **Auth:** Required. Extract account from context (`middleware.AccountFromContext`). Return `401` if nil.
- **Stream key:** `user:{accountID}`
- Delegates to `ServeSSE`.

Events delivered: `update` (home timeline posts from followed accounts), `notification`, `delete`.

#### `GET /api/v1/streaming/public`


- **Auth:** Optional (auth enables mute/block filtering — deferred to Phase 2; see Open Questions).
- **Query params:** `?local=true` switches to the local-only stream.
- **Stream key:** `public:local` if `local=true`, otherwise `public`.
- Delegates to `ServeSSE`.

Events delivered: `update` (public posts), `delete`.

#### `GET /api/v1/streaming/public/local`


- **Auth:** Optional.
- **Stream key:** `public:local`
- Delegates to `ServeSSE`.

Convenience endpoint — equivalent to `/public?local=true`.

#### `GET /api/v1/streaming/hashtag`


- **Auth:** Optional.
- **Query params:** `?tag=foo` (required). Return `400` if missing.
- **Tag normalization:** lowercase, strip leading `#` if present.
- **Stream key:** `hashtag:{normalizedTag}`
- Delegates to `ServeSSE`.

Events delivered: `update` (posts containing the hashtag), `delete`.

---

## 8. Metrics Instrumentation

### `monstera-fed_active_sse_connections` (Gauge)

- **Labels:** `stream` — the stream key category: `user`, `public`, `public:local`, `hashtag`.
  - For `user:{accountID}` and `hashtag:{tag}`, the label is the prefix only (`user`, `hashtag`) to prevent unbounded cardinality.
- **Incremented:** in `Hub.Subscribe` when a new client connects.
- **Decremented:** in the cancel function when a client disconnects.

### `monstera-fed_nats_publish_total` (Counter)

- **Labels:** `subject`, `result` (`ok` | `error`).
- **Incremented:** in `Publisher.PublishUpdate`, `PublishNotification`, `PublishDelete` on every `nc.Publish` call.
- **Subject label cardinality:** uses the NATS subject pattern, not the full subject. E.g., `events.public`, `events.public.local`, `stream.user.*`, `events.hashtag.*`. The wildcard replaces the variable segment to keep cardinality bounded.

Both metrics are defined in `observability.Metrics` (IMPLEMENTATION 01) and already have registry entries. This design specifies where they are incremented/decremented.

---

## 9. Configuration Addenda

No new environment variables are needed. The NATS connection is configured via the existing `NATS_URL` and `NATS_CREDS_FILE` (IMPLEMENTATION 01, §2). The SSE keepalive interval (30s) and channel buffer size (16) are compile-time constants — they don't need runtime configurability for Phase 1.

### Startup Wiring Update (`cmd/monstera-fed/serve.go`)

After the existing NATS connection and `EnsureStreams` call, add:

1. **Create Publisher:** `streaming.NewPublisher(natsClient.Conn, store, metrics, logger, cfg.InstanceDomain)`
2. **Create Hub:** `streaming.NewHub(natsClient.Conn, metrics, logger)`
3. **Start Hub:** `go hub.Start(ctx)` — runs until shutdown context is cancelled.
4. **Wire EventBus:** Pass the Publisher as the `service.EventBus` implementation to all services that need it (`StatusService`, `AccountService`, etc.).
5. **Wire ap.EventPublisher:** Pass the same Publisher to `ap.NewInboxProcessor` (it satisfies both interfaces).
6. **Create StreamingHandler:** `mastodon.NewStreamingHandler(hub, logger, cfg.InstanceDomain)`

### Shutdown Sequence Update

Inserts into the existing shutdown order from IMPLEMENTATION 01, §5:

1. HTTP drain (existing) — in-flight SSE handlers see `r.Context().Done()` and return.
2. **Stop Hub** — cancel the Hub's context. Hub unsubscribes from NATS, closes all subscriber channels.
3. Stop federation workers (existing).
4. Drain NATS (existing).
5. Close DB (existing).

---

## 10. Schema Addenda

### New Query: `GetLocalFollowerIDs`


Added to `internal/store/postgres/queries/follows.sql`. Returns only local follower account IDs — remote followers are irrelevant for SSE fan-out.

### New Query: `GetStatusMentionAccountIDs`


Used by `StatusService.Delete` to populate `PublishOpts.MentionedAccountIDs` for direct-visibility deletes before soft-deleting the status.
