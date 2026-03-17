//go:build integration

package postgres

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
)

func TestIntegration_ContentStore_Mentions(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("CreateStatusMention_GetStatusMentions", func(t *testing.T) {
		author := createTestLocalAccount(t, s, ctx)
		mentioned := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, author.ID)

		err := s.CreateStatusMention(ctx, st.ID, mentioned.ID)
		require.NoError(t, err)

		mentions, err := s.GetStatusMentions(ctx, st.ID)
		require.NoError(t, err)
		assert.Len(t, mentions, 1)
		assert.Equal(t, mentioned.ID, mentions[0].ID)
	})

	t.Run("GetStatusMentionAccountIDs", func(t *testing.T) {
		author := createTestLocalAccount(t, s, ctx)
		m1 := createTestLocalAccount(t, s, ctx)
		m2 := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, author.ID)

		err := s.CreateStatusMention(ctx, st.ID, m1.ID)
		require.NoError(t, err)
		err = s.CreateStatusMention(ctx, st.ID, m2.ID)
		require.NoError(t, err)

		ids, err := s.GetStatusMentionAccountIDs(ctx, st.ID)
		require.NoError(t, err)
		assert.Len(t, ids, 2)
		assert.Contains(t, ids, m1.ID)
		assert.Contains(t, ids, m2.ID)
	})

	t.Run("DeleteStatusMentions", func(t *testing.T) {
		author := createTestLocalAccount(t, s, ctx)
		mentioned := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, author.ID)

		err := s.CreateStatusMention(ctx, st.ID, mentioned.ID)
		require.NoError(t, err)

		err = s.DeleteStatusMentions(ctx, st.ID)
		require.NoError(t, err)

		mentions, err := s.GetStatusMentions(ctx, st.ID)
		require.NoError(t, err)
		assert.Empty(t, mentions)
	})
}

func TestIntegration_ContentStore_Hashtags(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("GetOrCreateHashtag_idempotent", func(t *testing.T) {
		name := "testhashtag_" + uid.New()[:8]
		h1, err := s.GetOrCreateHashtag(ctx, name)
		require.NoError(t, err)
		assert.Equal(t, strings.ToLower(name), h1.Name)

		h2, err := s.GetOrCreateHashtag(ctx, name)
		require.NoError(t, err)
		assert.Equal(t, h1.ID, h2.ID)
	})

	t.Run("SearchHashtagsByPrefix", func(t *testing.T) {
		prefix := "srch_" + uid.New()[:6]
		_, err := s.GetOrCreateHashtag(ctx, prefix+"alpha")
		require.NoError(t, err)
		_, err = s.GetOrCreateHashtag(ctx, prefix+"beta")
		require.NoError(t, err)

		results, err := s.SearchHashtagsByPrefix(ctx, prefix, 10)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(results), 2)
	})

	t.Run("AttachHashtagsToStatus_GetStatusHashtags", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, acc.ID)
		h, err := s.GetOrCreateHashtag(ctx, "attach_"+uid.New()[:8])
		require.NoError(t, err)

		err = s.AttachHashtagsToStatus(ctx, st.ID, []string{h.ID})
		require.NoError(t, err)

		tags, err := s.GetStatusHashtags(ctx, st.ID)
		require.NoError(t, err)
		assert.Len(t, tags, 1)
		assert.Equal(t, h.ID, tags[0].ID)
	})

	t.Run("DeleteStatusHashtags", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, acc.ID)
		h, err := s.GetOrCreateHashtag(ctx, "del_"+uid.New()[:8])
		require.NoError(t, err)
		err = s.AttachHashtagsToStatus(ctx, st.ID, []string{h.ID})
		require.NoError(t, err)

		err = s.DeleteStatusHashtags(ctx, st.ID)
		require.NoError(t, err)

		tags, err := s.GetStatusHashtags(ctx, st.ID)
		require.NoError(t, err)
		assert.Empty(t, tags)
	})

	t.Run("FollowTag_ListFollowedTags_UnfollowTag", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		h, err := s.GetOrCreateHashtag(ctx, "foltag_"+uid.New()[:8])
		require.NoError(t, err)

		err = s.FollowTag(ctx, uid.New(), acc.ID, h.ID)
		require.NoError(t, err)

		tags, _, err := s.ListFollowedTags(ctx, acc.ID, nil, 10)
		require.NoError(t, err)
		found := false
		for _, tag := range tags {
			if tag.ID == h.ID {
				found = true
			}
		}
		assert.True(t, found, "followed tag not in list")

		err = s.UnfollowTag(ctx, acc.ID, h.ID)
		require.NoError(t, err)

		tags, _, err = s.ListFollowedTags(ctx, acc.ID, nil, 10)
		require.NoError(t, err)
		for _, tag := range tags {
			assert.NotEqual(t, h.ID, tag.ID, "unfollowed tag still in list")
		}
	})

	t.Run("FeaturedTag_CRUD", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		h, err := s.GetOrCreateHashtag(ctx, "feat_"+uid.New()[:8])
		require.NoError(t, err)
		ftID := uid.New()

		err = s.CreateFeaturedTag(ctx, ftID, acc.ID, h.ID)
		require.NoError(t, err)

		got, err := s.GetFeaturedTagByID(ctx, ftID, acc.ID)
		require.NoError(t, err)
		assert.Equal(t, ftID, got.ID)
		assert.Equal(t, h.Name, got.Name)

		tags, err := s.ListFeaturedTags(ctx, acc.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, tags)

		err = s.DeleteFeaturedTag(ctx, ftID, acc.ID)
		require.NoError(t, err)

		_, err = s.GetFeaturedTagByID(ctx, ftID, acc.ID)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestIntegration_ContentStore_Conversations(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("CreateConversation_SetStatusConversationID_GetStatusConversationID", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, acc.ID)
		convID := uid.New()

		err := s.CreateConversation(ctx, convID)
		require.NoError(t, err)

		err = s.SetStatusConversationID(ctx, st.ID, convID)
		require.NoError(t, err)

		gotID, err := s.GetStatusConversationID(ctx, st.ID)
		require.NoError(t, err)
		require.NotNil(t, gotID)
		assert.Equal(t, convID, *gotID)
	})

	t.Run("GetStatusConversationID_nil_when_unset", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, acc.ID)

		gotID, err := s.GetStatusConversationID(ctx, st.ID)
		require.NoError(t, err)
		assert.Nil(t, gotID)
	})

	t.Run("ConversationMute_CRUD", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		convID := uid.New()
		err := s.CreateConversation(ctx, convID)
		require.NoError(t, err)

		err = s.CreateConversationMute(ctx, acc.ID, convID)
		require.NoError(t, err)

		muted, err := s.IsConversationMuted(ctx, acc.ID, convID)
		require.NoError(t, err)
		assert.True(t, muted)

		err = s.DeleteConversationMute(ctx, acc.ID, convID)
		require.NoError(t, err)

		muted, err = s.IsConversationMuted(ctx, acc.ID, convID)
		require.NoError(t, err)
		assert.False(t, muted)
	})

	t.Run("AccountConversation_upsert_list_read_delete", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, acc.ID)
		convID := uid.New()
		acID := uid.New()

		err := s.CreateConversation(ctx, convID)
		require.NoError(t, err)

		err = s.UpsertAccountConversation(ctx, store.UpsertAccountConversationInput{
			ID:             acID,
			AccountID:      acc.ID,
			ConversationID: convID,
			LastStatusID:   st.ID,
			Unread:         true,
		})
		require.NoError(t, err)

		got, err := s.GetAccountConversation(ctx, acc.ID, convID)
		require.NoError(t, err)
		assert.Equal(t, acID, got.ID)
		assert.True(t, got.Unread)

		listed, _, err := s.ListAccountConversations(ctx, acc.ID, nil, 10)
		require.NoError(t, err)
		assert.NotEmpty(t, listed)

		err = s.MarkAccountConversationRead(ctx, acc.ID, convID)
		require.NoError(t, err)

		got, err = s.GetAccountConversation(ctx, acc.ID, convID)
		require.NoError(t, err)
		assert.False(t, got.Unread)

		err = s.DeleteAccountConversation(ctx, acc.ID, convID)
		require.NoError(t, err)

		_, err = s.GetAccountConversation(ctx, acc.ID, convID)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})
}
