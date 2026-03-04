package monstera

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chairswithlegs/monstera-fed/internal/api/middleware"
	"github.com/chairswithlegs/monstera-fed/internal/api/monstera/apimodel"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/service"
	"github.com/chairswithlegs/monstera-fed/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminUsersHandler_GETUsers(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	modSvc := service.NewModerationService(st)
	handler := NewAdminUsersHandler(accountSvc, modSvc)

	t.Run("no account returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
		rec := httptest.NewRecorder()
		handler.GETUsers(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("account with RoleUser returns 403", func(t *testing.T) {
		userAcc := createAccountWithRole(t, st, "user", domain.RoleUser)
		req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), userAcc))
		rec := httptest.NewRecorder()
		handler.GETUsers(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("admin returns 200 and user list", func(t *testing.T) {
		createAccountWithRole(t, st, "admin", domain.RoleAdmin)
		adminAcc := createAccountWithRole(t, st, "admin2", domain.RoleAdmin)
		req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), adminAcc))
		rec := httptest.NewRecorder()
		handler.GETUsers(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body apimodel.AdminUserList
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.GreaterOrEqual(t, len(body.Users), 1)
	})
}

func TestAdminUsersHandler_GETUser(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	modSvc := service.NewModerationService(st)
	handler := NewAdminUsersHandler(accountSvc, modSvc)
	adminAcc := createAccountWithRole(t, st, "admin", domain.RoleAdmin)

	t.Run("no account returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/users/"+adminAcc.ID, nil)
		rec := httptest.NewRecorder()
		handler.GETUser(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("admin with valid account id returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/users/"+adminAcc.ID, nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), adminAcc))
		req = testutil.AddChiURLParam(req, "id", adminAcc.ID)
		rec := httptest.NewRecorder()
		handler.GETUser(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body apimodel.AdminUser
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, adminAcc.ID, body.AccountID)
		assert.Equal(t, "admin", body.Username)
	})

	t.Run("admin with nonexistent id returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/users/01nonexistent", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), adminAcc))
		req = testutil.AddChiURLParam(req, "id", "01nonexistent")
		rec := httptest.NewRecorder()
		handler.GETUser(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestAdminUsersHandler_POSTSuspend(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	modSvc := service.NewModerationService(st)
	handler := NewAdminUsersHandler(accountSvc, modSvc)
	adminAcc := createAccountWithRole(t, st, "admin", domain.RoleAdmin)
	targetAcc := createAccountWithRole(t, st, "target", domain.RoleUser)

	t.Run("no account returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/users/"+targetAcc.ID+"/suspend", nil)
		req = testutil.AddChiURLParam(req, "id", targetAcc.ID)
		rec := httptest.NewRecorder()
		handler.POSTSuspend(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("admin with valid target returns 204", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/users/"+targetAcc.ID+"/suspend", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), adminAcc))
		req = testutil.AddChiURLParam(req, "id", targetAcc.ID)
		rec := httptest.NewRecorder()
		handler.POSTSuspend(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}

func TestAdminUsersHandler_PUTRole(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	modSvc := service.NewModerationService(st)
	handler := NewAdminUsersHandler(accountSvc, modSvc)
	adminAcc := createAccountWithRole(t, st, "admin", domain.RoleAdmin)
	modAcc := createAccountWithRole(t, st, "mod", domain.RoleModerator)
	targetAcc := createAccountWithRole(t, st, "target", domain.RoleUser)
	targetUser, err := st.GetUserByAccountID(ctx, targetAcc.ID)
	require.NoError(t, err)
	targetUserID := targetUser.ID
	modUser, err := st.GetUserByAccountID(ctx, modAcc.ID)
	require.NoError(t, err)

	t.Run("moderator returns 403", func(t *testing.T) {
		body := map[string]string{"role": domain.RoleModerator}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPut, "/admin/users/"+modUser.ID+"/role", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), modAcc))
		req = testutil.AddChiURLParam(req, "id", modUser.ID)
		rec := httptest.NewRecorder()
		handler.PUTRole(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("admin with valid role returns 204", func(t *testing.T) {
		body := map[string]string{"role": domain.RoleModerator}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPut, "/admin/users/"+targetUserID+"/role", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), adminAcc))
		req = testutil.AddChiURLParam(req, "id", targetUserID)
		rec := httptest.NewRecorder()
		handler.PUTRole(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("admin with invalid role returns 400", func(t *testing.T) {
		body := map[string]string{"role": "invalid"}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPut, "/admin/users/"+targetUserID+"/role", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), adminAcc))
		req = testutil.AddChiURLParam(req, "id", targetUserID)
		rec := httptest.NewRecorder()
		handler.PUTRole(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestAdminUsersHandler_DELETEUser(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	modSvc := service.NewModerationService(st)
	handler := NewAdminUsersHandler(accountSvc, modSvc)
	adminAcc := createAccountWithRole(t, st, "admin", domain.RoleAdmin)
	targetAcc := createAccountWithRole(t, st, "target", domain.RoleUser)

	t.Run("moderator returns 403", func(t *testing.T) {
		modAcc := createAccountWithRole(t, st, "mod", domain.RoleModerator)
		req := httptest.NewRequest(http.MethodDelete, "/admin/users/"+targetAcc.ID, nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), modAcc))
		req = testutil.AddChiURLParam(req, "id", targetAcc.ID)
		rec := httptest.NewRecorder()
		handler.DELETEUser(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("admin with valid id returns 204", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/admin/users/"+targetAcc.ID, nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), adminAcc))
		req = testutil.AddChiURLParam(req, "id", targetAcc.ID)
		rec := httptest.NewRecorder()
		handler.DELETEUser(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}
