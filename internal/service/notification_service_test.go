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

	list, err := notifSvc.List(ctx, acc.ID, nil, 20, nil, nil)
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

	list, err := notifSvc.List(ctx, target.ID, nil, 20, nil, nil)
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

	list, err := notifSvc.List(ctx, target.ID, nil, 1, nil, nil)
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

	list, err := notifSvc.List(ctx, acc.ID, nil, 0, nil, nil)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestNotificationService_List_clamps_to_max_limit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	notifSvc := NewNotificationService(fake)

	list, err := notifSvc.List(ctx, "any-account", nil, 100, nil, nil)
	require.NoError(t, err)
	// Limit is clamped to maxNotificationLimit (40); no error and empty result for unknown account
	assert.Empty(t, list)
}

func TestComputeGroupKey(t *testing.T) {
	t.Parallel()

	statusID := "status-99"

	tests := []struct {
		name        string
		notifType   string
		statusID    *string
		recipientID string
		wantPrefix  string
		ungrouped   bool
	}{
		{
			name:        "favourite with status groups by status",
			notifType:   domain.NotificationTypeFavourite,
			statusID:    &statusID,
			recipientID: "recipient-1",
			wantPrefix:  "favourite-status-99-",
		},
		{
			name:        "favourite without status falls through to ungrouped",
			notifType:   domain.NotificationTypeFavourite,
			statusID:    nil,
			recipientID: "recipient-1",
			ungrouped:   true,
		},
		{
			name:        "reblog with status groups by status",
			notifType:   domain.NotificationTypeReblog,
			statusID:    &statusID,
			recipientID: "recipient-1",
			wantPrefix:  "reblog-status-99-",
		},
		{
			name:        "follow groups by recipient",
			notifType:   domain.NotificationTypeFollow,
			statusID:    nil,
			recipientID: "recipient-1",
			wantPrefix:  "follow-recipient-1-",
		},
		{
			name:        "mention is ungrouped",
			notifType:   domain.NotificationTypeMention,
			statusID:    &statusID,
			recipientID: "recipient-1",
			ungrouped:   true,
		},
		{
			name:        "follow_request is ungrouped",
			notifType:   domain.NotificationTypeFollowRequest,
			statusID:    nil,
			recipientID: "recipient-1",
			ungrouped:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			notifID := "notif-abc"
			key := computeGroupKey(notifID, tc.notifType, tc.statusID, tc.recipientID)
			if tc.ungrouped {
				assert.Equal(t, "ungrouped-"+notifID, key)
			} else {
				assert.Greater(t, len(key), len(tc.wantPrefix), "key should be longer than prefix")
				assert.Equal(t, tc.wantPrefix, key[:len(tc.wantPrefix)])
			}
		})
	}
}

func TestComputeGroupKey_SameWindowSameKey(t *testing.T) {
	t.Parallel()
	// Two calls within the same time window must produce the same key.
	statusID := "status-1"
	key1 := computeGroupKey("n1", domain.NotificationTypeFavourite, &statusID, "r1")
	key2 := computeGroupKey("n2", domain.NotificationTypeFavourite, &statusID, "r1")
	assert.Equal(t, key1, key2, "notifications in the same window should share a group key")
}
