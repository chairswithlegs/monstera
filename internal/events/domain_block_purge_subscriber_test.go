package events

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/chairswithlegs/monstera/internal/uid"
)

// seedRemoteDomain seeds n remote accounts on the given domain (ids
// id-1…id-n in sorted order), each owning statusesPerAccount statuses.
// Returns the purge payload the subscriber consumes.
func seedRemoteDomain(t *testing.T, fake *testutil.FakeStore, domainName string, n, statusesPerAccount int) (domain.DomainBlockSuspendedPayload, string) {
	t.Helper()
	ctx := context.Background()
	blockID := "blk-" + uid.New()

	// Create a domain block + purge tracker via the fake store directly,
	// bypassing the service (we're testing the subscriber in isolation).
	_, err := fake.CreateDomainBlock(ctx, store.CreateDomainBlockInput{
		ID: blockID, Domain: domainName, Severity: domain.DomainBlockSeveritySuspend,
	})
	require.NoError(t, err)
	require.NoError(t, fake.CreateDomainBlockPurge(ctx, blockID, domainName))

	for i := 1; i <= n; i++ {
		accID := fmt.Sprintf("%s-%04d", domainName, i)
		d := domainName
		fake.SeedAccount(&domain.Account{ID: accID, Username: accID, Domain: &d, APID: "https://" + domainName + "/users/" + accID})
		for j := 1; j <= statusesPerAccount; j++ {
			fake.SeedStatus(&domain.Status{
				ID:        fmt.Sprintf("%s-s%04d", accID, j),
				AccountID: accID,
				APID:      fmt.Sprintf("https://%s/statuses/%s-%d", domainName, accID, j),
				URI:       fmt.Sprintf("https://%s/statuses/%s-%d", domainName, accID, j),
			})
		}
	}
	return domain.DomainBlockSuspendedPayload{BlockID: blockID, Domain: domainName}, blockID
}

// makeSubscriber constructs a DomainBlockPurgeSubscriber wired against a
// FakeStore. js is nil; tests invoke processBatch directly.
func makeSubscriber(fake *testutil.FakeStore) *DomainBlockPurgeSubscriber {
	return &DomainBlockPurgeSubscriber{deps: fake}
}

func TestDomainBlockPurgeSubscriber_happyPath(t *testing.T) {
	t.Parallel()
	fake := testutil.NewFakeStore()
	payload, blockID := seedRemoteDomain(t, fake, "bad.example", 3, 2)
	sub := makeSubscriber(fake)

	sub.processBatch(context.Background(), payload)

	// Visibility (domain_suspended) is set atomically in CreateDomainBlock;
	// the subscriber's job is only to hard-delete content. Verify all
	// statuses for every account on the domain are gone.
	statuses, err := fake.GetAccountStatuses(context.Background(), "bad.example-0001", nil, 100)
	require.NoError(t, err)
	assert.Empty(t, statuses)

	// Purge tracker marked complete (only 3 accounts, which is below
	// accountsPerMessage, so the subscriber sees the tail and marks
	// complete on the same invocation via the empty-list branch... except
	// ListRemoteAccountsByDomainPaginated returned 3 on this call — the
	// completion happens on the NEXT invocation with cursor at the tail.
	// Simulate that with a redelivery.
	sub.processBatch(context.Background(), payload)
	purge, err := fake.GetDomainBlockPurge(context.Background(), blockID)
	require.NoError(t, err)
	require.NotNil(t, purge.CompletedAt, "purge should be marked complete after second pass drains the empty tail")

	// EventMediaPurge emitted once per account (3 events).
	var mediaPurgeCount int
	for _, e := range fake.OutboxEvents {
		if e.EventType == domain.EventMediaPurge {
			mediaPurgeCount++
		}
	}
	assert.Equal(t, 3, mediaPurgeCount)
}

func TestDomainBlockPurgeSubscriber_chunkedStatusDelete(t *testing.T) {
	t.Parallel()
	// One account seeded with > statusDeleteBatch statuses.
	fake := testutil.NewFakeStore()
	const n = statusDeleteBatch + 250
	payload, _ := seedRemoteDomain(t, fake, "chunky.example", 1, n)
	sub := makeSubscriber(fake)

	sub.processBatch(context.Background(), payload)

	statuses, err := fake.GetAccountStatuses(context.Background(), "chunky.example-0001", nil, 100)
	require.NoError(t, err)
	assert.Empty(t, statuses, "all statuses should be deleted after chunked loop completes")
}

func TestDomainBlockPurgeSubscriber_redeliveryAfterComplete(t *testing.T) {
	t.Parallel()
	fake := testutil.NewFakeStore()
	payload, blockID := seedRemoteDomain(t, fake, "done.example", 2, 1)
	sub := makeSubscriber(fake)

	// Seed a status so we can assert "no work happened".
	fake.SeedStatus(&domain.Status{ID: "keepme", AccountID: "done.example-0001", APID: "https://done.example/statuses/keepme", URI: "https://done.example/statuses/keepme"})

	// Mark complete up front; redelivery should no-op.
	require.NoError(t, fake.MarkDomainBlockPurgeComplete(context.Background(), blockID))
	sub.processBatch(context.Background(), payload)

	// No EventMediaPurge emitted (we bailed before any per-account work),
	// and seeded status is still there.
	for _, e := range fake.OutboxEvents {
		assert.NotEqual(t, domain.EventMediaPurge, e.EventType, "already-complete purge should not re-process accounts")
	}
	kept, err := fake.GetStatusByID(context.Background(), "keepme")
	require.NoError(t, err)
	require.NotNil(t, kept)
}

func TestDomainBlockPurgeSubscriber_blockDeleted(t *testing.T) {
	t.Parallel()
	// If the admin removes the domain block mid-purge the CASCADE drops
	// the purge row; the subscriber sees ErrNotFound and bails cleanly.
	fake := testutil.NewFakeStore()
	payload, _ := seedRemoteDomain(t, fake, "gone.example", 1, 1)
	sub := makeSubscriber(fake)

	require.NoError(t, fake.DeleteDomainBlock(context.Background(), "gone.example"))

	sub.processBatch(context.Background(), payload) // must not panic, must not touch accounts

	acc, err := fake.GetAccountByID(context.Background(), "gone.example-0001")
	require.NoError(t, err)
	assert.False(t, acc.Suspended)
}

func TestDomainBlockPurgeSubscriber_continuationEventWhenBatchFull(t *testing.T) {
	t.Parallel()
	// accountsPerMessage accounts exactly: subscriber should emit a
	// continuation EventDomainBlockSuspended so a follow-up message picks
	// up from the cursor.
	fake := testutil.NewFakeStore()
	payload, blockID := seedRemoteDomain(t, fake, "big.example", accountsPerMessage, 1)
	sub := makeSubscriber(fake)

	sub.processBatch(context.Background(), payload)

	// Cursor advanced to the highest account id.
	purge, err := fake.GetDomainBlockPurge(context.Background(), blockID)
	require.NoError(t, err)
	require.NotNil(t, purge.Cursor)
	assert.Equal(t, fmt.Sprintf("big.example-%04d", accountsPerMessage), *purge.Cursor)

	// Continuation event emitted.
	var continuation int
	for _, e := range fake.OutboxEvents {
		if e.EventType == domain.EventDomainBlockSuspended {
			continuation++
		}
	}
	assert.Equal(t, 1, continuation, "subscriber should emit one continuation event when the batch is full")
}
