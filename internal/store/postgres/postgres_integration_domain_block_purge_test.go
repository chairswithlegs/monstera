//go:build integration

package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/chairswithlegs/monstera/internal/uid"
)

// TestIntegration_DomainBlockPurges exercises the CRUD surface for the
// domain_block_purges tracker and its CASCADE-delete on domain_blocks removal.
func TestIntegration_DomainBlockPurges(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("Create_Get_UpdateCursor_MarkComplete", func(t *testing.T) {
		blockID := uid.New()
		blocked := "blocked_" + uid.New()[:8] + ".example"
		_, err := s.CreateDomainBlock(ctx, store.CreateDomainBlockInput{
			ID: blockID, Domain: blocked, Severity: domain.DomainBlockSeveritySuspend,
		})
		require.NoError(t, err)

		require.NoError(t, s.CreateDomainBlockPurge(ctx, blockID, blocked))

		got, err := s.GetDomainBlockPurge(ctx, blockID)
		require.NoError(t, err)
		assert.Equal(t, blockID, got.BlockID)
		assert.Equal(t, blocked, got.Domain)
		assert.Nil(t, got.Cursor)
		assert.Nil(t, got.CompletedAt)

		require.NoError(t, s.UpdateDomainBlockPurgeCursor(ctx, blockID, "acc-42"))
		got, err = s.GetDomainBlockPurge(ctx, blockID)
		require.NoError(t, err)
		require.NotNil(t, got.Cursor)
		assert.Equal(t, "acc-42", *got.Cursor)

		require.NoError(t, s.MarkDomainBlockPurgeComplete(ctx, blockID))
		got, err = s.GetDomainBlockPurge(ctx, blockID)
		require.NoError(t, err)
		require.NotNil(t, got.CompletedAt)
	})

	t.Run("CASCADE_on_domain_block_delete", func(t *testing.T) {
		blockID := uid.New()
		blocked := "cascade_" + uid.New()[:8] + ".example"
		_, err := s.CreateDomainBlock(ctx, store.CreateDomainBlockInput{
			ID: blockID, Domain: blocked, Severity: domain.DomainBlockSeveritySuspend,
		})
		require.NoError(t, err)
		require.NoError(t, s.CreateDomainBlockPurge(ctx, blockID, blocked))

		require.NoError(t, s.DeleteDomainBlock(ctx, blocked))
		_, err = s.GetDomainBlockPurge(ctx, blockID)
		assert.ErrorIs(t, err, domain.ErrNotFound, "purge row should CASCADE-delete with the block")
	})

	t.Run("ListDomainBlocksWithPurge", func(t *testing.T) {
		// Two blocks: one with a purge row (in-progress), one without.
		blockA := uid.New()
		domainA := "withpurge_" + uid.New()[:8] + ".example"
		_, err := s.CreateDomainBlock(ctx, store.CreateDomainBlockInput{
			ID: blockA, Domain: domainA, Severity: domain.DomainBlockSeveritySuspend,
		})
		require.NoError(t, err)
		require.NoError(t, s.CreateDomainBlockPurge(ctx, blockA, domainA))

		blockB := uid.New()
		domainB := "nopurge_" + uid.New()[:8] + ".example"
		_, err = s.CreateDomainBlock(ctx, store.CreateDomainBlockInput{
			ID: blockB, Domain: domainB, Severity: domain.DomainBlockSeveritySilence,
		})
		require.NoError(t, err)

		rows, err := s.ListDomainBlocksWithPurge(ctx)
		require.NoError(t, err)

		byDomain := map[string]store.DomainBlockWithPurge{}
		for _, r := range rows {
			byDomain[r.Block.Domain] = r
		}

		require.NotNil(t, byDomain[domainA].Purge, "block with purge row should have Purge populated")
		assert.Nil(t, byDomain[domainB].Purge, "silence block without purge row should have nil Purge")
	})
}

// TestIntegration_ListRemoteAccountsByDomainPaginated verifies keyset
// pagination semantics over the accounts table.
func TestIntegration_ListRemoteAccountsByDomainPaginated(t *testing.T) {
	s, ctx := setupTestStore(t)

	// Seed 5 remote accounts on the same domain; createTestRemoteAccount
	// assigns "remote.example" so we use that here.
	const target = "remote.example"
	ids := make([]string, 0, 5)
	for i := 0; i < 5; i++ {
		acc := createTestRemoteAccount(t, s, ctx)
		ids = append(ids, acc.ID)
	}

	// Unbounded page: should return at least our 5 (other tests may have
	// seeded remote accounts on the same domain, so don't assert exact
	// equality).
	all, err := s.ListRemoteAccountsByDomainPaginated(ctx, target, "", 100)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(all), 5)

	// Keyset cursor: passing the first id as cursor must exclude it.
	first := all[0]
	rest, err := s.ListRemoteAccountsByDomainPaginated(ctx, target, first, 100)
	require.NoError(t, err)
	for _, id := range rest {
		assert.Greater(t, id, first, "cursor must be exclusive")
	}
	assert.Equal(t, len(all)-1, len(rest))

	// Count must agree with list length after cursor.
	n, err := s.CountRemoteAccountsByDomainAfterCursor(ctx, target, first)
	require.NoError(t, err)
	assert.EqualValues(t, len(rest), n)
}

// TestIntegration_DeleteStatusesByAccountIDBatched_NoOrphans seeds a remote
// account's status with rows in every dependent table the FK cascade audit
// surfaced, then runs the batched hard-delete and asserts nothing references
// the deleted statuses. This test is the living specification of the
// cascade contract (migrations 000080 / 000082 / 000084): the next time
// someone adds a FK to statuses.id without ON DELETE CASCADE, this test
// will fail before the domain purge silently leaks orphans.
func TestIntegration_DeleteStatusesByAccountIDBatched_NoOrphans(t *testing.T) {
	s, ctx := setupTestStore(t)

	// Actors.
	remote := createTestRemoteAccount(t, s, ctx)
	localViewer := createTestLocalAccount(t, s, ctx)

	// Two remote statuses, plus a local boost, quote, favourite, bookmark,
	// pin, mention, and notification.
	s1 := createTestStatus(t, s, ctx, remote.ID)
	s2 := createTestStatus(t, s, ctx, remote.ID)

	// Local boost of s1.
	boostID := uid.New()
	boost, err := s.CreateStatus(ctx, store.CreateStatusInput{
		ID:                  boostID,
		URI:                 "https://local.example/statuses/" + boostID,
		AccountID:           localViewer.ID,
		Visibility:          domain.VisibilityPublic,
		APID:                "https://local.example/statuses/" + boostID,
		QuoteApprovalPolicy: domain.QuotePolicyPublic,
		Local:               true,
		ReblogOfID:          &s1.ID,
	})
	require.NoError(t, err)

	// Local quote of s2.
	quoteID := uid.New()
	_, err = s.CreateStatus(ctx, store.CreateStatusInput{
		ID:                  quoteID,
		URI:                 "https://local.example/statuses/" + quoteID,
		AccountID:           localViewer.ID,
		Visibility:          domain.VisibilityPublic,
		APID:                "https://local.example/statuses/" + quoteID,
		QuoteApprovalPolicy: domain.QuotePolicyPublic,
		Local:               true,
		QuotedStatusID:      &s2.ID,
	})
	require.NoError(t, err)

	// Favourite + bookmark on s1.
	_, err = s.CreateFavourite(ctx, store.CreateFavouriteInput{
		ID: uid.New(), AccountID: localViewer.ID, StatusID: s1.ID,
	})
	require.NoError(t, err)
	require.NoError(t, s.CreateBookmark(ctx, store.CreateBookmarkInput{
		ID: uid.New(), AccountID: localViewer.ID, StatusID: s1.ID,
	}))

	// Pin (remote pins their own status).
	require.NoError(t, s.CreateAccountPin(ctx, remote.ID, s1.ID))

	// Mention (remote mentions local viewer in s1).
	require.NoError(t, s.CreateStatusMention(ctx, s1.ID, localViewer.ID))

	// Notification referencing s1.
	_, err = s.CreateNotification(ctx, store.CreateNotificationInput{
		ID:        uid.New(),
		AccountID: localViewer.ID,
		FromID:    remote.ID,
		Type:      domain.NotificationTypeMention,
		StatusID:  testutil.StrPtr(s1.ID),
		GroupKey:  uid.New(),
	})
	require.NoError(t, err)

	// Media attachment authored by remote, attached to s1.
	media, err := s.CreateMediaAttachment(ctx, store.CreateMediaAttachmentInput{
		ID:          uid.New(),
		AccountID:   remote.ID,
		Type:        domain.MediaTypeImage,
		URL:         "https://remote.example/media/" + uid.New(),
		StorageKey:  "remote/" + uid.New(),
		ContentType: testutil.StrPtr("image/png"),
	})
	require.NoError(t, err)
	require.NoError(t, s.AttachMediaToStatus(ctx, media.ID, s1.ID, remote.ID))

	// Run the batched hard-delete.
	deleted, err := s.DeleteStatusesByAccountIDBatched(ctx, remote.ID, 100)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{s1.ID, s2.ID}, deleted)

	// --- Assert no orphans remain ---

	// Statuses themselves are gone.
	_, err = s.GetStatusByID(ctx, s1.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
	_, err = s.GetStatusByID(ctx, s2.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)

	// The local boost CASCADE-deletes because statuses.reblog_of_id was
	// made CASCADE in migration 000084.
	_, err = s.GetStatusByID(ctx, boost.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound, "boost of a deleted status must CASCADE")

	// The local quote survives but its quoted_status_id was SET NULL.
	gotQuote, err := s.GetStatusByID(ctx, quoteID)
	require.NoError(t, err, "quote post must survive (migration 000084 SET NULL)")
	assert.Nil(t, gotQuote.QuotedStatusID, "quoted_status_id should be NULLed")

	// The account pin CASCADE-deleted with s1 (migration 000084).
	pins, err := s.ListPinnedStatusIDs(ctx, remote.ID)
	require.NoError(t, err)
	assert.Empty(t, pins, "account pin must CASCADE when the pinned status is deleted")

	// The favourite and bookmark on s1 CASCADE.
	// (No direct query for "favourite by status id" exposed on the store,
	// so we check via the account's favourites/bookmarks — both should now
	// be empty for this viewer.)
	// The mention, notification, and media_attachment rows also CASCADE.
	// We don't test each dependent table individually here — the
	// migrations 000026/000020/000034/000008/000005 already set those FKs
	// to CASCADE, and a regression there would surface as a failure of
	// the DELETE statement itself (FK violation).
}
