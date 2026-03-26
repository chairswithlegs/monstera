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

func newFollowTestServices(fake *testutil.FakeStore) (AccountService, FollowService, RemoteFollowService) {
	accountSvc := NewAccountService(fake, "https://example.com")
	remoteFollowSvc := NewRemoteFollowService(fake)
	followSvc := NewFollowService(fake, accountSvc, remoteFollowSvc, nil)
	return accountSvc, followSvc, remoteFollowSvc
}

func TestFollowService_Follow_success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc, followSvc, _ := newFollowTestServices(fake)

	actor, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	target, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	rel, err := followSvc.Follow(ctx, actor.ID, target.ID)
	require.NoError(t, err)
	require.NotNil(t, rel)
	assert.Equal(t, target.ID, rel.TargetID)
	assert.True(t, rel.Following)
}

func TestFollowService_Follow_self_returns_validation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc, followSvc, _ := newFollowTestServices(fake)

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)

	_, err = followSvc.Follow(ctx, acc.ID, acc.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestFollowService_Follow_target_not_found(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc, followSvc, _ := newFollowTestServices(fake)

	actor, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)

	_, err = followSvc.Follow(ctx, actor.ID, "01nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestFollowService_Follow_target_suspended_returns_not_found(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc, followSvc, _ := newFollowTestServices(fake)

	actor, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	target, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)
	require.NoError(t, fake.SuspendAccount(ctx, target.ID))

	_, err = followSvc.Follow(ctx, actor.ID, target.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestFollowService_Follow_blocked_returns_forbidden(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc, followSvc, _ := newFollowTestServices(fake)

	actor, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	target, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)
	require.NoError(t, fake.CreateBlock(ctx, store.CreateBlockInput{
		ID:        "block-1",
		AccountID: target.ID,
		TargetID:  actor.ID,
	}))

	_, err = followSvc.Follow(ctx, actor.ID, target.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestFollowService_Follow_already_following_returns_relationship(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc, followSvc, _ := newFollowTestServices(fake)

	actor, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	target, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	rel1, err := followSvc.Follow(ctx, actor.ID, target.ID)
	require.NoError(t, err)
	assert.True(t, rel1.Following)

	rel2, err := followSvc.Follow(ctx, actor.ID, target.ID)
	require.NoError(t, err)
	require.NotNil(t, rel2)
	assert.True(t, rel2.Following)
}

func TestFollowService_Follow_locked_account_creates_pending(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc, followSvc, _ := newFollowTestServices(fake)

	actor, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	target, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob", Locked: true})
	require.NoError(t, err)

	rel, err := followSvc.Follow(ctx, actor.ID, target.ID)
	require.NoError(t, err)
	require.NotNil(t, rel)
	assert.True(t, rel.Following)
	assert.True(t, rel.Requested)
}

func TestFollowService_Follow_remote_unlocked_account_creates_pending_no_count_increments(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc, followSvc, _ := newFollowTestServices(fake)

	actor, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	target, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID:           uid.New(),
		Username:     "bob",
		Domain:       testutil.StrPtr("mastodon.social"),
		InboxURL:     "https://mastodon.social/users/bob/inbox",
		OutboxURL:    "https://mastodon.social/users/bob/outbox",
		FollowersURL: "https://mastodon.social/users/bob/followers",
		FollowingURL: "https://mastodon.social/users/bob/following",
		APID:         "https://mastodon.social/users/bob",
		PublicKey:    "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----",
	})
	require.NoError(t, err)

	rel, err := followSvc.Follow(ctx, actor.ID, target.ID)
	require.NoError(t, err)
	require.NotNil(t, rel)
	assert.True(t, rel.Following)
	assert.True(t, rel.Requested)

	follow, err := followSvc.GetFollow(ctx, actor.ID, target.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.FollowStatePending, follow.State)

	actorAfter, err := fake.GetAccountByID(ctx, actor.ID)
	require.NoError(t, err)
	targetAfter, err := fake.GetAccountByID(ctx, target.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, actorAfter.FollowingCount, "following_count should not be incremented for remote pending follow")
	assert.Equal(t, 0, targetAfter.FollowersCount, "followers_count should not be incremented for remote pending follow")
}

func TestFollowService_Follow_local_unlocked_account_creates_accepted_increments_counts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc, followSvc, _ := newFollowTestServices(fake)

	actor, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	target, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	rel, err := followSvc.Follow(ctx, actor.ID, target.ID)
	require.NoError(t, err)
	require.NotNil(t, rel)
	assert.True(t, rel.Following)
	assert.False(t, rel.Requested)

	follow, err := followSvc.GetFollow(ctx, actor.ID, target.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.FollowStateAccepted, follow.State)

	actorAfter, err := fake.GetAccountByID(ctx, actor.ID)
	require.NoError(t, err)
	targetAfter, err := fake.GetAccountByID(ctx, target.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, actorAfter.FollowingCount, "following_count should be incremented for local accepted follow")
	assert.Equal(t, 1, targetAfter.FollowersCount, "followers_count should be incremented for local accepted follow")
}

func TestFollowService_Unfollow_success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc, followSvc, _ := newFollowTestServices(fake)

	actor, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	target, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	_, err = followSvc.Follow(ctx, actor.ID, target.ID)
	require.NoError(t, err)

	rel, err := followSvc.Unfollow(ctx, actor.ID, target.ID)
	require.NoError(t, err)
	require.NotNil(t, rel)
	assert.False(t, rel.Following)
}

func TestFollowService_Unfollow_no_follow_returns_relationship(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc, followSvc, _ := newFollowTestServices(fake)

	actor, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	target, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	rel, err := followSvc.Unfollow(ctx, actor.ID, target.ID)
	require.NoError(t, err)
	require.NotNil(t, rel)
	assert.False(t, rel.Following)
}
