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

func TestNotificationsHandler_GETNotification(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	notifSvc := service.NewNotificationService(st)
	handler := NewNotificationsHandler(notifSvc, accountSvc, "example.com")

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	fromAcc, err := accountSvc.Create(ctx, service.CreateAccountInput{Username: "bob"})
	require.NoError(t, err)
	notifID := uid.New()
	_, err = st.CreateNotification(ctx, store.CreateNotificationInput{
		ID:        notifID,
		AccountID: acc.ID,
		FromID:    fromAcc.ID,
		Type:      domain.NotificationTypeFollow,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/"+notifID, nil)
		req = testutil.AddChiURLParam(req, "id", notifID)
		rec := httptest.NewRecorder()
		handler.GETNotification(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("not found returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/nonexistent", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", "nonexistent")
		rec := httptest.NewRecorder()
		handler.GETNotification(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("authenticated returns 200 and notification", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/"+notifID, nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", notifID)
		rec := httptest.NewRecorder()
		handler.GETNotification(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, notifID, body["id"])
		assert.Equal(t, domain.NotificationTypeFollow, body["type"])
	})
}

func TestNotificationsHandler_POSTClear(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	notifSvc := service.NewNotificationService(st)
	handler := NewNotificationsHandler(notifSvc, accountSvc, "example.com")

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/clear", nil)
		rec := httptest.NewRecorder()
		handler.POSTClear(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("authenticated returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/clear", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.POSTClear(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestNotificationsHandler_POSTDismiss(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	notifSvc := service.NewNotificationService(st)
	handler := NewNotificationsHandler(notifSvc, accountSvc, "example.com")

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	notifID := uid.New()
	fromAcc, err := accountSvc.Create(ctx, service.CreateAccountInput{Username: "bob"})
	require.NoError(t, err)
	_, err = st.CreateNotification(ctx, store.CreateNotificationInput{
		ID:        notifID,
		AccountID: acc.ID,
		FromID:    fromAcc.ID,
		Type:      domain.NotificationTypeFollow,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/"+notifID+"/dismiss", nil)
		req = testutil.AddChiURLParam(req, "id", notifID)
		rec := httptest.NewRecorder()
		handler.POSTDismiss(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("authenticated returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/"+notifID+"/dismiss", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", notifID)
		rec := httptest.NewRecorder()
		handler.POSTDismiss(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}
