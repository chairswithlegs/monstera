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
