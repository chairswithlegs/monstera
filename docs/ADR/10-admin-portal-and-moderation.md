# ADR 10 — Admin Portal & Content Moderation

> **Status:** Draft v0.1 — Feb 24, 2026  
> **Addresses:** `design-prompts/10-admin-portal-and-moderation.md`

---

## Design Decisions (answered before authoring)

| Question | Decision |
|----------|----------|
| Session ID mechanism | **Opaque random** (32 bytes crypto/rand, hex-encoded) — no HMAC signing. 256 bits of entropy makes brute-force infeasible; HMAC adds complexity without meaningful security gain. |
| Admin session storage | Cache store key `admin_session:{sessionID}` → JSON payload. 8-hour sliding TTL (extended on activity). |
| Frontend technology | **HTMX + Go templates + Pico.css** — zero build step; files embedded directly. Revises the React + Vite decision from ADR 01 (see §3 for rationale and ADR 01 amendments). |
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

### Handler Signatures

```go
package admin

// LoginHandler renders the login page (GET) and processes login form submissions (POST).
type LoginHandler struct {
    store     store.UserStore
    cache     cache.Store
    templates *Templates
    logger    *slog.Logger
    isDev     bool
}

// ServeHTTP handles both GET /admin/login (render form) and POST /admin/login (authenticate).
//
// POST flow:
//   1. Parse form body: email, password.
//   2. Look up user by email via store.GetUserByEmail.
//   3. Verify password with bcrypt.CompareHashAndPassword.
//   4. Check user.Role is "admin" or "moderator". Reject "user" role.
//   5. Check user.ConfirmedAt is not NULL. Reject unconfirmed accounts.
//   6. Generate 32-byte random session ID, hex-encode.
//   7. Store session in cache: admin_session:{sessionID} → {user_id, account_id, role}.
//   8. Set monstera-fed_admin_session cookie.
//   9. Redirect to /admin/ (HTTP 303 See Other).
//
// On any failure: re-render login page with a generic error message.
// Never reveal whether the email exists — always "invalid email or password".
func (h *LoginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request)

// LogoutHandler clears the admin session.
type LogoutHandler struct {
    cache  cache.Store
    logger *slog.Logger
}

// ServeHTTP handles POST /admin/logout.
//   1. Read session ID from cookie.
//   2. Delete admin_session:{sessionID} from cache.
//   3. Set cookie with MaxAge=-1 (expires immediately).
//   4. Redirect to /admin/login.
func (h *LogoutHandler) ServeHTTP(w http.ResponseWriter, r *http.Request)
```

### AdminAuth Middleware

```go
// AdminSession holds the resolved session data injected into the request context.
type AdminSession struct {
    UserID    string
    AccountID string
    Role      string // "admin" or "moderator"
}

// AdminAuth returns middleware that validates the admin session cookie.
//
// Flow:
//   1. Read monstera-fed_admin_session cookie value.
//   2. Look up admin_session:{sessionID} in cache.
//   3. On cache miss or invalid JSON: redirect to /admin/login.
//   4. On cache hit: refresh TTL (sliding window), inject AdminSession into context.
//
// HTMX-aware response:
//   When the session is invalid and the request has the HX-Request header (an HTMX
//   fetch), the middleware returns a 200 with HX-Redirect: /admin/login header instead
//   of a 302. This allows HTMX to perform a full-page redirect rather than swapping
//   a login page fragment into the current page's content area.
func AdminAuth(cache cache.Store, logger *slog.Logger) func(http.Handler) http.Handler

// SessionFromContext retrieves the AdminSession from the request context.
// Returns nil if no session is present (should not happen behind AdminAuth middleware).
func SessionFromContext(ctx context.Context) *AdminSession
```

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

### Helper

```go
// requireAdmin checks that the current session has role "admin".
// Returns true if authorized; writes a 403 error response and returns false otherwise.
// When the request has the HX-Request header, renders an inline "access denied" fragment.
// Otherwise, renders a full error page.
func requireAdmin(w http.ResponseWriter, r *http.Request, templates *Templates) bool
```

---

## 3. Frontend Technology — HTMX + Go Templates + Pico.css

### Rationale

**Revision from ADR 01:** ADR 01 chose React + Vite for the admin portal. That decision is revised here based on a better understanding of the admin portal's requirements. The admin UI is a CRUD application — paginated tables, forms, confirmation dialogs, and stat cards. This is exactly the use case HTMX was designed for.

| Criterion | React + Vite | HTMX + Go templates |
|-----------|-------------|---------------------|
| Build step | Node.js in CI, multi-stage Docker | None — files embedded directly |
| Runtime size | ~140KB (React + ReactDOM minified) | ~14KB (HTMX) + ~13KB (Pico.css) |
| State management | Client-side (useState/useReducer) | Server-side (Go templates render current state) |
| Developer toolchain | npm, TypeScript, Vite, React devtools | Go templates + browser devtools |
| Fit for CRUD admin UI | Overpowered — most React features unused | Purpose-built for server-rendered CRUD |

**Key advantages of HTMX for this use case:**

1. **Zero build step.** HTMX (~14KB) and Pico.css (~13KB) are vendored as static files and embedded directly into the Go binary. No Node.js anywhere in the build pipeline.
2. **Simplified Dockerfile.** The multi-stage build from ADR 01 drops from 3 stages (Node → Go → distroless) to 2 stages (Go → distroless).
3. **Clean data flow.** Admin handlers call the service layer, then render Go templates with the results. No JSON serialization layer between the admin UI and the business logic.
4. **Progressive enhancement.** The UI works as standard HTML forms and links; HTMX adds smooth partial-page updates on top. Browsers with JS disabled still function (not a primary use case, but a sign of architectural simplicity).
5. **Phase 2 compatibility.** The Mastodon Admin API (`/api/v1/admin/...`) will call the same service layer with JSON responses. The service layer is unaffected by this choice.

### Amendments to ADR 01

The following sections of ADR 01 are superseded by this decision:

| ADR 01 Section | Change |
|-------------------|--------|
| Design Decisions table: "Admin portal frontend" | React + Vite → HTMX + Go templates + Pico.css |
| §8 "Admin Portal: Go Embed Setup" | React/Vite build pipeline → static file embed + template embed (see below) |
| Makefile: `build-admin` target | Removed — no build step needed |
| Multi-stage Dockerfile: Node.js stage | Removed — Go stage embeds templates and static files directly |
| `web/admin/` directory | SPA source tree → templates + vendored static assets |

**Revised Dockerfile** (replaces ADR 01 §8 three-stage build):

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

### Go Embed and Template Engine

```go
package admin

//go:embed web/admin/static
var staticFS embed.FS

//go:embed web/admin/templates
var templateFS embed.FS

// Templates wraps parsed html/template trees for all admin views.
// Constructed once at startup; safe for concurrent use.
type Templates struct {
    templates map[string]*template.Template
}

// NewTemplates parses all templates from the embedded FS.
// Every page template is parsed together with layout.html so that
// {{template "content" .}} works for full-page renders, while
// partial templates are parsed standalone for HTMX fragment responses.
func NewTemplates(fs embed.FS) (*Templates, error)

// RenderPage renders a full page (layout + page content).
// Used for initial page loads and non-HTMX requests.
func (t *Templates) RenderPage(w http.ResponseWriter, name string, data any) error

// RenderPartial renders a partial template (no layout wrapper).
// Used for HTMX fragment responses (hx-swap targets).
func (t *Templates) RenderPartial(w http.ResponseWriter, name string, data any) error
```

### HTMX Request Detection

Handlers use the `HX-Request` header to decide between full-page and partial responses:

```go
// isHTMX returns true if the request was initiated by HTMX.
func isHTMX(r *http.Request) bool {
    return r.Header.Get("HX-Request") == "true"
}
```

A typical handler pattern:

```go
func (h *UsersHandler) List(w http.ResponseWriter, r *http.Request) {
    query := r.URL.Query().Get("q")
    page := parseIntParam(r, "page", 1)

    users, total, err := h.moderationSvc.ListUsers(r.Context(), query, page, 50)
    if err != nil {
        h.renderError(w, r, err)
        return
    }

    data := UsersPageData{
        Users:      users,
        Query:      query,
        Page:       page,
        TotalPages: (total + 49) / 50,
        Session:    SessionFromContext(r.Context()),
    }

    if isHTMX(r) {
        h.templates.RenderPartial(w, "users_table", data)
    } else {
        h.templates.RenderPage(w, "users", data)
    }
}
```

### Admin Route Registration

```go
r.Route("/admin", func(r chi.Router) {
    // Public: login
    r.Get("/login", loginHandler.ServeHTTP)
    r.Post("/login", loginHandler.ServeHTTP)

    // Static assets (no auth required — CSS/JS must load on login page)
    r.Handle("/static/*", http.StripPrefix("/admin/static/",
        http.FileServer(http.FS(staticFS))))

    // Authenticated routes
    r.Group(func(r chi.Router) {
        r.Use(AdminAuth(cache, logger))
        r.Post("/logout", logoutHandler.ServeHTTP)

        // Dashboard
        r.Get("/", dashboardHandler.Get)

        // Users
        r.Get("/users", usersHandler.List)
        r.Get("/users/{id}", usersHandler.Detail)
        r.Post("/users/{id}/suspend", usersHandler.Suspend)
        r.Post("/users/{id}/unsuspend", usersHandler.Unsuspend)
        r.Post("/users/{id}/silence", usersHandler.Silence)
        r.Post("/users/{id}/unsilence", usersHandler.Unsilence)
        r.Post("/users/{id}/set-role", usersHandler.SetRole)       // admin-only
        r.Delete("/users/{id}", usersHandler.Delete)                // admin-only

        // Registrations
        r.Get("/registrations", registrationsHandler.List)
        r.Post("/registrations/{id}/approve", registrationsHandler.Approve)
        r.Post("/registrations/{id}/reject", registrationsHandler.Reject)

        // Invites
        r.Get("/invites", invitesHandler.List)
        r.Post("/invites", invitesHandler.Create)
        r.Delete("/invites/{id}", invitesHandler.Revoke)

        // Reports
        r.Get("/reports", reportsHandler.List)
        r.Get("/reports/{id}", reportsHandler.Detail)
        r.Post("/reports/{id}/assign", reportsHandler.Assign)
        r.Post("/reports/{id}/resolve", reportsHandler.Resolve)

        // Federation (domain blocks: admin-only)
        r.Get("/federation", federationHandler.List)
        r.Post("/federation/domain-blocks", federationHandler.CreateBlock)     // admin-only
        r.Delete("/federation/domain-blocks/{domain}", federationHandler.RemoveBlock) // admin-only

        // Content
        r.Get("/content/emojis", contentHandler.ListEmojis)
        r.Post("/content/emojis", contentHandler.UploadEmoji)
        r.Delete("/content/emojis/{shortcode}", contentHandler.DeleteEmoji)
        r.Get("/content/filters", contentHandler.ListFilters)
        r.Post("/content/filters", contentHandler.CreateFilter)
        r.Delete("/content/filters/{id}", contentHandler.DeleteFilter)

        // Settings (admin-only)
        r.Get("/settings", settingsHandler.Get)
        r.Post("/settings", settingsHandler.Update)
    })
})
```

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

```sql
-- Add to CREATE TABLE media_attachments:
    size_bytes  BIGINT NOT NULL DEFAULT 0,       -- file size in bytes; set at upload time
```

The media upload handler already knows the file size from the `Content-Length` header or by counting bytes read. This column enables `SELECT COALESCE(SUM(size_bytes), 0) FROM media_attachments` for the admin dashboard.

### Amendment: `server_filters` (migration 000021)

Add `whole_word` to support word-boundary matching:

```sql
-- Amended CREATE TABLE server_filters:
CREATE TABLE server_filters (
    id         TEXT PRIMARY KEY,
    phrase     TEXT NOT NULL,
    scope      TEXT NOT NULL DEFAULT 'all',    -- 'public_timeline'|'all'
    action     TEXT NOT NULL DEFAULT 'hide',   -- 'warn'|'hide'
    whole_word BOOLEAN NOT NULL DEFAULT FALSE,  -- match at word boundaries only
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

When `whole_word = TRUE`, the filter phrase is matched at word boundaries (regex `\b` anchors) rather than as a substring. This prevents "ass" from matching "assistant".

### `000030_create_admin_actions.up.sql`

```sql
CREATE TABLE admin_actions (
    id                TEXT PRIMARY KEY,
    moderator_id      TEXT NOT NULL REFERENCES users(id),
    target_account_id TEXT REFERENCES accounts(id) ON DELETE SET NULL,  -- NULL for non-account actions; SET NULL preserves audit trail after hard delete
    action            TEXT NOT NULL,
    comment           TEXT,
    metadata          JSONB,                          -- contextual data: report_id, domain, old_role, new_role, etc.
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_admin_actions_target ON admin_actions (target_account_id, created_at DESC)
    WHERE target_account_id IS NOT NULL;
CREATE INDEX idx_admin_actions_moderator ON admin_actions (moderator_id, created_at DESC);
CREATE INDEX idx_admin_actions_created ON admin_actions (created_at DESC);
```

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

```sql
DROP TABLE IF EXISTS admin_actions CASCADE;
```

### `000031_create_known_instances.up.sql`

```sql
CREATE TABLE known_instances (
    id               TEXT PRIMARY KEY,
    domain           TEXT NOT NULL UNIQUE,
    software         TEXT,                         -- from NodeInfo: "mastodon", "pleroma", etc.
    software_version TEXT,
    first_seen_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_known_instances_last_seen ON known_instances (last_seen_at DESC);
```

**Population strategy:** The `known_instances` table is upserted whenever Monstera-fed interacts with a remote domain:

1. **Inbox processing** — when an activity arrives, extract the domain from the actor's AP ID and upsert with `last_seen_at = NOW()`.
2. **Remote actor resolution** — when fetching an unknown remote actor, upsert the domain.
3. **Outbound delivery** — when the federation worker delivers to a remote inbox, upsert the domain.

The upsert is a single query (`ON CONFLICT (domain) DO UPDATE SET last_seen_at = NOW()`) — cheap enough to run on every interaction without batching.

The `software` and `software_version` fields are populated lazily: when the admin views the federation page, the handler can trigger a background NodeInfo fetch for instances with NULL software. This avoids blocking federation processing on NodeInfo lookups.

**Account count per instance** is computed at query time via a correlated subquery against `accounts.domain` (indexed by `idx_accounts_domain`). This avoids maintaining a denormalized counter.

### `000031_create_known_instances.down.sql`

```sql
DROP TABLE IF EXISTS known_instances CASCADE;
```

### `000032_create_custom_emojis.up.sql`

```sql
CREATE TABLE custom_emojis (
    id                TEXT PRIMARY KEY,
    shortcode         TEXT NOT NULL,               -- e.g. "blobcat" (without colons)
    domain            TEXT,                         -- NULL for local; domain for remote copies
    storage_key       TEXT,                         -- key in MediaStore (local emojis only)
    url               TEXT NOT NULL,                -- public URL of the emoji image
    static_url        TEXT,                         -- static (non-animated) version
    visible_in_picker BOOLEAN NOT NULL DEFAULT TRUE,
    disabled          BOOLEAN NOT NULL DEFAULT FALSE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (shortcode, domain)
);

CREATE INDEX idx_custom_emojis_local ON custom_emojis (shortcode)
    WHERE domain IS NULL AND disabled = FALSE;
```

The `shortcode + domain` uniqueness constraint allows the same shortcode to exist as both a local emoji and copies from different remote instances. The local index powers `GET /api/v1/custom_emojis` (returns only local, enabled emojis for client display).

### `000032_create_custom_emojis.down.sql`

```sql
DROP TABLE IF EXISTS custom_emojis CASCADE;
```

---

## 5. New and Amended sqlc Queries

### `admin_actions.sql`

```sql
-- name: CreateAdminAction :one
INSERT INTO admin_actions (id, moderator_id, target_account_id, action, comment, metadata)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListAdminActions :many
SELECT * FROM admin_actions
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListAdminActionsByTarget :many
SELECT * FROM admin_actions
WHERE target_account_id = $1
ORDER BY created_at DESC;

-- name: ListAdminActionsByModerator :many
SELECT * FROM admin_actions
WHERE moderator_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
```

### `known_instances.sql`

```sql
-- name: UpsertKnownInstance :exec
INSERT INTO known_instances (id, domain)
VALUES ($1, $2)
ON CONFLICT (domain) DO UPDATE SET last_seen_at = NOW();

-- name: UpdateKnownInstanceSoftware :exec
UPDATE known_instances SET software = $2, software_version = $3
WHERE domain = $1;

-- name: ListKnownInstances :many
-- Account count is computed via correlated subquery (idx_accounts_domain makes this efficient).
SELECT ki.*,
    (SELECT COUNT(*) FROM accounts a WHERE a.domain = ki.domain) AS accounts_count
FROM known_instances ki
ORDER BY ki.last_seen_at DESC
LIMIT $1 OFFSET $2;

-- name: CountKnownInstances :one
SELECT COUNT(*) FROM known_instances;
```

### `custom_emojis.sql`

```sql
-- name: CreateCustomEmoji :one
INSERT INTO custom_emojis (id, shortcode, domain, storage_key, url, static_url, visible_in_picker)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListLocalCustomEmojis :many
-- Used by GET /api/v1/custom_emojis (Mastodon client API).
SELECT * FROM custom_emojis
WHERE domain IS NULL AND disabled = FALSE
ORDER BY shortcode ASC;

-- name: ListAllCustomEmojis :many
-- Admin view: includes disabled and remote emojis.
SELECT * FROM custom_emojis
ORDER BY domain NULLS FIRST, shortcode ASC
LIMIT $1 OFFSET $2;

-- name: GetCustomEmojiByShortcode :one
SELECT * FROM custom_emojis WHERE shortcode = $1 AND domain IS NULL;

-- name: DeleteCustomEmoji :exec
DELETE FROM custom_emojis WHERE shortcode = $1 AND domain IS NULL;
```

### Additional admin queries (append to existing `.sql` files)

**`users.sql` — additions:**

```sql
-- name: SearchLocalUsersAdmin :many
-- Admin search: find local users by username or email prefix match.
SELECT u.id, u.account_id, u.email, u.confirmed_at, u.role, u.created_at,
       a.username, a.display_name, a.suspended, a.silenced
FROM users u
INNER JOIN accounts a ON a.id = u.account_id
WHERE a.domain IS NULL
  AND ($1::text = '' OR a.username ILIKE $1 OR u.email ILIKE $1)
ORDER BY u.created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountSearchLocalUsersAdmin :one
SELECT COUNT(*) FROM users u
INNER JOIN accounts a ON a.id = u.account_id
WHERE a.domain IS NULL
  AND ($1::text = '' OR a.username ILIKE $1 OR u.email ILIKE $1);

-- name: ListPendingRegistrations :many
SELECT u.id, u.account_id, u.email, u.created_at,
       a.username
FROM users u
INNER JOIN accounts a ON a.id = u.account_id
WHERE u.confirmed_at IS NULL AND a.domain IS NULL
ORDER BY u.created_at ASC
LIMIT $1 OFFSET $2;

-- name: CountPendingRegistrations :one
SELECT COUNT(*) FROM users u
INNER JOIN accounts a ON a.id = u.account_id
WHERE u.confirmed_at IS NULL AND a.domain IS NULL;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;
```

**`invites.sql` — additions:**

```sql
-- name: ListAllInvites :many
-- Admin view: all invites with creator info.
SELECT i.*, u.email AS creator_email
FROM invites i
INNER JOIN users u ON u.id = i.created_by
ORDER BY i.created_at DESC
LIMIT $1 OFFSET $2;
```

**`reports.sql` — additions:**

```sql
-- name: ListReportsAdmin :many
-- Admin view with reporter/target usernames and optional state+category filters.
SELECT r.*,
    reporter_acct.username AS reporter_username,
    target_acct.username AS target_username
FROM reports r
INNER JOIN accounts reporter_acct ON reporter_acct.id = r.account_id
INNER JOIN accounts target_acct ON target_acct.id = r.target_id
WHERE ($1::text = '' OR r.state = $1)
  AND ($2::text = '' OR r.category = $2)
ORDER BY r.created_at DESC
LIMIT $3 OFFSET $4;

-- name: CountOpenReports :one
SELECT COUNT(*) FROM reports WHERE state = 'open';
```

**`statuses.sql` — addition:**

```sql
-- name: CountLocalStatuses :one
SELECT COUNT(*) FROM statuses WHERE local = TRUE AND deleted_at IS NULL;
```

**`media.sql` — addition:**

```sql
-- name: SumMediaStorageBytes :one
SELECT COALESCE(SUM(size_bytes), 0)::bigint FROM media_attachments;
```

---

## 6. Store Interface Additions

### New sub-interfaces

```go
type AdminActionStore interface {
    CreateAdminAction(ctx context.Context, arg db.CreateAdminActionParams) (AdminAction, error)
    ListAdminActions(ctx context.Context, limit, offset int32) ([]AdminAction, error)
    ListAdminActionsByTarget(ctx context.Context, targetAccountID string) ([]AdminAction, error)
    ListAdminActionsByModerator(ctx context.Context, moderatorID string, limit, offset int32) ([]AdminAction, error)
}

type KnownInstanceStore interface {
    UpsertKnownInstance(ctx context.Context, id, domain string) error
    UpdateKnownInstanceSoftware(ctx context.Context, domain, software, version string) error
    ListKnownInstances(ctx context.Context, limit, offset int32) ([]db.ListKnownInstancesRow, error)
    CountKnownInstances(ctx context.Context) (int64, error)
}

type CustomEmojiStore interface {
    CreateCustomEmoji(ctx context.Context, arg db.CreateCustomEmojiParams) (CustomEmoji, error)
    ListLocalCustomEmojis(ctx context.Context) ([]CustomEmoji, error)
    ListAllCustomEmojis(ctx context.Context, limit, offset int32) ([]CustomEmoji, error)
    GetCustomEmojiByShortcode(ctx context.Context, shortcode string) (CustomEmoji, error)
    DeleteCustomEmoji(ctx context.Context, shortcode string) error
}
```

### Additions to existing sub-interfaces

```go
// Add to UserStore:
    SearchLocalUsersAdmin(ctx context.Context, query string, limit, offset int32) ([]db.SearchLocalUsersAdminRow, error)
    CountSearchLocalUsersAdmin(ctx context.Context, query string) (int64, error)
    ListPendingRegistrations(ctx context.Context, limit, offset int32) ([]db.ListPendingRegistrationsRow, error)
    CountPendingRegistrations(ctx context.Context) (int64, error)
    DeleteUser(ctx context.Context, id string) error

// Add to InviteStore:
    ListAllInvites(ctx context.Context, limit, offset int32) ([]db.ListAllInvitesRow, error)

// Add to ReportStore:
    ListReportsAdmin(ctx context.Context, state, category string, limit, offset int32) ([]db.ListReportsAdminRow, error)
    CountOpenReports(ctx context.Context) (int64, error)

// Add to StatusStore:
    CountLocalStatuses(ctx context.Context) (int64, error)

// Add to AttachmentStore:
    SumMediaStorageBytes(ctx context.Context) (int64, error)
```

### Root Store interface — additions

```go
type Store interface {
    // ... existing sub-interfaces ...
    AdminActionStore
    KnownInstanceStore
    CustomEmojiStore
    // ... existing WithTx ...
}
```

Type aliases to add in `store.go`:

```go
type (
    // ... existing aliases ...
    AdminAction   = db.AdminAction
    KnownInstance = db.KnownInstance
    CustomEmoji   = db.CustomEmoji
)
```

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

### `dashboard.go`

```go
type DashboardHandler struct {
    store     store.Store
    cache     cache.Store
    templates *Templates
    logger    *slog.Logger
}

// DashboardData is the template data for the dashboard page.
type DashboardData struct {
    LocalAccounts        int64
    RemoteAccounts       int64
    LocalStatuses        int64
    FederatedInstances   int64
    StorageUsedBytes     int64
    PendingReports       int64
    PendingRegistrations int64
    Session              *AdminSession
}

// Get handles GET /admin/.
// Queries all dashboard metrics. Results are cached under "admin:dashboard_stats"
// with a 5-minute TTL to avoid running count queries on every page load.
// On cache miss, executes all count queries, serialises the result, and stores it.
func (h *DashboardHandler) Get(w http.ResponseWriter, r *http.Request)
```

### `users.go`

```go
type UsersHandler struct {
    moderationSvc    *service.ModerationService
    registrationSvc  *service.RegistrationService
    store            store.Store
    templates        *Templates
    logger           *slog.Logger
}

// List handles GET /admin/users.
// Query params: q (search), page (default 1).
// Full page on normal request; users_table partial on HTMX request.
func (h *UsersHandler) List(w http.ResponseWriter, r *http.Request)

// Detail handles GET /admin/users/{id}.
// Loads user + account, recent statuses, reports targeting the account,
// and moderation history from admin_actions.
func (h *UsersHandler) Detail(w http.ResponseWriter, r *http.Request)

// Suspend handles POST /admin/users/{id}/suspend.
// Form field: comment.
// Calls ModerationService.SuspendAccount, renders updated user_actions partial.
func (h *UsersHandler) Suspend(w http.ResponseWriter, r *http.Request)

// Unsuspend handles POST /admin/users/{id}/unsuspend.
// Calls ModerationService.UnsuspendAccount, renders updated user_actions partial.
func (h *UsersHandler) Unsuspend(w http.ResponseWriter, r *http.Request)

// Silence handles POST /admin/users/{id}/silence.
// Form field: comment.
// Calls ModerationService.SilenceAccount, renders updated user_actions partial.
func (h *UsersHandler) Silence(w http.ResponseWriter, r *http.Request)

// Unsilence handles POST /admin/users/{id}/unsilence.
// Calls ModerationService.UnsilenceAccount, renders updated user_actions partial.
func (h *UsersHandler) Unsilence(w http.ResponseWriter, r *http.Request)

// SetRole handles POST /admin/users/{id}/set-role. Admin-only.
// Form field: role ("user"|"moderator"|"admin").
// Calls ModerationService.SetRole, renders updated user_actions partial.
func (h *UsersHandler) SetRole(w http.ResponseWriter, r *http.Request)

// Delete handles DELETE /admin/users/{id}. Admin-only.
// Calls ModerationService.DeleteAccount, redirects to /admin/users.
func (h *UsersHandler) Delete(w http.ResponseWriter, r *http.Request)
```

### `registrations.go`

```go
type RegistrationsHandler struct {
    registrationSvc *service.RegistrationService
    store           store.Store
    templates       *Templates
    logger          *slog.Logger
}

// List handles GET /admin/registrations.
// Query params: page (default 1).
// Shows pending registrations (users with confirmed_at IS NULL).
func (h *RegistrationsHandler) List(w http.ResponseWriter, r *http.Request)

// Approve handles POST /admin/registrations/{id}/approve.
// Calls RegistrationService.ApproveRegistration.
// On success: removes the row from the table via HTMX swap (empty response
// with HX-Trigger to update pending count in sidebar).
func (h *RegistrationsHandler) Approve(w http.ResponseWriter, r *http.Request)

// Reject handles POST /admin/registrations/{id}/reject.
// Form field: reason (sent in rejection email).
// Calls RegistrationService.RejectRegistration.
// Same HTMX swap pattern as Approve.
func (h *RegistrationsHandler) Reject(w http.ResponseWriter, r *http.Request)
```

### `invites.go`

```go
type InvitesHandler struct {
    registrationSvc *service.RegistrationService
    store           store.Store
    templates       *Templates
    logger          *slog.Logger
}

// List handles GET /admin/invites.
// Query params: page (default 1).
// Shows all invite codes with usage stats and creator info.
func (h *InvitesHandler) List(w http.ResponseWriter, r *http.Request)

// Create handles POST /admin/invites.
// Form fields: max_uses (optional int), expires_in (optional, e.g. "7d", "30d").
// Calls RegistrationService.CreateInvite, renders updated invites_table partial.
func (h *InvitesHandler) Create(w http.ResponseWriter, r *http.Request)

// Revoke handles DELETE /admin/invites/{id}.
// Deletes the invite code. Renders updated invites_table partial.
func (h *InvitesHandler) Revoke(w http.ResponseWriter, r *http.Request)
```

### `reports.go`

```go
type ReportsHandler struct {
    moderationSvc *service.ModerationService
    store         store.Store
    templates     *Templates
    logger        *slog.Logger
}

// List handles GET /admin/reports.
// Query params: state ("open"|"resolved"|""), category ("spam"|"illegal"|"violation"|"other"|""), page.
// Full page on normal request; reports_table partial on HTMX request.
func (h *ReportsHandler) List(w http.ResponseWriter, r *http.Request)

// Detail handles GET /admin/reports/{id}.
// Loads report with reporter account, target account, reported statuses,
// and moderation history.
func (h *ReportsHandler) Detail(w http.ResponseWriter, r *http.Request)

// Assign handles POST /admin/reports/{id}/assign.
// Form field: moderator_id.
// Calls ModerationService.AssignReport, renders updated report_actions partial.
func (h *ReportsHandler) Assign(w http.ResponseWriter, r *http.Request)

// Resolve handles POST /admin/reports/{id}/resolve.
// Form fields: action ("warn"|"silence"|"suspend"|"dismiss"), comment.
// Calls ModerationService.ResolveReport (which may trigger account actions),
// renders updated report_actions partial.
func (h *ReportsHandler) Resolve(w http.ResponseWriter, r *http.Request)
```

### `federation.go`

```go
type FederationHandler struct {
    moderationSvc *service.ModerationService
    store         store.Store
    templates     *Templates
    logger        *slog.Logger
}

// List handles GET /admin/federation.
// Query params: tab ("domain-blocks"|"instances"), page.
// Renders both domain block list and known instances list.
// HTMX tab switching renders the appropriate partial.
func (h *FederationHandler) List(w http.ResponseWriter, r *http.Request)

// CreateBlock handles POST /admin/federation/domain-blocks. Admin-only.
// Form fields: domain, severity ("silence"|"suspend"), reason.
// Calls ModerationService.BlockDomain, renders updated domain_blocks_table partial.
func (h *FederationHandler) CreateBlock(w http.ResponseWriter, r *http.Request)

// RemoveBlock handles DELETE /admin/federation/domain-blocks/{domain}. Admin-only.
// Calls ModerationService.UnblockDomain, renders updated domain_blocks_table partial.
func (h *FederationHandler) RemoveBlock(w http.ResponseWriter, r *http.Request)
```

### `content.go`

```go
type ContentHandler struct {
    store      store.Store
    mediaStore media.Store
    templates  *Templates
    logger     *slog.Logger
}

// ListEmojis handles GET /admin/content/emojis.
// Shows all custom emojis (local + remote, enabled + disabled).
func (h *ContentHandler) ListEmojis(w http.ResponseWriter, r *http.Request)

// UploadEmoji handles POST /admin/content/emojis.
// Multipart form: shortcode (text), image (file).
// Validates: shortcode format ([a-z0-9_]+, 2–32 chars), file type (PNG/GIF, max 50KB),
// image dimensions (max 128×128).
// Stores the image via MediaStore, creates the custom_emojis row.
// Renders updated emojis_grid partial.
func (h *ContentHandler) UploadEmoji(w http.ResponseWriter, r *http.Request)

// DeleteEmoji handles DELETE /admin/content/emojis/{shortcode}.
// Deletes the emoji row and the media file from MediaStore.
// Renders updated emojis_grid partial.
func (h *ContentHandler) DeleteEmoji(w http.ResponseWriter, r *http.Request)

// ListFilters handles GET /admin/content/filters.
// Shows all server-side content filters.
func (h *ContentHandler) ListFilters(w http.ResponseWriter, r *http.Request)

// CreateFilter handles POST /admin/content/filters.
// Form fields: phrase, scope ("public_timeline"|"all"), whole_word (checkbox), action ("warn"|"hide").
// Creates the server_filters row, invalidates the in-process filter cache.
// Renders updated filters_table partial.
func (h *ContentHandler) CreateFilter(w http.ResponseWriter, r *http.Request)

// DeleteFilter handles DELETE /admin/content/filters/{id}.
// Deletes the filter row, invalidates the in-process filter cache.
// Renders updated filters_table partial.
func (h *ContentHandler) DeleteFilter(w http.ResponseWriter, r *http.Request)
```

### `settings.go`

```go
type SettingsHandler struct {
    instanceSvc *service.InstanceService
    templates   *Templates
    logger      *slog.Logger
}

// Get handles GET /admin/settings. Admin-only.
// Loads all instance settings and renders the settings form.
func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request)

// Update handles POST /admin/settings. Admin-only.
// Form fields match the instance_settings keys:
//   instance_name, instance_description, registration_mode,
//   contact_email, max_status_chars, media_max_bytes, rules_text.
// Calls InstanceService.Set for each changed field.
// Invalidates the "instance:settings" cache key.
// Re-renders the settings form with a success flash message.
func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request)
```

---

## 8. `internal/service/moderation_service.go` — Moderation Logic

### Constructor and Dependencies

```go
type ModerationService struct {
    store  store.Store
    cache  cache.Store
    email  *EmailService
    outbox *ap.OutboxPublisher
    logger *slog.Logger
}

func NewModerationService(
    s store.Store,
    c cache.Store,
    email *EmailService,
    outbox *ap.OutboxPublisher,
    logger *slog.Logger,
) *ModerationService
```

### SuspendAccount

```go
// SuspendAccount suspends a local account.
//
// Steps:
//   1. Load the target account and user.
//   2. Guard: cannot suspend another admin (only admins can be suspended by
//      direct DB intervention — prevents moderator lock-out scenarios).
//   3. Set accounts.suspended = TRUE.
//   4. Revoke all OAuth access tokens for the account (forces client logout).
//   5. Send AP Delete{Person} to all remote followers. Remote instances treat
//      this as an account removal and clean up local copies of the actor's
//      content. This matches Mastodon's suspension behaviour.
//   6. Send moderation email to the user (action: "suspension").
//   7. Create admin_actions audit entry.
//
// AP side effects:
//   Delete{Person} is used (not per-status Delete{Note}) because:
//   - A suspended account may have thousands of statuses; enqueuing one
//     Delete per status would flood the federation delivery queue.
//   - Remote Mastodon instances handle Delete{Person} by purging all content
//     from that actor, achieving the same result in a single activity.
//   - On unsuspend, the actor re-appears but severed follows are NOT restored
//     (consistent with Mastodon behaviour).
func (s *ModerationService) SuspendAccount(ctx context.Context, targetAccountID, moderatorID, comment string) error
```

### UnsuspendAccount

```go
// UnsuspendAccount reverses a suspension.
//
// Steps:
//   1. Set accounts.suspended = FALSE.
//   2. Create admin_actions audit entry.
//
// No AP activity is sent. Remote instances that processed the Delete{Person}
// have already purged the actor. The account becomes reachable again, but
// remote follows must be re-established manually. This is consistent with
// Mastodon's behaviour — suspension is a severe action with lasting
// federation consequences.
//
// No email is sent (the user can now log in and see their account is restored).
func (s *ModerationService) UnsuspendAccount(ctx context.Context, targetAccountID, moderatorID string) error
```

### SilenceAccount

```go
// SilenceAccount marks an account as silenced.
//
// Steps:
//   1. Set accounts.silenced = TRUE.
//   2. Send moderation email to the user (action: "silence").
//   3. Create admin_actions audit entry.
//
// No AP side effects. Silencing is a local-only action: the account's posts
// are hidden from public/federated timelines on this instance, but federation
// continues normally. Remote instances are unaware of the silence.
func (s *ModerationService) SilenceAccount(ctx context.Context, targetAccountID, moderatorID, comment string) error
```

### UnsilenceAccount

```go
// UnsilenceAccount reverses a silence.
//
// Steps:
//   1. Set accounts.silenced = FALSE.
//   2. Create admin_actions audit entry.
func (s *ModerationService) UnsilenceAccount(ctx context.Context, targetAccountID, moderatorID string) error
```

### WarnAccount

```go
// WarnAccount sends a warning email without modifying account state.
//
// Steps:
//   1. Send moderation email to the user (action: "warning").
//   2. Create admin_actions audit entry with the warning reason.
func (s *ModerationService) WarnAccount(ctx context.Context, targetAccountID, moderatorID, reason string) error
```

### SetRole

```go
// SetRole changes a user's role.
//
// Steps:
//   1. Load the target user.
//   2. Guard: cannot change own role (prevents accidental self-demotion).
//   3. Guard: cannot demote the last admin (ensures at least one admin always exists).
//   4. Update users.role.
//   5. Create admin_actions audit entry with old_role and new_role in metadata.
func (s *ModerationService) SetRole(ctx context.Context, targetUserID, moderatorID, newRole string) error
```

### DeleteAccount

```go
// DeleteAccount hard-deletes a local account and all associated data.
// This is irreversible.
//
// Steps:
//   1. Load the target account, user, and all media attachment storage keys.
//   2. Guard: cannot delete another admin.
//   3. Send AP Delete{Person} to all remote followers (while keys still exist
//      for HTTP Signature signing).
//   4. Create admin_actions audit entry with username/email in metadata
//      (the audit entry survives the deletion via ON DELETE SET NULL on the FK).
//   5. Begin database transaction:
//      a. DELETE FROM favourites WHERE account_id = ?
//      b. DELETE FROM notifications WHERE account_id = ? OR from_id = ?
//      c. DELETE FROM status_hashtags WHERE status_id IN (SELECT id FROM statuses WHERE account_id = ?)
//      d. DELETE FROM media_attachments WHERE account_id = ?
//      e. DELETE FROM statuses WHERE account_id = ?
//      f. DELETE FROM follows WHERE account_id = ? OR target_id = ?
//      g. DELETE FROM mutes WHERE account_id = ? OR target_id = ?
//      h. DELETE FROM blocks WHERE account_id = ? OR target_id = ?
//      i. DELETE FROM reports WHERE account_id = ?  (reports filed BY this user)
//      j. DELETE FROM oauth_access_tokens WHERE account_id = ?
//      k. DELETE FROM users WHERE account_id = ?
//      l. DELETE FROM accounts WHERE id = ?
//   6. After successful commit: delete media files from MediaStore.
//      Media deletion is fire-and-forget with error logging — if some files
//      fail to delete, they become orphans (cleaned up by a future reaper).
//
// Deletion order satisfies FK constraints: children are deleted before parents.
// Reports AGAINST this user (target_id = ?) are preserved with the account
// reference nullified — moderators can still review the report history.
func (s *ModerationService) DeleteAccount(ctx context.Context, targetAccountID, moderatorID string) error
```

### BlockDomain

```go
// BlockDomain creates or updates a domain block.
//
// Steps:
//   1. Upsert into domain_blocks (id, domain, severity, reason).
//      If the domain is already blocked, update severity and reason.
//   2. Invalidate the domain block cache key ("domain_blocks:set").
//   3. If severity is "suspend": sever all existing follows between local
//      accounts and accounts on the blocked domain.
//      - DELETE FROM follows WHERE account_id IN (local accounts) AND target_id IN (accounts on domain)
//      - DELETE FROM follows WHERE target_id IN (local accounts) AND account_id IN (accounts on domain)
//      This is done in a single transaction with the domain block creation.
//   4. Create admin_actions audit entry with domain and severity in metadata.
func (s *ModerationService) BlockDomain(ctx context.Context, domain, severity, reason, moderatorID string) error
```

### UnblockDomain

```go
// UnblockDomain removes a domain block.
//
// Steps:
//   1. Delete from domain_blocks WHERE domain = ?.
//   2. Invalidate the domain block cache key.
//   3. Create admin_actions audit entry.
//
// Severed follows are NOT restored (same as account unsuspension — federation
// consequences of a suspend-level block are permanent).
func (s *ModerationService) UnblockDomain(ctx context.Context, domain, moderatorID string) error
```

### ResolveReport

```go
// ResolveReport resolves a report with an action.
//
// Steps:
//   1. Load the report.
//   2. Guard: report must be in state "open".
//   3. Perform the action on the target account:
//      - "dismiss": no account action (report was unfounded).
//      - "warn": call WarnAccount.
//      - "silence": call SilenceAccount.
//      - "suspend": call SuspendAccount.
//   4. Update reports: state = 'resolved', action_taken = action, resolved_at = NOW().
//   5. Create admin_actions audit entry with report_id and resolution in metadata.
func (s *ModerationService) ResolveReport(ctx context.Context, reportID, moderatorID, action, comment string) error
```

### AssignReport

```go
// AssignReport assigns a report to a moderator for review.
//
// Steps:
//   1. Update reports: assigned_to_id = moderatorID.
//   2. Create admin_actions audit entry with report_id in metadata.
func (s *ModerationService) AssignReport(ctx context.Context, reportID, moderatorID string) error
```

### ListUsers (admin search)

```go
// ListUsers returns a paginated, searchable list of local users for the admin portal.
// The query parameter is matched against username and email via ILIKE.
// Returns users with their associated account data (suspended, silenced flags).
func (s *ModerationService) ListUsers(ctx context.Context, query string, page, limit int) ([]db.SearchLocalUsersAdminRow, int64, error)
```

### AP Outbox Addition

The `OutboxPublisher` (ADR 07, §4) needs one new method:

```go
// DeleteActor creates a Delete{Person} activity and delivers it to all
// remote followers. Used for account suspension and hard deletion.
//
// The activity JSON:
//   {
//     "@context": "https://www.w3.org/ns/activitystreams",
//     "id": "{activityID}",
//     "type": "Delete",
//     "actor": "{account.ap_id}",
//     "to": ["https://www.w3.org/ns/activitystreams#Public"],
//     "object": "{account.ap_id}"
//   }
//
// Delivered via fanOutToFollowers — one NATS message per unique remote
// follower inbox URL.
func (p *OutboxPublisher) DeleteActor(ctx context.Context, account *store.Account) error
```

---

## 9. `internal/service/registration_service.go` — Registration & Invites

### Constructor and Dependencies

```go
type RegistrationService struct {
    store  store.Store
    cache  cache.Store
    email  *EmailService
    cfg    *config.Config
    logger *slog.Logger
}

func NewRegistrationService(
    s store.Store,
    c cache.Store,
    email *EmailService,
    cfg *config.Config,
    logger *slog.Logger,
) *RegistrationService
```

### Register

```go
type RegisterRequest struct {
    Username   string
    Email      string
    Password   string
    InviteCode string // required in "invite" mode
    Reason     string // optional in "approval" mode (shown to admin)
}

// Register creates a new local account and user.
//
// The registration mode is read from instance settings (cached).
// Both modes share the same validation and account creation steps;
// they differ only in confirmation behaviour.
//
// Shared steps (both modes):
//   1. Read registration_mode from InstanceService (cache-backed).
//   2. If "invite" mode: validate invite code via ValidateInvite.
//      Reject with a clear error if the code is invalid, expired, or exhausted.
//   3. Validate username:
//      - 3–30 characters.
//      - Pattern: [a-z0-9_]+ (lowercase only; input is lowercased before validation).
//      - Not already taken: query accounts WHERE username = ? AND domain IS NULL.
//   4. Validate email:
//      - Well-formed (contains @ with a domain part).
//      - Not already registered: query users WHERE email = ?.
//   5. Validate password:
//      - Minimum 8 characters.
//   6. Hash password with bcrypt (cost 12).
//   7. Generate RSA-2048 key pair for the AP Actor.
//      Synchronous — ~10ms, acceptable latency for a registration request.
//   8. Begin database transaction:
//      a. Create accounts row:
//         - id = uid.New()
//         - username, domain = NULL (local)
//         - public_key, private_key (PEM-encoded)
//         - AP URLs constructed from config.InstanceDomain:
//           inbox_url:    https://{domain}/users/{username}/inbox
//           outbox_url:   https://{domain}/users/{username}/outbox
//           followers_url: https://{domain}/users/{username}/followers
//           following_url: https://{domain}/users/{username}/following
//           ap_id:        https://{domain}/users/{username}
//      b. Create users row:
//         - id = uid.New()
//         - account_id, email, password_hash, role = "user"
//         - confirmed_at: depends on mode (see below)
//
// Approval mode:
//   - confirmed_at = NULL (user cannot log in until admin approves).
//   - No email sent at this point. Admin sees the pending registration.
//   - The optional "reason" field is stored in the user's metadata
//     (or a separate registration_reason column — see note below).
//
// Invite mode:
//   - confirmed_at = NOW() (user can log in immediately).
//   - Increment invites.uses within the same transaction.
//   - Send welcome email (not verification — already confirmed).
//
// Returns the created User on success.
func (s *RegistrationService) Register(ctx context.Context, req RegisterRequest) (*store.User, error)
```

**Note on registration reason:** The `RegisterRequest.Reason` field (used in approval mode to explain why the user wants to join) needs storage. Options:

1. Add `registration_reason TEXT` column to `users`. Clean and queryable.
2. Store in a JSONB `metadata` column on `users`. More flexible, less queryable.

**Decision:** Add `registration_reason TEXT` to the `users` table (amend migration 000004). This is a simple, nullable column that the admin registrations view queries directly. No JSONB complexity needed for a single field. Length is enforced at the API layer: max 500 characters (enough for a short paragraph explaining why the user wants to join).

### ApproveRegistration

```go
// ApproveRegistration confirms a pending user (admin action).
//
// Steps:
//   1. Load the user. Guard: confirmed_at must be NULL.
//   2. Set users.confirmed_at = NOW().
//   3. Send "account approved" email to the user.
//      This is a notification email (not a click-to-verify flow).
//      The user can now log in immediately.
//   4. Create admin_actions audit entry (action: "approve_registration").
func (s *RegistrationService) ApproveRegistration(ctx context.Context, userID, moderatorID string) error
```

### RejectRegistration

```go
// RejectRegistration rejects a pending registration (admin action).
//
// Steps:
//   1. Load the user. Guard: confirmed_at must be NULL.
//   2. Send rejection email with the admin-provided reason.
//   3. Create admin_actions audit entry (action: "reject_registration",
//      metadata: {"email_reason": "..."}).
//   4. Hard-delete the user and account records (the registration was never
//      confirmed, so there are no statuses, follows, or other data to clean up).
//      This frees the username and email for re-registration.
func (s *RegistrationService) RejectRegistration(ctx context.Context, userID, moderatorID, reason string) error
```

### CreateInvite

```go
// CreateInvite generates a new invite code.
//
// Steps:
//   1. Generate a URL-safe invite code: 16 bytes from crypto/rand, base32-encoded
//      (Crockford alphabet, 26 characters). Human-friendly for copy-paste.
//   2. Compute expires_at from the optional duration (nil = never expires).
//   3. Create invites row.
//   4. Return the Invite (including the plaintext code for display to the admin).
func (s *RegistrationService) CreateInvite(ctx context.Context, createdByUserID string, maxUses *int, expiresIn *time.Duration) (*store.Invite, error)
```

### ValidateInvite

```go
// ValidateInvite checks if an invite code is currently valid.
//
// A code is valid if:
//   - It exists in the invites table.
//   - expires_at is NULL or in the future.
//   - max_uses is NULL or uses < max_uses.
//
// Returns the Invite on success, or an error describing why the code is invalid.
func (s *RegistrationService) ValidateInvite(ctx context.Context, code string) (*store.Invite, error)
```

### Amendment to `users` table (migration 000004)

Add `registration_reason` for approval-mode registrations:

```sql
-- Add to CREATE TABLE users:
    registration_reason TEXT,  -- optional: user-provided reason for wanting to join (approval mode)
```

---

## 10. `internal/service/instance_service.go` — Instance Settings

### Constructor and Dependencies

```go
type InstanceService struct {
    store  store.Store
    cache  cache.Store
    logger *slog.Logger
}

func NewInstanceService(
    s store.Store,
    c cache.Store,
    logger *slog.Logger,
) *InstanceService
```

### Cache Strategy

- **Cache key:** `instance:settings`
- **Value:** JSON-serialized `map[string]string` containing all rows from `instance_settings`.
- **TTL:** 5 minutes.
- **Invalidation:** On any write via `Set`, the cache key is deleted immediately. The next read repopulates it from the database.

This is a simple read-through cache: `GetAll` checks the cache first, deserialises and returns if present, otherwise queries the database and populates the cache. Individual `Get` calls use `GetAll` internally and extract the requested key from the map — this avoids per-key cache entries while keeping the hot path (reading a single setting during request processing) to a single cache lookup.

### GetAll

```go
// GetAll returns all instance settings as a map.
//
// Flow:
//   1. Check cache for "instance:settings".
//   2. On hit: deserialise JSON map, return.
//   3. On miss: query store.ListSettings, build map, serialise to JSON,
//      store in cache with 5-minute TTL, return.
func (s *InstanceService) GetAll(ctx context.Context) (map[string]string, error)
```

### Get

```go
// Get returns a single setting value by key.
// Calls GetAll internally and extracts the value. Returns empty string
// if the key does not exist (not an error — callers use defaults).
func (s *InstanceService) Get(ctx context.Context, key string) (string, error)
```

### Set

```go
// Set updates a single instance setting.
//
// Steps:
//   1. Validate the key is a known setting name (reject unknown keys to
//      prevent arbitrary data insertion).
//   2. Upsert via store.SetSetting (INSERT ... ON CONFLICT DO UPDATE).
//   3. Delete the "instance:settings" cache key.
//
// Known keys: instance_name, instance_description, registration_mode,
// contact_email, max_status_chars, media_max_bytes, rules_text.
func (s *InstanceService) Set(ctx context.Context, key, value string) error
```

### SetMany

```go
// SetMany updates multiple settings in a single call.
// Used by the admin settings form (POST /admin/settings).
//
// Steps:
//   1. Validate all keys.
//   2. For each key-value pair, call store.SetSetting.
//   3. Delete the "instance:settings" cache key once (not per-key).
func (s *InstanceService) SetMany(ctx context.Context, settings map[string]string) error
```

### GetInstanceInfo

```go
// GetInstanceInfo returns the structured instance metadata for the
// Mastodon REST API (GET /api/v2/instance).
//
// Combines cached instance settings with computed values:
//   - title, description, contact from settings
//   - version: hardcoded Monstera-fed version string
//   - registrations: { enabled, approval_required } derived from registration_mode
//   - stats: { user_count, status_count, domain_count } from count queries
//   - rules: parsed from the rules_text setting
//
// The stats sub-object uses the same cached dashboard counts (cache key:
// "admin:dashboard_stats", 5-minute TTL) to avoid redundant count queries.
func (s *InstanceService) GetInstanceInfo(ctx context.Context) (*InstanceInfo, error)

// InstanceInfo is the response shape for GET /api/v2/instance.
// Matches the Mastodon API v2 instance format.
type InstanceInfo struct {
    Domain        string          `json:"domain"`
    Title         string          `json:"title"`
    Version       string          `json:"version"`
    SourceURL     string          `json:"source_url"`
    Description   string          `json:"description"`
    Registrations RegistrationInfo `json:"registrations"`
    Contact       ContactInfo     `json:"contact"`
    Rules         []Rule          `json:"rules"`
    Usage         UsageInfo       `json:"usage"`
}

type RegistrationInfo struct {
    Enabled          bool `json:"enabled"`
    ApprovalRequired bool `json:"approval_required"`
    Message          *string `json:"message"`
}

type ContactInfo struct {
    Email   string `json:"email"`
    Account *any   `json:"account"` // admin account, nullable
}

type Rule struct {
    ID   string `json:"id"`
    Text string `json:"text"`
}

type UsageInfo struct {
    Users UsageUsers `json:"users"`
}

type UsageUsers struct {
    ActiveMonth int64 `json:"active_month"` // 0 in Phase 1 (see future feature #6)
}
```

---

## 11. Additional Email Templates

ADR 05 defined four email templates: `email_verification`, `password_reset`, `invite`, and `moderation_action`. The admin portal and registration flows in this design require two additional templates.

### `account_approved.html` / `account_approved.txt`

Sent when an admin approves a pending registration (approval mode).

```go
type AccountApprovedData struct {
    InstanceName string
    Username     string
    LoginURL     string // https://{INSTANCE_DOMAIN}
}
```

**Subject:** `Welcome to {InstanceName}, @{Username}!`

**Content:** Notifies the user that their registration has been approved and they can now log in using any Mastodon-compatible client. Includes the instance URL.

### `registration_rejected.html` / `registration_rejected.txt`

Sent when an admin rejects a pending registration.

```go
type RegistrationRejectedData struct {
    InstanceName string
    Username     string
    Reason       string // admin-provided rejection reason
}
```

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

```go
func (s *EmailService) SendAccountApproved(ctx context.Context, to, username string) error

func (s *EmailService) SendRegistrationRejected(ctx context.Context, to, username, reason string) error
```

---

## 12. Domain Block Follow Severance Query

When a domain block with severity `suspend` is created, all follows between local accounts and accounts on the blocked domain must be severed. This requires a query not yet in the existing `follows.sql`:

```sql
-- name: DeleteFollowsByDomain :exec
-- Severs all follows in both directions between local accounts and accounts on the given domain.
-- Used when creating a domain block with severity "suspend".
DELETE FROM follows
WHERE id IN (
    SELECT f.id FROM follows f
    INNER JOIN accounts a ON (a.id = f.account_id OR a.id = f.target_id)
    WHERE a.domain = $1
);
```

Add to `FollowStore` interface:

```go
    DeleteFollowsByDomain(ctx context.Context, domain string) error
```

---

## 13. Amendments to Prior ADRs

This design output introduces changes to prior outputs. All amendments are documented here for traceability.

### ADR 01 — Project Foundation

| Section | Change |
|---------|--------|
| Design Decisions: "Admin portal frontend" | React + Vite → **HTMX + Go templates + Pico.css** |
| §8 "Admin Portal: Go Embed Setup" | Replaced by §3 of this output (template embed + static file embed) |
| Makefile `build-admin` target | **Removed** — no Node.js build step |
| Multi-stage Dockerfile | **Simplified** — 2 stages (Go builder → distroless), Node.js stage removed |
| `web/admin/` directory structure | SPA source tree → templates/ + static/ (see §3 file structure) |
| `RouterDeps` | Add `ModerationSvc`, `RegistrationSvc`, `InstanceSvc` (already partially listed) |
| Open Question #3 (invite generation) | **Resolved**: Phase 1 invites are admin/moderator-only via the admin portal. Regular user invite generation deferred. |

### ADR 02 — Data Model & Database Layer

| Section | Change |
|---------|--------|
| Migration 000002 (`media_attachments`) | Add `size_bytes BIGINT NOT NULL DEFAULT 0` |
| Migration 000004 (`users`) | Add `registration_reason TEXT` |
| Migration 000021 (`server_filters`) | Add `whole_word BOOLEAN NOT NULL DEFAULT FALSE` |
| Migration list | Add 000030 (`admin_actions`), 000031 (`known_instances`), 000032 (`custom_emojis`) |
| §7 Store Interface | Add `AdminActionStore`, `KnownInstanceStore`, `CustomEmojiStore` to root `Store`; add methods to `UserStore`, `InviteStore`, `ReportStore`, `StatusStore`, `AttachmentStore`, `FollowStore` |
| Type aliases | Add `AdminAction`, `KnownInstance`, `CustomEmoji` |

### ADR 05 — Email Abstraction

| Section | Change |
|---------|--------|
| Template list | Add `account_approved.html/.txt`, `registration_rejected.html/.txt` |
| `EmailService` | Add `SendAccountApproved`, `SendRegistrationRejected` methods |
| Template data structs | Add `AccountApprovedData`, `RegistrationRejectedData` |

### ADR 07 — ActivityPub & Federation

| Section | Change |
|---------|--------|
| §4 OutboxPublisher | Add `DeleteActor(ctx, account)` method — sends `Delete{Person}` to all remote followers |

---

## 14. Open Questions

| # | Question | Impact |
|---|----------|--------|
| ~~1~~ | ~~**Admin login rate limiting**~~ — resolved: **add in-app per-IP rate limiting**. Cache key `admin_login_attempts:{ip}` with value = failure count, 15-minute TTL. After 5 failures, return 429 with `Retry-After` header. This closes the gap for self-hosters without a gateway. | N/A |
| 2 | **Custom emoji ingestion from federation**: The `custom_emojis` table supports remote emojis (`domain IS NOT NULL`). When should these be created? Mastodon copies remote custom emojis from incoming `Create{Note}` activities (the emoji data is in the Note's `tag` array as `Emoji` objects). Should the inbox processor create `custom_emojis` rows for remote emojis on ingest, or defer this to Phase 2? | Medium — affects whether remote emojis render correctly in clients. Without ingest, remote emoji shortcodes display as `:shortcode:` text. |
| ~~3~~ | ~~**Admin session invalidation on role change**~~ — resolved: **immediate invalidation via reverse index**. On login, store an additional cache key `admin_sessions_by_user:{userID}` containing the set of active session IDs. On role change, look up and delete all sessions for that user. One extra cache write per login; enables expected behavior when demoting a moderator. | N/A |
| 4 | **Dashboard stats staleness indicator**: The dashboard stats are cached for 5 minutes. Should the UI display a "last updated" timestamp so admins know they're looking at potentially stale numbers? | Low — UX polish, not architectural. |

---

*End of ADR 10 — Admin Portal & Content Moderation*
