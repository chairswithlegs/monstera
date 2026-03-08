package activitypub

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
)

func TestBlocklistCache_RefreshAndIsBlocked(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	bl := NewBlocklistCache(fake)
	err := bl.Refresh(ctx)
	require.NoError(t, err)
	assert.False(t, bl.IsBlocked(ctx, "evil.example"))
	assert.Empty(t, bl.Severity(ctx, "evil.example"))
}

func TestBlocklistCache_IsSuspended_IsSilenced_Severity(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	_, err := fake.CreateDomainBlock(ctx, store.CreateDomainBlockInput{
		ID:       "1",
		Domain:   "suspended.example",
		Severity: domain.DomainBlockSeveritySuspend,
		Reason:   nil,
	})
	require.NoError(t, err)
	_, err = fake.CreateDomainBlock(ctx, store.CreateDomainBlockInput{
		ID:       "2",
		Domain:   "silenced.example",
		Severity: domain.DomainBlockSeveritySilence,
		Reason:   nil,
	})
	require.NoError(t, err)

	bl := NewBlocklistCache(fake)
	err = bl.Refresh(ctx)
	require.NoError(t, err)

	assert.True(t, bl.IsBlocked(ctx, "suspended.example"))
	assert.True(t, bl.IsSuspended(ctx, "suspended.example"))
	assert.False(t, bl.IsSilenced(ctx, "suspended.example"))
	assert.Equal(t, domain.DomainBlockSeveritySuspend, bl.Severity(ctx, "suspended.example"))

	assert.True(t, bl.IsBlocked(ctx, "silenced.example"))
	assert.False(t, bl.IsSuspended(ctx, "silenced.example"))
	assert.True(t, bl.IsSilenced(ctx, "silenced.example"))
	assert.Equal(t, domain.DomainBlockSeveritySilence, bl.Severity(ctx, "silenced.example"))

	assert.False(t, bl.IsBlocked(ctx, "unknown.example"))
	assert.False(t, bl.IsSuspended(ctx, "unknown.example"))
	assert.False(t, bl.IsSilenced(ctx, "unknown.example"))
	assert.Empty(t, bl.Severity(ctx, "unknown.example"))
}

func TestBlocklistCache_lookupNormalizesDomain(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	_, err := fake.CreateDomainBlock(ctx, store.CreateDomainBlockInput{
		ID:       "1",
		Domain:   "DOMAIN.example",
		Severity: domain.DomainBlockSeveritySuspend,
		Reason:   nil,
	})
	require.NoError(t, err)
	bl := NewBlocklistCache(fake)
	err = bl.Refresh(ctx)
	require.NoError(t, err)
	// Lookup with different casing and spacing should still find the block (normalizeDomain).
	assert.True(t, bl.IsBlocked(ctx, "  domain.example  "))
	assert.True(t, bl.IsSuspended(ctx, "DOMAIN.EXAMPLE"))
}
