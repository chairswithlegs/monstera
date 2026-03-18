package sse

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
)

// --- fakes ---

type publishedMsg struct {
	Subject string
	Data    []byte
}

type fakePublisher struct {
	mu   sync.Mutex
	msgs []publishedMsg
}

func (f *fakePublisher) Publish(subject string, data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.msgs = append(f.msgs, publishedMsg{Subject: subject, Data: data})
	return nil
}

func (f *fakePublisher) published() []publishedMsg {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]publishedMsg, len(f.msgs))
	copy(out, f.msgs)
	return out
}

type fakeSubscriberStore struct {
	followers     map[string][]string
	listsByMember map[string][]string
	statuses      map[string]*domain.Status
	accounts      map[string]*domain.Account
	mentions      map[string][]*domain.Account
	hashtags      map[string][]domain.Hashtag
	attachments   map[string][]domain.MediaAttachment
}

func newFakeSubscriberStore() *fakeSubscriberStore {
	return &fakeSubscriberStore{
		followers:     make(map[string][]string),
		listsByMember: make(map[string][]string),
		statuses:      make(map[string]*domain.Status),
		accounts:      make(map[string]*domain.Account),
		mentions:      make(map[string][]*domain.Account),
		hashtags:      make(map[string][]domain.Hashtag),
		attachments:   make(map[string][]domain.MediaAttachment),
	}
}

func (f *fakeSubscriberStore) GetLocalFollowerAccountIDs(_ context.Context, targetID string) ([]string, error) {
	return f.followers[targetID], nil
}
func (f *fakeSubscriberStore) GetListIDsByMemberAccountID(_ context.Context, accountID string) ([]string, error) {
	return f.listsByMember[accountID], nil
}
func (f *fakeSubscriberStore) GetStatusByID(_ context.Context, id string) (*domain.Status, error) {
	if s, ok := f.statuses[id]; ok {
		return s, nil
	}
	return nil, domain.ErrNotFound
}
func (f *fakeSubscriberStore) GetAccountByID(_ context.Context, id string) (*domain.Account, error) {
	if a, ok := f.accounts[id]; ok {
		return a, nil
	}
	return nil, domain.ErrNotFound
}
func (f *fakeSubscriberStore) GetStatusMentions(_ context.Context, statusID string) ([]*domain.Account, error) {
	return f.mentions[statusID], nil
}
func (f *fakeSubscriberStore) GetStatusHashtags(_ context.Context, statusID string) ([]domain.Hashtag, error) {
	return f.hashtags[statusID], nil
}
func (f *fakeSubscriberStore) GetStatusAttachments(_ context.Context, statusID string) ([]domain.MediaAttachment, error) {
	return f.attachments[statusID], nil
}

type fakeMsg struct {
	data  []byte
	acked bool
	mu    sync.Mutex
}

func (m *fakeMsg) Ack() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.acked = true
	return nil
}
func (m *fakeMsg) wasAcked() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.acked
}
func (m *fakeMsg) Data() []byte                              { return m.data }
func (m *fakeMsg) Subject() string                           { return "" }
func (m *fakeMsg) Reply() string                             { return "" }
func (m *fakeMsg) Nak() error                                { return nil }
func (m *fakeMsg) NakWithDelay(_ time.Duration) error        { return nil }
func (m *fakeMsg) InProgress() error                         { return nil }
func (m *fakeMsg) Term() error                               { return nil }
func (m *fakeMsg) TermWithReason(_ string) error             { return nil }
func (m *fakeMsg) Metadata() (*jetstream.MsgMetadata, error) { return nil, nil }
func (m *fakeMsg) Headers() nats.Header                      { return nil }
func (m *fakeMsg) DoubleAck(_ context.Context) error         { return nil }

// --- helpers ---

func newTestSubscriber(t *testing.T) (*Subscriber, *fakePublisher, *fakeSubscriberStore) {
	t.Helper()
	pub := &fakePublisher{}
	store := newFakeSubscriberStore()
	sub := &Subscriber{
		nc:             pub,
		store:          store,
		instanceDomain: "example.com",
	}
	return sub, pub, store
}

func makeDomainEvent(t *testing.T, eventType string, payload any) []byte {
	t.Helper()
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	ev := domain.DomainEvent{
		ID:        "evt-1",
		EventType: eventType,
		Payload:   raw,
	}
	data, err := json.Marshal(ev)
	require.NoError(t, err)
	return data
}

func decodePublished(t *testing.T, msg publishedMsg) SSEEvent {
	t.Helper()
	var ev SSEEvent
	require.NoError(t, json.Unmarshal(msg.Data, &ev))
	return ev
}

func makeMinimalStatus(id, accountID, visibility string, local bool) *domain.Status {
	now := time.Now()
	content := "<p>hello</p>"
	return &domain.Status{
		ID:         id,
		AccountID:  accountID,
		Visibility: visibility,
		Local:      local,
		CreatedAt:  now,
		Content:    &content,
	}
}

func makeMinimalAccount(id, username string) *domain.Account {
	return &domain.Account{
		ID:        id,
		Username:  username,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// --- tests ---

func TestProcessMessage_InvalidJSON_AcksAndSkips(t *testing.T) {
	t.Parallel()
	sub, pub, _ := newTestSubscriber(t)
	msg := &fakeMsg{data: []byte("not json")}

	sub.processMessage(context.Background(), msg)

	assert.True(t, msg.wasAcked())
	assert.Empty(t, pub.published())
}

func TestProcessMessage_UnknownEventType_AcksAndSkips(t *testing.T) {
	t.Parallel()
	sub, pub, _ := newTestSubscriber(t)

	data := makeDomainEvent(t, "unknown.event", map[string]string{"key": "value"})
	msg := &fakeMsg{data: data}

	sub.processMessage(context.Background(), msg)

	assert.True(t, msg.wasAcked())
	assert.Empty(t, pub.published())
}

func TestProcessMessage_StatusCreated_Public_PublishesToPublicAndFollowers(t *testing.T) {
	t.Parallel()
	sub, pub, store := newTestSubscriber(t)

	store.followers["author-1"] = []string{"follower-1", "follower-2"}

	payload := domain.StatusCreatedPayload{
		Status: makeMinimalStatus("status-1", "author-1", domain.VisibilityPublic, true),
		Author: makeMinimalAccount("author-1", "alice"),
	}
	data := makeDomainEvent(t, domain.EventStatusCreated, payload)
	msg := &fakeMsg{data: data}

	sub.processMessage(context.Background(), msg)

	assert.True(t, msg.wasAcked())

	msgs := pub.published()
	subjects := make(map[string]int)
	for _, m := range msgs {
		subjects[m.Subject]++
	}

	assert.Contains(t, subjects, SubjectPrefixPublic, "should publish to public")
	assert.Contains(t, subjects, SubjectPrefixPublicLocal, "local status should publish to public:local")
	assert.Contains(t, subjects, SubjectPrefixUser+"follower-1")
	assert.Contains(t, subjects, SubjectPrefixUser+"follower-2")

	for _, m := range msgs {
		ev := decodePublished(t, m)
		assert.Equal(t, EventUpdate, ev.Event)
		assert.NotEmpty(t, ev.Data)
	}
}

func TestProcessMessage_StatusCreated_Public_Remote_SkipsPublicLocal(t *testing.T) {
	t.Parallel()
	sub, pub, store := newTestSubscriber(t)

	store.followers["author-1"] = []string{"follower-1"}

	payload := domain.StatusCreatedPayload{
		Status: makeMinimalStatus("status-1", "author-1", domain.VisibilityPublic, false),
		Author: makeMinimalAccount("author-1", "alice"),
	}
	data := makeDomainEvent(t, domain.EventStatusCreated, payload)
	msg := &fakeMsg{data: data}

	sub.processMessage(context.Background(), msg)

	msgs := pub.published()
	for _, m := range msgs {
		assert.NotEqual(t, SubjectPrefixPublicLocal, m.Subject, "remote status should not go to public:local")
	}
}

func TestProcessMessage_StatusCreated_Unlisted_PublishesToFollowersAndHashtags(t *testing.T) {
	t.Parallel()
	sub, pub, store := newTestSubscriber(t)

	store.followers["author-1"] = []string{"follower-1"}

	payload := domain.StatusCreatedPayload{
		Status: makeMinimalStatus("status-1", "author-1", domain.VisibilityUnlisted, true),
		Author: makeMinimalAccount("author-1", "alice"),
		Tags:   []domain.Hashtag{{Name: "golang"}},
	}
	data := makeDomainEvent(t, domain.EventStatusCreated, payload)
	msg := &fakeMsg{data: data}

	sub.processMessage(context.Background(), msg)

	msgs := pub.published()
	subjects := make(map[string]bool)
	for _, m := range msgs {
		subjects[m.Subject] = true
	}

	assert.True(t, subjects[SubjectPrefixUser+"follower-1"], "should publish to follower")
	assert.True(t, subjects[SubjectPrefixHashtag+"golang"], "should publish to hashtag")
	assert.False(t, subjects[SubjectPrefixPublic], "unlisted should not go to public timeline")
}

func TestProcessMessage_StatusCreated_Private_OnlyFollowers(t *testing.T) {
	t.Parallel()
	sub, pub, store := newTestSubscriber(t)

	store.followers["author-1"] = []string{"follower-1"}

	payload := domain.StatusCreatedPayload{
		Status: makeMinimalStatus("status-1", "author-1", domain.VisibilityPrivate, true),
		Author: makeMinimalAccount("author-1", "alice"),
	}
	data := makeDomainEvent(t, domain.EventStatusCreated, payload)
	msg := &fakeMsg{data: data}

	sub.processMessage(context.Background(), msg)

	msgs := pub.published()
	require.Len(t, msgs, 1)
	assert.Equal(t, SubjectPrefixUser+"follower-1", msgs[0].Subject)
}

func TestProcessMessage_StatusCreated_Direct_OnlyMentionedAccounts(t *testing.T) {
	t.Parallel()
	sub, pub, store := newTestSubscriber(t)

	store.followers["author-1"] = []string{"follower-1"}

	payload := domain.StatusCreatedPayload{
		Status:              makeMinimalStatus("status-1", "author-1", domain.VisibilityDirect, true),
		Author:              makeMinimalAccount("author-1", "alice"),
		MentionedAccountIDs: []string{"mentioned-1", "mentioned-2"},
	}
	data := makeDomainEvent(t, domain.EventStatusCreated, payload)
	msg := &fakeMsg{data: data}

	sub.processMessage(context.Background(), msg)

	msgs := pub.published()
	subjects := make(map[string]bool)
	for _, m := range msgs {
		subjects[m.Subject] = true
	}

	assert.True(t, subjects[SubjectPrefixUser+"mentioned-1"])
	assert.True(t, subjects[SubjectPrefixUser+"mentioned-2"])
	assert.False(t, subjects[SubjectPrefixUser+"follower-1"], "followers should not see direct messages")
	assert.False(t, subjects[SubjectPrefixPublic])
}

func TestProcessMessage_StatusCreated_NilStatus_NoPublish(t *testing.T) {
	t.Parallel()
	sub, pub, _ := newTestSubscriber(t)

	payload := domain.StatusCreatedPayload{
		Status: nil,
		Author: makeMinimalAccount("author-1", "alice"),
	}
	data := makeDomainEvent(t, domain.EventStatusCreated, payload)
	msg := &fakeMsg{data: data}

	sub.processMessage(context.Background(), msg)

	assert.True(t, msg.wasAcked())
	assert.Empty(t, pub.published())
}

func TestProcessMessage_StatusDeleted_Public_PublishesDeleteEvent(t *testing.T) {
	t.Parallel()
	sub, pub, store := newTestSubscriber(t)

	store.followers["author-1"] = []string{"follower-1"}

	payload := domain.StatusDeletedPayload{
		StatusID:   "status-1",
		AccountID:  "author-1",
		Visibility: domain.VisibilityPublic,
		Local:      true,
	}
	data := makeDomainEvent(t, domain.EventStatusDeleted, payload)
	msg := &fakeMsg{data: data}

	sub.processMessage(context.Background(), msg)

	assert.True(t, msg.wasAcked())

	msgs := pub.published()
	subjects := make(map[string]bool)
	for _, m := range msgs {
		subjects[m.Subject] = true
		ev := decodePublished(t, m)
		assert.Equal(t, EventDelete, ev.Event)
		assert.Equal(t, "status-1", ev.Data)
	}

	assert.True(t, subjects[SubjectPrefixPublic])
	assert.True(t, subjects[SubjectPrefixPublicLocal])
	assert.True(t, subjects[SubjectPrefixUser+"follower-1"])
}

func TestProcessMessage_StatusDeleted_WithHashtags(t *testing.T) {
	t.Parallel()
	sub, pub, _ := newTestSubscriber(t)

	payload := domain.StatusDeletedPayload{
		StatusID:     "status-1",
		AccountID:    "author-1",
		Visibility:   domain.VisibilityPublic,
		Local:        false,
		HashtagNames: []string{"golang", "fediverse"},
	}
	data := makeDomainEvent(t, domain.EventStatusDeleted, payload)
	msg := &fakeMsg{data: data}

	sub.processMessage(context.Background(), msg)

	msgs := pub.published()
	subjects := make(map[string]bool)
	for _, m := range msgs {
		subjects[m.Subject] = true
	}

	assert.True(t, subjects[SubjectPrefixHashtag+"golang"])
	assert.True(t, subjects[SubjectPrefixHashtag+"fediverse"])
}

func TestProcessMessage_StatusDeleted_Direct_OnlyMentioned(t *testing.T) {
	t.Parallel()
	sub, pub, store := newTestSubscriber(t)

	store.followers["author-1"] = []string{"follower-1"}

	payload := domain.StatusDeletedPayload{
		StatusID:            "status-1",
		AccountID:           "author-1",
		Visibility:          domain.VisibilityDirect,
		MentionedAccountIDs: []string{"mentioned-1"},
	}
	data := makeDomainEvent(t, domain.EventStatusDeleted, payload)
	msg := &fakeMsg{data: data}

	sub.processMessage(context.Background(), msg)

	msgs := pub.published()
	subjects := make(map[string]bool)
	for _, m := range msgs {
		subjects[m.Subject] = true
	}

	assert.True(t, subjects[SubjectPrefixUser+"mentioned-1"])
	assert.True(t, subjects[SubjectPrefixUser+"follower-1"], "delete still publishes to followers for cleanup")
}

func TestProcessMessage_NotificationCreated_PublishesToRecipient(t *testing.T) {
	t.Parallel()
	sub, pub, _ := newTestSubscriber(t)

	notif := &domain.Notification{
		ID:        "notif-1",
		AccountID: "recipient-1",
		FromID:    "sender-1",
		Type:      domain.NotificationTypeMention,
		CreatedAt: time.Now(),
	}

	payload := domain.NotificationCreatedPayload{
		RecipientAccountID: "recipient-1",
		Notification:       notif,
		FromAccount:        makeMinimalAccount("sender-1", "bob"),
	}
	data := makeDomainEvent(t, domain.EventNotificationCreated, payload)
	msg := &fakeMsg{data: data}

	sub.processMessage(context.Background(), msg)

	assert.True(t, msg.wasAcked())

	msgs := pub.published()
	require.Len(t, msgs, 1)
	assert.Equal(t, SubjectPrefixUser+"recipient-1", msgs[0].Subject)

	ev := decodePublished(t, msgs[0])
	assert.Equal(t, EventNotification, ev.Event)
	assert.Equal(t, streamNameUser, ev.Stream)
}

func TestProcessMessage_NotificationCreated_WithStatus(t *testing.T) {
	t.Parallel()
	sub, pub, store := newTestSubscriber(t)

	statusID := "status-1"
	store.statuses[statusID] = makeMinimalStatus(statusID, "sender-1", domain.VisibilityPublic, true)
	store.accounts["sender-1"] = makeMinimalAccount("sender-1", "bob")

	notif := &domain.Notification{
		ID:        "notif-1",
		AccountID: "recipient-1",
		FromID:    "sender-1",
		Type:      domain.NotificationTypeMention,
		StatusID:  &statusID,
		CreatedAt: time.Now(),
	}

	payload := domain.NotificationCreatedPayload{
		RecipientAccountID: "recipient-1",
		Notification:       notif,
		FromAccount:        makeMinimalAccount("sender-1", "bob"),
		StatusID:           &statusID,
	}
	data := makeDomainEvent(t, domain.EventNotificationCreated, payload)
	msg := &fakeMsg{data: data}

	sub.processMessage(context.Background(), msg)

	msgs := pub.published()
	require.Len(t, msgs, 1)

	ev := decodePublished(t, msgs[0])
	assert.Equal(t, EventNotification, ev.Event)

	var notifJSON map[string]any
	require.NoError(t, json.Unmarshal([]byte(ev.Data), &notifJSON))
	assert.NotNil(t, notifJSON["status"], "notification with status should include status object")
}

func TestProcessMessage_NotificationCreated_NilNotification_NoPublish(t *testing.T) {
	t.Parallel()
	sub, pub, _ := newTestSubscriber(t)

	payload := domain.NotificationCreatedPayload{
		RecipientAccountID: "recipient-1",
		Notification:       nil,
		FromAccount:        makeMinimalAccount("sender-1", "bob"),
	}
	data := makeDomainEvent(t, domain.EventNotificationCreated, payload)
	msg := &fakeMsg{data: data}

	sub.processMessage(context.Background(), msg)

	assert.True(t, msg.wasAcked())
	assert.Empty(t, pub.published())
}

func TestProcessMessage_StatusCreated_PublicWithHashtags(t *testing.T) {
	t.Parallel()
	sub, pub, store := newTestSubscriber(t)

	store.followers["author-1"] = []string{}

	payload := domain.StatusCreatedPayload{
		Status: makeMinimalStatus("status-1", "author-1", domain.VisibilityPublic, true),
		Author: makeMinimalAccount("author-1", "alice"),
		Tags:   []domain.Hashtag{{Name: "golang"}, {Name: "activitypub"}},
	}
	data := makeDomainEvent(t, domain.EventStatusCreated, payload)
	msg := &fakeMsg{data: data}

	sub.processMessage(context.Background(), msg)

	msgs := pub.published()
	subjects := make(map[string]bool)
	for _, m := range msgs {
		subjects[m.Subject] = true
	}

	assert.True(t, subjects[SubjectPrefixPublic])
	assert.True(t, subjects[SubjectPrefixPublicLocal])
	assert.True(t, subjects[SubjectPrefixHashtag+"golang"])
	assert.True(t, subjects[SubjectPrefixHashtag+"activitypub"])
}

func TestProcessMessage_StatusDeletedRemote_IsHandled(t *testing.T) {
	t.Parallel()
	sub, pub, store := newTestSubscriber(t)

	store.followers["author-1"] = []string{"follower-1"}

	payload := domain.StatusDeletedPayload{
		StatusID:   "status-1",
		AccountID:  "author-1",
		Visibility: domain.VisibilityPublic,
		Local:      false,
	}
	data := makeDomainEvent(t, domain.EventStatusDeletedRemote, payload)
	msg := &fakeMsg{data: data}

	sub.processMessage(context.Background(), msg)

	assert.True(t, msg.wasAcked())
	assert.NotEmpty(t, pub.published(), "status.deleted.remote should be handled")
}

func TestProcessMessage_StatusCreatedRemote_IsHandled(t *testing.T) {
	t.Parallel()
	sub, pub, store := newTestSubscriber(t)

	store.followers["author-1"] = []string{"follower-1"}

	payload := domain.StatusCreatedPayload{
		Status: makeMinimalStatus("status-1", "author-1", domain.VisibilityPublic, false),
		Author: makeMinimalAccount("author-1", "alice"),
	}
	data := makeDomainEvent(t, domain.EventStatusCreatedRemote, payload)
	msg := &fakeMsg{data: data}

	sub.processMessage(context.Background(), msg)

	assert.True(t, msg.wasAcked())
	assert.NotEmpty(t, pub.published(), "status.created.remote should be handled")
}

func TestProcessMessage_StatusCreated_Public_PublishesToListStreams(t *testing.T) {
	t.Parallel()
	sub, pub, store := newTestSubscriber(t)

	store.followers["author-1"] = []string{}
	store.listsByMember["author-1"] = []string{"list-1", "list-2"}

	payload := domain.StatusCreatedPayload{
		Status: makeMinimalStatus("status-1", "author-1", domain.VisibilityPublic, true),
		Author: makeMinimalAccount("author-1", "alice"),
	}
	data := makeDomainEvent(t, domain.EventStatusCreated, payload)
	msg := &fakeMsg{data: data}

	sub.processMessage(context.Background(), msg)

	msgs := pub.published()
	subjects := make(map[string]bool)
	for _, m := range msgs {
		subjects[m.Subject] = true
	}

	assert.True(t, subjects[SubjectPrefixList+"list-1"], "should publish to list-1")
	assert.True(t, subjects[SubjectPrefixList+"list-2"], "should publish to list-2")
}

func TestProcessMessage_StatusCreated_Private_PublishesToListStreams(t *testing.T) {
	t.Parallel()
	sub, pub, store := newTestSubscriber(t)

	store.followers["author-1"] = []string{"follower-1"}
	store.listsByMember["author-1"] = []string{"list-1"}

	payload := domain.StatusCreatedPayload{
		Status: makeMinimalStatus("status-1", "author-1", domain.VisibilityPrivate, true),
		Author: makeMinimalAccount("author-1", "alice"),
	}
	data := makeDomainEvent(t, domain.EventStatusCreated, payload)
	msg := &fakeMsg{data: data}

	sub.processMessage(context.Background(), msg)

	msgs := pub.published()
	subjects := make(map[string]bool)
	for _, m := range msgs {
		subjects[m.Subject] = true
	}

	assert.True(t, subjects[SubjectPrefixUser+"follower-1"], "should publish to follower")
	assert.True(t, subjects[SubjectPrefixList+"list-1"], "should publish to list stream")
}

func TestProcessMessage_StatusCreated_Direct_PublishesToDirectStreams(t *testing.T) {
	t.Parallel()
	sub, pub, store := newTestSubscriber(t)

	store.followers["author-1"] = []string{"follower-1"}

	payload := domain.StatusCreatedPayload{
		Status:              makeMinimalStatus("status-1", "author-1", domain.VisibilityDirect, true),
		Author:              makeMinimalAccount("author-1", "alice"),
		MentionedAccountIDs: []string{"mentioned-1", "mentioned-2"},
	}
	data := makeDomainEvent(t, domain.EventStatusCreated, payload)
	msg := &fakeMsg{data: data}

	sub.processMessage(context.Background(), msg)

	msgs := pub.published()
	subjects := make(map[string]bool)
	for _, m := range msgs {
		subjects[m.Subject] = true
	}

	assert.True(t, subjects[SubjectPrefixUser+"mentioned-1"], "should publish to user stream")
	assert.True(t, subjects[SubjectPrefixUser+"mentioned-2"], "should publish to user stream")
	assert.True(t, subjects[SubjectPrefixDirect+"mentioned-1"], "should publish to direct stream")
	assert.True(t, subjects[SubjectPrefixDirect+"mentioned-2"], "should publish to direct stream")
	assert.False(t, subjects[SubjectPrefixUser+"follower-1"], "followers should not see direct messages")
}

func TestProcessMessage_StatusCreated_Direct_NoListStreams(t *testing.T) {
	t.Parallel()
	sub, pub, store := newTestSubscriber(t)

	store.listsByMember["author-1"] = []string{"list-1"}

	payload := domain.StatusCreatedPayload{
		Status:              makeMinimalStatus("status-1", "author-1", domain.VisibilityDirect, true),
		Author:              makeMinimalAccount("author-1", "alice"),
		MentionedAccountIDs: []string{"mentioned-1"},
	}
	data := makeDomainEvent(t, domain.EventStatusCreated, payload)
	msg := &fakeMsg{data: data}

	sub.processMessage(context.Background(), msg)

	msgs := pub.published()
	for _, m := range msgs {
		assert.NotEqual(t, SubjectPrefixList+"list-1", m.Subject, "direct messages should not go to list streams")
	}
}
