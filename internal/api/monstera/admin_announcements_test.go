package monstera

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chairswithlegs/monstera/internal/api/monstera/apimodel"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/chairswithlegs/monstera/internal/uid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminAnnouncementsHandler_GETAnnouncements(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	announcementSvc := service.NewAnnouncementService(st)
	handler := NewAdminAnnouncementsHandler(announcementSvc)

	t.Run("empty list returns 200 and []", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/announcements", nil)
		rec := httptest.NewRecorder()
		handler.GETAnnouncements(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []apimodel.AdminAnnouncement
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body)
	})

	t.Run("with announcements returns list", func(t *testing.T) {
		ctx := context.Background()
		now := time.Now().UTC()
		_, err := st.CreateAnnouncement(ctx, store.CreateAnnouncementInput{
			ID:          uid.New(),
			Content:     "<p>Admin announcement</p>",
			PublishedAt: now,
		})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/admin/announcements", nil)
		rec := httptest.NewRecorder()
		handler.GETAnnouncements(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []apimodel.AdminAnnouncement
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		require.Len(t, body, 1)
		assert.Equal(t, "<p>Admin announcement</p>", body[0].Content)
		assert.NotEmpty(t, body[0].ID)
		assert.NotEmpty(t, body[0].PublishedAt)
		assert.NotEmpty(t, body[0].UpdatedAt)
	})
}

func TestAdminAnnouncementsHandler_POSTAnnouncements(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	announcementSvc := service.NewAnnouncementService(st)
	handler := NewAdminAnnouncementsHandler(announcementSvc)

	t.Run("valid body returns 201 and created announcement", func(t *testing.T) {
		body := map[string]any{
			"content": "<p>New announcement</p>",
			"all_day": false,
		}
		b, err := json.Marshal(body)
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, "/admin/announcements", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.POSTAnnouncements(rec, req)
		assert.Equal(t, http.StatusCreated, rec.Code)
		var out apimodel.AdminAnnouncement
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&out))
		assert.Equal(t, "<p>New announcement</p>", out.Content)
		assert.False(t, out.AllDay)
		assert.NotEmpty(t, out.ID)
		assert.NotEmpty(t, out.PublishedAt)
		assert.NotEmpty(t, out.UpdatedAt)
	})

	t.Run("missing content returns 422", func(t *testing.T) {
		body := map[string]any{"all_day": true}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/admin/announcements", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.POSTAnnouncements(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/announcements", bytes.NewReader([]byte("not json")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.POSTAnnouncements(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid starts_at returns 422", func(t *testing.T) {
		body := map[string]any{
			"content":   "<p>Ok</p>",
			"starts_at": "not-a-date",
		}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/admin/announcements", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.POSTAnnouncements(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})
}

func TestAdminAnnouncementsHandler_PUTAnnouncement(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	announcementSvc := service.NewAnnouncementService(st)
	handler := NewAdminAnnouncementsHandler(announcementSvc)

	annID := uid.New()
	_, err := st.CreateAnnouncement(ctx, store.CreateAnnouncementInput{
		ID:          annID,
		Content:     "<p>Original</p>",
		PublishedAt: time.Now().UTC(),
	})
	require.NoError(t, err)

	t.Run("valid body returns 200 and updated announcement", func(t *testing.T) {
		body := map[string]any{"content": "<p>Updated content</p>"}
		b, err := json.Marshal(body)
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPut, "/admin/announcements/"+annID, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req = testutil.AddChiURLParam(req, "id", annID)
		rec := httptest.NewRecorder()
		handler.PUTAnnouncement(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var out apimodel.AdminAnnouncement
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&out))
		assert.Equal(t, "<p>Updated content</p>", out.Content)
		assert.Equal(t, annID, out.ID)
	})

	t.Run("nonexistent id returns 404", func(t *testing.T) {
		body := map[string]any{"content": "<p>No effect</p>"}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPut, "/admin/announcements/nonexistent", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req = testutil.AddChiURLParam(req, "id", "nonexistent")
		rec := httptest.NewRecorder()
		handler.PUTAnnouncement(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("empty id returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/admin/announcements/", nil)
		req = testutil.AddChiURLParam(req, "id", "")
		rec := httptest.NewRecorder()
		handler.PUTAnnouncement(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}
