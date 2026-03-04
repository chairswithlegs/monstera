package monstera

import (
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

func createAccountWithRole(t *testing.T, st store.Store, username, role string) *domain.Account {
	t.Helper()
	ctx := context.Background()
	accID := uid.New()
	baseURL := "https://example.com"
	acc, err := st.CreateAccount(ctx, store.CreateAccountInput{
		ID:           accID,
		Username:     username,
		Domain:       nil,
		PublicKey:    "test-public-key",
		InboxURL:     baseURL + "/users/" + username + "/inbox",
		OutboxURL:    baseURL + "/users/" + username + "/outbox",
		FollowersURL: baseURL + "/users/" + username + "/followers",
		FollowingURL: baseURL + "/users/" + username + "/following",
		APID:         baseURL + "/users/" + username,
	})
	require.NoError(t, err)
	_, err = st.CreateUser(ctx, store.CreateUserInput{
		ID:           uid.New(),
		AccountID:    acc.ID,
		Email:        username + "@example.com",
		PasswordHash: "hash",
		Role:         role,
	})
	require.NoError(t, err)
	return acc
}

func TestAdminDashboardHandler_GETDashboard(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	instanceSvc := service.NewInstanceService(st)
	modSvc := service.NewModerationService(st)
	handler := NewAdminDashboardHandler(instanceSvc, modSvc)

	t.Run("with admin account returns 200 and dashboard body", func(t *testing.T) {
		adminAcc := createAccountWithRole(t, st, "admin", domain.RoleAdmin)
		req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), adminAcc))
		rec := httptest.NewRecorder()
		handler.GETDashboard(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body apimodel.AdminDashboard
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, int64(1), body.LocalUsersCount)
		assert.GreaterOrEqual(t, body.LocalStatusesCount, int64(0))
		assert.GreaterOrEqual(t, body.OpenReportsCount, 0)
	})
}
