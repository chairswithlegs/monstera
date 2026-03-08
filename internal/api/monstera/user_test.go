package monstera

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserHandler_GETUser(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	handler := NewUserHandler(accountSvc)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/monstera/api/v1/user", nil)
		rec := httptest.NewRecorder()
		handler.GETUser(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("authenticated returns 200 and user", func(t *testing.T) {
		u := &domain.User{
			ID:               "01USER",
			AccountID:        "01ACC",
			Email:            "alice@example.com",
			Role:             domain.RoleUser,
			DefaultPrivacy:   "public",
			DefaultSensitive: false,
			DefaultLanguage:  "en",
			CreatedAt:        time.Now(),
		}
		req := httptest.NewRequest(http.MethodGet, "/monstera/api/v1/user", nil)
		req = req.WithContext(middleware.WithUser(req.Context(), u))
		rec := httptest.NewRecorder()
		handler.GETUser(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, "01USER", body["id"])
		assert.Equal(t, "01ACC", body["account_id"])
		assert.Equal(t, "alice@example.com", body["email"])
		assert.Equal(t, "user", body["role"])
	})
}
