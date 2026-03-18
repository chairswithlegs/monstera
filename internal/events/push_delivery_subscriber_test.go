package events

import (
	"context"
	"errors"
	"testing"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/webpush"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeLister struct {
	subs []domain.PushSubscription
	err  error
}

func (f *fakeLister) ListByAccountID(ctx context.Context, accountID string) ([]domain.PushSubscription, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.subs, nil
}

type fakeSender struct {
	calls []*domain.PushSubscription
	err   error
}

func (f *fakeSender) Send(ctx context.Context, sub *domain.PushSubscription, payload []byte) error {
	f.calls = append(f.calls, sub)
	return f.err
}

type fakeDeleter struct {
	deleted []string
}

func (f *fakeDeleter) Delete(ctx context.Context, accessTokenID string) error {
	f.deleted = append(f.deleted, accessTokenID)
	return nil
}

func TestAlertEnabled(t *testing.T) {
	t.Parallel()
	alerts := domain.PushAlerts{
		Follow:    true,
		Favourite: true,
		Reblog:    false,
		Mention:   true,
	}
	tests := []struct {
		name      string
		notifType string
		want      bool
	}{
		{"follow enabled", domain.NotificationTypeFollow, true},
		{"favourite enabled", domain.NotificationTypeFavourite, true},
		{"reblog disabled", domain.NotificationTypeReblog, false},
		{"mention enabled", domain.NotificationTypeMention, true},
		{"follow_request disabled", domain.NotificationTypeFollowRequest, false},
		{"unknown disabled", "unknown_type", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, alertEnabled(alerts, tt.notifType))
		})
	}
}

func TestBuildPushPayload(t *testing.T) {
	t.Parallel()
	displayName := "Alice"
	payload := domain.NotificationCreatedPayload{
		RecipientAccountID: "acct-1",
		Notification: &domain.Notification{
			ID:   "notif-1",
			Type: domain.NotificationTypeFollow,
		},
		FromAccount: &domain.Account{
			Username:    "alice",
			DisplayName: &displayName,
		},
	}
	raw := buildPushPayload(payload, domain.NotificationTypeFollow)
	assert.Contains(t, string(raw), "Alice followed you")
	assert.Contains(t, string(raw), "notif-1")
}

func TestBuildPushPayload_NilFromAccount(t *testing.T) {
	t.Parallel()
	payload := domain.NotificationCreatedPayload{
		Notification: &domain.Notification{
			ID:   "notif-2",
			Type: domain.NotificationTypeMention,
		},
	}
	raw := buildPushPayload(payload, domain.NotificationTypeMention)
	assert.Contains(t, string(raw), "mention")
}

func TestDeliver_SendsToMatchingSubscriptions(t *testing.T) {
	t.Parallel()
	lister := &fakeLister{
		subs: []domain.PushSubscription{
			{
				ID:            "sub-1",
				AccessTokenID: "token-1",
				AccountID:     "acct-1",
				Endpoint:      "https://example.com/push-1",
				Alerts:        domain.PushAlerts{Follow: true},
			},
			{
				ID:            "sub-2",
				AccessTokenID: "token-2",
				AccountID:     "acct-1",
				Endpoint:      "https://example.com/push-2",
				Alerts:        domain.PushAlerts{Follow: false},
			},
		},
	}
	sender := &fakeSender{}
	deleter := &fakeDeleter{}
	sub := NewPushDeliverySubscriber(nil, PushDeliveryDeps{
		PushSubs: lister,
		Deleter:  deleter,
		Sender:   sender,
	})
	payload := domain.NotificationCreatedPayload{
		RecipientAccountID: "acct-1",
		Notification:       &domain.Notification{ID: "notif-1", Type: domain.NotificationTypeFollow},
		FromAccount:        &domain.Account{Username: "alice"},
	}
	sub.deliver(context.Background(), payload)
	require.Len(t, sender.calls, 1, "expected 1 send call for subscription with Follow enabled")
	assert.Equal(t, "sub-1", sender.calls[0].ID)
}

func TestDeliver_NoSubscriptions(t *testing.T) {
	t.Parallel()
	lister := &fakeLister{subs: []domain.PushSubscription{}}
	sender := &fakeSender{}
	deleter := &fakeDeleter{}
	sub := NewPushDeliverySubscriber(nil, PushDeliveryDeps{
		PushSubs: lister,
		Deleter:  deleter,
		Sender:   sender,
	})
	payload := domain.NotificationCreatedPayload{
		RecipientAccountID: "acct-1",
		Notification:       &domain.Notification{ID: "notif-1", Type: domain.NotificationTypeFollow},
		FromAccount:        &domain.Account{Username: "alice"},
	}
	sub.deliver(context.Background(), payload)
	assert.Empty(t, sender.calls, "sender should not be called when there are no subscriptions")
}

func TestDeliver_GoneDeletesSubscription(t *testing.T) {
	t.Parallel()
	lister := &fakeLister{
		subs: []domain.PushSubscription{
			{
				ID:            "sub-1",
				AccessTokenID: "token-gone",
				AccountID:     "acct-1",
				Endpoint:      "https://example.com/push-1",
				Alerts:        domain.PushAlerts{Follow: true},
			},
		},
	}
	sender := &fakeSender{err: webpush.ErrSubscriptionGone}
	deleter := &fakeDeleter{}
	sub := NewPushDeliverySubscriber(nil, PushDeliveryDeps{
		PushSubs: lister,
		Deleter:  deleter,
		Sender:   sender,
	})
	payload := domain.NotificationCreatedPayload{
		RecipientAccountID: "acct-1",
		Notification:       &domain.Notification{ID: "notif-1", Type: domain.NotificationTypeFollow},
		FromAccount:        &domain.Account{Username: "alice"},
	}
	sub.deliver(context.Background(), payload)
	require.Len(t, deleter.deleted, 1, "deleter should be called once for gone subscription")
	assert.Equal(t, "token-gone", deleter.deleted[0])
}

func TestDeliver_SendErrorDoesNotDelete(t *testing.T) {
	t.Parallel()
	lister := &fakeLister{
		subs: []domain.PushSubscription{
			{
				ID:            "sub-1",
				AccessTokenID: "token-1",
				AccountID:     "acct-1",
				Endpoint:      "https://example.com/push-1",
				Alerts:        domain.PushAlerts{Follow: true},
			},
		},
	}
	sender := &fakeSender{err: errors.New("network error")}
	deleter := &fakeDeleter{}
	sub := NewPushDeliverySubscriber(nil, PushDeliveryDeps{
		PushSubs: lister,
		Deleter:  deleter,
		Sender:   sender,
	})
	payload := domain.NotificationCreatedPayload{
		RecipientAccountID: "acct-1",
		Notification:       &domain.Notification{ID: "notif-1", Type: domain.NotificationTypeFollow},
		FromAccount:        &domain.Account{Username: "alice"},
	}
	sub.deliver(context.Background(), payload)
	assert.Empty(t, deleter.deleted, "deleter should not be called for generic send error")
}
