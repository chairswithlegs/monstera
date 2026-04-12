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
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newStatusesActionsHandler(st *testutil.FakeStore) *StatusesHandler {
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, "https://example.com", "example.com", 500)
	conversationSvc := service.NewConversationService(st, statusSvc)
	statusWriteSvc := service.NewStatusWriteService(st, statusSvc, conversationSvc, "https://example.com", "example.com", 500)
	interactionSvc := service.NewStatusInteractionService(st, statusSvc, "https://example.com")
	scheduledSvc := service.NewScheduledStatusService(st, statusWriteSvc)
	return NewStatusesHandler(accountSvc, statusSvc, statusWriteSvc, interactionSvc, scheduledSvc, conversationSvc, "example.com", nil, nil)
}

func TestStatusesActions_Unauthenticated(t *testing.T) {
	t.Parallel()
	handler := newStatusesActionsHandler(testutil.NewFakeStore())

	methods := []struct {
		name string
		call func(http.ResponseWriter, *http.Request)
	}{
		{"POSTReblog", handler.POSTReblog},
		{"POSTUnreblog", handler.POSTUnreblog},
		{"POSTFavourite", handler.POSTFavourite},
		{"POSTUnfavourite", handler.POSTUnfavourite},
		{"POSTBookmark", handler.POSTBookmark},
		{"POSTUnbookmark", handler.POSTUnbookmark},
		{"POSTMuteConversation", handler.POSTMuteConversation},
		{"POSTUnmuteConversation", handler.POSTUnmuteConversation},
	}
	for _, m := range methods {
		t.Run(m.name+" returns 401", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses/someid/action", nil)
			req = testutil.AddChiURLParam(req, "id", "someid")
			rec := httptest.NewRecorder()
			m.call(rec, req)
			assert.Equal(t, http.StatusUnauthorized, rec.Code)
		})
	}
}

func TestStatusesActions_MissingID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	handler := newStatusesActionsHandler(st)
	accountSvc := service.NewAccountService(st, "https://example.com")

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "password123",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	methods := []struct {
		name string
		call func(http.ResponseWriter, *http.Request)
	}{
		{"POSTReblog", handler.POSTReblog},
		{"POSTUnreblog", handler.POSTUnreblog},
		{"POSTFavourite", handler.POSTFavourite},
		{"POSTUnfavourite", handler.POSTUnfavourite},
		{"POSTBookmark", handler.POSTBookmark},
		{"POSTUnbookmark", handler.POSTUnbookmark},
		{"POSTMuteConversation", handler.POSTMuteConversation},
		{"POSTUnmuteConversation", handler.POSTUnmuteConversation},
	}
	for _, m := range methods {
		t.Run(m.name+" returns 404", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses//action", nil)
			req = req.WithContext(middleware.WithAccount(req.Context(), acc))
			rec := httptest.NewRecorder()
			m.call(rec, req)
			assert.Equal(t, http.StatusNotFound, rec.Code)
		})
	}
}

func TestStatusesActions_POSTFavourite_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	handler := newStatusesActionsHandler(st)
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, "https://example.com", "example.com", 500)
	conversationSvc := service.NewConversationService(st, statusSvc)
	statusWriteSvc := service.NewStatusWriteService(st, statusSvc, conversationSvc, "https://example.com", "example.com", 500)

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username: "favuser",
		Email:    "favuser@example.com",
		Password: "password123",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	status, err := statusWriteSvc.Create(ctx, service.CreateStatusInput{
		AccountID:  acc.ID,
		Username:   acc.Username,
		Text:       "test content",
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)
	statusID := status.Status.ID

	req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses/"+statusID+"/favourite", nil)
	req = req.WithContext(middleware.WithAccount(req.Context(), acc))
	req = testutil.AddChiURLParam(req, "id", statusID)
	rec := httptest.NewRecorder()
	handler.POSTFavourite(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.True(t, body["favourited"].(bool))
}
