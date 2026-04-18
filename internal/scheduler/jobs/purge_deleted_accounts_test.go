package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
)

// seedLocalDeletionPending creates a local account and drives it into
// deletion-pending state (DeletionRequestedAt = now, suspended = true).
func seedLocalDeletionPending(t *testing.T, st *testutil.FakeStore, username string) string {
	t.Helper()
	ctx := context.Background()
	acc, err := st.CreateAccount(ctx, store.CreateAccountInput{
		ID:           "acc-" + username,
		Username:     username,
		PublicKey:    "pk",
		InboxURL:     "https://example.com/users/" + username + "/inbox",
		OutboxURL:    "https://example.com/users/" + username + "/outbox",
		FollowersURL: "https://example.com/users/" + username + "/followers",
		FollowingURL: "https://example.com/users/" + username + "/following",
		APID:         "https://example.com/users/" + username,
	})
	require.NoError(t, err)
	require.NoError(t, st.RequestAccountDeletion(ctx, acc.ID))
	return acc.ID
}

// TestPurgeDeletedAccounts_past_grace_is_purged uses grace=0 so "deletion
// requested at time T0 is past grace immediately" — the handler picks up the
// account on its first tick and hard-deletes it.
func TestPurgeDeletedAccounts_past_grace_is_purged(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	svc := service.NewAccountService(st, "https://example.com")
	accID := seedLocalDeletionPending(t, st, "alice")

	err := PurgeDeletedAccounts(st, svc, 0)(ctx)
	require.NoError(t, err)

	_, err = st.GetAccountByID(ctx, accID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// TestPurgeDeletedAccounts_inside_grace_is_kept checks the time filter: with a
// 30-day grace, a just-requested deletion is not past grace and the account
// must remain intact.
func TestPurgeDeletedAccounts_inside_grace_is_kept(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	svc := service.NewAccountService(st, "https://example.com")
	accID := seedLocalDeletionPending(t, st, "alice")

	err := PurgeDeletedAccounts(st, svc, 30*24*time.Hour)(ctx)
	require.NoError(t, err)

	got, err := st.GetAccountByID(ctx, accID)
	require.NoError(t, err)
	assert.NotNil(t, got.DeletionRequestedAt)
}

// TestPurgeDeletedAccounts_remote_account_is_never_selected seeds a remote
// account with deletion_requested_at + past grace. The SQL predicate (domain
// IS NULL) must exclude it regardless; the job is a no-op.
func TestPurgeDeletedAccounts_remote_account_is_never_selected(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	svc := service.NewAccountService(st, "https://example.com")

	remoteDomain := "remote.example"
	acc, err := st.CreateAccount(ctx, store.CreateAccountInput{
		ID:        "remote-1",
		Username:  "bob",
		Domain:    &remoteDomain,
		PublicKey: "pk",
		InboxURL:  "https://remote.example/users/bob/inbox",
		APID:      "https://remote.example/users/bob",
	})
	require.NoError(t, err)

	// Flagging a remote account for deletion is not a real production flow
	// (the schema allows it but service layer blocks it via requireLocal).
	// Pushing it into that state directly ensures the SQL filter is what
	// protects us, not the service guard. The fake store's
	// RequestAccountDeletion doesn't filter by locality so we can drive the
	// row there directly.
	require.NoError(t, st.RequestAccountDeletion(ctx, acc.ID))

	err = PurgeDeletedAccounts(st, svc, 0)(ctx)
	require.NoError(t, err)

	got, err := st.GetAccountByID(ctx, acc.ID)
	require.NoError(t, err)
	assert.NotNil(t, got.DeletionRequestedAt, "remote account must survive the purge sweep")
}

func TestPurgeDeletedAccounts_no_pending_accounts_is_noop(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	svc := service.NewAccountService(st, "https://example.com")

	err := PurgeDeletedAccounts(st, svc, 30*24*time.Hour)(ctx)
	assert.NoError(t, err)
}
