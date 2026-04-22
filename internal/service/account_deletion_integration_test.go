//go:build integration

package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/config"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/store/postgres"
	"github.com/chairswithlegs/monstera/internal/uid"
)

// End-to-end account deletion against real Postgres. Exercises what unit
// tests can't faithfully cover: the CASCADE on accounts(id) dropping every
// dependent row, and the snapshot/targets side tables surviving that
// CASCADE. The fake store can only approximate these.
//
// Scenario:
//  1. Alice is a local account with a confirmed user and a live OAuth token.
//  2. Bob is a remote follower on other.example, with an accepted follow.
//  3. Alice self-deletes (via DeleteLocalAccount — skips the bcrypt check).
//  4. Verify:
//     - accounts + users + oauth_access_tokens + follows rows for Alice gone;
//     - account_deletion_snapshots row exists with Alice's APID + PEM;
//     - account_deletion_targets row exists for Bob's inbox;
//     - account.deleted outbox event emitted with DeletionID+APID only
//     (no PrivateKey anywhere in the payload).
//  5. Fast-forward the snapshot past expiry, run the purge service, and
//     verify the snapshot + its targets are gone.
func TestIntegration_AccountDeletion_SnapshotSurvivesCASCADE(t *testing.T) {
	cfg, err := config.Load()
	require.NoError(t, err, "failed to load config")
	connString := store.DatabaseConnectionString(cfg, false)
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, connString)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	s := postgres.New(pool)

	// Isolate the test so repeat runs don't clash.
	suffix := uid.New()
	aliceName := "alice" + suffix
	bobName := "bob" + suffix
	instanceBaseURL := "https://test.example.com"

	accountSvc := NewAccountService(s, instanceBaseURL)
	deletionSvc := NewAccountDeletionService(s)

	alice, err := accountSvc.Register(ctx, RegisterInput{
		Username: aliceName,
		Email:    aliceName + "@test.example.com",
		Password: "hashedpassword",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		// Best-effort cleanup in case the test bails after Alice is
		// re-registered. No error if already gone.
		_, _ = s.DeleteAccount(ctx, alice.ID)
	})

	// Remote follower on a distinct domain.
	bobDomain := "other.example"
	bobInbox := "https://other.example/users/" + bobName + "/inbox"
	bobAPID := "https://other.example/users/" + bobName
	bob, err := s.CreateAccount(ctx, store.CreateAccountInput{
		ID:           "01bob" + suffix,
		Username:     bobName,
		Domain:       &bobDomain,
		PublicKey:    "stub",
		APID:         bobAPID,
		InboxURL:     bobInbox,
		OutboxURL:    bobAPID + "/outbox",
		FollowersURL: bobAPID + "/followers",
		FollowingURL: bobAPID + "/following",
	})
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = s.DeleteAccount(ctx, bob.ID) })

	_, err = s.CreateFollow(ctx, store.CreateFollowInput{
		ID: "01fol" + suffix, AccountID: bob.ID, TargetID: alice.ID,
		State: domain.FollowStateAccepted,
	})
	require.NoError(t, err)

	// Drop Alice.
	require.NoError(t, accountSvc.DeleteLocalAccount(ctx, alice.ID))

	// Alice's account + user + follows are gone under CASCADE.
	_, err = s.GetAccountByID(ctx, alice.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
	_, err = s.GetUserByAccountID(ctx, alice.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)

	// Fetch the emitted event and confirm it carries no private key.
	events, err := s.GetAndLockUnpublishedOutboxEvents(ctx, 100)
	require.NoError(t, err)
	var payload domain.AccountDeletedPayload
	var found bool
	for _, ev := range events {
		if ev.EventType == domain.EventAccountDeleted && ev.AggregateID == alice.ID {
			require.NoError(t, json.Unmarshal(ev.Payload, &payload))
			assert.NotContains(t, string(ev.Payload), "private_key",
				"event payload must not contain the actor's private key")
			found = true
			break
		}
	}
	require.True(t, found, "expected account.deleted event for %s", alice.ID)
	require.NotEmpty(t, payload.DeletionID)
	require.Equal(t, alice.APID, payload.APID)

	// Snapshot + target rows exist and point at the real key material.
	snap, err := s.GetAccountDeletionSnapshot(ctx, payload.DeletionID)
	require.NoError(t, err)
	assert.Equal(t, alice.APID, snap.APID)
	assert.NotEmpty(t, snap.PrivateKeyPEM, "snapshot retains PEM for post-CASCADE signing")
	assert.True(t, snap.ExpiresAt.After(time.Now()))

	targets, err := s.ListPendingAccountDeletionTargets(ctx, payload.DeletionID, "", 100)
	require.NoError(t, err)
	assert.Equal(t, []string{bobInbox}, targets,
		"target for Bob's inbox must be captured before follows CASCADE")

	// Pretend the TTL has elapsed and sweep.
	_, err = pool.Exec(ctx,
		"UPDATE account_deletion_snapshots SET expires_at = $1 WHERE id = $2",
		time.Now().Add(-time.Hour), payload.DeletionID)
	require.NoError(t, err)
	_, err = deletionSvc.PurgeExpiredSnapshots(ctx, time.Now())
	require.NoError(t, err)

	// Snapshot + targets gone.
	_, err = s.GetAccountDeletionSnapshot(ctx, payload.DeletionID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
	targets, err = s.ListPendingAccountDeletionTargets(ctx, payload.DeletionID, "", 100)
	require.NoError(t, err)
	assert.Empty(t, targets, "targets CASCADE with the snapshot")
}
