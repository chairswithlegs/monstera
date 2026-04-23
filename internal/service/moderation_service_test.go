package service

import (
	"context"
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
	// parse it.
	pk := "-----BEGIN RSA PRIVATE KEY-----\nstub\n-----END RSA PRIVATE KEY-----"
	acc, err := fake.CreateAccount(context.Background(), store.CreateAccountInput{
		ID:         id,
		Username:   username,
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
				require.Error(t, err)
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
				acc, err := fake.GetAccountByID(ctx, targetID)
				require.NoError(t, err)
				assert.True(t, acc.Suspended)
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

	t.Run("suspend severity deletes follows", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewModerationService(fake, noopBlocklistRefresher{})

		mod := seedLocalAccount(t, fake, "mod-1", "moderator")

		block, err := svc.CreateDomainBlock(ctx, mod.ID, CreateDomainBlockInput{
			Domain:   "bad.example.com",
			Severity: domain.DomainBlockSeveritySuspend,
		})
		require.NoError(t, err)
		require.NotNil(t, block)
		assert.Equal(t, "bad.example.com", block.Domain)
		assert.Equal(t, domain.DomainBlockSeveritySuspend, block.Severity)
	})
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
