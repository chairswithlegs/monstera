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

func TestModeratorReportsHandler_GETReports(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	modSvc := service.NewModerationService(st)
	handler := NewModeratorReportsHandler(modSvc)

	t.Run("returns 200 and report list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/reports", nil)
		rec := httptest.NewRecorder()
		handler.GETReports(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body apimodel.AdminReportList
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.NotNil(t, body.Reports)
	})
}

func TestModeratorReportsHandler_GETReport(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	modSvc := service.NewModerationService(st)
	handler := NewModeratorReportsHandler(modSvc)

	t.Run("with nonexistent id returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/reports/01nonexistent", nil)
		req = testutil.AddChiURLParam(req, "id", "01nonexistent")
		rec := httptest.NewRecorder()
		handler.GETReport(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestModeratorReportsHandler_POSTAssign(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	modSvc := service.NewModerationService(st)
	handler := NewModeratorReportsHandler(modSvc)
	adminAcc := createAccountWithRole(t, st, "admin", domain.RoleAdmin)
	adminUser := getUserByAccountID(t, st, adminAcc.ID)

	t.Run("no user returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/reports/01reportid/assign", nil)
		req = testutil.AddChiURLParam(req, "id", "01reportid")
		rec := httptest.NewRecorder()
		handler.POSTAssign(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("with user returns 204", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/reports/01reportid/assign", bytes.NewReader([]byte("{}")))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithUser(req.Context(), adminUser))
		req = testutil.AddChiURLParam(req, "id", "01reportid")
		rec := httptest.NewRecorder()
		handler.POSTAssign(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}

func TestModeratorReportsHandler_POSTResolve(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	modSvc := service.NewModerationService(st)
	handler := NewModeratorReportsHandler(modSvc)
	adminAcc := createAccountWithRole(t, st, "admin", domain.RoleAdmin)
	adminUser := getUserByAccountID(t, st, adminAcc.ID)

	t.Run("no user returns 403", func(t *testing.T) {
		body := map[string]string{"resolution": "resolved"}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/admin/reports/01reportid/resolve", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req = testutil.AddChiURLParam(req, "id", "01reportid")
		rec := httptest.NewRecorder()
		handler.POSTResolve(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("with user returns 204", func(t *testing.T) {
		body := map[string]string{"resolution": "resolved"}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/admin/reports/01reportid/resolve", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithUser(req.Context(), adminUser))
		req = testutil.AddChiURLParam(req, "id", "01reportid")
		rec := httptest.NewRecorder()
		handler.POSTResolve(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}
