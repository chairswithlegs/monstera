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

Processing is synchronous: no queue. Remote actor resolution and store writes happen on the request goroutine. SSE delivery and notification events are handled asynchronously via domain events (the inbox calls service methods that emit events). See roadmap for async inbox options.

## Outbox and federation delivery

When a local user creates a status, follows, or deletes a status, the server must deliver the corresponding ActivityPub activity to the inboxes of remote followers/servers. This is done asynchronously via the **domain event system** (see [06-domain-events-and-outbox.md](06-domain-events-and-outbox.md)):

1. **Service layer**: Writes a domain event (e.g. `status.created`, `follow.created`) to the `outbox_events` table within the same database transaction as the domain change.
2. **Outbox poller** (`internal/events/outbox_poller.go`): Polls for unpublished events and publishes them to the `DOMAIN_EVENTS` NATS JetStream stream.
3. **Federation subscriber** (`internal/activitypub/federation_subscriber.go`): Consumes domain events and translates them into ActivityPub activities (e.g. `Create{Note}`, `Follow`, `Accept{Follow}`). Enqueues activities to either the **fanout stream** (broadcast to all followers) or the **delivery stream** (single target inbox).
4. **Fanout worker** (`internal/activitypub/internal/outbox_fanout_worker.go`): Consumes from the fanout stream, paginates through follower inbox URLs, and enqueues individual deliveries.
5. **Delivery worker** (`internal/activitypub/internal/outbox_delivery_worker.go`): Consumes from the delivery stream and POSTs activities to remote inboxes with HTTP Signature. Retries and dead-letter behaviour are configured on the stream.

Inbox URLs are resolved from the store (e.g. follower list); deduplication is by raw inbox URL (shared_inbox coalescence is a planned improvement; see roadmap).

The `activitypub` package has no direct dependency on `store.Store` — it uses narrow interfaces for the specific data it needs (follower inbox URLs, domain blocks).

### Account deletion: snapshot side tables

Account deletion is a tx that hard-deletes `accounts` and relies on Postgres `ON DELETE CASCADE` to remove every dependent row — statuses, follows, oauth tokens, media, etc. (see migration `000080_cascade_account_fks`). That wipes the two things the federation flow normally depends on: the follower inboxes (joined from `follows` + `accounts`) and the sender's private key (stored on `accounts.private_key`). Without a workaround, the `Delete{Actor}` fanout for a deleted account would run against an empty follower list and a non-existent signer.

To let federation run after the row is gone, the service-layer `deleteLocalAccount` (`internal/service/account_service.go`) populates two side tables inside the same tx, **before** the `DELETE` fires (see migration `000081_account_deletion_snapshots`):

- `account_deletion_snapshots(id PK, ap_id, private_key_pem, created_at, expires_at)` — one row per deletion, keyed by a ULID `deletion_id`. Holds the actor IRI and PEM-encoded private key so the delivery worker can sign the `Delete{Actor}` after the `accounts` row is gone.
- `account_deletion_targets(deletion_id FK, inbox_url, delivered_at, PK(deletion_id, inbox_url))` — one row per distinct remote follower inbox. Populated via `SELECT DISTINCT a.inbox_url FROM follows f JOIN accounts a ON f.account_id = a.id WHERE f.target_id = <dying> AND a.domain IS NOT NULL` while `follows` is still live. Targets CASCADE with their snapshot.

The `EventAccountDeleted` payload (`internal/domain/events.go`) carries only `{DeletionID, APID, Local}` — the private key never hits `outbox_events` or the NATS stream. The federation subscriber branches on `DeletionID`:

- **Fanout**: `OutboxFanoutMessage.DeletionID` routes `outbox_fanout_worker` to `AccountDeletionService.ListPendingTargets` instead of `RemoteFollowService.GetFollowerInboxURLsPaginated`, paginating from `account_deletion_targets` and marking each target `delivered_at` as deliveries are enqueued.
- **Delivery**: `OutboxDeliveryMessage.DeletionID` routes `outbox_delivery_worker` to `HTTPSignatureService.SignWithDeletionID`, which loads the PEM from `account_deletion_snapshots` instead of the (missing) `accounts` row.

A scheduler job, `PurgeAccountDeletionSnapshots` (`internal/scheduler/jobs/purge_account_deletion_snapshots.go`), sweeps snapshots past `expires_at` (default `service.AccountDeletionSnapshotTTL` = 24h) on an hourly interval. CASCADE drops the targets along with the snapshot, reclaiming the private-key material.

Concurrent-delete races are closed at the tx level: `deleteLocalAccount` opens the tx with `SELECT ... FOR UPDATE` on the `accounts` row, so a second caller blocks on the row lock and sees `ErrNotFound` after the first commits — no duplicate `EventAccountDeleted`, no duplicate audit row.

## Supported activity types (inbox)

- **Create(Note)**: Ingest status; resolve author if remote; store; notify mentions; publish to SSE.
- **Follow / Accept(Follow) / Undo(Follow)**: Follow relationship creation and acceptance/undo.
- **Delete(Note)** / **Delete(Person)**: Soft delete or actor removal semantics.
- **Update(Note)**: Edit handling (store and federation side).
- **Like / Announce**: Favourite and reblog.

Blocklist and domain blocks are applied during inbox processing. HTTP Signature verification and key resolution are in `internal/activitypub/httpsignature.go`.
