# Domain events and transactional outbox

This document describes the event-driven architecture that decouples the service layer from federation, SSE, notifications, and push delivery.

## Solution: transactional outbox

Services write **domain events** to an `outbox_events` table within the same database transaction as the domain change. A **poller** reads unpublished events and publishes them to a NATS JetStream stream. Independent **subscribers** (federation, SSE, notifications, push delivery) consume events and perform their work. Services are fully ignorant of downstream consumers. The inbox is a thin AP-to-service translation layer.

```
  Service Layer                  Outbox Poller           NATS (DOMAIN_EVENTS)
  ┌──────────┐                  ┌───────────┐          ┌───────────────────┐
  │ domain   │  same tx  ┌────>│ poll DB   │ publish  │                   │
  │ write +  │───────────┤     │ mark sent │─────────>│  domain.events.>  │
  │ outbox   │           │     └───────────┘          │                   │
  │ INSERT   │           │                            └─────┬───────┬─────┘
  └──────────┘           │                                  │       │
                         │                     ┌────────────┘       └──────────┐
                         │                     ▼                               ▼
                         │            ┌─────────────────┐            ┌──────────────┐
                         │            │ Federation Sub  │            │  SSE Sub     │
                         │            │ builds AP JSON  │            │ builds SSE   │
                         │            │ enqueues to     │            │ publishes to │
                         │            │ delivery/fanout │            │ NATS core    │
                         │            │ streams         │            │ for Hub      │
                         │            └────────┬────────┘            └──────────────┘
                         │                     │
                         │    ┌────────────────┼────────────────┐
                         │    ▼                ▼                ▼
                         │  existing       existing         (calls service
                         │  delivery       fanout           for follower
                         │  worker         worker           inbox URLs)
                         │
                     outbox_events table (Postgres)
```

## Event types

Defined in `internal/domain/events.go`.

| Event | Subscribers | Trigger |
|-------|-------------|---------|
| `status.created` | Federation, SSE, Notifications | Local status published |
| `status.updated` | Federation | Local status edited |
| `status.deleted` | Federation, SSE | Local status deleted |
| `status.created.remote` | SSE, Notifications | Remote status ingested via inbox |
| `status.updated.remote` | SSE | Remote status edited via inbox |
| `status.deleted.remote` | SSE | Remote status deleted via inbox |
| `follow.created` | Federation, Notifications | User follows someone |
| `follow.removed` | Federation | User unfollows |
| `follow.accepted` | Federation | Follow request accepted |
| `follow.requested` | Notifications | Remote follow request received |
| `favourite.created` | Federation, Notifications | Status favourited |
| `favourite.removed` | Federation | Favourite removed (undo) |
| `reblog.created` | Federation, Notifications | Status reblogged |
| `reblog.removed` | Federation | Reblog removed (undo) |
| `block.created` | Federation | User blocks someone |
| `block.removed` | Federation | User unblocks |
| `account.updated` | Federation | Local profile updated |
| `notification.created` | SSE, Push delivery | Notification created |

Each event payload carries the full domain objects needed by subscribers (status, author, mentions, hashtags, media attachments), so subscribers rarely need additional queries.

## Components

### Outbox table

The `outbox_events` table stores events atomically with the domain change:

```sql
CREATE TABLE outbox_events (
    id             TEXT PRIMARY KEY,
    event_type     TEXT NOT NULL,
    aggregate_type TEXT NOT NULL,
    aggregate_id   TEXT NOT NULL,
    payload        JSONB NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at   TIMESTAMPTZ
);
```

An index on `(created_at ASC) WHERE published_at IS NULL` supports efficient polling.

### Event emission (`internal/events/events.go`)

Services call `events.EmitEvent()` inside their `WithTx` blocks:

```go
events.EmitEvent(ctx, tx, domain.EventStatusCreated, "status", status.ID, payload)
```

This marshals the payload, generates a ULID for the event ID, and inserts into `outbox_events` — all within the same transaction as the domain write.

### Outbox poller (`internal/events/outbox_poller.go`)

A background worker polls every 500ms:

1. `GetAndLockUnpublishedOutboxEvents()` — `SELECT ... FOR UPDATE SKIP LOCKED LIMIT 100`
2. Publishes each event to NATS subject `domain.events.<eventType>` with `Nats-Msg-Id` set to the event ID (dedup)
3. `MarkOutboxEventsPublished()` — marks events as published

Safe for multiple instances: `SKIP LOCKED` prevents contention, NATS dedup (5-minute window) prevents duplicate delivery.

### NATS stream (`internal/events/streams.go`)

| Setting | Value |
|---------|-------|
| Stream | `DOMAIN_EVENTS` |
| Subjects | `domain.events.>` |
| Retention | Interest-based (deleted when all consumers ACK) |
| Dedup window | 5 minutes |
| Max age | 72 hours |

Four durable pull consumers:

| Consumer | Purpose | Filter | MaxAckPending |
|----------|---------|--------|---------------|
| `domain-events-federation` | Translates events to AP activities | all events | 50 |
| `domain-events-sse` | Fans out to SSE clients | all events | 100 |
| `domain-events-notifications` | Creates notifications reactively | all events | 50 |
| `domain-events-push-delivery` | Sends Web Push notifications | `notification.*` only | 50 |

### Federation subscriber (`internal/activitypub/federation_subscriber.go`)

Consumes from the `domain-events-federation` consumer. For each event:

- **Status events** → builds AP `Create{Note}`, `Update{Note}`, or `Delete{Tombstone}` and enqueues to the **fanout stream** (broadcast to followers)
- **Follow/block events** → builds AP `Follow`, `Undo{Follow}`, `Accept{Follow}`, `Block`, `Undo{Block}` and enqueues to the **delivery stream** (single inbox)
- **Favourite/reblog events** → builds AP `Like`, `Undo{Like}`, `Announce`, `Undo{Announce}` and enqueues to the **delivery stream**
- **Account updated** → builds AP `Update{Person}` and enqueues to the fanout stream
- **Non-federation events** → ACK and skip

The existing delivery and fanout workers handle the actual HTTP POST to remote inboxes.

### SSE subscriber (`internal/api/mastodon/sse/subscriber.go`)

Consumes from the `domain-events-sse` consumer. For each event:

- **Status created** → marshals to Mastodon API JSON, routes to visibility-based NATS core subjects (public, public:local, user streams, hashtag streams, list streams, direct streams)
- **Status deleted** → publishes delete events to the same subjects
- **Notification created** → enriches with status data and publishes to the user's stream
- **Non-SSE events** → ACK and skip

The SSE Hub (`internal/api/mastodon/sse/hub.go`) subscribes to these NATS core subjects and fans out to connected clients.

### Notification subscriber (`internal/events/notification_subscriber.go`)

Consumes from the `domain-events-notifications` consumer. Centralizes all notification creation, removing it from inline service code and inbox handlers. For each event:

- **Follow / follow request** → creates a follow or follow-request notification for the target if local
- **Favourite / reblog** → creates a notification for the status author if local (skips self-interactions)
- **Status created (local or remote)** → creates mention notifications for local mentioned accounts (respects conversation muting)

See [07-notifications-and-push.md](07-notifications-and-push.md) for details.

### Push delivery subscriber (`internal/events/push_delivery_subscriber.go`)

Consumes from the `domain-events-push-delivery` consumer (filtered to `notification.*` subjects only). When a `notification.created` event arrives, it looks up the recipient's Web Push subscriptions and delivers a push notification to each subscribed endpoint that has the relevant alert type enabled.

See [07-notifications-and-push.md](07-notifications-and-push.md) for details.

### Cleanup

A scheduled job (`cleanup-outbox-events`, hourly) deletes published events older than 24 hours via `DeletePublishedOutboxEventsBefore`.

## Failure modes

| Scenario | Behaviour |
|----------|-----------|
| NATS down during poll | Transaction rolls back; events retry on next poll |
| Subscriber fails to process | NAK with backoff; NATS redelivers |
| Duplicate publish | NATS dedup window (5 minutes) prevents duplicate delivery |
| App crash mid-poll | Unpublished events remain in DB; next poll picks them up |

## Key files

| File | Responsibility |
|------|----------------|
| `internal/domain/events.go` | Event type constants and payload structs |
| `internal/service/outbox.go` | `emitEvent()` helper used within service transactions |
| `internal/events/outbox_poller.go` | Background poller: DB → NATS |
| `internal/events/streams.go` | NATS stream and consumer configuration |
| `internal/activitypub/federation_subscriber.go` | Domain events → AP activities |
| `internal/api/mastodon/sse/subscriber.go` | Domain events → SSE fan-out |
| `internal/events/notification_subscriber.go` | Domain events → notification creation |
| `internal/events/push_delivery_subscriber.go` | Notification events → Web Push delivery |
| `internal/scheduler/jobs/jobs.go` | `CleanupOutboxEvents` job |
