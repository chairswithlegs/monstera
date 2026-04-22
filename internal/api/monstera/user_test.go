package monstera

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestHandler(t *testing.T) (*UserHandler, *domain.User, *domain.Account) {
	t.Helper()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	handler := NewUserHandler(accountSvc)
	u := &domain.User{
		ID:                 "01USER",
		AccountID:          "01ACC",
		Email:              "alice@example.com",
		PasswordHash:       "",
		Role:               domain.RoleUser,
		DefaultPrivacy:     "public",
		DefaultSensitive:   false,
		DefaultLanguage:    "en",
		DefaultQuotePolicy: "public",
		CreatedAt:          time.Now(),
	}
	a := &domain.Account{
		ID:       "01ACC",
		Username: "alice",
	}
	require.NoError(t, st.SeedUserAndAccount(u, a))
	return handler, u, a
}

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
		a := &domain.Account{
			ID:       "01ACC",
			Username: "alice",
		}
		req := httptest.NewRequest(http.MethodGet, "/monstera/api/v1/user", nil)
		ctx := middleware.WithUser(req.Context(), u)
		ctx = middleware.WithAccount(ctx, a)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		handler.GETUser(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, "01USER", body["id"])
		assert.Equal(t, "01ACC", body["account_id"])
		assert.Equal(t, "alice", body["username"])
		assert.Equal(t, "alice@example.com", body["email"])
		assert.Equal(t, "user", body["role"])
	})
}

func TestUserHandler_PATCHProfile(t *testing.T) {
	t.Parallel()

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		t.Parallel()
		handler, _, _ := newTestHandler(t)
		req := httptest.NewRequest(http.MethodPatch, "/monstera/api/v1/account/profile", nil)
		rec := httptest.NewRecorder()
		handler.PATCHProfile(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("authenticated updates profile and returns 200", func(t *testing.T) {
		t.Parallel()
		handler, u, a := newTestHandler(t)
		displayName := "Alice Wonderland"
		payload, _ := json.Marshal(map[string]any{
			"display_name": displayName,
			"locked":       false,
			"bot":          false,
		})
		req := httptest.NewRequest(http.MethodPatch, "/monstera/api/v1/account/profile", bytes.NewReader(payload))
		ctx := middleware.WithUser(req.Context(), u)
		ctx = middleware.WithAccount(ctx, a)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		handler.PATCHProfile(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, displayName, body["display_name"])
	})
}

func TestUserHandler_PATCHPreferences(t *testing.T) {
	t.Parallel()

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		t.Parallel()
		handler, _, _ := newTestHandler(t)
		req := httptest.NewRequest(http.MethodPatch, "/monstera/api/v1/account/preferences", nil)
		rec := httptest.NewRecorder()
		handler.PATCHPreferences(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("invalid default_privacy returns 422", func(t *testing.T) {
		t.Parallel()
		handler, u, a := newTestHandler(t)
		payload, _ := json.Marshal(map[string]any{
			"default_privacy":      "invalid",
			"default_quote_policy": "public",
		})
		req := httptest.NewRequest(http.MethodPatch, "/monstera/api/v1/account/preferences", bytes.NewReader(payload))
		ctx := middleware.WithUser(req.Context(), u)
		ctx = middleware.WithAccount(ctx, a)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		handler.PATCHPreferences(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("valid preferences returns 200", func(t *testing.T) {
		t.Parallel()
		handler, u, a := newTestHandler(t)
		payload, _ := json.Marshal(map[string]any{
			"default_privacy":      "unlisted",
			"default_sensitive":    true,
			"default_language":     "fr",
			"default_quote_policy": "followers",
		})
		req := httptest.NewRequest(http.MethodPatch, "/monstera/api/v1/account/preferences", bytes.NewReader(payload))
		ctx := middleware.WithUser(req.Context(), u)
		ctx = middleware.WithAccount(ctx, a)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		handler.PATCHPreferences(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, "unlisted", body["default_privacy"])
		assert.Equal(t, "followers", body["default_quote_policy"])
	})
}

func TestUserHandler_PATCHEmail(t *testing.T) {
	t.Parallel()

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		t.Parallel()
		handler, _, _ := newTestHandler(t)
		req := httptest.NewRequest(http.MethodPatch, "/monstera/api/v1/account/security/email", nil)
		rec := httptest.NewRecorder()
		handler.PATCHEmail(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("missing email returns 422", func(t *testing.T) {
		t.Parallel()
		handler, u, a := newTestHandler(t)
		payload, _ := json.Marshal(map[string]any{"email": ""})
		req := httptest.NewRequest(http.MethodPatch, "/monstera/api/v1/account/security/email", bytes.NewReader(payload))
		ctx := middleware.WithUser(req.Context(), u)
		ctx = middleware.WithAccount(ctx, a)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		handler.PATCHEmail(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("valid email update returns 200", func(t *testing.T) {
		t.Parallel()
		handler, u, a := newTestHandler(t)
		payload, _ := json.Marshal(map[string]any{"email": "new@example.com"})
		req := httptest.NewRequest(http.MethodPatch, "/monstera/api/v1/account/security/email", bytes.NewReader(payload))
		ctx := middleware.WithUser(req.Context(), u)
		ctx = middleware.WithAccount(ctx, a)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		handler.PATCHEmail(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, "new@example.com", body["email"])
	})
}

func TestUserHandler_PATCHPassword(t *testing.T) {
	t.Parallel()

	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.DefaultCost)
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		t.Parallel()
		handler, _, _ := newTestHandler(t)
		req := httptest.NewRequest(http.MethodPatch, "/monstera/api/v1/account/security/password", nil)
		rec := httptest.NewRecorder()
		handler.PATCHPassword(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("wrong current password returns 403", func(t *testing.T) {
		t.Parallel()
		st := testutil.NewFakeStore()
		accountSvc := service.NewAccountService(st, "https://example.com")
		handler := NewUserHandler(accountSvc)
		u := &domain.User{
			ID:           "01USER",
			AccountID:    "01ACC",
			Email:        "alice@example.com",
			PasswordHash: string(hash),
			CreatedAt:    time.Now(),
		}
		a := &domain.Account{ID: "01ACC", Username: "alice"}
		require.NoError(t, st.SeedUserAndAccount(u, a))

		payload, _ := json.Marshal(map[string]any{
			"current_password": "wrong-password",
			"new_password":     "new-secret",
		})
		req := httptest.NewRequest(http.MethodPatch, "/monstera/api/v1/account/security/password", bytes.NewReader(payload))
		ctx := middleware.WithUser(req.Context(), u)
		ctx = middleware.WithAccount(ctx, a)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		handler.PATCHPassword(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("correct password returns 204", func(t *testing.T) {
		t.Parallel()
		st := testutil.NewFakeStore()
		accountSvc := service.NewAccountService(st, "https://example.com")
		handler := NewUserHandler(accountSvc)
		u := &domain.User{
			ID:           "01USER",
			AccountID:    "01ACC",
			Email:        "alice@example.com",
			PasswordHash: string(hash),
			CreatedAt:    time.Now(),
		}
		a := &domain.Account{ID: "01ACC", Username: "alice"}
		require.NoError(t, st.SeedUserAndAccount(u, a))

		payload, _ := json.Marshal(map[string]any{
			"current_password": "correct-password",
			"new_password":     "new-secret",
		})
		req := httptest.NewRequest(http.MethodPatch, "/monstera/api/v1/account/security/password", bytes.NewReader(payload))
		ctx := middleware.WithUser(req.Context(), u)
		ctx = middleware.WithAccount(ctx, a)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		handler.PATCHPassword(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("missing new_password returns 422", func(t *testing.T) {
		t.Parallel()
		handler, u, a := newTestHandler(t)
		payload, _ := json.Marshal(map[string]any{
			"current_password": "some-pass",
			"new_password":     "",
		})
		req := httptest.NewRequest(http.MethodPatch, "/monstera/api/v1/account/security/password", bytes.NewReader(payload))
		ctx := middleware.WithUser(req.Context(), u)
		ctx = middleware.WithAccount(ctx, a)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		handler.PATCHPassword(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})
}

func TestUserHandler_DELETEUser(t *testing.T) {
	t.Parallel()

	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	require.NoError(t, err)

	type fixture struct {
		handler *UserHandler
		store   *testutil.FakeStore
		user    *domain.User
		account *domain.Account
	}
	seed := func(t *testing.T) fixture {
		t.Helper()
		st := testutil.NewFakeStore()
		accountSvc := service.NewAccountService(st, "https://example.com")
		handler := NewUserHandler(accountSvc)
		u := &domain.User{
			ID: "01USER", AccountID: "01ACC", Email: "alice@example.com",
			PasswordHash: string(hash), CreatedAt: time.Now(),
		}
		pk := "-----BEGIN RSA PRIVATE KEY-----\nstub\n-----END RSA PRIVATE KEY-----"
		a := &domain.Account{ID: "01ACC", Username: "alice", APID: "https://example.com/users/alice", PrivateKey: &pk}
		require.NoError(t, st.SeedUserAndAccount(u, a))
		return fixture{handler: handler, store: st, user: u, account: a}
	}

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		t.Parallel()
		handler, _, _ := newTestHandler(t)
		req := httptest.NewRequest(http.MethodDelete, "/monstera/api/v1/user", nil)
		rec := httptest.NewRecorder()
		handler.DELETEUser(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("missing current_password returns 422", func(t *testing.T) {
		t.Parallel()
		f := seed(t)
		payload, _ := json.Marshal(map[string]any{"current_password": ""})
		req := httptest.NewRequest(http.MethodDelete, "/monstera/api/v1/user", bytes.NewReader(payload))
		req = req.WithContext(middleware.WithUser(req.Context(), f.user))
		rec := httptest.NewRecorder()
		f.handler.DELETEUser(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("wrong password returns 403 and account intact", func(t *testing.T) {
		t.Parallel()
		f := seed(t)
		payload, _ := json.Marshal(map[string]any{"current_password": "nope"})
		req := httptest.NewRequest(http.MethodDelete, "/monstera/api/v1/user", bytes.NewReader(payload))
		req = req.WithContext(middleware.WithUser(req.Context(), f.user))
		rec := httptest.NewRecorder()
		f.handler.DELETEUser(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)

		acc, err := f.store.GetAccountByID(req.Context(), f.account.ID)
		require.NoError(t, err)
		assert.False(t, acc.Suspended)
	})

	t.Run("correct password returns 204 and hard-deletes account", func(t *testing.T) {
		t.Parallel()
		f := seed(t)
		payload, _ := json.Marshal(map[string]any{"current_password": "correct-password"})
		req := httptest.NewRequest(http.MethodDelete, "/monstera/api/v1/user", bytes.NewReader(payload))
		req = req.WithContext(middleware.WithUser(req.Context(), f.user))
		rec := httptest.NewRecorder()
		f.handler.DELETEUser(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)

		_, err := f.store.GetAccountByID(req.Context(), f.account.ID)
		require.ErrorIs(t, err, domain.ErrNotFound)
		_, err = f.store.GetUserByAccountID(req.Context(), f.account.ID)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})
}
