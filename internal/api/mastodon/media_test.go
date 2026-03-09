package mastodon

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/media"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
func (f *fakeMediaStore) Delete(ctx context.Context, key string) error { return nil }
func (f *fakeMediaStore) URL(ctx context.Context, key string) (string, error) {
	return f.baseURL + "/" + key, nil
}

func TestMediaHandler_POSTMedia(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	mediaStore := &fakeMediaStore{baseURL: "https://media.example.com"}
	mediaSvc := service.NewMediaService(st, mediaStore, 10<<20)
	handler := NewMediaHandler(mediaSvc)

	acc, err := accountSvc.Create(ctx, service.CreateAccountInput{Username: "alice"})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		fw, _ := w.CreateFormFile("file", "image.jpg")
		_, _ = fw.Write(testutil.MinimalJPEG(t))
		contentType := w.FormDataContentType()
		require.NoError(t, w.Close())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/media", &buf)
		req.Header.Set("Content-Type", contentType)
		rec := httptest.NewRecorder()
		handler.POSTMedia(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("missing file returns 400", func(t *testing.T) {
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		_ = w.WriteField("description", "alt")
		contentType := w.FormDataContentType()
		require.NoError(t, w.Close())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/media", &buf)
		req.Header.Set("Content-Type", contentType)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.POSTMedia(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("valid file returns 200 and attachment", func(t *testing.T) {
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		partHeader := textproto.MIMEHeader{
			"Content-Disposition": {`form-data; name="file"; filename="image.jpg"`},
			"Content-Type":        {"image/jpeg"},
		}
		fw, err := w.CreatePart(partHeader)
		require.NoError(t, err)
		_, err = fw.Write(testutil.MinimalJPEG(t))
		require.NoError(t, err)
		contentType := w.FormDataContentType()
		require.NoError(t, w.Close())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/media", &buf)
		req.Header.Set("Content-Type", contentType)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.POSTMedia(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.NotEmpty(t, body["id"])
		assert.Equal(t, "image", body["type"])
	})
}

func TestMediaHandler_PUTMedia(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	mediaStore := &fakeMediaStore{baseURL: "https://media.example.com"}
	mediaSvc := service.NewMediaService(st, mediaStore, 10<<20)
	handler := NewMediaHandler(mediaSvc)

	acc, err := accountSvc.Create(ctx, service.CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	otherAcc, err := accountSvc.Create(ctx, service.CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	uploadResult, err := mediaSvc.Upload(ctx, acc.ID, bytes.NewReader(testutil.MinimalJPEG(t)), "image/jpeg", nil)
	require.NoError(t, err)
	mediaID := uploadResult.Attachment.ID

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/media/"+mediaID, strings.NewReader("description=alt"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = testutil.AddChiURLParam(req, "id", mediaID)
		rec := httptest.NewRecorder()
		handler.PUTMedia(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("missing id returns 422", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/media/", strings.NewReader(""))
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.PUTMedia(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("not found returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/media/01HZXY99999999999999999999", strings.NewReader("description=alt"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", "01HZXY99999999999999999999")
		rec := httptest.NewRecorder()
		handler.PUTMedia(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("wrong account returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/media/"+mediaID, strings.NewReader("description=hacked"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(middleware.WithAccount(req.Context(), otherAcc))
		req = testutil.AddChiURLParam(req, "id", mediaID)
		rec := httptest.NewRecorder()
		handler.PUTMedia(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("invalid focus format returns 422", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/media/"+mediaID, strings.NewReader("focus=not-two-numbers"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", mediaID)
		rec := httptest.NewRecorder()
		handler.PUTMedia(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("focus out of range returns 422", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/media/"+mediaID, strings.NewReader("focus=1.5,-0.5"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", mediaID)
		rec := httptest.NewRecorder()
		handler.PUTMedia(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("success description returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/media/"+mediaID, strings.NewReader("description=updated+alt+text"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", mediaID)
		rec := httptest.NewRecorder()
		handler.PUTMedia(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, mediaID, body["id"])
		assert.Equal(t, "updated alt text", body["description"])
	})

	t.Run("success focus returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/media/"+mediaID, strings.NewReader("focus=-0.42,0.69"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", mediaID)
		rec := httptest.NewRecorder()
		handler.PUTMedia(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, mediaID, body["id"])
	})

	t.Run("already attached returns 422", func(t *testing.T) {
		uploadResult2, err := mediaSvc.Upload(ctx, acc.ID, bytes.NewReader(testutil.MinimalJPEG(t)), "image/jpeg", nil)
		require.NoError(t, err)
		attachedID := uploadResult2.Attachment.ID
		statusID := "01HZXY00000000000000000001"
		uploadResult2.Attachment.StatusID = &statusID

		req := httptest.NewRequest(http.MethodPut, "/api/v1/media/"+attachedID, strings.NewReader("description=no"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", attachedID)
		rec := httptest.NewRecorder()
		handler.PUTMedia(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})
}
