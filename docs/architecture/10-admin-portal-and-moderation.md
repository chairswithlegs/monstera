# Admin portal and moderation

This document describes the desired admin UI (HTMX + Go templates), session auth, RBAC, and moderation flows. Build order is in [roadmap.md](../roadmap.md).

---

## Design decisions

| Question | Decision |
|----------|----------|
| Session ID mechanism | **Opaque random** (32 bytes crypto/rand, hex-encoded) — no HMAC signing. 256 bits of entropy makes brute-force infeasible; HMAC adds complexity without meaningful security gain. |
| Admin session storage | Cache store key `admin_session:{sessionID}` → JSON payload. 8-hour sliding TTL (extended on activity). |
| Frontend technology | **HTMX + Go templates + Pico.css** — zero build step; files embedded directly. |
| RBAC enforcement | **Per-handler role check** via a `requireAdmin` helper — not a separate middleware layer. `AdminAuth` middleware gates all `/admin/` routes; individual handlers call `requireAdmin(ctx)` for admin-only endpoints. |
| Domain block permissions | **Admin-only.** Moderator access deferred to future features. |
| `admin_actions` audit log | Dedicated table; one row per moderation action. Immutable append-only log. |
| Known instances tracking | New `known_instances` table, populated on federation ingress/egress. |
| `server_filters.whole_word` | Amend migration 000021 to add `whole_word BOOLEAN NOT NULL DEFAULT FALSE`. |
| Password strength (registration) | Minimum 8 characters. No complexity rules (length is the primary defense; complexity rules annoy users without meaningfully improving security per NIST 800-63B). |
| RSA key generation timing | **Synchronous** — RSA-2048 takes ~10ms, well within acceptable request latency. |
| `Flag` AP activity for remote reports | **Deferred** to future features. |
| 2FA for admin portal | **Deferred** to future features. |
| Admin email notifications on new reports | **Deferred** to future features. |

---

## 1. Admin Session Authentication

**File:** `internal/api/admin/auth.go`

### Session ID Generation

Session IDs are 32 bytes from `crypto/rand`, hex-encoded to a 64-character string. This provides 256 bits of entropy — computationally infeasible to brute-force.

**Why opaque random over HMAC-signed:** The session ID is an opaque lookup key into the cache store. Its sole purpose is to be unguessable. With 256 bits of randomness, an attacker cannot forge a valid session ID without access to the cache itself. HMAC signing would guard against forgery of structured tokens, but a random session ID has no structure to forge. Adding HMAC signing introduces key management complexity (`SECRET_KEY_BASE` rotation would invalidate all sessions) for no security benefit.

### Session Storage

Sessions are stored in the cache abstraction (same `cache.Store` used throughout Monstera-fed):

- **Cache key:** `admin_session:{sessionID}`
- **Value:** JSON `{"user_id": "...", "account_id": "...", "role": "admin|moderator"}`
- **TTL:** 8 hours, refreshed on every authenticated request (sliding window)

The sliding TTL means active admins stay logged in; inactive sessions expire after 8 hours. On each request that passes `AdminAuth`, the middleware re-sets the cache entry with a fresh 8-hour TTL.

### Cookie Configuration

| Property | Value |
|----------|-------|
| Name | `monstera-fed_admin_session` |
| Value | The 64-character hex session ID |
| Path | `/admin` |
| HttpOnly | `true` |
| SameSite | `Strict` |
| Secure | `true` when `APP_ENV=production` |
| MaxAge | 28800 (8 hours, matches cache TTL) |

`Path=/admin` scopes the cookie so it is never sent with Mastodon API requests or federation inbox POSTs.

---

## 2. Role-Based Access Control

### Approach

All admin portal routes are gated by the `AdminAuth` middleware, which requires `role = 'admin'` or `role = 'moderator'`. Within that group, specific endpoints require `role = 'admin'`. The check is a per-handler call, not a separate middleware layer, because:

1. Only a few endpoint groups are admin-only (settings, role management, account deletion, domain blocks).
2. A per-handler check is explicit and readable — no middleware ordering surprises.
3. The check is a one-liner; the overhead of a separate middleware is not warranted.

### Permission Matrix

| Endpoint Group | Admin | Moderator |
|---------------|-------|-----------|
| Dashboard | full | full |
| Users — browse, view detail | full | full |
| Users — suspend/unsuspend/silence | full | full |
| Users — set role | full | **denied** |
| Users — delete account | full | **denied** |
| Registrations — approve/reject | full | full |
| Invites — create/revoke | full | full |
| Reports — list, view, assign, resolve | full | full |
| Federation — known instances (read) | full | full |
| Federation — domain blocks (create/remove) | full | **denied** |
| Content — custom emoji | full | full |
| Content — server filters | full | full |
| Instance Settings | full | **denied** |

---

## 3. Frontend Technology — HTMX + Go Templates + Pico.css

### Rationale

**Revision from IMPLEMENTATION 01:** IMPLEMENTATION 01 chose React + Vite for the admin portal. That decision is revised here based on a better understanding of the admin portal's requirements. The admin UI is a CRUD application — paginated tables, forms, confirmation dialogs, and stat cards. This is exactly the use case HTMX was designed for.

| Criterion | React + Vite | HTMX + Go templates |
|-----------|-------------|---------------------|
| Build step | Node.js in CI, multi-stage Docker | None — files embedded directly |
| Runtime size | ~140KB (React + ReactDOM minified) | ~14KB (HTMX) + ~13KB (Pico.css) |
| State management | Client-side (useState/useReducer) | Server-side (Go templates render current state) |
| Developer toolchain | npm, TypeScript, Vite, React devtools | Go templates + browser devtools |
| Fit for CRUD admin UI | Overpowered — most React features unused | Purpose-built for server-rendered CRUD |

**Key advantages of HTMX for this use case:**

1. **Zero build step.** HTMX (~14KB) and Pico.css (~13KB) are vendored as static files and embedded directly into the Go binary. No Node.js anywhere in the build pipeline.
2. **Simplified Dockerfile.** The multi-stage build from IMPLEMENTATION 01 drops from 3 stages (Node → Go → distroless) to 2 stages (Go → distroless).
3. **Clean data flow.** Admin handlers call the service layer, then render Go templates with the results. No JSON serialization layer between the admin UI and the business logic.
4. **Progressive enhancement.** The UI works as standard HTML forms and links; HTMX adds smooth partial-page updates on top. Browsers with JS disabled still function (not a primary use case, but a sign of architectural simplicity).
5. **Phase 2 compatibility.** The Mastodon Admin API (`/api/v1/admin/...`) will call the same service layer with JSON responses. The service layer is unaffected by this choice.

### Amendments to IMPLEMENTATION 01

The following sections of IMPLEMENTATION 01 are superseded by this decision:

| IMPLEMENTATION 01 Section | Change |
|-------------------|--------|
| Design Decisions table: "Admin portal frontend" | React + Vite → HTMX + Go templates + Pico.css |
| §8 "Admin Portal: Go Embed Setup" | React/Vite build pipeline → static file embed + template embed (see below) |
| Makefile: `build-admin` target | Removed — no build step needed |
| Multi-stage Dockerfile: Node.js stage | Removed — Go stage embeds templates and static files directly |
| `web/admin/` directory | SPA source tree → templates + vendored static assets |

**Revised Dockerfile** (replaces IMPLEMENTATION 01 §8 three-stage build):


**Revised Makefile:**

```makefile
.PHONY: build

build:
	CGO_ENABLED=0 go build -o bin/monstera-fed ./cmd/monstera-fed

docker-build:
	docker build -t monstera-fed:latest .
```

### File Structure

```
web/admin/
├── static/
│   ├── htmx.min.js            — vendored HTMX 2.x (~14KB)
│   ├── pico.min.css            — vendored Pico.css 2.x (~13KB)
│   └── admin.css               — custom admin overrides (minimal)
└── templates/
    ├── layout.html             — base layout: sidebar nav + content area + flash messages
    ├── login.html              — standalone login page (no sidebar)
    ├── dashboard.html
    ├── users.html
    ├── user_detail.html
    ├── registrations.html
    ├── invites.html
    ├── reports.html
    ├── report_detail.html
    ├── federation.html
    ├── content_emojis.html
    ├── content_filters.html
    ├── settings.html
    └── partials/
        ├── users_table.html        — table body for HTMX swap on search/paginate
        ├── user_actions.html       — action buttons, swapped after moderation action
        ├── registrations_table.html
        ├── invites_table.html
        ├── reports_table.html
        ├── report_actions.html
        ├── domain_blocks_table.html
        ├── emojis_grid.html
        ├── filters_table.html
        ├── settings_form.html
        └── flash.html              — success/error message, swapped into layout
```

### HTMX Request Detection

Handlers use the `HX-Request` header to decide between full-page and partial responses.

---

## 4. New Database Tables

Three new tables and two amendments to existing migrations.

### Migration Summary

| # | File prefix | Description |
|---|-------------|-------------|
| — | (amend 000002) | Add `size_bytes BIGINT NOT NULL DEFAULT 0` to `media_attachments` |
| — | (amend 000021) | Add `whole_word BOOLEAN NOT NULL DEFAULT FALSE` to `server_filters` |
| 30 | `000030_create_admin_actions` | Moderation audit log |
| 31 | `000031_create_known_instances` | Federated instance tracking |
| 32 | `000032_create_custom_emojis` | Custom emoji storage |

Since all design outputs are pre-implementation, amendments to existing migrations are applied directly to their DDL — no ALTER TABLE migrations needed.

### Amendment: `media_attachments` (migration 000002)

Add `size_bytes` to enable the dashboard storage utilisation metric:


The media upload handler already knows the file size from the `Content-Length` header or by counting bytes read. This column enables `SELECT COALESCE(SUM(size_bytes), 0) FROM media_attachments` for the admin dashboard.

### Amendment: `server_filters` (migration 000021)

Add `whole_word` to support word-boundary matching:


When `whole_word = TRUE`, the filter phrase is matched at word boundaries (regex `\b` anchors) rather than as a substring. This prevents "ass" from matching "assistant".

### `000030_create_admin_actions.up.sql`


**Action values:**

| `action` | Description | `metadata` fields |
|----------|-------------|-------------------|
| `suspend` | Account suspended | — |
| `unsuspend` | Suspension reversed | — |
| `silence` | Account silenced | — |
| `unsilence` | Silence reversed | — |
| `warn` | Warning email sent | — |
| `delete_account` | Account hard-deleted | — |
| `set_role` | User role changed | `{"old_role": "...", "new_role": "..."}` |
| `approve_registration` | Pending user approved | — |
| `reject_registration` | Pending user rejected | `{"email_reason": "..."}` |
| `resolve_report` | Report resolved | `{"report_id": "...", "resolution": "..."}` |
| `assign_report` | Report assigned | `{"report_id": "..."}` |
| `create_domain_block` | Domain block created | `{"domain": "...", "severity": "..."}` |
| `remove_domain_block` | Domain block removed | `{"domain": "..."}` |

The `admin_actions` table is append-only — rows are never updated or deleted. This ensures a tamper-resistant audit trail.

### `000030_create_admin_actions.down.sql`


### `000031_create_known_instances.up.sql`


**Population strategy:** The `known_instances` table is upserted whenever Monstera-fed interacts with a remote domain:

1. **Inbox processing** — when an activity arrives, extract the domain from the actor's AP ID and upsert with `last_seen_at = NOW()`.
2. **Remote actor resolution** — when fetching an unknown remote actor, upsert the domain.
3. **Outbound delivery** — when the federation worker delivers to a remote inbox, upsert the domain.

The upsert is a single query (`ON CONFLICT (domain) DO UPDATE SET last_seen_at = NOW()`) — cheap enough to run on every interaction without batching.

The `software` and `software_version` fields are populated lazily: when the admin views the federation page, the handler can trigger a background NodeInfo fetch for instances with NULL software. This avoids blocking federation processing on NodeInfo lookups.

**Account count per instance** is computed at query time via a correlated subquery against `accounts.domain` (indexed by `idx_accounts_domain`). This avoids maintaining a denormalized counter.

### `000031_create_known_instances.down.sql`


### `000032_create_custom_emojis.up.sql`


The `shortcode + domain` uniqueness constraint allows the same shortcode to exist as both a local emoji and copies from different remote instances. The local index powers `GET /api/v1/custom_emojis` (returns only local, enabled emojis for client display).

### `000032_create_custom_emojis.down.sql`


---

## 6. Store Interface Additions

Type aliases to add in `store.go`:


---

## 7. Admin Handler Signatures

All handlers live in `internal/api/admin/`. Each handler struct takes the services it needs (constructor injection). All handlers behind `AdminAuth` middleware can access `SessionFromContext(ctx)`.

### File Layout

```
internal/api/admin/
├── auth.go             — LoginHandler, LogoutHandler, AdminAuth middleware, session helpers
├── templates.go        — Templates type, NewTemplates, RenderPage, RenderPartial
├── helpers.go          — requireAdmin, isHTMX, parseIntParam, pagination helpers
├── dashboard.go        — DashboardHandler
├── users.go            — UsersHandler
├── registrations.go    — RegistrationsHandler
├── invites.go          — InvitesHandler
├── reports.go          — ReportsHandler
├── federation.go       — FederationHandler
├── content.go          — ContentHandler
└── settings.go         — SettingsHandler
```
 
---
 
## 8. `internal/service/moderation_service.go` — Moderation Logic
 
### AP Outbox Addition
 
The `OutboxPublisher` (IMPLEMENTATION 07, §4) needs one new method:
 
 
---
 
## 9. `internal/service/registration_service.go` — Registration & Invites

**Note on registration reason:** The `RegisterRequest.Reason` field (used in approval mode to explain why the user wants to join) needs storage. Options:

1. Add `registration_reason TEXT` column to `users`. Clean and queryable.
2. Store in a JSONB `metadata` column on `users`. More flexible, less queryable.

**Decision:** Add `registration_reason TEXT` to the `users` table (amend migration 000004). This is a simple, nullable column that the admin registrations view queries directly. No JSONB complexity needed for a single field. Length is enforced at the API layer: max 500 characters (enough for a short paragraph explaining why the user wants to join).

### Amendment to `users` table (migration 000004)

Add `registration_reason` for approval-mode registrations:


---

## 10. `internal/service/instance_service.go` — Instance Settings

### Cache Strategy

- **Cache key:** `instance:settings`
- **Value:** JSON-serialized `map[string]string` containing all rows from `instance_settings`.
- **TTL:** 5 minutes.
- **Invalidation:** On any write via `Set`, the cache key is deleted immediately. The next read repopulates it from the database.

This is a simple read-through cache: `GetAll` checks the cache first, deserialises and returns if present, otherwise queries the database and populates the cache. Individual `Get` calls use `GetAll` internally and extract the requested key from the map — this avoids per-key cache entries while keeping the hot path (reading a single setting during request processing) to a single cache lookup.


---

## 11. Additional Email Templates

IMPLEMENTATION 05 defined four email templates: `email_verification`, `password_reset`, `invite`, and `moderation_action`. The admin portal and registration flows in this design require two additional templates.

### `account_approved.html` / `account_approved.txt`

Sent when an admin approves a pending registration (approval mode).


**Subject:** `Welcome to {InstanceName}, @{Username}!`

**Content:** Notifies the user that their registration has been approved and they can now log in using any Mastodon-compatible client. Includes the instance URL.

### `registration_rejected.html` / `registration_rejected.txt`

Sent when an admin rejects a pending registration.


**Subject:** `Your registration at {InstanceName}`

**Content:** Informs the user that their registration was not approved, includes the admin's reason. Keeps a neutral, non-hostile tone.

### File additions to `internal/email/templates/`

```
internal/email/templates/
├── ... (existing templates) ...
├── account_approved.html
├── account_approved.txt
├── registration_rejected.html
└── registration_rejected.txt
```

The `EmailService` gains two new methods following the same pattern as `SendModerationAction`:


---

## 12. Domain Block Follow Severance Query

When a domain block with severity `suspend` is created, all follows between local accounts and accounts on the blocked domain must be severed. This requires a query not yet in the existing `follows.sql`:


Add to `FollowStore` interface:


---

## 13. Amendments to Prior IMPLEMENTATIONs

This design output introduces changes to prior outputs. All amendments are documented here for traceability.

### IMPLEMENTATION 01 — Project Foundation

| Section | Change |
|---------|--------|
| Design Decisions: "Admin portal frontend" | React + Vite → **HTMX + Go templates + Pico.css** |
| §8 "Admin Portal: Go Embed Setup" | Replaced by §3 of this output (template embed + static file embed) |
| Makefile `build-admin` target | **Removed** — no Node.js build step |
| Multi-stage Dockerfile | **Simplified** — 2 stages (Go builder → distroless), Node.js stage removed |
| `web/admin/` directory structure | SPA source tree → templates/ + static/ (see §3 file structure) |
| `RouterDeps` | Add `ModerationSvc`, `RegistrationSvc`, `InstanceSvc` (already partially listed) |
| Open Question #3 (invite generation) | **Resolved**: Phase 1 invites are admin/moderator-only via the admin portal. Regular user invite generation deferred. |

### IMPLEMENTATION 02 — Data Model & Database Layer

| Section | Change |
|---------|--------|
| Migration 000002 (`media_attachments`) | Add `size_bytes BIGINT NOT NULL DEFAULT 0` |
| Migration 000004 (`users`) | Add `registration_reason TEXT` |
| Migration 000021 (`server_filters`) | Add `whole_word BOOLEAN NOT NULL DEFAULT FALSE` |
| Migration list | Add 000030 (`admin_actions`), 000031 (`known_instances`), 000032 (`custom_emojis`) |
| §7 Store Interface | Add `AdminActionStore`, `KnownInstanceStore`, `CustomEmojiStore` to root `Store`; add methods to `UserStore`, `InviteStore`, `ReportStore`, `StatusStore`, `AttachmentStore`, `FollowStore` |
| Type aliases | Add `AdminAction`, `KnownInstance`, `CustomEmoji` |

### IMPLEMENTATION 05 — Email Abstraction

| Section | Change |
|---------|--------|
| Template list | Add `account_approved.html/.txt`, `registration_rejected.html/.txt` |
| `EmailService` | Add `SendAccountApproved`, `SendRegistrationRejected` methods |
| Template data structs | Add `AccountApprovedData`, `RegistrationRejectedData` |

### IMPLEMENTATION 07 — ActivityPub & Federation

| Section | Change |
|---------|--------|
| §4 OutboxPublisher | Add `DeleteActor(ctx, account)` method — sends `Delete{Person}` to all remote followers |

---

Unresolved decisions for this area are in [open_questions.md](../open_questions.md).

---
