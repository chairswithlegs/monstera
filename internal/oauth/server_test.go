package oauth

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/chairswithlegs/monstera-fed/internal/cache"
	"github.com/chairswithlegs/monstera-fed/internal/store"
	"github.com/chairswithlegs/monstera-fed/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestServer_RegisterApplication(t *testing.T) {
	ctx := context.Background()
	c, err := cache.New(cache.Config{Driver: "memory"})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	srv := NewServer(testutil.NewFakeStore(), c, slog.Default())

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

	srv := NewServer(st, c, slog.Default())

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

	srv := NewServer(st, c, slog.Default())

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

	srv := NewServer(st, c, slog.Default())
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
