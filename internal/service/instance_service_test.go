package service

import (
	"context"
	"testing"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstanceService_GetNodeInfoStats(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	_ = fake.UpdateMonsteraSettings(ctx, &domain.MonsteraSettings{RegistrationMode: domain.MonsteraRegistrationModeApproval})
	accountSvc := NewAccountService(fake, "https://example.com")
	instanceSvc := NewInstanceService(fake)

	_, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	_, err = accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	stats, err := instanceSvc.GetNodeInfoStats(ctx)
	require.NoError(t, err)
	require.NotNil(t, stats)
	assert.Equal(t, int64(2), stats.UserCount)
	assert.False(t, stats.OpenRegistrations)
}

func TestInstanceService_GetNodeInfoStats_open_registrations(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	_ = fake.UpdateMonsteraSettings(ctx, &domain.MonsteraSettings{RegistrationMode: domain.MonsteraRegistrationModeOpen})
	instanceSvc := NewInstanceService(fake)

	stats, err := instanceSvc.GetNodeInfoStats(ctx)
	require.NoError(t, err)
	require.NotNil(t, stats)
	assert.True(t, stats.OpenRegistrations)
}

func TestInstanceService_GetNodeInfoStats_local_post_count(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	instanceSvc := NewInstanceService(fake)

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	text := "first post"
	_, err = statusSvc.CreateLocal(ctx, CreateStatusInput{
		AccountID:  acc.ID,
		Text:       &text,
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)

	stats, err := instanceSvc.GetNodeInfoStats(ctx)
	require.NoError(t, err)
	require.NotNil(t, stats)
	assert.Equal(t, int64(1), stats.LocalPostCount)
}
