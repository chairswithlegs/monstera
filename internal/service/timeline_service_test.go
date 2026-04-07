package service

import (
	"context"
	"testing"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/chairswithlegs/monstera/internal/uid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimelineService_HomeEnriched_empty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	timelineSvc := NewTimelineService(fake, accountSvc, statusSvc, nil)

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)

	enriched, err := timelineSvc.HomeEnriched(ctx, acc.ID, nil, 10)
	require.NoError(t, err)
	assert.Empty(t, enriched)
}

func TestTimelineService_HomeEnriched_one_status(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)
	timelineSvc := NewTimelineService(fake, accountSvc, statusSvc, nil)

	acc, err := accountSvc.Register(ctx, RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
	})
	require.NoError(t, err)

	text := "Hello world"
	_, err = statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  acc.ID,
		Text:       text,
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)

	enriched, err := timelineSvc.HomeEnriched(ctx, acc.ID, nil, 10)
	require.NoError(t, err)
	require.Len(t, enriched, 1)
	assert.Equal(t, "Hello world", *enriched[0].Status.Text)
	assert.Equal(t, acc.ID, enriched[0].Status.AccountID)
	require.NotNil(t, enriched[0].Author)
	assert.Equal(t, "alice", enriched[0].Author.Username)
	assert.Empty(t, enriched[0].Mentions)
	assert.Empty(t, enriched[0].Tags)
	assert.Empty(t, enriched[0].Media)
}

func TestTimelineService_ListTimelineEnriched_excludes_private_status_when_list_owner_does_not_follow_author(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)
	timelineSvc := NewTimelineService(fake, accountSvc, statusSvc, nil)

	alice, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	bob, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	listID := uid.New()
	_, err = fake.CreateList(ctx, store.CreateListInput{
		ID:            listID,
		AccountID:     alice.ID,
		Title:         "My list",
		RepliesPolicy: "list",
		Exclusive:     false,
	})
	require.NoError(t, err)
	err = fake.AddAccountToList(ctx, listID, bob.ID)
	require.NoError(t, err)

	privText := "private post"
	_, err = statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  bob.ID,
		Text:       privText,
		Visibility: domain.VisibilityPrivate,
	})
	require.NoError(t, err)

	enriched, err := timelineSvc.ListTimelineEnriched(ctx, alice.ID, listID, nil, 10)
	require.NoError(t, err)
	assert.Empty(t, enriched, "list owner should not see private status from list member they do not follow")
}

func TestTimelineService_ListTimelineEnriched_includes_private_status_when_list_owner_follows_author(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)
	timelineSvc := NewTimelineService(fake, accountSvc, statusSvc, nil)

	alice, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	bob, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	listID := uid.New()
	_, err = fake.CreateList(ctx, store.CreateListInput{
		ID:            listID,
		AccountID:     alice.ID,
		Title:         "My list",
		RepliesPolicy: "list",
		Exclusive:     false,
	})
	require.NoError(t, err)
	err = fake.AddAccountToList(ctx, listID, bob.ID)
	require.NoError(t, err)

	_, err = fake.CreateFollow(ctx, store.CreateFollowInput{
		ID:        uid.New(),
		AccountID: alice.ID,
		TargetID:  bob.ID,
		State:     domain.FollowStateAccepted,
		APID:      nil,
	})
	require.NoError(t, err)

	privText := "private post"
	_, err = statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  bob.ID,
		Text:       privText,
		Visibility: domain.VisibilityPrivate,
	})
	require.NoError(t, err)

	enriched, err := timelineSvc.ListTimelineEnriched(ctx, alice.ID, listID, nil, 10)
	require.NoError(t, err)
	require.Len(t, enriched, 1)
	assert.Equal(t, "private post", *enriched[0].Status.Text)
	assert.Equal(t, domain.VisibilityPrivate, enriched[0].Status.Visibility)
	assert.Equal(t, bob.ID, enriched[0].Status.AccountID)
}

// --- replies_policy tests ---

func newTimelineTestEnv(t *testing.T) (context.Context, *testutil.FakeStore, AccountService, StatusWriteService, TimelineService) {
	t.Helper()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)
	timelineSvc := NewTimelineService(fake, accountSvc, statusSvc, nil)
	return ctx, fake, accountSvc, statusWriteSvc, timelineSvc
}

func TestTimelineService_ListTimelineEnriched_RepliesPolicy_None_ExcludesReplies(t *testing.T) {
	t.Parallel()
	ctx, fake, accountSvc, statusWriteSvc, timelineSvc := newTimelineTestEnv(t)

	alice, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	bob, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)
	carol, err := accountSvc.Create(ctx, CreateAccountInput{Username: "carol"})
	require.NoError(t, err)

	// Alice follows bob and carol so she can see their statuses.
	_, err = fake.CreateFollow(ctx, store.CreateFollowInput{ID: uid.New(), AccountID: alice.ID, TargetID: bob.ID, State: domain.FollowStateAccepted})
	require.NoError(t, err)
	_, err = fake.CreateFollow(ctx, store.CreateFollowInput{ID: uid.New(), AccountID: alice.ID, TargetID: carol.ID, State: domain.FollowStateAccepted})
	require.NoError(t, err)

	listID := uid.New()
	_, err = fake.CreateList(ctx, store.CreateListInput{
		ID: listID, AccountID: alice.ID, Title: "None policy", RepliesPolicy: domain.ListRepliesPolicyNone,
	})
	require.NoError(t, err)
	require.NoError(t, fake.AddAccountToList(ctx, listID, bob.ID))

	// Bob creates a non-reply status.
	_, err = statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID: bob.ID, Username: bob.Username, Text: "hello", Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)

	// Carol creates a status, then Bob replies to it.
	carolStatus, err := statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID: carol.ID, Username: carol.Username, Text: "original", Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)
	_, err = statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID: bob.ID, Username: bob.Username, Text: "reply", Visibility: domain.VisibilityPublic, InReplyToID: &carolStatus.Status.ID,
	})
	require.NoError(t, err)

	enriched, err := timelineSvc.ListTimelineEnriched(ctx, alice.ID, listID, nil, 10)
	require.NoError(t, err)
	require.Len(t, enriched, 1, "only non-reply should appear with none policy")
	assert.Equal(t, "hello", *enriched[0].Status.Text)
}

func TestTimelineService_ListTimelineEnriched_RepliesPolicy_List_IncludesRepliesToMembers(t *testing.T) {
	t.Parallel()
	ctx, fake, accountSvc, statusWriteSvc, timelineSvc := newTimelineTestEnv(t)

	alice, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	bob, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)
	carol, err := accountSvc.Create(ctx, CreateAccountInput{Username: "carol"})
	require.NoError(t, err)

	_, err = fake.CreateFollow(ctx, store.CreateFollowInput{ID: uid.New(), AccountID: alice.ID, TargetID: bob.ID, State: domain.FollowStateAccepted})
	require.NoError(t, err)
	_, err = fake.CreateFollow(ctx, store.CreateFollowInput{ID: uid.New(), AccountID: alice.ID, TargetID: carol.ID, State: domain.FollowStateAccepted})
	require.NoError(t, err)

	listID := uid.New()
	_, err = fake.CreateList(ctx, store.CreateListInput{
		ID: listID, AccountID: alice.ID, Title: "List policy", RepliesPolicy: domain.ListRepliesPolicyList,
	})
	require.NoError(t, err)
	require.NoError(t, fake.AddAccountToList(ctx, listID, bob.ID))
	require.NoError(t, fake.AddAccountToList(ctx, listID, carol.ID))

	// Carol creates a status, Bob replies to Carol (a list member).
	carolStatus, err := statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID: carol.ID, Username: carol.Username, Text: "original", Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)
	_, err = statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID: bob.ID, Username: bob.Username, Text: "reply to member", Visibility: domain.VisibilityPublic, InReplyToID: &carolStatus.Status.ID,
	})
	require.NoError(t, err)

	enriched, err := timelineSvc.ListTimelineEnriched(ctx, alice.ID, listID, nil, 10)
	require.NoError(t, err)

	texts := make([]string, 0, len(enriched))
	for _, e := range enriched {
		texts = append(texts, *e.Status.Text)
	}
	assert.Contains(t, texts, "reply to member", "reply to list member should be included")
}

func TestTimelineService_ListTimelineEnriched_RepliesPolicy_List_ExcludesRepliesToNonMembers(t *testing.T) {
	t.Parallel()
	ctx, fake, accountSvc, statusWriteSvc, timelineSvc := newTimelineTestEnv(t)

	alice, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	bob, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)
	dave, err := accountSvc.Create(ctx, CreateAccountInput{Username: "dave"})
	require.NoError(t, err)

	_, err = fake.CreateFollow(ctx, store.CreateFollowInput{ID: uid.New(), AccountID: alice.ID, TargetID: bob.ID, State: domain.FollowStateAccepted})
	require.NoError(t, err)

	listID := uid.New()
	_, err = fake.CreateList(ctx, store.CreateListInput{
		ID: listID, AccountID: alice.ID, Title: "List policy", RepliesPolicy: domain.ListRepliesPolicyList,
	})
	require.NoError(t, err)
	require.NoError(t, fake.AddAccountToList(ctx, listID, bob.ID))
	// dave is NOT a member

	daveStatus, err := statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID: dave.ID, Username: dave.Username, Text: "dave post", Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)
	_, err = statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID: bob.ID, Username: bob.Username, Text: "reply to non-member", Visibility: domain.VisibilityPublic, InReplyToID: &daveStatus.Status.ID,
	})
	require.NoError(t, err)

	enriched, err := timelineSvc.ListTimelineEnriched(ctx, alice.ID, listID, nil, 10)
	require.NoError(t, err)
	for _, e := range enriched {
		assert.NotEqual(t, "reply to non-member", *e.Status.Text, "reply to non-member should be excluded")
	}
}

func TestTimelineService_ListTimelineEnriched_RepliesPolicy_Followed_IncludesRepliesToFollowed(t *testing.T) {
	t.Parallel()
	ctx, fake, accountSvc, statusWriteSvc, timelineSvc := newTimelineTestEnv(t)

	alice, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	bob, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)
	carol, err := accountSvc.Create(ctx, CreateAccountInput{Username: "carol"})
	require.NoError(t, err)

	// Alice follows both bob and carol.
	_, err = fake.CreateFollow(ctx, store.CreateFollowInput{ID: uid.New(), AccountID: alice.ID, TargetID: bob.ID, State: domain.FollowStateAccepted})
	require.NoError(t, err)
	_, err = fake.CreateFollow(ctx, store.CreateFollowInput{ID: uid.New(), AccountID: alice.ID, TargetID: carol.ID, State: domain.FollowStateAccepted})
	require.NoError(t, err)

	listID := uid.New()
	_, err = fake.CreateList(ctx, store.CreateListInput{
		ID: listID, AccountID: alice.ID, Title: "Followed policy", RepliesPolicy: domain.ListRepliesPolicyFollowed,
	})
	require.NoError(t, err)
	require.NoError(t, fake.AddAccountToList(ctx, listID, bob.ID))

	carolStatus, err := statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID: carol.ID, Username: carol.Username, Text: "carol post", Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)
	_, err = statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID: bob.ID, Username: bob.Username, Text: "reply to followed", Visibility: domain.VisibilityPublic, InReplyToID: &carolStatus.Status.ID,
	})
	require.NoError(t, err)

	enriched, err := timelineSvc.ListTimelineEnriched(ctx, alice.ID, listID, nil, 10)
	require.NoError(t, err)

	texts := make([]string, 0, len(enriched))
	for _, e := range enriched {
		texts = append(texts, *e.Status.Text)
	}
	assert.Contains(t, texts, "reply to followed", "reply to followed account should be included")
}

func TestTimelineService_ListTimelineEnriched_RepliesPolicy_Followed_ExcludesRepliesToNotFollowed(t *testing.T) {
	t.Parallel()
	ctx, fake, accountSvc, statusWriteSvc, timelineSvc := newTimelineTestEnv(t)

	alice, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	bob, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)
	dave, err := accountSvc.Create(ctx, CreateAccountInput{Username: "dave"})
	require.NoError(t, err)

	// Alice follows bob but NOT dave.
	_, err = fake.CreateFollow(ctx, store.CreateFollowInput{ID: uid.New(), AccountID: alice.ID, TargetID: bob.ID, State: domain.FollowStateAccepted})
	require.NoError(t, err)

	listID := uid.New()
	_, err = fake.CreateList(ctx, store.CreateListInput{
		ID: listID, AccountID: alice.ID, Title: "Followed policy", RepliesPolicy: domain.ListRepliesPolicyFollowed,
	})
	require.NoError(t, err)
	require.NoError(t, fake.AddAccountToList(ctx, listID, bob.ID))

	daveStatus, err := statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID: dave.ID, Username: dave.Username, Text: "dave post", Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)
	_, err = statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID: bob.ID, Username: bob.Username, Text: "reply to not followed", Visibility: domain.VisibilityPublic, InReplyToID: &daveStatus.Status.ID,
	})
	require.NoError(t, err)

	enriched, err := timelineSvc.ListTimelineEnriched(ctx, alice.ID, listID, nil, 10)
	require.NoError(t, err)
	for _, e := range enriched {
		assert.NotEqual(t, "reply to not followed", *e.Status.Text, "reply to unfollowed account should be excluded")
	}
}

func TestTimelineService_ListTimelineEnriched_NonRepliesAlwaysIncluded(t *testing.T) {
	t.Parallel()
	policies := []string{domain.ListRepliesPolicyNone, domain.ListRepliesPolicyList, domain.ListRepliesPolicyFollowed}

	for _, policy := range policies {
		t.Run(policy, func(t *testing.T) {
			t.Parallel()
			ctx, fake, accountSvc, statusWriteSvc, timelineSvc := newTimelineTestEnv(t)

			alice, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
			require.NoError(t, err)
			bob, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
			require.NoError(t, err)

			_, err = fake.CreateFollow(ctx, store.CreateFollowInput{ID: uid.New(), AccountID: alice.ID, TargetID: bob.ID, State: domain.FollowStateAccepted})
			require.NoError(t, err)

			listID := uid.New()
			_, err = fake.CreateList(ctx, store.CreateListInput{
				ID: listID, AccountID: alice.ID, Title: "Test", RepliesPolicy: policy,
			})
			require.NoError(t, err)
			require.NoError(t, fake.AddAccountToList(ctx, listID, bob.ID))

			_, err = statusWriteSvc.Create(ctx, CreateStatusInput{
				AccountID: bob.ID, Username: bob.Username, Text: "non-reply", Visibility: domain.VisibilityPublic,
			})
			require.NoError(t, err)

			enriched, err := timelineSvc.ListTimelineEnriched(ctx, alice.ID, listID, nil, 10)
			require.NoError(t, err)
			require.Len(t, enriched, 1, "non-reply should always be included for policy %s", policy)
			assert.Equal(t, "non-reply", *enriched[0].Status.Text)
		})
	}
}

// --- exclusive flag tests ---

func TestTimelineService_HomeEnriched_ExclusiveList_ExcludesFromHome(t *testing.T) {
	t.Parallel()
	ctx, fake, accountSvc, statusWriteSvc, timelineSvc := newTimelineTestEnv(t)

	alice, err := accountSvc.Register(ctx, RegisterInput{Username: "alice", Email: "alice@example.com", Password: "hash"})
	require.NoError(t, err)
	bob, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	_, err = fake.CreateFollow(ctx, store.CreateFollowInput{ID: uid.New(), AccountID: alice.ID, TargetID: bob.ID, State: domain.FollowStateAccepted})
	require.NoError(t, err)

	listID := uid.New()
	_, err = fake.CreateList(ctx, store.CreateListInput{
		ID: listID, AccountID: alice.ID, Title: "Exclusive", RepliesPolicy: domain.ListRepliesPolicyList, Exclusive: true,
	})
	require.NoError(t, err)
	require.NoError(t, fake.AddAccountToList(ctx, listID, bob.ID))

	_, err = statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID: bob.ID, Username: bob.Username, Text: "exclusive post", Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)

	enriched, err := timelineSvc.HomeEnriched(ctx, alice.ID, nil, 10)
	require.NoError(t, err)
	assert.Empty(t, enriched, "status from exclusive list member should not appear in home timeline")
}

func TestTimelineService_HomeEnriched_NonExclusiveList_StillAppearsInHome(t *testing.T) {
	t.Parallel()
	ctx, fake, accountSvc, statusWriteSvc, timelineSvc := newTimelineTestEnv(t)

	alice, err := accountSvc.Register(ctx, RegisterInput{Username: "alice", Email: "alice@example.com", Password: "hash"})
	require.NoError(t, err)
	bob, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	_, err = fake.CreateFollow(ctx, store.CreateFollowInput{ID: uid.New(), AccountID: alice.ID, TargetID: bob.ID, State: domain.FollowStateAccepted})
	require.NoError(t, err)

	listID := uid.New()
	_, err = fake.CreateList(ctx, store.CreateListInput{
		ID: listID, AccountID: alice.ID, Title: "Non-exclusive", RepliesPolicy: domain.ListRepliesPolicyList, Exclusive: false,
	})
	require.NoError(t, err)
	require.NoError(t, fake.AddAccountToList(ctx, listID, bob.ID))

	_, err = statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID: bob.ID, Username: bob.Username, Text: "normal post", Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)

	enriched, err := timelineSvc.HomeEnriched(ctx, alice.ID, nil, 10)
	require.NoError(t, err)
	require.Len(t, enriched, 1, "status from non-exclusive list member should appear in home timeline")
	assert.Equal(t, "normal post", *enriched[0].Status.Text)
}

func TestTimelineService_ExclusiveList_StillAppearsInListTimeline(t *testing.T) {
	t.Parallel()
	ctx, fake, accountSvc, statusWriteSvc, timelineSvc := newTimelineTestEnv(t)

	alice, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	bob, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	_, err = fake.CreateFollow(ctx, store.CreateFollowInput{ID: uid.New(), AccountID: alice.ID, TargetID: bob.ID, State: domain.FollowStateAccepted})
	require.NoError(t, err)

	listID := uid.New()
	_, err = fake.CreateList(ctx, store.CreateListInput{
		ID: listID, AccountID: alice.ID, Title: "Exclusive", RepliesPolicy: domain.ListRepliesPolicyList, Exclusive: true,
	})
	require.NoError(t, err)
	require.NoError(t, fake.AddAccountToList(ctx, listID, bob.ID))

	_, err = statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID: bob.ID, Username: bob.Username, Text: "exclusive post", Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)

	enriched, err := timelineSvc.ListTimelineEnriched(ctx, alice.ID, listID, nil, 10)
	require.NoError(t, err)
	require.Len(t, enriched, 1, "status should still appear in the exclusive list timeline")
	assert.Equal(t, "exclusive post", *enriched[0].Status.Text)
}

func TestTimelineService_HomeEnriched_OwnStatuses_NotExcluded(t *testing.T) {
	t.Parallel()
	ctx, fake, accountSvc, statusWriteSvc, timelineSvc := newTimelineTestEnv(t)

	alice, err := accountSvc.Register(ctx, RegisterInput{Username: "alice", Email: "alice@example.com", Password: "hash"})
	require.NoError(t, err)
	bob, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	_, err = fake.CreateFollow(ctx, store.CreateFollowInput{ID: uid.New(), AccountID: alice.ID, TargetID: bob.ID, State: domain.FollowStateAccepted})
	require.NoError(t, err)

	// Alice has an exclusive list with bob. Bob's posts should be excluded,
	// but alice's own posts should still appear.
	listID := uid.New()
	_, err = fake.CreateList(ctx, store.CreateListInput{
		ID: listID, AccountID: alice.ID, Title: "Exclusive", RepliesPolicy: domain.ListRepliesPolicyList, Exclusive: true,
	})
	require.NoError(t, err)
	require.NoError(t, fake.AddAccountToList(ctx, listID, bob.ID))

	_, err = statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID: bob.ID, Username: bob.Username, Text: "bob post", Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)
	_, err = statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID: alice.ID, Username: alice.Username, Text: "my own post", Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)

	enriched, err := timelineSvc.HomeEnriched(ctx, alice.ID, nil, 10)
	require.NoError(t, err)
	require.Len(t, enriched, 1, "only alice's own status should appear; bob's should be excluded")
	assert.Equal(t, "my own post", *enriched[0].Status.Text)
}

// ── Silenced domain filtering tests ────────────────────────────────────────

type fakeSilenceChecker struct {
	silenced map[string]bool
}

func (f *fakeSilenceChecker) IsSilenced(_ context.Context, domain string) bool {
	return f.silenced[domain]
}

func TestTimelineService_PublicLocalEnriched_filters_silenced_domain(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	sc := &fakeSilenceChecker{silenced: map[string]bool{"silenced.example": true}}
	timelineSvc := NewTimelineService(fake, accountSvc, statusSvc, sc)

	// Create a local account.
	local, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)

	// Create a remote account on a silenced domain.
	silencedDomain := "silenced.example"
	remoteAcc, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID:       uid.New(),
		Username: "bob",
		Domain:   &silencedDomain,
		APID:     "https://silenced.example/users/bob",
	})
	require.NoError(t, err)

	// Create a public status from each account.
	localText := "local post"
	_, err = fake.CreateStatus(ctx, store.CreateStatusInput{
		ID:         uid.New(),
		URI:        "https://example.com/statuses/1",
		AccountID:  local.ID,
		Text:       &localText,
		Visibility: domain.VisibilityPublic,
		Local:      true,
	})
	require.NoError(t, err)

	remoteText := "remote silenced post"
	_, err = fake.CreateStatus(ctx, store.CreateStatusInput{
		ID:         uid.New(),
		URI:        "https://silenced.example/statuses/1",
		AccountID:  remoteAcc.ID,
		Text:       &remoteText,
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)

	enriched, err := timelineSvc.PublicLocalEnriched(ctx, false, nil, nil, 10)
	require.NoError(t, err)
	require.Len(t, enriched, 1, "silenced domain status should be filtered")
	assert.Equal(t, "local post", *enriched[0].Status.Text)
}

func TestTimelineService_PublicLocalEnriched_allows_non_silenced_remote(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	sc := &fakeSilenceChecker{silenced: map[string]bool{"silenced.example": true}}
	timelineSvc := NewTimelineService(fake, accountSvc, statusSvc, sc)

	// Create a remote account on a non-silenced domain.
	okDomain := "ok.example"
	remoteAcc, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID:       uid.New(),
		Username: "carol",
		Domain:   &okDomain,
		APID:     "https://ok.example/users/carol",
	})
	require.NoError(t, err)

	text := "non-silenced remote post"
	_, err = fake.CreateStatus(ctx, store.CreateStatusInput{
		ID:         uid.New(),
		URI:        "https://ok.example/statuses/1",
		AccountID:  remoteAcc.ID,
		Text:       &text,
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)

	enriched, err := timelineSvc.PublicLocalEnriched(ctx, false, nil, nil, 10)
	require.NoError(t, err)
	require.Len(t, enriched, 1, "non-silenced remote status should appear")
	assert.Equal(t, "non-silenced remote post", *enriched[0].Status.Text)
}

func TestTimelineService_HomeEnriched_does_not_filter_silenced_domain(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	sc := &fakeSilenceChecker{silenced: map[string]bool{"silenced.example": true}}
	timelineSvc := NewTimelineService(fake, accountSvc, statusSvc, sc)

	alice, err := accountSvc.Register(ctx, RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
	})
	require.NoError(t, err)

	silencedDomain := "silenced.example"
	remoteAcc, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID:       uid.New(),
		Username: "bob",
		Domain:   &silencedDomain,
		APID:     "https://silenced.example/users/bob",
	})
	require.NoError(t, err)

	// Alice follows the silenced account.
	_, err = fake.CreateFollow(ctx, store.CreateFollowInput{
		ID:        uid.New(),
		AccountID: alice.ID,
		TargetID:  remoteAcc.ID,
		State:     domain.FollowStateAccepted,
	})
	require.NoError(t, err)

	remoteText := "silenced but followed post"
	_, err = fake.CreateStatus(ctx, store.CreateStatusInput{
		ID:         uid.New(),
		URI:        "https://silenced.example/statuses/1",
		AccountID:  remoteAcc.ID,
		Text:       &remoteText,
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)

	enriched, err := timelineSvc.HomeEnriched(ctx, alice.ID, nil, 10)
	require.NoError(t, err)
	require.Len(t, enriched, 1, "silenced domain statuses should still appear on home timeline")
	assert.Equal(t, "silenced but followed post", *enriched[0].Status.Text)
}
