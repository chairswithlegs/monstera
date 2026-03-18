package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/testutil"
)

func TestPushSubscriptionService_Create_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	svc := NewPushSubscriptionService(st)

	alerts := domain.PushAlerts{Mention: true, Favourite: true}
	ps, err := svc.Create(ctx, "token-1", "acc-1", "https://push.example.com/sub", "p256dh-key", "auth-key", alerts, "all")
	require.NoError(t, err)
	require.NotNil(t, ps)
	assert.NotEmpty(t, ps.ID)
	assert.Equal(t, "token-1", ps.AccessTokenID)
	assert.Equal(t, "acc-1", ps.AccountID)
	assert.Equal(t, "https://push.example.com/sub", ps.Endpoint)
	assert.Equal(t, "p256dh-key", ps.KeyP256DH)
	assert.Equal(t, "auth-key", ps.KeyAuth)
	assert.True(t, ps.Alerts.Mention)
	assert.True(t, ps.Alerts.Favourite)
	assert.Equal(t, "all", ps.Policy)
}

func TestPushSubscriptionService_Create_EmptyEndpoint(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	svc := NewPushSubscriptionService(st)

	_, err := svc.Create(ctx, "token-1", "acc-1", "", "p256dh", "auth", domain.PushAlerts{}, "all")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestPushSubscriptionService_Create_EmptyP256DH(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	svc := NewPushSubscriptionService(st)

	_, err := svc.Create(ctx, "token-1", "acc-1", "https://push.example.com/sub", "", "auth", domain.PushAlerts{}, "all")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestPushSubscriptionService_Create_EmptyAuth(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	svc := NewPushSubscriptionService(st)

	_, err := svc.Create(ctx, "token-1", "acc-1", "https://push.example.com/sub", "p256dh", "", domain.PushAlerts{}, "all")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestPushSubscriptionService_Create_InvalidEndpointURL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	svc := NewPushSubscriptionService(st)

	_, err := svc.Create(ctx, "token-1", "acc-1", "not-a-url", "p256dh", "auth", domain.PushAlerts{}, "all")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestPushSubscriptionService_Create_HTTPEndpointRejected(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	svc := NewPushSubscriptionService(st)

	_, err := svc.Create(ctx, "token-1", "acc-1", "http://push.example.com/sub", "p256dh", "auth", domain.PushAlerts{}, "all")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestPushSubscriptionService_Create_HTTPSEndpointAccepted(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	svc := NewPushSubscriptionService(st)

	ps, err := svc.Create(ctx, "token-1", "acc-1", "https://push.example.com/sub", "p256dh", "auth", domain.PushAlerts{}, "all")
	require.NoError(t, err)
	assert.Equal(t, "https://push.example.com/sub", ps.Endpoint)
}

func TestPushSubscriptionService_Get_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	svc := NewPushSubscriptionService(st)

	_, err := svc.Get(ctx, "unknown-token-id")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestPushSubscriptionService_Delete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	svc := NewPushSubscriptionService(st)

	ps, err := svc.Create(ctx, "token-del", "acc-1", "https://push.example.com/sub", "p256dh", "auth", domain.PushAlerts{}, "all")
	require.NoError(t, err)
	require.NotNil(t, ps)

	err = svc.Delete(ctx, "token-del")
	require.NoError(t, err)

	_, err = svc.Get(ctx, "token-del")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}
