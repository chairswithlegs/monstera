# Monstera-fed — Implementation Order

> Build sequence for Phase 1, organized around dependency order and validation milestones.

---

## Guiding Principles

1. **Build leaves first.** Packages with no internal dependencies are implemented before packages that depend on them.
2. **Validate early and often.** Each stage ends with a concrete validation step — a passing test suite, a working migration, a client connection — not just "code compiles."
3. **Defer horizontal breadth.** Implement one full vertical slice (e.g., one API endpoint from handler → service → store) before fanning out to all endpoints. This catches integration issues early.

---

## Stage 1 — Scaffolding & Leaf Packages

**Packages:** `config`, `domain`, `uid`, `observability`

These have zero internal dependencies and unlock everything downstream.

| Package | Key deliverables |
|---------|-----------------|
| `internal/config` | `Load()`, `Validate()`, env var parsing, HKDF sub-key derivation from `SECRET_KEY_BASE` |
| `internal/domain` | All domain structs (`Account`, `Status`, `Follow`, etc.), `errors.go` with sentinel errors |
| `internal/uid` | ULID generation wrapper |
| `internal/observability` | slog JSON setup, Prometheus registry, `RequestID` middleware |

**Validation:** Unit tests only. `go test ./internal/config/... ./internal/domain/... ./internal/uid/... ./internal/observability/...` passes. No infrastructure required.

---

## Stage 2 — Data Layer

**Packages:** `store/postgres` (migrations + sqlc), `cache`

| Package | Key deliverables |
|---------|-----------------|
| `internal/store/migrations/` | All migration SQL files (000001–000032) |
| `internal/store/postgres/` | sqlc config, generated code, `db.go` (pgxpool setup), `Store` interface |
| `internal/cache/memory` | Ristretto-backed in-memory cache |
| `internal/cache/redis` | Redis/Valkey cache driver |

**Validation:**
- Stand up PostgreSQL via Docker Compose.
- Run `cmd/monstera-fed migrate up` — all 32 migrations apply cleanly.
- Run `cmd/monstera-fed migrate down` — full rollback succeeds.
- Integration tests (build tag `integration`) verify every sqlc query against a real database.
- Cache unit tests verify get/set/delete/TTL for both memory and Redis drivers.

This is the highest-risk stage — schema bugs found later are expensive. Invest in thorough query-level tests here.

---

## Stage 3 — Infrastructure Abstractions

**Packages:** `media`, `email`, `nats`

| Package | Key deliverables |
|---------|-----------------|
| `internal/media/local` | Local filesystem storage |
| `internal/media/s3` | S3-compatible storage |
| `internal/email/noop` | No-op sender (dev/testing) |
| `internal/email/smtp` | SMTP sender |
| `internal/nats/` | Client wrapper, `EnsureStreams`, federation producer |

**Validation:**
- Media: unit tests with a temp directory (local), integration test with MinIO (S3).
- Email: unit tests with noop driver, manual SMTP test with Mailpit or similar.
- NATS: integration test that publishes and consumes a message through JetStream.

These packages are isolated — they can be built in parallel by different contributors.

---

## Stage 4 — Auth & Identity

**Packages:** `oauth`, `ap` (HTTP signatures + actor key management)

| Package | Key deliverables |
|---------|-----------------|
| `internal/oauth/` | Authorization code + PKCE flow, token issuance, token validation, scope checking |
| `internal/ap/httpsig.go` | HTTP Signature sign/verify with key rotation retry |
| `internal/ap/vocab.go` | ActivityStreams type definitions |

**Validation:**
- OAuth: integration test that walks through the full authorization code flow (create app → authorize → token → verify).
- HTTP Signatures: unit tests with known-good signature fixtures from the Mastodon test suite or [go-fed/httpsig](https://github.com/go-fed/httpsig) test vectors.
- Private key encryption: round-trip test (generate key → encrypt → store → load → decrypt → sign).

---

## Stage 5 — Service Layer (First Vertical Slice)

**Package:** `service`

Build one complete slice first to validate the full stack:

**Recommended first slice: Account registration + status creation**

| Service | Methods |
|---------|---------|
| `AccountService` | `Create`, `GetByID`, `GetByUsername` |
| `StatusService` | `Create`, `GetByID`, `Delete` |
| `TimelineService` | `Home`, `PublicLocal` |

**Validation:**
- Unit tests with mocked store/cache.
- Integration test that registers a user, creates a status, and reads it back from the home timeline — exercising the full store → service → domain round-trip.

After this slice works end-to-end, fan out to the remaining services: `FollowService`, `NotificationService`, `ModerationService`, `FederationService`, etc.

---

## Stage 6 — API Handlers (First Client Connection)

**Package:** `api` (Mastodon REST handlers, router, middleware)

Build handlers in this order — each enables a meaningful client interaction:

| Priority | Endpoints | Why |
|----------|----------|-----|
| 1 | `POST /api/v1/apps`, OAuth flow, `GET /api/v1/accounts/verify_credentials` | Client can authenticate |
| 2 | `POST /api/v1/statuses`, `GET /api/v1/timelines/home` | Client can post and read |
| 3 | `GET /api/v1/accounts/:id`, `POST /api/v1/accounts/:id/follow` | Client can browse profiles and follow |
| 4 | `GET /api/v1/notifications` | Client shows follow/mention notifications |
| 5 | `POST /api/v2/media`, `GET /api/v1/timelines/public` | Media uploads, public timeline |
| 6 | Remaining CRUD endpoints | Full API surface |

### Milestone: First client connection

After priorities 1–2 are implemented, connect a real Mastodon client and verify:
- App registration succeeds.
- OAuth login flow completes.
- Posting a status works.
- Home timeline displays the post.

This is the most important validation milestone in the project. Client compatibility issues surface here — undocumented API behaviors, missing response fields, header quirks. Fix these before building more endpoints.

**Validation:**
- `httptest`-based handler tests for every endpoint.
- Manual testing with **Tusky** (Android) — the most widely used mobile client, exercises core API well.
- Manual testing with the **Mastodon web UI** — the reference frontend implementation. If it works here, most other clients will too.

---

## Stage 7 — Federation

**Packages:** `ap/inbox.go`, `ap/outbox.go`, `nats/federation/` (producer + worker)

| Component | Key deliverables |
|-----------|-----------------|
| Outbox | On local status create → enqueue `Create{Note}` delivery to follower inboxes |
| Inbox | Receive and process `Create`, `Delete`, `Follow`, `Undo`, `Accept`, `Reject`, `Announce`, `Like`, `Block`, `Update` |
| Worker | Pull from NATS `FEDERATION` stream, deliver via HTTP POST with signatures, retry with backoff, DLQ after 5 attempts |
| WebFinger + NodeInfo | Discovery endpoints |

### Milestone: First federated interaction

Stand up a local Mastodon instance (via its Docker Compose) alongside Monstera-fed. Verify:
- Mastodon can discover a Monstera-fed user via WebFinger.
- Following a Monstera-fed user from Mastodon sends a `Follow` activity that Monstera-fed accepts.
- A Monstera-fed post appears in the Mastodon user's home timeline.
- A Mastodon reply appears in Monstera-fed's inbox and creates a notification.

This is the second critical milestone. Federation bugs are the hardest to diagnose — invest time here.

**Validation:**
- Unit tests for each activity type handler with fixture JSON.
- Integration test with a real Mastodon instance (Docker Compose).
- Verify HTTP Signature verification against Mastodon's signatures.

---

## Stage 8 — SSE Streaming

**Packages:** `nats/streaming/` (publisher + hub)

| Component | Key deliverables |
|-----------|-----------------|
| Publisher | Publish `SSEEvent` to NATS `events.*` subjects on status/notification/delete |
| Hub | Subscribe to NATS subjects on demand, fan out to connected SSE clients |
| `GET /api/v1/streaming` | SSE endpoint with `stream` query parameter |

**Validation:**
- Connect a Mastodon client's streaming endpoint. Post a status from another account. Verify the status appears in real-time without a page refresh.
- Test with two replicas (scale `monstera-fed` to 2 in Docker Compose) to verify cross-replica fan-out via NATS.

---

## Stage 9 — Admin Portal & Moderation

**Packages:** `api/admin/` (handlers + templates), `service/moderation_service.go`

Build in this order:
1. Login/logout + session management
2. Dashboard (stats)
3. User management (browse, suspend, silence)
4. Reports (list, view, resolve)
5. Federation (known instances, domain blocks)
6. Instance settings
7. Invites + registration management
8. Content (custom emoji, filters)

**Validation:**
- Browser testing of each admin page.
- Verify HTMX partial updates work (actions like suspend/unsuspend swap the action buttons without full reload).
- Verify RBAC: moderator cannot access admin-only pages.

---

## Stage 10 — Deployment & Polish

| Task | Details |
|------|---------|
| Docker Compose | Full local dev environment (Monstera-fed, PostgreSQL, NATS, Redis, MinIO, Mailpit) |
| Kubernetes manifests | Deployment, Service, ConfigMap, HPA, NATS Helm values |
| Health endpoints | `/healthz/live`, `/healthz/ready` |
| Prometheus metrics | Verify all counters/histograms emit under load |
| CI pipeline | Lint → unit tests → integration tests (GitHub Actions) |

**Validation:**
- Deploy to a real Kubernetes cluster (or `kind`/`k3d`).
- Run two replicas. Verify stateless behavior: any replica can serve any request.
- Federation works across a real network (not just localhost).

---

## Summary: Validation Milestones

| After | You can verify |
|-------|---------------|
| Stage 2 | Migrations apply and roll back cleanly; queries return correct results |
| Stage 4 | OAuth flow works; HTTP signatures verify against known-good fixtures |
| Stage 6 | A real Mastodon client can log in, post, and read timelines |
| Stage 7 | A local Mastodon instance can follow and receive posts from Monstera-fed |
| Stage 8 | Real-time updates appear in client without refresh |
| Stage 9 | Admin can log in, view dashboard, and moderate users |
| Stage 10 | Multi-replica Kubernetes deployment works end-to-end |
