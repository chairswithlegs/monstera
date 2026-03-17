//go:build integration

package postgres

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/config"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/chairswithlegs/monstera/internal/uid"
)

func setupTestStore(t *testing.T) (*PostgresStore, context.Context) {
	t.Helper()
	cfg, err := config.Load()
	require.NoError(t, err)
	connString := store.DatabaseConnectionString(cfg, false)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, connString)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	s := New(pool).(*PostgresStore)
	return s, ctx
}

const testPublicKey = "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA\n-----END PUBLIC KEY-----"

func createTestRemoteAccount(t *testing.T, s *PostgresStore, ctx context.Context) *domain.Account {
	t.Helper()
	id := uid.New()
	acc, err := s.CreateAccount(ctx, store.CreateAccountInput{
		ID:           id,
		Username:     "user_" + id,
		Domain:       testutil.StrPtr("remote.example"),
		DisplayName:  testutil.StrPtr("Test User " + id[:8]),
		PublicKey:    testPublicKey,
		InboxURL:     "https://remote.example/users/" + id + "/inbox",
		OutboxURL:    "https://remote.example/users/" + id + "/outbox",
		FollowersURL: "https://remote.example/users/" + id + "/followers",
		FollowingURL: "https://remote.example/users/" + id + "/following",
		APID:         "https://remote.example/users/" + id,
	})
	require.NoError(t, err)
	return acc
}

func createTestLocalAccount(t *testing.T, s *PostgresStore, ctx context.Context) *domain.Account {
	t.Helper()
	id := uid.New()
	privKey := "-----BEGIN RSA PRIVATE KEY-----\ntest_" + id + "\n-----END RSA PRIVATE KEY-----"
	acc, err := s.CreateAccount(ctx, store.CreateAccountInput{
		ID:           id,
		Username:     "local_" + id,
		PublicKey:    testPublicKey,
		PrivateKey:   &privKey,
		InboxURL:     "https://local.example/users/" + id + "/inbox",
		OutboxURL:    "https://local.example/users/" + id + "/outbox",
		FollowersURL: "https://local.example/users/" + id + "/followers",
		FollowingURL: "https://local.example/users/" + id + "/following",
		APID:         "https://local.example/users/" + id,
	})
	require.NoError(t, err)
	return acc
}

func createTestLocalAccountWithUser(t *testing.T, s *PostgresStore, ctx context.Context) (*domain.Account, *domain.User) {
	t.Helper()
	acc := createTestLocalAccount(t, s, ctx)
	userID := uid.New()
	u, err := s.CreateUser(ctx, store.CreateUserInput{
		ID:           userID,
		AccountID:    acc.ID,
		Email:        "user_" + userID + "@test.example",
		PasswordHash: "$2a$10$fakehashfakehashfakehashfakehashfakehashfakehashfake",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	return acc, u
}

func createTestStatus(t *testing.T, s *PostgresStore, ctx context.Context, accountID string) *domain.Status {
	t.Helper()
	id := uid.New()
	st, err := s.CreateStatus(ctx, store.CreateStatusInput{
		ID:                  id,
		URI:                 "https://local.example/statuses/" + id,
		AccountID:           accountID,
		Text:                testutil.StrPtr("Hello world " + id[:8]),
		Content:             testutil.StrPtr("<p>Hello world " + id[:8] + "</p>"),
		Visibility:          domain.VisibilityPublic,
		Language:            testutil.StrPtr("en"),
		APID:                "https://local.example/statuses/" + id,
		QuoteApprovalPolicy: domain.QuotePolicyPublic,
		Local:               true,
	})
	require.NoError(t, err)
	return st
}

func createTestOAuthApp(t *testing.T, s *PostgresStore, ctx context.Context) *domain.OAuthApplication {
	t.Helper()
	id := uid.New()
	clientID := uid.New()
	app, err := s.CreateApplication(ctx, store.CreateApplicationInput{
		ID:           id,
		Name:         "TestApp_" + id[:8],
		ClientID:     clientID,
		ClientSecret: "secret_" + id,
		RedirectURIs: "urn:ietf:wg:oauth:2.0:oob",
		Scopes:       "read write follow push",
	})
	require.NoError(t, err)
	return app
}

func intPtr(n int) *int {
	return &n
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}
