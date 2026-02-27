# ActivityPub and federation

This document describes the desired ActivityPub inbox/outbox, HTTP Signatures, federation worker, and supported activity types. Build order is in [roadmap.md](../roadmap.md).

---

## Design decisions

| Question | Decision |
|----------|----------|
| Polymorphic `object` field | **`json.RawMessage`** with typed accessor methods — avoids custom `UnmarshalJSON` complexity; callers decode on demand |
| `@context` value | **Array literal** — `["https://www.w3.org/ns/activitystreams", "https://w3id.org/security/v1", mastodonExtensions]` |
| Mastodon extensions in context | Phase 1: `sensitive`, `manuallyApprovesFollowers`, `Hashtag`, `toot:Emoji`. Deferred: `movedTo` (Phase 2 account migration), `featured`, `featuredTags` |
| `to`/`cc` addressing | `[]string` with constant `PublicAddress = "https://www.w3.org/ns/activitystreams#Public"` |
| Content negotiation on `GET /users/:username` | **AP JSON for all requests** — Monstera-fed has no HTML profile (users bring their own client). `Content-Type: application/activity+json` always. |
| Inbox processing mode | **Synchronous** on the HTTP handler goroutine for Phase 1. Async goroutine pool is a Phase 2 enhancement. |
| Remote media on `Create{Note}` | **Store `remote_url` only** — no fetch in Phase 1. Lazy-fetch on first access is Phase 2. |
| Idempotency | `INSERT … ON CONFLICT (ap_id) DO NOTHING` — duplicate activities silently ignored |
| `Undo{Like}` activity ID tracking | **`ap_id` column on `favourites`** — consistent with `follows.ap_id` and `statuses.ap_id`; enables exact match on incoming `Undo{Like}` by activity ID, not just `(actor, object)` heuristic |
| `Undo{Announce}` tracking | Boosts are stored as statuses with `ap_id` — already covered |
| Shared inbox delivery | **Deduplicate by inbox URL** — one delivery per unique shared inbox, not per follower |
| `featured` collection | **Empty stub** in Phase 1 — prevents 404 from remote instances fetching pinned posts |
| Federation worker consumer type | **Pull consumer** — backpressure-friendly; configurable concurrency |
| NATS delivery subject | `federation.deliver.{activityType}` — e.g. `federation.deliver.create`, `federation.deliver.follow` |

---

## Architecture overview

```mermaid
flowchart TD
    subgraph remote [Remote Instance]
        RemoteActor["Remote Actor"]
    end

    subgraph monstera [Monstera-fed]
        subgraph discovery [Discovery Layer]
            WF["WebFinger Handler"]
            NI["NodeInfo Handler"]
            ActorH["Actor Handler"]
        end

        subgraph inbound [Inbound Federation]
            InboxH["Inbox HTTP Handler"]
            SigVerify["HTTP Sig Verify"]
            IP["InboxProcessor"]
            BL["BlocklistCache"]
        end

        subgraph outbound [Outbound Federation]
            OP["OutboxPublisher"]
            NATSProd["NATS Producer"]
            FedStream["FEDERATION Stream"]
            Worker["Federation Worker"]
            SigSign["HTTP Sig Sign"]
        end

        subgraph core [Core]
            StoreIface["store.Store"]
            DB[(PostgreSQL)]
            Cache[(Cache)]
        end

        ServiceLayer["Service Layer"]
    end

    RemoteActor -->|"GET /.well-known/webfinger"| WF
    RemoteActor -->|"GET /users/{username}"| ActorH
    RemoteActor -->|"POST /inbox"| InboxH

    InboxH --> SigVerify --> IP
    IP --> BL
    IP --> StoreIface

    ServiceLayer --> OP
    OP --> NATSProd --> FedStream --> Worker
    Worker --> SigSign -->|"POST to remote inbox"| RemoteActor

    StoreIface --> DB
    BL --> Cache
    SigVerify --> Cache
```

---

## `internal/ap/blocklist.go` — Domain Block Cache

The blocklist is checked on every inbound activity and before every outbound delivery. It must be fast (no DB round-trip per request) and consistent across replicas.

---

## `internal/ap/outbox.go` — Outbox Publisher

The OutboxPublisher is called by the service layer when a local user performs an action that must be federated. It builds the AP activity JSON, determines the delivery targets, and enqueues NATS messages for the federation worker.

---

## `internal/api/activitypub/` — AP HTTP Handlers

All handlers live under `internal/api/activitypub/`. Each file exports a handler struct constructed via dependency injection. Handlers never reference the NATS client or federation worker directly — they work through the `InboxProcessor` and `OutboxPublisher` interface.

### Handler Summary Table

| File | Endpoint | Content-Type | Cache | Auth |
|------|----------|-------------|-------|------|
| `webfinger.go` | `GET /.well-known/webfinger` | `application/jrd+json` | `max-age=3600` | None |
| `nodeinfo.go` | `GET /.well-known/nodeinfo` | `application/json` | `max-age=1800` | None |
| `nodeinfo.go` | `GET /nodeinfo/2.0` | `application/json` | `max-age=1800` | None |
| `actor.go` | `GET /users/{username}` | `application/activity+json` | `max-age=180` | None |
| `outbox.go` | `GET /users/{username}/outbox` | `application/activity+json` | None | None |
| `collections.go` | `GET /users/{username}/followers` | `application/activity+json` | `max-age=180` | None |
| `collections.go` | `GET /users/{username}/following` | `application/activity+json` | `max-age=180` | None |
| `collections.go` | `GET /users/{username}/collections/featured` | `application/activity+json` | `max-age=180` | None |
| `inbox.go` | `POST /users/{username}/inbox` | N/A (accepts `activity+json`) | None | HTTP Signature |
| `inbox.go` | `POST /inbox` | N/A (accepts `activity+json`) | None | HTTP Signature |

---
