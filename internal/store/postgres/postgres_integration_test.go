//go:build integration

package postgres

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/store"
	"github.com/chairswithlegs/monstera-fed/internal/testutil"
	"github.com/chairswithlegs/monstera-fed/internal/uid"
	"github.com/stretchr/testify/require"
)

func TestIntegration_PostgresStore(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	require.NotEmpty(t, url, "DATABASE_URL must be set for integration test")
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	s := New(pool).(*PostgresStore)

	t.Run("GetAccountByID_not_found", func(t *testing.T) {
		_, err := s.GetAccountByID(ctx, "01nonexistent")
		require.Error(t, err)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("CreateAccount_GetAccountByID", func(t *testing.T) {
		id := uid.New()
		apID := "https://test.example/users/alice"
		inbox := "https://test.example/users/alice/inbox"
		outbox := "https://test.example/users/alice/outbox"
		followers := "https://test.example/users/alice/followers"
		following := "https://test.example/users/alice/following"
		in := store.CreateAccountInput{
			ID:           id,
			Username:     "alice",
			Domain:       testutil.StrPtr("test.example"),
			DisplayName:  testutil.StrPtr("Alice"),
			Note:         nil,
			PublicKey:    "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA\n-----END PUBLIC KEY-----",
			PrivateKey:   nil,
			InboxURL:     inbox,
			OutboxURL:    outbox,
			FollowersURL: followers,
			FollowingURL: following,
			APID:         apID,
			ApRaw:        nil,
			Bot:          false,
			Locked:       false,
		}
		acc, err := s.CreateAccount(ctx, in)
		require.NoError(t, err)
		require.NotNil(t, acc)
		require.Equal(t, id, acc.ID)
		require.Equal(t, "alice", acc.Username)

		got, err := s.GetAccountByID(ctx, id)
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Equal(t, id, got.ID)
		require.Equal(t, "alice", got.Username)
	})

	t.Run("GetSetting_ListSettings", func(t *testing.T) {
		val, err := s.GetSetting(ctx, "instance_name")
		require.NoError(t, err)
		require.NotEmpty(t, val)

		settings, err := s.ListSettings(ctx)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(settings), 1)
	})

	t.Run("CountLocalAccounts_CountRemoteAccounts", func(t *testing.T) {
		local, err := s.CountLocalAccounts(ctx)
		require.NoError(t, err)
		remote, err := s.CountRemoteAccounts(ctx)
		require.NoError(t, err)
		require.GreaterOrEqual(t, local, int64(0))
		require.GreaterOrEqual(t, remote, int64(0))
	})
}
