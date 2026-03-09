# OAuth & Authentication

Design doc: `docs/architecture/04-authentication-authorization.md`

## Conventions

- Authorization Code flow with PKCE (S256). No implicit grant.
- Tokens are non-expiring in Phase 1 (matches Mastodon client expectations). Revocation on password change or logout.
- Scope system: `read`, `write`, `follow`, `push`, `admin:read`, `admin:write`. Parsed via `ScopeSet`.
- Password hashing: `golang.org/x/crypto/bcrypt`.
- `SECRET_KEY_BASE` → HKDF sub-keys with purpose strings (`monstera-csrf`, `monstera-email-token`, `monstera-actor-private-key`, `monstera-invite-token`).
- CSRF protection on the admin login form via HMAC token.
