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

func TestAdminReportsHandler_GETReports(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	modSvc := service.NewModerationService(st)
	handler := NewAdminReportsHandler(accountSvc, modSvc)

	t.Run("no account returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/reports", nil)
		rec := httptest.NewRecorder()
		handler.GETReports(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("admin returns 200 and report list", func(t *testing.T) {
		adminAcc := createAccountWithRole(t, st, "admin", domain.RoleAdmin)
		req := httptest.NewRequest(http.MethodGet, "/admin/reports", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), adminAcc))
		rec := httptest.NewRecorder()
		handler.GETReports(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body apimodel.AdminReportList
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.NotNil(t, body.Reports)
	})
}

func TestAdminReportsHandler_GETReport(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	modSvc := service.NewModerationService(st)
	handler := NewAdminReportsHandler(accountSvc, modSvc)
	adminAcc := createAccountWithRole(t, st, "admin", domain.RoleAdmin)

	t.Run("no account returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/reports/01reportid", nil)
		req = testutil.AddChiURLParam(req, "id", "01reportid")
		rec := httptest.NewRecorder()
		handler.GETReport(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("admin with nonexistent id returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/reports/01nonexistent", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), adminAcc))
		req = testutil.AddChiURLParam(req, "id", "01nonexistent")
		rec := httptest.NewRecorder()
		handler.GETReport(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestAdminReportsHandler_POSTAssign(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	modSvc := service.NewModerationService(st)
	handler := NewAdminReportsHandler(accountSvc, modSvc)
	adminAcc := createAccountWithRole(t, st, "admin", domain.RoleAdmin)

	t.Run("no account returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/reports/01reportid/assign", nil)
		req = testutil.AddChiURLParam(req, "id", "01reportid")
		rec := httptest.NewRecorder()
		handler.POSTAssign(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("admin returns 204", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/reports/01reportid/assign", bytes.NewReader([]byte("{}")))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), adminAcc))
		req = testutil.AddChiURLParam(req, "id", "01reportid")
		rec := httptest.NewRecorder()
		handler.POSTAssign(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}

func TestAdminReportsHandler_POSTResolve(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	modSvc := service.NewModerationService(st)
	handler := NewAdminReportsHandler(accountSvc, modSvc)
	adminAcc := createAccountWithRole(t, st, "admin", domain.RoleAdmin)

	t.Run("no account returns 403", func(t *testing.T) {
		body := map[string]string{"resolution": "resolved"}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/admin/reports/01reportid/resolve", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req = testutil.AddChiURLParam(req, "id", "01reportid")
		rec := httptest.NewRecorder()
		handler.POSTResolve(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("admin returns 204", func(t *testing.T) {
		body := map[string]string{"resolution": "resolved"}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/admin/reports/01reportid/resolve", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), adminAcc))
		req = testutil.AddChiURLParam(req, "id", "01reportid")
		rec := httptest.NewRecorder()
		handler.POSTResolve(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}
