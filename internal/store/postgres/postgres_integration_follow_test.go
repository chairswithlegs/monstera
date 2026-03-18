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

func TestIntegration_FollowStore(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("CreateFollow_GetFollow", func(t *testing.T) {
		a := createTestLocalAccount(t, s, ctx)
		b := createTestLocalAccount(t, s, ctx)
		followID := uid.New()
		apID := "https://local.example/follows/" + followID

		f, err := s.CreateFollow(ctx, store.CreateFollowInput{
			ID:        followID,
			AccountID: a.ID,
			TargetID:  b.ID,
			State:     domain.FollowStatePending,
			APID:      &apID,
		})
		require.NoError(t, err)
		assert.Equal(t, followID, f.ID)
		assert.Equal(t, domain.FollowStatePending, f.State)

		got, err := s.GetFollow(ctx, a.ID, b.ID)
		require.NoError(t, err)
		assert.Equal(t, followID, got.ID)
	})

	t.Run("GetFollow_not_found", func(t *testing.T) {
		_, err := s.GetFollow(ctx, uid.New(), uid.New())
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("GetFollowByID", func(t *testing.T) {
		a := createTestLocalAccount(t, s, ctx)
		b := createTestLocalAccount(t, s, ctx)
		followID := uid.New()
		_, err := s.CreateFollow(ctx, store.CreateFollowInput{
			ID:        followID,
			AccountID: a.ID,
			TargetID:  b.ID,
			State:     domain.FollowStateAccepted,
		})
		require.NoError(t, err)

		got, err := s.GetFollowByID(ctx, followID)
		require.NoError(t, err)
		assert.Equal(t, a.ID, got.AccountID)
		assert.Equal(t, b.ID, got.TargetID)
	})

	t.Run("GetFollowByID_not_found", func(t *testing.T) {
		_, err := s.GetFollowByID(ctx, "nonexistent_"+uid.New())
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("GetFollowByAPID", func(t *testing.T) {
		a := createTestLocalAccount(t, s, ctx)
		b := createTestLocalAccount(t, s, ctx)
		followID := uid.New()
		apID := "https://local.example/follows/" + followID

		_, err := s.CreateFollow(ctx, store.CreateFollowInput{
			ID:        followID,
			AccountID: a.ID,
			TargetID:  b.ID,
			State:     domain.FollowStateAccepted,
			APID:      &apID,
		})
		require.NoError(t, err)

		got, err := s.GetFollowByAPID(ctx, apID)
		require.NoError(t, err)
		assert.Equal(t, followID, got.ID)
	})

	t.Run("AcceptFollow", func(t *testing.T) {
		a := createTestLocalAccount(t, s, ctx)
		b := createTestLocalAccount(t, s, ctx)
		followID := uid.New()

		_, err := s.CreateFollow(ctx, store.CreateFollowInput{
			ID:        followID,
			AccountID: a.ID,
			TargetID:  b.ID,
			State:     domain.FollowStatePending,
		})
		require.NoError(t, err)

		err = s.AcceptFollow(ctx, followID)
		require.NoError(t, err)

		got, err := s.GetFollowByID(ctx, followID)
		require.NoError(t, err)
		assert.Equal(t, domain.FollowStateAccepted, got.State)
	})

	t.Run("DeleteFollow", func(t *testing.T) {
		a := createTestLocalAccount(t, s, ctx)
		b := createTestLocalAccount(t, s, ctx)
		_, err := s.CreateFollow(ctx, store.CreateFollowInput{
			ID:        uid.New(),
			AccountID: a.ID,
			TargetID:  b.ID,
			State:     domain.FollowStateAccepted,
		})
		require.NoError(t, err)

		err = s.DeleteFollow(ctx, a.ID, b.ID)
		require.NoError(t, err)

		_, err = s.GetFollow(ctx, a.ID, b.ID)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("GetFollowers_GetFollowing", func(t *testing.T) {
		a := createTestLocalAccount(t, s, ctx)
		b := createTestLocalAccount(t, s, ctx)

		_, err := s.CreateFollow(ctx, store.CreateFollowInput{
			ID:        uid.New(),
			AccountID: a.ID,
			TargetID:  b.ID,
			State:     domain.FollowStateAccepted,
		})
		require.NoError(t, err)

		followers, err := s.GetFollowers(ctx, b.ID, nil, 10)
		require.NoError(t, err)
		assert.NotEmpty(t, followers)
		found := false
		for _, f := range followers {
			if f.ID == a.ID {
				found = true
			}
		}
		assert.True(t, found, "follower not found")

		following, err := s.GetFollowing(ctx, a.ID, nil, 10)
		require.NoError(t, err)
		assert.NotEmpty(t, following)
		found = false
		for _, f := range following {
			if f.ID == b.ID {
				found = true
			}
		}
		assert.True(t, found, "following not found")
	})

	t.Run("CountFollowers_CountFollowing", func(t *testing.T) {
		a := createTestLocalAccount(t, s, ctx)
		b := createTestLocalAccount(t, s, ctx)

		_, err := s.CreateFollow(ctx, store.CreateFollowInput{
			ID:        uid.New(),
			AccountID: a.ID,
			TargetID:  b.ID,
			State:     domain.FollowStateAccepted,
		})
		require.NoError(t, err)

		err = s.IncrementFollowersCount(ctx, b.ID)
		require.NoError(t, err)
		err = s.IncrementFollowingCount(ctx, a.ID)
		require.NoError(t, err)

		fCount, err := s.CountFollowers(ctx, b.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(1), fCount)

		gCount, err := s.CountFollowing(ctx, a.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(1), gCount)

		err = s.DecrementFollowersCount(ctx, b.ID)
		require.NoError(t, err)
		err = s.DecrementFollowingCount(ctx, a.ID)
		require.NoError(t, err)
	})

	t.Run("GetFollowerInboxURLs", func(t *testing.T) {
		a := createTestRemoteAccount(t, s, ctx)
		b := createTestLocalAccount(t, s, ctx)

		_, err := s.CreateFollow(ctx, store.CreateFollowInput{
			ID:        uid.New(),
			AccountID: a.ID,
			TargetID:  b.ID,
			State:     domain.FollowStateAccepted,
		})
		require.NoError(t, err)

		urls, err := s.GetFollowerInboxURLs(ctx, b.ID)
		require.NoError(t, err)
		assert.Contains(t, urls, a.InboxURL)
	})

	t.Run("GetDistinctFollowerInboxURLsPaginated", func(t *testing.T) {
		a := createTestRemoteAccount(t, s, ctx)
		b := createTestLocalAccount(t, s, ctx)

		_, err := s.CreateFollow(ctx, store.CreateFollowInput{
			ID:        uid.New(),
			AccountID: a.ID,
			TargetID:  b.ID,
			State:     domain.FollowStateAccepted,
		})
		require.NoError(t, err)

		urls, err := s.GetDistinctFollowerInboxURLsPaginated(ctx, b.ID, "", 100)
		require.NoError(t, err)
		assert.NotEmpty(t, urls)
	})

	t.Run("GetLocalFollowerAccountIDs", func(t *testing.T) {
		a := createTestLocalAccount(t, s, ctx)
		b := createTestLocalAccount(t, s, ctx)

		_, err := s.CreateFollow(ctx, store.CreateFollowInput{
			ID:        uid.New(),
			AccountID: a.ID,
			TargetID:  b.ID,
			State:     domain.FollowStateAccepted,
		})
		require.NoError(t, err)

		ids, err := s.GetLocalFollowerAccountIDs(ctx, b.ID)
		require.NoError(t, err)
		assert.Contains(t, ids, a.ID)
	})

	t.Run("GetPendingFollowRequests", func(t *testing.T) {
		a := createTestLocalAccount(t, s, ctx)
		b := createTestLocalAccount(t, s, ctx)

		_, err := s.CreateFollow(ctx, store.CreateFollowInput{
			ID:        uid.New(),
			AccountID: a.ID,
			TargetID:  b.ID,
			State:     domain.FollowStatePending,
		})
		require.NoError(t, err)

		reqs, _, err := s.GetPendingFollowRequests(ctx, b.ID, nil, 10)
		require.NoError(t, err)
		found := false
		for _, r := range reqs {
			if r.ID == a.ID {
				found = true
			}
		}
		assert.True(t, found, "pending request not found")
	})

	t.Run("CreateFollow_duplicate_conflict", func(t *testing.T) {
		a := createTestLocalAccount(t, s, ctx)
		b := createTestLocalAccount(t, s, ctx)
		apID1 := "https://local.example/follows/" + uid.New()
		apID2 := "https://local.example/follows/" + uid.New()

		_, err := s.CreateFollow(ctx, store.CreateFollowInput{
			ID:        uid.New(),
			AccountID: a.ID,
			TargetID:  b.ID,
			State:     domain.FollowStateAccepted,
			APID:      testutil.StrPtr(apID1),
		})
		require.NoError(t, err)

		_, err = s.CreateFollow(ctx, store.CreateFollowInput{
			ID:        uid.New(),
			AccountID: a.ID,
			TargetID:  b.ID,
			State:     domain.FollowStatePending,
			APID:      testutil.StrPtr(apID2),
		})
		require.ErrorIs(t, err, domain.ErrConflict)
	})
}
