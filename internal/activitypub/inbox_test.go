package activitypub

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera-fed/internal/cache"
	"github.com/chairswithlegs/monstera-fed/internal/config"
	"github.com/chairswithlegs/monstera-fed/internal/testutil"
)

func TestInboxProcessor_Process_unsupportedType(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	cacheStore, err := cache.New(cache.Config{Driver: "memory", Logger: slog.Default()})
	require.NoError(t, err)
	bl := NewBlocklistCache(fake, slog.Default())
	_ = bl.Refresh(ctx)
	proc := NewInboxProcessor(fake, cacheStore, bl, nil, nil, &config.Config{InstanceDomain: "example.com"}, slog.Default(), nil)
	activity := &Activity{Type: "Unknown", ID: "https://remote.example/activities/1", Actor: "https://remote.example/users/alice"}
	err = proc.Process(ctx, activity)
	assert.NoError(t, err)
}

func TestInboxProcessor_Process_emptyActorDomain(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	cacheStore, _ := cache.New(cache.Config{Driver: "memory", Logger: slog.Default()})
	bl := NewBlocklistCache(fake, slog.Default())
	proc := NewInboxProcessor(fake, cacheStore, bl, nil, nil, &config.Config{InstanceDomain: "example.com"}, slog.Default(), nil)
	activity := &Activity{Type: "Follow", Actor: "not-a-url"}
	err := proc.Process(ctx, activity)
	assert.ErrorIs(t, err, ErrFatal)
}
