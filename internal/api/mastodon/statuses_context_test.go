package mastodon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
	return NewStatusesHandler(accountSvc, statusSvc, statusWriteSvc, interactionSvc, scheduledSvc, conversationSvc, "example.com", nil, nil)
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
