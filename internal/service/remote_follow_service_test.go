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

func TestRemoteFollowService_AcceptFollow_increments_counts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc, _, remoteFollowSvc := newFollowTestServices(fake)

	actor, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	target, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	follow, err := remoteFollowSvc.CreateRemoteFollow(ctx, actor.ID, target.ID, domain.FollowStatePending, nil)
	require.NoError(t, err)
	require.NotNil(t, follow)

	err = remoteFollowSvc.AcceptFollow(ctx, follow.ID)
	require.NoError(t, err)

	targetAfter, err := fake.GetAccountByID(ctx, target.ID)
	require.NoError(t, err)
	actorAfter, err := fake.GetAccountByID(ctx, actor.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, targetAfter.FollowersCount, "target's followers_count should be incremented")
	assert.Equal(t, 1, actorAfter.FollowingCount, "actor's following_count should be incremented")
}

func TestRemoteFollowService_AcceptFollow_idempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc, _, remoteFollowSvc := newFollowTestServices(fake)

	actor, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	target, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	follow, err := remoteFollowSvc.CreateRemoteFollow(ctx, actor.ID, target.ID, domain.FollowStatePending, nil)
	require.NoError(t, err)

	require.NoError(t, remoteFollowSvc.AcceptFollow(ctx, follow.ID))
	require.NoError(t, remoteFollowSvc.AcceptFollow(ctx, follow.ID))

	targetAfter, err := fake.GetAccountByID(ctx, target.ID)
	require.NoError(t, err)
	actorAfter, err := fake.GetAccountByID(ctx, actor.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, targetAfter.FollowersCount, "counts should not double-increment")
	assert.Equal(t, 1, actorAfter.FollowingCount, "counts should not double-increment")
}

func TestRemoteFollowService_AcceptFollow_not_found(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	_, _, remoteFollowSvc := newFollowTestServices(fake)

	err := remoteFollowSvc.AcceptFollow(ctx, "01nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestRemoteFollowService_CreateRemoteFollow_accepted_increments_counts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc, _, remoteFollowSvc := newFollowTestServices(fake)

	actor, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	target, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	follow, err := remoteFollowSvc.CreateRemoteFollow(ctx, actor.ID, target.ID, domain.FollowStateAccepted, nil)
	require.NoError(t, err)
	require.NotNil(t, follow)
	assert.Equal(t, domain.FollowStateAccepted, follow.State)

	targetAfter, err := fake.GetAccountByID(ctx, target.ID)
	require.NoError(t, err)
	actorAfter, err := fake.GetAccountByID(ctx, actor.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, targetAfter.FollowersCount, "target's followers_count should be incremented when state is accepted")
	assert.Equal(t, 1, actorAfter.FollowingCount, "actor's following_count should be incremented when state is accepted")
}

func TestRemoteFollowService_CreateRemoteFollow_pending_does_not_increment_counts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc, _, remoteFollowSvc := newFollowTestServices(fake)

	actor, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	target, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	follow, err := remoteFollowSvc.CreateRemoteFollow(ctx, actor.ID, target.ID, domain.FollowStatePending, nil)
	require.NoError(t, err)
	require.NotNil(t, follow)
	assert.Equal(t, domain.FollowStatePending, follow.State)

	targetAfter, err := fake.GetAccountByID(ctx, target.ID)
	require.NoError(t, err)
	actorAfter, err := fake.GetAccountByID(ctx, actor.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, targetAfter.FollowersCount, "followers_count should not be incremented when state is pending")
	assert.Equal(t, 0, actorAfter.FollowingCount, "following_count should not be incremented when state is pending")
}

func TestRemoteFollowService_DeleteRemoteFollow_accepted_decrements_counts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc, _, remoteFollowSvc := newFollowTestServices(fake)

	actor, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	target, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	_, err = remoteFollowSvc.CreateRemoteFollow(ctx, actor.ID, target.ID, domain.FollowStateAccepted, nil)
	require.NoError(t, err)

	targetBefore, err := fake.GetAccountByID(ctx, target.ID)
	require.NoError(t, err)
	actorBefore, err := fake.GetAccountByID(ctx, actor.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, targetBefore.FollowersCount)
	assert.Equal(t, 1, actorBefore.FollowingCount)

	err = remoteFollowSvc.DeleteRemoteFollow(ctx, actor.ID, target.ID)
	require.NoError(t, err)

	targetAfter, err := fake.GetAccountByID(ctx, target.ID)
	require.NoError(t, err)
	actorAfter, err := fake.GetAccountByID(ctx, actor.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, targetAfter.FollowersCount, "followers_count should be decremented when deleting accepted follow")
	assert.Equal(t, 0, actorAfter.FollowingCount, "following_count should be decremented when deleting accepted follow")
}

func TestRemoteFollowService_DeleteRemoteFollow_pending_does_not_decrement_counts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc, _, remoteFollowSvc := newFollowTestServices(fake)

	actor, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	target, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	_, err = remoteFollowSvc.CreateRemoteFollow(ctx, actor.ID, target.ID, domain.FollowStatePending, nil)
	require.NoError(t, err)

	err = remoteFollowSvc.DeleteRemoteFollow(ctx, actor.ID, target.ID)
	require.NoError(t, err)

	targetAfter, err := fake.GetAccountByID(ctx, target.ID)
	require.NoError(t, err)
	actorAfter, err := fake.GetAccountByID(ctx, actor.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, targetAfter.FollowersCount, "followers_count should remain 0 when deleting pending follow")
	assert.Equal(t, 0, actorAfter.FollowingCount, "following_count should remain 0 when deleting pending follow")
}

func TestRemoteFollowService_DeleteRemoteFollow_not_found(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc, _, remoteFollowSvc := newFollowTestServices(fake)

	actor, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	target, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	err = remoteFollowSvc.DeleteRemoteFollow(ctx, actor.ID, target.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestRemoteFollowService_CreateRemoteBlock_removes_follows_and_creates_block(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc, _, remoteFollowSvc := newFollowTestServices(fake)

	actor, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	target, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	_, err = remoteFollowSvc.CreateRemoteFollow(ctx, actor.ID, target.ID, domain.FollowStateAccepted, nil)
	require.NoError(t, err)

	err = remoteFollowSvc.CreateRemoteBlock(ctx, actor.ID, target.ID)
	require.NoError(t, err)

	_, err = fake.GetFollow(ctx, actor.ID, target.ID)
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrNotFound)

	block, err := fake.GetBlock(ctx, actor.ID, target.ID)
	require.NoError(t, err)
	assert.NotNil(t, block)

	actorAfter, err := fake.GetAccountByID(ctx, actor.ID)
	require.NoError(t, err)
	targetAfter, err := fake.GetAccountByID(ctx, target.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, actorAfter.FollowingCount, "actor following_count should be decremented")
	assert.Equal(t, 0, targetAfter.FollowersCount, "target followers_count should be decremented")
}

func TestRemoteFollowService_CreateRemoteBlock_removes_both_direction_follows(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc, _, remoteFollowSvc := newFollowTestServices(fake)

	actor, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	target, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	_, err = remoteFollowSvc.CreateRemoteFollow(ctx, actor.ID, target.ID, domain.FollowStateAccepted, nil)
	require.NoError(t, err)
	_, err = remoteFollowSvc.CreateRemoteFollow(ctx, target.ID, actor.ID, domain.FollowStateAccepted, nil)
	require.NoError(t, err)

	err = remoteFollowSvc.CreateRemoteBlock(ctx, actor.ID, target.ID)
	require.NoError(t, err)

	_, err = fake.GetFollow(ctx, actor.ID, target.ID)
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = fake.GetFollow(ctx, target.ID, actor.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestRemoteFollowService_CreateRemoteBlock_self_block_returns_validation_error(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc, _, remoteFollowSvc := newFollowTestServices(fake)

	actor, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)

	err = remoteFollowSvc.CreateRemoteBlock(ctx, actor.ID, actor.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestRemoteFollowService_CreateRemoteBlock_target_not_found(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc, _, remoteFollowSvc := newFollowTestServices(fake)

	actor, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)

	err = remoteFollowSvc.CreateRemoteBlock(ctx, actor.ID, "01nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestRemoteFollowService_HasLocalFollower_returns_true_when_has_local_followers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc, _, remoteFollowSvc := newFollowTestServices(fake)

	follower, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	target, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	_, err = remoteFollowSvc.CreateRemoteFollow(ctx, follower.ID, target.ID, domain.FollowStateAccepted, nil)
	require.NoError(t, err)

	has, err := remoteFollowSvc.HasLocalFollower(ctx, target.ID)
	require.NoError(t, err)
	assert.True(t, has)
}

func TestRemoteFollowService_HasLocalFollower_returns_false_when_no_followers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	_, _, remoteFollowSvc := newFollowTestServices(fake)

	account, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID:       uid.New(),
		Username: "bob",
	})
	require.NoError(t, err)

	has, err := remoteFollowSvc.HasLocalFollower(ctx, account.ID)
	require.NoError(t, err)
	assert.False(t, has)
}

func TestRemoteFollowService_HasLocalFollower_returns_false_when_only_remote_followers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc, _, remoteFollowSvc := newFollowTestServices(fake)

	target, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	remoteID := uid.New()
	remoteDomain := testRemoteDomain
	_, err = fake.CreateAccount(ctx, store.CreateAccountInput{
		ID:       remoteID,
		Username: "alice",
		Domain:   testutil.StrPtr(remoteDomain),
		InboxURL: "https://" + remoteDomain + "/users/alice/inbox",
	})
	require.NoError(t, err)

	_, err = remoteFollowSvc.CreateRemoteFollow(ctx, remoteID, target.ID, domain.FollowStateAccepted, nil)
	require.NoError(t, err)

	has, err := remoteFollowSvc.HasLocalFollower(ctx, target.ID)
	require.NoError(t, err)
	assert.False(t, has, "HasLocalFollower should return false when only remote followers exist")
}

func TestRemoteFollowService_GetFollowerInboxURLsPaginated_returns_remote_follower_inbox_urls(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc, _, remoteFollowSvc := newFollowTestServices(fake)

	target, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	remoteID := uid.New()
	remoteDomain := testRemoteDomain
	inboxURL := "https://" + remoteDomain + "/users/alice/inbox"
	_, err = fake.CreateAccount(ctx, store.CreateAccountInput{
		ID:       remoteID,
		Username: "alice",
		Domain:   testutil.StrPtr(remoteDomain),
		InboxURL: inboxURL,
	})
	require.NoError(t, err)

	_, err = remoteFollowSvc.CreateRemoteFollow(ctx, remoteID, target.ID, domain.FollowStateAccepted, nil)
	require.NoError(t, err)

	urls, err := remoteFollowSvc.GetFollowerInboxURLsPaginated(ctx, target.ID, "", 10)
	require.NoError(t, err)
	require.Len(t, urls, 1)
	assert.Equal(t, inboxURL, urls[0])
}

func TestRemoteFollowService_GetFollowerInboxURLsPaginated_returns_empty_when_no_remote_followers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc, _, remoteFollowSvc := newFollowTestServices(fake)

	target, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	urls, err := remoteFollowSvc.GetFollowerInboxURLsPaginated(ctx, target.ID, "", 10)
	require.NoError(t, err)
	assert.Empty(t, urls)
}

func TestRemoteFollowService_GetFollowerInboxURLsPaginated_respects_limit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc, _, remoteFollowSvc := newFollowTestServices(fake)

	target, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	remote1 := uid.New()
	remote2 := uid.New()
	fake.SeedAccount(&domain.Account{
		ID:       remote1,
		Username: "user1",
		Domain:   testutil.StrPtr("remote1.example"),
		InboxURL: "https://remote1.example/users/1/inbox",
	})
	fake.SeedAccount(&domain.Account{
		ID:       remote2,
		Username: "user2",
		Domain:   testutil.StrPtr("remote2.example"),
		InboxURL: "https://remote2.example/users/2/inbox",
	})

	_, err = remoteFollowSvc.CreateRemoteFollow(ctx, remote1, target.ID, domain.FollowStateAccepted, nil)
	require.NoError(t, err)
	_, err = remoteFollowSvc.CreateRemoteFollow(ctx, remote2, target.ID, domain.FollowStateAccepted, nil)
	require.NoError(t, err)

	urls, err := remoteFollowSvc.GetFollowerInboxURLsPaginated(ctx, target.ID, "", 1)
	require.NoError(t, err)
	assert.Len(t, urls, 1, "limit 1 should return at most 1 URL")
}

func TestRemoteFollowService_GetFollowByAPID_returns_follow_when_found(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc, _, remoteFollowSvc := newFollowTestServices(fake)

	actor, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	target, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	apID := "https://example.com/follows/01abc123"
	follow, err := remoteFollowSvc.CreateRemoteFollow(ctx, actor.ID, target.ID, domain.FollowStatePending, &apID)
	require.NoError(t, err)
	require.NotNil(t, follow)

	got, err := remoteFollowSvc.GetFollowByAPID(ctx, apID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, follow.ID, got.ID)
	assert.Equal(t, actor.ID, got.AccountID)
	assert.Equal(t, target.ID, got.TargetID)
	assert.Equal(t, apID, *got.APID)
}

func TestRemoteFollowService_GetFollowByAPID_returns_not_found_when_missing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	_, _, remoteFollowSvc := newFollowTestServices(fake)

	got, err := remoteFollowSvc.GetFollowByAPID(ctx, "https://example.com/follows/nonexistent")
	require.Error(t, err)
	assert.Nil(t, got)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}
