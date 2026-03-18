package mastodon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/chairswithlegs/monstera/internal/uid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConversationsHandler_GETConversations(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, "https://example.com", "example.com", 500)
	convSvc := service.NewConversationService(st, statusSvc)
	handler := NewConversationsHandler(convSvc, "example.com")

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations", nil)
		rec := httptest.NewRecorder()
		handler.GETConversations(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("authenticated empty returns 200 and empty array", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.GETConversations(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body)
	})
}

func TestConversationsHandler_DELETEConversation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, "https://example.com", "example.com", 500)
	convSvc := service.NewConversationService(st, statusSvc)
	handler := NewConversationsHandler(convSvc, "example.com")

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)
	convID := uid.New()

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/conversations/"+convID, nil)
		req = testutil.AddChiURLParam(req, "id", convID)
		rec := httptest.NewRecorder()
		handler.DELETEConversation(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("authenticated delete returns 200", func(t *testing.T) {
		require.NoError(t, st.CreateConversation(ctx, convID))
		require.NoError(t, st.UpsertAccountConversation(ctx, store.UpsertAccountConversationInput{
			ID:             uid.New(),
			AccountID:      acc.ID,
			ConversationID: convID,
			LastStatusID:   "",
			Unread:         true,
		}))
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/conversations/"+convID, nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", convID)
		rec := httptest.NewRecorder()
		handler.DELETEConversation(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body)
	})
}

func TestConversationsHandler_POSTConversationRead(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, "https://example.com", "example.com", 500)
	convSvc := service.NewConversationService(st, statusSvc)
	handler := NewConversationsHandler(convSvc, "example.com")

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)
	convID := uid.New()

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+convID+"/read", nil)
		req = testutil.AddChiURLParam(req, "id", convID)
		rec := httptest.NewRecorder()
		handler.POSTConversationRead(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("not found returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/nonexistent/read", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", "nonexistent")
		rec := httptest.NewRecorder()
		handler.POSTConversationRead(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("authenticated mark read returns 200", func(t *testing.T) {
		require.NoError(t, st.CreateConversation(ctx, convID))
		require.NoError(t, st.UpsertAccountConversation(ctx, store.UpsertAccountConversationInput{
			ID:             uid.New(),
			AccountID:      acc.ID,
			ConversationID: convID,
			LastStatusID:   "",
			Unread:         true,
		}))
		req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+convID+"/read", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", convID)
		rec := httptest.NewRecorder()
		handler.POSTConversationRead(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, convID, body["id"])
		assert.Equal(t, false, body["unread"])
	})
}
