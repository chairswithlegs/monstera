# Email abstraction

This document describes the desired email sender interface and drivers (noop, SMTP).
---

## Design decisions

| Question | Decision |
|----------|----------|
| Implementation count | **Two drivers: noop + SMTP** — SendGrid, Postmark, SES, and Mailgun all expose SMTP endpoints, so a single SMTP driver covers every provider. Vendor-specific HTTP API drivers can be added later without changing the `Sender` interface. |
| SMTP library | **`github.com/jordan-wright/email`** — lightweight, handles STARTTLS and multipart MIME cleanly; no heavy framework needed for low-volume transactional email |
| Connection reuse vs. per-message dial | **Per-message dial** — transactional email volume is low (registrations, resets); connection pooling adds complexity with no measurable benefit |
| Token storage | **Dedicated `email_tokens` DB table** — tokens survive cache evictions, server restarts, and multi-replica deployments; negligible write volume |
| Token format | **32 bytes crypto/rand, base64url-encoded** — stored as SHA-256 hash in DB; raw token only in the email URL |
| Template engine | `html/template` for HTML variants, `text/template` for text variants — no external dependency |
| Factory input | `email.Config` struct (not `*config.Config`) — keeps the email package dependency-free, matching cache and media patterns |
| `Message.From` override | Optional — defaults to `Config.From`/`Config.FromName` when empty |

---

## Responsibilities and flows

- **Interface and drivers**: `internal/email/sender.go` defines the `Sender` interface and `Message` type; `New` chooses between the noop and SMTP drivers based on config. Callers do not depend on any particular provider.
- **Templates**: `internal/email/templates.go` plus the files under `internal/email/templates/` define all user-visible email content (verification, reset, invite, moderation). The architecture requires that templates be pure data; no logic lives in the templates beyond simple conditionals/loops.
- **Tokens**: flows that require one-click links (email verification, password reset, invites) create records in `email_tokens` and embed only opaque tokens in URLs. The database stores only a hash; services validate and consume tokens via `email_tokens` queries.
- **Service layer**: `internal/service/email_service.go` owns the higher-level flows (e.g. “send verification email”), composing: token creation, template rendering, and a call to `Sender.Send`.
- **Failure semantics**: email-sending failures are logged and surfaced to operators, but should not corrupt domain state; for example, account creation can succeed even if the verification email fails, as long as the token is recorded. Retries or operator re-send actions are handled at the service/application level.

Unresolved decisions for this area are in [open_questions.md](../open_questions.md).

---

# Email abstraction

## Design decisions

| Question | Decision |
|----------|----------|
| Implementation count | **Two drivers: noop + SMTP** — SendGrid, Postmark, SES, and Mailgun all expose SMTP endpoints, so a single SMTP driver covers every provider. Vendor-specific HTTP API drivers can be added later without changing the `Sender` interface. |
| SMTP library | **`github.com/jordan-wright/email`** — lightweight, handles STARTTLS and multipart MIME cleanly; no heavy framework needed for low-volume transactional email |
| Connection reuse vs. per-message dial | **Per-message dial** — transactional email volume is low (registrations, resets); connection pooling adds complexity with no measurable benefit |
| Token storage | **Dedicated `email_tokens` DB table** — tokens survive cache evictions, server restarts, and multi-replica deployments; negligible write volume |
| Token format | **32 bytes crypto/rand, base64url-encoded** — stored as SHA-256 hash in DB; raw token only in the email URL |
| Template engine | `html/template` for HTML variants, `text/template` for text variants — no external dependency |
| Factory input | `email.Config` struct (not `*config.Config`) — keeps the email package dependency-free, matching cache and media patterns |
| `Message.From` override | Optional — defaults to `Config.From`/`Config.FromName` when empty |

---

## 6. Token Storage Design

### Comparison

| Criteria | Cache (`confirm:{token}` → userID) | DB table (`email_tokens`) |
|----------|-------------------------------------|---------------------------|
| **Durability** | Lost on cache eviction, memory driver restart, or Redis flush | Persists until explicitly consumed or expired |
| **Multi-replica** | Requires `CACHE_DRIVER=redis`; memory driver would lose tokens on the replica that didn't write them | Works on all replicas via shared PostgreSQL |
| **Replay prevention** | Delete key on use; race condition possible without atomic get-and-delete (Redis supports `GETDEL`; memory driver does not) | `consumed_at IS NOT NULL` check with row-level locking; no race condition |
| **Auditability** | No record of issued/consumed tokens | Full audit trail: issued, consumed, expired |
| **Cleanup** | Automatic via TTL | Periodic `DELETE WHERE expires_at < NOW()` reaper query |
| **Write volume** | Negligible | Negligible (one row per registration or password reset) |
| **Complexity** | Simpler — no migration needed | Requires a new table and migration |

### Recommendation: DB table

A confirmation token that survives for 24 hours must not be lost to cache eviction. A user who registers and checks their email 12 hours later expects the link to work. The memory cache driver has no durability guarantees, and even Redis under memory pressure may evict keys before their TTL expires (when `maxmemory-policy` allows it).

The DB table also provides atomic consume-or-reject semantics: a `SELECT ... WHERE consumed_at IS NULL FOR UPDATE` inside a transaction guarantees that a token can only be used once, even under concurrent requests. The cache approach requires `GETDEL` (Redis 6.2+), which the `cache.Store` interface does not expose.

The write volume is negligible — one row per registration, one per password reset. A daily reaper query (`DELETE FROM email_tokens WHERE expires_at < NOW()`) keeps the table small.

### Token Generation and Storage


**Why hash the token before storage:**

The raw token is sent in the email URL. The database stores only the SHA-256 hash. If the database is compromised, an attacker cannot reconstruct valid confirmation/reset URLs from the hashed values. This is the same pattern used for OAuth access tokens in the cache (IMPLEMENTATION 03, §6).
