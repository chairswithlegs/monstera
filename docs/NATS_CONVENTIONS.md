# Monstera-fed — NATS Subject & Stream Conventions

> Naming patterns for NATS JetStream streams and core pub/sub subjects.
> For full implementation details see ADR 07 (federation) and ADR 09 (SSE streaming).

---

## Subject Hierarchy

All Monstera-fed subjects use a dot-separated hierarchy. The first token identifies the subsystem.

```
{subsystem}.{resource}.{qualifier}
```

| Subsystem | Transport | Purpose |
|-----------|-----------|---------|
| `federation` | JetStream (durable, at-least-once) | Activity delivery to remote inboxes |
| `events` | Core pub/sub (ephemeral, at-most-once) | SSE event fan-out across replicas |

---

## JetStream Streams

### `FEDERATION`

Durable work queue for outbound ActivityPub delivery.

| Property | Value |
|----------|-------|
| Stream name | `FEDERATION` |
| Subjects | `federation.deliver.>` |
| Retention | WorkQueue (deleted after ack) |
| Storage | File |
| Consumer | `federation-worker` (pull, durable) |

**Subject pattern:** `federation.deliver.{activityType}`

The activity type suffix (e.g., `create`, `delete`, `follow`, `undo`) is informational — the consumer subscribes to the wildcard `federation.deliver.>` and dispatches based on the message payload. The suffix enables subject-level filtering in NATS tooling and monitoring.

Examples:
```
federation.deliver.create
federation.deliver.delete
federation.deliver.follow
federation.deliver.undo
federation.deliver.accept
federation.deliver.reject
federation.deliver.announce
federation.deliver.like
federation.deliver.block
federation.deliver.update
```

### `FEDERATION_DLQ`

Dead-letter queue for deliveries that exhausted retries (5 attempts).

| Property | Value |
|----------|-------|
| Stream name | `FEDERATION_DLQ` |
| Subjects | `federation.dlq.>` |
| Retention | Limits (retained for admin inspection) |
| Storage | File |

**Subject pattern:** `federation.dlq.{activityType}` — mirrors the original delivery subject.

---

## Core Pub/Sub Subjects (SSE)

Ephemeral fan-out for real-time streaming. No JetStream — messages are fire-and-forget. If a replica isn't subscribed, the message is dropped (clients backfill via REST on reconnect).

**Subject pattern:** `events.{channel}.{qualifier?}`

| Subject | Channel value in SSEEvent | Published when |
|---------|--------------------------|----------------|
| `events.public` | `public` | Public status created or deleted |
| `events.public.local` | `public:local` | Local public status created or deleted |
| `events.user.{accountID}` | `user` | Status visible to user, or notification |
| `events.hashtag.{tag}` | `hashtag:{tag}` | Status with hashtag (public/unlisted) |

A single status may publish to multiple subjects simultaneously. For example, a public local post with `#golang` publishes to:
- `events.public`
- `events.public.local`
- `events.user.{followerID}` (one per local follower)
- `events.hashtag.golang`

The SSE Hub on each replica subscribes to subjects on demand (when a client opens an SSE connection) and unsubscribes when the last client for that subject disconnects.

---

## Naming Rules

1. **Lowercase, dot-separated.** No camelCase, no underscores in subjects (underscores are reserved for stream names).
2. **Stream names are UPPER_SNAKE_CASE.** `FEDERATION`, `FEDERATION_DLQ`.
3. **Consumer names are lower-kebab-case.** `federation-worker`.
4. **Use `>` wildcards only in stream subject filters**, never in publish calls. Publish to an exact subject.
5. **Subsystem prefix is mandatory.** Every subject starts with its subsystem token. This prevents collisions and makes NATS monitoring dashboards readable.
6. **New subsystems get their own stream.** If a future feature needs durable delivery (e.g., `media.fetch` for lazy remote media), create a new JetStream stream rather than overloading `FEDERATION`.

---

## Adding a New Stream (Checklist)

When adding a new JetStream stream:

1. Define the stream name and subject pattern following the conventions above.
2. Add the `CreateOrUpdateStream` call to `internal/nats/streams.go` inside `EnsureStreams`.
3. Define the message payload struct in the relevant package.
4. Create a producer (publisher) and consumer (worker) following the federation pattern.
5. Document the stream in the table above.
