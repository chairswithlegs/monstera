package mastodon

import (
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

func TestAnnouncementsHandler_GETAnnouncements(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	announcementSvc := service.NewAnnouncementService(st)
	handler := NewAnnouncementsHandler(announcementSvc)

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/announcements", nil)
		rec := httptest.NewRecorder()
		handler.GETAnnouncements(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("authenticated empty list returns 200 and []", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/announcements", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.GETAnnouncements(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body)
	})

	t.Run("authenticated with active announcement returns list", func(t *testing.T) {
		now := time.Now().UTC()
		_, err := st.CreateAnnouncement(ctx, store.CreateAnnouncementInput{
			ID:          uid.New(),
			Content:     "<p>Test announcement</p>",
			AllDay:      false,
			PublishedAt: now,
		})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/announcements", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.GETAnnouncements(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		require.Len(t, body, 1)
		assert.Equal(t, "<p>Test announcement</p>", body[0]["content"])
		assert.False(t, body[0]["read"].(bool))
		assert.NotEmpty(t, body[0]["id"])
	})
}

func TestAnnouncementsHandler_POSTDismissAnnouncement(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	announcementSvc := service.NewAnnouncementService(st)
	handler := NewAnnouncementsHandler(announcementSvc)

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	annID := uid.New()
	_, err = st.CreateAnnouncement(ctx, store.CreateAnnouncementInput{
		ID:          annID,
		Content:     "<p>Dismiss me</p>",
		PublishedAt: time.Now().UTC(),
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/announcements/"+annID+"/dismiss", nil)
		req = testutil.AddChiURLParam(req, "id", annID)
		rec := httptest.NewRecorder()
		handler.POSTDismissAnnouncement(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("dismiss returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/announcements/"+annID+"/dismiss", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", annID)
		rec := httptest.NewRecorder()
		handler.POSTDismissAnnouncement(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("dismiss then list shows read true", func(t *testing.T) {
		list, err := announcementSvc.ListActive(ctx, acc.ID)
		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.True(t, list[0].Read)
	})

	t.Run("dismiss nonexistent returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/announcements/nonexistent/dismiss", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", "nonexistent")
		rec := httptest.NewRecorder()
		handler.POSTDismissAnnouncement(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestAnnouncementsHandler_Reactions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	announcementSvc := service.NewAnnouncementService(st)
	handler := NewAnnouncementsHandler(announcementSvc)

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	annID := uid.New()
	_, err = st.CreateAnnouncement(ctx, store.CreateAnnouncementInput{
		ID:          annID,
		Content:     "<p>React to me</p>",
		PublishedAt: time.Now().UTC(),
	})
	require.NoError(t, err)

	t.Run("PUT reaction returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/announcements/"+annID+"/reactions/👍", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParams(req, map[string]string{"id": annID, "name": "👍"})
		rec := httptest.NewRecorder()
		handler.PUTAnnouncementReaction(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("list includes reaction with me true", func(t *testing.T) {
		list, err := announcementSvc.ListActive(ctx, acc.ID)
		require.NoError(t, err)
		require.Len(t, list, 1)
		require.Len(t, list[0].Reactions, 1)
		assert.Equal(t, "👍", list[0].Reactions[0].Name)
		assert.Equal(t, 1, list[0].Reactions[0].Count)
		assert.True(t, list[0].Reactions[0].Me)
	})

	t.Run("DELETE reaction returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/announcements/"+annID+"/reactions/👍", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParams(req, map[string]string{"id": annID, "name": "👍"})
		rec := httptest.NewRecorder()
		handler.DELETEAnnouncementReaction(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("after delete list has no reactions", func(t *testing.T) {
		list, err := announcementSvc.ListActive(ctx, acc.ID)
		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.Empty(t, list[0].Reactions)
	})

	t.Run("PUT reaction on nonexistent returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/announcements/nonexistent/reactions/👍", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParams(req, map[string]string{"id": "nonexistent", "name": "👍"})
		rec := httptest.NewRecorder()
		handler.PUTAnnouncementReaction(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}
