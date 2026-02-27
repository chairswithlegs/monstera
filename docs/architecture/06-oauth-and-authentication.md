# OAuth and authentication

This document describes the desired OAuth 2.0 (Authorization Code + PKCE), HTTP Signatures, and auth middleware. Build order is in [roadmap.md](../roadmap.md).

---

## Design decisions (answered before authoring)

| Question | Decision |
|----------|----------|
| PKCE `plain` method | **Rejected** — only `S256` is accepted. `plain` provides no security benefit and no major Mastodon client uses it. Return `invalid_request` if `code_challenge_method` is anything other than `S256`. |
| Access token format | **Opaque** — 32 bytes `crypto/rand`, hex-encoded (64 chars). Not JWT. Mastodon clients cache raw token strings and expect them to work indefinitely. |
| Token expiry | **Non-expiring by default** (Mastodon convention). `expires_at` is NULL. Tokens are revoked explicitly via `POST /oauth/revoke` or by admin suspension. Token cache TTL (24h) provides periodic revalidation against the DB. |
| App-only token scopes | `client_credentials` grant yields a token with `account_id = NULL` and scopes limited to `read` (no `write`, no `admin:*`). Used by clients that need instance metadata before user login. |
| Authorization code format | 32 bytes `crypto/rand`, hex-encoded (64 chars). Stored raw in DB (short-lived; no hash needed). |
| Authorization code TTL | **10 minutes**, single-use — row deleted on exchange. |
| `state` parameter | Passed through; not stored server-side. Clients validate it themselves (CSRF protection on the client side). |
| Login session for authorize flow | **Short-lived signed cookie** — HMAC-SHA256 with `SECRET_KEY_BASE`, 10-minute expiry. Only lives between form submission and redirect. |
| Consent screen | **Implicit consent** — after login, the code is issued immediately. No separate "Allow/Deny" step. Mastodon clients expect this behavior; the user authorized the app by entering credentials. |
| HTTP Signature algorithm | `rsa-sha256` (draft-cavage-http-signatures-12) — Mastodon's de facto standard. |
| HTTP Signature replay prevention | Cache key `httpsig:{sha256(keyId+date+requestTarget)}`, TTL 60s. |
| HTTP Signature clock skew | **±30 seconds** — reject requests with `Date` header outside this window. |

---

## Replay Prevention Design

### Problem

An attacker who captures a valid signed HTTP request (e.g., by compromising a network hop) could re-send it to the target inbox. The signature is still valid because the same key and headers are used.

### Solution

After a signature is verified, the triple `(keyId, Date, requestTarget)` is stored in the cache with a TTL of **60 seconds**. Any subsequent request with the same triple is rejected as a replay.

### Cache key format

```
httpsig:{sha256_hex_16(keyId + ":" + date + ":" + requestTarget)}
```

Example:
```
httpsig:a7f3c9e2b1d04f83
```

The SHA-256 truncated to 16 bytes (32 hex chars) keeps keys fixed-length while providing sufficient collision resistance for this use case (the TTL is only 60 seconds; the total keyspace during any TTL window is small).

### TTL justification

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| Clock skew tolerance | ±30 seconds | Accounts for NTP drift between federated servers. Mastodon uses ±30s. |
| Replay cache TTL | 60 seconds | Must be ≥ 2× clock skew (30s) to cover the full window. At 60s, a request timestamped 29s in the future (just within tolerance) can be replayed up to 29s after the server receives it; the 60s TTL covers this. |
| Public key cache TTL | 1 hour | Remote actor profiles change rarely. Key rotation is handled passively: a cache miss triggers a fresh fetch. |

### Multi-replica correctness

When `CACHE_DRIVER=redis`, the replay set is shared across all replicas — a replayed request sent to a different pod is still caught. With `CACHE_DRIVER=memory`, each replica maintains an independent replay set. This means a replay could succeed if routed to a different replica within the 60s window. This is an accepted limitation of the memory driver (it is documented as dev-only); production deployments should use `CACHE_DRIVER=redis`.

---
