package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"golang.org/x/sync/singleflight"
)

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
// protection. On miss it calls fn (once per key under concurrency), stores the
// result, and returns it. Cache write errors are not returned.
// Get errors (e.g. connection failure) are treated as a miss: fn is called and its result returned.
func GetOrSet[T any](ctx context.Context, s Store, key string, ttl time.Duration, fn func() (T, error)) (T, error) {
	var cached T
	if hit, _ := GetJSON(ctx, s, key, &cached); hit {
		return cached, nil
	}
	v, err, _ := sfGroup.Do(key, func() (any, error) {
		result, err := fn()
		if err != nil {
			return result, err
		}
		_ = SetJSON(ctx, s, key, result, ttl)
		return result, nil
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return v.(T), nil
}
