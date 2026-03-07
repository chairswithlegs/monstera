package service

import (
	"bytes"
	"context"
	"encoding/json"
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

	body := bytes.NewReader(testutil.MinimalJPEG(t))
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
	require.NotNil(t, result.Attachment.PreviewURL, "image upload should set preview URL")
	assert.Contains(t, *result.Attachment.PreviewURL, "https://media.example.com/")
	require.NotNil(t, result.Attachment.Blurhash, "image upload should set blurhash")
	assert.NotEmpty(t, *result.Attachment.Blurhash)
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

	body := bytes.NewReader(testutil.MinimalPNG(t))
	result, err := mediaSvc.Upload(ctx, acc.ID, body, "image/png", nil)
	require.NoError(t, err)
	require.NotNil(t, result.Attachment)
	assert.Nil(t, result.Attachment.Description)
}

func TestMediaService_Upload_body_exceeds_max_returns_validation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	mediaStore := &fakeMediaStore{baseURL: "https://media.example.com"}
	mediaSvc := NewMediaService(fake, mediaStore, 5)

	acc, err := NewAccountService(fake, "https://example.com").Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)

	body := strings.NewReader("123456")
	_, err = mediaSvc.Upload(ctx, acc.ID, body, "image/jpeg", nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestMediaService_Upload_invalid_image_bytes_returns_validation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	mediaStore := &fakeMediaStore{baseURL: "https://media.example.com"}
	mediaSvc := NewMediaService(fake, mediaStore, 10<<20)

	acc, err := NewAccountService(fake, "https://example.com").Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)

	body := strings.NewReader("not a valid image")
	_, err = mediaSvc.Upload(ctx, acc.ID, body, "image/jpeg", nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestMediaService_Update_success_description(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	mediaStore := &fakeMediaStore{baseURL: "https://media.example.com"}
	mediaSvc := NewMediaService(fake, mediaStore, 10<<20)

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	result, err := mediaSvc.Upload(ctx, acc.ID, bytes.NewReader(testutil.MinimalJPEG(t)), "image/jpeg", nil)
	require.NoError(t, err)
	mediaID := result.Attachment.ID

	newDesc := "updated alt text"
	updated, err := mediaSvc.Update(ctx, acc.ID, mediaID, &newDesc, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "updated alt text", *updated.Description)
	assert.Equal(t, mediaID, updated.ID)
}

func TestMediaService_Update_success_focus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	mediaStore := &fakeMediaStore{baseURL: "https://media.example.com"}
	mediaSvc := NewMediaService(fake, mediaStore, 10<<20)

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	result, err := mediaSvc.Upload(ctx, acc.ID, bytes.NewReader(testutil.MinimalJPEG(t)), "image/jpeg", nil)
	require.NoError(t, err)
	mediaID := result.Attachment.ID

	fx, fy := -0.5, 0.5
	updated, err := mediaSvc.Update(ctx, acc.ID, mediaID, nil, &fx, &fy)
	require.NoError(t, err)
	require.NotNil(t, updated)
	require.NotNil(t, updated.Meta)
	var meta map[string]any
	require.NoError(t, json.Unmarshal(updated.Meta, &meta))
	focus, ok := meta["focus"].(map[string]any)
	require.True(t, ok)
	assert.InDelta(t, -0.5, focus["x"], 1e-9)
	assert.InDelta(t, 0.5, focus["y"], 1e-9)
}

func TestMediaService_Update_success_description_and_focus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	mediaStore := &fakeMediaStore{baseURL: "https://media.example.com"}
	mediaSvc := NewMediaService(fake, mediaStore, 10<<20)

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	result, err := mediaSvc.Upload(ctx, acc.ID, bytes.NewReader(testutil.MinimalJPEG(t)), "image/jpeg", nil)
	require.NoError(t, err)
	mediaID := result.Attachment.ID

	desc := "new description"
	fx, fy := 0.0, 0.0
	updated, err := mediaSvc.Update(ctx, acc.ID, mediaID, &desc, &fx, &fy)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "new description", *updated.Description)
	var meta map[string]any
	require.NoError(t, json.Unmarshal(updated.Meta, &meta))
	focus, ok := meta["focus"].(map[string]any)
	require.True(t, ok)
	assert.InDelta(t, 0.0, focus["x"], 1e-9)
	assert.InDelta(t, 0.0, focus["y"], 1e-9)
}

func TestMediaService_Update_not_found(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	mediaStore := &fakeMediaStore{baseURL: "https://media.example.com"}
	mediaSvc := NewMediaService(fake, mediaStore, 10<<20)

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)

	desc := "desc"
	_, err = mediaSvc.Update(ctx, acc.ID, "01HZXY99999999999999999999", &desc, nil, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestMediaService_Update_wrong_account_returns_not_found(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	mediaStore := &fakeMediaStore{baseURL: "https://media.example.com"}
	mediaSvc := NewMediaService(fake, mediaStore, 10<<20)

	alice, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	bob, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)
	result, err := mediaSvc.Upload(ctx, alice.ID, bytes.NewReader(testutil.MinimalJPEG(t)), "image/jpeg", nil)
	require.NoError(t, err)
	mediaID := result.Attachment.ID

	desc := "hacked"
	_, err = mediaSvc.Update(ctx, bob.ID, mediaID, &desc, nil, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestMediaService_Update_already_attached_returns_validation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	mediaStore := &fakeMediaStore{baseURL: "https://media.example.com"}
	mediaSvc := NewMediaService(fake, mediaStore, 10<<20)

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	result, err := mediaSvc.Upload(ctx, acc.ID, bytes.NewReader(testutil.MinimalJPEG(t)), "image/jpeg", nil)
	require.NoError(t, err)
	mediaID := result.Attachment.ID
	statusID := "01HZXY00000000000000000001"
	result.Attachment.StatusID = &statusID

	desc := "updated"
	_, err = mediaSvc.Update(ctx, acc.ID, mediaID, &desc, nil, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrUnprocessable)
}
