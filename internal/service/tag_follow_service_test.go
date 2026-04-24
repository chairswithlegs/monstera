package service

import (
	"context"
	"errors"
	"testing"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTagFollowService_FollowTag_succeeds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewTagFollowService(fake, 0)

	tag, err := svc.FollowTag(ctx, "acct1", "golang")
	require.NoError(t, err)
	require.NotNil(t, tag)
	assert.Equal(t, "golang", tag.Name)
	assert.NotEmpty(t, tag.ID)

	tags, _, err := svc.ListFollowedTags(ctx, "acct1", nil, 10)
	require.NoError(t, err)
	require.Len(t, tags, 1)
	assert.Equal(t, "golang", tags[0].Name)
}

func TestTagFollowService_FollowTag_enforces_max_tags_cap(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewTagFollowService(fake, 2)

	_, err := svc.FollowTag(ctx, "acct1", "go")
	require.NoError(t, err)
	_, err = svc.FollowTag(ctx, "acct1", "rust")
	require.NoError(t, err)

	// Third follow should trip the cap.
	tag, err := svc.FollowTag(ctx, "acct1", "zig")
	require.Error(t, err)
	assert.Nil(t, tag)
	require.ErrorIs(t, err, domain.ErrFollowedTagLimitReached)

	// A different account is not affected by the first account's count.
	_, err = svc.FollowTag(ctx, "acct2", "go")
	require.NoError(t, err)
}

func TestTagFollowService_FollowTag_zero_cap_disables_enforcement(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewTagFollowService(fake, 0)

	for _, name := range []string{"a", "b", "c", "d", "e"} {
		_, err := svc.FollowTag(ctx, "acct1", name)
		require.NoError(t, err)
	}
	tags, _, err := svc.ListFollowedTags(ctx, "acct1", nil, 10)
	require.NoError(t, err)
	assert.Len(t, tags, 5)
}

func TestTagFollowService_FollowTag_empty_name_returns_validation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewTagFollowService(fake, 0)

	tag, err := svc.FollowTag(ctx, "acct1", "")
	require.Error(t, err)
	assert.Nil(t, tag)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestTagFollowService_FollowTag_whitespace_only_returns_validation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewTagFollowService(fake, 0)

	tag, err := svc.FollowTag(ctx, "acct1", "   ")
	require.Error(t, err)
	assert.Nil(t, tag)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestTagFollowService_UnfollowTag_succeeds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewTagFollowService(fake, 0)

	tag, err := svc.FollowTag(ctx, "acct1", "rust")
	require.NoError(t, err)
	require.NotNil(t, tag)

	err = svc.UnfollowTag(ctx, "acct1", tag.ID)
	require.NoError(t, err)

	tags, _, err := svc.ListFollowedTags(ctx, "acct1", nil, 10)
	require.NoError(t, err)
	assert.Empty(t, tags)
}

func TestTagFollowService_UnfollowTag_store_fails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	tag, err := fake.GetOrCreateHashtag(ctx, "foo")
	require.NoError(t, err)
	require.NoError(t, fake.FollowTag(ctx, "row1", "acct1", tag.ID))

	failingStore := &unfollowFailingStore{FakeStore: fake}
	svc := NewTagFollowService(failingStore, 0)

	err = svc.UnfollowTag(ctx, "acct1", tag.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "UnfollowTag")
}

type unfollowFailingStore struct {
	*testutil.FakeStore
}

func (s *unfollowFailingStore) UnfollowTag(ctx context.Context, accountID, tagID string) error {
	return errors.New("db write failed")
}

func TestTagFollowService_GetTagByName_succeeds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewTagFollowService(fake, 0)

	_, err := svc.FollowTag(ctx, "acct1", "golang")
	require.NoError(t, err)

	tag, err := svc.GetTagByName(ctx, "golang")
	require.NoError(t, err)
	assert.Equal(t, "golang", tag.Name)
}

func TestTagFollowService_GetTagByName_not_found(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewTagFollowService(fake, 0)

	_, err := svc.GetTagByName(ctx, "nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestTagFollowService_GetTagByName_empty_returns_validation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewTagFollowService(fake, 0)

	_, err := svc.GetTagByName(ctx, "")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestTagFollowService_IsFollowingTag(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewTagFollowService(fake, 0)

	tag, err := svc.FollowTag(ctx, "acct1", "golang")
	require.NoError(t, err)

	following, err := svc.IsFollowingTag(ctx, "acct1", tag.ID)
	require.NoError(t, err)
	assert.True(t, following)

	following, err = svc.IsFollowingTag(ctx, "acct2", tag.ID)
	require.NoError(t, err)
	assert.False(t, following)
}

func TestTagFollowService_UnfollowTagByName_succeeds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewTagFollowService(fake, 0)

	_, err := svc.FollowTag(ctx, "acct1", "rust")
	require.NoError(t, err)

	tag, err := svc.UnfollowTagByName(ctx, "acct1", "rust")
	require.NoError(t, err)
	assert.Equal(t, "rust", tag.Name)

	tags, _, err := svc.ListFollowedTags(ctx, "acct1", nil, 10)
	require.NoError(t, err)
	assert.Empty(t, tags)
}

func TestTagFollowService_UnfollowTagByName_not_found(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewTagFollowService(fake, 0)

	_, err := svc.UnfollowTagByName(ctx, "acct1", "nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestTagFollowService_ListFollowedTags_empty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewTagFollowService(fake, 0)

	tags, next, err := svc.ListFollowedTags(ctx, "acct1", nil, 10)
	require.NoError(t, err)
	assert.Empty(t, tags)
	assert.Nil(t, next)
}

func TestTagFollowService_ListFollowedTags_with_results(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewTagFollowService(fake, 0)

	_, err := svc.FollowTag(ctx, "acct1", "golang")
	require.NoError(t, err)
	_, err = svc.FollowTag(ctx, "acct1", "rust")
	require.NoError(t, err)

	tags, next, err := svc.ListFollowedTags(ctx, "acct1", nil, 10)
	require.NoError(t, err)
	require.Len(t, tags, 2)
	assert.Nil(t, next)

	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = t.Name
	}
	assert.Contains(t, names, "golang")
	assert.Contains(t, names, "rust")
}

func TestTagFollowService_AreFollowingTagsByName_empty_input(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc := NewTagFollowService(testutil.NewFakeStore(), 0)

	result, err := svc.AreFollowingTagsByName(ctx, "acct1", []string{})
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestTagFollowService_AreFollowingTagsByName_mixed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := NewTagFollowService(fake, 0)

	_, err := svc.FollowTag(ctx, "acct1", "golang")
	require.NoError(t, err)

	result, err := svc.AreFollowingTagsByName(ctx, "acct1", []string{"golang", "rust"})
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"golang": true, "rust": false}, result)
}

func TestTagFollowService_AreFollowingTagsByName_store_error(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	failingStore := &areFollowingTagsByNameFailingStore{FakeStore: testutil.NewFakeStore()}
	svc := NewTagFollowService(failingStore, 0)

	_, err := svc.AreFollowingTagsByName(ctx, "acct1", []string{"golang"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AreFollowingTagsByName")
}

type areFollowingTagsByNameFailingStore struct {
	*testutil.FakeStore
}

func (s *areFollowingTagsByNameFailingStore) AreFollowingTagsByName(_ context.Context, _ string, _ []string) (map[string]bool, error) {
	return nil, errors.New("db read failed")
}
