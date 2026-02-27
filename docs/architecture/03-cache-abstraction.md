# Cache abstraction

This document describes the desired cache interface, drivers (memory, Redis), and usage conventions.

---

## Design decisions

| Question | Decision |
|----------|----------|
| `ErrCacheMiss` type | `errors.New` sentinel — callers use `errors.Is` |
| Factory input | `cache.Config` struct (not `*config.Config`) — keeps the cache package dependency-free |
| Ristretto cost unit | `len(value)` bytes — MaxCost = 128 MB per replica |
| Redis zero-TTL behaviour | TTL = 0 means no expiry (Redis default); callers must always pass a non-zero TTL |
| Health check pattern | Optional `Pinger` interface in `cache.go`; both implementations satisfy it |
| Thundering herd mitigation | `singleflight` included in `GetOrSet` — AP actor stampede is a realistic scenario |
| Helper location | `internal/cache/helpers.go` — separate file from the interface definition |

---

## File Layout

```
internal/cache/
├── cache.go        — Store interface, ErrCacheMiss, Config, New factory, Pinger interface
├── helpers.go      — GetJSON, SetJSON, GetOrSet (generic helpers over Store)
├── memory/
│   └── memory.go   — ristretto/v2 implementation
└── redis/
    └── redis.go    — go-redis/v9 implementation
```

---

## Caching conventions

### When to cache

**Cache:** Data that is read frequently, changes infrequently, and can tolerate brief staleness.

| Data | Reason |
|------|--------|
| OAuth access tokens | Every authenticated request performs a token lookup; DB round-trip is wasteful |
| AP actor documents | Remote actor profiles are fetched on every inbound activity signature verification |
| Domain block list | Read on every inbound federation request; changes rarely |
| Instance settings | Read on many request paths; admin changes are infrequent |
| Home timeline pages | Expensive join query; 60s staleness is acceptable |

**Do not cache:** Data where correctness matters more than latency.

| Data | Reason |
|------|--------|
| Account suspension status | A suspended account must be blocked immediately, not after a TTL |
| Individual status content | The content is small and fast to fetch; stale rendered HTML is confusing |
| Follow relationships | Used in timeline queries that are already index-optimised |
| Reports and moderation queue | Admin workflows require real-time accuracy |

---

### Key naming convention

All keys use lowercase words separated by `:`, following the pattern:

```
{namespace}:{entity}:{identifier}
```

| Key | Example |
|-----|---------|
| `token:{sha256(token)}` | `token:a3f1c9...` |
| `ap:actor:{ap_id_hash}` | `ap:actor:7b2f3a...` |
| `domain_blocks` | `domain_blocks` (single key, no identifier) |
| `httpsig:{keyId}:{date}:{requestTarget}` | `httpsig:https://remote.example/users/bob#main-key:...` |
| `timeline:home:{accountID}` | `timeline:home:01HQ3M...` |
| `instance:settings` | `instance:settings` |

**Rules:**
- Token keys hash the token value (SHA-256, hex-encoded) to prevent a cache read from leaking the token itself in logs or memory dumps.
- AP actor IDs can be long URLs; hash them when they would produce unwieldy keys.
- Never embed user-controlled strings verbatim in cache keys without sanitisation.

---

### TTL guidelines

| Category | TTL | Rationale |
|----------|-----|-----------|
| OAuth access token | Until `expires_at` (non-expiring tokens: 24h) | Mirrors the token's own validity; a short cache TTL prevents stale revocation checks |
| AP actor document | 1 hour | Remote profiles change rarely; federation handles key rotation via `Update{Person}` |
| Domain block list | 1 hour with background refresh | Admin changes propagate within an hour; cache is refreshed proactively rather than expired |
| HTTP Signature replay set | 60 seconds | Matches the ±30s clock skew window; beyond this, replays are implausible |
| Home timeline | 60 seconds | Intentionally short (see below) |
| Instance settings | 5 minutes | Frequent reads, infrequent writes; admin sees changes within 5 minutes without a restart |

**Why 60 seconds for the home timeline, not event-driven invalidation:**  
Event-driven invalidation (deleting the timeline cache entry whenever any followed account posts) requires the status service to know which accounts are following the poster and delete each of their cached timelines. That is fan-out-on-write logic. Since Monstera-fed uses fan-out-on-read, the simplest consistent choice is a short fixed TTL — the timeline is at most 60 seconds stale, which matches the expected client polling interval for Mastodon streaming clients that reconnect on SSE disconnect. When the SSE stream is active, clients receive real-time pushes and do not rely on the cached timeline anyway.

---

### Cache invalidation on writes

Cache invalidation is **explicit and local to the service layer**. There is no magic invalidation, no cache event bus, and no TTL-only strategy for mutable data.

Pattern:

Rules:
1. Every `UPDATE` or `DELETE` that touches a cached entity must call `cache.Delete` for the affected key(s).
2. Invalidation failures are logged but not fatal — the TTL will expire the stale entry.
3. A service must only invalidate keys it owns. The token cache is managed by the auth middleware; the AP actor cache is managed by the federation service.
4. `GetOrSet` in `helpers.go` handles the read-side of cache-aside automatically. The write-side invalidation is always manual.

---
