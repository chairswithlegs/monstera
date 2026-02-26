package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockStore is a minimal Store for testing helpers.
type mockStore struct {
	getBytes     []byte
	getErr       error
	setErr       error
	deleteCalled string
}

func (m *mockStore) Get(ctx context.Context, key string) ([]byte, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.getBytes, nil
}

func (m *mockStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return m.setErr
}

func (m *mockStore) Delete(ctx context.Context, key string) error {
	m.deleteCalled = key
	return nil
}

func (m *mockStore) Exists(ctx context.Context, key string) (bool, error) {
	return len(m.getBytes) > 0 && m.getErr == nil, nil
}

func (m *mockStore) Close() error { return nil }

func TestGetJSON_Miss(t *testing.T) {
	ctx := context.Background()
	s := &mockStore{getErr: ErrCacheMiss}
	var out struct{ A int }
	hit, err := GetJSON(ctx, s, "k", &out)
	require.NoError(t, err)
	assert.False(t, hit)
}

func TestGetJSON_GetError(t *testing.T) {
	ctx := context.Background()
	wantErr := errors.New("get failed")
	s := &mockStore{getErr: wantErr}
	var out struct{ A int }
	hit, err := GetJSON(ctx, s, "k", &out)
	require.Error(t, err)
	require.ErrorIs(t, err, wantErr)
	assert.False(t, hit)
	assert.Contains(t, err.Error(), "cache: get")
}

func TestGetJSON_UnmarshalError(t *testing.T) {
	ctx := context.Background()
	s := &mockStore{getBytes: []byte("not valid json")}
	var out struct{ A int }
	hit, err := GetJSON(ctx, s, "k", &out)
	require.Error(t, err)
	assert.False(t, hit)
	assert.Contains(t, err.Error(), "unmarshal")
	assert.Equal(t, "k", s.deleteCalled)
}

func TestGetJSON_Hit(t *testing.T) {
	ctx := context.Background()
	s := &mockStore{getBytes: []byte(`{"A":42}`)}
	var out struct{ A int }
	hit, err := GetJSON(ctx, s, "k", &out)
	require.NoError(t, err)
	assert.True(t, hit)
	assert.Equal(t, 42, out.A)
}

func TestSetJSON_MarshalError(t *testing.T) {
	ctx := context.Background()
	s := &mockStore{}
	err := SetJSON(ctx, s, "k", make(chan int), time.Minute)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "marshal")
}

func TestSetJSON_SetError(t *testing.T) {
	ctx := context.Background()
	wantErr := errors.New("set failed")
	s := &mockStore{setErr: wantErr}
	err := SetJSON(ctx, s, "k", struct{ A int }{1}, time.Minute)
	require.Error(t, err)
	require.ErrorIs(t, err, wantErr)
	assert.Contains(t, err.Error(), "cache: set")
}

func TestSetJSON_OK(t *testing.T) {
	ctx := context.Background()
	s := &mockStore{}
	err := SetJSON(ctx, s, "k", struct{ A int }{1}, time.Minute)
	require.NoError(t, err)
}

func TestGetOrSet_Hit(t *testing.T) {
	ctx := context.Background()
	s := &mockStore{getBytes: []byte(`{"A":10}`)}
	got, err := GetOrSet(ctx, s, "k", time.Minute, func() (struct{ A int }, error) {
		return struct{ A int }{99}, nil
	})
	require.NoError(t, err)
	assert.Equal(t, 10, got.A)
}

func TestGetOrSet_Miss(t *testing.T) {
	ctx := context.Background()
	s := &mockStore{getErr: ErrCacheMiss}
	got, err := GetOrSet(ctx, s, "k", time.Minute, func() (struct{ A int }, error) {
		return struct{ A int }{7}, nil
	})
	require.NoError(t, err)
	assert.Equal(t, 7, got.A)
}

func TestGetOrSet_FnError(t *testing.T) {
	ctx := context.Background()
	wantErr := errors.New("fn failed")
	s := &mockStore{getErr: ErrCacheMiss}
	got, err := GetOrSet(ctx, s, "k", time.Minute, func() (struct{ A int }, error) {
		return struct{ A int }{}, wantErr
	})
	require.Error(t, err)
	require.ErrorIs(t, err, wantErr)
	assert.Contains(t, err.Error(), "get-or-set")
	assert.Equal(t, 0, got.A)
}
