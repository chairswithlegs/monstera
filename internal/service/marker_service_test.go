package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/testutil"
)

func TestMarkerService_SetMarker_invalidTimeline(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := NewAccountService(st, "https://example.com")
	markerSvc := NewMarkerService(st)

	account, err := accountSvc.Register(ctx, RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	err = markerSvc.SetMarker(ctx, account.ID, "invalid", "01HQXXX")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}
