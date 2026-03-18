//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/chairswithlegs/monstera/internal/config"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/chairswithlegs/monstera/internal/uid"
	"github.com/stretchr/testify/require"
)

func TestIntegration_PostgresStore(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	connString := store.DatabaseConnectionString(cfg, false)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, connString)
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

	t.Run("GetMonsteraSettings_UpdateMonsteraSettings", func(t *testing.T) {
		settings, err := s.GetMonsteraSettings(ctx)
		require.NoError(t, err)
		require.NotNil(t, settings)
		require.NotEmpty(t, string(settings.RegistrationMode))

		err = s.UpdateMonsteraSettings(ctx, &domain.MonsteraSettings{RegistrationMode: domain.MonsteraRegistrationModeClosed})
		require.NoError(t, err)
		settings, err = s.GetMonsteraSettings(ctx)
		require.NoError(t, err)
		require.Equal(t, domain.MonsteraRegistrationModeClosed, settings.RegistrationMode)
	})

	t.Run("GetAccountByID_populates_AvatarURL", func(t *testing.T) {
		accID := uid.New()
		apID := "https://join.example/users/bob_" + accID
		acc, err := s.CreateAccount(ctx, store.CreateAccountInput{
			ID:           accID,
			Username:     "bob_" + accID,
			PublicKey:    "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----",
			InboxURL:     "https://join.example/users/bob/inbox",
			OutboxURL:    "https://join.example/users/bob/outbox",
			FollowersURL: "https://join.example/users/bob/followers",
			FollowingURL: "https://join.example/users/bob/following",
			APID:         apID,
		})
		require.NoError(t, err)

		mediaID := uid.New()
		avatarURL := "https://cdn.example/avatar_" + mediaID + ".jpg"
		_, err = s.CreateMediaAttachment(ctx, store.CreateMediaAttachmentInput{
			ID:         mediaID,
			AccountID:  acc.ID,
			Type:       "image",
			StorageKey: "avatar_" + mediaID,
			URL:        avatarURL,
		})
		require.NoError(t, err)

		err = s.UpdateAccount(ctx, store.UpdateAccountInput{
			ID:            acc.ID,
			AvatarMediaID: &mediaID,
			AvatarURL:     &avatarURL,
		})
		require.NoError(t, err)

		got, err := s.GetAccountByID(ctx, acc.ID)
		require.NoError(t, err)
		require.Equal(t, avatarURL, got.AvatarURL)
		require.Empty(t, got.HeaderURL)
	})

	t.Run("GetAccountByID_populates_HeaderURL", func(t *testing.T) {
		accID := uid.New()
		apID := "https://join.example/users/carol_" + accID
		acc, err := s.CreateAccount(ctx, store.CreateAccountInput{
			ID:           accID,
			Username:     "carol_" + accID,
			PublicKey:    "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----",
			InboxURL:     "https://join.example/users/carol/inbox",
			OutboxURL:    "https://join.example/users/carol/outbox",
			FollowersURL: "https://join.example/users/carol/followers",
			FollowingURL: "https://join.example/users/carol/following",
			APID:         apID,
		})
		require.NoError(t, err)

		mediaID := uid.New()
		headerURL := "https://cdn.example/header_" + mediaID + ".jpg"
		_, err = s.CreateMediaAttachment(ctx, store.CreateMediaAttachmentInput{
			ID:         mediaID,
			AccountID:  acc.ID,
			Type:       "image",
			StorageKey: "header_" + mediaID,
			URL:        headerURL,
		})
		require.NoError(t, err)

		err = s.UpdateAccount(ctx, store.UpdateAccountInput{
			ID:            acc.ID,
			HeaderMediaID: &mediaID,
			HeaderURL:     &headerURL,
		})
		require.NoError(t, err)

		got, err := s.GetAccountByID(ctx, acc.ID)
		require.NoError(t, err)
		require.Empty(t, got.AvatarURL)
		require.Equal(t, headerURL, got.HeaderURL)
	})
}
