package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chairswithlegs/monstera-fed/internal/cache"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	oauthpkg "github.com/chairswithlegs/monstera-fed/internal/oauth"
	"github.com/chairswithlegs/monstera-fed/internal/service"
	"github.com/chairswithlegs/monstera-fed/internal/store"
	"github.com/chairswithlegs/monstera-fed/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithUser_UserFromContext(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	assert.Nil(t, UserFromContext(ctx))

	u := &domain.User{ID: "user1", AccountID: "acc1", Role: domain.RoleUser}
	ctx = WithUser(ctx, u)
	got := UserFromContext(ctx)
	require.NotNil(t, got)
	assert.Equal(t, "user1", got.ID)
	assert.Equal(t, domain.RoleUser, got.Role)
}

func TestRequireModerator(t *testing.T) {
	t.Parallel()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := RequireModerator()

	t.Run("no user in context returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		mw(next).ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("user with role user returns 403", func(t *testing.T) {
		u := &domain.User{ID: "u1", Role: domain.RoleUser}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(WithUser(req.Context(), u))
		rec := httptest.NewRecorder()
		mw(next).ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("user with role moderator passes", func(t *testing.T) {
		u := &domain.User{ID: "u1", Role: domain.RoleModerator}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(WithUser(req.Context(), u))
		rec := httptest.NewRecorder()
		mw(next).ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("user with role admin passes", func(t *testing.T) {
		u := &domain.User{ID: "u1", Role: domain.RoleAdmin}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(WithUser(req.Context(), u))
		rec := httptest.NewRecorder()
		mw(next).ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestRequireAdmin(t *testing.T) {
	t.Parallel()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := RequireAdmin()

	t.Run("no user in context returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		mw(next).ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("user with role moderator returns 403", func(t *testing.T) {
		u := &domain.User{ID: "u1", Role: domain.RoleModerator}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(WithUser(req.Context(), u))
		rec := httptest.NewRecorder()
		mw(next).ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("user with role user returns 403", func(t *testing.T) {
		u := &domain.User{ID: "u1", Role: domain.RoleUser}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(WithUser(req.Context(), u))
		rec := httptest.NewRecorder()
		mw(next).ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("user with role admin passes", func(t *testing.T) {
		u := &domain.User{ID: "u1", Role: domain.RoleAdmin}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(WithUser(req.Context(), u))
		rec := httptest.NewRecorder()
		mw(next).ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestRequireAuth_stores_account_and_user_in_context(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	c, err := cache.New(cache.Config{Driver: "memory"})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()
	oauthSrv := oauthpkg.NewServer(st, c, slog.Default())

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleModerator,
	})
	require.NoError(t, err)

	app, err := oauthSrv.RegisterApplication(ctx, "Test App", "https://example.com/cb", "read write", "")
	require.NoError(t, err)

	code, err := oauthSrv.AuthorizeRequest(ctx, oauthpkg.AuthorizeRequest{
		ApplicationID:       app.ID,
		AccountID:           acc.ID,
		RedirectURI:         "https://example.com/cb",
		Scopes:              "read write",
		CodeChallenge:       oauthpkg.GenerateCodeChallenge("verifier"),
		CodeChallengeMethod: "S256",
	})
	require.NoError(t, err)

	resp, err := oauthSrv.ExchangeCode(ctx, oauthpkg.TokenRequest{
		GrantType:    "authorization_code",
		Code:         code,
		RedirectURI:  "https://example.com/cb",
		ClientID:     app.ClientID,
		ClientSecret: app.ClientSecret,
		CodeVerifier: "verifier",
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.AccessToken)

	var nextCalled bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		gotAcc := AccountFromContext(r.Context())
		gotUser := UserFromContext(r.Context())
		assert.NotNil(t, gotAcc, "account should be in context")
		assert.NotNil(t, gotUser, "user should be in context")
		if gotAcc != nil {
			assert.Equal(t, acc.ID, gotAcc.ID)
		}
		if gotUser != nil {
			assert.Equal(t, acc.ID, gotUser.AccountID)
			assert.Equal(t, domain.RoleModerator, gotUser.Role)
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := RequireAuth(oauthSrv, accountSvc)(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+resp.AccessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.True(t, nextCalled, "next handler should have been called")
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireAuth_no_token_returns_401(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	c, err := cache.New(cache.Config{Driver: "memory"})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()
	oauthSrv := oauthpkg.NewServer(st, c, slog.Default())

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next should not be called")
	})
	handler := RequireAuth(oauthSrv, accountSvc)(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// Ensure OptionalAuth does not panic when account service returns nil account.
// This tests the defensive check: we only use account when err == nil && account != nil.
func TestOptionalAuth_nil_account_no_panic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	var getAccountWithUserCalled bool
	fakeAccounts := &fakeAccountService{
		getAccountWithUser: func(context.Context, string) (*domain.Account, *domain.User, error) {
			getAccountWithUserCalled = true
			return nil, nil, nil
		},
	}

	st := testutil.NewFakeStore()
	app, _ := st.CreateApplication(ctx, store.CreateApplicationInput{
		ID:           "app1",
		Name:         "App",
		ClientID:     "client",
		ClientSecret: "secret",
		RedirectURIs: "https://example.com/cb",
		Scopes:       "read write",
		Website:      nil,
	})
	require.NotNil(t, app)
	c, err := cache.New(cache.Config{Driver: "memory"})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()
	oauthSrv := oauthpkg.NewServer(st, c, slog.Default())
	tok, _ := st.CreateAccessToken(ctx, store.CreateAccessTokenInput{
		ID:            "tid1",
		Token:         "test-token-with-account",
		ApplicationID: app.ID,
		AccountID:     strPtr("nonexistent"),
		Scopes:        "read write",
	})
	require.NotNil(t, tok)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Nil(t, AccountFromContext(r.Context()))
		assert.Nil(t, UserFromContext(r.Context()))
		w.WriteHeader(http.StatusOK)
	})
	handler := OptionalAuth(oauthSrv, fakeAccounts)(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(ctx)
	req.Header.Set("Authorization", "Bearer test-token-with-account")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.True(t, getAccountWithUserCalled)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func strPtr(s string) *string { return &s }

type fakeAccountService struct {
	getAccountWithUser func(context.Context, string) (*domain.Account, *domain.User, error)
}

func (f *fakeAccountService) GetAccountWithUser(ctx context.Context, accountID string) (*domain.Account, *domain.User, error) {
	return f.getAccountWithUser(ctx, accountID)
}

func (f *fakeAccountService) GetByID(context.Context, string) (*domain.Account, error) {
	panic("unused")
}
func (f *fakeAccountService) GetByAPID(context.Context, string) (*domain.Account, error) {
	panic("unused")
}
func (f *fakeAccountService) GetLocalByUsername(context.Context, string) (*domain.Account, error) {
	panic("unused")
}
func (f *fakeAccountService) Create(context.Context, service.CreateAccountInput) (*domain.Account, error) {
	panic("unused")
}
func (f *fakeAccountService) CreateOrUpdateRemoteAccount(context.Context, service.CreateOrUpdateRemoteInput) (*domain.Account, error) {
	panic("unused")
}
func (f *fakeAccountService) Suspend(context.Context, string) error { panic("unused") }
func (f *fakeAccountService) GetByUsername(context.Context, string, *string) (*domain.Account, error) {
	panic("unused")
}
func (f *fakeAccountService) GetLocalActorForFederation(context.Context, string) (*domain.Account, error) {
	panic("unused")
}
func (f *fakeAccountService) GetLocalActorWithMedia(context.Context, string) (*service.LocalActorWithMedia, error) {
	panic("unused")
}
func (f *fakeAccountService) CountFollowers(context.Context, string) (int64, error) { panic("unused") }
func (f *fakeAccountService) CountFollowing(context.Context, string) (int64, error) { panic("unused") }
func (f *fakeAccountService) GetRelationship(context.Context, string, string) (*domain.Relationship, error) {
	panic("unused")
}
func (f *fakeAccountService) UpdateCredentials(context.Context, service.UpdateCredentialsInput) (*domain.Account, *domain.User, error) {
	panic("unused")
}
func (f *fakeAccountService) Register(context.Context, service.RegisterInput) (*domain.Account, error) {
	panic("unused")
}
func (f *fakeAccountService) ListLocalUsers(context.Context, int, int) ([]domain.User, error) {
	panic("unused")
}
func (f *fakeAccountService) GetUserByID(context.Context, string) (*domain.User, error) {
	panic("unused")
}
