package events

import (
	"context"
	"encoding/json"
	"testing"
	"time"

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

// ── Fakes for policy filtering ──────────────────────────────────────────────

type fakeFollowChecker struct {
	// follows maps "actorID:targetID" → *domain.Follow
	follows map[string]*domain.Follow
}

func (f *fakeFollowChecker) GetFollow(_ context.Context, actorID, targetID string) (*domain.Follow, error) {
	fol, ok := f.follows[actorID+":"+targetID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return fol, nil
}

type fakeNotifPolicyProvider struct {
	policies map[string]*domain.NotificationPolicy
	requests []notifRequestCall
}

type notifRequestCall struct {
	AccountID     string
	FromAccountID string
	LastStatusID  *string
}

func (f *fakeNotifPolicyProvider) GetOrCreatePolicy(_ context.Context, accountID string) (*domain.NotificationPolicy, error) {
	p, ok := f.policies[accountID]
	if !ok {
		return &domain.NotificationPolicy{AccountID: accountID}, nil
	}
	return p, nil
}

func (f *fakeNotifPolicyProvider) UpsertNotificationRequest(_ context.Context, accountID, fromAccountID string, lastStatusID *string) error {
	f.requests = append(f.requests, notifRequestCall{
		AccountID:     accountID,
		FromAccountID: fromAccountID,
		LastStatusID:  lastStatusID,
	})
	return nil
}

func newTestSubWithPolicy() (*NotificationSubscriber, *fakeNotifCreator, *fakeAccountLookup, *fakeFollowChecker, *fakeNotifPolicyProvider) {
	nc := &fakeNotifCreator{}
	al := &fakeAccountLookup{accounts: make(map[string]*domain.Account)}
	cm := &fakeConversationMuteChecker{muted: make(map[string]bool)}
	fc := &fakeFollowChecker{follows: make(map[string]*domain.Follow)}
	pp := &fakeNotifPolicyProvider{policies: make(map[string]*domain.NotificationPolicy)}
	sub := &NotificationSubscriber{deps: NotificationDeps{
		Notifications:      nc,
		Accounts:           al,
		Conversations:      cm,
		Follows:            fc,
		NotificationPolicy: pp,
	}}
	return sub, nc, al, fc, pp
}

func acceptedFollow(actorID, targetID string) *domain.Follow {
	return &domain.Follow{ID: "f-1", AccountID: actorID, TargetID: targetID, State: domain.FollowStateAccepted}
}

// ── Policy filtering tests ──────────────────────────────────────────────────

func TestShouldFilter_NoFiltersEnabled(t *testing.T) {
	t.Parallel()
	sub, nc, _, _, pp := newTestSubWithPolicy()
	pp.policies["recipient-1"] = &domain.NotificationPolicy{AccountID: "recipient-1"}

	actor := localAccount("actor-1", "alice")
	target := localAccount("recipient-1", "bob")
	event := makeEvent(t, domain.EventFollowCreated, domain.FollowCreatedPayload{
		Follow: &domain.Follow{ID: "f-1", AccountID: actor.ID, TargetID: target.ID, State: "accepted"},
		Actor:  actor,
		Target: target,
	})
	sub.handleFollowCreated(context.Background(), event)

	require.Len(t, nc.calls, 1, "notification should be created when no filters are enabled")
	assert.Empty(t, pp.requests)
}

func TestShouldFilter_FilterNotFollowing(t *testing.T) {
	t.Parallel()
	sub, nc, al, fc, pp := newTestSubWithPolicy()

	pp.policies["recipient-1"] = &domain.NotificationPolicy{
		AccountID:          "recipient-1",
		FilterNotFollowing: true,
	}
	author := localAccount("recipient-1", "bob")
	al.accounts[author.ID] = author
	faver := localAccount("faver-1", "alice")
	al.accounts[faver.ID] = faver

	t.Run("filtered when recipient does not follow sender", func(t *testing.T) {
		event := makeEvent(t, domain.EventFavouriteCreated, domain.FavouriteCreatedPayload{
			AccountID:      faver.ID,
			StatusID:       "status-1",
			StatusAuthorID: author.ID,
			FromAccount:    faver,
		})
		sub.handleFavouriteCreated(context.Background(), event)

		assert.Empty(t, nc.calls)
		require.Len(t, pp.requests, 1)
		assert.Equal(t, author.ID, pp.requests[0].AccountID)
		assert.Equal(t, faver.ID, pp.requests[0].FromAccountID)
	})

	// Reset
	nc.calls = nil
	pp.requests = nil

	t.Run("allowed when recipient follows sender", func(t *testing.T) {
		fc.follows["recipient-1:faver-1"] = acceptedFollow("recipient-1", "faver-1")

		event := makeEvent(t, domain.EventFavouriteCreated, domain.FavouriteCreatedPayload{
			AccountID:      faver.ID,
			StatusID:       "status-2",
			StatusAuthorID: author.ID,
			FromAccount:    faver,
		})
		sub.handleFavouriteCreated(context.Background(), event)

		require.Len(t, nc.calls, 1)
		assert.Empty(t, pp.requests)
	})
}

func TestShouldFilter_FilterNotFollowers(t *testing.T) {
	t.Parallel()
	sub, nc, al, fc, pp := newTestSubWithPolicy()

	pp.policies["recipient-1"] = &domain.NotificationPolicy{
		AccountID:          "recipient-1",
		FilterNotFollowers: true,
	}
	author := localAccount("recipient-1", "bob")
	al.accounts[author.ID] = author
	reblogger := localAccount("reblogger-1", "alice")
	al.accounts[reblogger.ID] = reblogger

	t.Run("filtered when sender does not follow recipient", func(t *testing.T) {
		event := makeEvent(t, domain.EventReblogCreated, domain.ReblogCreatedPayload{
			AccountID:        reblogger.ID,
			ReblogStatusID:   "reblog-1",
			OriginalStatusID: "status-1",
			OriginalAuthorID: author.ID,
			FromAccount:      reblogger,
		})
		sub.handleReblogCreated(context.Background(), event)

		assert.Empty(t, nc.calls)
		require.Len(t, pp.requests, 1)
	})

	nc.calls = nil
	pp.requests = nil

	t.Run("allowed when sender follows recipient", func(t *testing.T) {
		fc.follows["reblogger-1:recipient-1"] = acceptedFollow("reblogger-1", "recipient-1")

		event := makeEvent(t, domain.EventReblogCreated, domain.ReblogCreatedPayload{
			AccountID:        reblogger.ID,
			ReblogStatusID:   "reblog-2",
			OriginalStatusID: "status-2",
			OriginalAuthorID: author.ID,
			FromAccount:      reblogger,
		})
		sub.handleReblogCreated(context.Background(), event)

		require.Len(t, nc.calls, 1)
		assert.Empty(t, pp.requests)
	})
}

func TestShouldFilter_FilterNewAccounts(t *testing.T) {
	t.Parallel()
	sub, nc, al, _, pp := newTestSubWithPolicy()

	pp.policies["recipient-1"] = &domain.NotificationPolicy{
		AccountID:         "recipient-1",
		FilterNewAccounts: true,
	}
	recipient := localAccount("recipient-1", "bob")
	al.accounts[recipient.ID] = recipient

	t.Run("filtered when sender account is new", func(t *testing.T) {
		newActor := &domain.Account{ID: "new-1", Username: "newbie", CreatedAt: time.Now().Add(-7 * 24 * time.Hour)}
		al.accounts[newActor.ID] = newActor

		event := makeEvent(t, domain.EventFollowCreated, domain.FollowCreatedPayload{
			Follow: &domain.Follow{ID: "f-1", AccountID: newActor.ID, TargetID: recipient.ID, State: "accepted"},
			Actor:  newActor,
			Target: recipient,
		})
		sub.handleFollowCreated(context.Background(), event)

		assert.Empty(t, nc.calls)
		require.Len(t, pp.requests, 1)
	})

	nc.calls = nil
	pp.requests = nil

	t.Run("allowed when sender account is old", func(t *testing.T) {
		oldActor := &domain.Account{ID: "old-1", Username: "veteran", CreatedAt: time.Now().Add(-90 * 24 * time.Hour)}
		al.accounts[oldActor.ID] = oldActor

		event := makeEvent(t, domain.EventFollowCreated, domain.FollowCreatedPayload{
			Follow: &domain.Follow{ID: "f-2", AccountID: oldActor.ID, TargetID: recipient.ID, State: "accepted"},
			Actor:  oldActor,
			Target: recipient,
		})
		sub.handleFollowCreated(context.Background(), event)

		require.Len(t, nc.calls, 1)
		assert.Empty(t, pp.requests)
	})
}

func TestShouldFilter_FilterPrivateMentions(t *testing.T) {
	t.Parallel()
	sub, nc, _, _, pp := newTestSubWithPolicy()

	pp.policies["mentioned-1"] = &domain.NotificationPolicy{
		AccountID:             "mentioned-1",
		FilterPrivateMentions: true,
	}

	author := localAccount("author-1", "alice")
	mentioned := localAccount("mentioned-1", "bob")

	t.Run("filtered for direct mention", func(t *testing.T) {
		status := &domain.Status{ID: "status-1", AccountID: author.ID, Visibility: domain.VisibilityDirect}
		event := makeEvent(t, domain.EventStatusCreated, domain.StatusCreatedPayload{
			Status:   status,
			Author:   author,
			Mentions: []*domain.Account{mentioned},
		})
		sub.handleStatusCreatedMentions(context.Background(), event)

		assert.Empty(t, nc.calls)
		require.Len(t, pp.requests, 1)
		assert.Equal(t, mentioned.ID, pp.requests[0].AccountID)
	})

	nc.calls = nil
	pp.requests = nil

	t.Run("allowed for public mention", func(t *testing.T) {
		status := &domain.Status{ID: "status-2", AccountID: author.ID, Visibility: domain.VisibilityPublic}
		event := makeEvent(t, domain.EventStatusCreated, domain.StatusCreatedPayload{
			Status:   status,
			Author:   author,
			Mentions: []*domain.Account{mentioned},
		})
		sub.handleStatusCreatedMentions(context.Background(), event)

		require.Len(t, nc.calls, 1)
		assert.Empty(t, pp.requests)
	})
}

func TestShouldFilter_NoPolicyProvider_AllowsNotification(t *testing.T) {
	t.Parallel()
	// Use newTestSub which doesn't set NotificationPolicy
	sub, nc, _, _ := newTestSub()

	actor := localAccount("actor-1", "alice")
	target := localAccount("target-1", "bob")
	event := makeEvent(t, domain.EventFollowCreated, domain.FollowCreatedPayload{
		Follow: &domain.Follow{ID: "f-1", AccountID: actor.ID, TargetID: target.ID, State: "accepted"},
		Actor:  actor,
		Target: target,
	})
	sub.handleFollowCreated(context.Background(), event)

	require.Len(t, nc.calls, 1, "notification should be created when no policy provider is set")
}
