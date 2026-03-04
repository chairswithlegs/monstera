package mastodon

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/cache"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/events"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/chairswithlegs/monstera/internal/uid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusesHandler_Create(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, "https://example.com", "example.com", 500, slog.Default())
	handler := NewStatusesHandler(accountSvc, statusSvc, "example.com", nil)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		body := bytes.NewBufferString(`{"status":"hello world"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.POSTStatuses(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("blank status returns 422", func(t *testing.T) {
		acc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username:     "alice",
			Email:        "alice@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
		})
		require.NoError(t, err)

		body := bytes.NewBufferString(`{"status":"   "}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.POSTStatuses(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("valid JSON creates status and returns 200", func(t *testing.T) {
		acc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username:     "bob",
			Email:        "bob@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
		})
		require.NoError(t, err)

		body := bytes.NewBufferString(`{"status":"Hello from API test"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.POSTStatuses(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var statusBody map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&statusBody))
		assert.Contains(t, statusBody["content"], "Hello from API test")
		assert.Equal(t, "bob", statusBody["account"].(map[string]any)["username"])
	})

	t.Run("valid form body creates status", func(t *testing.T) {
		acc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username:     "charlie",
			Email:        "charlie@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
		})
		require.NoError(t, err)

		formBody := bytes.NewBufferString("status=Form+post+content")
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses", formBody)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.POSTStatuses(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var statusBody map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&statusBody))
		assert.Contains(t, statusBody["content"], "Form post content")
	})

}

func TestStatusesHandler_Create_account_without_user_returns_401(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, "https://example.com", "example.com", 500, slog.Default())
	handler := NewStatusesHandler(accountSvc, statusSvc, "example.com", nil)

	acc, err := accountSvc.Create(ctx, service.CreateAccountInput{Username: "nouser"})
	require.NoError(t, err)

	body := bytes.NewBufferString(`{"status":"orphan account post"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(middleware.WithAccount(req.Context(), acc))
	rec := httptest.NewRecorder()
	handler.POSTStatuses(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestParseCreateStatusRequest(t *testing.T) {
	t.Parallel()

	t.Run("invalid JSON returns error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses", bytes.NewBufferString(`{invalid`))
		req.Header.Set("Content-Type", "application/json")
		_, err := parseCreateStatusRequest(req)
		assert.ErrorIs(t, err, api.ErrUnprocessable)
	})

	t.Run("empty status returns error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses", bytes.NewBufferString(`{"status":""}`))
		req.Header.Set("Content-Type", "application/json")
		_, err := parseCreateStatusRequest(req)
		assert.ErrorIs(t, err, api.ErrUnprocessable)
	})

	t.Run("valid JSON parses fields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses", bytes.NewBufferString(`{"status":"hi","visibility":"private","spoiler_text":"cw","sensitive":true,"language":"en"}`))
		req.Header.Set("Content-Type", "application/json")
		parsed, err := parseCreateStatusRequest(req)
		require.NoError(t, err)
		assert.Equal(t, "hi", parsed.Status)
		assert.Equal(t, "private", parsed.Visibility)
		assert.Equal(t, "cw", parsed.SpoilerText)
		assert.True(t, parsed.Sensitive)
		assert.Equal(t, "en", parsed.Language)
	})
}

func TestStatusesHandler_POSTReblog(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, "https://example.com", "example.com", 500, slog.Default())
	handler := NewStatusesHandler(accountSvc, statusSvc, "example.com", nil)

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	statusID := uid.New()
	_, err = st.CreateStatus(ctx, store.CreateStatusInput{
		ID:         statusID,
		URI:        "https://example.com/statuses/" + statusID,
		AccountID:  acc.ID,
		Text:       testutil.StrPtr("original"),
		Content:    testutil.StrPtr("<p>original</p>"),
		Visibility: domain.VisibilityPublic,
		APID:       "https://example.com/statuses/" + statusID,
		Local:      true,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses/"+statusID+"/reblog", nil)
		req = testutil.AddChiURLParam(req, "id", statusID)
		rec := httptest.NewRecorder()
		handler.POSTReblog(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("authenticated returns 200 and reblog status", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses/"+statusID+"/reblog", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", statusID)
		rec := httptest.NewRecorder()
		handler.POSTReblog(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.NotEmpty(t, body["id"])
		assert.NotNil(t, body["reblog"], "boost should include nested reblog (original status)")
	})
}

func TestStatusesHandler_POSTUnreblog(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, "https://example.com", "example.com", 500, slog.Default())
	handler := NewStatusesHandler(accountSvc, statusSvc, "example.com", nil)

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	statusID := uid.New()
	_, err = st.CreateStatus(ctx, store.CreateStatusInput{
		ID:         statusID,
		URI:        "https://example.com/statuses/" + statusID,
		AccountID:  acc.ID,
		Text:       testutil.StrPtr("post"),
		Content:    testutil.StrPtr("<p>post</p>"),
		Visibility: domain.VisibilityPublic,
		APID:       "https://example.com/statuses/" + statusID,
		Local:      true,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses/"+statusID+"/unreblog", nil)
		req = testutil.AddChiURLParam(req, "id", statusID)
		rec := httptest.NewRecorder()
		handler.POSTUnreblog(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("authenticated returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses/"+statusID+"/unreblog", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", statusID)
		rec := httptest.NewRecorder()
		handler.POSTUnreblog(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestStatusesHandler_POSTFavourite(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, "https://example.com", "example.com", 500, slog.Default())
	handler := NewStatusesHandler(accountSvc, statusSvc, "example.com", nil)

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	statusID := uid.New()
	_, err = st.CreateStatus(ctx, store.CreateStatusInput{
		ID:         statusID,
		URI:        "https://example.com/statuses/" + statusID,
		AccountID:  acc.ID,
		Text:       testutil.StrPtr("post"),
		Content:    testutil.StrPtr("<p>post</p>"),
		Visibility: domain.VisibilityPublic,
		APID:       "https://example.com/statuses/" + statusID,
		Local:      true,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses/"+statusID+"/favourite", nil)
		req = testutil.AddChiURLParam(req, "id", statusID)
		rec := httptest.NewRecorder()
		handler.POSTFavourite(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("authenticated when favourite fails returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses/"+statusID+"/favourite", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", statusID)
		rec := httptest.NewRecorder()
		handler.POSTFavourite(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestStatusesHandler_POSTUnfavourite(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, "https://example.com", "example.com", 500, slog.Default())
	handler := NewStatusesHandler(accountSvc, statusSvc, "example.com", nil)

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	statusID := uid.New()
	_, err = st.CreateStatus(ctx, store.CreateStatusInput{
		ID:         statusID,
		URI:        "https://example.com/statuses/" + statusID,
		AccountID:  acc.ID,
		Text:       testutil.StrPtr("post"),
		Content:    testutil.StrPtr("<p>post</p>"),
		Visibility: domain.VisibilityPublic,
		APID:       "https://example.com/statuses/" + statusID,
		Local:      true,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses/"+statusID+"/unfavourite", nil)
		req = testutil.AddChiURLParam(req, "id", statusID)
		rec := httptest.NewRecorder()
		handler.POSTUnfavourite(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("authenticated returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses/"+statusID+"/unfavourite", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", statusID)
		rec := httptest.NewRecorder()
		handler.POSTUnfavourite(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestStatusesHandler_GETContext(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, "https://example.com", "example.com", 500, slog.Default())
	handler := NewStatusesHandler(accountSvc, statusSvc, "example.com", nil)

	statusID := uid.New()
	acc, err := accountSvc.Create(ctx, service.CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	_, err = st.CreateStatus(ctx, store.CreateStatusInput{
		ID:         statusID,
		URI:        "https://example.com/statuses/" + statusID,
		AccountID:  acc.ID,
		Text:       testutil.StrPtr("post"),
		Content:    testutil.StrPtr("<p>post</p>"),
		Visibility: domain.VisibilityPublic,
		APID:       "https://example.com/statuses/" + statusID,
		Local:      true,
	})
	require.NoError(t, err)

	t.Run("returns 200 with ancestors and descendants", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/statuses/"+statusID+"/context", nil)
		req = testutil.AddChiURLParam(req, "id", statusID)
		rec := httptest.NewRecorder()
		handler.GETContext(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body["ancestors"])
		assert.Empty(t, body["descendants"])
	})
}

func TestStatusesHandler_GETFavouritedBy(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, "https://example.com", "example.com", 500, slog.Default())
	handler := NewStatusesHandler(accountSvc, statusSvc, "example.com", nil)

	statusID := "01status"
	t.Run("returns 200 and empty list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/statuses/"+statusID+"/favourited_by", nil)
		req = testutil.AddChiURLParam(req, "id", statusID)
		rec := httptest.NewRecorder()
		handler.GETFavouritedBy(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body)
	})
}

func TestStatusesHandler_GETRebloggedBy(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, "https://example.com", "example.com", 500, slog.Default())
	handler := NewStatusesHandler(accountSvc, statusSvc, "example.com", nil)

	statusID := "01status"
	t.Run("returns 200 and empty list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/statuses/"+statusID+"/reblogged_by", nil)
		req = testutil.AddChiURLParam(req, "id", statusID)
		rec := httptest.NewRecorder()
		handler.GETRebloggedBy(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body)
	})
}

// syncMapCache is a synchronous cache.Store for tests (e.g. idempotency).
type syncMapCache map[string][]byte

func (c syncMapCache) Get(_ context.Context, key string) ([]byte, error) {
	if b, ok := c[key]; ok {
		return b, nil
	}
	return nil, cache.ErrCacheMiss
}

func (c syncMapCache) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	c[key] = value
	return nil
}

func (c syncMapCache) Delete(_ context.Context, key string) error {
	delete(c, key)
	return nil
}

func (c syncMapCache) Exists(_ context.Context, key string) (bool, error) {
	_, ok := c[key]
	return ok, nil
}

func (c syncMapCache) Close() error { return nil }

func TestStatusesHandler_POSTStatuses_idempotency(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cacheStore := make(syncMapCache)
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, "https://example.com", "example.com", 500, slog.Default())
	handler := NewStatusesHandler(accountSvc, statusSvc, "example.com", cacheStore)

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	body := bytes.NewBufferString(`{"status":"idempotent post"}`)
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/statuses", body)
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Idempotency-Key", "same-key")
	req1 = req1.WithContext(middleware.WithAccount(req1.Context(), acc))
	rec1 := httptest.NewRecorder()
	handler.POSTStatuses(rec1, req1)
	require.Equal(t, http.StatusOK, rec1.Code)
	var firstResp map[string]any
	require.NoError(t, json.NewDecoder(rec1.Body).Decode(&firstResp))

	body2 := bytes.NewBufferString(`{"status":"idempotent post"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/statuses", body2)
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", "same-key")
	req2 = req2.WithContext(middleware.WithAccount(req2.Context(), acc))
	rec2 := httptest.NewRecorder()
	handler.POSTStatuses(rec2, req2)
	require.Equal(t, http.StatusOK, rec2.Code)
	var secondResp map[string]any
	require.NoError(t, json.NewDecoder(rec2.Body).Decode(&secondResp))
	assert.Equal(t, firstResp["id"], secondResp["id"], "idempotency key should return cached response with same status id")
	assert.Contains(t, secondResp["content"], "idempotent post")
}
