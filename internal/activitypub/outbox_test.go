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

func TestOutboxPublisher_SendAcceptFollow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	cfg := &config.Config{InstanceDomain: "example.com"}
	var enqueued []struct {
		ActivityType string
		Msg          DeliveryMessage
	}
	enqueuer := &fakeDeliveryEnqueuer{
		enqueue: func(_ context.Context, activityType string, msg DeliveryMessage) error {
			enqueued = append(enqueued, struct {
				ActivityType string
				Msg          DeliveryMessage
			}{activityType, msg})
			return nil
		},
	}
	pub := NewOutboxPublisher(fake, enqueuer, cfg)

	target, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01local", Username: "alice", APID: "https://example.com/users/alice",
	})
	require.NoError(t, err)
	remoteDomain := "remote.example"
	actor, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01remote", Username: "bob", Domain: &remoteDomain,
		APID: "https://remote.example/users/bob", InboxURL: "https://remote.example/inbox",
	})
	require.NoError(t, err)

	err = pub.SendAcceptFollow(ctx, target, actor, "01follow-ulid")
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

type fakeDeliveryEnqueuer struct {
	enqueue func(context.Context, string, DeliveryMessage) error
}

func (f *fakeDeliveryEnqueuer) EnqueueDelivery(ctx context.Context, activityType string, msg DeliveryMessage) error {
	if f.enqueue != nil {
		return f.enqueue(ctx, activityType, msg)
	}
	return nil
}
