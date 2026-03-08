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
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())
	handler := NewStatusesHandler(accountSvc, statusSvc, "example.com", nil, nil)

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

	t.Run("valid JSON creates status and returns 201", func(t *testing.T) {
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
		assert.Equal(t, http.StatusCreated, rec.Code)
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
		assert.Equal(t, http.StatusCreated, rec.Code)
		var statusBody map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&statusBody))
		assert.Contains(t, statusBody["content"], "Form post content")
	})

	t.Run("scheduled_at in future returns ScheduledStatus", func(t *testing.T) {
		acc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username:     "scheduler",
			Email:        "scheduler@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
		})
		require.NoError(t, err)
		scheduledAt := time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339)
		body := bytes.NewBufferString(`{"status":"Scheduled post","scheduled_at":"` + scheduledAt + `"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.POSTStatuses(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var out map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&out))
		assert.NotEmpty(t, out["id"])
		assert.Equal(t, scheduledAt, out["scheduled_at"])
		params, ok := out["params"].(map[string]any)
		require.True(t, ok, "params should be object")
		assert.Equal(t, "Scheduled post", params["text"])
		assert.Contains(t, params, "application_id", "params should include Mastodon compatibility keys")
		assert.Contains(t, params, "poll", "params should include poll (null)")
		assert.Contains(t, params, "with_rate_limit", "params should include with_rate_limit")
	})

	t.Run("scheduled_at in past returns 422", func(t *testing.T) {
		acc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username:     "past",
			Email:        "past@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
		})
		require.NoError(t, err)
		past := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339)
		body := bytes.NewBufferString(`{"status":"Past post","scheduled_at":"` + past + `"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.POSTStatuses(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("create with poll returns status with embedded poll", func(t *testing.T) {
		acc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username:     "pollster",
			Email:        "pollster@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
		})
		require.NoError(t, err)
		body := bytes.NewBufferString(`{"status":"What do you think?","poll":{"options":["Yes","No"],"expires_in":3600,"multiple":false}}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.POSTStatuses(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code)
		var out map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&out))
		assert.NotEmpty(t, out["id"])
		poll, ok := out["poll"].(map[string]any)
		require.True(t, ok, "response should include poll")
		assert.Equal(t, false, poll["multiple"])
		assert.InDelta(t, 0.0, poll["votes_count"].(float64), 0.01)
		options, ok := poll["options"].([]any)
		require.True(t, ok)
		require.Len(t, options, 2)
		assert.Equal(t, "Yes", (options[0].(map[string]any))["title"])
		assert.Equal(t, "No", (options[1].(map[string]any))["title"])
	})
}

func TestStatusesHandler_Create_account_without_user_returns_401(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())
	handler := NewStatusesHandler(accountSvc, statusSvc, "example.com", nil, nil)

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
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())
	handler := NewStatusesHandler(accountSvc, statusSvc, "example.com", nil, nil)

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
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())
	handler := NewStatusesHandler(accountSvc, statusSvc, "example.com", nil, nil)

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

func TestStatusesHandler_POSTPin(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())
	handler := NewStatusesHandler(accountSvc, statusSvc, "example.com", nil, nil)

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
		Text:       testutil.StrPtr("pin me"),
		Content:    testutil.StrPtr("<p>pin me</p>"),
		Visibility: domain.VisibilityPublic,
		APID:       "https://example.com/statuses/" + statusID,
		Local:      true,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses/"+statusID+"/pin", nil)
		req = testutil.AddChiURLParam(req, "id", statusID)
		rec := httptest.NewRecorder()
		handler.POSTPin(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("pin own public status returns 200 and pinned true", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses/"+statusID+"/pin", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", statusID)
		rec := httptest.NewRecorder()
		handler.POSTPin(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.True(t, body["pinned"].(bool))
	})

	t.Run("pin other account status returns 403", func(t *testing.T) {
		otherAcc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username:     "bob",
			Email:        "bob@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
		})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses/"+statusID+"/pin", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), otherAcc))
		req = testutil.AddChiURLParam(req, "id", statusID)
		rec := httptest.NewRecorder()
		handler.POSTPin(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

func TestStatusesHandler_POSTUnpin(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())
	handler := NewStatusesHandler(accountSvc, statusSvc, "example.com", nil, nil)

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
	require.NoError(t, st.CreateAccountPin(ctx, acc.ID, statusID))

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses/"+statusID+"/unpin", nil)
		req = testutil.AddChiURLParam(req, "id", statusID)
		rec := httptest.NewRecorder()
		handler.POSTUnpin(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("unpin own pinned status returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses/"+statusID+"/unpin", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", statusID)
		rec := httptest.NewRecorder()
		handler.POSTUnpin(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.False(t, body["pinned"].(bool))
	})
}

func TestStatusesHandler_ConversationMute(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())
	handler := NewStatusesHandler(accountSvc, statusSvc, "example.com", nil, nil)

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

	t.Run("unauthenticated mute returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses/"+statusID+"/mute", nil)
		req = testutil.AddChiURLParam(req, "id", statusID)
		rec := httptest.NewRecorder()
		handler.POSTMuteConversation(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("mute returns 200 and status with muted true", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses/"+statusID+"/mute", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", statusID)
		rec := httptest.NewRecorder()
		handler.POSTMuteConversation(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.True(t, body["muted"].(bool))
	})

	t.Run("GET status returns muted true when conversation muted", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/statuses/"+statusID, nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", statusID)
		rec := httptest.NewRecorder()
		handler.GETStatuses(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.True(t, body["muted"].(bool))
	})

	t.Run("unmute returns 200 and status with muted false", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses/"+statusID+"/unmute", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", statusID)
		rec := httptest.NewRecorder()
		handler.POSTUnmuteConversation(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.False(t, body["muted"].(bool))
	})

	t.Run("GET status returns muted false after unmute", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/statuses/"+statusID, nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", statusID)
		rec := httptest.NewRecorder()
		handler.GETStatuses(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.False(t, body["muted"].(bool))
	})
}

func TestStatusesHandler_PUTStatuses(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())
	handler := NewStatusesHandler(accountSvc, statusSvc, "example.com", nil, nil)

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
		body := bytes.NewBufferString(`{"status":"updated"}`)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/statuses/"+statusID, body)
		req.Header.Set("Content-Type", "application/json")
		req = testutil.AddChiURLParam(req, "id", statusID)
		rec := httptest.NewRecorder()
		handler.PUTStatuses(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("edit own status returns 200 and updated content", func(t *testing.T) {
		body := bytes.NewBufferString(`{"status":"updated text","spoiler_text":"cw","sensitive":true}`)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/statuses/"+statusID, body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", statusID)
		rec := httptest.NewRecorder()
		handler.PUTStatuses(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var out map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&out))
		assert.Contains(t, out["content"], "updated text")
		assert.True(t, out["sensitive"].(bool))
		assert.Equal(t, "cw", out["spoiler_text"])
	})

	t.Run("edit other account status returns 403", func(t *testing.T) {
		otherAcc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username:     "bob",
			Email:        "bob@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
		})
		require.NoError(t, err)
		body := bytes.NewBufferString(`{"status":"hacked"}`)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/statuses/"+statusID, body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), otherAcc))
		req = testutil.AddChiURLParam(req, "id", statusID)
		rec := httptest.NewRecorder()
		handler.PUTStatuses(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

func TestStatusesHandler_GETStatusHistory(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())
	handler := NewStatusesHandler(accountSvc, statusSvc, "example.com", nil, nil)

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
	require.NoError(t, st.CreateStatusEdit(ctx, store.CreateStatusEditInput{
		ID:             "edit1",
		StatusID:       statusID,
		AccountID:      acc.ID,
		Text:           testutil.StrPtr("first"),
		Content:        testutil.StrPtr("<p>first</p>"),
		ContentWarning: nil,
		Sensitive:      false,
	}))

	t.Run("returns edits in order", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/statuses/"+statusID+"/history", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", statusID)
		rec := httptest.NewRecorder()
		handler.GETStatusHistory(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var edits []map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&edits))
		require.Len(t, edits, 1)
		assert.Equal(t, "<p>first</p>", edits[0]["content"])
	})

	t.Run("no edits returns empty array", func(t *testing.T) {
		statusIDNoEdits := uid.New()
		_, err := st.CreateStatus(ctx, store.CreateStatusInput{
			ID:         statusIDNoEdits,
			URI:        "https://example.com/statuses/" + statusIDNoEdits,
			AccountID:  acc.ID,
			Text:       testutil.StrPtr("never edited"),
			Content:    testutil.StrPtr("<p>never edited</p>"),
			Visibility: domain.VisibilityPublic,
			APID:       "https://example.com/statuses/" + statusIDNoEdits,
			Local:      true,
		})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/statuses/"+statusIDNoEdits+"/history", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", statusIDNoEdits)
		rec := httptest.NewRecorder()
		handler.GETStatusHistory(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var edits []map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&edits))
		assert.Empty(t, edits)
	})
}

func TestStatusesHandler_GETStatusSource(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())
	handler := NewStatusesHandler(accountSvc, statusSvc, "example.com", nil, nil)

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	statusID := uid.New()
	_, err = st.CreateStatus(ctx, store.CreateStatusInput{
		ID:             statusID,
		URI:            "https://example.com/statuses/" + statusID,
		AccountID:      acc.ID,
		Text:           testutil.StrPtr("plain text"),
		Content:        testutil.StrPtr("<p>plain text</p>"),
		ContentWarning: testutil.StrPtr("spoiler"),
		Visibility:     domain.VisibilityPublic,
		APID:           "https://example.com/statuses/" + statusID,
		Local:          true,
	})
	require.NoError(t, err)

	t.Run("returns id text spoiler_text", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/statuses/"+statusID+"/source", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", statusID)
		rec := httptest.NewRecorder()
		handler.GETStatusSource(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var src map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&src))
		assert.Equal(t, statusID, src["id"])
		assert.Equal(t, "plain text", src["text"])
		assert.Equal(t, "spoiler", src["spoiler_text"])
	})
}

func TestStatusesHandler_POSTFavourite(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())
	handler := NewStatusesHandler(accountSvc, statusSvc, "example.com", nil, nil)

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
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())
	handler := NewStatusesHandler(accountSvc, statusSvc, "example.com", nil, nil)

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
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())
	handler := NewStatusesHandler(accountSvc, statusSvc, "example.com", nil, nil)

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

func TestStatusesHandler_GETStatuses_private_returns_404_when_unauthenticated(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())
	handler := NewStatusesHandler(accountSvc, statusSvc, "example.com", nil, nil)

	acc, err := accountSvc.Create(ctx, service.CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	statusID := uid.New()
	content := "<p>private</p>"
	_, err = st.CreateStatus(ctx, store.CreateStatusInput{
		ID:         statusID,
		URI:        "https://example.com/statuses/" + statusID,
		AccountID:  acc.ID,
		Text:       testutil.StrPtr("private"),
		Content:    &content,
		Visibility: domain.VisibilityPrivate,
		APID:       "https://example.com/statuses/" + statusID,
		Local:      true,
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/statuses/"+statusID, nil)
	req = testutil.AddChiURLParam(req, "id", statusID)
	rec := httptest.NewRecorder()
	handler.GETStatuses(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestStatusesHandler_GETFavouritedBy(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())
	handler := NewStatusesHandler(accountSvc, statusSvc, "example.com", nil, nil)

	acc, err := accountSvc.Create(ctx, service.CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	text := "post"
	status, err := statusSvc.Create(ctx, service.CreateStatusInput{AccountID: acc.ID, Text: &text, Visibility: domain.VisibilityPublic})
	require.NoError(t, err)
	statusID := status.ID

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
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())
	handler := NewStatusesHandler(accountSvc, statusSvc, "example.com", nil, nil)

	acc, err := accountSvc.Create(ctx, service.CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	text := "post"
	status, err := statusSvc.Create(ctx, service.CreateStatusInput{AccountID: acc.ID, Text: &text, Visibility: domain.VisibilityPublic})
	require.NoError(t, err)
	statusID := status.ID

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
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())
	handler := NewStatusesHandler(accountSvc, statusSvc, "example.com", cacheStore, nil)

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
	require.Equal(t, http.StatusCreated, rec1.Code)
	var firstResp map[string]any
	require.NoError(t, json.NewDecoder(rec1.Body).Decode(&firstResp))

	body2 := bytes.NewBufferString(`{"status":"idempotent post"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/statuses", body2)
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", "same-key")
	req2 = req2.WithContext(middleware.WithAccount(req2.Context(), acc))
	rec2 := httptest.NewRecorder()
	handler.POSTStatuses(rec2, req2)
	require.Equal(t, http.StatusCreated, rec2.Code)
	var secondResp map[string]any
	require.NoError(t, json.NewDecoder(rec2.Body).Decode(&secondResp))
	assert.Equal(t, firstResp["id"], secondResp["id"], "idempotency key should return cached response with same status id")
	assert.Contains(t, secondResp["content"], "idempotent post")
}

func TestStatusesHandler_POSTStatuses_quote(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())
	handler := NewStatusesHandler(accountSvc, statusSvc, "example.com", nil, nil)

	alice, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	quotedID := uid.New()
	_, err = st.CreateStatus(ctx, store.CreateStatusInput{
		ID:                  quotedID,
		URI:                 "https://example.com/statuses/" + quotedID,
		AccountID:           alice.ID,
		Text:                testutil.StrPtr("original"),
		Content:             testutil.StrPtr("<p>original</p>"),
		Visibility:          domain.VisibilityPublic,
		QuoteApprovalPolicy: domain.QuotePolicyPublic,
		APID:                "https://example.com/statuses/" + quotedID,
		Local:               true,
	})
	require.NoError(t, err)

	bob, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "bob",
		Email:        "bob@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("success creates quote and returns 201 with quote_approval", func(t *testing.T) {
		body := bytes.NewBufferString(`{"status":"quoting you","quoted_status_id":"` + quotedID + `"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), bob))
		rec := httptest.NewRecorder()
		handler.POSTStatuses(rec, req)
		assert.Equal(t, http.StatusCreated, rec.Code)
		var out map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&out))
		assert.Contains(t, out["content"], "quoting you")
		quoteApproval, ok := out["quote_approval"].(map[string]any)
		require.True(t, ok, "response should include quote_approval")
		assert.Equal(t, "accepted", quoteApproval["state"])
	})

	t.Run("quoted_status_id with media_ids returns 422", func(t *testing.T) {
		body := bytes.NewBufferString(`{"status":"quote with media","quoted_status_id":"` + quotedID + `","media_ids":["01HXYZ"]}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), bob))
		rec := httptest.NewRecorder()
		handler.POSTStatuses(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("quoted_status_id with poll returns 422", func(t *testing.T) {
		body := bytes.NewBufferString(`{"status":"quote with poll","quoted_status_id":"` + quotedID + `","poll":{"options":["A","B"],"expires_in":300,"multiple":false}}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), bob))
		rec := httptest.NewRecorder()
		handler.POSTStatuses(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("quote_approval_policy nobody returns 403 for non-author", func(t *testing.T) {
		nobodyID := uid.New()
		_, err := st.CreateStatus(ctx, store.CreateStatusInput{
			ID:                  nobodyID,
			URI:                 "https://example.com/statuses/" + nobodyID,
			AccountID:           alice.ID,
			Text:                testutil.StrPtr("nobody may quote"),
			Content:             testutil.StrPtr("<p>nobody</p>"),
			Visibility:          domain.VisibilityPublic,
			QuoteApprovalPolicy: domain.QuotePolicyNobody,
			APID:                "https://example.com/statuses/" + nobodyID,
			Local:               true,
		})
		require.NoError(t, err)
		body := bytes.NewBufferString(`{"status":"trying to quote","quoted_status_id":"` + nobodyID + `"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), bob))
		rec := httptest.NewRecorder()
		handler.POSTStatuses(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("nonexistent quoted_status_id returns 404", func(t *testing.T) {
		body := bytes.NewBufferString(`{"status":"quote missing","quoted_status_id":"01H0000000000000000000000"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), bob))
		rec := httptest.NewRecorder()
		handler.POSTStatuses(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestStatusesHandler_GETQuotes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())
	handler := NewStatusesHandler(accountSvc, statusSvc, "example.com", nil, nil)

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	quotedID := uid.New()
	_, err = st.CreateStatus(ctx, store.CreateStatusInput{
		ID:         quotedID,
		URI:        "https://example.com/statuses/" + quotedID,
		AccountID:  acc.ID,
		Text:       testutil.StrPtr("original"),
		Content:    testutil.StrPtr("<p>original</p>"),
		Visibility: domain.VisibilityPublic,
		APID:       "https://example.com/statuses/" + quotedID,
		Local:      true,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/statuses/"+quotedID+"/quotes", nil)
		req = testutil.AddChiURLParam(req, "id", quotedID)
		rec := httptest.NewRecorder()
		handler.GETQuotes(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("authenticated returns 200 and empty list when no quotes", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/statuses/"+quotedID+"/quotes", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", quotedID)
		rec := httptest.NewRecorder()
		handler.GETQuotes(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var out []any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&out))
		assert.Empty(t, out)
	})

	t.Run("after creating quote returns 200 with one status", func(t *testing.T) {
		body := bytes.NewBufferString(`{"status":"a quote","quoted_status_id":"` + quotedID + `"}`)
		postReq := httptest.NewRequest(http.MethodPost, "/api/v1/statuses", body)
		postReq.Header.Set("Content-Type", "application/json")
		postReq = postReq.WithContext(middleware.WithAccount(postReq.Context(), acc))
		postRec := httptest.NewRecorder()
		handler.POSTStatuses(postRec, postReq)
		require.Equal(t, http.StatusCreated, postRec.Code)
		var postOut map[string]any
		require.NoError(t, json.NewDecoder(postRec.Body).Decode(&postOut))
		quotingID, _ := postOut["id"].(string)
		require.NotEmpty(t, quotingID)

		getReq := httptest.NewRequest(http.MethodGet, "/api/v1/statuses/"+quotedID+"/quotes", nil)
		getReq = getReq.WithContext(middleware.WithAccount(getReq.Context(), acc))
		getReq = testutil.AddChiURLParam(getReq, "id", quotedID)
		getRec := httptest.NewRecorder()
		handler.GETQuotes(getRec, getReq)
		assert.Equal(t, http.StatusOK, getRec.Code)
		var list []any
		require.NoError(t, json.NewDecoder(getRec.Body).Decode(&list))
		require.Len(t, list, 1)
		assert.Equal(t, quotingID, (list[0].(map[string]any))["id"])
	})

	t.Run("nonexistent status returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/statuses/01H0000000000000000000000/quotes", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", "01H0000000000000000000000")
		rec := httptest.NewRecorder()
		handler.GETQuotes(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestStatusesHandler_POSTRevokeQuote(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())
	handler := NewStatusesHandler(accountSvc, statusSvc, "example.com", nil, nil)

	alice, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	bob, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "bob",
		Email:        "bob@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	quotedID := uid.New()
	_, err = st.CreateStatus(ctx, store.CreateStatusInput{
		ID:         quotedID,
		URI:        "https://example.com/statuses/" + quotedID,
		AccountID:  alice.ID,
		Text:       testutil.StrPtr("alice post"),
		Content:    testutil.StrPtr("<p>alice</p>"),
		Visibility: domain.VisibilityPublic,
		APID:       "https://example.com/statuses/" + quotedID,
		Local:      true,
	})
	require.NoError(t, err)
	quotingID := uid.New()
	_, err = st.CreateStatus(ctx, store.CreateStatusInput{
		ID:                  quotingID,
		URI:                 "https://example.com/statuses/" + quotingID,
		AccountID:           bob.ID,
		Text:                testutil.StrPtr("bob quote"),
		Content:             testutil.StrPtr("<p>bob quote</p>"),
		Visibility:          domain.VisibilityPublic,
		QuotedStatusID:      &quotedID,
		QuoteApprovalPolicy: domain.QuotePolicyPublic,
		APID:                "https://example.com/statuses/" + quotingID,
		Local:               true,
	})
	require.NoError(t, err)
	require.NoError(t, st.CreateQuoteApproval(ctx, quotingID, quotedID))
	require.NoError(t, st.IncrementQuotesCount(ctx, quotedID))

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses/"+quotedID+"/quotes/"+quotingID+"/revoke", nil)
		req = testutil.AddChiURLParams(req, map[string]string{"id": quotedID, "quoting_status_id": quotingID})
		rec := httptest.NewRecorder()
		handler.POSTRevokeQuote(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("non-owner of quoted status returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses/"+quotedID+"/quotes/"+quotingID+"/revoke", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), bob))
		req = testutil.AddChiURLParams(req, map[string]string{"id": quotedID, "quoting_status_id": quotingID})
		rec := httptest.NewRecorder()
		handler.POSTRevokeQuote(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("owner of quoted status revokes returns 204", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses/"+quotedID+"/quotes/"+quotingID+"/revoke", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), alice))
		req = testutil.AddChiURLParams(req, map[string]string{"id": quotedID, "quoting_status_id": quotingID})
		rec := httptest.NewRecorder()
		handler.POSTRevokeQuote(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("nonexistent quoting_status_id returns 404", func(t *testing.T) {
		fakeQuotingID := "01H0000000000000000000000"
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses/"+quotedID+"/quotes/"+fakeQuotingID+"/revoke", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), alice))
		req = testutil.AddChiURLParams(req, map[string]string{"id": quotedID, "quoting_status_id": fakeQuotingID})
		rec := httptest.NewRecorder()
		handler.POSTRevokeQuote(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestStatusesHandler_PUTInteractionPolicy(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())
	handler := NewStatusesHandler(accountSvc, statusSvc, "example.com", nil, nil)

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	otherAcc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "bob",
		Email:        "bob@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	statusID := uid.New()
	_, err = st.CreateStatus(ctx, store.CreateStatusInput{
		ID:         statusID,
		URI:        "https://example.com/statuses/" + statusID,
		AccountID:  acc.ID,
		Text:       testutil.StrPtr("my status"),
		Content:    testutil.StrPtr("<p>my status</p>"),
		Visibility: domain.VisibilityPublic,
		APID:       "https://example.com/statuses/" + statusID,
		Local:      true,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		body := bytes.NewBufferString(`{"quote_approval_policy":"followers"}`)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/statuses/"+statusID+"/interaction_policy", body)
		req.Header.Set("Content-Type", "application/json")
		req = testutil.AddChiURLParam(req, "id", statusID)
		rec := httptest.NewRecorder()
		handler.PUTInteractionPolicy(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("owner updates policy returns 200 with quote_approval_policy in response", func(t *testing.T) {
		body := bytes.NewBufferString(`{"quote_approval_policy":"followers"}`)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/statuses/"+statusID+"/interaction_policy", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", statusID)
		rec := httptest.NewRecorder()
		handler.PUTInteractionPolicy(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var out map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&out))
		assert.Equal(t, statusID, out["id"], "response should be the updated status")
		assert.Equal(t, "followers", out["quote_approval_policy"], "response should include updated policy")
	})

	t.Run("empty quote_approval_policy returns 422", func(t *testing.T) {
		body := bytes.NewBufferString(`{"quote_approval_policy":"   "}`)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/statuses/"+statusID+"/interaction_policy", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", statusID)
		rec := httptest.NewRecorder()
		handler.PUTInteractionPolicy(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("non-owner returns 403", func(t *testing.T) {
		body := bytes.NewBufferString(`{"quote_approval_policy":"nobody"}`)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/statuses/"+statusID+"/interaction_policy", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), otherAcc))
		req = testutil.AddChiURLParam(req, "id", statusID)
		rec := httptest.NewRecorder()
		handler.PUTInteractionPolicy(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("invalid policy returns 422", func(t *testing.T) {
		body := bytes.NewBufferString(`{"quote_approval_policy":"invalid"}`)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/statuses/"+statusID+"/interaction_policy", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", statusID)
		rec := httptest.NewRecorder()
		handler.PUTInteractionPolicy(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("nonexistent status returns 404", func(t *testing.T) {
		body := bytes.NewBufferString(`{"quote_approval_policy":"public"}`)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/statuses/01H0000000000000000000000/interaction_policy", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", "01H0000000000000000000000")
		rec := httptest.NewRecorder()
		handler.PUTInteractionPolicy(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}
