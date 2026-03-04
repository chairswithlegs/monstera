package service

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/media"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeMediaStore implements media.MediaStore for tests.
type fakeMediaStore struct {
	baseURL string
}

func (f *fakeMediaStore) Put(ctx context.Context, key string, r io.Reader, contentType string) error {
	_, _ = io.Copy(io.Discard, r)
	return nil
}

func (f *fakeMediaStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	return nil, media.ErrNotFound
}

func (f *fakeMediaStore) Delete(ctx context.Context, key string) error {
	return nil
}

func (f *fakeMediaStore) URL(ctx context.Context, key string) (string, error) {
	return f.baseURL + "/" + key, nil
}

func TestMediaService_Upload_success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	mediaStore := &fakeMediaStore{baseURL: "https://media.example.com"}
	mediaSvc := NewMediaService(fake, mediaStore, 10<<20)

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)

	body := strings.NewReader("image bytes")
	desc := "alt text"
	result, err := mediaSvc.Upload(ctx, acc.ID, body, "image/jpeg", &desc)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Attachment)
	assert.Equal(t, acc.ID, result.Attachment.AccountID)
	assert.Equal(t, domain.MediaTypeImage, result.Attachment.Type)
	assert.NotEmpty(t, result.Attachment.StorageKey)
	assert.Contains(t, result.Attachment.URL, "https://media.example.com/")
	assert.Equal(t, "alt text", *result.Attachment.Description)
}

func TestMediaService_Upload_invalid_content_type_returns_validation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	mediaStore := &fakeMediaStore{baseURL: "https://media.example.com"}
	mediaSvc := NewMediaService(fake, mediaStore, 10<<20)

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)

	body := strings.NewReader("data")
	_, err = mediaSvc.Upload(ctx, acc.ID, body, "application/octet-stream", nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestMediaService_Upload_nil_description_ok(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	mediaStore := &fakeMediaStore{baseURL: "https://media.example.com"}
	mediaSvc := NewMediaService(fake, mediaStore, 10<<20)

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)

	body := strings.NewReader("image")
	result, err := mediaSvc.Upload(ctx, acc.ID, body, "image/png", nil)
	require.NoError(t, err)
	require.NotNil(t, result.Attachment)
	assert.Nil(t, result.Attachment.Description)
}
