package mastodon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/chairswithlegs/monstera/internal/uid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testStatusID = "status-1"

func newGroupedNotifHandler(t *testing.T) (*GroupedNotificationsHandler, *testutil.FakeStore, *domain.Account) {
	t.Helper()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	notifSvc := service.NewNotificationService(st)
	handler := NewGroupedNotificationsHandler(notifSvc, accountSvc, nil, "example.com")

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	return handler, st, acc
}

func createNotification(t *testing.T, st *testutil.FakeStore, accountID, fromID, notifType, groupKey string, statusID *string) {
	t.Helper()
	_, err := st.CreateNotification(context.Background(), store.CreateNotificationInput{
		ID:        uid.New(),
		AccountID: accountID,
		FromID:    fromID,
		Type:      notifType,
		StatusID:  statusID,
		GroupKey:  groupKey,
	})
	require.NoError(t, err)
}

func TestGroupedNotificationsHandler_GETGroupedNotifications(t *testing.T) {
	t.Parallel()
	handler, st, acc := newGroupedNotifHandler(t)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v2/notifications", nil)
		rec := httptest.NewRecorder()
		handler.GETGroupedNotifications(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("empty returns 200 with empty groups", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v2/notifications", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.GETGroupedNotifications(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		var body apimodel.GroupedNotificationsResponse
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body.NotificationGroups)
		assert.Empty(t, body.Accounts)
		assert.Empty(t, body.Statuses)
	})

	t.Run("groups favourites on same status", func(t *testing.T) {
		statusID := testStatusID
		createNotification(t, st, acc.ID, "from-1", domain.NotificationTypeFavourite, "favourite-status-1", &statusID)
		createNotification(t, st, acc.ID, "from-2", domain.NotificationTypeFavourite, "favourite-status-1", &statusID)

		req := httptest.NewRequest(http.MethodGet, "/api/v2/notifications", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.GETGroupedNotifications(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		var body apimodel.GroupedNotificationsResponse
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		require.Len(t, body.NotificationGroups, 1)
		assert.Equal(t, "favourite-status-1", body.NotificationGroups[0].GroupKey)
		assert.Equal(t, 2, body.NotificationGroups[0].NotificationsCount)
		assert.Len(t, body.NotificationGroups[0].SampleAccountIDs, 2)
	})
}

func TestGroupedNotificationsHandler_GETUnreadCount(t *testing.T) {
	t.Parallel()
	handler, st, acc := newGroupedNotifHandler(t)

	createNotification(t, st, acc.ID, "from-1", domain.NotificationTypeFollow, "ungrouped-f1", nil)
	statusID := testStatusID
	createNotification(t, st, acc.ID, "from-2", domain.NotificationTypeFavourite, "favourite-status-1", &statusID)
	createNotification(t, st, acc.ID, "from-3", domain.NotificationTypeFavourite, "favourite-status-1", &statusID)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/notifications/unread_count", nil)
	req = req.WithContext(middleware.WithAccount(req.Context(), acc))
	rec := httptest.NewRecorder()
	handler.GETUnreadCount(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	var body apimodel.UnreadCountResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, int64(2), body.Count) // 2 distinct group keys
}

func TestGroupedNotificationsHandler_POSTDismissNotificationGroup(t *testing.T) {
	t.Parallel()
	handler, st, acc := newGroupedNotifHandler(t)

	statusID := testStatusID
	createNotification(t, st, acc.ID, "from-1", domain.NotificationTypeFavourite, "favourite-status-1", &statusID)
	createNotification(t, st, acc.ID, "from-2", domain.NotificationTypeFavourite, "favourite-status-1", &statusID)

	// Dismiss the group.
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("group_key", "favourite-status-1")
	req := httptest.NewRequest(http.MethodPost, "/api/v2/notifications/favourite-status-1/dismiss", nil)
	req = req.WithContext(context.WithValue(middleware.WithAccount(req.Context(), acc), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	handler.POSTDismissNotificationGroup(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Verify the group is gone by listing.
	req2 := httptest.NewRequest(http.MethodGet, "/api/v2/notifications", nil)
	req2 = req2.WithContext(middleware.WithAccount(req2.Context(), acc))
	rec2 := httptest.NewRecorder()
	handler.GETGroupedNotifications(rec2, req2)
	assert.Equal(t, http.StatusOK, rec2.Code)

	var body apimodel.GroupedNotificationsResponse
	require.NoError(t, json.NewDecoder(rec2.Body).Decode(&body))
	assert.Empty(t, body.NotificationGroups)
}
