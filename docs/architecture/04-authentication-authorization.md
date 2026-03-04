# Authentication and authorization

This document describes how authentication and authorization work for the Go API and for the Next.js UI.

## OAuth 2.0 (Mastodon API clients)

Mastodon clients use OAuth 2.0 to obtain an access token, then send it on every request as `Authorization: Bearer <token>`.

### Flows

- **Authorization Code with PKCE**: Primary flow. Client gets an authorization code from `GET /oauth/authorize` (with `code_challenge`, `code_challenge_method=S256`), then exchanges it at `POST /oauth/token` with `code` and `code_verifier`. Used by mobile and desktop apps.
- **Authorization Code (without PKCE)**: Supported for server-side or legacy clients.

OAuth handlers live in `internal/api/oauth/`; server logic (token issue, validation, PKCE) in `internal/oauth/` (e.g. `server.go`, `pkce`).

### Token storage and lookup

Access tokens are stored in the `oauth_access_tokens` table. Lookup is done via the OAuth server’s `LookupToken`; a short-TTL cache (keyed by token hash) avoids hitting the database on every request. Tokens can have an optional `expires_at`; non-expiring is the default (see roadmap for configurable expiry).

### Scopes

Scopes restrict what an access token can do. Examples: `read`, `read:accounts`, `read:statuses`, `read:notifications`, `write`, `write:statuses`, `write:media`, `write:follows`, `write:blocks`, `write:mutes`, etc. Handlers that need a specific scope use `middleware.RequiredScopes("scope:name")` on the route.

## API middleware (Go)

| Middleware | Purpose |
|------------|---------|
| **RequireAuth** | Resolves Bearer token via OAuth server, loads account, puts it in context. Returns 401 if missing or invalid. |
| **OptionalAuth** | Same resolution, but does not return 401 if no token; context may have no account. |
| **RequiredScopes** | Runs after auth; checks token scopes and returns 403 if the required scope is missing. |
| **StreamingTokenFromQuery** | For streaming routes: copies `access_token` query param into `Authorization: Bearer` so EventSource clients (which cannot set custom headers) still get auth. |

Account and user are attached to context; handlers read them via `middleware.AccountFromContext(r.Context())` and, for admin routes, `middleware.UserFromContext`. Role checks are done by:

- **RequireModerator**: User must be in context with role `moderator` or `admin`; else 403.
- **RequireAdmin**: User must be in context with role `admin`; else 403.

Middleware is in `internal/api/middleware/` (auth, streaming_auth, etc.).

## Admin / moderator API (Monstera)

Routes under `/monstera/api/v1/admin/*` require a valid Bearer token (RequireAuth) and then RequireModerator or RequireAdmin. The token is the same OAuth access token as for the Mastodon API; the difference is that the authenticated user must have role `moderator` or `admin`. The Next.js UI uses the same token for both Mastodon API calls (if any) and Monstera admin calls.

## Next.js UI auth

The UI is a separate app that uses OAuth to obtain an access token, then stores and sends it on every request.

- **API client** (`ui/lib/api/client.ts`): `authFetch` adds `Authorization: Bearer <token>` and `Content-Type: application/json`. On 401, it attempts a token refresh (e.g. via refresh endpoint or re-login), then retries once; on failure it triggers logout.
- **Login / register**: Handled by the UI (e.g. `/login`, `/register`). The backend exposes `POST /oauth/login` and OAuth authorize/token endpoints; the UI drives the user through the flow and stores the returned access token.
- **Token storage and refresh**: Implemented in `ui/lib/auth/` (tokens, logout). Refresh behaviour is defined there (e.g. refresh token if supported, or re-prompt login).
