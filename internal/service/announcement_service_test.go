package service

import (
	"context"
	"testing"
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/chairswithlegs/monstera/internal/uid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestAnnouncement(t *testing.T, ctx context.Context, svc AnnouncementService, content string) *domain.Announcement {
	t.Helper()
	a, err := svc.Create(ctx, CreateAnnouncementInput{Content: content})
	require.NoError(t, err)
	return a
}

func TestAnnouncementService_Create(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewAnnouncementService(fake)

		a, err := svc.Create(ctx, CreateAnnouncementInput{Content: "Hello world"})
		require.NoError(t, err)
		require.NotNil(t, a)
		assert.Equal(t, "Hello world", a.Content)
		assert.NotEmpty(t, a.ID)
		assert.False(t, a.PublishedAt.IsZero())
	})
}

func TestAnnouncementService_GetByID(t *testing.T) {
	t.Parallel()

	t.Run("existing", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewAnnouncementService(fake)

		created := createTestAnnouncement(t, ctx, svc, "test")

		got, err := svc.GetByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, got.ID)
		assert.Equal(t, "test", got.Content)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewAnnouncementService(fake)

		_, err := svc.GetByID(ctx, "nonexistent")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestAnnouncementService_ListAll(t *testing.T) {
	t.Parallel()

	t.Run("returns all", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewAnnouncementService(fake)

		createTestAnnouncement(t, ctx, svc, "first")
		createTestAnnouncement(t, ctx, svc, "second")

		list, err := svc.ListAll(ctx)
		require.NoError(t, err)
		assert.Len(t, list, 2)
	})
}

func TestAnnouncementService_ListActive(t *testing.T) {
	t.Parallel()

	t.Run("returns active with read state and reactions", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewAnnouncementService(fake)

		a := createTestAnnouncement(t, ctx, svc, "active announcement")
		viewerID := uid.New()

		err := svc.Dismiss(ctx, viewerID, a.ID)
		require.NoError(t, err)

		otherAccountID := uid.New()
		err = svc.AddReaction(ctx, otherAccountID, a.ID, "👍")
		require.NoError(t, err)
		err = svc.AddReaction(ctx, viewerID, a.ID, "👍")
		require.NoError(t, err)

		items, err := svc.ListActive(ctx, viewerID)
		require.NoError(t, err)
		require.Len(t, items, 1)

		item := items[0]
		assert.Equal(t, a.ID, item.Announcement.ID)
		assert.True(t, item.Read)
		require.Len(t, item.Reactions, 1)
		assert.Equal(t, "👍", item.Reactions[0].Name)
		assert.Equal(t, 2, item.Reactions[0].Count)
		assert.True(t, item.Reactions[0].Me)
	})

	t.Run("no announcements", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewAnnouncementService(fake)

		items, err := svc.ListActive(ctx, "account-1")
		require.NoError(t, err)
		assert.Empty(t, items)
	})
}

func TestAnnouncementService_Dismiss(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewAnnouncementService(fake)

		a := createTestAnnouncement(t, ctx, svc, "dismiss me")

		err := svc.Dismiss(ctx, "account-1", a.ID)
		require.NoError(t, err)

		items, err := svc.ListActive(ctx, "account-1")
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.True(t, items[0].Read)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewAnnouncementService(fake)

		err := svc.Dismiss(ctx, "account-1", "nonexistent")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestAnnouncementService_AddReaction(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewAnnouncementService(fake)

		a := createTestAnnouncement(t, ctx, svc, "react to me")

		err := svc.AddReaction(ctx, "account-1", a.ID, "🎉")
		require.NoError(t, err)

		items, err := svc.ListActive(ctx, "account-1")
		require.NoError(t, err)
		require.Len(t, items, 1)
		require.Len(t, items[0].Reactions, 1)
		assert.Equal(t, "🎉", items[0].Reactions[0].Name)
		assert.Equal(t, 1, items[0].Reactions[0].Count)
		assert.True(t, items[0].Reactions[0].Me)
	})

	t.Run("empty name", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewAnnouncementService(fake)

		a := createTestAnnouncement(t, ctx, svc, "test")

		err := svc.AddReaction(ctx, "account-1", a.ID, "")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrValidation)
	})

	t.Run("announcement not found", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewAnnouncementService(fake)

		err := svc.AddReaction(ctx, "account-1", "nonexistent", "👍")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestAnnouncementService_RemoveReaction(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewAnnouncementService(fake)

		a := createTestAnnouncement(t, ctx, svc, "react then remove")

		err := svc.AddReaction(ctx, "account-1", a.ID, "🎉")
		require.NoError(t, err)

		err = svc.RemoveReaction(ctx, "account-1", a.ID, "🎉")
		require.NoError(t, err)

		items, err := svc.ListActive(ctx, "account-1")
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Empty(t, items[0].Reactions)
	})

	t.Run("empty name", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewAnnouncementService(fake)

		a := createTestAnnouncement(t, ctx, svc, "test")

		err := svc.RemoveReaction(ctx, "account-1", a.ID, "")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrValidation)
	})

	t.Run("announcement not found", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewAnnouncementService(fake)

		err := svc.RemoveReaction(ctx, "account-1", "nonexistent", "👍")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestAnnouncementService_Update(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewAnnouncementService(fake)

		a := createTestAnnouncement(t, ctx, svc, "original")

		err := svc.Update(ctx, UpdateAnnouncementInput{
			ID:          a.ID,
			Content:     "updated",
			PublishedAt: time.Now(),
		})
		require.NoError(t, err)

		got, err := svc.GetByID(ctx, a.ID)
		require.NoError(t, err)
		assert.Equal(t, "updated", got.Content)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fake := testutil.NewFakeStore()
		svc := NewAnnouncementService(fake)

		err := svc.Update(ctx, UpdateAnnouncementInput{
			ID:          "nonexistent",
			Content:     "nope",
			PublishedAt: time.Now(),
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}
