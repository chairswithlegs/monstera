package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
)

// The scheduler job drops snapshots past their expires_at. Targets cascade
// with them. Non-expired snapshots (and their targets) must survive.
func TestPurgeAccountDeletionSnapshots_removesExpiredOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := service.NewAccountDeletionService(fake)

	require.NoError(t, fake.CreateAccountDeletionSnapshot(ctx, store.CreateAccountDeletionSnapshotInput{
		ID:            "expired",
		APID:          "https://example.com/users/alice",
		PrivateKeyPEM: "pem",
		ExpiresAt:     time.Now().Add(-time.Hour),
	}))
	require.NoError(t, fake.CreateAccountDeletionSnapshot(ctx, store.CreateAccountDeletionSnapshotInput{
		ID:            "fresh",
		APID:          "https://example.com/users/bob",
		PrivateKeyPEM: "pem",
		ExpiresAt:     time.Now().Add(time.Hour),
	}))
	fake.SeedAccountDeletionTargets("expired", []string{"https://a.example/inbox"})
	fake.SeedAccountDeletionTargets("fresh", []string{"https://b.example/inbox"})

	require.NoError(t, PurgeAccountDeletionSnapshots(svc)(ctx))

	_, err := fake.GetAccountDeletionSnapshot(ctx, "expired")
	require.Error(t, err, "expired snapshot must be purged")

	fresh, err := fake.GetAccountDeletionSnapshot(ctx, "fresh")
	require.NoError(t, err)
	assert.Equal(t, "fresh", fresh.ID)

	expiredTargets, err := fake.ListPendingAccountDeletionTargets(ctx, "expired", "", 10)
	require.NoError(t, err)
	assert.Empty(t, expiredTargets, "CASCADE drops targets with their snapshot")

	freshTargets, err := fake.ListPendingAccountDeletionTargets(ctx, "fresh", "", 10)
	require.NoError(t, err)
	assert.Len(t, freshTargets, 1)
}
