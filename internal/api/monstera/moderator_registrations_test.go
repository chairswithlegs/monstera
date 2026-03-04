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
	"github.com/chairswithlegs/monstera-fed/internal/store"
	"github.com/chairswithlegs/monstera-fed/internal/testutil"
	"github.com/chairswithlegs/monstera-fed/internal/uid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testInstanceURL = "https://example.com"

func TestModeratorRegistrationsHandler_GETRegistrations(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	regSvc := service.NewRegistrationService(st, nil, nil, testInstanceURL, "Example")
	handler := NewModeratorRegistrationsHandler(regSvc)

	t.Run("returns 200 and pending list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/registrations", nil)
		rec := httptest.NewRecorder()
		handler.GETRegistrations(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body apimodel.AdminPendingRegistrationList
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.NotNil(t, body.Pending)
	})
}

func TestModeratorRegistrationsHandler_POSTApprove(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	regSvc := service.NewRegistrationService(st, nil, nil, testInstanceURL, "Example")
	handler := NewModeratorRegistrationsHandler(regSvc)
	adminAcc := createAccountWithRole(t, st, "admin", domain.RoleAdmin)
	adminUser := getUserByAccountID(t, st, adminAcc.ID)

	accID := uid.New()
	_, err := st.CreateAccount(ctx, store.CreateAccountInput{
		ID:           accID,
		Username:     "pendinguser",
		Domain:       nil,
		PublicKey:    "pk",
		InboxURL:     testInstanceURL + "/users/pendinguser/inbox",
		OutboxURL:    testInstanceURL + "/users/pendinguser/outbox",
		FollowersURL: testInstanceURL + "/users/pendinguser/followers",
		FollowingURL: testInstanceURL + "/users/pendinguser/following",
		APID:         testInstanceURL + "/users/pendinguser",
	})
	require.NoError(t, err)
	userID := uid.New()
	_, err = st.CreateUser(ctx, store.CreateUserInput{
		ID:           userID,
		AccountID:    accID,
		Email:        "pending@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("no user returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/registrations/"+userID+"/approve", nil)
		req = testutil.AddChiURLParam(req, "id", userID)
		rec := httptest.NewRecorder()
		handler.POSTApprove(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("with user returns 204", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/registrations/"+userID+"/approve", nil)
		req = req.WithContext(middleware.WithUser(req.Context(), adminUser))
		req = testutil.AddChiURLParam(req, "id", userID)
		rec := httptest.NewRecorder()
		handler.POSTApprove(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}

func TestModeratorRegistrationsHandler_POSTReject(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	regSvc := service.NewRegistrationService(st, nil, nil, testInstanceURL, "Example")
	handler := NewModeratorRegistrationsHandler(regSvc)
	adminAcc := createAccountWithRole(t, st, "admin", domain.RoleAdmin)
	adminUser := getUserByAccountID(t, st, adminAcc.ID)

	accID := uid.New()
	_, err := st.CreateAccount(ctx, store.CreateAccountInput{
		ID:           accID,
		Username:     "rejectuser",
		Domain:       nil,
		PublicKey:    "pk",
		InboxURL:     testInstanceURL + "/users/rejectuser/inbox",
		OutboxURL:    testInstanceURL + "/users/rejectuser/outbox",
		FollowersURL: testInstanceURL + "/users/rejectuser/followers",
		FollowingURL: testInstanceURL + "/users/rejectuser/following",
		APID:         testInstanceURL + "/users/rejectuser",
	})
	require.NoError(t, err)
	userID := uid.New()
	_, err = st.CreateUser(ctx, store.CreateUserInput{
		ID:           userID,
		AccountID:    accID,
		Email:        "reject@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("with user returns 204", func(t *testing.T) {
		body := map[string]string{"reason": "not allowed"}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/admin/registrations/"+userID+"/reject", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithUser(req.Context(), adminUser))
		req = testutil.AddChiURLParam(req, "id", userID)
		rec := httptest.NewRecorder()
		handler.POSTReject(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}
