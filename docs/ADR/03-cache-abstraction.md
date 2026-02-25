# ADR 03 — Cache Abstraction

> **Status:** Draft v0.1 — Feb 24, 2026  
> **Addresses:** `design-prompts/03-cache-abstraction.md`

---

## Design Decisions

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

## 1. `internal/cache/cache.go`

```go
package cache

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// ErrCacheMiss is returned by Get when the requested key does not exist in the cache.
// Callers must distinguish a cache miss from a genuine error:
//
//	val, err := store.Get(ctx, key)
//	if errors.Is(err, cache.ErrCacheMiss) {
//	    // key absent — fetch from source of truth
//	}
var ErrCacheMiss = errors.New("cache miss")

// Store is the cache abstraction used throughout Monstera-fed.
// Implementations must be safe for concurrent use by multiple goroutines.
// All keys and values are opaque byte slices; structured data is marshalled
// by the helpers in helpers.go.
type Store interface {
	// Get retrieves the value for key. Returns ErrCacheMiss if the key does
	// not exist or has expired.
	Get(ctx context.Context, key string) ([]byte, error)

	// Set stores value under key with the given TTL. A TTL of 0 means the
	// entry never expires (use with caution — prefer explicit TTLs).
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Delete removes the key from the cache. A no-op if the key is absent.
	Delete(ctx context.Context, key string) error

	// Exists reports whether key is present and unexpired in the cache.
	Exists(ctx context.Context, key string) (bool, error)
}

// Pinger is an optional interface that cache implementations may satisfy.
// The health checker type-asserts the Store to Pinger rather than adding
// Ping to the core interface, keeping Store minimal.
//
// Usage in the health checker:
//
//	if p, ok := cacheStore.(cache.Pinger); ok {
//	    if err := p.Ping(ctx); err != nil { ... }
//	}
type Pinger interface {
	Ping(ctx context.Context) error
}

// Config holds the fields the cache factory needs. Constructed by serve.go
// from *config.Config so that this package has no dependency on internal/config.
type Config struct {
	Driver   string // "memory" | "redis"
	RedisURL string // required when Driver == "redis"
	Logger   *slog.Logger
}

// New returns the Store implementation selected by cfg.Driver.
// Returns an error if the driver is unknown or if the implementation fails
// to initialise (e.g. Redis URL is unparseable or unreachable).
func New(cfg Config) (Store, error) {
	switch cfg.Driver {
	case "memory", "":
		return memory.New(cfg.Logger)
	case "redis":
		if cfg.RedisURL == "" {
			return nil, fmt.Errorf("cache: CACHE_REDIS_URL is required when CACHE_DRIVER=redis")
		}
		return redis.New(cfg.RedisURL)
	default:
		return nil, fmt.Errorf("cache: unknown driver %q (valid: memory, redis)", cfg.Driver)
	}
}
```

> **Note on imports:** The `New` factory references the sub-packages `memory` and `redis` via their full import paths (`github.com/yourorg/monstera-fed/internal/cache/memory` etc.). The snippet above omits those import paths for readability; they must be present in the actual file.

---

## 2. `internal/cache/memory/memory.go`

```go
// Package memory provides an in-process cache implementation backed by
// ristretto/v2. It is suitable for development and single-node deployments
// only. In a multi-replica deployment all replicas maintain independent caches,
// so entries written on one replica are invisible to others. Use the redis
// driver in production.
package memory

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/dgraph-io/ristretto/v2"

	"github.com/yourorg/monstera-fed/internal/cache"
)

// Store is the in-memory cache implementation.
// Safe for concurrent use — ristretto uses internal sharding and atomic operations.
type Store struct {
	c      *ristretto.Cache[string, []byte]
	logger *slog.Logger
}

// New creates a ristretto-backed in-memory Store.
//
// Ristretto configuration rationale:
//   - NumCounters: 10 × expected unique keys. We estimate ~100,000 keys at
//     peak (tokens, actors, timeline pages); 1,000,000 counters gives ristretto
//     enough frequency data to make good eviction decisions.
//   - MaxCost: 128 MB. A development instance rarely needs more; excess RAM use
//     on developer laptops is undesirable.
//   - BufferItems: 64 (ristretto's recommended default). Controls the size of
//     the internal admission buffer. Increasing it improves throughput at the
//     cost of higher memory overhead per shard.
//
// If logger is non-nil and APP_ENV is production, a warning is logged because
// the in-memory driver is not safe for multi-replica deployments.
func New(logger *slog.Logger) (*Store, error) {
	c, err := ristretto.NewCache(&ristretto.Config[string, []byte]{
		NumCounters: 1_000_000,
		MaxCost:     128 << 20, // 128 MB
		BufferItems: 64,
	})
	if err != nil {
		return nil, fmt.Errorf("cache/memory: init ristretto: %w", err)
	}
	return &Store{c: c, logger: logger}, nil
}

// WarnIfProduction logs a warning when the in-memory driver is used in production.
// Called from serve.go after constructing the store.
func (s *Store) WarnIfProduction(appEnv string) {
	if appEnv == "production" && s.logger != nil {
		s.logger.Warn(
			"in-memory cache driver is active in production; " +
				"cache state is NOT shared across replicas — set CACHE_DRIVER=redis",
		)
	}
}

// Get retrieves the value for key. Returns cache.ErrCacheMiss if the key is
// absent or expired. Ristretto TTL expiry is enforced internally.
func (s *Store) Get(_ context.Context, key string) ([]byte, error) {
	val, found := s.c.Get(key)
	if !found {
		return nil, cache.ErrCacheMiss
	}
	return val, nil
}

// Set stores value under key. If ttl > 0 the entry expires after that duration;
// if ttl == 0 the entry is held until evicted by cost pressure.
//
// Ristretto's Set is asynchronous: the item is admitted via a ring buffer and
// may not be immediately visible to Get. This is acceptable for a cache — a
// brief window of unavailability results in an extra DB read, not a correctness
// error.
//
// Cost is set to len(value), so MaxCost acts as a byte-level memory cap.
func (s *Store) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	cost := int64(len(value))
	if cost == 0 {
		cost = 1 // ristretto ignores zero-cost items
	}
	var ok bool
	if ttl > 0 {
		ok = s.c.SetWithTTL(key, value, cost, ttl)
	} else {
		ok = s.c.Set(key, value, cost)
	}
	if !ok {
		// SetWithTTL returns false when the item is rejected by the admission
		// policy (e.g. cost exceeds MaxCost). Treat as a best-effort write.
		return nil
	}
	return nil
}

// Delete removes key from the cache. Safe to call on absent keys.
func (s *Store) Delete(_ context.Context, key string) error {
	s.c.Del(key)
	return nil
}

// Exists reports whether key is present and unexpired.
func (s *Store) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.Get(ctx, key)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, cache.ErrCacheMiss) {
		return false, nil
	}
	return false, err
}

// Ping satisfies the cache.Pinger interface. Always returns nil for the
// in-memory implementation.
func (s *Store) Ping(_ context.Context) error {
	return nil
}
```

---

## 3. `internal/cache/redis/redis.go`

```go
// Package redis provides a Redis-backed cache implementation using go-redis/v9.
// Compatible with Redis 7+, Valkey, and KeyDB.
package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/yourorg/monstera-fed/internal/cache"
)

// Store is the Redis cache implementation.
// The underlying go-redis client maintains a connection pool and is safe
// for concurrent use by multiple goroutines.
type Store struct {
	client *goredis.Client
}

// New parses redisURL and returns a connected Store.
// Performs a Ping to verify connectivity at startup; returns an error if
// the server is unreachable so that serve.go can fail fast.
//
// Connection pool rationale:
//   - PoolSize: 10 — each replica needs at most a handful of concurrent cache
//     operations. 10 connections comfortably handles bursts without over-
//     provisioning Redis server-side connections.
//   - MinIdleConns: 2 — keeps warm connections ready; avoids handshake latency
//     on the first requests after a quiet period.
//   - ConnMaxIdleTime: 5m — matches the pgxpool idle timeout convention.
func New(redisURL string) (*Store, error) {
	opts, err := goredis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("cache/redis: parse URL: %w", err)
	}

	opts.PoolSize        = 10
	opts.MinIdleConns    = 2
	opts.ConnMaxIdleTime = 5 * time.Minute

	client := goredis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("cache/redis: ping: %w", err)
	}

	return &Store{client: client}, nil
}

// Get retrieves the raw bytes stored at key.
// Returns cache.ErrCacheMiss when the key does not exist or has expired.
// All other Redis errors are returned as-is so callers can decide whether
// to degrade gracefully or propagate the failure.
func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, cache.ErrCacheMiss
		}
		return nil, fmt.Errorf("cache/redis: get %q: %w", key, err)
	}
	return val, nil
}

// Set stores value at key with the given TTL using SET key value EX seconds.
// If ttl == 0 the key is stored without an expiry (use with caution).
// Redis SETEX requires ttl >= 1 second; sub-second TTLs are rounded up to 1s
// via SET … PX (milliseconds), which go-redis selects automatically when
// ttl is a non-zero duration.
func (s *Store) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := s.client.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("cache/redis: set %q: %w", key, err)
	}
	return nil
}

// Delete removes key from Redis. A no-op if the key does not exist.
func (s *Store) Delete(ctx context.Context, key string) error {
	if err := s.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("cache/redis: del %q: %w", key, err)
	}
	return nil
}

// Exists reports whether key is present and unexpired.
// Uses the Redis EXISTS command, which returns the count of matching keys.
func (s *Store) Exists(ctx context.Context, key string) (bool, error) {
	n, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("cache/redis: exists %q: %w", key, err)
	}
	return n > 0, nil
}

// Ping sends a Redis PING and returns the error, if any.
// Satisfies the cache.Pinger interface for the /healthz/ready endpoint.
func (s *Store) Ping(ctx context.Context) error {
	if err := s.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("cache/redis: ping: %w", err)
	}
	return nil
}

// Close gracefully shuts down the connection pool.
// Called during server shutdown after all request handlers have drained.
func (s *Store) Close() error {
	return s.client.Close()
}
```

---

## 4. `internal/cache/helpers.go`

```go
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"golang.org/x/sync/singleflight"
)

// sfGroup deduplicates concurrent cache-miss fetches for the same key.
// A package-level group is appropriate: the key namespace is already
// globally unique (prefixed per use case), and singleflight.Group is
// goroutine-safe with no initialisation required.
var sfGroup singleflight.Group

// GetJSON unmarshals the cached bytes at key into dest.
// Returns (true, nil) on a cache hit, (false, nil) on a miss, and
// (false, err) if the value exists but cannot be unmarshalled.
func GetJSON[T any](ctx context.Context, s Store, key string, dest *T) (bool, error) {
	b, err := s.Get(ctx, key)
	if err != nil {
		if errors.Is(err, ErrCacheMiss) {
			return false, nil
		}
		return false, err
	}
	if err := json.Unmarshal(b, dest); err != nil {
		// Corrupted or schema-changed entry — treat as a miss.
		_ = s.Delete(ctx, key)
		return false, nil
	}
	return true, nil
}

// SetJSON marshals val to JSON and stores it at key with the given TTL.
func SetJSON[T any](ctx context.Context, s Store, key string, val T, ttl time.Duration) error {
	b, err := json.Marshal(val)
	if err != nil {
		return fmt.Errorf("cache: marshal %q: %w", key, err)
	}
	return s.Set(ctx, key, b, ttl)
}

// GetOrSet implements the cache-aside pattern with singleflight thundering-herd
// protection.
//
// It first attempts to read key from the cache. On a hit it returns the
// unmarshalled value immediately. On a miss it calls fn — but only once per
// unique key even under concurrent load (singleflight deduplicates callers
// waiting on the same key). The computed value is stored in the cache before
// returning.
//
// Thundering-herd note: singleflight is included because AP actor document
// fetches are a realistic stampede scenario — when a popular remote account
// is followed by many local users simultaneously, all of them miss the actor
// cache at the same moment and would otherwise each issue an outbound HTTP
// request. singleflight collapses those into a single fetch.
//
// Cache write errors are logged but not returned — a failed cache write is
// never fatal; the next caller will simply re-fetch from the source.
func GetOrSet[T any](ctx context.Context, s Store, key string, ttl time.Duration, fn func() (T, error)) (T, error) {
	// Fast path: cache hit.
	var cached T
	if hit, _ := GetJSON(ctx, s, key, &cached); hit {
		return cached, nil
	}

	// Slow path: cache miss — deduplicate with singleflight.
	v, err, _ := sfGroup.Do(key, func() (any, error) {
		result, err := fn()
		if err != nil {
			return result, err
		}
		// Best-effort cache write; errors are intentionally discarded.
		_ = SetJSON(ctx, s, key, result, ttl)
		return result, nil
	})
	if err != nil {
		var zero T
		return zero, err
	}

	// The value returned by singleflight is always of type T because fn
	// returns T and we pass it through as-is.
	return v.(T), nil //nolint:forcetypeassert
}
```

---

## 5. Health Check Integration

The `/healthz/ready` handler (designed in ADR 01) should type-assert the cache `Store` to `cache.Pinger`. No changes to the `HealthChecker` struct signature are required — just an additional check in `Readiness`:

```go
// In internal/api/health.go — updated Readiness handler pseudocode:

type HealthChecker struct {
    db    *pgxpool.Pool
    nats  *nats.Conn
    cache cache.Store // added
}

func (h *HealthChecker) Readiness(w http.ResponseWriter, r *http.Request) {
    checks := map[string]string{}

    // PostgreSQL
    if err := h.db.Ping(ctx); err != nil {
        checks["postgres"] = "error"
    } else {
        checks["postgres"] = "ok"
    }

    // NATS
    if !h.nats.IsConnected() {
        checks["nats"] = "error"
    } else {
        checks["nats"] = "ok"
    }

    // Cache (optional — only if implementation supports Ping)
    if pinger, ok := h.cache.(cache.Pinger); ok {
        if err := pinger.Ping(ctx); err != nil {
            checks["cache"] = "error"
        } else {
            checks["cache"] = "ok"
        }
    }

    allOK := true
    for _, v := range checks {
        if v == "error" {
            allOK = false
        }
    }
    status := http.StatusOK
    if !allOK {
        status = http.StatusServiceUnavailable
    }
    writeJSON(w, status, map[string]any{
        "status": map[bool]string{true: "ok", false: "error"}[allOK],
        "checks": checks,
    })
}
```

The type assertion `h.cache.(cache.Pinger)` is safe because both the memory and redis implementations satisfy `Pinger`. If a future implementation does not, the check is simply skipped — the interface remains optional.

---

## 6. Caching Conventions

*Suitable for the project wiki or `docs/caching.md`.*

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
```go
// In service layer — after a successful DB write:
func (s *AccountService) UpdateProfile(ctx context.Context, ...) error {
    if err := s.store.UpdateAccount(ctx, params); err != nil {
        return err
    }
    // Explicit invalidation — next read will re-fetch from DB and re-cache.
    _ = s.cache.Delete(ctx, apActorKey(account.APID))
    return nil
}
```

Rules:
1. Every `UPDATE` or `DELETE` that touches a cached entity must call `cache.Delete` for the affected key(s).
2. Invalidation failures are logged but not fatal — the TTL will expire the stale entry.
3. A service must only invalidate keys it owns. The token cache is managed by the auth middleware; the AP actor cache is managed by the federation service.
4. `GetOrSet` in `helpers.go` handles the read-side of cache-aside automatically. The write-side invalidation is always manual.

---

## 7. `go.mod` Additions

```
require (
    github.com/dgraph-io/ristretto/v2  v2.x.x
    github.com/redis/go-redis/v9       v9.x.x
    golang.org/x/sync                  v0.x.x  // for singleflight
)
```

Run `go get github.com/dgraph-io/ristretto/v2 github.com/redis/go-redis/v9 golang.org/x/sync` to resolve exact versions.

---

## 8. Open Questions

No blocking open questions. One minor note for implementation:

| # | Note | Impact |
|---|------|--------|
| 1 | **Ristretto `Wait()` in tests** — ristretto's async admission means that in unit tests, `Set` followed immediately by `Get` may return a miss. Tests of the memory store should call `store.c.Wait()` after `Set` to synchronise. This is only a test concern; production code is unaffected (the brief admission window is acceptable for a cache). | Testing only |

---

*End of ADR 03 — Cache Abstraction*
