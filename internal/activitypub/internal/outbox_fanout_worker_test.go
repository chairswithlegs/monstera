package internal

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
)

const testRemoteDomain = "remote.example"

type mockDeliveryPublisher struct {
	publishFn func(ctx context.Context, activityType string, msg OutboxDeliveryMessage) error
	startFn   func(ctx context.Context) error
}

func (m *mockDeliveryPublisher) Publish(ctx context.Context, activityType string, msg OutboxDeliveryMessage) error {
	if m.publishFn != nil {
		return m.publishFn(ctx, activityType, msg)
	}
	return nil
}

func (m *mockDeliveryPublisher) Start(ctx context.Context) error {
	if m.startFn != nil {
		return m.startFn(ctx)
	}
	return nil
}

func TestOutboxFanoutWorker_ProcessMessage_PublishesDeliveryPerInbox(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	followSvc := service.NewRemoteFollowService(fake)

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

	var delivered []OutboxDeliveryMessage
	deliveryMock := &mockDeliveryPublisher{
		publishFn: func(_ context.Context, _ string, msg OutboxDeliveryMessage) error {
			delivered = append(delivered, msg)
			return nil
		},
	}

	fanoutMsg := OutboxFanoutMessage{
		ActivityID: "https://example.com/activities/01act",
		Activity:   json.RawMessage(`{"type":"Create","object":{"type":"Note"}}`),
		SenderID:   acc.ID,
	}
	data, err := json.Marshal(fanoutMsg)
	require.NoError(t, err)

	w := &outboxFanoutWorker{followers: followSvc, delivery: deliveryMock, workerConcurrency: 0}
	w.processMessage(ctx, &testutil.MockJetstreamMsg{DataBytes: data})

	require.Len(t, delivered, 1)
	assert.Equal(t, "https://example.com/activities/01act", delivered[0].ActivityID)
	assert.Equal(t, "https://"+testRemoteDomain+"/inbox", delivered[0].TargetInbox)
	assert.Equal(t, acc.ID, delivered[0].SenderID)
	assert.Equal(t, string(fanoutMsg.Activity), string(delivered[0].Activity))
}

func TestOutboxFanoutWorker_ProcessMessage_Pagination(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	followSvc := service.NewRemoteFollowService(fake)

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

	var delivered []OutboxDeliveryMessage
	deliveryMock := &mockDeliveryPublisher{
		publishFn: func(_ context.Context, _ string, msg OutboxDeliveryMessage) error {
			delivered = append(delivered, msg)
			return nil
		},
	}

	fanoutMsg := OutboxFanoutMessage{
		ActivityID: "https://example.com/activities/01act",
		Activity:   json.RawMessage(`{"type":"Create"}`),
		SenderID:   acc.ID,
	}
	data, err := json.Marshal(fanoutMsg)
	require.NoError(t, err)

	w := &outboxFanoutWorker{followers: followSvc, delivery: deliveryMock, workerConcurrency: 0}
	w.processMessage(ctx, &testutil.MockJetstreamMsg{DataBytes: data})

	require.Len(t, delivered, 3)
	inboxes := make(map[string]bool)
	for _, d := range delivered {
		inboxes[d.TargetInbox] = true
	}
	assert.Len(t, inboxes, 3, "each follower inbox should appear exactly once")
}

func TestOutboxFanoutWorker_ProcessMessage_EmptyFollowers_Acks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	followSvc := service.NewRemoteFollowService(fake)

	acc, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01author", Username: "alice", APID: "https://example.com/users/alice",
	})
	require.NoError(t, err)

	var delivered []OutboxDeliveryMessage
	deliveryMock := &mockDeliveryPublisher{
		publishFn: func(_ context.Context, _ string, msg OutboxDeliveryMessage) error {
			delivered = append(delivered, msg)
			return nil
		},
	}

	fanoutMsg := OutboxFanoutMessage{
		ActivityID: "https://example.com/activities/01act",
		Activity:   json.RawMessage(`{"type":"Delete"}`),
		SenderID:   acc.ID,
	}
	data, err := json.Marshal(fanoutMsg)
	require.NoError(t, err)

	acked := false
	mockMsg := &testutil.MockJetstreamMsg{DataBytes: data, AckFn: func() { acked = true }}

	w := &outboxFanoutWorker{followers: followSvc, delivery: deliveryMock, workerConcurrency: 0}
	w.processMessage(ctx, mockMsg)

	assert.Empty(t, delivered)
	assert.True(t, acked)
}

type failingFollowSvc struct {
	service.RemoteFollowService
	err error
}

func (f *failingFollowSvc) GetFollowerInboxURLsPaginated(_ context.Context, _ string, _ string, _ int) ([]string, error) {
	return nil, f.err
}

func TestOutboxFanoutWorker_ProcessMessage_DBError_RetriesRemaining_NakWithDelay(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	followSvc := service.NewRemoteFollowService(fake)
	failingFollowSvc := &failingFollowSvc{RemoteFollowService: followSvc, err: errors.New("db error")}

	acc, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01author", Username: "alice", APID: "https://example.com/users/alice",
	})
	require.NoError(t, err)

	nakWithDelayCalled := false
	var nakDelay time.Duration
	mockMsg := &testutil.MockJetstreamMsg{
		DataBytes:        mustMarshal(t, OutboxFanoutMessage{ActivityID: "https://example.com/01act", Activity: json.RawMessage(`{}`), SenderID: acc.ID}),
		NumDeliveredUInt: 0,
		NakWithDelayFn: func(d time.Duration) {
			nakWithDelayCalled = true
			nakDelay = d
		},
	}
	deliveryMock := &mockDeliveryPublisher{publishFn: func(context.Context, string, OutboxDeliveryMessage) error { return nil }}

	w := &outboxFanoutWorker{followers: failingFollowSvc, delivery: deliveryMock, workerConcurrency: 0}
	w.processMessage(ctx, mockMsg)

	assert.True(t, nakWithDelayCalled)
	assert.Equal(t, fanoutRetries[0], nakDelay)
}

func TestOutboxFanoutWorker_ProcessMessage_DBError_RetryExhausted_SendsToDLQ(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	followSvc := service.NewRemoteFollowService(fake)
	failingFollowSvc := &failingFollowSvc{RemoteFollowService: followSvc, err: errors.New("db error")}

	acc, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01author", Username: "alice", APID: "https://example.com/users/alice",
	})
	require.NoError(t, err)

	jsMock := &mockJetStreamPublisher{}
	acked := false
	mockMsg := &testutil.MockJetstreamMsg{
		DataBytes:        mustMarshal(t, OutboxFanoutMessage{ActivityID: "https://example.com/01act", Activity: json.RawMessage(`{}`), SenderID: acc.ID}),
		SubjectString:    subjectPrefixFanout + "create",
		NumDeliveredUInt: uint64(len(fanoutRetries)),
		AckFn:            func() { acked = true },
	}
	deliveryMock := &mockDeliveryPublisher{publishFn: func(context.Context, string, OutboxDeliveryMessage) error { return nil }}

	w := &outboxFanoutWorker{js: jsMock, followers: failingFollowSvc, delivery: deliveryMock, workerConcurrency: 0}
	w.processMessage(ctx, mockMsg)

	assert.True(t, acked)
	require.Len(t, jsMock.published, 1)
	assert.Equal(t, subjectPrefixFanoutDLQ+"create", jsMock.published[0].subject)
	assert.NotEmpty(t, jsMock.published[0].data)
}

func TestOutboxFanoutWorker_ProcessMessage_PublishError_RetriesRemaining_NakWithDelay(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	followSvc := service.NewRemoteFollowService(fake)

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
	mockMsg := &testutil.MockJetstreamMsg{
		DataBytes: mustMarshal(t, OutboxFanoutMessage{
			ActivityID: "https://example.com/01act", Activity: json.RawMessage(`{}`), SenderID: acc.ID,
		}),
		NumDeliveredUInt: 0,
		NakWithDelayFn:   func(time.Duration) { nakWithDelayCalled = true },
	}
	deliveryMock := &mockDeliveryPublisher{
		publishFn: func(context.Context, string, OutboxDeliveryMessage) error { return errors.New("nats publish failed") },
	}

	w := &outboxFanoutWorker{followers: followSvc, delivery: deliveryMock, workerConcurrency: 0}
	w.processMessage(ctx, mockMsg)

	assert.True(t, nakWithDelayCalled)
}

func TestOutboxFanoutWorker_ProcessMessage_PublishError_RetryExhausted_SendsToDLQ(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	followSvc := service.NewRemoteFollowService(fake)

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

	jsMock := &mockJetStreamPublisher{}
	acked := false
	mockMsg := &testutil.MockJetstreamMsg{
		DataBytes:        mustMarshal(t, OutboxFanoutMessage{ActivityID: "https://example.com/01act", Activity: json.RawMessage(`{}`), SenderID: acc.ID}),
		SubjectString:    subjectPrefixFanout + "delete",
		NumDeliveredUInt: uint64(len(fanoutRetries)),
		AckFn:            func() { acked = true },
	}
	deliveryMock := &mockDeliveryPublisher{
		publishFn: func(context.Context, string, OutboxDeliveryMessage) error { return errors.New("nats publish failed") },
	}

	w := &outboxFanoutWorker{js: jsMock, followers: followSvc, delivery: deliveryMock, workerConcurrency: 0}
	w.processMessage(ctx, mockMsg)

	assert.True(t, acked)
	require.Len(t, jsMock.published, 1)
	assert.Equal(t, subjectPrefixFanoutDLQ+"delete", jsMock.published[0].subject)
}

func TestOutboxFanoutWorker_getActivityType(t *testing.T) {
	t.Parallel()
	w := &outboxFanoutWorker{}
	assert.Equal(t, "create", w.getActivityType(subjectPrefixFanout+"create"))
	assert.Equal(t, "delete", w.getActivityType(subjectPrefixFanout+"delete"))
	assert.Equal(t, "unknown", w.getActivityType("unknown"))
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return data
}

type publishedMsg struct {
	subject string
	data    []byte
}

// mockJetStreamPublisher implements jetstream.JetStream with only Publish for test use.
type mockJetStreamPublisher struct {
	jetstream.JetStream
	published []publishedMsg
}

func (m *mockJetStreamPublisher) Publish(_ context.Context, subject string, payload []byte, _ ...jetstream.PublishOpt) (*jetstream.PubAck, error) {
	m.published = append(m.published, publishedMsg{subject: subject, data: payload})
	return &jetstream.PubAck{}, nil
}
