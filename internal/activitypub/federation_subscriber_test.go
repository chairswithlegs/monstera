package activitypub

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/activitypub/internal"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/testutil"
)

type mockDeliveryWorker struct {
	publishFn func(context.Context, string, internal.OutboxDeliveryMessage) error
	startFn   func(context.Context) error
}

func (m *mockDeliveryWorker) Publish(ctx context.Context, activityType string, msg internal.OutboxDeliveryMessage) error {
	if m.publishFn != nil {
		return m.publishFn(ctx, activityType, msg)
	}
	return nil
}

func (m *mockDeliveryWorker) Start(ctx context.Context) error {
	if m.startFn != nil {
		return m.startFn(ctx)
	}
	return nil
}

type mockFanoutWorker struct {
	publishFn func(ctx context.Context, activityType string, msg internal.OutboxFanoutMessage) error
	startFn   func(context.Context) error
}

func (m *mockFanoutWorker) Publish(ctx context.Context, activityType string, msg internal.OutboxFanoutMessage) error {
	if m.publishFn != nil {
		return m.publishFn(ctx, activityType, msg)
	}
	return nil
}

func (m *mockFanoutWorker) Start(ctx context.Context) error {
	if m.startFn != nil {
		return m.startFn(ctx)
	}
	return nil
}

// newTestSubscriber builds a FederationSubscriber wired to the given mocks.
func newTestSubscriber(deliveryWorker mockDeliveryWorker, fanoutWorker mockFanoutWorker) *FederationSubscriber {
	return &FederationSubscriber{
		delivery:        &deliveryWorker,
		fanout:          &fanoutWorker,
		instanceBaseURL: "https://example.com",
	}
}

// makeMsg encodes a DomainEvent with the given type and payload into a mock NATS message.
func makeMsg(t *testing.T, eventType string, payload any) *testutil.MockJetstreamMsg {
	t.Helper()
	payloadJSON, err := json.Marshal(payload)
	require.NoError(t, err)
	ev := domain.DomainEvent{ID: "ev01", EventType: eventType, Payload: payloadJSON}
	data, err := json.Marshal(ev)
	require.NoError(t, err)
	return &testutil.MockJetstreamMsg{DataBytes: data}
}

// localAccount returns a minimal local domain.Account.
func localAccount(id string) *domain.Account {
	return &domain.Account{
		ID:       id,
		Username: "alice",
		APID:     "https://example.com/users/alice",
	}
}

// remoteAccount returns a minimal remote domain.Account with an inbox URL on remote.example.
func remoteAccount(id string) *domain.Account {
	d := "remote.example"
	return &domain.Account{
		ID:       id,
		Username: "bob",
		Domain:   &d,
		APID:     "https://remote.example/users/bob",
		InboxURL: "https://remote.example/inbox",
	}
}

func TestFederationSubscriber_processMessage_InvalidJSON_Acks(t *testing.T) {
	t.Parallel()
	s := newTestSubscriber(mockDeliveryWorker{}, mockFanoutWorker{})
	acked := false
	msg := &testutil.MockJetstreamMsg{DataBytes: []byte("not json"), AckFn: func() { acked = true }}
	s.processMessage(context.Background(), msg)
	assert.True(t, acked)
}

func TestFederationSubscriber_processMessage_SSEOnlyEvent_AcksWithoutPublish(t *testing.T) {
	t.Parallel()
	sseEvents := []string{
		domain.EventStatusCreatedRemote,
		domain.EventStatusDeletedRemote,
		domain.EventNotificationCreated,
	}
	for _, evType := range sseEvents {
		evType := evType
		t.Run(evType, func(t *testing.T) {
			t.Parallel()
			published := false
			fanout := mockFanoutWorker{publishFn: func(context.Context, string, internal.OutboxFanoutMessage) error {
				published = true
				return nil
			}}
			delivery := mockDeliveryWorker{publishFn: func(context.Context, string, internal.OutboxDeliveryMessage) error {
				published = true
				return nil
			}}
			s := newTestSubscriber(delivery, fanout)
			acked := false
			msg := makeMsg(t, evType, map[string]any{})
			msg.AckFn = func() { acked = true }
			s.processMessage(context.Background(), msg)
			assert.True(t, acked)
			assert.False(t, published)
		})
	}
}

func TestFederationSubscriber_processMessage_UnknownEventType_AcksWithoutPublish(t *testing.T) {
	t.Parallel()
	published := false
	fanout := mockFanoutWorker{publishFn: func(context.Context, string, internal.OutboxFanoutMessage) error {
		published = true
		return nil
	}}
	s := newTestSubscriber(mockDeliveryWorker{}, fanout)
	acked := false
	msg := makeMsg(t, "unknown.event.type", map[string]any{})
	msg.AckFn = func() { acked = true }
	s.processMessage(context.Background(), msg)
	assert.True(t, acked)
	assert.False(t, published)
}

func TestFederationSubscriber_processMessage_HandlerError_Naks(t *testing.T) {
	t.Parallel()
	fanout := mockFanoutWorker{publishFn: func(context.Context, string, internal.OutboxFanoutMessage) error {
		return errors.New("nats down")
	}}
	s := newTestSubscriber(mockDeliveryWorker{}, fanout)
	naked := false
	author := localAccount("01author")
	status := &domain.Status{ID: "01status", APID: "https://example.com/statuses/01status"}
	msg := makeMsg(t, domain.EventStatusCreated, domain.StatusCreatedPayload{Status: status, Author: author, Local: true})
	msg.NakFn = func() { naked = true }
	s.processMessage(context.Background(), msg)
	assert.True(t, naked)
}

func TestFederationSubscriber_processMessage_HandlerSuccess_Acks(t *testing.T) {
	t.Parallel()
	fanout := mockFanoutWorker{}
	s := newTestSubscriber(mockDeliveryWorker{}, fanout)
	acked := false
	author := localAccount("01author")
	status := &domain.Status{ID: "01status", APID: "https://example.com/statuses/01status"}
	msg := makeMsg(t, domain.EventStatusCreated, domain.StatusCreatedPayload{Status: status, Author: author, Local: true})
	msg.AckFn = func() { acked = true }
	s.processMessage(context.Background(), msg)
	assert.True(t, acked)
}

func TestFederationSubscriber_handleStatusCreated_PublishesFanout(t *testing.T) {
	t.Parallel()
	var got internal.OutboxFanoutMessage
	var gotType string
	fanout := mockFanoutWorker{publishFn: func(_ context.Context, actType string, msg internal.OutboxFanoutMessage) error {
		gotType = actType
		got = msg
		return nil
	}}
	s := newTestSubscriber(mockDeliveryWorker{}, fanout)

	author := localAccount("01author")
	status := &domain.Status{ID: "01status", APID: "https://example.com/statuses/01status"}
	payload := domain.StatusCreatedPayload{Status: status, Author: author, Local: true}

	err := s.handleStatusCreated(context.Background(), domainEvent(t, domain.EventStatusCreated, payload))
	require.NoError(t, err)
	assert.Equal(t, "create", gotType)
	assert.Equal(t, status.APID, got.ActivityID)
	assert.Equal(t, author.ID, got.SenderID)
	assert.NotEmpty(t, got.Activity)
}

func TestFederationSubscriber_handleStatusCreated_FallsBackToGeneratedID(t *testing.T) {
	t.Parallel()
	var got internal.OutboxFanoutMessage
	fanout := mockFanoutWorker{publishFn: func(_ context.Context, _ string, msg internal.OutboxFanoutMessage) error {
		got = msg
		return nil
	}}
	s := newTestSubscriber(mockDeliveryWorker{}, fanout)

	author := localAccount("01author")
	// No APID or URI set — subscriber should generate a URL
	status := &domain.Status{ID: "01status"}
	payload := domain.StatusCreatedPayload{Status: status, Author: author, Local: true}

	err := s.handleStatusCreated(context.Background(), domainEvent(t, domain.EventStatusCreated, payload))
	require.NoError(t, err)
	assert.Contains(t, got.ActivityID, "https://example.com/activities/")
}

func TestFederationSubscriber_handleStatusCreated_RemoteAuthor_Skips(t *testing.T) {
	t.Parallel()
	published := false
	fanout := mockFanoutWorker{publishFn: func(context.Context, string, internal.OutboxFanoutMessage) error {
		published = true
		return nil
	}}
	s := newTestSubscriber(mockDeliveryWorker{}, fanout)

	author := remoteAccount("01author")
	status := &domain.Status{ID: "01status", APID: "https://example.com/statuses/01status"}
	payload := domain.StatusCreatedPayload{Status: status, Author: author}

	err := s.handleStatusCreated(context.Background(), domainEvent(t, domain.EventStatusCreated, payload))
	require.NoError(t, err)
	assert.False(t, published)
}

// --- handleStatusDeleted ---

func TestFederationSubscriber_handleStatusDeleted_NonLocal_Skips(t *testing.T) {
	t.Parallel()
	published := false
	fanout := mockFanoutWorker{publishFn: func(context.Context, string, internal.OutboxFanoutMessage) error {
		published = true
		return nil
	}}
	s := newTestSubscriber(mockDeliveryWorker{}, fanout)

	author := localAccount("01author")
	payload := domain.StatusDeletedPayload{
		StatusID:  "01status",
		AccountID: author.ID,
		Author:    author,
		Local:     false,
	}
	err := s.handleStatusDeleted(context.Background(), domainEvent(t, domain.EventStatusDeleted, payload))
	require.NoError(t, err)
	assert.False(t, published)
}

func TestFederationSubscriber_handleStatusDeleted_Local_PublishesFanout(t *testing.T) {
	t.Parallel()
	var got internal.OutboxFanoutMessage
	var gotType string
	fanout := mockFanoutWorker{publishFn: func(_ context.Context, actType string, msg internal.OutboxFanoutMessage) error {
		gotType = actType
		got = msg
		return nil
	}}
	s := newTestSubscriber(mockDeliveryWorker{}, fanout)

	author := localAccount("01author")
	apID := "https://example.com/statuses/01status"
	payload := domain.StatusDeletedPayload{
		StatusID:  "01status",
		AccountID: author.ID,
		Author:    author,
		Local:     true,
		APID:      apID,
	}
	err := s.handleStatusDeleted(context.Background(), domainEvent(t, domain.EventStatusDeleted, payload))
	require.NoError(t, err)
	assert.Equal(t, "delete", gotType)
	assert.Equal(t, apID+"#delete", got.ActivityID)
	assert.Equal(t, author.ID, got.SenderID)
}

func TestFederationSubscriber_handleStatusDeleted_Local_FallsBackToGeneratedID(t *testing.T) {
	t.Parallel()
	var got internal.OutboxFanoutMessage
	fanout := mockFanoutWorker{publishFn: func(_ context.Context, _ string, msg internal.OutboxFanoutMessage) error {
		got = msg
		return nil
	}}
	s := newTestSubscriber(mockDeliveryWorker{}, fanout)

	author := localAccount("01author")
	payload := domain.StatusDeletedPayload{
		StatusID:  "01status",
		AccountID: author.ID,
		Author:    author,
		Local:     true,
		// No APID or URI
	}
	err := s.handleStatusDeleted(context.Background(), domainEvent(t, domain.EventStatusDeleted, payload))
	require.NoError(t, err)
	assert.Contains(t, got.ActivityID, "https://example.com/statuses/01status#delete")
}

// --- handleStatusUpdated ---

func TestFederationSubscriber_handleStatusUpdated_PublishesFanout(t *testing.T) {
	t.Parallel()
	var got internal.OutboxFanoutMessage
	var gotType string
	fanout := mockFanoutWorker{publishFn: func(_ context.Context, actType string, msg internal.OutboxFanoutMessage) error {
		gotType = actType
		got = msg
		return nil
	}}
	s := newTestSubscriber(mockDeliveryWorker{}, fanout)

	author := localAccount("01author")
	status := &domain.Status{ID: "01status", APID: "https://example.com/statuses/01status"}
	payload := domain.StatusUpdatedPayload{Status: status, Author: author, Local: true}

	err := s.handleStatusUpdated(context.Background(), domainEvent(t, domain.EventStatusUpdated, payload))
	require.NoError(t, err)
	assert.Equal(t, "update", gotType)
	assert.Equal(t, status.APID+"#update", got.ActivityID)
	assert.Equal(t, author.ID, got.SenderID)
}

func TestFederationSubscriber_handleStatusUpdated_RemoteAuthor_Skips(t *testing.T) {
	t.Parallel()
	published := false
	fanout := mockFanoutWorker{publishFn: func(context.Context, string, internal.OutboxFanoutMessage) error {
		published = true
		return nil
	}}
	s := newTestSubscriber(mockDeliveryWorker{}, fanout)

	author := remoteAccount("01author")
	status := &domain.Status{ID: "01status", APID: "https://example.com/statuses/01status"}
	payload := domain.StatusUpdatedPayload{Status: status, Author: author}

	err := s.handleStatusUpdated(context.Background(), domainEvent(t, domain.EventStatusUpdated, payload))
	require.NoError(t, err)
	assert.False(t, published)
}

func TestFederationSubscriber_handleFollowCreated_RemoteActor_Skips(t *testing.T) {
	t.Parallel()
	published := false
	delivery := mockDeliveryWorker{publishFn: func(context.Context, string, internal.OutboxDeliveryMessage) error {
		published = true
		return nil
	}}
	s := newTestSubscriber(delivery, mockFanoutWorker{})

	actor := remoteAccount("01actor")
	target := remoteAccount("01target")
	payload := domain.FollowCreatedPayload{
		Follow: &domain.Follow{ID: "01follow"},
		Actor:  actor,
		Target: target,
	}
	err := s.handleFollowCreated(context.Background(), domainEvent(t, domain.EventFollowCreated, payload))
	require.NoError(t, err)
	assert.False(t, published)
}

func TestFederationSubscriber_handleFollowCreated_WithTargetInbox_PublishesDelivery(t *testing.T) {
	t.Parallel()
	var got internal.OutboxDeliveryMessage
	var gotType string
	delivery := mockDeliveryWorker{publishFn: func(_ context.Context, actType string, msg internal.OutboxDeliveryMessage) error {
		gotType = actType
		got = msg
		return nil
	}}
	s := newTestSubscriber(delivery, mockFanoutWorker{})

	actor := localAccount("01actor")
	target := remoteAccount("01target")
	follow := &domain.Follow{ID: "01follow"}
	payload := domain.FollowCreatedPayload{Follow: follow, Actor: actor, Target: target, Local: true}

	err := s.handleFollowCreated(context.Background(), domainEvent(t, domain.EventFollowCreated, payload))
	require.NoError(t, err)
	assert.Equal(t, "follow", gotType)
	assert.Equal(t, target.InboxURL, got.TargetInbox)
	assert.Equal(t, actor.ID, got.SenderID)
	assert.Contains(t, got.ActivityID, "https://example.com/activities/")
}

func TestFederationSubscriber_handleFollowRemoved_RemoteActor_Skips(t *testing.T) {
	t.Parallel()
	published := false
	delivery := mockDeliveryWorker{publishFn: func(context.Context, string, internal.OutboxDeliveryMessage) error {
		published = true
		return nil
	}}
	s := newTestSubscriber(delivery, mockFanoutWorker{})

	actor := remoteAccount("01actor")
	target := remoteAccount("01target")
	payload := domain.FollowRemovedPayload{FollowID: "01follow", Actor: actor, Target: target}

	err := s.handleFollowRemoved(context.Background(), domainEvent(t, domain.EventFollowRemoved, payload))
	require.NoError(t, err)
	assert.False(t, published)
}

func TestFederationSubscriber_handleFollowRemoved_WithTargetInbox_PublishesUndo(t *testing.T) {
	t.Parallel()
	var got internal.OutboxDeliveryMessage
	var gotType string
	delivery := mockDeliveryWorker{publishFn: func(_ context.Context, actType string, msg internal.OutboxDeliveryMessage) error {
		gotType = actType
		got = msg
		return nil
	}}
	s := newTestSubscriber(delivery, mockFanoutWorker{})

	actor := localAccount("01actor")
	target := remoteAccount("01target")
	payload := domain.FollowRemovedPayload{FollowID: "01follow", Actor: actor, Target: target, Local: true}

	err := s.handleFollowRemoved(context.Background(), domainEvent(t, domain.EventFollowRemoved, payload))
	require.NoError(t, err)
	assert.Equal(t, "undo", gotType)
	assert.Equal(t, target.InboxURL, got.TargetInbox)
	assert.Equal(t, actor.ID, got.SenderID)
	assert.Contains(t, got.ActivityID, "undo-01follow")
}

func TestFederationSubscriber_handleFollowAccepted_RemoteTarget_Skips(t *testing.T) {
	t.Parallel()
	published := false
	delivery := mockDeliveryWorker{publishFn: func(context.Context, string, internal.OutboxDeliveryMessage) error {
		published = true
		return nil
	}}
	s := newTestSubscriber(delivery, mockFanoutWorker{})

	target := remoteAccount("01target")
	actor := remoteAccount("01actor")
	follow := &domain.Follow{ID: "01follow"}
	payload := domain.FollowAcceptedPayload{Follow: follow, Target: target, Actor: actor}

	err := s.handleFollowAccepted(context.Background(), domainEvent(t, domain.EventFollowAccepted, payload))
	require.NoError(t, err)
	assert.False(t, published)
}

func TestFederationSubscriber_handleFollowAccepted_WithActorInbox_PublishesAccept(t *testing.T) {
	t.Parallel()
	var got internal.OutboxDeliveryMessage
	var gotType string
	delivery := mockDeliveryWorker{publishFn: func(_ context.Context, actType string, msg internal.OutboxDeliveryMessage) error {
		gotType = actType
		got = msg
		return nil
	}}
	s := newTestSubscriber(delivery, mockFanoutWorker{})

	target := localAccount("01target")
	actor := remoteAccount("01actor")
	follow := &domain.Follow{ID: "01follow"}
	payload := domain.FollowAcceptedPayload{Follow: follow, Target: target, Actor: actor, Local: true}

	err := s.handleFollowAccepted(context.Background(), domainEvent(t, domain.EventFollowAccepted, payload))
	require.NoError(t, err)
	assert.Equal(t, "accept", gotType)
	assert.Equal(t, actor.InboxURL, got.TargetInbox)
	assert.Equal(t, target.ID, got.SenderID)
	assert.Contains(t, got.ActivityID, "accept-01follow")
}

func TestFederationSubscriber_handleBlockCreated_RemoteActor_Skips(t *testing.T) {
	t.Parallel()
	published := false
	delivery := mockDeliveryWorker{publishFn: func(context.Context, string, internal.OutboxDeliveryMessage) error {
		published = true
		return nil
	}}
	s := newTestSubscriber(delivery, mockFanoutWorker{})

	actor := remoteAccount("01actor")
	target := remoteAccount("01target")
	payload := domain.BlockCreatedPayload{Actor: actor, Target: target}

	err := s.handleBlockCreated(context.Background(), domainEvent(t, domain.EventBlockCreated, payload))
	require.NoError(t, err)
	assert.False(t, published)
}

func TestFederationSubscriber_handleBlockCreated_WithTargetInbox_PublishesBlock(t *testing.T) {
	t.Parallel()
	var got internal.OutboxDeliveryMessage
	var gotType string
	delivery := mockDeliveryWorker{publishFn: func(_ context.Context, actType string, msg internal.OutboxDeliveryMessage) error {
		gotType = actType
		got = msg
		return nil
	}}
	s := newTestSubscriber(delivery, mockFanoutWorker{})

	actor := localAccount("01actor")
	target := remoteAccount("01target")
	payload := domain.BlockCreatedPayload{Actor: actor, Target: target, Local: true}

	err := s.handleBlockCreated(context.Background(), domainEvent(t, domain.EventBlockCreated, payload))
	require.NoError(t, err)
	assert.Equal(t, "block", gotType)
	assert.Equal(t, target.InboxURL, got.TargetInbox)
	assert.Equal(t, actor.ID, got.SenderID)
}

func TestFederationSubscriber_handleBlockRemoved_RemoteActor_Skips(t *testing.T) {
	t.Parallel()
	published := false
	delivery := mockDeliveryWorker{publishFn: func(context.Context, string, internal.OutboxDeliveryMessage) error {
		published = true
		return nil
	}}
	s := newTestSubscriber(delivery, mockFanoutWorker{})

	actor := remoteAccount("01actor")
	target := remoteAccount("01target")
	payload := domain.BlockRemovedPayload{Actor: actor, Target: target}

	err := s.handleBlockRemoved(context.Background(), domainEvent(t, domain.EventBlockRemoved, payload))
	require.NoError(t, err)
	assert.False(t, published)
}

func TestFederationSubscriber_handleBlockRemoved_WithTargetInbox_PublishesUndoBlock(t *testing.T) {
	t.Parallel()
	var got internal.OutboxDeliveryMessage
	var gotType string
	delivery := mockDeliveryWorker{publishFn: func(_ context.Context, actType string, msg internal.OutboxDeliveryMessage) error {
		gotType = actType
		got = msg
		return nil
	}}
	s := newTestSubscriber(delivery, mockFanoutWorker{})

	actor := localAccount("01actor")
	target := remoteAccount("01target")
	payload := domain.BlockRemovedPayload{Actor: actor, Target: target, Local: true}

	err := s.handleBlockRemoved(context.Background(), domainEvent(t, domain.EventBlockRemoved, payload))
	require.NoError(t, err)
	assert.Equal(t, "undo", gotType)
	assert.Equal(t, target.InboxURL, got.TargetInbox)
	assert.Equal(t, actor.ID, got.SenderID)
	assert.Contains(t, got.ActivityID, "undo-block-")
}

func TestFederationSubscriber_handleAccountUpdated_PublishesFanout(t *testing.T) {
	t.Parallel()
	var got internal.OutboxFanoutMessage
	var gotType string
	fanout := mockFanoutWorker{publishFn: func(_ context.Context, actType string, msg internal.OutboxFanoutMessage) error {
		gotType = actType
		got = msg
		return nil
	}}
	s := newTestSubscriber(mockDeliveryWorker{}, fanout)

	account := localAccount("01account")
	payload := domain.AccountUpdatedPayload{Account: account, Local: true}

	err := s.handleAccountUpdated(context.Background(), domainEvent(t, domain.EventAccountUpdated, payload))
	require.NoError(t, err)
	assert.Equal(t, "update", gotType)
	assert.Equal(t, account.ID, got.SenderID)
	assert.NotEmpty(t, got.ActivityID)
	assert.NotEmpty(t, got.Activity)
}

func TestFederationSubscriber_handleAccountUpdated_RemoteAccount_Skips(t *testing.T) {
	t.Parallel()
	published := false
	fanout := mockFanoutWorker{publishFn: func(context.Context, string, internal.OutboxFanoutMessage) error {
		published = true
		return nil
	}}
	s := newTestSubscriber(mockDeliveryWorker{}, fanout)

	account := remoteAccount("01account")
	payload := domain.AccountUpdatedPayload{Account: account}

	err := s.handleAccountUpdated(context.Background(), domainEvent(t, domain.EventAccountUpdated, payload))
	require.NoError(t, err)
	assert.False(t, published)
}

// --- handleAccountDeleted ---

func TestFederationSubscriber_handleAccountDeleted_Local_PublishesFanout(t *testing.T) {
	t.Parallel()
	var got internal.OutboxFanoutMessage
	var gotType string
	fanout := mockFanoutWorker{publishFn: func(_ context.Context, actType string, msg internal.OutboxFanoutMessage) error {
		gotType = actType
		got = msg
		return nil
	}}
	s := newTestSubscriber(mockDeliveryWorker{}, fanout)

	const apID = "https://example.com/users/alice"
	const deletionID = "01DELETION"
	payload := domain.AccountDeletedPayload{DeletionID: deletionID, APID: apID, Local: true}

	err := s.handleAccountDeleted(context.Background(), domainEvent(t, domain.EventAccountDeleted, payload))
	require.NoError(t, err)

	assert.Equal(t, "delete", gotType)
	assert.Equal(t, apID+"#delete", got.ActivityID)
	// Deletion fanout routes via DeletionID, not SenderID — the sender's
	// accounts row is gone by the time the worker runs.
	assert.Equal(t, deletionID, got.DeletionID)
	assert.Empty(t, got.SenderID)

	// Wrapped Tombstone object IRI must equal the actor IRI.
	var activity struct {
		Type   string `json:"type"`
		Actor  string `json:"actor"`
		Object struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"object"`
	}
	require.NoError(t, json.Unmarshal(got.Activity, &activity))
	assert.Equal(t, "Delete", activity.Type)
	assert.Equal(t, apID, activity.Actor)
	assert.Equal(t, apID, activity.Object.ID)
	assert.Equal(t, "Tombstone", activity.Object.Type)
}

func TestFederationSubscriber_handleAccountDeleted_Remote_Skips(t *testing.T) {
	t.Parallel()
	published := false
	fanout := mockFanoutWorker{publishFn: func(context.Context, string, internal.OutboxFanoutMessage) error {
		published = true
		return nil
	}}
	s := newTestSubscriber(mockDeliveryWorker{}, fanout)

	payload := domain.AccountDeletedPayload{Local: false}

	err := s.handleAccountDeleted(context.Background(), domainEvent(t, domain.EventAccountDeleted, payload))
	require.NoError(t, err)
	assert.False(t, published)
}

func TestFederationSubscriber_handleAccountDeleted_MissingDeletionID_Skips(t *testing.T) {
	t.Parallel()
	published := false
	fanout := mockFanoutWorker{publishFn: func(context.Context, string, internal.OutboxFanoutMessage) error {
		published = true
		return nil
	}}
	s := newTestSubscriber(mockDeliveryWorker{}, fanout)

	// Local event with no DeletionID / APID is malformed — the emitter
	// (deleteLocalAccount) always sets both. The handler warns and acks.
	payload := domain.AccountDeletedPayload{Local: true}

	err := s.handleAccountDeleted(context.Background(), domainEvent(t, domain.EventAccountDeleted, payload))
	require.NoError(t, err)
	assert.False(t, published)
}

func TestFederationSubscriber_handleReblogCreated_RemoteFromAccount_Skips(t *testing.T) {
	t.Parallel()
	published := false
	fanout := mockFanoutWorker{publishFn: func(context.Context, string, internal.OutboxFanoutMessage) error {
		published = true
		return nil
	}}
	delivery := mockDeliveryWorker{publishFn: func(context.Context, string, internal.OutboxDeliveryMessage) error {
		published = true
		return nil
	}}
	s := newTestSubscriber(delivery, fanout)

	fromAccount := remoteAccount("01booster")
	originalAuthor := remoteAccount("01author")
	payload := domain.ReblogCreatedPayload{
		FromAccount:        fromAccount,
		OriginalAuthor:     originalAuthor,
		OriginalStatusAPID: "https://remote.example/statuses/01status",
	}
	err := s.handleReblogCreated(context.Background(), domainEvent(t, domain.EventReblogCreated, payload))
	require.NoError(t, err)
	assert.False(t, published)
}

func TestFederationSubscriber_handleReblogCreated_LocalFromAccount_PublishesFanoutAndDelivery(t *testing.T) {
	t.Parallel()
	var gotDelivery internal.OutboxDeliveryMessage
	var gotFanout internal.OutboxFanoutMessage
	delivery := mockDeliveryWorker{publishFn: func(_ context.Context, actType string, msg internal.OutboxDeliveryMessage) error {
		assert.Equal(t, "announce", actType)
		gotDelivery = msg
		return nil
	}}
	fanout := mockFanoutWorker{publishFn: func(_ context.Context, actType string, msg internal.OutboxFanoutMessage) error {
		assert.Equal(t, "announce", actType)
		gotFanout = msg
		return nil
	}}
	s := newTestSubscriber(delivery, fanout)

	fromAccount := localAccount("01booster")
	originalAuthor := remoteAccount("01author")
	originalStatusAPID := "https://remote.example/statuses/01status"
	payload := domain.ReblogCreatedPayload{
		FromAccount:        fromAccount,
		OriginalAuthor:     originalAuthor,
		OriginalStatusAPID: originalStatusAPID,
		Local:              true,
	}
	err := s.handleReblogCreated(context.Background(), domainEvent(t, domain.EventReblogCreated, payload))
	require.NoError(t, err)
	assert.Equal(t, originalAuthor.InboxURL, gotDelivery.TargetInbox)
	assert.Equal(t, fromAccount.ID, gotDelivery.SenderID)
	assert.Equal(t, fromAccount.ID, gotFanout.SenderID)
	assert.Contains(t, string(gotFanout.Activity), "Announce")
	assert.Contains(t, string(gotFanout.Activity), `"https://www.w3.org/ns/activitystreams#Public"`, "Announce should include Public in to")
	assert.Contains(t, string(gotFanout.Activity), "/followers", "Announce should include followers URL in cc")
}

func TestFederationSubscriber_handleReblogCreated_LocalOriginalAuthor_SkipsDelivery_PublishesFanout(t *testing.T) {
	t.Parallel()
	deliveryCalled := false
	fanoutCalled := false
	delivery := mockDeliveryWorker{publishFn: func(context.Context, string, internal.OutboxDeliveryMessage) error {
		deliveryCalled = true
		return nil
	}}
	fanout := mockFanoutWorker{publishFn: func(context.Context, string, internal.OutboxFanoutMessage) error {
		fanoutCalled = true
		return nil
	}}
	s := newTestSubscriber(delivery, fanout)

	fromAccount := localAccount("01booster")
	originalAuthor := localAccount("01author")
	payload := domain.ReblogCreatedPayload{
		FromAccount:        fromAccount,
		OriginalAuthor:     originalAuthor,
		OriginalStatusAPID: "https://example.com/statuses/01status",
		Local:              true,
	}
	err := s.handleReblogCreated(context.Background(), domainEvent(t, domain.EventReblogCreated, payload))
	require.NoError(t, err)
	assert.False(t, deliveryCalled)
	assert.True(t, fanoutCalled)
}

func TestFederationSubscriber_handleFavouriteCreated_RemoteFromAccount_Skips(t *testing.T) {
	t.Parallel()
	published := false
	delivery := mockDeliveryWorker{publishFn: func(context.Context, string, internal.OutboxDeliveryMessage) error {
		published = true
		return nil
	}}
	s := newTestSubscriber(delivery, mockFanoutWorker{})

	fromAccount := remoteAccount("01favouriter")
	statusAuthor := remoteAccount("01author")
	payload := domain.FavouriteCreatedPayload{
		FromAccount:  fromAccount,
		StatusAuthor: statusAuthor,
		StatusAPID:   "https://remote.example/statuses/01status",
	}
	err := s.handleFavouriteCreated(context.Background(), domainEvent(t, domain.EventFavouriteCreated, payload))
	require.NoError(t, err)
	assert.False(t, published)
}

func TestFederationSubscriber_handleFavouriteCreated_LocalFromAccount_RemoteAuthor_PublishesDelivery(t *testing.T) {
	t.Parallel()
	var got internal.OutboxDeliveryMessage
	delivery := mockDeliveryWorker{publishFn: func(_ context.Context, actType string, msg internal.OutboxDeliveryMessage) error {
		assert.Equal(t, "like", actType)
		got = msg
		return nil
	}}
	s := newTestSubscriber(delivery, mockFanoutWorker{})

	fromAccount := localAccount("01favouriter")
	statusAuthor := remoteAccount("01author")
	statusAPID := "https://remote.example/statuses/01status"
	payload := domain.FavouriteCreatedPayload{
		FromAccount:  fromAccount,
		StatusAuthor: statusAuthor,
		StatusAPID:   statusAPID,
		Local:        true,
	}
	err := s.handleFavouriteCreated(context.Background(), domainEvent(t, domain.EventFavouriteCreated, payload))
	require.NoError(t, err)
	assert.Equal(t, statusAuthor.InboxURL, got.TargetInbox)
	assert.Equal(t, fromAccount.ID, got.SenderID)
	assert.Contains(t, string(got.Activity), "Like")
	assert.Contains(t, string(got.Activity), statusAuthor.APID, "Like activity should include status author IRI in to field")
}

func TestFederationSubscriber_handleFavouriteCreated_LocalAuthor_Skips(t *testing.T) {
	t.Parallel()
	published := false
	delivery := mockDeliveryWorker{publishFn: func(context.Context, string, internal.OutboxDeliveryMessage) error {
		published = true
		return nil
	}}
	s := newTestSubscriber(delivery, mockFanoutWorker{})

	fromAccount := localAccount("01favouriter")
	statusAuthor := localAccount("01author")
	payload := domain.FavouriteCreatedPayload{
		FromAccount:  fromAccount,
		StatusAuthor: statusAuthor,
		StatusAPID:   "https://example.com/statuses/01status",
		Local:        true,
	}
	err := s.handleFavouriteCreated(context.Background(), domainEvent(t, domain.EventFavouriteCreated, payload))
	require.NoError(t, err)
	assert.False(t, published)
}

// domainEvent is a helper that encodes an event type + payload into a domain.DomainEvent.
func domainEvent(t *testing.T, eventType string, payload any) domain.DomainEvent {
	t.Helper()
	payloadJSON, err := json.Marshal(payload)
	require.NoError(t, err)
	return domain.DomainEvent{ID: "ev01", EventType: eventType, Payload: payloadJSON}
}
