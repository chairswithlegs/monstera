//go:build integration

package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/chairswithlegs/monstera/internal/uid"
)

func TestIntegration_InteractionStore_Blocks(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("CreateBlock_GetBlock_DeleteBlock", func(t *testing.T) {
		a := createTestLocalAccount(t, s, ctx)
		b := createTestLocalAccount(t, s, ctx)

		err := s.CreateBlock(ctx, store.CreateBlockInput{
			ID:        uid.New(),
			AccountID: a.ID,
			TargetID:  b.ID,
		})
		require.NoError(t, err)

		block, err := s.GetBlock(ctx, a.ID, b.ID)
		require.NoError(t, err)
		assert.Equal(t, a.ID, block.AccountID)
		assert.Equal(t, b.ID, block.TargetID)

		err = s.DeleteBlock(ctx, a.ID, b.ID)
		require.NoError(t, err)

		_, err = s.GetBlock(ctx, a.ID, b.ID)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("GetBlock_not_found", func(t *testing.T) {
		_, err := s.GetBlock(ctx, uid.New(), uid.New())
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("IsBlockedEitherDirection", func(t *testing.T) {
		a := createTestLocalAccount(t, s, ctx)
		b := createTestLocalAccount(t, s, ctx)

		blocked, err := s.IsBlockedEitherDirection(ctx, a.ID, b.ID)
		require.NoError(t, err)
		assert.False(t, blocked)

		err = s.CreateBlock(ctx, store.CreateBlockInput{
			ID:        uid.New(),
			AccountID: a.ID,
			TargetID:  b.ID,
		})
		require.NoError(t, err)

		blocked, err = s.IsBlockedEitherDirection(ctx, a.ID, b.ID)
		require.NoError(t, err)
		assert.True(t, blocked)

		blocked, err = s.IsBlockedEitherDirection(ctx, b.ID, a.ID)
		require.NoError(t, err)
		assert.True(t, blocked)
	})

	t.Run("ListBlockedAccounts", func(t *testing.T) {
		a := createTestLocalAccount(t, s, ctx)
		b := createTestLocalAccount(t, s, ctx)

		err := s.CreateBlock(ctx, store.CreateBlockInput{
			ID:        uid.New(),
			AccountID: a.ID,
			TargetID:  b.ID,
		})
		require.NoError(t, err)

		accounts, _, err := s.ListBlockedAccounts(ctx, a.ID, nil, 10)
		require.NoError(t, err)
		found := false
		for _, acc := range accounts {
			if acc.ID == b.ID {
				found = true
			}
		}
		assert.True(t, found, "blocked account not in list")
	})
}

func TestIntegration_InteractionStore_Mutes(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("CreateMute_GetMute_DeleteMute", func(t *testing.T) {
		a := createTestLocalAccount(t, s, ctx)
		b := createTestLocalAccount(t, s, ctx)

		err := s.CreateMute(ctx, store.CreateMuteInput{
			ID:                uid.New(),
			AccountID:         a.ID,
			TargetID:          b.ID,
			HideNotifications: true,
		})
		require.NoError(t, err)

		mute, err := s.GetMute(ctx, a.ID, b.ID)
		require.NoError(t, err)
		assert.Equal(t, a.ID, mute.AccountID)
		assert.True(t, mute.HideNotifications)

		err = s.DeleteMute(ctx, a.ID, b.ID)
		require.NoError(t, err)

		_, err = s.GetMute(ctx, a.ID, b.ID)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("GetMute_not_found", func(t *testing.T) {
		_, err := s.GetMute(ctx, uid.New(), uid.New())
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("ListMutedAccounts", func(t *testing.T) {
		a := createTestLocalAccount(t, s, ctx)
		b := createTestLocalAccount(t, s, ctx)

		err := s.CreateMute(ctx, store.CreateMuteInput{
			ID:        uid.New(),
			AccountID: a.ID,
			TargetID:  b.ID,
		})
		require.NoError(t, err)

		accounts, _, err := s.ListMutedAccounts(ctx, a.ID, nil, 10)
		require.NoError(t, err)
		found := false
		for _, acc := range accounts {
			if acc.ID == b.ID {
				found = true
			}
		}
		assert.True(t, found, "muted account not in list")
	})
}

func TestIntegration_InteractionStore_Favourites(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("CreateFavourite_GetByAccountAndStatus_Delete", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, acc.ID)
		favID := uid.New()
		apID := "https://local.example/favourites/" + favID

		fav, err := s.CreateFavourite(ctx, store.CreateFavouriteInput{
			ID:        favID,
			AccountID: acc.ID,
			StatusID:  st.ID,
			APID:      &apID,
		})
		require.NoError(t, err)
		assert.Equal(t, favID, fav.ID)

		got, err := s.GetFavouriteByAccountAndStatus(ctx, acc.ID, st.ID)
		require.NoError(t, err)
		assert.Equal(t, favID, got.ID)

		err = s.DeleteFavourite(ctx, acc.ID, st.ID)
		require.NoError(t, err)

		_, err = s.GetFavouriteByAccountAndStatus(ctx, acc.ID, st.ID)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("GetFavouriteByAPID", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, acc.ID)
		favID := uid.New()
		apID := "https://local.example/favourites/" + favID

		_, err := s.CreateFavourite(ctx, store.CreateFavouriteInput{
			ID:        favID,
			AccountID: acc.ID,
			StatusID:  st.ID,
			APID:      &apID,
		})
		require.NoError(t, err)

		got, err := s.GetFavouriteByAPID(ctx, apID)
		require.NoError(t, err)
		assert.Equal(t, favID, got.ID)
	})

	t.Run("GetFavouriteByAPID_not_found", func(t *testing.T) {
		_, err := s.GetFavouriteByAPID(ctx, "https://nowhere.example/fav/ghost")
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("IncrementFavouritesCount_DecrementFavouritesCount", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, acc.ID)

		err := s.IncrementFavouritesCount(ctx, st.ID)
		require.NoError(t, err)
		got, err := s.GetStatusByID(ctx, st.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, got.FavouritesCount)

		err = s.DecrementFavouritesCount(ctx, st.ID)
		require.NoError(t, err)
		got, err = s.GetStatusByID(ctx, st.ID)
		require.NoError(t, err)
		assert.Equal(t, 0, got.FavouritesCount)
	})
}

func TestIntegration_InteractionStore_Bookmarks(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("CreateBookmark_IsBookmarked_DeleteBookmark", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, acc.ID)

		err := s.CreateBookmark(ctx, store.CreateBookmarkInput{
			ID:        uid.New(),
			AccountID: acc.ID,
			StatusID:  st.ID,
		})
		require.NoError(t, err)

		bookmarked, err := s.IsBookmarked(ctx, acc.ID, st.ID)
		require.NoError(t, err)
		assert.True(t, bookmarked)

		err = s.DeleteBookmark(ctx, acc.ID, st.ID)
		require.NoError(t, err)

		bookmarked, err = s.IsBookmarked(ctx, acc.ID, st.ID)
		require.NoError(t, err)
		assert.False(t, bookmarked)
	})

	t.Run("GetBookmarks", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, acc.ID)

		err := s.CreateBookmark(ctx, store.CreateBookmarkInput{
			ID:        uid.New(),
			AccountID: acc.ID,
			StatusID:  st.ID,
		})
		require.NoError(t, err)

		statuses, _, err := s.GetBookmarks(ctx, acc.ID, nil, 10)
		require.NoError(t, err)
		assert.NotEmpty(t, statuses)
	})

	t.Run("IsBookmarked_false", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, acc.ID)

		bookmarked, err := s.IsBookmarked(ctx, acc.ID, st.ID)
		require.NoError(t, err)
		assert.False(t, bookmarked)
	})
}

func TestIntegration_InteractionStore_Reblogs(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("IncrementReblogsCount_DecrementReblogsCount", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, acc.ID)

		err := s.IncrementReblogsCount(ctx, st.ID)
		require.NoError(t, err)
		got, err := s.GetStatusByID(ctx, st.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, got.ReblogsCount)

		err = s.DecrementReblogsCount(ctx, st.ID)
		require.NoError(t, err)
		got, err = s.GetStatusByID(ctx, st.ID)
		require.NoError(t, err)
		assert.Equal(t, 0, got.ReblogsCount)
	})

	t.Run("GetReblogByAccountAndTarget_not_found", func(t *testing.T) {
		_, err := s.GetReblogByAccountAndTarget(ctx, uid.New(), uid.New())
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("Reblog_create_and_find", func(t *testing.T) {
		author := createTestLocalAccount(t, s, ctx)
		original := createTestStatus(t, s, ctx, author.ID)

		reblogger := createTestLocalAccount(t, s, ctx)
		reblogID := uid.New()
		_, err := s.CreateStatus(ctx, store.CreateStatusInput{
			ID:                  reblogID,
			URI:                 "https://local.example/statuses/" + reblogID,
			AccountID:           reblogger.ID,
			Visibility:          domain.VisibilityPublic,
			ReblogOfID:          &original.ID,
			APID:                "https://local.example/statuses/" + reblogID,
			QuoteApprovalPolicy: domain.QuotePolicyPublic,
			Local:               true,
		})
		require.NoError(t, err)

		got, err := s.GetReblogByAccountAndTarget(ctx, reblogger.ID, original.ID)
		require.NoError(t, err)
		assert.Equal(t, reblogID, got.ID)
	})
}

func TestIntegration_InteractionStore_Quotes(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("IncrementQuotesCount_DecrementQuotesCount", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, acc.ID)

		err := s.IncrementQuotesCount(ctx, st.ID)
		require.NoError(t, err)
		got, err := s.GetStatusByID(ctx, st.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, got.QuotesCount)

		err = s.DecrementQuotesCount(ctx, st.ID)
		require.NoError(t, err)
		got, err = s.GetStatusByID(ctx, st.ID)
		require.NoError(t, err)
		assert.Equal(t, 0, got.QuotesCount)
	})

	t.Run("CreateQuoteApproval_GetQuoteApproval_RevokeQuote", func(t *testing.T) {
		author := createTestLocalAccount(t, s, ctx)
		quoted := createTestStatus(t, s, ctx, author.ID)

		quoter := createTestLocalAccount(t, s, ctx)
		quotingID := uid.New()
		_, err := s.CreateStatus(ctx, store.CreateStatusInput{
			ID:                  quotingID,
			URI:                 "https://local.example/statuses/" + quotingID,
			AccountID:           quoter.ID,
			Text:                testutil.StrPtr("quoting you"),
			Content:             testutil.StrPtr("<p>quoting you</p>"),
			Visibility:          domain.VisibilityPublic,
			QuotedStatusID:      &quoted.ID,
			APID:                "https://local.example/statuses/" + quotingID,
			QuoteApprovalPolicy: domain.QuotePolicyPublic,
			Local:               true,
		})
		require.NoError(t, err)

		err = s.CreateQuoteApproval(ctx, quotingID, quoted.ID)
		require.NoError(t, err)

		record, err := s.GetQuoteApproval(ctx, quotingID)
		require.NoError(t, err)
		assert.Equal(t, quotingID, record.QuotingStatusID)
		assert.Equal(t, quoted.ID, record.QuotedStatusID)
		assert.Nil(t, record.RevokedAt)

		err = s.RevokeQuote(ctx, quoted.ID, quotingID)
		require.NoError(t, err)

		record, err = s.GetQuoteApproval(ctx, quotingID)
		require.NoError(t, err)
		assert.NotNil(t, record.RevokedAt)
	})

	t.Run("IncrementRepliesCount", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, acc.ID)

		err := s.IncrementRepliesCount(ctx, st.ID)
		require.NoError(t, err)
		got, err := s.GetStatusByID(ctx, st.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, got.RepliesCount)
	})
}
