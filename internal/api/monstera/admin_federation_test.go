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

func TestAdminFederationHandler_GETInstances(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	instanceSvc := service.NewInstanceService(st)
	modSvc := service.NewModerationService(st)
	handler := NewAdminFederationHandler(instanceSvc, modSvc)

	t.Run("returns 200 and instances list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/federation/instances", nil)
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
	instanceSvc := service.NewInstanceService(st)
	modSvc := service.NewModerationService(st)
	handler := NewAdminFederationHandler(instanceSvc, modSvc)

	t.Run("returns 200 and domain blocks list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/federation/domain-blocks", nil)
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
	instanceSvc := service.NewInstanceService(st)
	modSvc := service.NewModerationService(st)
	handler := NewAdminFederationHandler(instanceSvc, modSvc)
	adminAcc := createAccountWithRole(t, st, "admin", domain.RoleAdmin)
	adminUser := getUserByAccountID(t, st, adminAcc.ID)

	t.Run("no user returns 403", func(t *testing.T) {
		body := map[string]string{"domain": "evil.example", "severity": domain.DomainBlockSeveritySilence}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/admin/federation/domain-blocks", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.POSTDomainBlocks(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("with user and valid body returns 204", func(t *testing.T) {
		body := map[string]string{"domain": "evil.example", "severity": domain.DomainBlockSeveritySilence}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/admin/federation/domain-blocks", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithUser(req.Context(), adminUser))
		rec := httptest.NewRecorder()
		handler.POSTDomainBlocks(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}

func TestAdminFederationHandler_DELETEDomainBlock(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	instanceSvc := service.NewInstanceService(st)
	modSvc := service.NewModerationService(st)
	handler := NewAdminFederationHandler(instanceSvc, modSvc)
	adminAcc := createAccountWithRole(t, st, "admin", domain.RoleAdmin)
	adminUser := getUserByAccountID(t, st, adminAcc.ID)

	t.Run("no user returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/admin/federation/domain-blocks/evil.example", nil)
		req = testutil.AddChiURLParam(req, "domain", "evil.example")
		rec := httptest.NewRecorder()
		handler.DELETEDomainBlock(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("with user returns 204", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/admin/federation/domain-blocks/evil.example", nil)
		req = req.WithContext(middleware.WithUser(req.Context(), adminUser))
		req = testutil.AddChiURLParam(req, "domain", "evil.example")
		rec := httptest.NewRecorder()
		handler.DELETEDomainBlock(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}
