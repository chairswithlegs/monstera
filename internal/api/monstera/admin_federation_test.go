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

func TestAdminFederationHandler_GETInstances(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	instanceSvc := service.NewInstanceService(st)
	modSvc := service.NewModerationService(st)
	handler := NewAdminFederationHandler(accountSvc, instanceSvc, modSvc)

	t.Run("no account returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/federation/instances", nil)
		rec := httptest.NewRecorder()
		handler.GETInstances(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("admin returns 200 and instances list", func(t *testing.T) {
		adminAcc := createAccountWithRole(t, st, "admin", domain.RoleAdmin)
		req := httptest.NewRequest(http.MethodGet, "/admin/federation/instances", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), adminAcc))
		rec := httptest.NewRecorder()
		handler.GETInstances(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body apimodel.AdminKnownInstanceList
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.NotNil(t, body.Instances)
	})
}

func TestAdminFederationHandler_GETDomainBlocks(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	instanceSvc := service.NewInstanceService(st)
	modSvc := service.NewModerationService(st)
	handler := NewAdminFederationHandler(accountSvc, instanceSvc, modSvc)

	t.Run("no account returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/federation/domain-blocks", nil)
		rec := httptest.NewRecorder()
		handler.GETDomainBlocks(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("admin returns 200 and domain blocks list", func(t *testing.T) {
		adminAcc := createAccountWithRole(t, st, "admin", domain.RoleAdmin)
		req := httptest.NewRequest(http.MethodGet, "/admin/federation/domain-blocks", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), adminAcc))
		rec := httptest.NewRecorder()
		handler.GETDomainBlocks(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body apimodel.AdminDomainBlockList
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.NotNil(t, body.DomainBlocks)
	})
}

func TestAdminFederationHandler_POSTDomainBlocks(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	instanceSvc := service.NewInstanceService(st)
	modSvc := service.NewModerationService(st)
	handler := NewAdminFederationHandler(accountSvc, instanceSvc, modSvc)
	adminAcc := createAccountWithRole(t, st, "admin", domain.RoleAdmin)
	modAcc := createAccountWithRole(t, st, "mod", domain.RoleModerator)

	t.Run("moderator returns 403", func(t *testing.T) {
		body := map[string]string{"domain": "evil.example", "severity": domain.DomainBlockSeveritySilence}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/admin/federation/domain-blocks", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), modAcc))
		rec := httptest.NewRecorder()
		handler.POSTDomainBlocks(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("admin with valid body returns 204", func(t *testing.T) {
		body := map[string]string{"domain": "evil.example", "severity": domain.DomainBlockSeveritySilence}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/admin/federation/domain-blocks", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), adminAcc))
		rec := httptest.NewRecorder()
		handler.POSTDomainBlocks(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}

func TestAdminFederationHandler_DELETEDomainBlock(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	instanceSvc := service.NewInstanceService(st)
	modSvc := service.NewModerationService(st)
	handler := NewAdminFederationHandler(accountSvc, instanceSvc, modSvc)
	adminAcc := createAccountWithRole(t, st, "admin", domain.RoleAdmin)
	modAcc := createAccountWithRole(t, st, "mod", domain.RoleModerator)

	t.Run("moderator returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/admin/federation/domain-blocks/evil.example", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), modAcc))
		req = testutil.AddChiURLParam(req, "domain", "evil.example")
		rec := httptest.NewRecorder()
		handler.DELETEDomainBlock(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("admin returns 204", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/admin/federation/domain-blocks/evil.example", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), adminAcc))
		req = testutil.AddChiURLParam(req, "domain", "evil.example")
		rec := httptest.NewRecorder()
		handler.DELETEDomainBlock(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}
