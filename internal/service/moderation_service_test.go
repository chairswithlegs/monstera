package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type noopBlocklistRefresher struct{}

func (noopBlocklistRefresher) Refresh(_ context.Context) error { return nil }

func seedLocalAccount(t *testing.T, fake *testutil.FakeStore, id, username string) *domain.Account {
	t.Helper()
	// deleteLocalAccount requires a private key so the Delete{Actor} signer
	// has something to snapshot. Use a stub PEM — the fake store doesn't
	// parse it. The APID mirrors the convention used elsewhere in tests so
	// federation event payloads (suspend, delete) carry a realistic actor IRI.
	pk := "-----BEGIN RSA PRIVATE KEY-----\nstub\n-----END RSA PRIVATE KEY-----"
	acc, err := fake.CreateAccount(context.Background(), store.CreateAccountInput{
		ID:         id,
		Username:   username,
		APID:       "https://example.com/users/" + username,
		PrivateKey: &pk,
	})
	require.NoError(t, err)
	return acc
}

func seedRemoteAccount(t *testing.T, fake *testutil.FakeStore, id, username, remoteDomain string) *domain.Account {
	t.Helper()
	d := remoteDomain
	acc, err := fake.CreateAccount(context.Background(), store.CreateAccountInput{
		ID:       id,
		Username: username,
		Domain:   &d,
	})
	require.NoError(t, err)
	return acc
}

func TestModerationService_SuspendAccount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T, fake *testutil.FakeStore) (moderatorID, targetID string)
		wantErr error
	}{
		{
			name: "local account",
			setup: func(t *testing.T, fake *testutil.FakeStore) (string, string) {
				t.Helper()
				mod := seedLocalAccount(t, fake, "mod-1", "moderator")
				target := seedLocalAccount(t, fake, "target-1", "target")
				return mod.ID, target.ID
			},
		},
		{
			name: "remote account",
			setup: func(t *testing.T, fake *testutil.FakeStore) (string, string) {
				t.Helper()
				mod := seedLocalAccount(t, fake, "mod-2", "moderator2")
				target := seedRemoteAccount(t, fake, "remote-1", "remoteuser", "remote.example.com")
				return mod.ID, target.ID
			},
			wantErr: domain.ErrForbidden,
		},
		{
			name: "not found",
			setup: func(t *testing.T, fake *testutil.FakeStore) (string, string) {
				t.Helper()
				mod := seedLocalAccount(t, fake, "mod-3", "moderator3")
				return mod.ID, "nonexistent"
			},
			wantErr: domain.ErrNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			fake := testutil.NewFakeStore()
			svc := NewModerationService(fake, noopBlocklistRefresher{})

			modID, targetID := tc.setup(t, fake)
			err := svc.SuspendAccount(ctx, modID, targetID)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				// Failed suspends must not record an audit row or emit an event.
				for _, a := range fake.AdminActions {
					assert.NotEqual(t, AdminActionSuspend, a.Action)
				}
				for _, e := range fake.OutboxEvents {
					assert.NotEqual(t, domain.EventAccountSuspended, e.EventType)
				}
			} else {
				require.NoError(t, err)
				acc, err := fake.GetAccountByID(ctx, targetID)
				require.NoError(t, err)
				assert.True(t, acc.Suspended)

				// Audit row recorded in the same tx as the flag flip.
				assertAdminActionRecorded(t, fake, AdminActionSuspend, targetID)

				// Federation event emitted so remote followers receive Delete{Actor}.
				assertOutboxContainsEvent(t, fake, domain.EventAccountSuspended, targetID)

				// Verify payload carries APID + Local=true so the federation
				// subscriber can build the Delete activity without re-querying.
				var found bool
				for _, ev := range fake.OutboxEvents {
					if ev.EventType != domain.EventAccountSuspended {
						continue
					}
					var p domain.AccountSuspendedPayload
					require.NoError(t, json.Unmarshal(ev.Payload, &p))
					assert.Equal(t, targetID, p.AccountID)
					assert.NotEmpty(t, p.APID)
					assert.True(t, p.Local)
					found = true
				}
				assert.True(t, found, "EventAccountSuspended payload should be present")
			}
		})
	}
}

func TestModerationService_SilenceAccount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T, fake *testutil.FakeStore) (moderatorID, targetID string)
		wantErr error
	}{
		{
			name: "local account",
			setup: func(t *testing.T, fake *testutil.FakeStore) (string, string) {
				t.Helper()
				mod := seedLocalAccount(t, fake, "mod-1", "moderator")
				target := seedLocalAccount(t, fake, "target-1", "target")
				return mod.ID, target.ID
			},
		},
		{
			name: "remote account",
			setup: func(t *testing.T, fake *testutil.FakeStore) (string, string) {
				t.Helper()
				mod := seedLocalAccount(t, fake, "mod-2", "moderator2")
				target := seedRemoteAccount(t, fake, "remote-1", "remoteuser", "remote.example.com")
				return mod.ID, target.ID
			},
			wantErr: domain.ErrForbidden,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			fake := testutil.NewFakeStore()
			svc := NewModerationService(fake, noopBlocklistRefresher{})

			modID, targetID := tc.setup(t, fake)
			err := svc.SilenceAccount(ctx, modID, targetID)

			if tc.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
				acc, err := fake.GetAccountByID(ctx, targetID)
				require.NoError(t, err)
				assert.True(t, acc.Silenced)
			}
		})
	}
}

func TestModerationService_UnsuspendAccount(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewModerationService(fake, noopBlocklistRefresher{})

		mod := seedLocalAccount(t, fake, "mod-1", "moderator")
		target := seedLocalAccount(t, fake, "target-1", "target")
		require.NoError(t, fake.SuspendAccount(ctx, target.ID))

		err := svc.UnsuspendAccount(ctx, mod.ID, target.ID)
		require.NoError(t, err)

		acc, err := fake.GetAccountByID(ctx, target.ID)
		require.NoError(t, err)
		assert.False(t, acc.Suspended)
	})
}

func TestModerationService_UnsilenceAccount(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewModerationService(fake, noopBlocklistRefresher{})

		mod := seedLocalAccount(t, fake, "mod-1", "moderator")
		target := seedLocalAccount(t, fake, "target-1", "target")
		require.NoError(t, fake.SilenceAccount(ctx, target.ID))

		err := svc.UnsilenceAccount(ctx, mod.ID, target.ID)
		require.NoError(t, err)

		acc, err := fake.GetAccountByID(ctx, target.ID)
		require.NoError(t, err)
		assert.False(t, acc.Silenced)
	})
}

func TestModerationService_CreateDomainBlock(t *testing.T) {
	t.Parallel()

	t.Run("suspend severity creates purge row, emits event, and flips domain_suspended", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewModerationService(fake, noopBlocklistRefresher{})

		mod := seedLocalAccount(t, fake, "mod-1", "moderator")
		// Two pre-existing remote accounts on the target domain.
		remoteDomain := "bad.example.com"
		for _, id := range []string{"remote-a", "remote-b"} {
			fake.SeedAccount(&domain.Account{ID: id, Username: id, Domain: &remoteDomain, APID: "https://" + remoteDomain + "/users/" + id})
		}

		block, err := svc.CreateDomainBlock(ctx, mod.ID, CreateDomainBlockInput{
			Domain:   remoteDomain,
			Severity: domain.DomainBlockSeveritySuspend,
		})
		require.NoError(t, err)
		require.NotNil(t, block)
		assert.Equal(t, remoteDomain, block.Domain)
		assert.Equal(t, domain.DomainBlockSeveritySuspend, block.Severity)

		// Purge tracker row created so the subscriber can pick it up.
		purge, err := fake.GetDomainBlockPurge(ctx, block.ID)
		require.NoError(t, err)
		assert.Equal(t, remoteDomain, purge.Domain)
		assert.Nil(t, purge.CompletedAt)

		// EventDomainBlockSuspended emitted through the outbox.
		var found bool
		for _, e := range fake.OutboxEvents {
			if e.EventType == domain.EventDomainBlockSuspended {
				found = true
				break
			}
		}
		assert.True(t, found, "EventDomainBlockSuspended should have been emitted")

		// Pre-existing remote accounts had domain_suspended flipped on in
		// the same tx — lookups 404 immediately, no race with the subscriber.
		for _, id := range []string{"remote-a", "remote-b"} {
			a, err := fake.GetAccountByID(ctx, id)
			require.NoError(t, err)
			assert.True(t, a.DomainSuspended, "account %s domain_suspended should be true", id)
			assert.False(t, a.Suspended, "account %s.suspended should be untouched", id)
			assert.True(t, a.IsHidden())
		}
	})

	t.Run("silence severity does not create purge row or emit event", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewModerationService(fake, noopBlocklistRefresher{})

		mod := seedLocalAccount(t, fake, "mod-1", "moderator")
		// Seed a remote account on the target domain to verify that
		// domain_suspended is NOT flipped for silence severity.
		remoteDomain := "noisy.example.com"
		fake.SeedAccount(&domain.Account{ID: "remote-silence", Username: "remote-silence", Domain: &remoteDomain, APID: "https://noisy.example.com/users/x"})

		block, err := svc.CreateDomainBlock(ctx, mod.ID, CreateDomainBlockInput{
			Domain:   remoteDomain,
			Severity: domain.DomainBlockSeveritySilence,
		})
		require.NoError(t, err)

		_, err = fake.GetDomainBlockPurge(ctx, block.ID)
		require.ErrorIs(t, err, domain.ErrNotFound, "silence blocks do not create a purge tracker")

		for _, e := range fake.OutboxEvents {
			assert.NotEqual(t, domain.EventDomainBlockSuspended, e.EventType,
				"silence blocks must not emit domain_block.suspended")
		}

		a, err := fake.GetAccountByID(ctx, "remote-silence")
		require.NoError(t, err)
		assert.False(t, a.DomainSuspended, "silence severity must not flip domain_suspended")
	})
}

// TestModerationService_DeleteDomainBlock_ReversesDomainSuspension is the
// regression test for the bug where a remote account stayed 404 after its
// domain block was removed. Covers two cases: (a) an account that was only
// suspended because of the domain block becomes visible again on unblock;
// (b) an account that was also individually suspended (e.g. via federation
// Delete{Person}) stays hidden — the unblock only reverses the domain-level
// cause, not the individual one.
func TestModerationService_DeleteDomainBlock_ReversesDomainSuspension(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewModerationService(fake, noopBlocklistRefresher{})

	mod := seedLocalAccount(t, fake, "mod-1", "moderator")
	remoteDomain := "example.test"

	// Case (a): untouched remote account.
	fake.SeedAccount(&domain.Account{ID: "alice", Username: "alice", Domain: &remoteDomain, APID: "https://example.test/users/alice"})
	// Case (b): already individually suspended.
	fake.SeedAccount(&domain.Account{ID: "bob", Username: "bob", Domain: &remoteDomain, APID: "https://example.test/users/bob", Suspended: true})

	// Suspend the domain.
	_, err := svc.CreateDomainBlock(ctx, mod.ID, CreateDomainBlockInput{
		Domain: remoteDomain, Severity: domain.DomainBlockSeveritySuspend,
	})
	require.NoError(t, err)

	alice, err := fake.GetAccountByID(ctx, "alice")
	require.NoError(t, err)
	assert.True(t, alice.IsHidden(), "alice should be hidden while blocked")
	assert.True(t, alice.DomainSuspended)
	assert.False(t, alice.Suspended)

	bob, err := fake.GetAccountByID(ctx, "bob")
	require.NoError(t, err)
	assert.True(t, bob.IsHidden())
	assert.True(t, bob.DomainSuspended)
	assert.True(t, bob.Suspended, "bob's individual suspension must survive the domain block")

	// Unblock.
	require.NoError(t, svc.DeleteDomainBlock(ctx, mod.ID, remoteDomain))

	alice, err = fake.GetAccountByID(ctx, "alice")
	require.NoError(t, err)
	assert.False(t, alice.IsHidden(), "alice should be visible after unblock (was the bug)")
	assert.False(t, alice.DomainSuspended)
	assert.False(t, alice.Suspended)

	bob, err = fake.GetAccountByID(ctx, "bob")
	require.NoError(t, err)
	assert.True(t, bob.IsHidden(), "bob must stay hidden (individually suspended)")
	assert.False(t, bob.DomainSuspended, "domain-level cause cleared")
	assert.True(t, bob.Suspended, "individual-level cause preserved")
}

func TestModerationService_ListDomainBlocksWithPurge(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewModerationService(fake, noopBlocklistRefresher{})

	// Silence block (no purge row).
	mod := seedLocalAccount(t, fake, "mod-1", "moderator")
	silence, err := svc.CreateDomainBlock(ctx, mod.ID, CreateDomainBlockInput{
		Domain:   "silence.example",
		Severity: domain.DomainBlockSeveritySilence,
	})
	require.NoError(t, err)

	// Suspend block (in-progress purge, cursor not yet set; 2 remote accounts remaining).
	suspend, err := svc.CreateDomainBlock(ctx, mod.ID, CreateDomainBlockInput{
		Domain:   "suspend.example",
		Severity: domain.DomainBlockSeveritySuspend,
	})
	require.NoError(t, err)
	remoteDomain := "suspend.example"
	for _, id := range []string{"remote-a", "remote-b"} {
		fake.SeedAccount(&domain.Account{ID: id, Username: id, Domain: &remoteDomain, APID: "https://suspend.example/users/" + id})
	}

	// Suspend block with completed purge.
	done, err := svc.CreateDomainBlock(ctx, mod.ID, CreateDomainBlockInput{
		Domain:   "done.example",
		Severity: domain.DomainBlockSeveritySuspend,
	})
	require.NoError(t, err)
	require.NoError(t, fake.MarkDomainBlockPurgeComplete(ctx, done.ID))

	rows, err := svc.ListDomainBlocksWithPurge(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 3)

	byID := map[string]DomainBlockWithPurgeResult{}
	for _, r := range rows {
		byID[r.Block.ID] = r
	}

	assert.Nil(t, byID[silence.ID].Purge, "silence block has no purge row")
	assert.Nil(t, byID[silence.ID].AccountsRemaining)

	require.NotNil(t, byID[suspend.ID].Purge)
	assert.Nil(t, byID[suspend.ID].Purge.CompletedAt)
	require.NotNil(t, byID[suspend.ID].AccountsRemaining)
	assert.EqualValues(t, 2, *byID[suspend.ID].AccountsRemaining)

	require.NotNil(t, byID[done.ID].Purge)
	require.NotNil(t, byID[done.ID].Purge.CompletedAt)
	assert.Nil(t, byID[done.ID].AccountsRemaining, "completed purge does not report accounts_remaining")
}

func TestModerationService_DeleteDomainBlock(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewModerationService(fake, noopBlocklistRefresher{})

		mod := seedLocalAccount(t, fake, "mod-1", "moderator")
		_, err := fake.CreateDomainBlock(ctx, store.CreateDomainBlockInput{
			ID:       "db-1",
			Domain:   "bad.example.com",
			Severity: domain.DomainBlockSeveritySuspend,
		})
		require.NoError(t, err)

		err = svc.DeleteDomainBlock(ctx, mod.ID, "bad.example.com")
		require.NoError(t, err)

		blocks, err := fake.ListDomainBlocks(ctx)
		require.NoError(t, err)
		assert.Empty(t, blocks)
	})
}

func TestModerationService_CreateReport(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewModerationService(fake, noopBlocklistRefresher{})

		reporter := seedLocalAccount(t, fake, "reporter-1", "reporter")
		target := seedLocalAccount(t, fake, "target-1", "target")

		comment := "spam content"
		report, err := svc.CreateReport(ctx, CreateReportInput{
			AccountID: reporter.ID,
			TargetID:  target.ID,
			Comment:   &comment,
			Category:  domain.ReportCategorySpam,
		})
		require.NoError(t, err)
		require.NotNil(t, report)
		require.NotNil(t, report.AccountID)
		require.NotNil(t, report.TargetID)
		assert.Equal(t, reporter.ID, *report.AccountID)
		assert.Equal(t, target.ID, *report.TargetID)
		assert.Equal(t, domain.ReportCategorySpam, report.Category)
	})
}

func TestModerationService_ListReports(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewModerationService(fake, noopBlocklistRefresher{})

		reports, err := svc.ListReports(ctx, domain.ReportStateOpen, 10, 0)
		require.NoError(t, err)
		assert.Empty(t, reports)
	})
}

func TestModerationService_GetReport(t *testing.T) {
	t.Parallel()

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewModerationService(fake, noopBlocklistRefresher{})

		_, err := svc.GetReport(ctx, "nonexistent")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestModerationService_AssignReport(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewModerationService(fake, noopBlocklistRefresher{})

		mod := seedLocalAccount(t, fake, "mod-1", "moderator")
		assignee := "assignee-1"

		err := svc.AssignReport(ctx, mod.ID, "report-1", &assignee)
		require.NoError(t, err)
	})
}

func TestModerationService_ResolveReport(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewModerationService(fake, noopBlocklistRefresher{})

		mod := seedLocalAccount(t, fake, "mod-1", "moderator")

		err := svc.ResolveReport(ctx, mod.ID, "report-1", "warned")
		require.NoError(t, err)
	})
}

func TestModerationService_SetUserRole(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewModerationService(fake, noopBlocklistRefresher{})

		mod := seedLocalAccount(t, fake, "mod-1", "moderator")
		target := seedLocalAccount(t, fake, "target-1", "target")
		_, err := fake.CreateUser(ctx, store.CreateUserInput{
			ID:           "user-target",
			AccountID:    target.ID,
			Email:        "target@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
		})
		require.NoError(t, err)

		err = svc.SetUserRole(ctx, mod.ID, "user-target", domain.RoleAdmin)
		require.NoError(t, err)

		u, err := fake.GetUserByID(ctx, "user-target")
		require.NoError(t, err)
		assert.Equal(t, domain.RoleAdmin, u.Role)
	})
}

type trackingBlocklistRefresher struct {
	refreshed bool
}

func (t *trackingBlocklistRefresher) Refresh(_ context.Context) error {
	t.refreshed = true
	return nil
}

func TestModerationService_CreateDomainBlock_RefreshesBlocklist(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	tracker := &trackingBlocklistRefresher{}
	svc := NewModerationService(fake, tracker)

	mod := seedLocalAccount(t, fake, "mod-1", "moderator")

	_, err := svc.CreateDomainBlock(ctx, mod.ID, CreateDomainBlockInput{
		Domain:   "evil.example",
		Severity: domain.DomainBlockSeveritySuspend,
	})
	require.NoError(t, err)
	assert.True(t, tracker.refreshed, "blocklist should have been refreshed after CreateDomainBlock")
}

func TestModerationService_DeleteDomainBlock_RefreshesBlocklist(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	tracker := &trackingBlocklistRefresher{}
	svc := NewModerationService(fake, tracker)

	mod := seedLocalAccount(t, fake, "mod-1", "moderator")

	// Seed a domain block to delete.
	_, err := fake.CreateDomainBlock(ctx, store.CreateDomainBlockInput{
		ID:       "db-1",
		Domain:   "evil.example",
		Severity: domain.DomainBlockSeveritySuspend,
	})
	require.NoError(t, err)

	err = svc.DeleteDomainBlock(ctx, mod.ID, "evil.example")
	require.NoError(t, err)
	assert.True(t, tracker.refreshed, "blocklist should have been refreshed after DeleteDomainBlock")
}

func TestModerationService_DeleteAccount(t *testing.T) {
	t.Parallel()

	seed := func(t *testing.T, fake *testutil.FakeStore) (mod, target *domain.Account) {
		t.Helper()
		ctx := context.Background()
		mod = seedLocalAccount(t, fake, "mod-1", "moderator")
		target = seedLocalAccount(t, fake, "target-1", "target")
		_, err := fake.CreateUser(ctx, store.CreateUserInput{
			ID:           "user-target",
			AccountID:    target.ID,
			Email:        "target@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
		})
		require.NoError(t, err)
		return mod, target
	}

	t.Run("hard_delete", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewModerationService(fake, noopBlocklistRefresher{})
		mod, target := seed(t, fake)

		err := svc.DeleteAccount(ctx, mod.ID, target.ID)
		require.NoError(t, err)

		_, err = fake.GetAccountByID(ctx, target.ID)
		require.ErrorIs(t, err, domain.ErrNotFound)
		_, err = fake.GetUserByAccountID(ctx, target.ID)
		require.ErrorIs(t, err, domain.ErrNotFound)

		// Federation event published so followers tombstone the actor.
		assertOutboxContainsEvent(t, fake, domain.EventAccountDeleted, target.ID)

		// Audit row is written in the same transaction as the delete.
		assertAdminActionRecorded(t, fake, AdminActionDeleteAccount, target.ID)
	})

	t.Run("remote_refused", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewModerationService(fake, noopBlocklistRefresher{})
		mod := seedLocalAccount(t, fake, "mod-1", "moderator")
		remote := seedRemoteAccount(t, fake, "remote-1", "remote", "other.example")

		err := svc.DeleteAccount(ctx, mod.ID, remote.ID)
		assert.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func assertOutboxContainsEvent(t *testing.T, fake *testutil.FakeStore, eventType, aggregateID string) {
	t.Helper()
	for _, ev := range fake.OutboxEvents {
		if ev.EventType == eventType && ev.AggregateID == aggregateID {
			return
		}
	}
	t.Fatalf("expected outbox event %q for %q, found %d events", eventType, aggregateID, len(fake.OutboxEvents))
}

func assertAdminActionRecorded(t *testing.T, fake *testutil.FakeStore, action, targetAccountID string) {
	t.Helper()
	for _, a := range fake.AdminActions {
		if a.Action != action {
			continue
		}
		if a.TargetAccountID == nil || *a.TargetAccountID != targetAccountID {
			continue
		}
		return
	}
	t.Fatalf("expected admin action %q for target %q, found %d actions", action, targetAccountID, len(fake.AdminActions))
}
