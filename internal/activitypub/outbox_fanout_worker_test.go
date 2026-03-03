package activitypub

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera-fed/internal/config"
	natsutil "github.com/chairswithlegs/monstera-fed/internal/nats"
	"github.com/chairswithlegs/monstera-fed/internal/store"
	"github.com/chairswithlegs/monstera-fed/internal/testutil"
)

func TestFanoutWorker_ProcessMessage_PublishesDeliveryPerInbox(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	cfg := &config.Config{InstanceDomain: "example.com"}

	acc, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01author", Username: "alice", APID: "https://example.com/users/alice",
	})
	require.NoError(t, err)
	remoteDomain := testRemoteDomain
	f1, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01f1", Username: "bob", Domain: &remoteDomain,
		APID: "https://" + testRemoteDomain + "/users/bob", InboxURL: "https://" + testRemoteDomain + "/inbox",
	})
	require.NoError(t, err)
	_, err = fake.CreateFollow(ctx, store.CreateFollowInput{
		ID: "01follow", AccountID: f1.ID, TargetID: acc.ID, State: "accepted",
	})
	require.NoError(t, err)

	var delivered []outboxDeliveryMessage
	deliveryMock := &mockDeliveryPublisher{
		publishFn: func(_ context.Context, _ string, msg outboxDeliveryMessage) error {
			delivered = append(delivered, msg)
			return nil
		},
	}

	fanoutMsg := outboxFanoutMessage{
		ActivityID:   "https://example.com/activities/01act",
		Activity:     json.RawMessage(`{"type":"Create","object":{"type":"Note"}}`),
		ActivityType: "create",
		SenderID:     acc.ID,
	}
	data, err := json.Marshal(fanoutMsg)
	require.NoError(t, err)

	w := &outboxFanoutWorker{store: fake, delivery: deliveryMock, cfg: cfg}
	w.processMessage(ctx, &mockJetstreamMsg{data: data})

	require.Len(t, delivered, 1)
	assert.Equal(t, "https://example.com/activities/01act", delivered[0].ActivityID)
	assert.Equal(t, "https://"+testRemoteDomain+"/inbox", delivered[0].TargetInbox)
	assert.Equal(t, acc.ID, delivered[0].SenderID)
	assert.Equal(t, string(fanoutMsg.Activity), string(delivered[0].Activity))
}

func TestFanoutWorker_ProcessMessage_Pagination(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	cfg := &config.Config{InstanceDomain: "example.com"}

	acc, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01author", Username: "alice", APID: "https://example.com/users/alice",
	})
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		domain := "remote" + string(rune('a'+i)) + ".example"
		f, err := fake.CreateAccount(ctx, store.CreateAccountInput{
			ID: "01f" + string(rune('0'+i)), Username: "u" + string(rune('0'+i)), Domain: &domain,
			InboxURL: "https://remote" + string(rune('a'+i)) + ".example/inbox",
		})
		require.NoError(t, err)
		_, err = fake.CreateFollow(ctx, store.CreateFollowInput{
			ID: "01follow" + string(rune('0'+i)), AccountID: f.ID, TargetID: acc.ID, State: "accepted",
		})
		require.NoError(t, err)
	}

	var delivered []outboxDeliveryMessage
	deliveryMock := &mockDeliveryPublisher{
		publishFn: func(_ context.Context, _ string, msg outboxDeliveryMessage) error {
			delivered = append(delivered, msg)
			return nil
		},
	}

	fanoutMsg := outboxFanoutMessage{
		ActivityID:   "https://example.com/activities/01act",
		Activity:     json.RawMessage(`{"type":"Create"}`),
		ActivityType: "create",
		SenderID:     acc.ID,
	}
	data, err := json.Marshal(fanoutMsg)
	require.NoError(t, err)

	w := &outboxFanoutWorker{store: fake, delivery: deliveryMock, cfg: cfg}
	w.processMessage(ctx, &mockJetstreamMsg{data: data})

	require.Len(t, delivered, 3)
	inboxes := make(map[string]bool)
	for _, d := range delivered {
		inboxes[d.TargetInbox] = true
	}
	assert.Len(t, inboxes, 3, "each follower inbox should appear exactly once")
}

func TestFanoutWorker_ProcessMessage_EmptyFollowers_Acks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	cfg := &config.Config{InstanceDomain: "example.com"}

	acc, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01author", Username: "alice", APID: "https://example.com/users/alice",
	})
	require.NoError(t, err)

	var delivered []outboxDeliveryMessage
	deliveryMock := &mockDeliveryPublisher{
		publishFn: func(_ context.Context, _ string, msg outboxDeliveryMessage) error {
			delivered = append(delivered, msg)
			return nil
		},
	}

	fanoutMsg := outboxFanoutMessage{
		ActivityID:   "https://example.com/activities/01act",
		Activity:     json.RawMessage(`{"type":"Delete"}`),
		ActivityType: "delete",
		SenderID:     acc.ID,
	}
	data, err := json.Marshal(fanoutMsg)
	require.NoError(t, err)

	acked := false
	mockMsg := &mockJetstreamMsg{data: data, ackFn: func() { acked = true }}

	w := &outboxFanoutWorker{store: fake, delivery: deliveryMock, cfg: cfg}
	w.processMessage(ctx, mockMsg)

	assert.Empty(t, delivered)
	assert.True(t, acked)
}

// failingFanoutStore is a store that returns an error from GetDistinctFollowerInboxURLsPaginated.
type failingFanoutStore struct {
	*testutil.FakeStore
	err error
}

func (f *failingFanoutStore) GetDistinctFollowerInboxURLsPaginated(_ context.Context, _ string, _ string, _ int) ([]string, error) {
	return nil, f.err
}

func TestFanoutWorker_ProcessMessage_DBError_RetriesRemaining_NakWithDelay(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	failingStore := &failingFanoutStore{FakeStore: fake, err: errors.New("db error")}
	cfg := &config.Config{InstanceDomain: "example.com"}

	acc, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01author", Username: "alice", APID: "https://example.com/users/alice",
	})
	require.NoError(t, err)

	nakWithDelayCalled := false
	var nakDelay time.Duration
	mockMsg := &mockJetstreamMsg{
		data:         mustMarshal(t, outboxFanoutMessage{ActivityID: "https://example.com/01act", Activity: json.RawMessage(`{}`), ActivityType: "create", SenderID: acc.ID}),
		numDelivered: 1,
		nakWithDelayFn: func(d time.Duration) {
			nakWithDelayCalled = true
			nakDelay = d
		},
	}
	deliveryMock := &mockDeliveryPublisher{publishFn: func(context.Context, string, outboxDeliveryMessage) error { return nil }}

	w := &outboxFanoutWorker{store: failingStore, delivery: deliveryMock, cfg: cfg}
	w.processMessage(ctx, mockMsg)

	assert.True(t, nakWithDelayCalled)
	assert.Equal(t, 30*time.Second, nakDelay)
}

func TestFanoutWorker_ProcessMessage_DBError_RetryExhausted_SendsToDLQ(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	failingStore := &failingFanoutStore{FakeStore: fake, err: errors.New("db error")}
	cfg := &config.Config{InstanceDomain: "example.com"}

	acc, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01author", Username: "alice", APID: "https://example.com/users/alice",
	})
	require.NoError(t, err)

	var dlqSubject string
	var dlqPayload []byte
	dlqMock := &mockFanoutDLQPublisher{
		publishFn: func(_ context.Context, subject string, payload []byte, _ ...jetstream.PublishOpt) (*jetstream.PubAck, error) {
			dlqSubject = subject
			dlqPayload = payload
			return nil, nil
		},
	}
	acked := false
	mockMsg := &mockJetstreamMsg{
		data:         mustMarshal(t, outboxFanoutMessage{ActivityID: "https://example.com/01act", Activity: json.RawMessage(`{}`), ActivityType: "create", SenderID: acc.ID}),
		numDelivered: natsutil.MaxDeliverActivityPubOutboundFanout,
		ackFn:        func() { acked = true },
	}
	deliveryMock := &mockDeliveryPublisher{publishFn: func(context.Context, string, outboxDeliveryMessage) error { return nil }}

	w := &outboxFanoutWorker{store: failingStore, delivery: deliveryMock, dlqPublisher: dlqMock, cfg: cfg}
	w.processMessage(ctx, mockMsg)

	assert.True(t, acked)
	assert.Equal(t, natsutil.SubjectPrefixActivityPubOutboundFanoutDLQ+"create", dlqSubject)
	assert.NotEmpty(t, dlqPayload)
}

func TestFanoutWorker_ProcessMessage_PublishError_RetriesRemaining_NakWithDelay(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	cfg := &config.Config{InstanceDomain: "example.com"}

	acc, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01author", Username: "alice", APID: "https://example.com/users/alice",
	})
	require.NoError(t, err)
	remoteDomain := testRemoteDomain
	f1, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01f1", Username: "bob", Domain: &remoteDomain,
		APID: "https://" + testRemoteDomain + "/users/bob", InboxURL: "https://" + testRemoteDomain + "/inbox",
	})
	require.NoError(t, err)
	_, err = fake.CreateFollow(ctx, store.CreateFollowInput{
		ID: "01follow", AccountID: f1.ID, TargetID: acc.ID, State: "accepted",
	})
	require.NoError(t, err)

	nakWithDelayCalled := false
	mockMsg := &mockJetstreamMsg{
		data: mustMarshal(t, outboxFanoutMessage{
			ActivityID: "https://example.com/01act", Activity: json.RawMessage(`{}`), ActivityType: "create", SenderID: acc.ID,
		}),
		numDelivered:   1,
		nakWithDelayFn: func(time.Duration) { nakWithDelayCalled = true },
	}
	deliveryMock := &mockDeliveryPublisher{
		publishFn: func(context.Context, string, outboxDeliveryMessage) error { return errors.New("nats publish failed") },
	}

	w := &outboxFanoutWorker{store: fake, delivery: deliveryMock, cfg: cfg}
	w.processMessage(ctx, mockMsg)

	assert.True(t, nakWithDelayCalled)
}

func TestFanoutWorker_ProcessMessage_PublishError_RetryExhausted_SendsToDLQ(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	cfg := &config.Config{InstanceDomain: "example.com"}

	acc, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01author", Username: "alice", APID: "https://example.com/users/alice",
	})
	require.NoError(t, err)
	remoteDomain := testRemoteDomain
	f1, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01f1", Username: "bob", Domain: &remoteDomain,
		APID: "https://" + testRemoteDomain + "/users/bob", InboxURL: "https://" + testRemoteDomain + "/inbox",
	})
	require.NoError(t, err)
	_, err = fake.CreateFollow(ctx, store.CreateFollowInput{
		ID: "01follow", AccountID: f1.ID, TargetID: acc.ID, State: "accepted",
	})
	require.NoError(t, err)

	var dlqSubject string
	dlqMock := &mockFanoutDLQPublisher{
		publishFn: func(_ context.Context, subject string, payload []byte, _ ...jetstream.PublishOpt) (*jetstream.PubAck, error) {
			dlqSubject = subject
			return nil, nil
		},
	}
	acked := false
	mockMsg := &mockJetstreamMsg{
		data:         mustMarshal(t, outboxFanoutMessage{ActivityID: "https://example.com/01act", Activity: json.RawMessage(`{}`), ActivityType: "delete", SenderID: acc.ID}),
		numDelivered: natsutil.MaxDeliverActivityPubOutboundFanout,
		ackFn:        func() { acked = true },
	}
	deliveryMock := &mockDeliveryPublisher{
		publishFn: func(context.Context, string, outboxDeliveryMessage) error { return errors.New("nats publish failed") },
	}

	w := &outboxFanoutWorker{store: fake, delivery: deliveryMock, dlqPublisher: dlqMock, cfg: cfg}
	w.processMessage(ctx, mockMsg)

	assert.True(t, acked)
	assert.Equal(t, natsutil.SubjectPrefixActivityPubOutboundFanoutDLQ+"delete", dlqSubject)
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return data
}

type mockFanoutDLQPublisher struct {
	publishFn func(ctx context.Context, subject string, payload []byte, opts ...jetstream.PublishOpt) (*jetstream.PubAck, error)
}

func (m *mockFanoutDLQPublisher) Publish(ctx context.Context, subject string, payload []byte, opts ...jetstream.PublishOpt) (*jetstream.PubAck, error) {
	if m.publishFn != nil {
		return m.publishFn(ctx, subject, payload, opts...)
	}
	return nil, nil
}

type mockJetstreamMsg struct {
	data           []byte
	ackFn          func()
	nakFn          func()
	numDelivered   uint64
	nakWithDelayFn func(d time.Duration)
}

func (m *mockJetstreamMsg) Data() []byte         { return m.data }
func (m *mockJetstreamMsg) Headers() nats.Header { return nil }
func (m *mockJetstreamMsg) Subject() string      { return "" }
func (m *mockJetstreamMsg) Reply() string        { return "" }
func (m *mockJetstreamMsg) Ack() error {
	if m.ackFn != nil {
		m.ackFn()
	}
	return nil
}
func (m *mockJetstreamMsg) DoubleAck(context.Context) error { return nil }
func (m *mockJetstreamMsg) Nak() error {
	if m.nakFn != nil {
		m.nakFn()
	}
	return nil
}
func (m *mockJetstreamMsg) NakWithDelay(d time.Duration) error {
	if m.nakWithDelayFn != nil {
		m.nakWithDelayFn(d)
	}
	return nil
}
func (m *mockJetstreamMsg) InProgress() error           { return nil }
func (m *mockJetstreamMsg) Term() error                 { return nil }
func (m *mockJetstreamMsg) TermWithReason(string) error { return nil }
func (m *mockJetstreamMsg) Metadata() (*jetstream.MsgMetadata, error) {
	return &jetstream.MsgMetadata{NumDelivered: m.numDelivered}, nil
}
