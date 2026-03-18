# Rate limiting and shared cache

This document describes the rate limiting system and the shared cache layer that supports it.

## Shared cache

### Problem

Monstera is designed to run as multiple stateless API pods behind a load balancer. Some state must be consistent across pods: rate limit counters, idempotency keys, and HTTP signature replay detection. A per-process in-memory cache cannot enforce these guarantees because each pod has its own cache.

### Solution: SharedStore

`internal/cache/shared.go` defines a `SharedStore` interface distinct from the local `cache.Store`. The type separation ensures callers that need cross-pod consistency cannot accidentally be wired with a local-only implementation.

Two implementations exist:

| Implementation | Package | Backing | Use case |
|---|---|---|---|
| NATS JetStream KV | `internal/cache/nats` | NATS KV bucket (`CACHE`, memory storage) | Production / multi-pod |
| In-memory (ristretto) | `internal/cache` | Process-local | Single-pod / development |

The NATS KV implementation uses a single bucket with a configurable max TTL. Per-key TTL is not supported by NATS KV, so all entries share the bucket-level TTL. This is sufficient for the current use cases (rate limit windows, idempotency keys) which have similar lifetimes.

### Why NATS KV?

NATS JetStream is already required for federation delivery and domain events. Reusing it for shared cache avoids adding another infrastructure dependency (e.g. Redis). The KV API is simple, the data is ephemeral (memory storage), and it scales with the NATS cluster.

## Rate limiting

### Design

Rate limiting uses a sliding-window counter pattern backed by the shared cache. The `internal/ratelimit` package defines a `Limiter` interface and an implementation that:

1. Computes a window key from the caller-provided key + truncated timestamp.
2. Reads the current count from the shared cache.
3. If the count exceeds the limit, rejects the request.
4. Otherwise, increments the counter and writes it back.

The check is not atomic — under high contention, concurrent requests may slightly exceed the limit. This is acceptable for a soft rate limit. Strict enforcement would require an atomic compare-and-swap, which the NATS KV API does not currently support.

### HTTP middleware

`internal/api/middleware/ratelimit.go` provides middleware that integrates the limiter with the HTTP layer:

| Middleware | Key strategy | Use case |
|---|---|---|
| `RateLimitByAccount` | Authenticated account ID | Authenticated API endpoints |
| `RateLimitByIP` | Client IP address | Unauthenticated or public endpoints |
| `RateLimit` | Custom key function | Flexible; the above two are built on this |

When a request is rate limited:

- Standard `X-RateLimit-Limit`, `X-RateLimit-Remaining`, and `X-RateLimit-Reset` headers are set on every response.
- Rejected requests receive a 429 status with a `Retry-After` header.
- If the limiter itself fails (e.g. cache unavailable), the request is **allowed** — rate limiting is fail-open to avoid blocking traffic during infrastructure issues.

### Body size limiting

`internal/api/middleware/bodysize.go` provides a `MaxBodySize` middleware that wraps the request body with `http.MaxBytesReader`. This is separate from rate limiting but serves a related protective purpose: preventing oversized payloads from consuming memory.

## Key files

| File | Responsibility |
|------|----------------|
| `internal/cache/shared.go` | `SharedStore` interface |
| `internal/cache/nats/nats.go` | NATS JetStream KV implementation |
| `internal/ratelimit/ratelimit.go` | `Limiter` interface and sliding-window implementation |
| `internal/api/middleware/ratelimit.go` | HTTP rate limit middleware |
| `internal/api/middleware/bodysize.go` | Request body size limiting middleware |
