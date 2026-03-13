# SSE (Server-Sent Events) streaming

This document describes how real-time streaming works for the Mastodon-compatible streaming API.

## Overview

Clients connect to endpoints such as `GET /api/v1/streaming/user`, `GET /api/v1/streaming/public`, and `GET /api/v1/streaming/hashtag?tag=foo`. The server holds the connection open and sends SSE frames (`event:` and `data:` lines). Events are driven by NATS core pub/sub: the application publishes to NATS when something happens (new status, notification, delete); a per-process Hub subscribes to NATS and fans out to connected SSE clients.

## Flow

1. **Client** opens a long-lived GET to e.g. `/api/v1/streaming/user` with auth (query param `access_token` or `Authorization: Bearer`; see `middleware.StreamingTokenFromQuery`).
2. **StreamingHandler** (`internal/api/mastodon/streaming.go`) resolves the stream key (e.g. `user:{accountID}` for user stream, `public` or `public:local` for public, `hashtag:{tag}` for hashtag).
3. **Hub** (`internal/events/sse/hub.go`): `Subscribe(streamKey)` creates a channel for this client and, if needed, subscribes to the corresponding NATS subject. The handler reads from the channel and writes SSE frames (`event: <type>\ndata: <payload>\n\n`).
4. **Keepalive**: Every 30 seconds the server sends a comment line `:keepalive\n\n` to prevent proxy timeouts.
5. **Publishing**: Domain events flow through the transactional outbox (see [06-domain-events-and-outbox.md](06-domain-events-and-outbox.md)). The **SSE subscriber** (`internal/events/sse/subscriber.go`) consumes from the `domain-events-sse` NATS JetStream consumer and translates events into Mastodon API JSON. It publishes `SSEEvent` messages to NATS core subjects based on visibility: public, public:local, per-follower user streams, and per-hashtag streams. The Hub receives the NATS message, decodes the event, and sends it to all subscribers for that stream key.

## Stream keys and NATS subjects

| Stream key | NATS subject | Used by |
|------------|--------------|---------|
| `public` | `events.public` | All public statuses |
| `public:local` | `events.public.local` | Local-only public |
| `user:{accountID}` | `events.user.{accountID}` | Home timeline for that account |
| `hashtag:{tag}` | `events.hashtag.{tag}` | Hashtag timeline |

Constants and mapping live in `internal/events/sse/event.go` (`StreamKeyToSubject`, `SubjectToStreamKey`).

## Event types

- **update**: New status (or notification payload). `data` is the Mastodon JSON for the status or notification.
- **delete**: Status deleted. `data` is the status ID.
- **notification**: Dedicated notification event for the user stream.

## Auth

- **User stream** (`/api/v1/streaming/user`): Requires auth; stream key is `user:{accountID}` from the authenticated account.
- **Public / public local / hashtag**: Optional auth.

## Key files

| File | Responsibility |
|------|----------------|
| `internal/api/mastodon/streaming.go` | HTTP handlers; stream key selection; `serveSSE` (write headers, subscribe to Hub, loop write/keepalive). |
| `internal/api/middleware/streaming_auth.go` | Copies `access_token` query param into `Authorization: Bearer` so RequireAuth/OptionalAuth work. |
| `internal/events/sse/hub.go` | Subscribe/Unsubscribe; NATS subscriptions (always-on for public/public:local; on-demand for user/hashtag); fan-out to channels. |
| `internal/events/sse/event.go` | `SSEEvent` struct; stream key ↔ NATS subject mapping. |
| `internal/events/sse/subscriber.go` | Consumes domain events from `DOMAIN_EVENTS` JetStream; translates to Mastodon API JSON and publishes to NATS core subjects for the Hub. |

## Scaling

Each API process has one Hub. NATS core pub/sub is at-most-once; if no process is subscribed to a subject, messages are dropped. For horizontal scaling, every replica subscribes to the same NATS subjects, so each SSE connection is tied to one replica. Clients reconnect on disconnect and backfill via REST as needed.
