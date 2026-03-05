package service

import (
	"context"
	"testing"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFollowService_Follow_success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	followSvc := NewFollowService(fake, nil, nil)

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
	accountSvc := NewAccountService(fake, "https://example.com")
	followSvc := NewFollowService(fake, nil, nil)

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
	accountSvc := NewAccountService(fake, "https://example.com")
	followSvc := NewFollowService(fake, nil, nil)

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
	accountSvc := NewAccountService(fake, "https://example.com")
	followSvc := NewFollowService(fake, nil, nil)

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
	accountSvc := NewAccountService(fake, "https://example.com")
	followSvc := NewFollowService(fake, nil, nil)

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
	accountSvc := NewAccountService(fake, "https://example.com")
	followSvc := NewFollowService(fake, nil, nil)

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
	accountSvc := NewAccountService(fake, "https://example.com")
	followSvc := NewFollowService(fake, nil, nil)

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

func TestFollowService_Unfollow_success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	followSvc := NewFollowService(fake, nil, nil)

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
	accountSvc := NewAccountService(fake, "https://example.com")
	followSvc := NewFollowService(fake, nil, nil)

	actor, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	target, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	rel, err := followSvc.Unfollow(ctx, actor.ID, target.ID)
	require.NoError(t, err)
	require.NotNil(t, rel)
	assert.False(t, rel.Following)
}

func TestFollowService_AcceptFollow_increments_counts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	followSvc := NewFollowService(fake, nil, nil)

	actor, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	target, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	follow, err := followSvc.CreateFollowFromInbox(ctx, actor.ID, target.ID, domain.FollowStatePending, nil)
	require.NoError(t, err)
	require.NotNil(t, follow)

	err = followSvc.AcceptFollow(ctx, follow.ID)
	require.NoError(t, err)

	targetAfter, err := fake.GetAccountByID(ctx, target.ID)
	require.NoError(t, err)
	actorAfter, err := fake.GetAccountByID(ctx, actor.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, targetAfter.FollowersCount, "target's followers_count should be incremented")
	assert.Equal(t, 1, actorAfter.FollowingCount, "actor's following_count should be incremented")
}

func TestFollowService_AcceptFollow_idempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	followSvc := NewFollowService(fake, nil, nil)

	actor, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	target, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	follow, err := followSvc.CreateFollowFromInbox(ctx, actor.ID, target.ID, domain.FollowStatePending, nil)
	require.NoError(t, err)

	require.NoError(t, followSvc.AcceptFollow(ctx, follow.ID))
	require.NoError(t, followSvc.AcceptFollow(ctx, follow.ID))

	targetAfter, err := fake.GetAccountByID(ctx, target.ID)
	require.NoError(t, err)
	actorAfter, err := fake.GetAccountByID(ctx, actor.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, targetAfter.FollowersCount, "counts should not double-increment")
	assert.Equal(t, 1, actorAfter.FollowingCount, "counts should not double-increment")
}

func TestFollowService_AcceptFollow_not_found(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	followSvc := NewFollowService(fake, nil, nil)

	err := followSvc.AcceptFollow(ctx, "01nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestFollowService_CreateFollowFromInbox_accepted_increments_counts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	followSvc := NewFollowService(fake, nil, nil)

	actor, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	target, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	follow, err := followSvc.CreateFollowFromInbox(ctx, actor.ID, target.ID, domain.FollowStateAccepted, nil)
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

func TestFollowService_CreateFollowFromInbox_pending_does_not_increment_counts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	followSvc := NewFollowService(fake, nil, nil)

	actor, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	target, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	follow, err := followSvc.CreateFollowFromInbox(ctx, actor.ID, target.ID, domain.FollowStatePending, nil)
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
