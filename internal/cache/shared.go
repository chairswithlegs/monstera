package cache

import (
	"context"
	"time"
)

// SharedStore is the cache interface for state that must be consistent across
// all pods (e.g. idempotency keys, HTTP signature replay detection, OAuth token
// cache/revocation). It has the same method signatures as Store but is a
// distinct type so callers that require cross-pod consistency cannot
// accidentally be wired with a local-only implementation.
type SharedStore interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	Close() error
}
