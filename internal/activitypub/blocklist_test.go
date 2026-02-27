package activitypub

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera-fed/internal/testutil"
)

func TestBlocklistCache_RefreshAndIsBlocked(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	bl := NewBlocklistCache(fake, slog.Default())
	err := bl.Refresh(ctx)
	require.NoError(t, err)
	assert.False(t, bl.IsBlocked(ctx, "evil.example"))
	assert.Empty(t, bl.Severity(ctx, "evil.example"))
}
