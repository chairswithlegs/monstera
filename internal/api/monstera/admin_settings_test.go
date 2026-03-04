package monstera

import (
	"bytes"
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

func TestAdminSettingsHandler_GETSettings(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	instanceSvc := service.NewInstanceService(st)
	handler := NewAdminSettingsHandler(accountSvc, instanceSvc)

	t.Run("no account returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/settings", nil)
		rec := httptest.NewRecorder()
		handler.GETSettings(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("moderator returns 403", func(t *testing.T) {
		modAcc := createAccountWithRole(t, st, "mod", domain.RoleModerator)
		req := httptest.NewRequest(http.MethodGet, "/admin/settings", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), modAcc))
		rec := httptest.NewRecorder()
		handler.GETSettings(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("admin returns 200 and settings", func(t *testing.T) {
		adminAcc := createAccountWithRole(t, st, "admin", domain.RoleAdmin)
		req := httptest.NewRequest(http.MethodGet, "/admin/settings", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), adminAcc))
		rec := httptest.NewRecorder()
		handler.GETSettings(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body apimodel.AdminSettings
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	})
}

func TestAdminSettingsHandler_PUTSettings(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	instanceSvc := service.NewInstanceService(st)
	handler := NewAdminSettingsHandler(accountSvc, instanceSvc)
	adminAcc := createAccountWithRole(t, st, "admin", domain.RoleAdmin)

	t.Run("no account returns 403", func(t *testing.T) {
		body := map[string]any{"settings": map[string]string{"registration_mode": "open"}}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPut, "/admin/settings", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.PUTSettings(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("admin with valid body returns 204", func(t *testing.T) {
		body := map[string]any{"settings": map[string]string{"registration_mode": "open"}}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPut, "/admin/settings", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), adminAcc))
		rec := httptest.NewRecorder()
		handler.PUTSettings(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}
