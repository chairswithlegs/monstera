package service

import (
	"context"
	"testing"

	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchService_Search(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	svc := NewSearchService(st, nil, nil)

	t.Run("empty q returns empty result", func(t *testing.T) {
		res, err := svc.Search(ctx, nil, "   ", SearchTypeAll, false, false, 5, 0)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Empty(t, res.Accounts)
		assert.Empty(t, res.Statuses)
		assert.Empty(t, res.Hashtags)
	})

	t.Run("SearchTypeAccounts returns only accounts", func(t *testing.T) {
		acc, err := st.CreateAccount(ctx, store.CreateAccountInput{
			ID: "01alice", Username: "alice", APID: "https://example.com/users/alice",
		})
		require.NoError(t, err)
		_, _ = st.GetOrCreateHashtag(ctx, "foo")
		res, err := svc.Search(ctx, nil, "alice", SearchTypeAccounts, false, false, 5, 0)
		require.NoError(t, err)
		require.Len(t, res.Accounts, 1)
		assert.Equal(t, acc.ID, res.Accounts[0].ID)
		assert.Empty(t, res.Hashtags)
		assert.Empty(t, res.Statuses)
	})

	t.Run("SearchTypeHashtags returns only hashtags", func(t *testing.T) {
		_, _ = st.GetOrCreateHashtag(ctx, "golang")
		res, err := svc.Search(ctx, nil, "go", SearchTypeHashtags, false, false, 5, 0)
		require.NoError(t, err)
		assert.Empty(t, res.Accounts)
		require.Len(t, res.Hashtags, 1)
		assert.Equal(t, "golang", res.Hashtags[0].Name)
		assert.Empty(t, res.Statuses)
	})

	t.Run("SearchTypeAll returns accounts and hashtags", func(t *testing.T) {
		_, _ = st.CreateAccount(ctx, store.CreateAccountInput{
			ID: "01allie", Username: "allie", APID: "https://example.com/users/allie",
		})
		_, _ = st.GetOrCreateHashtag(ctx, "allstar")
		res, err := svc.Search(ctx, nil, "all", SearchTypeAll, false, false, 10, 0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(res.Accounts), 1)
		assert.GreaterOrEqual(t, len(res.Hashtags), 1)
		assert.Empty(t, res.Statuses)
	})

	t.Run("limit is respected", func(t *testing.T) {
		res, err := svc.Search(ctx, nil, "a", SearchTypeAll, false, false, 1, 0)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(res.Accounts), 1)
		assert.LessOrEqual(t, len(res.Hashtags), 1)
	})

	t.Run("offset skips leading results", func(t *testing.T) {
		_, err := st.CreateAccount(ctx, store.CreateAccountInput{
			ID: "01page1", Username: "pageacc1", APID: "https://example.com/users/pageacc1",
		})
		require.NoError(t, err)
		_, err = st.CreateAccount(ctx, store.CreateAccountInput{
			ID: "01page2", Username: "pageacc2", APID: "https://example.com/users/pageacc2",
		})
		require.NoError(t, err)

		// Full result set has both accounts.
		all, err := svc.Search(ctx, nil, "pageacc", SearchTypeAccounts, false, false, 40, 0)
		require.NoError(t, err)
		require.Len(t, all.Accounts, 2)

		// Skipping past all results returns empty.
		none, err := svc.Search(ctx, nil, "pageacc", SearchTypeAccounts, false, false, 40, 2)
		require.NoError(t, err)
		assert.Empty(t, none.Accounts)
	})

	t.Run("following=true restricts to followed accounts", func(t *testing.T) {
		viewer, err := st.CreateAccount(ctx, store.CreateAccountInput{
			ID: "01viewer", Username: "viewer", APID: "https://example.com/users/viewer",
		})
		require.NoError(t, err)
		followed, err := st.CreateAccount(ctx, store.CreateAccountInput{
			ID: "01followed", Username: "followed_one", APID: "https://example.com/users/followed_one",
		})
		require.NoError(t, err)
		_, err = st.CreateAccount(ctx, store.CreateAccountInput{
			ID: "01notfollowed", Username: "followed_two", APID: "https://example.com/users/followed_two",
		})
		require.NoError(t, err)
		_, err = st.CreateFollow(ctx, store.CreateFollowInput{
			ID: "01f1", AccountID: viewer.ID, TargetID: followed.ID, State: "accepted",
		})
		require.NoError(t, err)

		res, err := svc.Search(ctx, viewer, "followed", SearchTypeAccounts, false, true, 10, 0)
		require.NoError(t, err)
		require.Len(t, res.Accounts, 1)
		assert.Equal(t, followed.ID, res.Accounts[0].ID)
	})
}
