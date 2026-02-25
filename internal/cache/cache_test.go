package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_Memory(t *testing.T) {
	s, err := New(Config{Driver: "memory"})
	require.NoError(t, err)
	require.NotNil(t, s)
	ctx := context.Background()
	err = s.Set(ctx, "x", []byte("y"), time.Minute)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond) // ristretto admits asynchronously
	val, err := s.Get(ctx, "x")
	require.NoError(t, err)
	assert.Equal(t, []byte("y"), val)
	_, ok := s.(*MemoryStore)
	assert.True(t, ok)
}

func TestNew_UnknownDriver(t *testing.T) {
	_, err := New(Config{Driver: "invalid"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown driver")
}

func TestNew_RedisRequiresURL(t *testing.T) {
	_, err := New(Config{Driver: "redis"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CACHE_REDIS_URL")
}

func TestGetJSON_SetJSON(t *testing.T) {
	ctx := context.Background()
	s, err := New(Config{Driver: "memory"})
	require.NoError(t, err)
	type V struct{ A int }
	err = SetJSON(ctx, s, "j", V{A: 42}, time.Minute)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond) // ristretto admits asynchronously
	var out V
	hit, err := GetJSON(ctx, s, "j", &out)
	require.NoError(t, err)
	assert.True(t, hit)
	assert.Equal(t, 42, out.A)
	hit, _ = GetJSON(ctx, s, "missing", &out)
	assert.False(t, hit)
}
