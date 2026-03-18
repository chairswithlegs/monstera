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

func TestNotificationService_List_empty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	notifSvc := NewNotificationService(fake)

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)

	list, err := notifSvc.List(ctx, acc.ID, nil, 20)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestNotificationService_List_returns_notifications(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	notifSvc := NewNotificationService(fake)

	actor, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	target, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	_, err = fake.CreateNotification(ctx, store.CreateNotificationInput{
		ID:        "01notif1",
		AccountID: target.ID,
		FromID:    actor.ID,
		Type:      domain.NotificationTypeFollow,
	})
	require.NoError(t, err)

	list, err := notifSvc.List(ctx, target.ID, nil, 20)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, target.ID, list[0].AccountID)
	assert.Equal(t, actor.ID, list[0].FromID)
	assert.Equal(t, domain.NotificationTypeFollow, list[0].Type)
}

func TestNotificationService_List_respects_limit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	notifSvc := NewNotificationService(fake)

	actor1, _ := accountSvc.Create(ctx, CreateAccountInput{Username: "a1"})
	actor2, _ := accountSvc.Create(ctx, CreateAccountInput{Username: "a2"})
	target, _ := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})

	_, _ = fake.CreateNotification(ctx, store.CreateNotificationInput{
		ID:        "01notif1",
		AccountID: target.ID,
		FromID:    actor1.ID,
		Type:      domain.NotificationTypeFollow,
	})
	_, _ = fake.CreateNotification(ctx, store.CreateNotificationInput{
		ID:        "01notif2",
		AccountID: target.ID,
		FromID:    actor2.ID,
		Type:      domain.NotificationTypeFollow,
	})

	list, err := notifSvc.List(ctx, target.ID, nil, 1)
	require.NoError(t, err)
	assert.Len(t, list, 1)
}

func TestNotificationService_List_zero_limit_uses_default(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	notifSvc := NewNotificationService(fake)

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)

	list, err := notifSvc.List(ctx, acc.ID, nil, 0)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestNotificationService_List_clamps_to_max_limit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	notifSvc := NewNotificationService(fake)

	list, err := notifSvc.List(ctx, "any-account", nil, 100)
	require.NoError(t, err)
	// Limit is clamped to maxNotificationLimit (40); no error and empty result for unknown account
	assert.Empty(t, list)
}
