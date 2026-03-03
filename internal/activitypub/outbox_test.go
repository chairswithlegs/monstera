package activitypub

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera-fed/internal/config"
	"github.com/chairswithlegs/monstera-fed/internal/store"
	"github.com/chairswithlegs/monstera-fed/internal/testutil"
)

const testRemoteDomain = "remote.example"

func TestOutbox_SendAcceptFollow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	cfg := &config.Config{InstanceDomain: "example.com"}
	var enqueued []struct {
		ActivityType string
		Msg          outboxDeliveryMessage
	}
	delivery := &mockDeliveryPublisher{
		publishFn: func(_ context.Context, activityType string, msg outboxDeliveryMessage) error {
			enqueued = append(enqueued, struct {
				ActivityType string
				Msg          outboxDeliveryMessage
			}{ActivityType: activityType, Msg: msg})
			return nil
		},
	}
	fanout := &mockOutboxFanoutPublisher{publishFn: func(_ context.Context, _ outboxFanoutMessage) error { return nil }}
	outbox := &Outbox{store: fake, delivery: delivery, fanout: fanout, cfg: cfg}

	target, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01local", Username: "alice", APID: "https://example.com/users/alice",
	})
	require.NoError(t, err)
	remoteDomain := testRemoteDomain
	actor, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01remote", Username: "bob", Domain: &remoteDomain,
		APID: "https://remote.example/users/bob", InboxURL: "https://remote.example/inbox",
	})
	require.NoError(t, err)

	err = outbox.SendAcceptFollow(ctx, target, actor, "01follow-ulid")
	require.NoError(t, err)

	require.Len(t, enqueued, 1)
	assert.Equal(t, "accept", enqueued[0].ActivityType)
	assert.Equal(t, "https://remote.example/inbox", enqueued[0].Msg.TargetInbox)
	assert.Equal(t, target.ID, enqueued[0].Msg.SenderID)
	assert.Contains(t, enqueued[0].Msg.ActivityID, "accept-")
	var act map[string]any
	require.NoError(t, json.Unmarshal(enqueued[0].Msg.Activity, &act))
	assert.Equal(t, "Accept", act["type"])
	assert.Equal(t, "https://example.com/users/alice", act["actor"])
}

func TestOutbox_PublishStatus_EnqueuesFanoutMessage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	cfg := &config.Config{InstanceDomain: "example.com"}
	var fanoutMsgs []outboxFanoutMessage
	fanout := &mockOutboxFanoutPublisher{
		publishFn: func(_ context.Context, msg outboxFanoutMessage) error {
			fanoutMsgs = append(fanoutMsgs, msg)
			return nil
		},
	}
	delivery := &mockDeliveryPublisher{publishFn: func(context.Context, string, outboxDeliveryMessage) error { return nil }}
	outbox := &Outbox{store: fake, delivery: delivery, fanout: fanout, cfg: cfg}

	acc, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01author", Username: "alice", APID: "https://example.com/users/alice",
	})
	require.NoError(t, err)
	st, err := fake.CreateStatus(ctx, store.CreateStatusInput{
		ID: "01status", AccountID: acc.ID, Content: ptr("hello"),
	})
	require.NoError(t, err)

	err = outbox.PublishStatus(ctx, st)
	require.NoError(t, err)

	require.Len(t, fanoutMsgs, 1)
	assert.Equal(t, "create", fanoutMsgs[0].ActivityType)
	assert.Equal(t, acc.ID, fanoutMsgs[0].SenderID)
	assert.NotEmpty(t, fanoutMsgs[0].ActivityID)
	assert.NotEmpty(t, fanoutMsgs[0].Activity)
	var act map[string]any
	require.NoError(t, json.Unmarshal(fanoutMsgs[0].Activity, &act))
	assert.Equal(t, "Create", act["type"])
}

func TestOutbox_DeleteStatus_EnqueuesFanoutMessage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	cfg := &config.Config{InstanceDomain: "example.com"}
	var fanoutMsgs []outboxFanoutMessage
	fanout := &mockOutboxFanoutPublisher{
		publishFn: func(_ context.Context, msg outboxFanoutMessage) error {
			fanoutMsgs = append(fanoutMsgs, msg)
			return nil
		},
	}
	delivery := &mockDeliveryPublisher{publishFn: func(context.Context, string, outboxDeliveryMessage) error { return nil }}
	outbox := &Outbox{store: fake, delivery: delivery, fanout: fanout, cfg: cfg}

	acc, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01author", Username: "alice", APID: "https://example.com/users/alice",
	})
	require.NoError(t, err)
	st, err := fake.CreateStatus(ctx, store.CreateStatusInput{
		ID: "01status", AccountID: acc.ID, Content: ptr("hello"),
	})
	require.NoError(t, err)

	err = outbox.DeleteStatus(ctx, st)
	require.NoError(t, err)

	require.Len(t, fanoutMsgs, 1)
	assert.Equal(t, "delete", fanoutMsgs[0].ActivityType)
	assert.Equal(t, acc.ID, fanoutMsgs[0].SenderID)
	assert.Contains(t, fanoutMsgs[0].ActivityID, "#delete")
	assert.NotEmpty(t, fanoutMsgs[0].Activity)
	var act map[string]any
	require.NoError(t, json.Unmarshal(fanoutMsgs[0].Activity, &act))
	assert.Equal(t, "Delete", act["type"])
}

func TestOutbox_PublishFollow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	cfg := &config.Config{InstanceDomain: "example.com"}
	var enqueued []struct {
		ActivityType string
		Msg          outboxDeliveryMessage
	}
	delivery := &mockDeliveryPublisher{
		publishFn: func(_ context.Context, activityType string, msg outboxDeliveryMessage) error {
			enqueued = append(enqueued, struct {
				ActivityType string
				Msg          outboxDeliveryMessage
			}{ActivityType: activityType, Msg: msg})
			return nil
		},
	}
	fanout := &mockOutboxFanoutPublisher{publishFn: func(_ context.Context, _ outboxFanoutMessage) error { return nil }}
	outbox := &Outbox{store: fake, delivery: delivery, fanout: fanout, cfg: cfg}

	actor, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01actor", Username: "alice", APID: "https://example.com/users/alice",
	})
	require.NoError(t, err)
	remoteDomain := testRemoteDomain
	target, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01target", Username: "bob", Domain: &remoteDomain,
		APID: "https://remote.example/users/bob", InboxURL: "https://remote.example/inbox",
	})
	require.NoError(t, err)

	err = outbox.PublishFollow(ctx, actor, target, "01follow-ulid")
	require.NoError(t, err)

	require.Len(t, enqueued, 1)
	assert.Equal(t, "follow", enqueued[0].ActivityType)
	assert.Equal(t, "https://remote.example/inbox", enqueued[0].Msg.TargetInbox)
	assert.Equal(t, actor.ID, enqueued[0].Msg.SenderID)
	assert.Equal(t, "https://example.com/activities/01follow-ulid", enqueued[0].Msg.ActivityID)
	var act map[string]any
	require.NoError(t, json.Unmarshal(enqueued[0].Msg.Activity, &act))
	assert.Equal(t, "Follow", act["type"])
	assert.Equal(t, "https://example.com/users/alice", act["actor"])
	assert.Equal(t, "https://remote.example/users/bob", act["object"])
}

func TestOutbox_PublishFollow_EmptyInbox(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	cfg := &config.Config{InstanceDomain: "example.com"}
	delivery := &mockDeliveryPublisher{publishFn: func(context.Context, string, outboxDeliveryMessage) error {
		t.Error("publish should not be called when target has no inbox")
		return nil
	}}
	fanout := &mockOutboxFanoutPublisher{publishFn: func(context.Context, outboxFanoutMessage) error { return nil }}
	outbox := &Outbox{store: fake, delivery: delivery, fanout: fanout, cfg: cfg}

	actor, _ := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01actor", Username: "alice", APID: "https://example.com/users/alice",
	})
	target, _ := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01target", Username: "bob", Domain: ptr("remote.example"),
		APID: "https://remote.example/users/bob", InboxURL: "",
	})

	err := outbox.PublishFollow(ctx, actor, target, "01follow-ulid")
	require.NoError(t, err)
}

func TestOutbox_PublishUndoFollow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	cfg := &config.Config{InstanceDomain: "example.com"}
	var enqueued []struct {
		ActivityType string
		Msg          outboxDeliveryMessage
	}
	delivery := &mockDeliveryPublisher{
		publishFn: func(_ context.Context, activityType string, msg outboxDeliveryMessage) error {
			enqueued = append(enqueued, struct {
				ActivityType string
				Msg          outboxDeliveryMessage
			}{ActivityType: activityType, Msg: msg})
			return nil
		},
	}
	fanout := &mockOutboxFanoutPublisher{publishFn: func(_ context.Context, _ outboxFanoutMessage) error { return nil }}
	outbox := &Outbox{store: fake, delivery: delivery, fanout: fanout, cfg: cfg}

	actor, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01actor", Username: "alice", APID: "https://example.com/users/alice",
	})
	require.NoError(t, err)
	remoteDomain := testRemoteDomain
	target, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01target", Username: "bob", Domain: &remoteDomain,
		APID: "https://remote.example/users/bob", InboxURL: "https://remote.example/inbox",
	})
	require.NoError(t, err)

	err = outbox.PublishUndoFollow(ctx, actor, target, "01follow-ulid")
	require.NoError(t, err)

	require.Len(t, enqueued, 1)
	assert.Equal(t, "undo", enqueued[0].ActivityType)
	assert.Equal(t, "https://remote.example/inbox", enqueued[0].Msg.TargetInbox)
	assert.Contains(t, enqueued[0].Msg.ActivityID, "undo-")
	var act map[string]any
	require.NoError(t, json.Unmarshal(enqueued[0].Msg.Activity, &act))
	assert.Equal(t, "Undo", act["type"])
	inner, ok := act["object"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Follow", inner["type"])
}

func TestOutbox_PublishUndoFollow_EmptyInbox(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	cfg := &config.Config{InstanceDomain: "example.com"}
	delivery := &mockDeliveryPublisher{publishFn: func(context.Context, string, outboxDeliveryMessage) error {
		t.Error("publish should not be called when target has no inbox")
		return nil
	}}
	fanout := &mockOutboxFanoutPublisher{publishFn: func(context.Context, outboxFanoutMessage) error { return nil }}
	outbox := &Outbox{store: fake, delivery: delivery, fanout: fanout, cfg: cfg}

	actor, _ := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01actor", Username: "alice", APID: "https://example.com/users/alice",
	})
	target, _ := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01target", Username: "bob", Domain: ptr("remote.example"),
		APID: "https://remote.example/users/bob", InboxURL: "",
	})

	err := outbox.PublishUndoFollow(ctx, actor, target, "01follow-ulid")
	require.NoError(t, err)
}

func TestOutbox_SendAcceptFollow_EmptyInbox(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	cfg := &config.Config{InstanceDomain: "example.com"}
	delivery := &mockDeliveryPublisher{publishFn: func(context.Context, string, outboxDeliveryMessage) error {
		t.Error("publish should not be called when actor has no inbox")
		return nil
	}}
	fanout := &mockOutboxFanoutPublisher{publishFn: func(context.Context, outboxFanoutMessage) error { return nil }}
	outbox := &Outbox{store: fake, delivery: delivery, fanout: fanout, cfg: cfg}

	target, _ := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01local", Username: "alice", APID: "https://example.com/users/alice",
	})
	actor, _ := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01remote", Username: "bob", Domain: ptr("remote.example"),
		APID: "https://remote.example/users/bob", InboxURL: "",
	})

	err := outbox.SendAcceptFollow(ctx, target, actor, "01follow-ulid")
	require.NoError(t, err)
}

func ptr(s string) *string { return &s }

type mockDeliveryPublisher struct {
	publishFn func(context.Context, string, outboxDeliveryMessage) error
}

func (m *mockDeliveryPublisher) publish(ctx context.Context, activityType string, msg outboxDeliveryMessage) error {
	if m.publishFn != nil {
		return m.publishFn(ctx, activityType, msg)
	}
	return nil
}

type mockOutboxFanoutPublisher struct {
	publishFn func(context.Context, outboxFanoutMessage) error
}

func (m *mockOutboxFanoutPublisher) publish(ctx context.Context, msg outboxFanoutMessage) error {
	if m.publishFn != nil {
		return m.publishFn(ctx, msg)
	}
	return nil
}
