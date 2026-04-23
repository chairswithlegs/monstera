package oauth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/cache"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
)

func TestServer_RegisterApplication(t *testing.T) {
	ctx := context.Background()
	c, err := cache.New(cache.Config{Driver: "memory"})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	srv := NewServer(testutil.NewFakeStore(), c, "")

	app, err := srv.RegisterApplication(ctx, "Test App", "https://app.example/cb", "read write", "https://app.example")
	require.NoError(t, err)
	require.NotNil(t, app)
	require.NotEmpty(t, app.ID)
	require.Equal(t, "Test App", app.Name)
	require.Len(t, app.ClientID, 64)
	require.Len(t, app.ClientSecret, 64)
	require.Equal(t, "https://app.example/cb", app.RedirectURI)
	require.Empty(t, app.VapidKey)
}

func TestServer_AuthorizeRequest_ExchangeCode(t *testing.T) {
	ctx := context.Background()
	st := testutil.NewFakeStore()
	c, err := cache.New(cache.Config{Driver: "memory"})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	app, _ := st.CreateApplication(ctx, store.CreateApplicationInput{
		ID:           "app1",
		Name:         "App",
		ClientID:     "client123",
		ClientSecret: "secret456",
		RedirectURIs: "https://app.example/cb",
		Scopes:       "read write",
		Website:      nil,
	})
	require.NotNil(t, app)

	srv := NewServer(st, c, "")

	code, err := srv.AuthorizeRequest(ctx, AuthorizeRequest{
		ApplicationID:       app.ID,
		AccountID:           "acc1",
		RedirectURI:         "https://app.example/cb",
		Scopes:              "read write",
		CodeChallenge:       GenerateCodeChallenge("verifier123"),
		CodeChallengeMethod: "S256",
	})
	require.NoError(t, err)
	require.NotEmpty(t, code)

	resp, err := srv.ExchangeCode(ctx, TokenRequest{
		GrantType:    "authorization_code",
		Code:         code,
		RedirectURI:  "https://app.example/cb",
		ClientID:     "client123",
		ClientSecret: "secret456",
		CodeVerifier: "verifier123",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotEmpty(t, resp.AccessToken)
	require.Equal(t, "Bearer", resp.TokenType)
	require.Contains(t, resp.Scope, "read")

	claims, err := srv.LookupToken(ctx, resp.AccessToken)
	require.NoError(t, err)
	require.Equal(t, "acc1", claims.AccountID)
	require.Equal(t, app.ID, claims.ApplicationID)
	require.True(t, claims.Scopes.HasScope("read:statuses"))
}

func TestServer_ExchangeClientCredentials(t *testing.T) {
	ctx := context.Background()
	st := testutil.NewFakeStore()
	c, err := cache.New(cache.Config{Driver: "memory"})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	app, _ := st.CreateApplication(ctx, store.CreateApplicationInput{
		ID:           "app1",
		Name:         "App",
		ClientID:     "client",
		ClientSecret: "secret",
		RedirectURIs: "https://app.example/cb",
		Scopes:       "read write",
		Website:      nil,
	})
	require.NotNil(t, app)

	srv := NewServer(st, c, "")

	resp, err := srv.ExchangeClientCredentials(ctx, TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     "client",
		ClientSecret: "secret",
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.AccessToken)

	claims, err := srv.LookupToken(ctx, resp.AccessToken)
	require.NoError(t, err)
	require.Empty(t, claims.AccountID)
	require.True(t, claims.Scopes.HasScope("read"))
	require.False(t, claims.Scopes.HasScope("write"))
}

func TestServer_RevokeToken(t *testing.T) {
	ctx := context.Background()
	st := testutil.NewFakeStore()
	c, err := cache.New(cache.Config{Driver: "memory"})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	app, _ := st.CreateApplication(ctx, store.CreateApplicationInput{
		ID:           "app1",
		Name:         "App",
		ClientID:     "c",
		ClientSecret: "s",
		RedirectURIs: "https://x/cb",
		Scopes:       "read",
		Website:      nil,
	})
	require.NotNil(t, app)

	srv := NewServer(st, c, "")
	resp, err := srv.ExchangeClientCredentials(ctx, TokenRequest{
		GrantType: "client_credentials", ClientID: "c", ClientSecret: "s",
	})
	require.NoError(t, err)
	token := resp.AccessToken

	_, err = srv.LookupToken(ctx, token)
	require.NoError(t, err)

	err = srv.RevokeToken(ctx, token)
	require.NoError(t, err)

	// Cache eviction may be eventually consistent (e.g. ristretto), so retry
	// until LookupToken returns an error or we timeout.
	require.Eventually(t, func() bool {
		_, err := srv.LookupToken(ctx, token)
		return err != nil
	}, 500*time.Millisecond, 10*time.Millisecond,
		"LookupToken should return an error after RevokeToken (cache eviction may be eventual)")
}

func TestServer_GetApplicationInfo(t *testing.T) {
	ctx := context.Background()
	st := testutil.NewFakeStore()
	c, err := cache.New(cache.Config{Driver: "memory"})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	website := "https://example.com"
	app, err := st.CreateApplication(ctx, store.CreateApplicationInput{
		ID: "app_info_1", Name: "Tusky", ClientID: "cid", ClientSecret: "secret",
		RedirectURIs: "urn:ietf:wg:oauth:2.0:oob", Scopes: "read write", Website: &website,
	})
	require.NoError(t, err)

	srv := NewServer(st, c, "vapid-pub")
	info, err := srv.GetApplicationInfo(ctx, app.ID)
	require.NoError(t, err)
	assert.Equal(t, app.ID, info.ID)
	assert.Equal(t, "Tusky", info.Name)
	require.NotNil(t, info.Website)
	assert.Equal(t, "https://example.com", *info.Website)
	assert.Equal(t, "vapid-pub", info.VapidKey)
}

func TestServer_ListAuthorizedApplications(t *testing.T) {
	ctx := context.Background()
	st := testutil.NewFakeStore()
	c, err := cache.New(cache.Config{Driver: "memory"})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	accountID := "acct_abc"
	app, err := st.CreateApplication(ctx, store.CreateApplicationInput{
		ID: "app_list_1", Name: "Ivory", ClientID: "ivory_cid", ClientSecret: "ivory_secret",
		RedirectURIs: "monstera://callback", Scopes: "read write",
	})
	require.NoError(t, err)

	_, err = st.CreateAccessToken(ctx, store.CreateAccessTokenInput{
		ID: "tok_list_1", ApplicationID: app.ID, AccountID: &accountID,
		Token: "tok_list_raw", Scopes: "read write",
	})
	require.NoError(t, err)

	srv := NewServer(st, c, "")
	apps, err := srv.ListAuthorizedApplications(ctx, accountID)
	require.NoError(t, err)
	require.Len(t, apps, 1)
	assert.Equal(t, app.ID, apps[0].ID)
	assert.Equal(t, []string{"read", "write"}, apps[0].Scopes)

	empty, err := srv.ListAuthorizedApplications(ctx, "other_account")
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func TestServer_RevokeApplicationAuthorization(t *testing.T) {
	ctx := context.Background()
	st := testutil.NewFakeStore()
	c, err := cache.New(cache.Config{Driver: "memory"})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	accountID := "acct_xyz"
	app, err := st.CreateApplication(ctx, store.CreateApplicationInput{
		ID: "app_rev_1", Name: "Elk", ClientID: "elk_cid", ClientSecret: "elk_secret",
		RedirectURIs: "monstera://callback", Scopes: "read write",
	})
	require.NoError(t, err)

	rawToken := "rev_tok_" + app.ID
	_, err = st.CreateAccessToken(ctx, store.CreateAccessTokenInput{
		ID: "tok_rev_1", ApplicationID: app.ID, AccountID: &accountID,
		Token: rawToken, Scopes: "read",
	})
	require.NoError(t, err)

	srv := NewServer(st, c, "")

	// Prime the cache.
	_, err = srv.LookupToken(ctx, rawToken)
	require.NoError(t, err)

	require.NoError(t, srv.RevokeApplicationAuthorization(ctx, accountID, app.ID))

	require.Eventually(t, func() bool {
		_, err := srv.LookupToken(ctx, rawToken)
		return err != nil
	}, 500*time.Millisecond, 10*time.Millisecond, "token should no longer resolve after revoke")

	// Second call returns ErrNotFound now that there are no active tokens.
	err = srv.RevokeApplicationAuthorization(ctx, accountID, app.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
}
