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
	modSvc := service.NewModerationService(st, testutil.NoopBlocklistRefresher{})
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
	modSvc := service.NewModerationService(st, testutil.NoopBlocklistRefresher{})
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

	t.Run("surfaces purge status for silence / in_progress / complete blocks", func(t *testing.T) {
		st := testutil.NewFakeStore()
		modSvc := service.NewModerationService(st, testutil.NoopBlocklistRefresher{})
		handler := NewAdminFederationHandler(instanceSvc, modSvc)
		adminAcc := createAccountWithRole(t, st, "admin2", domain.RoleAdmin)

		// Silence — no purge fields.
		_, err := modSvc.CreateDomainBlock(t.Context(), adminAcc.ID, service.CreateDomainBlockInput{
			Domain: "silence.example", Severity: domain.DomainBlockSeveritySilence,
		})
		require.NoError(t, err)

		// Suspend in-progress — 2 remote accounts remaining.
		inProg, err := modSvc.CreateDomainBlock(t.Context(), adminAcc.ID, service.CreateDomainBlockInput{
			Domain: "suspend.example", Severity: domain.DomainBlockSeveritySuspend,
		})
		require.NoError(t, err)
		remoteDomain := "suspend.example"
		for _, id := range []string{"remote-a", "remote-b"} {
			st.SeedAccount(&domain.Account{ID: id, Username: id, Domain: &remoteDomain, APID: "https://suspend.example/users/" + id})
		}

		// Suspend completed.
		done, err := modSvc.CreateDomainBlock(t.Context(), adminAcc.ID, service.CreateDomainBlockInput{
			Domain: "done.example", Severity: domain.DomainBlockSeveritySuspend,
		})
		require.NoError(t, err)
		require.NoError(t, st.MarkDomainBlockPurgeComplete(t.Context(), done.ID))

		req := httptest.NewRequest(http.MethodGet, "/admin/federation/domain-blocks", nil)
		rec := httptest.NewRecorder()
		handler.GETDomainBlocks(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		var body apimodel.AdminDomainBlockList
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		byDomain := map[string]apimodel.AdminDomainBlock{}
		for _, b := range body.DomainBlocks {
			byDomain[b.Domain] = b
		}

		silence := byDomain["silence.example"]
		assert.Empty(t, silence.PurgeStatus)
		assert.Nil(t, silence.PurgeStartedAt)
		assert.Nil(t, silence.PurgeCompletedAt)
		assert.Nil(t, silence.PurgeAccountsRemaining)

		inProgress := byDomain["suspend.example"]
		assert.Equal(t, apimodel.PurgeStatusInProgress, inProgress.PurgeStatus)
		assert.NotNil(t, inProgress.PurgeStartedAt)
		assert.Nil(t, inProgress.PurgeCompletedAt)
		require.NotNil(t, inProgress.PurgeAccountsRemaining)
		assert.EqualValues(t, 2, *inProgress.PurgeAccountsRemaining)
		_ = inProg

		complete := byDomain["done.example"]
		assert.Equal(t, apimodel.PurgeStatusComplete, complete.PurgeStatus)
		assert.NotNil(t, complete.PurgeStartedAt)
		assert.NotNil(t, complete.PurgeCompletedAt)
		assert.Nil(t, complete.PurgeAccountsRemaining)
	})
}

func TestAdminFederationHandler_POSTDomainBlocks(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	instanceSvc := service.NewInstanceService(st)
	modSvc := service.NewModerationService(st, testutil.NoopBlocklistRefresher{})
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
	modSvc := service.NewModerationService(st, testutil.NoopBlocklistRefresher{})
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
