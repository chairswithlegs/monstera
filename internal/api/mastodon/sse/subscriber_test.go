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
	"github.com/chairswithlegs/monstera/internal/service"
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
	followers   map[string][]string
	lists       map[string][]domain.List   // accountID -> lists containing that account
	listMembers map[string][]string        // listID -> member account IDs
	follows     map[string]map[string]bool // accountID -> targetID -> true
}

func newFakeSubscriberStore() *fakeSubscriberStore {
	return &fakeSubscriberStore{
		followers:   make(map[string][]string),
		lists:       make(map[string][]domain.List),
		listMembers: make(map[string][]string),
		follows:     make(map[string]map[string]bool),
	}
}

func (f *fakeSubscriberStore) GetLocalFollowerAccountIDs(_ context.Context, targetID string) ([]string, error) {
	return f.followers[targetID], nil
}
func (f *fakeSubscriberStore) GetListsByMemberAccountID(_ context.Context, accountID string) ([]domain.List, error) {
	return f.lists[accountID], nil
}
func (f *fakeSubscriberStore) ListListAccountIDs(_ context.Context, listID string) ([]string, error) {
	return f.listMembers[listID], nil
}
func (f *fakeSubscriberStore) GetFollow(_ context.Context, accountID, targetID string) (*domain.Follow, error) {
	if targets, ok := f.follows[accountID]; ok && targets[targetID] {
		return &domain.Follow{}, nil
	}
	return nil, domain.ErrNotFound
}

// fakeStatusEnricher implements subscriberStatusService for tests.
type fakeStatusEnricher struct {
	statuses map[string]*domain.Status
	accounts map[string]*domain.Account
}

func newFakeStatusEnricher() *fakeStatusEnricher {
	return &fakeStatusEnricher{
		statuses: make(map[string]*domain.Status),
		accounts: make(map[string]*domain.Account),
	}
}

func (f *fakeStatusEnricher) GetByIDEnriched(_ context.Context, id string, _ *string) (service.EnrichedStatus, error) {
	st, ok := f.statuses[id]
	if !ok || st.DeletedAt != nil {
		return service.EnrichedStatus{}, domain.ErrNotFound
	}
	author := f.accounts[st.AccountID]
	return service.EnrichedStatus{Status: st, Author: author}, nil
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

func newTestSubscriber(t *testing.T) (*Subscriber, *fakePublisher, *fakeSubscriberStore, *fakeStatusEnricher) {
	t.Helper()
	pub := &fakePublisher{}
	store := newFakeSubscriberStore()
	enricher := newFakeStatusEnricher()
	sub := &Subscriber{
		nc:             pub,
		store:          store,
		statusSvc:      enricher,
		instanceDomain: "example.com",
	}
	return sub, pub, store, enricher
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
	sub, pub, _, _ := newTestSubscriber(t)
	msg := &fakeMsg{data: []byte("not json")}

	sub.processMessage(context.Background(), msg)

	assert.True(t, msg.wasAcked())
	assert.Empty(t, pub.published())
}

func TestProcessMessage_UnknownEventType_AcksAndSkips(t *testing.T) {
	t.Parallel()
	sub, pub, _, _ := newTestSubscriber(t)

	data := makeDomainEvent(t, "unknown.event", map[string]string{"key": "value"})
	msg := &fakeMsg{data: data}

	sub.processMessage(context.Background(), msg)

	assert.True(t, msg.wasAcked())
	assert.Empty(t, pub.published())
}

func TestProcessMessage_StatusCreated_Public_PublishesToPublicAndFollowers(t *testing.T) {
	t.Parallel()
	sub, pub, store, _ := newTestSubscriber(t)

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
	assert.Contains(t, subjects, SubjectPrefixUser+"author-1", "should publish to author")
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
	sub, pub, store, _ := newTestSubscriber(t)

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
	sub, pub, store, _ := newTestSubscriber(t)

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

	assert.True(t, subjects[SubjectPrefixUser+"author-1"], "should publish to author")
	assert.True(t, subjects[SubjectPrefixUser+"follower-1"], "should publish to follower")
	assert.True(t, subjects[SubjectPrefixHashtag+"golang"], "should publish to hashtag")
	assert.False(t, subjects[SubjectPrefixPublic], "unlisted should not go to public timeline")
}

func TestProcessMessage_StatusCreated_Private_OnlyFollowersAndAuthor(t *testing.T) {
	t.Parallel()
	sub, pub, store, _ := newTestSubscriber(t)

	store.followers["author-1"] = []string{"follower-1"}

	payload := domain.StatusCreatedPayload{
		Status: makeMinimalStatus("status-1", "author-1", domain.VisibilityPrivate, true),
		Author: makeMinimalAccount("author-1", "alice"),
	}
	data := makeDomainEvent(t, domain.EventStatusCreated, payload)
	msg := &fakeMsg{data: data}

	sub.processMessage(context.Background(), msg)

	msgs := pub.published()
	require.Len(t, msgs, 2)

	subjects := make(map[string]bool)
	for _, m := range msgs {
		subjects[m.Subject] = true
	}
	assert.True(t, subjects[SubjectPrefixUser+"author-1"], "should publish to author")
	assert.True(t, subjects[SubjectPrefixUser+"follower-1"], "should publish to follower")
}

func TestProcessMessage_StatusCreated_Direct_OnlyMentionedAccounts(t *testing.T) {
	t.Parallel()
	sub, pub, store, _ := newTestSubscriber(t)

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
	sub, pub, _, _ := newTestSubscriber(t)

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
	sub, pub, store, _ := newTestSubscriber(t)

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
	sub, pub, _, _ := newTestSubscriber(t)

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
	sub, pub, store, _ := newTestSubscriber(t)

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
	sub, pub, _, _ := newTestSubscriber(t)

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
	require.Len(t, msgs, 2)

	subjects := make(map[string]bool, len(msgs))
	for _, m := range msgs {
		subjects[m.Subject] = true
		ev := decodePublished(t, m)
		assert.Equal(t, EventNotification, ev.Event)
	}
	assert.True(t, subjects[SubjectPrefixUser+"recipient-1"], "should publish to user stream")
	assert.True(t, subjects[SubjectPrefixUserNotification+"recipient-1"], "should publish to user:notification stream")
}

func TestProcessMessage_NotificationCreated_WithStatus(t *testing.T) {
	t.Parallel()
	sub, pub, _, enricher := newTestSubscriber(t)

	statusID := "status-1"
	enricher.statuses[statusID] = makeMinimalStatus(statusID, "sender-1", domain.VisibilityPublic, true)
	enricher.accounts["sender-1"] = makeMinimalAccount("sender-1", "bob")

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
	require.Len(t, msgs, 2)

	ev := decodePublished(t, msgs[0])
	assert.Equal(t, EventNotification, ev.Event)

	var notifJSON map[string]any
	require.NoError(t, json.Unmarshal([]byte(ev.Data), &notifJSON))
	assert.NotNil(t, notifJSON["status"], "notification with status should include status object")
}

func TestProcessMessage_NotificationCreated_NilNotification_NoPublish(t *testing.T) {
	t.Parallel()
	sub, pub, _, _ := newTestSubscriber(t)

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
	sub, pub, store, _ := newTestSubscriber(t)

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
	sub, pub, store, _ := newTestSubscriber(t)

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
	sub, pub, store, _ := newTestSubscriber(t)

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
	sub, pub, store, _ := newTestSubscriber(t)

	store.followers["author-1"] = []string{}
	store.lists["author-1"] = []domain.List{
		{ID: "list-1", AccountID: "owner-1", RepliesPolicy: domain.ListRepliesPolicyList},
		{ID: "list-2", AccountID: "owner-2", RepliesPolicy: domain.ListRepliesPolicyList},
	}

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
	sub, pub, store, _ := newTestSubscriber(t)

	store.followers["author-1"] = []string{"follower-1"}
	store.lists["author-1"] = []domain.List{
		{ID: "list-1", AccountID: "owner-1", RepliesPolicy: domain.ListRepliesPolicyList},
	}

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
	sub, pub, store, _ := newTestSubscriber(t)

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
	sub, pub, store, _ := newTestSubscriber(t)

	store.lists["author-1"] = []domain.List{
		{ID: "list-1", AccountID: "owner-1", RepliesPolicy: domain.ListRepliesPolicyList},
	}

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

func TestProcessMessage_StatusCreated_Reblog_ExcludedFromPublicTimeline(t *testing.T) {
	t.Parallel()
	sub, pub, store, enricher := newTestSubscriber(t)

	store.followers["author-1"] = []string{"follower-1"}

	origID := "original-1"
	enricher.statuses[origID] = makeMinimalStatus(origID, "orig-author", domain.VisibilityPublic, true)
	enricher.accounts["orig-author"] = makeMinimalAccount("orig-author", "bob")

	reblogStatus := makeMinimalStatus("reblog-1", "author-1", domain.VisibilityPublic, true)
	reblogStatus.ReblogOfID = &origID

	payload := domain.StatusCreatedPayload{
		Status: reblogStatus,
		Author: makeMinimalAccount("author-1", "alice"),
	}
	data := makeDomainEvent(t, domain.EventStatusCreated, payload)
	msg := &fakeMsg{data: data}

	sub.processMessage(context.Background(), msg)

	assert.True(t, msg.wasAcked())

	msgs := pub.published()
	subjects := make(map[string]bool)
	for _, m := range msgs {
		subjects[m.Subject] = true
	}

	assert.False(t, subjects[SubjectPrefixPublic], "reblogs should not appear on public timeline")
	assert.False(t, subjects[SubjectPrefixPublicLocal], "reblogs should not appear on public:local timeline")
	assert.True(t, subjects[SubjectPrefixUser+"follower-1"], "reblogs should still be delivered to followers")
}

func TestProcessMessage_StatusUpdated_PublishesStatusUpdateEvent(t *testing.T) {
	t.Parallel()
	sub, pub, store, _ := newTestSubscriber(t)

	store.followers["author-1"] = []string{"follower-1"}

	payload := domain.StatusUpdatedPayload{
		Status: makeMinimalStatus("status-1", "author-1", domain.VisibilityPublic, true),
		Author: makeMinimalAccount("author-1", "alice"),
	}
	data := makeDomainEvent(t, domain.EventStatusUpdated, payload)
	msg := &fakeMsg{data: data}

	sub.processMessage(context.Background(), msg)

	assert.True(t, msg.wasAcked())

	msgs := pub.published()
	require.NotEmpty(t, msgs)

	subjects := make(map[string]bool)
	for _, m := range msgs {
		subjects[m.Subject] = true
		ev := decodePublished(t, m)
		assert.Equal(t, EventStatusUpdate, ev.Event, "edited statuses should use status.update event type")
	}

	assert.True(t, subjects[SubjectPrefixPublic], "should publish to public timeline")
	assert.True(t, subjects[SubjectPrefixPublicLocal], "local edits should publish to public:local")
	assert.True(t, subjects[SubjectPrefixUser+"follower-1"], "should publish to followers")
}

func TestProcessMessage_StatusUpdatedRemote_IsHandled(t *testing.T) {
	t.Parallel()
	sub, pub, store, _ := newTestSubscriber(t)

	store.followers["author-1"] = []string{"follower-1"}

	payload := domain.StatusUpdatedPayload{
		Status: makeMinimalStatus("status-1", "author-1", domain.VisibilityPublic, false),
		Author: makeMinimalAccount("author-1", "alice"),
	}
	data := makeDomainEvent(t, domain.EventStatusUpdatedRemote, payload)
	msg := &fakeMsg{data: data}

	sub.processMessage(context.Background(), msg)

	assert.True(t, msg.wasAcked())
	assert.NotEmpty(t, pub.published(), "status.updated.remote should be handled")
}

// --- replies_policy SSE tests ---

func makeReplyStatus() *domain.Status {
	s := makeMinimalStatus("status-1", "author-1", domain.VisibilityPublic, true)
	parentID := "parent-1"
	targetID := "target-1"
	s.InReplyToID = &parentID
	s.InReplyToAccountID = &targetID
	return s
}

func TestProcessMessage_StatusCreated_Reply_PolicyNone_NotPublishedToList(t *testing.T) {
	t.Parallel()
	sub, pub, store, _ := newTestSubscriber(t)

	store.followers["author-1"] = []string{}
	store.lists["author-1"] = []domain.List{
		{ID: "list-1", AccountID: "owner-1", RepliesPolicy: domain.ListRepliesPolicyNone},
	}

	payload := domain.StatusCreatedPayload{
		Status: makeReplyStatus(),
		Author: makeMinimalAccount("author-1", "alice"),
	}
	data := makeDomainEvent(t, domain.EventStatusCreated, payload)
	msg := &fakeMsg{data: data}

	sub.processMessage(context.Background(), msg)

	msgs := pub.published()
	for _, m := range msgs {
		assert.NotEqual(t, SubjectPrefixList+"list-1", m.Subject, "replies should not go to list with none policy")
	}
}

func TestProcessMessage_StatusCreated_Reply_PolicyList_PublishedWhenReplyToMember(t *testing.T) {
	t.Parallel()
	sub, pub, store, _ := newTestSubscriber(t)

	store.followers["author-1"] = []string{}
	store.lists["author-1"] = []domain.List{
		{ID: "list-1", AccountID: "owner-1", RepliesPolicy: domain.ListRepliesPolicyList},
	}
	store.listMembers["list-1"] = []string{"author-1", "target-1"}

	payload := domain.StatusCreatedPayload{
		Status: makeReplyStatus(),
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
	assert.True(t, subjects[SubjectPrefixList+"list-1"], "reply to list member should be published")
}

func TestProcessMessage_StatusCreated_Reply_PolicyList_NotPublishedWhenReplyToNonMember(t *testing.T) {
	t.Parallel()
	sub, pub, store, _ := newTestSubscriber(t)

	store.followers["author-1"] = []string{}
	store.lists["author-1"] = []domain.List{
		{ID: "list-1", AccountID: "owner-1", RepliesPolicy: domain.ListRepliesPolicyList},
	}
	store.listMembers["list-1"] = []string{"author-1"} // target-1 is NOT a member

	payload := domain.StatusCreatedPayload{
		Status: makeReplyStatus(),
		Author: makeMinimalAccount("author-1", "alice"),
	}
	data := makeDomainEvent(t, domain.EventStatusCreated, payload)
	msg := &fakeMsg{data: data}

	sub.processMessage(context.Background(), msg)

	msgs := pub.published()
	for _, m := range msgs {
		assert.NotEqual(t, SubjectPrefixList+"list-1", m.Subject, "reply to non-member should not be published to list")
	}
}

func TestProcessMessage_StatusCreated_Reply_PolicyFollowed_PublishedWhenOwnerFollowsTarget(t *testing.T) {
	t.Parallel()
	sub, pub, store, _ := newTestSubscriber(t)

	store.followers["author-1"] = []string{}
	store.lists["author-1"] = []domain.List{
		{ID: "list-1", AccountID: "owner-1", RepliesPolicy: domain.ListRepliesPolicyFollowed},
	}
	store.follows["owner-1"] = map[string]bool{"target-1": true}

	payload := domain.StatusCreatedPayload{
		Status: makeReplyStatus(),
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
	assert.True(t, subjects[SubjectPrefixList+"list-1"], "reply to followed account should be published")
}

func TestProcessMessage_StatusCreated_Reply_PolicyFollowed_NotPublishedWhenOwnerDoesNotFollowTarget(t *testing.T) {
	t.Parallel()
	sub, pub, store, _ := newTestSubscriber(t)

	store.followers["author-1"] = []string{}
	store.lists["author-1"] = []domain.List{
		{ID: "list-1", AccountID: "owner-1", RepliesPolicy: domain.ListRepliesPolicyFollowed},
	}
	// owner-1 does NOT follow target-1

	payload := domain.StatusCreatedPayload{
		Status: makeReplyStatus(),
		Author: makeMinimalAccount("author-1", "alice"),
	}
	data := makeDomainEvent(t, domain.EventStatusCreated, payload)
	msg := &fakeMsg{data: data}

	sub.processMessage(context.Background(), msg)

	msgs := pub.published()
	for _, m := range msgs {
		assert.NotEqual(t, SubjectPrefixList+"list-1", m.Subject, "reply to non-followed should not be published")
	}
}

func TestProcessMessage_StatusCreated_NonReply_AlwaysPublishedToList(t *testing.T) {
	t.Parallel()
	sub, pub, store, _ := newTestSubscriber(t)

	store.followers["author-1"] = []string{}
	store.lists["author-1"] = []domain.List{
		{ID: "list-1", AccountID: "owner-1", RepliesPolicy: domain.ListRepliesPolicyNone},
	}

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
	assert.True(t, subjects[SubjectPrefixList+"list-1"], "non-reply should always be published regardless of policy")
}

func TestProcessMessage_StatusDeleted_AlwaysPublishedToList(t *testing.T) {
	t.Parallel()
	sub, pub, store, _ := newTestSubscriber(t)

	store.followers["author-1"] = []string{}
	store.lists["author-1"] = []domain.List{
		{ID: "list-1", AccountID: "owner-1", RepliesPolicy: domain.ListRepliesPolicyNone},
	}

	payload := domain.StatusDeletedPayload{
		StatusID:  "status-1",
		AccountID: "author-1",
		Local:     true,
	}
	data := makeDomainEvent(t, domain.EventStatusDeleted, payload)
	msg := &fakeMsg{data: data}

	sub.processMessage(context.Background(), msg)

	msgs := pub.published()
	subjects := make(map[string]bool)
	for _, m := range msgs {
		subjects[m.Subject] = true
	}
	assert.True(t, subjects[SubjectPrefixList+"list-1"], "delete events should always be published to list streams")
}

// --- exclusive flag SSE tests ---

func TestProcessMessage_StatusCreated_ExclusiveList_SkipsHomeStreamForOwner(t *testing.T) {
	t.Parallel()
	sub, pub, store, _ := newTestSubscriber(t)

	store.followers["author-1"] = []string{"owner-1", "other-follower"}
	store.lists["author-1"] = []domain.List{
		{ID: "list-1", AccountID: "owner-1", Exclusive: true, RepliesPolicy: domain.ListRepliesPolicyList},
	}

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

	assert.False(t, subjects[SubjectPrefixUser+"owner-1"], "exclusive list owner should not receive on home stream")
	assert.True(t, subjects[SubjectPrefixUser+"other-follower"], "other followers should still receive on home stream")
	assert.True(t, subjects[SubjectPrefixList+"list-1"], "exclusive list should still receive the event")
}

func TestProcessMessage_StatusCreated_NonExclusiveList_StillPublishesToHome(t *testing.T) {
	t.Parallel()
	sub, pub, store, _ := newTestSubscriber(t)

	store.followers["author-1"] = []string{"owner-1"}
	store.lists["author-1"] = []domain.List{
		{ID: "list-1", AccountID: "owner-1", Exclusive: false, RepliesPolicy: domain.ListRepliesPolicyList},
	}

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

	assert.True(t, subjects[SubjectPrefixUser+"owner-1"], "non-exclusive list owner should still receive on home stream")
	assert.True(t, subjects[SubjectPrefixList+"list-1"], "list stream should receive the event")
}
