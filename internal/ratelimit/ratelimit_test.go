package ratelimit

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/chairswithlegs/monstera/internal/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// syncStore is a synchronous in-memory SharedStore for testing.
// Ristretto writes asynchronously so it's unsuitable for tests that
// need immediate read-after-write consistency.
type syncStore struct {
	mu    sync.Mutex
	items map[string][]byte
}

func newSyncStore() *syncStore {
	return &syncStore{items: make(map[string][]byte)}
}

func (s *syncStore) Get(_ context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.items[key]
	if !ok {
		return nil, cache.ErrCacheMiss
	}
	return v, nil
}

func (s *syncStore) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[key] = value
	return nil
}

func (s *syncStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, key)
	return nil
}

func (s *syncStore) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.Get(ctx, key)
	if err == nil {
		return true, nil
	}
	return false, nil
}

func (s *syncStore) Close() error { return nil }

func TestLimiter_AllowWithinLimit(t *testing.T) {
	t.Parallel()
	lim := New(newSyncStore())
	ctx := context.Background()

	for i := range 5 {
		r, err := lim.Allow(ctx, "test-user", 5, 5*time.Minute)
		require.NoError(t, err)
		assert.True(t, r.Allowed, "request %d should be allowed", i+1)
		assert.Equal(t, 5, r.Limit)
		assert.Equal(t, 5-i-1, r.Remaining)
	}
}

func TestLimiter_DenyOverLimit(t *testing.T) {
	t.Parallel()
	lim := New(newSyncStore())
	ctx := context.Background()

	for range 3 {
		r, err := lim.Allow(ctx, "user-2", 3, 5*time.Minute)
		require.NoError(t, err)
		assert.True(t, r.Allowed)
	}

	r, err := lim.Allow(ctx, "user-2", 3, 5*time.Minute)
	require.NoError(t, err)
	assert.False(t, r.Allowed)
	assert.Equal(t, 0, r.Remaining)
}

func TestLimiter_DifferentKeys(t *testing.T) {
	t.Parallel()
	lim := New(newSyncStore())
	ctx := context.Background()

	r1, _ := lim.Allow(ctx, "a", 1, 5*time.Minute)
	assert.True(t, r1.Allowed)
	r2, _ := lim.Allow(ctx, "b", 1, 5*time.Minute)
	assert.True(t, r2.Allowed)

	r3, _ := lim.Allow(ctx, "a", 1, 5*time.Minute)
	assert.False(t, r3.Allowed)
}

func TestLimiter_ResetAtIsWindowEnd(t *testing.T) {
	t.Parallel()
	lim := New(newSyncStore())
	ctx := context.Background()

	r, err := lim.Allow(ctx, "t", 10, 5*time.Minute)
	require.NoError(t, err)
	assert.False(t, r.ResetAt.IsZero())
	assert.True(t, r.ResetAt.After(time.Now()))
}
