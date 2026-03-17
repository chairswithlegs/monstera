package ratelimit

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/chairswithlegs/monstera/internal/cache"
)

// Result reports the outcome of a rate limit check.
type Result struct {
	Allowed   bool
	Limit     int
	Remaining int
	ResetAt   time.Time
}

// Limiter checks whether a request is allowed under rate limits.
type Limiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (Result, error)
}

type limiter struct {
	store cache.SharedStore
}

// New returns a Limiter backed by a SharedStore.
func New(store cache.SharedStore) Limiter {
	return &limiter{store: store}
}

// Allow uses a simple get-increment-set counter per window. The check is not
// atomic, so concurrent requests may slightly exceed the limit under high
// contention. This is acceptable for a soft rate limit; strict enforcement
// would require an atomic compare-and-swap or Lua-style scripting.
func (l *limiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (Result, error) {
	now := time.Now()
	windowStart := now.Truncate(window)
	resetAt := windowStart.Add(window)
	cacheKey := fmt.Sprintf("rl:%s:%d", key, windowStart.Unix())

	var count int
	b, err := l.store.Get(ctx, cacheKey)
	if err == nil && len(b) >= 4 {
		count = int(binary.BigEndian.Uint32(b))
	}

	if count >= limit {
		return Result{
			Allowed:   false,
			Limit:     limit,
			Remaining: 0,
			ResetAt:   resetAt,
		}, nil
	}

	count++
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(count)) //nolint:gosec // count is capped by limit
	_ = l.store.Set(ctx, cacheKey, buf, window)

	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}
	return Result{
		Allowed:   true,
		Limit:     limit,
		Remaining: remaining,
		ResetAt:   resetAt,
	}, nil
}
