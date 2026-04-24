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

// accountFKTables lists every table whose rows must be purged when an account
// is hard-deleted. Drives the CASCADE assertion so a new table with an FK to
// accounts(id) that forgets ON DELETE CASCADE surfaces as a red test rather
// than a production foot-gun.
var accountFKTables = []struct{ table, col string }{
	{"users", "account_id"},
	{"media_attachments", "account_id"},
	{"statuses", "account_id"},
	{"status_edits", "account_id"},
	{"follows", "account_id"},
	{"follows", "target_id"},
	{"notifications", "account_id"},
	{"notifications", "from_id"},
	{"oauth_access_tokens", "account_id"},
	{"oauth_authorization_codes", "account_id"},
	{"mutes", "account_id"},
	{"mutes", "target_id"},
	{"blocks", "account_id"},
	{"blocks", "target_id"},
	{"favourites", "account_id"},
	{"status_mentions", "account_id"},
	{"bookmarks", "account_id"},
	{"markers", "account_id"},
	{"account_pins", "account_id"},
	{"account_followed_tags", "account_id"},
	{"account_featured_tags", "account_id"},
	{"conversation_mutes", "account_id"},
	{"announcement_reads", "account_id"},
	{"announcement_reactions", "account_id"},
	{"account_conversations", "account_id"},
	{"push_subscriptions", "account_id"},
	{"notification_policies", "account_id"},
	{"notification_requests", "account_id"},
	{"notification_requests", "from_account_id"},
	{"scheduled_statuses", "account_id"},
	{"lists", "account_id"},
	{"list_accounts", "account_id"},
	{"user_filters", "account_id"},
	{"user_domain_blocks", "account_id"},
	{"poll_votes", "account_id"},
}

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
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = s.GetUserByAccountID(ctx, alice.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)

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
	require.ErrorIs(t, err, domain.ErrNotFound)
	targets, err = s.ListPendingAccountDeletionTargets(ctx, payload.DeletionID, "", 100)
	require.NoError(t, err)
	assert.Empty(t, targets, "targets CASCADE with the snapshot")
}

// TestIntegration_AccountDeletion_FullCascade_Self is the broad CASCADE
// regression test. It seeds Alice (local, via Register so we can authenticate
// with a plaintext password) plus one row in every table with an FK to
// accounts(id), then exercises the bcrypt-gated AccountService.DeleteSelf path
// and asserts every CASCADE/SET NULL landed. Unit tests against the fake store
// can't catch a missing ON DELETE CASCADE — this one can.
func TestIntegration_AccountDeletion_FullCascade_Self(t *testing.T) {
	pool, s, accountSvc, _, ctx := setupDeletionTest(t)
	const password = "correct-horse-battery-staple"
	seed := seedAccountWithAllDependents(t, ctx, s, password)

	require.NoError(t, accountSvc.DeleteSelf(ctx, seed.aliceUserID, password))

	// Alice's account row is gone.
	_, err := s.GetAccountByID(ctx, seed.alice.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)

	// Every account-FK table has zero rows referencing Alice.
	assertAccountDataPurged(t, ctx, pool, seed.alice.ID)

	// SET NULL columns fired: rows preserved, FK column nulled.
	assertAuditTrailPreserved(t, ctx, pool, seed)

	// Bob (remote follower) is untouched.
	bob, err := s.GetAccountByID(ctx, seed.bob.ID)
	require.NoError(t, err)
	assert.Equal(t, seed.bob.ID, bob.ID)
	bobStatus, err := s.GetStatusByID(ctx, seed.bobReplyStatusID)
	require.NoError(t, err, "Bob's reply status must survive Alice's delete")
	assert.Equal(t, seed.bob.ID, bobStatus.AccountID)

	// EventAccountDeleted outbox row emitted in the delete tx.
	assert.Equal(t, 1, countOutboxEvents(t, ctx, pool, seed.alice.ID),
		"expected exactly one account.deleted event for Alice")
}

// TestIntegration_AccountDeletion_WrongPassword_NoSideEffects verifies the
// bcrypt-gate: a wrong password returns ErrForbidden, the account survives,
// and no downstream state (dependents, outbox event, deletion snapshot) is
// touched.
func TestIntegration_AccountDeletion_WrongPassword_NoSideEffects(t *testing.T) {
	pool, s, accountSvc, _, ctx := setupDeletionTest(t)
	const password = "correct-horse-battery-staple"
	seed := seedAccountWithAllDependents(t, ctx, s, password)

	before := countAccountRelatedRows(t, ctx, pool, seed.alice.ID)
	require.NotEmpty(t, before, "seed produced no rows — seeding helper is broken")

	err := accountSvc.DeleteSelf(ctx, seed.aliceUserID, "nope-not-the-password")
	require.ErrorIs(t, err, domain.ErrForbidden)

	// Account still there, row counts unchanged.
	_, err = s.GetAccountByID(ctx, seed.alice.ID)
	require.NoError(t, err, "account row must survive a wrong-password attempt")
	assert.Equal(t, before, countAccountRelatedRows(t, ctx, pool, seed.alice.ID),
		"no dependent row count should change on wrong-password delete")

	// No federation event, no deletion snapshot — the bcrypt check fires
	// before we open the delete tx, so neither side effect should appear.
	assert.Zero(t, countOutboxEvents(t, ctx, pool, seed.alice.ID),
		"no account.deleted event should be emitted on auth failure")
	var snapshotCount int
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM account_deletion_snapshots WHERE ap_id = $1",
		seed.alice.APID).Scan(&snapshotCount))
	assert.Zero(t, snapshotCount, "no deletion snapshot should be created")
}

// TestIntegration_ModerationService_DeleteAccount_FullCascade_Admin exercises
// the admin-initiated hard-delete. Same CASCADE sweep as the self-delete path,
// plus an admin_actions audit row that must survive the tx with its
// target_account_id nulled (audit trail preserved even though the target row
// is gone).
func TestIntegration_ModerationService_DeleteAccount_FullCascade_Admin(t *testing.T) {
	pool, s, accountSvc, modSvc, ctx := setupDeletionTest(t)
	seed := seedAccountWithAllDependents(t, ctx, s, "irrelevant-no-password-check")

	// A local moderator to attribute the action to. moderator_id references
	// users(id), not accounts(id), so this account is not touched by Alice's
	// delete.
	modSuffix := uid.New()
	modAcc, err := accountSvc.Register(ctx, RegisterInput{
		Username: "mod" + modSuffix,
		Email:    "mod" + modSuffix + "@test.example.com",
		Password: "modpassword",
		Role:     domain.RoleModerator,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = s.DeleteAccount(ctx, modAcc.ID) })
	modUser, err := s.GetUserByAccountID(ctx, modAcc.ID)
	require.NoError(t, err)

	require.NoError(t, modSvc.DeleteAccount(ctx, modUser.ID, seed.alice.ID))

	_, err = s.GetAccountByID(ctx, seed.alice.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)

	assertAccountDataPurged(t, ctx, pool, seed.alice.ID)
	assertAuditTrailPreserved(t, ctx, pool, seed)

	// Exactly one admin_actions row for this moderator + action, with
	// target_account_id nulled by the CASCADE SET NULL.
	var actionCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM admin_actions WHERE moderator_id = $1 AND action = $2`,
		modUser.ID, AdminActionDeleteAccount).Scan(&actionCount))
	assert.Equal(t, 1, actionCount, "expected one delete_account admin action")

	var nulledCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM admin_actions
		 WHERE moderator_id = $1 AND action = $2 AND target_account_id IS NULL`,
		modUser.ID, AdminActionDeleteAccount).Scan(&nulledCount))
	assert.Equal(t, 1, nulledCount,
		"admin_actions.target_account_id must be NULL after target's CASCADE (audit preserved)")

	assert.Equal(t, 1, countOutboxEvents(t, ctx, pool, seed.alice.ID),
		"expected exactly one account.deleted event for Alice")
}

// TestIntegration_AccountDeletion_MediaPurgeSideTable exercises the post-CASCADE
// media cleanup machinery. It does not run the subscriber (that requires NATS
// and a real blob store); instead it asserts the in-tx materialization is
// correct so that when the subscriber runs in prod it has everything it needs.
//
//  1. media_purge_targets must hold a row for every storage_key Alice owned
//     before the delete.
//  2. An outbox EventMediaPurge row must be emitted with the same purge_id
//     so NATS can route the work to the subscriber.
func TestIntegration_AccountDeletion_MediaPurgeSideTable(t *testing.T) {
	pool, s, accountSvc, _, ctx := setupDeletionTest(t)
	const password = "correct-horse-battery-staple"
	seed := seedAccountWithAllDependents(t, ctx, s, password)

	require.NoError(t, accountSvc.DeleteSelf(ctx, seed.aliceUserID, password))

	// Pull the media.purge event from outbox_events and parse its deletion_id.
	var payloadRaw []byte
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT payload FROM outbox_events WHERE event_type = $1 AND aggregate_id = $2`,
		domain.EventMediaPurge, seed.alice.ID).Scan(&payloadRaw))
	var payload domain.MediaPurgePayload
	require.NoError(t, json.Unmarshal(payloadRaw, &payload))
	assert.Equal(t, seed.alice.ID, payload.AccountID)
	require.NotEmpty(t, payload.PurgeID, "media.purge event must carry a purge_id")

	// media_purge_targets must hold Alice's storage key, unmarked
	// (delivered_at IS NULL). Table name was generalised from
	// account_deletion_media_targets in migration 000085 so domain-block
	// suspend purges can share the same queue.
	rows, err := pool.Query(ctx,
		`SELECT storage_key, delivered_at FROM media_purge_targets WHERE purge_id = $1`,
		payload.PurgeID)
	require.NoError(t, err)
	defer rows.Close()

	targets := map[string]*time.Time{}
	for rows.Next() {
		var key string
		var deliveredAt *time.Time
		require.NoError(t, rows.Scan(&key, &deliveredAt))
		targets[key] = deliveredAt
	}
	require.NoError(t, rows.Err())
	assert.Contains(t, targets, seed.aliceStorageKey,
		"Alice's storage key must be captured before the CASCADE wipes media_attachments")
	assert.Nil(t, targets[seed.aliceStorageKey],
		"target row must be unmarked so the subscriber can pick it up")
}

// setupDeletionTest opens a pool, builds the services the deletion tests
// need, and registers pool cleanup with t.Cleanup.
func setupDeletionTest(t *testing.T) (
	*pgxpool.Pool,
	store.Store,
	AccountService,
	ModerationService,
	context.Context,
) {
	t.Helper()
	cfg, err := config.Load()
	require.NoError(t, err, "failed to load config")
	connString := store.DatabaseConnectionString(cfg, false)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, connString)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	s := postgres.New(pool)
	accountSvc := NewAccountService(s, "https://test.example.com")
	modSvc := NewModerationService(s, noopBlocklist{})
	return pool, s, accountSvc, modSvc, ctx
}

// noopBlocklist satisfies BlocklistRefresher for tests that don't exercise
// the domain-block path.
type noopBlocklist struct{}

func (noopBlocklist) Refresh(_ context.Context) error { return nil }

// deletionSeed carries the handles a test needs for post-delete assertions.
// The IDs are of rows that must survive Alice's delete with their FK column
// set to NULL; rows that cascade-delete are verified by the table-driven
// sweep in assertAccountDataPurged and don't need individual handles.
type deletionSeed struct {
	alice       *domain.Account
	aliceUserID string
	bob         *domain.Account
	// bobReplyStatusID is a status by Bob whose in_reply_to_account_id
	// points at Alice. After Alice's delete, the row must survive but the
	// column must be NULL (SET NULL).
	bobReplyStatusID string
	// reporterReportID has reports.account_id = Alice (reporter).
	// After delete, the row survives with account_id = NULL.
	reporterReportID string
	// targetReportID has reports.target_id = Alice (reported).
	// After delete, the row survives with target_id = NULL.
	targetReportID string
	// aliceStorageKey is the storage_key Alice's seeded media attachment
	// points at. media_attachments.account_id CASCADEs on Alice's delete,
	// but the key must already be captured in account_deletion_media_targets
	// for the media-purge subscriber to clean up the blob later.
	aliceStorageKey string
}

// seedAccountWithAllDependents registers Alice (local) with a confirmed user
// carrying a real bcrypt hash of password, creates Bob (remote follower on
// other.example), and plants at least one row in every table in
// accountFKTables plus the SET NULL audit tables. Tests that call DeleteSelf
// pass the same password back in; the admin test ignores it.
func seedAccountWithAllDependents(t *testing.T, ctx context.Context, s store.Store, password string) *deletionSeed {
	t.Helper()
	suffix := uid.New()
	aliceName := "alice" + suffix
	bobName := "bob" + suffix
	bobDomain := testRemoteDomain

	// --- accounts + users ------------------------------------------------
	accountSvc := NewAccountService(s, "https://test.example.com")
	aliceAcc, err := accountSvc.Register(ctx, RegisterInput{
		Username: aliceName,
		Email:    aliceName + "@test.example.com",
		Password: password,
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = s.DeleteAccount(ctx, aliceAcc.ID) })
	aliceUser, err := s.GetUserByAccountID(ctx, aliceAcc.ID)
	require.NoError(t, err)

	// Bob (remote). Needs a non-empty public_key because the column is NOT
	// NULL; the value is inert for this test.
	bobAPID := "https://" + testRemoteDomain + "/users/" + bobName
	bobStatusBase := "https://" + testRemoteDomain + "/statuses/"
	bobAcc, err := s.CreateAccount(ctx, store.CreateAccountInput{
		ID:           "01bob" + suffix,
		Username:     bobName,
		Domain:       &bobDomain,
		PublicKey:    "stub",
		APID:         bobAPID,
		InboxURL:     bobAPID + "/inbox",
		OutboxURL:    bobAPID + "/outbox",
		FollowersURL: bobAPID + "/followers",
		FollowingURL: bobAPID + "/following",
	})
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = s.DeleteAccount(ctx, bobAcc.ID) })

	// --- statuses (Alice + Bob reply) ------------------------------------
	aliceStatusID := uid.New()
	_, err = s.CreateStatus(ctx, store.CreateStatusInput{
		ID:                  aliceStatusID,
		URI:                 "https://test.example.com/statuses/" + aliceStatusID,
		AccountID:           aliceAcc.ID,
		Text:                strPtr("hello from alice"),
		Content:             strPtr("<p>hello from alice</p>"),
		Visibility:          domain.VisibilityPublic,
		APID:                "https://test.example.com/statuses/" + aliceStatusID,
		Local:               true,
		QuoteApprovalPolicy: domain.QuotePolicyPublic,
	})
	require.NoError(t, err)

	// Bob replies to Alice — this row must survive Alice's delete with
	// in_reply_to_account_id = NULL (SET NULL).
	bobReplyStatusID := "01bobstat" + suffix
	aliceAccID := aliceAcc.ID
	_, err = s.CreateStatus(ctx, store.CreateStatusInput{
		ID:                  bobReplyStatusID,
		URI:                 bobStatusBase + bobReplyStatusID,
		AccountID:           bobAcc.ID,
		Text:                strPtr("hi alice"),
		Content:             strPtr("<p>hi alice</p>"),
		Visibility:          domain.VisibilityPublic,
		InReplyToID:         &aliceStatusID,
		InReplyToAccountID:  &aliceAccID,
		APID:                bobStatusBase + bobReplyStatusID,
		Local:               false,
		QuoteApprovalPolicy: domain.QuotePolicyPublic,
	})
	require.NoError(t, err)

	// --- status_edits ----------------------------------------------------
	require.NoError(t, s.CreateStatusEdit(ctx, store.CreateStatusEditInput{
		ID:        uid.New(),
		StatusID:  aliceStatusID,
		AccountID: aliceAcc.ID,
		Text:      strPtr("hello from alice (edit)"),
		Content:   strPtr("<p>hello from alice (edit)</p>"),
	}))

	// --- follows (both directions) ---------------------------------------
	_, err = s.CreateFollow(ctx, store.CreateFollowInput{
		ID: "01fol1" + suffix, AccountID: bobAcc.ID, TargetID: aliceAcc.ID,
		State: domain.FollowStateAccepted,
	})
	require.NoError(t, err)
	_, err = s.CreateFollow(ctx, store.CreateFollowInput{
		ID: "01fol2" + suffix, AccountID: aliceAcc.ID, TargetID: bobAcc.ID,
		State: domain.FollowStateAccepted,
	})
	require.NoError(t, err)

	// --- favourites (both sides) -----------------------------------------
	_, err = s.CreateFavourite(ctx, store.CreateFavouriteInput{
		ID: uid.New(), AccountID: bobAcc.ID, StatusID: aliceStatusID,
	})
	require.NoError(t, err)
	_, err = s.CreateFavourite(ctx, store.CreateFavouriteInput{
		ID: uid.New(), AccountID: aliceAcc.ID, StatusID: bobReplyStatusID,
	})
	require.NoError(t, err)

	// --- bookmarks -------------------------------------------------------
	require.NoError(t, s.CreateBookmark(ctx, store.CreateBookmarkInput{
		ID: uid.New(), AccountID: aliceAcc.ID, StatusID: bobReplyStatusID,
	}))

	// --- blocks + mutes (both directions to hit account_id + target_id) --
	require.NoError(t, s.CreateBlock(ctx, store.CreateBlockInput{
		ID: uid.New(), AccountID: aliceAcc.ID, TargetID: bobAcc.ID,
	}))
	require.NoError(t, s.CreateBlock(ctx, store.CreateBlockInput{
		ID: uid.New(), AccountID: bobAcc.ID, TargetID: aliceAcc.ID,
	}))
	require.NoError(t, s.CreateMute(ctx, store.CreateMuteInput{
		ID: uid.New(), AccountID: aliceAcc.ID, TargetID: bobAcc.ID,
	}))
	require.NoError(t, s.CreateMute(ctx, store.CreateMuteInput{
		ID: uid.New(), AccountID: bobAcc.ID, TargetID: aliceAcc.ID,
	}))

	// --- notifications (recipient + sender directions) -------------------
	_, err = s.CreateNotification(ctx, store.CreateNotificationInput{
		ID: uid.New(), AccountID: aliceAcc.ID, FromID: bobAcc.ID,
		Type: domain.NotificationTypeFavourite, StatusID: &aliceStatusID,
		GroupKey: "g1" + suffix,
	})
	require.NoError(t, err)
	_, err = s.CreateNotification(ctx, store.CreateNotificationInput{
		ID: uid.New(), AccountID: bobAcc.ID, FromID: aliceAcc.ID,
		Type: domain.NotificationTypeFavourite, StatusID: &bobReplyStatusID,
		GroupKey: "g2" + suffix,
	})
	require.NoError(t, err)

	// --- status_mentions -------------------------------------------------
	require.NoError(t, s.CreateStatusMention(ctx, bobReplyStatusID, aliceAcc.ID))

	// --- media_attachments -----------------------------------------------
	aliceStorageKey := "test/" + suffix
	_, err = s.CreateMediaAttachment(ctx, store.CreateMediaAttachmentInput{
		ID:         uid.New(),
		AccountID:  aliceAcc.ID,
		Type:       "image",
		StorageKey: aliceStorageKey,
		URL:        "https://test.example.com/media/" + suffix,
	})
	require.NoError(t, err)

	// --- oauth application + access token + authorization code ----------
	app, err := s.CreateApplication(ctx, store.CreateApplicationInput{
		ID:           uid.New(),
		Name:         "testapp-" + suffix,
		ClientID:     "cid-" + suffix,
		ClientSecret: "secret-" + suffix,
		RedirectURIs: "urn:ietf:wg:oauth:2.0:oob",
		Scopes:       "read write",
	})
	require.NoError(t, err)
	accountIDPtr := aliceAcc.ID
	accessToken, err := s.CreateAccessToken(ctx, store.CreateAccessTokenInput{
		ID:            uid.New(),
		ApplicationID: app.ID,
		AccountID:     &accountIDPtr,
		Token:         "tok-" + suffix,
		Scopes:        "read write",
	})
	require.NoError(t, err)
	_, err = s.CreateAuthorizationCode(ctx, store.CreateAuthorizationCodeInput{
		ID:            uid.New(),
		Code:          "authcode-" + suffix,
		ApplicationID: app.ID,
		AccountID:     aliceAcc.ID,
		RedirectURI:   "urn:ietf:wg:oauth:2.0:oob",
		Scopes:        "read",
		ExpiresAt:     time.Now().Add(time.Hour),
	})
	require.NoError(t, err)

	// --- push_subscriptions (tied to the access token) -------------------
	_, err = s.CreatePushSubscription(ctx, store.CreatePushSubscriptionInput{
		ID:            uid.New(),
		AccessTokenID: accessToken.ID,
		AccountID:     aliceAcc.ID,
		Endpoint:      "https://push.example.com/" + suffix,
		KeyP256DH:     "p256-" + suffix,
		KeyAuth:       "auth-" + suffix,
		Alerts:        domain.PushAlerts{},
		Policy:        "all",
	})
	require.NoError(t, err)

	// --- account_pins ----------------------------------------------------
	require.NoError(t, s.CreateAccountPin(ctx, aliceAcc.ID, aliceStatusID))

	// --- hashtags + account_followed_tags + account_featured_tags -------
	tag, err := s.GetOrCreateHashtag(ctx, "testtag"+suffix)
	require.NoError(t, err)
	require.NoError(t, s.FollowTag(ctx, uid.New(), aliceAcc.ID, tag.ID))
	require.NoError(t, s.CreateFeaturedTag(ctx, uid.New(), aliceAcc.ID, tag.ID))

	// --- conversation + account_conversations + conversation_mutes ------
	conversationID := uid.New()
	require.NoError(t, s.CreateConversation(ctx, conversationID))
	require.NoError(t, s.SetStatusConversationID(ctx, aliceStatusID, conversationID))
	require.NoError(t, s.UpsertAccountConversation(ctx, store.UpsertAccountConversationInput{
		ID:             uid.New(),
		AccountID:      aliceAcc.ID,
		ConversationID: conversationID,
		LastStatusID:   aliceStatusID,
		Unread:         true,
	}))
	require.NoError(t, s.CreateConversationMute(ctx, aliceAcc.ID, conversationID))

	// --- markers ---------------------------------------------------------
	require.NoError(t, s.SetMarker(ctx, aliceAcc.ID, "home", aliceStatusID))

	// --- announcements + reads + reactions ------------------------------
	ann, err := s.CreateAnnouncement(ctx, store.CreateAnnouncementInput{
		ID:          uid.New(),
		Content:     "maintenance " + suffix,
		PublishedAt: time.Now(),
	})
	require.NoError(t, err)
	require.NoError(t, s.DismissAnnouncement(ctx, aliceAcc.ID, ann.ID))
	require.NoError(t, s.AddAnnouncementReaction(ctx, ann.ID, aliceAcc.ID, "+1"))

	// --- reports (reporter + reported directions, both SET NULL) --------
	reporterReport, err := s.CreateReport(ctx, store.CreateReportInput{
		ID:        uid.New(),
		AccountID: aliceAcc.ID, // reporter
		TargetID:  bobAcc.ID,
		Category:  domain.ReportCategoryOther,
	})
	require.NoError(t, err)
	targetReport, err := s.CreateReport(ctx, store.CreateReportInput{
		ID:        uid.New(),
		AccountID: bobAcc.ID,
		TargetID:  aliceAcc.ID, // reported
		Category:  domain.ReportCategoryOther,
	})
	require.NoError(t, err)

	// --- notification_policies + notification_requests -------------------
	_, err = s.UpsertNotificationPolicy(ctx, aliceAcc.ID)
	require.NoError(t, err)
	_, err = s.UpsertNotificationRequest(ctx, store.UpsertNotificationRequestInput{
		ID:            uid.New(),
		AccountID:     aliceAcc.ID,
		FromAccountID: bobAcc.ID,
	})
	require.NoError(t, err)
	// A second request where Alice is the from_account_id — covers that FK column.
	_, err = s.UpsertNotificationRequest(ctx, store.UpsertNotificationRequestInput{
		ID:            uid.New(),
		AccountID:     bobAcc.ID,
		FromAccountID: aliceAcc.ID,
	})
	require.NoError(t, err)

	// --- scheduled_statuses ----------------------------------------------
	_, err = s.CreateScheduledStatus(ctx, store.CreateScheduledStatusInput{
		ID:          uid.New(),
		AccountID:   aliceAcc.ID,
		Params:      []byte(`{"text":"queued"}`),
		ScheduledAt: time.Now().Add(24 * time.Hour),
	})
	require.NoError(t, err)

	// --- lists + list_accounts ------------------------------------------
	list, err := s.CreateList(ctx, store.CreateListInput{
		ID:            uid.New(),
		AccountID:     aliceAcc.ID,
		Title:         "friends-" + suffix,
		RepliesPolicy: domain.ListRepliesPolicyList,
	})
	require.NoError(t, err)
	require.NoError(t, s.AddAccountToList(ctx, list.ID, bobAcc.ID))

	// --- user_filters ----------------------------------------------------
	_, err = s.CreateFilter(ctx, store.CreateFilterInput{
		ID:           uid.New(),
		AccountID:    aliceAcc.ID,
		Title:        "filter-" + suffix,
		Context:      []domain.FilterContext{domain.FilterContextHome},
		FilterAction: domain.FilterActionHide,
	})
	require.NoError(t, err)

	// --- user_domain_blocks ----------------------------------------------
	require.NoError(t, s.CreateUserDomainBlock(ctx, store.CreateUserDomainBlockInput{
		ID:        uid.New(),
		AccountID: aliceAcc.ID,
		Domain:    "blocked-" + suffix + ".example",
	}))

	// --- poll (on Bob's reply) + poll_votes (by Alice) ------------------
	poll, err := s.CreatePoll(ctx, store.CreatePollInput{
		ID:       uid.New(),
		StatusID: bobReplyStatusID,
	})
	require.NoError(t, err)
	pollOption, err := s.CreatePollOption(ctx, store.CreatePollOptionInput{
		ID:       uid.New(),
		PollID:   poll.ID,
		Title:    "yes",
		Position: 0,
	})
	require.NoError(t, err)
	require.NoError(t, s.CreatePollVote(ctx, uid.New(), poll.ID, aliceAcc.ID, pollOption.ID))

	return &deletionSeed{
		alice:            aliceAcc,
		aliceUserID:      aliceUser.ID,
		bob:              bobAcc,
		bobReplyStatusID: bobReplyStatusID,
		reporterReportID: reporterReport.ID,
		targetReportID:   targetReport.ID,
		aliceStorageKey:  aliceStorageKey,
	}
}

// assertAccountDataPurged asserts zero rows in every table/column pair in
// accountFKTables that references the account. A non-zero count means either
// the migration is missing a CASCADE or the service layer forgot to drive
// CASCADE through a DELETE on accounts.
func assertAccountDataPurged(t *testing.T, ctx context.Context, pool *pgxpool.Pool, accountID string) {
	t.Helper()
	for _, tc := range accountFKTables {
		//nolint:gosec // table/col come from a static whitelist, not user input.
		q := "SELECT COUNT(*) FROM " + tc.table + " WHERE " + tc.col + " = $1"
		var n int
		require.NoError(t, pool.QueryRow(ctx, q, accountID).Scan(&n),
			"query failed for %s.%s", tc.table, tc.col)
		assert.Zerof(t, n, "%s.%s should have no rows referencing account %s after delete",
			tc.table, tc.col, accountID)
	}
}

// assertAuditTrailPreserved verifies SET NULL columns: the referencing rows
// must survive, but the FK column must now be NULL. Covers
// statuses.in_reply_to_account_id and reports.account_id / reports.target_id.
// admin_actions.target_account_id is checked directly by the admin test.
func assertAuditTrailPreserved(t *testing.T, ctx context.Context, pool *pgxpool.Pool, seed *deletionSeed) {
	t.Helper()

	// Bob's reply row survives with in_reply_to_account_id = NULL.
	var replyExists bool
	var replyFK *string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT TRUE, in_reply_to_account_id FROM statuses WHERE id = $1`,
		seed.bobReplyStatusID).Scan(&replyExists, &replyFK))
	assert.True(t, replyExists, "Bob's reply row must survive (SET NULL)")
	assert.Nil(t, replyFK, "statuses.in_reply_to_account_id must be NULL after Alice's delete")

	// reports row with account_id = Alice survives with account_id NULL.
	var reporterExists bool
	var reporterAccountFK *string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT TRUE, account_id FROM reports WHERE id = $1`,
		seed.reporterReportID).Scan(&reporterExists, &reporterAccountFK))
	assert.True(t, reporterExists, "reports row where Alice was reporter must survive (SET NULL)")
	assert.Nil(t, reporterAccountFK, "reports.account_id must be NULL after deleter's account CASCADE")

	// reports row with target_id = Alice survives with target_id NULL.
	var targetExists bool
	var targetFK *string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT TRUE, target_id FROM reports WHERE id = $1`,
		seed.targetReportID).Scan(&targetExists, &targetFK))
	assert.True(t, targetExists, "reports row where Alice was reported must survive (SET NULL)")
	assert.Nil(t, targetFK, "reports.target_id must be NULL after target's CASCADE")
}

// countAccountRelatedRows snapshots the row count per account-FK table for a
// given account. Used by the wrong-password test to compare before/after and
// prove nothing changed.
func countAccountRelatedRows(t *testing.T, ctx context.Context, pool *pgxpool.Pool, accountID string) map[string]int {
	t.Helper()
	out := make(map[string]int, len(accountFKTables))
	for _, tc := range accountFKTables {
		//nolint:gosec // table/col come from a static whitelist, not user input.
		q := "SELECT COUNT(*) FROM " + tc.table + " WHERE " + tc.col + " = $1"
		var n int
		require.NoError(t, pool.QueryRow(ctx, q, accountID).Scan(&n),
			"count query failed for %s.%s", tc.table, tc.col)
		out[tc.table+"."+tc.col] = n
	}
	return out
}

// countOutboxEvents counts outbox_events rows for EventAccountDeleted
// targeting aggregateID. Reads raw so it doesn't contend with
// GetAndLockUnpublishedOutboxEvents (which takes row locks).
func countOutboxEvents(t *testing.T, ctx context.Context, pool *pgxpool.Pool, aggregateID string) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM outbox_events WHERE event_type = $1 AND aggregate_id = $2`,
		domain.EventAccountDeleted, aggregateID).Scan(&n))
	return n
}

func strPtr(s string) *string { return &s }
