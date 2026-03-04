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

func TestAdminInvitesHandler_GETInvites(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	regSvc := service.NewRegistrationService(st, nil, nil, "https://example.com", "Example")
	handler := NewAdminInvitesHandler(accountSvc, regSvc)

	t.Run("no account returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/invites", nil)
		rec := httptest.NewRecorder()
		handler.GETInvites(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("admin returns 200 and invite list", func(t *testing.T) {
		adminAcc := createAccountWithRole(t, st, "admin", domain.RoleAdmin)
		req := httptest.NewRequest(http.MethodGet, "/admin/invites", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), adminAcc))
		rec := httptest.NewRecorder()
		handler.GETInvites(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body apimodel.AdminInviteList
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.NotNil(t, body.Invites)
	})
}

func TestAdminInvitesHandler_POSTInvites(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	regSvc := service.NewRegistrationService(st, nil, nil, "https://example.com", "Example")
	handler := NewAdminInvitesHandler(accountSvc, regSvc)
	adminAcc := createAccountWithRole(t, st, "admin", domain.RoleAdmin)

	t.Run("no account returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/invites", nil)
		rec := httptest.NewRecorder()
		handler.POSTInvites(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("admin returns 201 and created invite", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/invites", bytes.NewReader([]byte("{}")))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), adminAcc))
		rec := httptest.NewRecorder()
		handler.POSTInvites(rec, req)
		assert.Equal(t, http.StatusCreated, rec.Code)
		var body apimodel.AdminInvite
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.NotEmpty(t, body.ID)
		assert.NotEmpty(t, body.Code)
	})
}

func TestAdminInvitesHandler_DELETEInvite(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	regSvc := service.NewRegistrationService(st, nil, nil, "https://example.com", "Example")
	handler := NewAdminInvitesHandler(accountSvc, regSvc)
	adminAcc := createAccountWithRole(t, st, "admin", domain.RoleAdmin)

	t.Run("no account returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/admin/invites/01someid", nil)
		req = testutil.AddChiURLParam(req, "id", "01someid")
		rec := httptest.NewRecorder()
		handler.DELETEInvite(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("admin with valid id returns 204", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/admin/invites/01inviteid", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), adminAcc))
		req = testutil.AddChiURLParam(req, "id", "01inviteid")
		rec := httptest.NewRecorder()
		handler.DELETEInvite(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}
