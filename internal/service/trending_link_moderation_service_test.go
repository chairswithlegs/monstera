package service

import (
	"context"
	"testing"

	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrendingLinkDenylistService_Denylist(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc := NewTrendingLinkDenylistService(testutil.NewFakeStore())

	// Initially empty.
	urls, err := svc.GetDenylist(ctx)
	require.NoError(t, err)
	assert.Empty(t, urls)

	// Add a URL.
	require.NoError(t, svc.AddDenylist(ctx, "https://spam.example.com"))
	urls, err = svc.GetDenylist(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"https://spam.example.com"}, urls)

	// Adding duplicate is idempotent.
	require.NoError(t, svc.AddDenylist(ctx, "https://spam.example.com"))
	urls, err = svc.GetDenylist(ctx)
	require.NoError(t, err)
	assert.Len(t, urls, 1)

	// Remove the URL.
	require.NoError(t, svc.RemoveDenylist(ctx, "https://spam.example.com"))
	urls, err = svc.GetDenylist(ctx)
	require.NoError(t, err)
	assert.Empty(t, urls)
}
