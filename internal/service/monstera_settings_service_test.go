package service

import (
	"context"
	"strings"
	"testing"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr(s string) *string { return &s }

func TestMonsteraSettingsService_Get_default(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewMonsteraSettingsService(fake)

	settings, err := svc.Get(ctx)
	require.NoError(t, err)
	expected := domain.MonsteraSettings{RegistrationMode: domain.MonsteraRegistrationModeOpen, ServerName: ptr("Monstera")}
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
	expected := domain.MonsteraSettings{RegistrationMode: domain.MonsteraRegistrationModeApproval, ServerName: ptr("Monstera")}
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
		err := svc.Update(ctx, domain.MonsteraSettings{RegistrationMode: mode})
		require.NoError(t, err)
		settings, err := svc.Get(ctx)
		require.NoError(t, err)
		expected := domain.MonsteraSettings{RegistrationMode: mode, ServerName: ptr("Monstera")}
		assert.Equal(t, expected, settings)
	}
}

func TestMonsteraSettingsService_Update_server_name_validation(t *testing.T) {
	t.Parallel()

	exactly24 := strings.Repeat("a", 24)
	tooLong := strings.Repeat("a", 25)

	tests := []struct {
		name       string
		serverName *string
		wantErr    bool
	}{
		{"nil", nil, false},
		{"empty", ptr(""), true},
		{"short", ptr("My Server"), false},
		{"exactly 24 chars", ptr(exactly24), false},
		{"25 chars", ptr(tooLong), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			fake := testutil.NewFakeStore()
			svc := NewMonsteraSettingsService(fake)

			err := svc.Update(ctx, domain.MonsteraSettings{
				RegistrationMode: domain.MonsteraRegistrationModeOpen,
				ServerName:       tt.serverName,
			})
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, domain.ErrValidation)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
