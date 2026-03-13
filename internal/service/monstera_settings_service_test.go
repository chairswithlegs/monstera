package service

import (
	"context"
	"testing"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMonsteraSettingsService_Get_default(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewMonsteraSettingsService(fake)

	settings, err := svc.Get(ctx)
	require.NoError(t, err)
	expected := domain.MonsteraSettings{RegistrationMode: domain.MonsteraRegistrationModeOpen}
	assert.Equal(t, expected, settings)
}

func TestMonsteraSettingsService_Get_after_update(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewMonsteraSettingsService(fake)

	err := svc.Update(ctx, domain.MonsteraSettings{RegistrationMode: domain.MonsteraRegistrationModeApproval})
	require.NoError(t, err)

	settings, err := svc.Get(ctx)
	require.NoError(t, err)
	expected := domain.MonsteraSettings{RegistrationMode: domain.MonsteraRegistrationModeApproval}
	assert.Equal(t, expected, settings)
}

func TestMonsteraSettingsService_Update_invalid_mode(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewMonsteraSettingsService(fake)

	err := svc.Update(ctx, domain.MonsteraSettings{RegistrationMode: domain.MonsteraRegistrationMode("invalid")})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestMonsteraSettingsService_Update_all_modes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewMonsteraSettingsService(fake)

	modes := []domain.MonsteraRegistrationMode{
		domain.MonsteraRegistrationModeOpen,
		domain.MonsteraRegistrationModeApproval,
		domain.MonsteraRegistrationModeInvite,
		domain.MonsteraRegistrationModeClosed,
	}
	for _, mode := range modes {
		expected := domain.MonsteraSettings{RegistrationMode: mode}
		err := svc.Update(ctx, expected)
		require.NoError(t, err)
		settings, err := svc.Get(ctx)
		require.NoError(t, err)
		assert.Equal(t, expected, settings)
	}
}
