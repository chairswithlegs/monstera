# Monstera-fed — Project Specification

> **Status:** Draft v0.1 — Feb 24, 2026  
> **Project name placeholder:** *Monstera-fed* (rename as desired)

---

## Table of Contents

1. [Overview](#1-overview)
2. [Design Principles](#2-design-principles)
3. [Technology Stack](#3-technology-stack)
4. [System Architecture](#4-system-architecture)
5. [Project Structure](#5-project-structure)
6. [Data Model](#6-data-model)
7. [API Specification](#7-api-specification)
8. [Authentication & OAuth](#8-authentication--oauth)
9. [ActivityPub & Federation](#9-activitypub--federation)
10. [Media Storage Abstraction](#10-media-storage-abstraction)
11. [Cache Abstraction](#11-cache-abstraction)
12. [Email Abstraction](#12-email-abstraction)
13. [Real-Time Streaming (SSE)](#13-real-time-streaming-sse)
14. [NATS Integration](#14-nats-integration)
15. [Admin Portal](#15-admin-portal)
16. [Content Moderation](#16-content-moderation)
17. [Registration & Invites](#17-registration--invites)
18. [Observability](#18-observability)
19. [Configuration](#19-configuration)
20. [Deployment](#20-deployment)
21. [Development Roadmap](#21-development-roadmap)

---

## 1. Overview

Monstera-fed is a self-hosted **ActivityPub server** written in Go that exposes the **Mastodon-compatible REST API**. Because it speaks the Mastodon API, any Mastodon client (Ivory, Tusky, Elk, Mona, etc.) can connect to it without modification.

### Goals

- Implement the Mastodon client-facing API and ActivityPub server-to-server (S2S) protocol.
- Be horizontally scalable from the start — stateless application tier, external state in PostgreSQL + NATS.
- Expose clean abstraction boundaries so that storage, caching, email, and media backends can be swapped out via configuration.
- Provide an embedded admin web UI for instance management, moderation, and configuration.
- Support Mastodon-compatible clients through OAuth 2.0 (Authorization Code + PKCE).

### Non-Goals

- No client application (users bring their own Mastodon client).
- No multi-tenancy in v1 — one database, one instance, one community.
- No full ActivityPub compatibility with every AP server (Misskey, Pixelfed edge cases deferred).
- No rate limiting in-process — delegated to the Kubernetes ingress / API gateway.

---

## 2. Design Principles

| Principle | Implication |
|-----------|-------------|
| **Stateless application tier** | No in-process state that cannot be lost on restart. Sessions, tokens, and caches live in external stores. |
| **Abstraction over implementation** | Media, cache, email, and messaging are interfaces. Concrete implementations are registered at startup based on environment configuration. |
| **12-factor configuration** | All runtime configuration comes from environment variables. No config files in production. |
| **Horizontal scalability** | Multiple replicas behind a load balancer must work correctly. NATS carries fan-out work; no replica talks to another. |
| **Observability first** | Structured JSON logs, Prometheus metrics, and health endpoints are non-negotiable from day one. |
| **Phased feature delivery** | Phase 1 ships a working, usable server. Phase 2 adds richness. See §21. |
| **Minimal external dependencies** | A fully functional instance requires only **PostgreSQL and NATS** — both are easy to run in Kubernetes. Redis and S3 are optional enhancements; each has a built-in fallback (in-memory cache, local filesystem storage). Self-hosters should never need a managed cloud service to get started. |

---

## 3. Technology Stack

### Core

| Component | Technology | Rationale |
|-----------|-----------|-----------|
| Language | **Go 1.26+** | Static binaries, excellent concurrency model, strong HTTP library |
| HTTP Router | **chi** (`go-chi/chi`) | Lightweight, idiomatic, middleware-friendly |
| Database | **PostgreSQL 16+** | Relational social graph, JSONB for AP payloads, built-in full-text search (Phase 2), mature ecosystem |
| DB Migrations | **golang-migrate** | SQL-first migrations, version-controlled schema |
| DB Access | **pgx/v5** (+ **sqlc** for query generation) | Type-safe queries, no ORM magic |
| Message Broker | **NATS JetStream** | Durable federation delivery queues + pub/sub for SSE fan-out |
| Cache | Pluggable (see §11) | In-memory (single-node / dev), Redis/Valkey (multi-replica prod) |
| Media Storage | Pluggable (see §10) | Local filesystem (single-node / dev), S3-compatible (prod) |
| Email | Pluggable (see §12) | SMTP relay (covers self-hosted and managed services like SendGrid, Postmark, SES) |

### Why NATS JetStream?

Two concrete use cases make NATS worth the operational cost:

1. **Federation fan-out** — When a local user posts, the activity must be delivered to the inboxes of every remote follower's server. This is an unbounded, at-least-once delivery problem — exactly what JetStream durable consumers solve. Workers pull jobs, retry on failure, and acknowledge when done.
2. **SSE fan-out** — When an event (new post, notification, follow) is published, every replica that has an active SSE client for that user needs to be notified. NATS core pub/sub (no JetStream required) handles this broadcast efficiently across replicas without shared in-process state.

---

## 4. System Architecture

```
                          ┌──────────────────────────────────────┐
                          │           Kubernetes Cluster          │
                          │                                       │
  Mastodon Clients ──────▶│  Ingress (NGINX / Envoy)             │
  (Ivory, Tusky, etc.)    │    │  TLS termination                │
                          │    │  Rate limiting                   │
  Remote AP Servers ──────│────┤                                  │
  (federation inbox)      │    │                                  │
                          │    ▼                                  │
                          │  ┌─────────────────────────────────┐  │
                          │  │  Monstera-fed API Pods (N replicas) │  │
                          │  │                                 │  │
                          │  │  ┌──────┐  ┌────────────────┐  │  │
                          │  │  │ HTTP │  │ Federation     │  │  │
                          │  │  │ API  │  │ Worker (NATS   │  │  │
                          │  │  │ (chi)│  │ JetStream sub) │  │  │
                          │  │  └──────┘  └────────────────┘  │  │
                          │  │  ┌──────────────────────────┐   │  │
                          │  │  │ SSE Hub (NATS core sub)  │   │  │
                          │  │  └──────────────────────────┘   │  │
                          │  └─────────────────────────────────┘  │
                          │         │         │         │          │
                          │         ▼         ▼         ▼          │
                          │   PostgreSQL    NATS     Object        │
                          │   (primary +  JetStream  Storage       │
                          │    replicas)   cluster   (S3/local)    │
                          │                                        │
                          │              Cache Layer               │
                          │         (Redis/Valkey or local)        │
                          └────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility |
|-----------|----------------|
| **API Pod** | Handles all HTTP requests: Mastodon API, ActivityPub inbox/outbox, OAuth, admin portal, SSE endpoints |
| **Federation Worker** | Goroutine pool (or separate binary) consuming NATS JetStream jobs — delivers `POST` to remote inboxes with retries |
| **SSE Hub** | Per-replica goroutine that subscribes to NATS core pub/sub; pushes events to connected SSE clients |
| **PostgreSQL primary** | All writes |
| **PostgreSQL read replicas** | Timeline reads, search (optional) |
| **NATS JetStream** | Durable federation delivery queue, dead-letter stream |
| **NATS core** | Ephemeral pub/sub for SSE fan-out across replicas |
| **Object Storage** | Media attachment storage, served via CDN or presigned URLs |
| **Cache** | Timeline caching, idempotency keys, HTTP signature replay prevention |

---

## 5. Project Structure

```
monstera-fed/
├── cmd/
│   └── monstera-fed/
│       └── main.go               # Entry point; wires everything together
│
├── internal/
│   ├── api/                      # HTTP handlers, middleware, routing
│   │   ├── router.go
│   │   ├── middleware/
│   │   ├── mastodon/             # Mastodon-compatible REST handlers
│   │   │   ├── accounts.go
│   │   │   ├── statuses.go
│   │   │   ├── timelines.go
│   │   │   ├── notifications.go
│   │   │   ├── media.go
│   │   │   ├── search.go
│   │   │   └── streaming.go      # SSE /api/v1/streaming
│   │   ├── oauth/                # OAuth 2.0 + PKCE endpoints
│   │   ├── activitypub/          # AP inbox, outbox, webfinger, nodeinfo
│   │   └── admin/                # Admin portal handlers + embedded UI
│   │
│   ├── domain/                   # Pure domain types (no DB, no HTTP)
│   │   ├── errors.go             # Sentinel errors (ErrNotFound, ErrConflict, etc.)
│   │   ├── account.go
│   │   ├── status.go
│   │   ├── follow.go
│   │   ├── notification.go
│   │   ├── media.go
│   │   └── ...
│   │
│   ├── service/                  # Business logic layer
│   │   ├── account_service.go
│   │   ├── status_service.go
│   │   ├── timeline_service.go
│   │   ├── federation_service.go
│   │   ├── moderation_service.go
│   │   └── ...
│   │
│   ├── store/                    # Database access (sqlc-generated + custom)
│   │   ├── postgres/
│   │   │   ├── db.go             # pgx pool setup
│   │   │   ├── queries/          # .sql files for sqlc
│   │   │   └── generated/        # sqlc output
│   │   └── migrations/           # golang-migrate SQL files
│   │
│   ├── cache/                    # Cache abstraction
│   │   ├── cache.go              # Store interface
│   │   ├── memory/               # in-process implementation
│   │   └── redis/                # Redis/Valkey implementation
│   │
│   ├── media/                    # Media storage abstraction
│   │   ├── store.go              # MediaStore interface
│   │   ├── local/                # Local filesystem implementation
│   │   └── s3/                   # S3-compatible implementation
│   │
│   ├── email/                    # Email abstraction
│   │   ├── sender.go             # Sender interface
│   │   ├── templates.go          # Embedded email templates
│   │   ├── templates/            # HTML + text template files
│   │   ├── smtp/                 # SMTP implementation (covers all providers)
│   │   └── noop/                 # No-op (for dev/testing)
│   │
│   ├── nats/                     # NATS client, stream definitions
│   │   ├── client.go
│   │   ├── federation/           # Federation delivery producer/consumer
│   │   └── streaming/            # SSE pub/sub bridge
│   │
│   ├── ap/                       # ActivityPub types and HTTP signatures
│   │   ├── vocab.go              # AS2 / AP vocabulary structs
│   │   ├── httpsig.go            # HTTP Signature sign/verify
│   │   ├── inbox.go              # Inbox processing logic
│   │   └── outbox.go
│   │
│   ├── oauth/                    # OAuth 2.0 server logic (PKCE, tokens)
│   │
│   ├── config/                   # Environment-variable configuration
│   │   └── config.go
│   │
│   └── observability/            # Logging, metrics, health
│       ├── logger.go             # slog JSON setup
│       └── metrics.go            # Prometheus registry
│
├── web/                          # Admin portal (embedded via go:embed)
│   └── admin/
│       ├── templates/            # Go html/template files (HTMX partials + layouts)
│       └── static/               # Pico.css, htmx.min.js, custom CSS
│
├── deployments/
│   ├── docker-compose.yml        # Local development
│   ├── k8s/
│   │   ├── deployment.yaml
│   │   ├── service.yaml
│   │   ├── configmap.yaml
│   │   ├── hpa.yaml              # Horizontal Pod Autoscaler
│   │   └── nats-values.yaml      # Helm values for NATS
│   └── Dockerfile
│
├── go.mod
├── go.sum
├── Makefile
└── spec.md
```

---

## 6. Data Model

> **Complete schema:** IMPLEMENTATION 02 defines the full set of base migrations (000001–000023), including tables not shown below (`favourites`, `bookmarks`, `mutes`, `account_blocks`, `status_edits`, `markers`, `conversations`, `lists`, `list_members`, `email_tokens`, `server_filters`, `status_tags`, `tags`, `status_media`). Subsequent IMPLEMENTATIONs add further migrations — see each IMPLEMENTATION's migration section for details.

### Core Tables

#### `accounts`
```sql
CREATE TABLE accounts (
    id              TEXT PRIMARY KEY,           -- nanoid or UUID
    username        TEXT NOT NULL,              -- local username
    domain          TEXT,                       -- NULL for local accounts
    display_name    TEXT,
    note            TEXT,                       -- bio (HTML)
    avatar_media_id TEXT REFERENCES media_attachments(id),
    header_media_id TEXT REFERENCES media_attachments(id),
    public_key      TEXT NOT NULL,              -- RSA public key (PEM)
    private_key     TEXT,                       -- NULL for remote accounts
    inbox_url       TEXT NOT NULL,
    outbox_url      TEXT NOT NULL,
    followers_url   TEXT NOT NULL,
    following_url   TEXT NOT NULL,
    ap_id           TEXT NOT NULL UNIQUE,       -- canonical AP IRI
    ap_raw          JSONB,                      -- raw AP Actor JSON
    followers_count INT NOT NULL DEFAULT 0,     -- denormalized (IMPLEMENTATION 08)
    following_count INT NOT NULL DEFAULT 0,
    statuses_count  INT NOT NULL DEFAULT 0,
    fields          JSONB,                      -- profile metadata fields (IMPLEMENTATION 08)
    bot             BOOLEAN DEFAULT FALSE,
    locked          BOOLEAN DEFAULT FALSE,      -- requires follow approval
    suspended       BOOLEAN DEFAULT FALSE,
    silenced        BOOLEAN DEFAULT FALSE,
    suspension_origin TEXT,                       -- 'local' | 'remote' (why suspended)
    deletion_requested_at TIMESTAMPTZ,            -- soft-delete: triggers Delete{Person} federation, purged after 30 days
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (username, domain)
);
```

#### `users`
```sql
-- Separate from accounts: local users only (authentication concerns)
CREATE TABLE users (
    id              TEXT PRIMARY KEY,
    account_id      TEXT NOT NULL UNIQUE REFERENCES accounts(id),
    email           TEXT NOT NULL UNIQUE,
    password_hash   TEXT NOT NULL,              -- bcrypt
    confirmed_at    TIMESTAMPTZ,
    role            TEXT NOT NULL DEFAULT 'user', -- 'user' | 'moderator' | 'admin'
    default_privacy TEXT NOT NULL DEFAULT 'public',   -- (IMPLEMENTATION 08)
    default_sensitive BOOLEAN NOT NULL DEFAULT FALSE,
    default_language TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

#### `statuses`
```sql
CREATE TABLE statuses (
    id              TEXT PRIMARY KEY,
    uri             TEXT NOT NULL UNIQUE,       -- HTML permalink URL (may differ from ap_id for remote statuses)
    account_id      TEXT NOT NULL REFERENCES accounts(id),
    text            TEXT,                       -- original Markdown/plain text
    content         TEXT,                       -- rendered HTML
    content_warning TEXT,
    visibility      TEXT NOT NULL,              -- 'public' | 'unlisted' | 'private' | 'direct'
    language        TEXT,
    in_reply_to_id  TEXT REFERENCES statuses(id),
    in_reply_to_account_id TEXT REFERENCES accounts(id), -- (IMPLEMENTATION 08)
    reblog_of_id    TEXT REFERENCES statuses(id),
    ap_id           TEXT NOT NULL UNIQUE,
    ap_raw          JSONB,
    sensitive       BOOLEAN DEFAULT FALSE,
    local           BOOLEAN NOT NULL DEFAULT TRUE,
    replies_count   INT NOT NULL DEFAULT 0,
    reblogs_count   INT NOT NULL DEFAULT 0,
    favourites_count INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ                -- soft delete
);
CREATE INDEX idx_statuses_account ON statuses(account_id, created_at DESC);
CREATE INDEX idx_statuses_local_public ON statuses(created_at DESC) 
    WHERE local = TRUE AND visibility = 'public' AND deleted_at IS NULL;
```

#### `follows`
```sql
CREATE TABLE follows (
    id              TEXT PRIMARY KEY,
    account_id      TEXT NOT NULL REFERENCES accounts(id),  -- follower
    target_id       TEXT NOT NULL REFERENCES accounts(id),  -- followee
    state           TEXT NOT NULL DEFAULT 'pending',        -- 'pending' | 'accepted'
    ap_id           TEXT UNIQUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (account_id, target_id)
);
```

#### `notifications`
```sql
CREATE TABLE notifications (
    id          TEXT PRIMARY KEY,
    account_id  TEXT NOT NULL REFERENCES accounts(id),  -- recipient
    from_id     TEXT NOT NULL REFERENCES accounts(id),  -- actor
    type        TEXT NOT NULL,  -- 'follow' | 'mention' | 'reblog' | 'favourite' | 'follow_request'
    status_id   TEXT REFERENCES statuses(id),
    read        BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_notifications_account ON notifications(account_id, created_at DESC);
```

#### `media_attachments`
```sql
CREATE TABLE media_attachments (
    id              TEXT PRIMARY KEY,
    account_id      TEXT NOT NULL REFERENCES accounts(id),
    type            TEXT NOT NULL,              -- 'image' | 'video' | 'audio' | 'gifv'
    storage_key     TEXT NOT NULL,              -- opaque key handed to MediaStore
    url             TEXT NOT NULL,              -- public URL
    preview_url     TEXT,
    remote_url      TEXT,                       -- original URL for remote media
    description     TEXT,                       -- alt text
    blurhash        TEXT,
    meta            JSONB,                      -- width, height, duration, etc.
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

#### `oauth_applications`
```sql
CREATE TABLE oauth_applications (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    client_id       TEXT NOT NULL UNIQUE,
    client_secret   TEXT NOT NULL,
    redirect_uris   TEXT NOT NULL,
    scopes          TEXT NOT NULL DEFAULT 'read',
    website         TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

#### `oauth_access_tokens`
```sql
CREATE TABLE oauth_access_tokens (
    id              TEXT PRIMARY KEY,
    application_id  TEXT NOT NULL REFERENCES oauth_applications(id),
    account_id      TEXT REFERENCES accounts(id),
    token           TEXT NOT NULL UNIQUE,
    scopes          TEXT NOT NULL,
    expires_at      TIMESTAMPTZ,
    revoked_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

#### `domain_blocks`
```sql
CREATE TABLE domain_blocks (
    id          TEXT PRIMARY KEY,
    domain      TEXT NOT NULL UNIQUE,
    severity    TEXT NOT NULL DEFAULT 'suspend', -- 'silence' | 'suspend'
    reason      TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

#### `reports`
```sql
CREATE TABLE reports (
    id              TEXT PRIMARY KEY,
    account_id      TEXT NOT NULL REFERENCES accounts(id),  -- reporter
    target_id       TEXT NOT NULL REFERENCES accounts(id),  -- reported
    status_ids      TEXT[],                                 -- optional reported posts
    comment         TEXT,
    category        TEXT NOT NULL DEFAULT 'other',
    state           TEXT NOT NULL DEFAULT 'open',           -- 'open' | 'resolved'
    assigned_to_id  TEXT REFERENCES users(id),
    action_taken    TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at     TIMESTAMPTZ
);
```

#### `invites`
```sql
CREATE TABLE invites (
    id          TEXT PRIMARY KEY,
    code        TEXT NOT NULL UNIQUE,
    created_by  TEXT NOT NULL REFERENCES users(id),
    max_uses    INT,                            -- NULL = unlimited
    uses        INT NOT NULL DEFAULT 0,
    expires_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

#### `instance_settings`
```sql
CREATE TABLE instance_settings (
    key     TEXT PRIMARY KEY,
    value   TEXT NOT NULL
);
-- Keys: instance_name, instance_description, registration_mode ('approval'|'invite'),
--       contact_email, max_status_chars, media_max_bytes, etc.
```

---

## 7. API Specification

### 7.1 Mastodon API — Phase 1 (Launch)

All endpoints are prefixed `/api/v1/` unless noted.

#### Accounts
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/accounts/verify_credentials` | Current user |
| PATCH | `/api/v1/accounts/update_credentials` | Update profile |
| GET | `/api/v1/accounts/:id` | Account lookup |
| GET | `/api/v1/accounts/:id/statuses` | Account timeline |
| GET | `/api/v1/accounts/:id/followers` | Followers list |
| GET | `/api/v1/accounts/:id/following` | Following list |
| POST | `/api/v1/accounts/:id/follow` | Follow |
| POST | `/api/v1/accounts/:id/unfollow` | Unfollow |
| POST | `/api/v1/accounts/:id/block` | Block |
| POST | `/api/v1/accounts/:id/unblock` | Unblock |
| POST | `/api/v1/accounts/:id/mute` | Mute |
| POST | `/api/v1/accounts/:id/unmute` | Unmute |
| GET | `/api/v1/accounts/relationships` | Bulk relationship check |

#### Statuses
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/statuses` | Create status |
| GET | `/api/v1/statuses/:id` | Get status |
| DELETE | `/api/v1/statuses/:id` | Delete status |
| POST | `/api/v1/statuses/:id/reblog` | Boost |
| POST | `/api/v1/statuses/:id/unreblog` | Unboost |
| POST | `/api/v1/statuses/:id/favourite` | Favourite |
| POST | `/api/v1/statuses/:id/unfavourite` | Unfavourite |
| GET | `/api/v1/statuses/:id/context` | Thread context |
| GET | `/api/v1/statuses/:id/favourited_by` | Who favourited |
| GET | `/api/v1/statuses/:id/reblogged_by` | Who boosted |

#### Timelines
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/timelines/home` | Home timeline (follows) |
| GET | `/api/v1/timelines/public` | Local/federated public timeline |
| GET | `/api/v1/timelines/tag/:hashtag` | Hashtag timeline |

#### Notifications
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/notifications` | List notifications |
| POST | `/api/v1/notifications/clear` | Clear all |
| POST | `/api/v1/notifications/:id/dismiss` | Dismiss one |

#### Media
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v2/media` | Upload attachment |
| GET | `/api/v1/media/:id` | Get attachment |
| PUT | `/api/v1/media/:id` | Update description |

#### Search & Discovery
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v2/search` | Search accounts and hashtags |

#### Streaming
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/streaming/health` | Streaming health check |
| GET | `/api/v1/streaming/user` | Home timeline + notifications SSE |
| GET | `/api/v1/streaming/public` | Local public timeline SSE |
| GET | `/api/v1/streaming/public/local` | Local-only public SSE |
| GET | `/api/v1/streaming/hashtag` | Hashtag SSE |

#### Instance
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v2/instance` | Instance metadata |
| GET | `/api/v1/custom_emojis` | Custom emoji list |

#### OAuth & Auth
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/apps` | Register application |
| GET | `/oauth/authorize` | Authorization screen |
| POST | `/oauth/token` | Issue token |
| POST | `/oauth/revoke` | Revoke token |

### 7.2 ActivityPub Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/.well-known/webfinger` | WebFinger lookup |
| GET | `/.well-known/nodeinfo` | NodeInfo pointer |
| GET | `/nodeinfo/2.0` | NodeInfo document |
| GET | `/users/:username` | AP Actor |
| GET | `/users/:username/outbox` | AP Outbox (ordered collection) |
| GET | `/users/:username/followers` | AP Followers collection |
| GET | `/users/:username/following` | AP Following collection |
| POST | `/users/:username/inbox` | AP Inbox (receive activities) |
| POST | `/inbox` | Shared inbox |

### 7.3 Phase 2 (Post-Launch)

- Full-text post search (tsvector)
- Lists (`/api/v1/lists`)
- Filters (`/api/v1/filters`)
- Polls (`/api/v1/polls`)
- Bookmarks (`/api/v1/bookmarks`)
- Favourites collection (`/api/v1/favourites`)
- Followed hashtags (`/api/v1/followed_tags`)
- Announcements (`/api/v1/announcements`)
- Mastodon Admin API (`/api/v1/admin/...`)
- Push notification subscriptions (`/api/v1/push/subscription`)

---

## 8. Authentication & OAuth

### Supported Flows

#### Authorization Code with PKCE
The primary flow for mobile and desktop Mastodon clients:

```
Client                     Monstera-fed
  │                            │
  │── GET /oauth/authorize ────▶│  (code_challenge, code_challenge_method=S256)
  │                            │  Display login/consent screen
  │◀─ redirect with ?code ─────│
  │                            │
  │── POST /oauth/token ───────▶│  (code, code_verifier)
  │◀─ { access_token } ────────│
```

#### Authorization Code (without PKCE)
Supported for compatibility with server-side web clients and legacy applications.

### Scope System

| Scope | Description |
|-------|-------------|
| `read` | Full read access |
| `read:accounts` | Read account info |
| `read:statuses` | Read posts |
| `read:notifications` | Read notifications |
| `write` | Full write access |
| `write:statuses` | Publish posts |
| `write:media` | Upload media |
| `write:follows` | Follow/unfollow |
| `follow` | Alias for follow-related scopes |
| `push` | Push subscription management |
| `admin:read` | Admin read access |
| `admin:write` | Admin write access |

### Token Storage

Access tokens are stored in the `oauth_access_tokens` table with an index on the `token` column. A short-TTL cache entry (cache key: `token:{hash}`) is maintained to avoid hitting the database on every authenticated request.

### HTTP Signature Verification

All incoming ActivityPub `POST` requests to `/inbox` and `/users/:username/inbox` must carry a valid **HTTP Signature** (`Signature` header, RFC 9421 / Mastodon's draft-cavage-http-signatures-12). Signatures are verified against the sender's public key fetched from their AP Actor document (with a short cache TTL). Replay attacks are prevented by storing the `(keyId, Date, requestTarget)` tuple in the cache with a TTL matching the allowed clock skew window (±30 seconds).

---

## 9. ActivityPub & Federation

### Overview

Monstera-fed implements the ActivityPub Server-to-Server (S2S) protocol. The scope is **Mastodon-compatible federation** — full compatibility with Mastodon instances, and best-effort compatibility with other AP implementations (Pleroma, Calckey, Pixelfed).

### Supported Activity Types (Inbox)

| Activity | Action |
|----------|--------|
| `Follow` | Create follow relationship (pending if target.locked) |
| `Accept{Follow}` | Confirm pending follow |
| `Reject{Follow}` | Reject pending follow |
| `Undo{Follow}` | Remove follow |
| `Create{Note}` | Ingest remote status |
| `Announce{Note}` | Ingest remote boost |
| `Like{Note}` | Record favourite |
| `Undo{Like}` | Remove favourite |
| `Undo{Announce}` | Remove boost |
| `Delete{Note/Tombstone}` | Remove status |
| `Update{Note}` | Update status content |
| `Update{Person}` | Sync remote account profile |
| `Block` | Note remote block (defensive) |

### Outbox (Published Activities)

When a local user performs an action, the corresponding AP activity is:

1. Saved to the database.
2. Published as a NATS JetStream message to the `federation.deliver` subject.
3. Picked up by federation workers which `POST` the activity to each target inbox.

### Blocklist / Defederation

Domain blocks are stored in the `domain_blocks` table and loaded into the cache on startup with a TTL. Any activity from a blocked domain is silently dropped at the inbox handler. Outbound delivery to blocked domains is also suppressed.

Block severity levels:
- **`silence`** — Content from this domain is hidden from public timelines and not boosted. Follows still work.
- **`suspend`** — All activities from this domain are rejected. Existing follows are severed.

### Actor Key Management

Each local account has an RSA-2048 key pair generated at account creation time. The public key is embedded in the AP Actor document at `publicKey`. The private key is stored (encrypted at rest) in the `accounts.private_key` column and used to sign outgoing HTTP requests.

---

## 10. Media Storage Abstraction

### Interface

```go
// internal/media/store.go

type MediaStore interface {
    // Put stores the content and returns an opaque storage key.
    Put(ctx context.Context, key string, r io.Reader, contentType string) error

    // Get returns a reader for the stored content.
    Get(ctx context.Context, key string) (io.ReadCloser, error)

    // Delete removes the stored content.
    Delete(ctx context.Context, key string) error

    // URL returns the public URL for the given storage key.
    // For S3, this may be a presigned URL or a CDN URL.
    URL(ctx context.Context, key string) (string, error)
}
```

### Implementations

#### `local` (dev / small deployments)
- Stores files under a configurable `MEDIA_LOCAL_PATH` directory.
- `URL()` returns a path relative to the server's `MEDIA_BASE_URL`.
- The Go server serves these files at `/system/...`.

#### `s3` (production)
- Uses the AWS SDK v2 (compatible with MinIO, Cloudflare R2, Backblaze B2, etc.).
- Configured via `MEDIA_S3_BUCKET`, `MEDIA_S3_REGION`, `MEDIA_S3_ENDPOINT` (for non-AWS).
- `URL()` returns a CDN URL (`MEDIA_CDN_BASE`) or a presigned URL.

### Configuration

```
MEDIA_DRIVER=local|s3
MEDIA_LOCAL_PATH=/var/monstera-fed/media
MEDIA_BASE_URL=https://example.com

MEDIA_S3_BUCKET=monstera-fed-media
MEDIA_S3_REGION=us-east-1
MEDIA_S3_ENDPOINT=                   # optional: override for MinIO/R2
MEDIA_CDN_BASE=https://cdn.example.com
```

---

## 11. Cache Abstraction

### Interface

```go
// internal/cache/cache.go

type Store interface {
    Get(ctx context.Context, key string) ([]byte, error)
    Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
}
```

### Implementations

#### `memory` (dev / single-node testing)
- Backed by [ristretto](https://github.com/dgraph-io/ristretto) for cost-based eviction.
- Does **not** share state across replicas — use only for development.

#### `redis` (production)
- Connects to any Redis-compatible server (Redis 7+, Valkey, KeyDB).
- Configured via `CACHE_REDIS_URL`.
- Used for: OAuth token cache, AP actor cache, timeline cache, HTTP Signature replay set.

### Configuration

```
CACHE_DRIVER=memory|redis
CACHE_REDIS_URL=redis://localhost:6379/0
```

---

## 12. Email Abstraction

### Interface

```go
// internal/email/sender.go

type Message struct {
    To      string
    Subject string
    HTML    string
    Text    string
}

type Sender interface {
    Send(ctx context.Context, msg Message) error
}
```

### Implementations

| Driver | Use case |
|--------|----------|
| `noop` | Development — logs emails to stdout, never delivers |
| `smtp` | Universal — works with self-hosted relays (Postfix), Gmail/Fastmail, and managed transactional services (SendGrid, Postmark, SES, Mailgun) via their SMTP endpoints |

All major managed email services expose SMTP endpoints, so a single SMTP driver covers every provider without vendor-specific code:
- **SendGrid:** `smtp.sendgrid.net:587` (username: `apikey`, password: API key)
- **Postmark:** `smtp.postmarkapp.com:587` (username/password: server token)
- **SES:** `email-smtp.{region}.amazonaws.com:587`
- **Mailgun:** `smtp.mailgun.org:587`

The `Sender` interface allows additional vendor-specific HTTP API drivers to be added later without breaking changes if needed.

### Configuration

```
EMAIL_DRIVER=noop|smtp
EMAIL_FROM=noreply@example.com
EMAIL_FROM_NAME=Monstera-fed

EMAIL_SMTP_HOST=smtp.example.com
EMAIL_SMTP_PORT=587
EMAIL_SMTP_USERNAME=
EMAIL_SMTP_PASSWORD=
```

### Email Templates

Templates are embedded in the binary via `go:embed`. Types:

- **Email verification** — sent on registration
- **Password reset** — link with short-TTL token
- **Invite** — contains invite code and instance URL
- **Moderation action** — account warned, silenced, or suspended

---

## 13. Real-Time Streaming (SSE)

### Protocol

Monstera-fed implements the Mastodon Streaming API using **Server-Sent Events (SSE)**. Clients connect to a long-lived HTTP connection and receive newline-delimited event frames:

```
event: update
data: {"id":"...", "content":"...", ...}

event: notification
data: {"id":"...", "type":"mention", ...}

event: delete
data: 12345
```

### Architecture

Each API replica runs an **SSE Hub** — a goroutine that:

1. Subscribes to NATS core pub/sub subjects relevant to the connected clients.
2. Maintains a map of `(streamKey → []chan Event)` for all active SSE connections on that replica.
3. Fans events from NATS into the per-connection channels.

When an event is generated (e.g., a new status is published), the service layer publishes to the appropriate NATS subjects:

| NATS Subject | SSE Stream |
|---|---|
| `events.public.local` | `/api/v1/streaming/public/local` |
| `events.public` | `/api/v1/streaming/public` |
| `events.user.{accountID}` | `/api/v1/streaming/user` |
| `events.hashtag.{tag}` | `/api/v1/streaming/hashtag` |

This design is **replica-agnostic** — a client may connect to any pod and still receive events generated by a different pod.

### Connection Lifecycle

1. Client sends `GET /api/v1/streaming/user` with `Authorization: Bearer <token>`.
2. Handler authenticates token, subscribes the connection to `events.user.{accountID}`.
3. Server writes `Content-Type: text/event-stream` and begins streaming.
4. On client disconnect, the subscription is cleaned up and the NATS subscription is cancelled (or reference-counted if shared).

---

## 14. NATS Integration

### Stream Definitions (JetStream)

#### `FEDERATION` stream
- **Subject:** `federation.deliver.>`
- **Retention:** Work queue (message deleted after acknowledgement)
- **Storage:** File (durable across NATS restarts)
- **Message schema:**
  ```json
  {
    "activity_id": "...",
    "activity": { /* AP activity JSON */ },
    "target_inbox": "https://remote.example.com/inbox",
    "attempt": 1
  }
  ```
- **Consumer:** `federation-worker` — pull consumer, durable
- **Retry policy:** Exponential backoff with max 5 attempts; failed messages move to `FEDERATION_DLQ`.

#### `FEDERATION_DLQ` stream
- Dead-letter queue for undeliverable activities.
- Admin can inspect and optionally re-queue.

### Core Pub/Sub (Streaming)

No JetStream required — ephemeral, at-most-once delivery is acceptable for SSE fan-out. If a replica misses a message because the client wasn't connected yet, that event is simply not delivered (clients are expected to backfill via REST on reconnect).

---

## 15. Admin Portal

### Overview

The admin portal is a server-rendered web application using Go `html/template`, HTMX, and Pico.css, served directly by the Go binary via `go:embed`. It is accessible at `/admin` and protected by an opaque session cookie (random token, cached with sliding TTL) issued after authenticating as a user with `role = 'admin'` or `role = 'moderator'`.

### Features

#### Dashboard
- Active user count, local post count, federated server count
- Storage utilization
- Pending reports count
- Pending follow requests / registrations count

#### Users
- Browse all local users (paginated, searchable)
- View user details (posts, followers, reports)
- Suspend / silence / unsuspend accounts
- Assign moderator role
- Force-delete account and content
- Send account a warning message (email)

#### Registrations
- Switch between **Approval** and **Invite-only** modes
- Review pending approval requests (approve / reject)
- Create and manage invite codes (bulk-create, set expiry, max uses)

#### Reports
- List open reports with filters (unresolved, by category)
- View report detail (reporter, target, reported posts)
- Take action (warn, silence, suspend, dismiss as unfounded)
- Assign reports to moderators
- View action history

#### Federation
- Browse federated instances
- Add domain blocks (with severity and reason)
- Remove domain blocks
- View known servers with last-seen dates

#### Content
- Manage custom emoji (upload, rename, disable)
- Manage content filters

#### Instance Settings
- Instance name, description, contact email
- Registration mode (approval / invite)
- Max status character count
- Media size limits
- Rules / ToS text

---

## 16. Content Moderation

### User Reports

- Any authenticated user can report a post or an account.
- Reports optionally include specific status IDs and a category (`spam`, `illegal`, `violation`, `other`).
- Reports enter `state = 'open'` and appear in the admin queue.
- For reports against remote accounts, a copy of the report activity is forwarded to the remote instance's moderators (`Flag` activity in AP).

### Account Actions

| Action | Effect |
|--------|--------|
| **Silence** | Account's posts are hidden from public/federated timelines. Followers still see posts. |
| **Suspend** | Account cannot log in. Posts return 404. AP activities are rejected. Existing follows severed after grace period. |
| **Unsuspend** | Restores access. Does not restore removed follows. |

### Domain Blocks

- Stored in `domain_blocks` table.
- Loaded into the cache on startup (TTL: 1 hour, background refresh).
- Applied at both the inbox receiver (incoming) and the federation worker (outgoing).

### Content Filters (Server-Side)

- Admins can define keyword/regex filters with scope (`public_timeline`, `all`).
- Statuses matching a filter on ingest are flagged or hidden from affected timelines.
- Stored in `server_filters` table; cached in-process.

---

## 17. Registration & Invites

### Registration Modes

The `registration_mode` setting (stored in `instance_settings`) controls how new accounts are created:

#### `approval` mode
1. User completes the registration form (username, email, password, optional reason).
2. Account and user records are created with `confirmed = FALSE`.
3. Admin sees the pending registration in the admin queue.
4. Admin approves → confirmation email sent → user can log in.
5. Admin rejects → rejection email sent → records optionally deleted.

#### `invite` mode
1. An existing user or admin generates an invite code (via admin portal or future `/api/v1/invites`).
2. Invite codes have optional expiry and max-use count.
3. Registration form requires a valid invite code.
4. On successful registration with a valid code, the `invites.uses` counter is incremented and the user is confirmed immediately (no admin approval required).
5. Expired or exhausted codes are rejected.

The mode can be toggled by an admin in the instance settings without restarting the server.

---

## 18. Observability

### Structured Logging

- Library: Go's standard `log/slog` with JSON handler.
- Log level configurable via `LOG_LEVEL=debug|info|warn|error`.
- Every HTTP request logs: `method`, `path`, `status`, `duration_ms`, `request_id`, `account_id` (if authenticated).
- Every federation delivery attempt logs: `activity_id`, `target_inbox`, `attempt`, `status_code`, `duration_ms`.

### Prometheus Metrics

Exposed at `GET /metrics` (unauthenticated, or optionally protected via `METRICS_TOKEN`).

Key metrics:

| Metric | Type | Labels |
|--------|------|--------|
| `monstera-fed_http_requests_total` | Counter | method, path, status |
| `monstera-fed_http_request_duration_seconds` | Histogram | method, path |
| `monstera-fed_federation_deliveries_total` | Counter | result (success/failure/rejected) |
| `monstera-fed_federation_delivery_duration_seconds` | Histogram | |
| `monstera-fed_active_sse_connections` | Gauge | stream |
| `monstera-fed_nats_publish_total` | Counter | subject, result |
| `monstera-fed_db_query_duration_seconds` | Histogram | query_name |
| `monstera-fed_media_upload_bytes_total` | Counter | driver |
| `monstera-fed_accounts_total` | Gauge | type (local/remote) |

### Health Endpoints

| Endpoint | Check |
|----------|-------|
| `GET /healthz/live` | Process is alive (always 200) |
| `GET /healthz/ready` | PostgreSQL ping + NATS ping (200 if all pass) |

Used as Kubernetes `livenessProbe` and `readinessProbe` respectively.

---

## 19. Configuration

All configuration is via environment variables (12-factor). No config file in production.

```bash
# --- Core ---
APP_ENV=development|production
APP_PORT=8080
INSTANCE_DOMAIN=social.example.com
INSTANCE_NAME="Example Social"
LOG_LEVEL=info

# --- Database ---
DATABASE_URL=postgres://user:pass@host:5432/monstera-fed?sslmode=require
DATABASE_MAX_OPEN_CONNS=20
DATABASE_MAX_IDLE_CONNS=5

# --- NATS ---
NATS_URL=nats://localhost:4222
NATS_CREDS_FILE=              # optional: NATS credentials file path

# --- Cache ---
# CACHE_DRIVER=memory  →  no external service required; state is not shared across replicas.
# CACHE_DRIVER=redis   →  recommended for multi-replica deployments.
CACHE_DRIVER=memory|redis
CACHE_REDIS_URL=redis://localhost:6379/0

# --- Media ---
# MEDIA_DRIVER=local   →  no external service required; files stored on a PersistentVolume.
# MEDIA_DRIVER=s3      →  recommended for multi-replica or high-storage deployments.
MEDIA_DRIVER=local|s3
MEDIA_LOCAL_PATH=/var/monstera-fed/media
MEDIA_BASE_URL=https://social.example.com
MEDIA_S3_BUCKET=
MEDIA_S3_REGION=
MEDIA_S3_ENDPOINT=
MEDIA_CDN_BASE=

# --- Email ---
EMAIL_DRIVER=noop|smtp
EMAIL_FROM=noreply@example.com
EMAIL_FROM_NAME=Monstera-fed
EMAIL_SMTP_HOST=
EMAIL_SMTP_PORT=587
EMAIL_SMTP_USERNAME=
EMAIL_SMTP_PASSWORD=

# --- Security ---
SECRET_KEY_BASE=<64-byte random hex>   # used for signing invite tokens, etc.
METRICS_TOKEN=                          # optional: bearer token for /metrics

# --- Feature flags ---
FEDERATION_ENABLED=true
MAX_STATUS_CHARS=500
MEDIA_MAX_BYTES=10485760               # 10 MB
```

---

## 20. Deployment

### Dockerfile

```dockerfile
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o monstera-fed ./cmd/monstera-fed

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /app/monstera-fed /monstera-fed
ENTRYPOINT ["/monstera-fed"]
```

### Docker Compose (Local Development)

`deployments/docker-compose.yml` provides:
- `monstera-fed` — the Go server
- `postgres` — PostgreSQL 16 *(required)*
- `nats` — NATS with JetStream enabled *(required)*
- `redis` *(optional)* — include when testing `CACHE_DRIVER=redis`
- `minio` *(optional)* — include when testing `MEDIA_DRIVER=s3`

The default compose file runs with `CACHE_DRIVER=memory` and `MEDIA_DRIVER=local`, so Redis and MinIO are commented out. Developers can uncomment them to test the production-equivalent drivers locally.

```yaml
services:
  monstera-fed:
    build: .
    environment:
      APP_ENV: development
      DATABASE_URL: postgres://monstera-fed:monstera-fed@postgres:5432/monstera-fed?sslmode=disable
      NATS_URL: nats://nats:4222
      CACHE_DRIVER: memory
      MEDIA_DRIVER: local
      MEDIA_LOCAL_PATH: /media
    ports:
      - "8080:8080"
    depends_on:
      - postgres
      - nats

  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: monstera-fed
      POSTGRES_USER: monstera-fed
      POSTGRES_PASSWORD: monstera-fed
    volumes:
      - pgdata:/var/lib/postgresql/data

  nats:
    image: nats:2.10-alpine
    command: ["--jetstream", "--store_dir=/data"]
    volumes:
      - natsdata:/data

volumes:
  pgdata:
  natsdata:
```

### Kubernetes

`deployments/k8s/` contains:

- **`deployment.yaml`** — `Deployment` for the Monstera-fed API pods
  - `replicas: 3` (override with HPA)
  - `livenessProbe: /healthz/live`
  - `readinessProbe: /healthz/ready`
  - All config from a `ConfigMap` and `Secret`
- **`hpa.yaml`** — `HorizontalPodAutoscaler` targeting 70% CPU
- **`service.yaml`** — `ClusterIP` service
- **`ingress.yaml`** — Ingress with TLS (cert-manager annotation)
- **`nats-values.yaml`** — Helm values for the [NATS Helm chart](https://github.com/nats-io/k8s)
- **`pdb.yaml`** — `PodDisruptionBudget` (minAvailable: 2) for rolling updates

**External dependencies:**

| Service | Required | Deployment options |
|---------|----------|--------------------|
| PostgreSQL | **Yes** | Managed service (RDS, Cloud SQL, Supabase) recommended; or `postgres` Helm chart |
| NATS | **Yes** | [NATS Helm chart](https://github.com/nats-io/k8s) or NGS (managed NATS). Operationally simple — a single-node NATS pod with JetStream enabled is sufficient for most instances. |
| Redis/Valkey | No — use `CACHE_DRIVER=memory` for single-replica | Managed service (ElastiCache, Upstash) or `redis` Helm chart |
| Object Storage | No — use `MEDIA_DRIVER=local` with a PersistentVolume | AWS S3, Cloudflare R2, MinIO operator |

**Minimal K8s deployment** (PostgreSQL + NATS only, no managed cloud services required):
```
CACHE_DRIVER=memory       # in-process; acceptable for a single replica
MEDIA_DRIVER=local        # PersistentVolume mounted at MEDIA_LOCAL_PATH
EMAIL_DRIVER=smtp         # self-hosted relay, or noop during initial setup
```
Scale up by adding `CACHE_DRIVER=redis` and `MEDIA_DRIVER=s3` when multi-replica operation or large media volumes are needed.

### Database Migration Strategy

Migrations are run as a Kubernetes `Job` (or init container) before the `Deployment` rolls out:

```yaml
initContainers:
  - name: migrate
    image: monstera-fed:latest
    command: ["/monstera-fed", "migrate", "up"]
    envFrom:
      - secretRef:
          name: monstera-fed-secrets
```

`golang-migrate` manages versioned SQL migration files in `internal/store/migrations/`.

---

## 21. Development Roadmap

### Phase 1 — Foundations (MVP)

**Goal:** A working Mastodon-compatible instance that a real user can connect to with their preferred client.

| Feature | Notes |
|---------|-------|
| Account registration (approval + invite modes) | Core |
| Email verification | Core |
| OAuth 2.0 (Authorization Code + PKCE) | Core |
| Profile CRUD | Core |
| Status create / delete / boost / favourite | Core |
| Home timeline | Core |
| Local public timeline | Core |
| Notifications | Core |
| Follow / unfollow / block / mute | Core |
| Media upload (local + S3) | Core |
| ActivityPub inbox / outbox | Core |
| Mastodon-compatible federation | Core |
| Domain blocklist | Core |
| SSE streaming (user + public) | Core |
| Admin portal (users, reports, moderation, settings) | Core |
| Content moderation (reports, suspend, silence, domain blocks) | Core |
| WebFinger + NodeInfo | Core |
| Prometheus metrics + health endpoints | Core |
| Structured JSON logging | Core |
| Docker Compose dev environment | Core |
| Kubernetes manifests | Core |
| Database migrations | Core |

### Phase 2 — Richness

| Feature | Notes |
|---------|-------|
| Full-text post search (PostgreSQL tsvector) | Medium effort |
| Lists | Low effort |
| Filters (client-side + server-side) | Low effort |
| Polls | Medium effort |
| Bookmarks | Low effort |
| Custom emoji upload + management | Low effort |
| Mastodon Admin API (`/api/v1/admin/...`) | Enables admin client apps |
| Followed hashtags | Low effort |
| Push notification subscriptions (Web Push) | Medium effort |
| Trending tags / posts | Medium effort |
| Announcements | Low effort |
| Account migration (Move activity) | High effort |
| Relay support | Medium effort |

### Phase 3 — Scale & Operations

| Feature | Notes |
|---------|-------|
| Read replica routing | Needed at ~10k+ users |
| pgBouncer connection pooling | Needed with many pods |
| Media processing pipeline (thumbnails, video transcoding) | Background worker |
| CDN integration for media | Performance |
| Elasticsearch / Typesense integration (optional full-text) | Alternative to pg FTS at scale |
| Rate limiting as middleware option | For deployments without gateway |
| Multi-language admin UI | i18n |

---

*End of Monstera-fed Project Specification v0.1*
