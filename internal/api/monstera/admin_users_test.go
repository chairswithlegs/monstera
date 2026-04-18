package monstera

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/api/monstera/apimodel"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminUsersHandler_GETUsers(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	modSvc := service.NewModerationService(st, testutil.NoopBlocklistRefresher{})
	handler := NewAdminUsersHandler(accountSvc, modSvc)

	t.Run("returns 200 and user list", func(t *testing.T) {
		createAccountWithRole(t, st, "admin", domain.RoleAdmin)
		createAccountWithRole(t, st, "admin2", domain.RoleAdmin)
		req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
		rec := httptest.NewRecorder()
		handler.GETUsers(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body apimodel.AdminUserList
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.GreaterOrEqual(t, len(body.Users), 1)
	})

	t.Run("caps offset at MaxOffset", func(t *testing.T) {
		// With an excessive offset the value is clamped to api.MaxOffset,
		// which exceeds the number of seeded users, so the result is empty.
		req := httptest.NewRequest(http.MethodGet, "/admin/users?offset=999999999", nil)
		rec := httptest.NewRecorder()
		handler.GETUsers(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body apimodel.AdminUserList
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body.Users)
	})
}

func TestAdminUsersHandler_GETUser(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	modSvc := service.NewModerationService(st, testutil.NoopBlocklistRefresher{})
	handler := NewAdminUsersHandler(accountSvc, modSvc)
	adminAcc := createAccountWithRole(t, st, "admin", domain.RoleAdmin)

	t.Run("with valid account id returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/users/"+adminAcc.ID, nil)
		req = testutil.AddChiURLParam(req, "id", adminAcc.ID)
		rec := httptest.NewRecorder()
		handler.GETUser(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body apimodel.AdminUser
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, adminAcc.ID, body.AccountID)
		assert.Equal(t, "admin", body.Username)
	})

	t.Run("with nonexistent id returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/users/01nonexistent", nil)
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
	modSvc := service.NewModerationService(st, testutil.NoopBlocklistRefresher{})
	handler := NewAdminUsersHandler(accountSvc, modSvc)
	adminAcc := createAccountWithRole(t, st, "admin", domain.RoleAdmin)
	adminUser := getUserByAccountID(t, st, adminAcc.ID)
	targetAcc := createAccountWithRole(t, st, "target", domain.RoleUser)

	t.Run("no user returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/users/"+targetAcc.ID+"/suspend", nil)
		req = testutil.AddChiURLParam(req, "id", targetAcc.ID)
		rec := httptest.NewRecorder()
		handler.POSTSuspend(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("with user returns 204", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/users/"+targetAcc.ID+"/suspend", nil)
		req = req.WithContext(middleware.WithUser(req.Context(), adminUser))
		req = testutil.AddChiURLParam(req, "id", targetAcc.ID)
		rec := httptest.NewRecorder()
		handler.POSTSuspend(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}

func TestAdminUsersHandler_PUTRole(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	modSvc := service.NewModerationService(st, testutil.NoopBlocklistRefresher{})
	handler := NewAdminUsersHandler(accountSvc, modSvc)
	adminAcc := createAccountWithRole(t, st, "admin", domain.RoleAdmin)
	adminUser := getUserByAccountID(t, st, adminAcc.ID)
	targetAcc := createAccountWithRole(t, st, "target", domain.RoleUser)

	t.Run("no user returns 403", func(t *testing.T) {
		body := map[string]string{"role": domain.RoleModerator}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPut, "/admin/users/"+targetAcc.ID+"/role", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req = testutil.AddChiURLParam(req, "id", targetAcc.ID)
		rec := httptest.NewRecorder()
		handler.PUTRole(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("with user and valid role returns 204", func(t *testing.T) {
		body := map[string]string{"role": domain.RoleModerator}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPut, "/admin/users/"+targetAcc.ID+"/role", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithUser(req.Context(), adminUser))
		req = testutil.AddChiURLParam(req, "id", targetAcc.ID)
		rec := httptest.NewRecorder()
		handler.PUTRole(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("with invalid role returns 422", func(t *testing.T) {
		body := map[string]string{"role": "invalid"}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPut, "/admin/users/"+targetAcc.ID+"/role", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithUser(req.Context(), adminUser))
		req = testutil.AddChiURLParam(req, "id", targetAcc.ID)
		rec := httptest.NewRecorder()
		handler.PUTRole(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})
}

func TestAdminUsersHandler_DELETEUser(t *testing.T) {
	t.Parallel()

	type fixture struct {
		handler   *AdminUsersHandler
		store     *testutil.FakeStore
		adminUser *domain.User
		target    *domain.Account
	}
	seed := func(t *testing.T) fixture {
		t.Helper()
		st := testutil.NewFakeStore()
		accountSvc := service.NewAccountService(st, "https://example.com")
		modSvc := service.NewModerationService(st, testutil.NoopBlocklistRefresher{})
		handler := NewAdminUsersHandler(accountSvc, modSvc)
		adminAcc := createAccountWithRole(t, st, "admin", domain.RoleAdmin)
		adminUser := getUserByAccountID(t, st, adminAcc.ID)
		target := createAccountWithRole(t, st, "target", domain.RoleUser)
		return fixture{handler: handler, store: st, adminUser: adminUser, target: target}
	}

	t.Run("no user returns 403", func(t *testing.T) {
		t.Parallel()
		f := seed(t)
		req := httptest.NewRequest(http.MethodDelete, "/admin/users/"+f.target.ID, nil)
		req = testutil.AddChiURLParam(req, "id", f.target.ID)
		rec := httptest.NewRecorder()
		f.handler.DELETEUser(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("returns 204 and hard-deletes account", func(t *testing.T) {
		t.Parallel()
		f := seed(t)
		req := httptest.NewRequest(http.MethodDelete, "/admin/users/"+f.target.ID, nil)
		req = req.WithContext(middleware.WithUser(req.Context(), f.adminUser))
		req = testutil.AddChiURLParam(req, "id", f.target.ID)
		rec := httptest.NewRecorder()
		f.handler.DELETEUser(rec, req)
		require.Equal(t, http.StatusNoContent, rec.Code)

		_, err := f.store.GetAccountByID(req.Context(), f.target.ID)
		require.ErrorIs(t, err, domain.ErrNotFound)
		_, err = f.store.GetUserByAccountID(req.Context(), f.target.ID)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})
}
