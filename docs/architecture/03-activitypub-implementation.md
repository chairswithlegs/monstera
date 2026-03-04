# ActivityPub implementation

This document describes how ActivityPub server-to-server (S2S) and discovery are implemented.

## Overview

Monstera implements the ActivityPub protocol for federation: discovery (WebFinger, NodeInfo), actor documents, collections (outbox, followers, following, featured), and inbox processing. Outbound delivery to remote servers uses NATS JetStream for durable, at-least-once delivery.

## Discovery

| Endpoint | Handler | Purpose |
|----------|---------|---------|
| `GET /.well-known/webfinger?resource=acct:user@domain` | `WebFingerHandler` | Returns JRD with `subject`, `links` (self, self with type application/activity+json). Used by clients and remote servers to resolve account to actor URL. |
| `GET /.well-known/nodeinfo` | `NodeInfoPointerHandler` | Returns document with `links` to NodeInfo 2.0. |
| `GET /nodeinfo/2.0` | `NodeInfoHandler` | Returns NodeInfo 2.0 JSON (software, usage, etc.). |

Handlers and types live in `internal/api/activitypub/` (webfinger.go, nodeinfo.go) and the activitypub apimodel package.

## Actor and collections

| Endpoint | Handler | Purpose |
|----------|---------|---------|
| `GET /users/{username}` | `ActorHandler` | Returns ActivityStreams Actor document (JSON-LD). Used for S2S and client profile views. |
| `GET /users/{username}/outbox` | `OutboxHandler` | OrderedCollection of Create(Note) activities (public statuses). Paginated. |
| `GET /users/{username}/followers` | `CollectionsHandler` (GETFollowers) | Collection of followers. |
| `GET /users/{username}/following` | `CollectionsHandler` (GETFollowing) | Collection of following. |
| `GET /users/{username}/collections/featured` | `CollectionsHandler` (GETFeatured) | Pinned posts; currently returns empty collection. |

Actor and collection types are in `internal/activitypub/vocab.go` and helpers. The store provides the data; handlers map domain types to ActivityStreams JSON.

## Inbox

| Endpoint | Purpose |
|----------|---------|
| `POST /users/{username}/inbox` | Per-user inbox (actor-specific). |
| `POST /inbox` | Shared inbox. |

Both are handled by `InboxHandler.POSTInbox` (`internal/api/activitypub/`). The handler:

1. Verifies HTTP Signature (RFC 9421) using the sender’s public key (fetched from actor document, cached).
2. Parses the request body as ActivityStreams JSON.
3. Dispatches to `InboxProcessor.Process` (`internal/activitypub/inbox.go`), which handles supported activity types (e.g. Create(Note), Follow, Accept(Follow), Undo(Follow), Delete(Note), Update(Note)).
4. Returns 202 Accepted after processing.

Processing is synchronous: no queue. Remote actor resolution and side effects (store writes, notifications, SSE) happen on the request goroutine. See roadmap for async inbox options.

## Outbox and federation delivery

When a local user creates a status, follows, or deletes a status, the server must deliver the corresponding ActivityPub activity to the inboxes of remote followers/servers. This is done asynchronously:

1. **Outbox publisher** (`internal/activitypub/outbox.go`): Builds the activity (e.g. Create with Note), signs it, and enqueues a delivery job to NATS JetStream (subject `federation.deliver.*`).
2. **Federation worker** (`internal/activitypub/outbox_fanout_worker.go`): Consumes from the JetStream stream, fetches target inbox URLs (per follower or deduplicated by inbox), and POSTs the activity with HTTP Signature. Retries and dead-letter behaviour are configured on the stream.

Inbox URLs are resolved from the store (e.g. follower list); deduplication is by raw inbox URL (shared_inbox coalescence is a planned improvement; see roadmap).

## Supported activity types (inbox)

- **Create(Note)**: Ingest status; resolve author if remote; store; notify mentions; publish to SSE.
- **Follow / Accept(Follow) / Undo(Follow)**: Follow relationship creation and acceptance/undo.
- **Delete(Note)** / **Delete(Person)**: Soft delete or actor removal semantics.
- **Update(Note)**: Edit handling (store and federation side).
- **Like / Announce**: Favourite and reblog.

Blocklist and domain blocks are applied during inbox processing (`internal/activitypub/blocklist.go`). HTTP Signature verification and key resolution are in `internal/activitypub/httpsignature.go`.

## Key files

| File | Responsibility |
|------|----------------|
| `internal/api/activitypub/webfinger.go`, `nodeinfo.go` | WebFinger and NodeInfo handlers. |
| `internal/api/activitypub/actor.go` | Actor document handler. |
| `internal/api/activitypub/collections.go` | Outbox, followers, following, featured handlers. |
| `internal/api/activitypub/outbox.go` | Build and enqueue outbound activities. |
| `internal/activitypub/inbox.go` | Inbox processing and activity dispatch. |
| `internal/activitypub/httpsignature.go` | Sign and verify HTTP Signature. |
| `internal/activitypub/outbox_fanout_worker.go` | JetStream consumer and POST to remote inboxes. |
| `internal/activitypub/vocab.go` | ActivityStreams types. |
| `internal/activitypub/streams.go` | NATS stream definitions (e.g. FEDERATION). |
