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

const testNonexistentID = "01H0000000000000000000000"

func newStatusesContextHandler(st *testutil.FakeStore) *StatusesHandler {
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, "https://example.com", "example.com", 500)
	conversationSvc := service.NewConversationService(st, statusSvc)
	statusWriteSvc := service.NewStatusWriteService(st, statusSvc, conversationSvc, "https://example.com", "example.com", 500)
	interactionSvc := service.NewStatusInteractionService(st, statusSvc, "https://example.com")
	scheduledSvc := service.NewScheduledStatusService(st, statusWriteSvc)
	return NewStatusesHandler(accountSvc, statusSvc, statusWriteSvc, interactionSvc, scheduledSvc, conversationSvc, nil, "example.com", nil, nil)
}

func TestStatusesContext_GETContext_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	handler := newStatusesContextHandler(st)
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, "https://example.com", "example.com", 500)
	conversationSvc := service.NewConversationService(st, statusSvc)
	statusWriteSvc := service.NewStatusWriteService(st, statusSvc, conversationSvc, "https://example.com", "example.com", 500)

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username: "ctxuser",
		Email:    "ctxuser@example.com",
		Password: "password123",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	status, err := statusWriteSvc.Create(ctx, service.CreateStatusInput{
		AccountID:  acc.ID,
		Username:   acc.Username,
		Text:       "context test",
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)
	statusID := status.Status.ID

	req := httptest.NewRequest(http.MethodGet, "/api/v1/statuses/"+statusID+"/context", nil)
	req = testutil.AddChiURLParam(req, "id", statusID)
	rec := httptest.NewRecorder()
	handler.GETContext(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Contains(t, body, "ancestors")
	assert.Contains(t, body, "descendants")
}

func TestStatusesContext_GETFavouritedBy_NotFound(t *testing.T) {
	t.Parallel()
	handler := newStatusesContextHandler(testutil.NewFakeStore())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/statuses/"+testNonexistentID+"/favourited_by", nil)
	req = testutil.AddChiURLParam(req, "id", testNonexistentID)
	rec := httptest.NewRecorder()
	handler.GETFavouritedBy(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestStatusesContext_GETRebloggedBy_NotFound(t *testing.T) {
	t.Parallel()
	handler := newStatusesContextHandler(testutil.NewFakeStore())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/statuses/"+testNonexistentID+"/reblogged_by", nil)
	req = testutil.AddChiURLParam(req, "id", testNonexistentID)
	rec := httptest.NewRecorder()
	handler.GETRebloggedBy(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestStatusesContext_GETContext_ThreadFilterRemovesMatchingDescendant(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, "https://example.com", "example.com", 500)
	conversationSvc := service.NewConversationService(st, statusSvc)
	statusWriteSvc := service.NewStatusWriteService(st, statusSvc, conversationSvc, "https://example.com", "example.com", 500)
	interactionSvc := service.NewStatusInteractionService(st, statusSvc, "https://example.com")
	scheduledSvc := service.NewScheduledStatusService(st, statusWriteSvc)
	userFilterSvc := service.NewUserFilterService(st)
	handler := NewStatusesHandler(accountSvc, statusSvc, statusWriteSvc, interactionSvc, scheduledSvc, conversationSvc, userFilterSvc, "example.com", nil, nil)

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "password123",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	// Create a parent status and a reply.
	parent, err := statusWriteSvc.Create(ctx, service.CreateStatusInput{
		AccountID:  acc.ID,
		Username:   acc.Username,
		Text:       "parent post",
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)

	reply, err := statusWriteSvc.Create(ctx, service.CreateStatusInput{
		AccountID:   acc.ID,
		Username:    acc.Username,
		Text:        "bad word reply",
		Visibility:  domain.VisibilityPublic,
		InReplyToID: &parent.Status.ID,
	})
	require.NoError(t, err)

	// Create a v1 filter matching "bad word" in thread context.
	_, err = userFilterSvc.CreateFilter(ctx, acc.ID, "bad word", []string{domain.FilterContextThread}, false, nil, false)
	require.NoError(t, err)

	t.Run("authenticated viewer with thread filter sees reply removed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/statuses/"+parent.Status.ID+"/context", nil)
		req = testutil.AddChiURLParam(req, "id", parent.Status.ID)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.GETContext(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var body map[string][]map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body["ancestors"])
		// The reply should be filtered out.
		for _, d := range body["descendants"] {
			assert.NotEqual(t, reply.Status.ID, d["id"], "filtered reply should not appear in descendants")
		}
	})

	t.Run("unauthenticated viewer sees reply unfiltered", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/statuses/"+parent.Status.ID+"/context", nil)
		req = testutil.AddChiURLParam(req, "id", parent.Status.ID)
		rec := httptest.NewRecorder()
		handler.GETContext(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var body map[string][]map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		require.Len(t, body["descendants"], 1, "unauthenticated viewer should see the reply")
		assert.Equal(t, reply.Status.ID, body["descendants"][0]["id"])
	})
}
