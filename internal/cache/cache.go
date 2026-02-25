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
// Close releases resources (e.g. connection pools); no-op for in-memory stores.
type Store interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	Close() error
}

// Pinger is an optional interface that cache implementations may satisfy.
// The health checker type-asserts the Store to Pinger rather than adding
// Ping to the core interface, keeping Store minimal.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Config holds the fields the cache factory needs.
type Config struct {
	Driver   string // "memory" | "redis"
	RedisURL string // required when Driver == "redis"
	Logger   *slog.Logger
}

// New returns the Store implementation selected by cfg.Driver.
func New(cfg Config) (Store, error) {
	switch cfg.Driver {
	case "memory", "":
		return NewMemory(cfg.Logger)
	case "redis":
		if cfg.RedisURL == "" {
			return nil, errors.New("cache: CACHE_REDIS_URL is required when CACHE_DRIVER=redis")
		}
		return NewRedis(cfg.RedisURL)
	default:
		return nil, fmt.Errorf("cache: unknown driver %q (valid: memory, redis)", cfg.Driver)
	}
}
