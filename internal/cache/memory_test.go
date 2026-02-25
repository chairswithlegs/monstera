package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryStore_GetSetDelete(t *testing.T) {
	ctx := context.Background()
	s, err := NewMemory(nil)
	require.NoError(t, err)

	_, err = s.Get(ctx, "missing")
	assert.ErrorIs(t, err, ErrCacheMiss)

	err = s.Set(ctx, "k", []byte("v"), 0)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond) // ristretto admits asynchronously
	val, err := s.Get(ctx, "k")
	require.NoError(t, err)
	assert.Equal(t, []byte("v"), val)

	exists, err := s.Exists(ctx, "k")
	require.NoError(t, err)
	assert.True(t, exists)

	err = s.Delete(ctx, "k")
	require.NoError(t, err)
	_, err = s.Get(ctx, "k")
	assert.ErrorIs(t, err, ErrCacheMiss)
	exists, err = s.Exists(ctx, "k")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestMemoryStore_TTL(t *testing.T) {
	ctx := context.Background()
	s, err := NewMemory(nil)
	require.NoError(t, err)

	err = s.Set(ctx, "ttl-key", []byte("v"), 50*time.Millisecond)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond) // ristretto admits asynchronously
	val, err := s.Get(ctx, "ttl-key")
	require.NoError(t, err)
	assert.Equal(t, []byte("v"), val)

	time.Sleep(100 * time.Millisecond)
	_, err = s.Get(ctx, "ttl-key")
	assert.ErrorIs(t, err, ErrCacheMiss)
}

func TestMemoryStore_Ping(t *testing.T) {
	s, err := NewMemory(nil)
	require.NoError(t, err)
	assert.NoError(t, s.Ping(context.Background()))
}
