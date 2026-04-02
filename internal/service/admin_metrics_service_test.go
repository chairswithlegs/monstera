package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/cache"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/chairswithlegs/monstera/internal/uid"
)

// syncCache is a synchronous in-memory cache for testing.
// Ristretto writes asynchronously so it's unsuitable for tests that
// need immediate read-after-write consistency.
type syncCache struct {
	mu    sync.Mutex
	items map[string][]byte
}

func newSyncCache() *syncCache {
	return &syncCache{items: make(map[string][]byte)}
}

func (s *syncCache) Get(_ context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.items[key]
	if !ok {
		return nil, cache.ErrCacheMiss
	}
	return v, nil
}

func (s *syncCache) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[key] = value
	return nil
}

func (s *syncCache) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, key)
	return nil
}

func (s *syncCache) Exists(_ context.Context, key string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.items[key]
	return ok, nil
}

func (s *syncCache) Close() error { return nil }

// mockJetStream implements only the Stream method needed by AdminMetricsService.
type mockJetStream struct {
	jetstream.JetStream
	streams map[string]*jetstream.StreamInfo
}

func (m *mockJetStream) Stream(_ context.Context, name string) (jetstream.Stream, error) {
	info, ok := m.streams[name]
	if !ok {
		return nil, jetstream.ErrStreamNotFound
	}
	return &mockStream{info: info}, nil
}

type mockStream struct {
	jetstream.Stream
	info *jetstream.StreamInfo
}

func (m *mockStream) Info(_ context.Context, _ ...jetstream.StreamInfoOpt) (*jetstream.StreamInfo, error) {
	return m.info, nil
}

func TestAdminMetricsService_GetMetrics(t *testing.T) {
	t.Parallel()

	t.Run("returns counts from store and NATS", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		st := testutil.NewFakeStore()

		// Create local and remote accounts.
		remoteDomain := ptrTo("remote.example")
		_, err := st.CreateAccount(ctx, store.CreateAccountInput{
			ID: uid.New(), Username: "localuser", Domain: nil,
			PublicKey: "pk", InboxURL: "https://example.com/inbox",
			OutboxURL: "https://example.com/outbox", FollowersURL: "https://example.com/followers",
			FollowingURL: "https://example.com/following", APID: "https://example.com/users/localuser",
		})
		require.NoError(t, err)
		_, err = st.CreateAccount(ctx, store.CreateAccountInput{
			ID: uid.New(), Username: "remoteuser", Domain: remoteDomain,
			PublicKey: "pk", InboxURL: "https://remote.example/inbox",
			OutboxURL: "https://remote.example/outbox", FollowersURL: "https://remote.example/followers",
			FollowingURL: "https://remote.example/following", APID: "https://remote.example/users/remoteuser",
		})
		require.NoError(t, err)

		// Create local and remote statuses.
		_, err = st.CreateStatus(ctx, store.CreateStatusInput{
			ID: uid.New(), AccountID: "acc1", Content: ptrTo("hello"), Local: true,
			Visibility: domain.VisibilityPublic, APID: "https://example.com/statuses/1",
		})
		require.NoError(t, err)
		_, err = st.CreateStatus(ctx, store.CreateStatusInput{
			ID: uid.New(), AccountID: "acc2", Content: ptrTo("world"), Local: false,
			Visibility: domain.VisibilityPublic, APID: "https://remote.example/statuses/1",
		})
		require.NoError(t, err)

		js := &mockJetStream{
			streams: map[string]*jetstream.StreamInfo{
				"DLQ_DELIVERY": {State: jetstream.StreamState{Msgs: 3}},
				"DLQ_FANOUT":   {State: jetstream.StreamState{Msgs: 7}},
			},
		}

		svc := NewAdminMetricsService(st, js, newSyncCache(), "DLQ_DELIVERY", "DLQ_FANOUT")
		metrics, err := svc.GetMetrics(ctx)
		require.NoError(t, err)

		assert.Equal(t, int64(1), metrics.LocalAccounts)
		assert.Equal(t, int64(1), metrics.RemoteAccounts)
		assert.Equal(t, int64(1), metrics.LocalStatuses)
		assert.Equal(t, int64(1), metrics.RemoteStatuses)
		assert.Equal(t, int64(0), metrics.KnownInstances)
		assert.Equal(t, int64(0), metrics.OpenReports)
		assert.Equal(t, int64(3), metrics.DeliveryDLQDepth)
		assert.Equal(t, int64(7), metrics.FanoutDLQDepth)
	})

	t.Run("returns cached result on second call", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		st := testutil.NewFakeStore()
		js := &mockJetStream{
			streams: map[string]*jetstream.StreamInfo{
				"DLQ_D": {State: jetstream.StreamState{Msgs: 0}},
				"DLQ_F": {State: jetstream.StreamState{Msgs: 0}},
			},
		}
		svc := NewAdminMetricsService(st, js, newSyncCache(), "DLQ_D", "DLQ_F")

		m1, err := svc.GetMetrics(ctx)
		require.NoError(t, err)

		m2, err := svc.GetMetrics(ctx)
		require.NoError(t, err)

		assert.Equal(t, m1.LocalAccounts, m2.LocalAccounts)
	})
}

func ptrTo(s string) *string { return &s }
