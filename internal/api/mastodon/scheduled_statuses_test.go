package mastodon

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/chairswithlegs/monstera/internal/uid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScheduledStatusesHandler_GETScheduledStatuses(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	statusSvc := service.NewStatusService(st, "https://example.com", "example.com", 500)
	statusWriteSvc := service.NewStatusWriteService(st, statusSvc, service.NewConversationService(st, statusSvc), "https://example.com", "example.com", 500)
	scheduledSvc := service.NewScheduledStatusService(st, statusWriteSvc)
	handler := NewScheduledStatusesHandler(statusSvc, scheduledSvc, "example.com")

	acc, err := service.NewAccountService(st, "https://example.com").Register(ctx, service.RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	paramsJSON := []byte(`{"text":"scheduled","visibility":"public","media_ids":[]}`)
	scheduledAt := time.Now().Add(1 * time.Hour)
	_, err = st.CreateScheduledStatus(ctx, store.CreateScheduledStatusInput{
		ID:          uid.New(),
		AccountID:   acc.ID,
		Params:      paramsJSON,
		ScheduledAt: scheduledAt,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/scheduled_statuses", nil)
		rec := httptest.NewRecorder()
		handler.GETScheduledStatuses(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("authenticated returns list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/scheduled_statuses", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.GETScheduledStatuses(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var list []map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&list))
		assert.Len(t, list, 1)
		assert.NotEmpty(t, list[0]["id"])
		assert.NotEmpty(t, list[0]["scheduled_at"])
	})

	t.Run("Link header emitted when limit reached", func(t *testing.T) {
		// Create a second scheduled status so two exist; request with limit=1.
		_, err = st.CreateScheduledStatus(ctx, store.CreateScheduledStatusInput{
			ID:          uid.New(),
			AccountID:   acc.ID,
			Params:      paramsJSON,
			ScheduledAt: scheduledAt.Add(time.Minute),
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/scheduled_statuses?limit=1", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.GETScheduledStatuses(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var list []map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&list))
		assert.Len(t, list, 1)
		assert.Contains(t, rec.Header().Get("Link"), `rel="next"`)
	})
}

func TestScheduledStatusesHandler_GETScheduledStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	statusSvc := service.NewStatusService(st, "https://example.com", "example.com", 500)
	statusWriteSvc := service.NewStatusWriteService(st, statusSvc, service.NewConversationService(st, statusSvc), "https://example.com", "example.com", 500)
	scheduledSvc := service.NewScheduledStatusService(st, statusWriteSvc)
	handler := NewScheduledStatusesHandler(statusSvc, scheduledSvc, "example.com")

	acc, err := service.NewAccountService(st, "https://example.com").Register(ctx, service.RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	schedID := uid.New()
	paramsJSON := []byte(`{"text":"one"}`)
	scheduledAt := time.Now().Add(1 * time.Hour)
	_, err = st.CreateScheduledStatus(ctx, store.CreateScheduledStatusInput{
		ID:          schedID,
		AccountID:   acc.ID,
		Params:      paramsJSON,
		ScheduledAt: scheduledAt,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/scheduled_statuses/"+schedID, nil)
		req = testutil.AddChiURLParam(req, "id", schedID)
		rec := httptest.NewRecorder()
		handler.GETScheduledStatus(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("own scheduled status returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/scheduled_statuses/"+schedID, nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", schedID)
		rec := httptest.NewRecorder()
		handler.GETScheduledStatus(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var out map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&out))
		assert.Equal(t, schedID, out["id"])
	})

	t.Run("other account scheduled status returns 404", func(t *testing.T) {
		otherAcc, err := service.NewAccountService(st, "https://example.com").Register(ctx, service.RegisterInput{
			Username: "bob",
			Email:    "bob@example.com",
			Password: "hash",
			Role:     domain.RoleUser,
		})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/scheduled_statuses/"+schedID, nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), otherAcc))
		req = testutil.AddChiURLParam(req, "id", schedID)
		rec := httptest.NewRecorder()
		handler.GETScheduledStatus(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestScheduledStatusesHandler_PUTScheduledStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	statusSvc := service.NewStatusService(st, "https://example.com", "example.com", 500)
	statusWriteSvc := service.NewStatusWriteService(st, statusSvc, service.NewConversationService(st, statusSvc), "https://example.com", "example.com", 500)
	scheduledSvc := service.NewScheduledStatusService(st, statusWriteSvc)
	handler := NewScheduledStatusesHandler(statusSvc, scheduledSvc, "example.com")

	acc, err := service.NewAccountService(st, "https://example.com").Register(ctx, service.RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	schedID := uid.New()
	paramsJSON := []byte(`{"text":"reschedule me"}`)
	scheduledAt := time.Now().Add(2 * time.Hour)
	_, err = st.CreateScheduledStatus(ctx, store.CreateScheduledStatusInput{
		ID:          schedID,
		AccountID:   acc.ID,
		Params:      paramsJSON,
		ScheduledAt: scheduledAt,
	})
	require.NoError(t, err)

	newScheduledAt := time.Now().Add(3 * time.Hour).UTC().Format(time.RFC3339)

	t.Run("valid reschedule returns 200", func(t *testing.T) {
		body := bytes.NewBufferString(`{"scheduled_at":"` + newScheduledAt + `"}`)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/scheduled_statuses/"+schedID, body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", schedID)
		rec := httptest.NewRecorder()
		handler.PUTScheduledStatus(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var out map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&out))
		assert.Equal(t, newScheduledAt, out["scheduled_at"])
	})

	t.Run("reschedule with updated params returns 200", func(t *testing.T) {
		schedID3 := uid.New()
		_, err = st.CreateScheduledStatus(ctx, store.CreateScheduledStatusInput{
			ID:          schedID3,
			AccountID:   acc.ID,
			Params:      []byte(`{"text":"original"}`),
			ScheduledAt: time.Now().Add(1 * time.Hour),
		})
		require.NoError(t, err)
		body := bytes.NewBufferString(`{"scheduled_at":"` + newScheduledAt + `","params":{"text":"updated text","language":"en"}}`)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/scheduled_statuses/"+schedID3, body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", schedID3)
		rec := httptest.NewRecorder()
		handler.PUTScheduledStatus(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var out map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&out))
		assert.Equal(t, newScheduledAt, out["scheduled_at"])
		params, ok := out["params"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "updated text", params["text"])
	})

	t.Run("scheduled_at in past returns 422", func(t *testing.T) {
		schedID2 := uid.New()
		_, err = st.CreateScheduledStatus(ctx, store.CreateScheduledStatusInput{
			ID:          schedID2,
			AccountID:   acc.ID,
			Params:      paramsJSON,
			ScheduledAt: time.Now().Add(1 * time.Hour),
		})
		require.NoError(t, err)
		past := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339)
		body := bytes.NewBufferString(`{"scheduled_at":"` + past + `"}`)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/scheduled_statuses/"+schedID2, body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", schedID2)
		rec := httptest.NewRecorder()
		handler.PUTScheduledStatus(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})
}

func TestScheduledStatusesHandler_DELETEScheduledStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	statusSvc := service.NewStatusService(st, "https://example.com", "example.com", 500)
	statusWriteSvc := service.NewStatusWriteService(st, statusSvc, service.NewConversationService(st, statusSvc), "https://example.com", "example.com", 500)
	scheduledSvc := service.NewScheduledStatusService(st, statusWriteSvc)
	handler := NewScheduledStatusesHandler(statusSvc, scheduledSvc, "example.com")

	acc, err := service.NewAccountService(st, "https://example.com").Register(ctx, service.RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	schedID := uid.New()
	_, err = st.CreateScheduledStatus(ctx, store.CreateScheduledStatusInput{
		ID:          schedID,
		AccountID:   acc.ID,
		Params:      []byte(`{"text":"delete me"}`),
		ScheduledAt: time.Now().Add(1 * time.Hour),
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/scheduled_statuses/"+schedID, nil)
		req = testutil.AddChiURLParam(req, "id", schedID)
		rec := httptest.NewRecorder()
		handler.DELETEScheduledStatus(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("delete own returns 204", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/scheduled_statuses/"+schedID, nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", schedID)
		rec := httptest.NewRecorder()
		handler.DELETEScheduledStatus(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
		_, err := st.GetScheduledStatusByID(ctx, schedID)
		require.Error(t, err)
	})
}
