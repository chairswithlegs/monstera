package cache

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/dgraph-io/ristretto/v2"
)

// MemoryStore is the in-memory cache implementation.
type MemoryStore struct {
	c      *ristretto.Cache[string, []byte]
	logger *slog.Logger
}

// NewMemory creates a ristretto-backed in-memory Store.
func NewMemory(logger *slog.Logger) (*MemoryStore, error) {
	c, err := ristretto.NewCache(&ristretto.Config[string, []byte]{
		NumCounters: 1_000_000,
		MaxCost:     128 << 20,
		BufferItems: 64,
	})
	if err != nil {
		return nil, fmt.Errorf("cache/memory: init ristretto: %w", err)
	}
	return &MemoryStore{c: c, logger: logger}, nil
}

// Get retrieves the value for key. Returns ErrCacheMiss if the key is absent or expired.
func (s *MemoryStore) Get(_ context.Context, key string) ([]byte, error) {
	val, found := s.c.Get(key)
	if !found {
		return nil, ErrCacheMiss
	}
	return val, nil
}

// Set stores value under key. If ttl > 0 the entry expires after that duration.
// Returns an error if the cache rejects the item (e.g. admission policy or capacity).
func (s *MemoryStore) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	cost := int64(len(value))
	if cost == 0 {
		cost = 1
	}
	ok := false
	if ttl > 0 {
		ok = s.c.SetWithTTL(key, value, cost, ttl)
	} else {
		ok = s.c.Set(key, value, cost)
	}
	if !ok {
		return fmt.Errorf("cache/memory: admission rejected for key %q", key)
	}
	return nil
}

// Delete removes key from the cache.
func (s *MemoryStore) Delete(_ context.Context, key string) error {
	s.c.Del(key)
	return nil
}

// Exists reports whether key is present and unexpired.
func (s *MemoryStore) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.Get(ctx, key)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, ErrCacheMiss) {
		return false, nil
	}
	return false, err
}

// Ping satisfies the Pinger interface.
func (s *MemoryStore) Ping(_ context.Context) error {
	return nil
}

// Close satisfies the Store interface. In-memory cache has no resources to release.
func (s *MemoryStore) Close() error {
	return nil
}
