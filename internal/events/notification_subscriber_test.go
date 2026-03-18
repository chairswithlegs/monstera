package events

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
)

type fakeNotifCreator struct {
	calls []notifCall
}

type notifCall struct {
	RecipientID   string
	FromAccountID string
	Type          string
	StatusID      *string
}

func (f *fakeNotifCreator) CreateAndEmit(_ context.Context, recipientID, fromAccountID, notifType string, statusID *string) error {
	f.calls = append(f.calls, notifCall{
		RecipientID:   recipientID,
		FromAccountID: fromAccountID,
		Type:          notifType,
		StatusID:      statusID,
	})
	return nil
}

type fakeAccountLookup struct {
	accounts map[string]*domain.Account
}

func (f *fakeAccountLookup) GetByID(_ context.Context, id string) (*domain.Account, error) {
	a, ok := f.accounts[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return a, nil
}

type fakeConversationMuteChecker struct {
	muted map[string]bool // key: "accountID:statusID"
}

func (f *fakeConversationMuteChecker) IsConversationMutedForViewer(_ context.Context, viewerAccountID, statusID string) (bool, error) {
	return f.muted[viewerAccountID+":"+statusID], nil
}

func newTestSub() (*NotificationSubscriber, *fakeNotifCreator, *fakeAccountLookup, *fakeConversationMuteChecker) {
	nc := &fakeNotifCreator{}
	al := &fakeAccountLookup{accounts: make(map[string]*domain.Account)}
	cm := &fakeConversationMuteChecker{muted: make(map[string]bool)}
	sub := &NotificationSubscriber{deps: NotificationDeps{
		Notifications: nc,
		Accounts:      al,
		Conversations: cm,
	}}
	return sub, nc, al, cm
}

func makeEvent(t *testing.T, eventType string, payload any) domain.DomainEvent {
	t.Helper()
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	return domain.DomainEvent{
		ID:        "evt-1",
		EventType: eventType,
		Payload:   raw,
	}
}

func localAccount(id, username string) *domain.Account {
	return &domain.Account{ID: id, Username: username}
}

func remoteAccount(id, username, domainName string) *domain.Account {
	return &domain.Account{ID: id, Username: username, Domain: &domainName}
}

func TestHandleFollowCreated_LocalTarget(t *testing.T) {
	t.Parallel()
	sub, nc, _, _ := newTestSub()

	actor := localAccount("actor-1", "alice")
	target := localAccount("target-1", "bob")
	event := makeEvent(t, domain.EventFollowCreated, domain.FollowCreatedPayload{
		Follow: &domain.Follow{ID: "f-1", AccountID: actor.ID, TargetID: target.ID, State: "accepted"},
		Actor:  actor,
		Target: target,
	})

	sub.handleFollowCreated(context.Background(), event)

	require.Len(t, nc.calls, 1)
	assert.Equal(t, target.ID, nc.calls[0].RecipientID)
	assert.Equal(t, actor.ID, nc.calls[0].FromAccountID)
	assert.Equal(t, domain.NotificationTypeFollow, nc.calls[0].Type)
	assert.Nil(t, nc.calls[0].StatusID)
}

func TestHandleFollowCreated_SkipsRemoteTarget(t *testing.T) {
	t.Parallel()
	sub, nc, _, _ := newTestSub()

	actor := localAccount("actor-1", "alice")
	target := remoteAccount("target-1", "bob", "remote.example")
	event := makeEvent(t, domain.EventFollowCreated, domain.FollowCreatedPayload{
		Follow: &domain.Follow{ID: "f-1", AccountID: actor.ID, TargetID: target.ID, State: "accepted"},
		Actor:  actor,
		Target: target,
	})

	sub.handleFollowCreated(context.Background(), event)

	assert.Empty(t, nc.calls)
}

func TestHandleFollowRequested_LocalTarget(t *testing.T) {
	t.Parallel()
	sub, nc, _, _ := newTestSub()

	actor := remoteAccount("actor-1", "alice", "remote.example")
	target := localAccount("target-1", "bob")
	event := makeEvent(t, domain.EventFollowRequested, domain.FollowRequestedPayload{
		Follow: &domain.Follow{ID: "f-1", AccountID: actor.ID, TargetID: target.ID, State: "pending"},
		Actor:  actor,
		Target: target,
	})

	sub.handleFollowRequested(context.Background(), event)

	require.Len(t, nc.calls, 1)
	assert.Equal(t, target.ID, nc.calls[0].RecipientID)
	assert.Equal(t, actor.ID, nc.calls[0].FromAccountID)
	assert.Equal(t, domain.NotificationTypeFollowRequest, nc.calls[0].Type)
}

func TestHandleFavouriteCreated_LocalAuthor(t *testing.T) {
	t.Parallel()
	sub, nc, al, _ := newTestSub()

	author := localAccount("author-1", "bob")
	al.accounts[author.ID] = author

	faver := localAccount("faver-1", "alice")
	event := makeEvent(t, domain.EventFavouriteCreated, domain.FavouriteCreatedPayload{
		AccountID:      faver.ID,
		StatusID:       "status-1",
		StatusAuthorID: author.ID,
		FromAccount:    faver,
	})

	sub.handleFavouriteCreated(context.Background(), event)

	require.Len(t, nc.calls, 1)
	assert.Equal(t, author.ID, nc.calls[0].RecipientID)
	assert.Equal(t, faver.ID, nc.calls[0].FromAccountID)
	assert.Equal(t, domain.NotificationTypeFavourite, nc.calls[0].Type)
	require.NotNil(t, nc.calls[0].StatusID)
	assert.Equal(t, "status-1", *nc.calls[0].StatusID)
}

func TestHandleFavouriteCreated_SkipsSelfFavourite(t *testing.T) {
	t.Parallel()
	sub, nc, al, _ := newTestSub()

	author := localAccount("author-1", "bob")
	al.accounts[author.ID] = author

	event := makeEvent(t, domain.EventFavouriteCreated, domain.FavouriteCreatedPayload{
		AccountID:      author.ID,
		StatusID:       "status-1",
		StatusAuthorID: author.ID,
		FromAccount:    author,
	})

	sub.handleFavouriteCreated(context.Background(), event)

	assert.Empty(t, nc.calls)
}

func TestHandleReblogCreated_LocalAuthor(t *testing.T) {
	t.Parallel()
	sub, nc, al, _ := newTestSub()

	author := localAccount("author-1", "bob")
	al.accounts[author.ID] = author

	reblogger := localAccount("reblogger-1", "alice")
	event := makeEvent(t, domain.EventReblogCreated, domain.ReblogCreatedPayload{
		AccountID:        reblogger.ID,
		ReblogStatusID:   "reblog-status-1",
		OriginalStatusID: "status-1",
		OriginalAuthorID: author.ID,
		FromAccount:      reblogger,
	})

	sub.handleReblogCreated(context.Background(), event)

	require.Len(t, nc.calls, 1)
	assert.Equal(t, author.ID, nc.calls[0].RecipientID)
	assert.Equal(t, reblogger.ID, nc.calls[0].FromAccountID)
	assert.Equal(t, domain.NotificationTypeReblog, nc.calls[0].Type)
	require.NotNil(t, nc.calls[0].StatusID)
	assert.Equal(t, "status-1", *nc.calls[0].StatusID)
}

func TestHandleReblogCreated_SkipsSelfReblog(t *testing.T) {
	t.Parallel()
	sub, nc, al, _ := newTestSub()

	author := localAccount("author-1", "bob")
	al.accounts[author.ID] = author

	event := makeEvent(t, domain.EventReblogCreated, domain.ReblogCreatedPayload{
		AccountID:        author.ID,
		ReblogStatusID:   "reblog-status-1",
		OriginalStatusID: "status-1",
		OriginalAuthorID: author.ID,
		FromAccount:      author,
	})

	sub.handleReblogCreated(context.Background(), event)

	assert.Empty(t, nc.calls)
}

func TestHandleStatusCreatedMentions_LocalMentioned(t *testing.T) {
	t.Parallel()
	sub, nc, _, _ := newTestSub()

	author := localAccount("author-1", "alice")
	mentioned := localAccount("mentioned-1", "bob")
	status := &domain.Status{ID: "status-1", AccountID: author.ID}

	event := makeEvent(t, domain.EventStatusCreated, domain.StatusCreatedPayload{
		Status:   status,
		Author:   author,
		Mentions: []*domain.Account{mentioned},
	})

	sub.handleStatusCreatedMentions(context.Background(), event)

	require.Len(t, nc.calls, 1)
	assert.Equal(t, mentioned.ID, nc.calls[0].RecipientID)
	assert.Equal(t, author.ID, nc.calls[0].FromAccountID)
	assert.Equal(t, domain.NotificationTypeMention, nc.calls[0].Type)
	require.NotNil(t, nc.calls[0].StatusID)
	assert.Equal(t, status.ID, *nc.calls[0].StatusID)
}

func TestHandleStatusCreatedMentions_SkipsAuthorMention(t *testing.T) {
	t.Parallel()
	sub, nc, _, _ := newTestSub()

	author := localAccount("author-1", "alice")
	status := &domain.Status{ID: "status-1", AccountID: author.ID}

	event := makeEvent(t, domain.EventStatusCreated, domain.StatusCreatedPayload{
		Status:   status,
		Author:   author,
		Mentions: []*domain.Account{author},
	})

	sub.handleStatusCreatedMentions(context.Background(), event)

	assert.Empty(t, nc.calls)
}

func TestHandleStatusCreatedMentions_SkipsMutedConversation(t *testing.T) {
	t.Parallel()
	sub, nc, _, cm := newTestSub()

	cm.muted["mentioned-1:status-1"] = true

	author := localAccount("author-1", "alice")
	mentioned := localAccount("mentioned-1", "bob")
	status := &domain.Status{ID: "status-1", AccountID: author.ID}

	event := makeEvent(t, domain.EventStatusCreated, domain.StatusCreatedPayload{
		Status:   status,
		Author:   author,
		Mentions: []*domain.Account{mentioned},
	})

	sub.handleStatusCreatedMentions(context.Background(), event)

	assert.Empty(t, nc.calls)
}
