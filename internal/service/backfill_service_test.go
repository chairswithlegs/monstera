package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/chairswithlegs/monstera/internal/uid"
)

// fakeJetStream embeds the full interface so only Publish needs implementing.
type fakeJetStream struct {
	jetstream.JetStream

	mu        sync.Mutex
	publishes []fakePublish
}

type fakePublish struct {
	subject string
	data    []byte
}

func (f *fakeJetStream) Publish(_ context.Context, subject string, payload []byte, _ ...jetstream.PublishOpt) (*jetstream.PubAck, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.publishes = append(f.publishes, fakePublish{subject: subject, data: payload})
	return &jetstream.PubAck{}, nil
}

func (f *fakeJetStream) published() []fakePublish {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]fakePublish, len(f.publishes))
	copy(out, f.publishes)
	return out
}

func createRemoteAccount(t *testing.T, ctx context.Context, s *testutil.FakeStore, opts ...func(*store.CreateAccountInput)) *domain.Account {
	t.Helper()
	id := uid.New()
	in := store.CreateAccountInput{
		ID:        id,
		Username:  "remote-" + id[:8],
		Domain:    ptr("example.com"),
		PublicKey: "pk",
		OutboxURL: "https://example.com/users/remote/outbox",
		APID:      "https://example.com/users/remote-" + id[:8],
		InboxURL:  "https://example.com/users/remote/inbox",
	}
	for _, o := range opts {
		o(&in)
	}
	acc, err := s.CreateAccount(ctx, in)
	require.NoError(t, err)
	return acc
}

func createLocalAccount(t *testing.T, ctx context.Context, s *testutil.FakeStore) *domain.Account {
	t.Helper()
	id := uid.New()
	acc, err := s.CreateAccount(ctx, store.CreateAccountInput{
		ID:        id,
		Username:  "local-" + id[:8],
		PublicKey: "pk",
		APID:      "https://local.test/users/local-" + id[:8],
	})
	require.NoError(t, err)
	return acc
}

func TestBackfillService_RequestBackfill(t *testing.T) {
	t.Parallel()

	t.Run("account not found returns error", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		js := &fakeJetStream{}
		svc := NewBackfillService(fake, js, time.Hour)

		err := svc.RequestBackfill(ctx, "nonexistent")
		require.Error(t, err)
		require.ErrorIs(t, err, domain.ErrNotFound)
		assert.Empty(t, js.published())
	})

	t.Run("local account is no-op", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		js := &fakeJetStream{}
		svc := NewBackfillService(fake, js, time.Hour)

		acc := createLocalAccount(t, ctx, fake)

		err := svc.RequestBackfill(ctx, acc.ID)
		require.NoError(t, err)
		assert.Empty(t, js.published())
	})

	t.Run("remote account with no outbox URL is no-op", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		js := &fakeJetStream{}
		svc := NewBackfillService(fake, js, time.Hour)

		acc := createRemoteAccount(t, ctx, fake, func(in *store.CreateAccountInput) {
			in.OutboxURL = ""
		})

		err := svc.RequestBackfill(ctx, acc.ID)
		require.NoError(t, err)
		assert.Empty(t, js.published())
	})

	t.Run("remote account recently backfilled is no-op", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		js := &fakeJetStream{}
		cooldown := 2 * time.Hour
		svc := NewBackfillService(fake, js, cooldown)

		acc := createRemoteAccount(t, ctx, fake)

		// Set LastBackfilledAt to a recent time (within cooldown).
		recent := time.Now().Add(-30 * time.Minute)
		err := fake.UpdateAccountLastBackfilledAt(ctx, acc.ID, recent)
		require.NoError(t, err)

		err = svc.RequestBackfill(ctx, acc.ID)
		require.NoError(t, err)
		assert.Empty(t, js.published())
	})

	t.Run("remote account eligible publishes backfill message", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		js := &fakeJetStream{}
		svc := NewBackfillService(fake, js, time.Hour)

		acc := createRemoteAccount(t, ctx, fake)

		err := svc.RequestBackfill(ctx, acc.ID)
		require.NoError(t, err)

		pubs := js.published()
		require.Len(t, pubs, 1)
		assert.Equal(t, BackfillSubjectPrefix+acc.ID, pubs[0].subject)
		assert.Equal(t, []byte(acc.ID), pubs[0].data)
	})

	t.Run("remote account backfilled beyond cooldown publishes message", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		js := &fakeJetStream{}
		cooldown := time.Hour
		svc := NewBackfillService(fake, js, cooldown)

		acc := createRemoteAccount(t, ctx, fake)

		// Set LastBackfilledAt to well beyond the cooldown.
		old := time.Now().Add(-3 * time.Hour)
		err := fake.UpdateAccountLastBackfilledAt(ctx, acc.ID, old)
		require.NoError(t, err)

		err = svc.RequestBackfill(ctx, acc.ID)
		require.NoError(t, err)

		pubs := js.published()
		require.Len(t, pubs, 1)
		assert.Equal(t, BackfillSubjectPrefix+acc.ID, pubs[0].subject)
	})
}
