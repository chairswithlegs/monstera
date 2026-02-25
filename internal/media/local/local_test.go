package local

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera-fed/internal/media"
)

func TestStore_PutGetRoundTrip(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	store, err := New(dir, "https://example.com")
	require.NoError(t, err)

	key := "media/2026/02/25/test123.jpg"
	content := []byte("jpeg content")
	err = store.Put(ctx, key, bytes.NewReader(content), "image/jpeg")
	require.NoError(t, err)

	rc, err := store.Get(ctx, key)
	require.NoError(t, err)
	defer rc.Close()
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, content, got)
}

func TestStore_GetNotFound(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	store, err := New(dir, "https://example.com")
	require.NoError(t, err)

	_, err = store.Get(ctx, "media/2026/02/25/nonexistent.jpg")
	require.Error(t, err)
	assert.ErrorIs(t, err, media.ErrNotFound)
}

func TestStore_Delete(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	store, err := New(dir, "https://example.com")
	require.NoError(t, err)

	key := "media/2026/02/25/delete-me.jpg"
	require.NoError(t, store.Put(ctx, key, bytes.NewReader([]byte("x")), "image/jpeg"))

	err = store.Delete(ctx, key)
	require.NoError(t, err)

	_, err = store.Get(ctx, key)
	assert.ErrorIs(t, err, media.ErrNotFound)
}

func TestStore_DeleteNonexistent(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	store, err := New(dir, "https://example.com")
	require.NoError(t, err)

	err = store.Delete(ctx, "media/2026/02/25/not-there.jpg")
	require.NoError(t, err)
}

func TestStore_URL(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	store, err := New(dir, "https://example.com")
	require.NoError(t, err)

	url, err := store.URL(ctx, "media/2026/02/25/foo.jpg")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/system/media/2026/02/25/foo.jpg", url)
}
