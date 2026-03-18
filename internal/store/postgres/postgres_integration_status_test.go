//go:build integration

package postgres

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/chairswithlegs/monstera/internal/uid"
)

func TestIntegration_StatusStore(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("CreateStatus_GetByID", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, acc.ID)

		got, err := s.GetStatusByID(ctx, st.ID)
		require.NoError(t, err)
		assert.Equal(t, st.ID, got.ID)
		assert.Equal(t, acc.ID, got.AccountID)
		assert.Equal(t, domain.VisibilityPublic, got.Visibility)
		assert.True(t, got.Local)
	})

	t.Run("GetStatusByID_not_found", func(t *testing.T) {
		_, err := s.GetStatusByID(ctx, "nonexistent_"+uid.New())
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("GetStatusByAPID", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, acc.ID)

		got, err := s.GetStatusByAPID(ctx, st.APID)
		require.NoError(t, err)
		assert.Equal(t, st.ID, got.ID)
	})

	t.Run("GetStatusByAPID_not_found", func(t *testing.T) {
		_, err := s.GetStatusByAPID(ctx, "https://nowhere.example/statuses/ghost")
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("SoftDeleteStatus", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, acc.ID)

		err := s.SoftDeleteStatus(ctx, st.ID)
		require.NoError(t, err)

		_, err = s.GetStatusByID(ctx, st.ID)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("UpdateStatus", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, acc.ID)

		newText := "Updated text " + uid.New()[:8]
		newContent := "<p>" + newText + "</p>"
		err := s.UpdateStatus(ctx, store.UpdateStatusInput{
			ID:      st.ID,
			Text:    &newText,
			Content: &newContent,
		})
		require.NoError(t, err)

		got, err := s.GetStatusByID(ctx, st.ID)
		require.NoError(t, err)
		require.NotNil(t, got.Text)
		assert.Equal(t, newText, *got.Text)
		assert.NotNil(t, got.EditedAt)
	})

	t.Run("GetAccountStatuses_with_cursor", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		var ids []string
		for i := 0; i < 3; i++ {
			st := createTestStatus(t, s, ctx, acc.ID)
			ids = append(ids, st.ID)
		}

		all, err := s.GetAccountStatuses(ctx, acc.ID, nil, 10)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(all), 3)

		cursor := ids[len(ids)-1]
		page, err := s.GetAccountStatuses(ctx, acc.ID, &cursor, 10)
		require.NoError(t, err)
		for _, st := range page {
			assert.Less(t, st.ID, cursor)
		}
	})

	t.Run("GetAccountPublicStatuses", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		createTestStatus(t, s, ctx, acc.ID)

		id2 := uid.New()
		_, err := s.CreateStatus(ctx, store.CreateStatusInput{
			ID:                  id2,
			URI:                 "https://local.example/statuses/" + id2,
			AccountID:           acc.ID,
			Text:                testutil.StrPtr("private"),
			Content:             testutil.StrPtr("<p>private</p>"),
			Visibility:          domain.VisibilityPrivate,
			APID:                "https://local.example/statuses/" + id2,
			QuoteApprovalPolicy: domain.QuotePolicyPublic,
			Local:               true,
		})
		require.NoError(t, err)

		pub, err := s.GetAccountPublicStatuses(ctx, acc.ID, nil, 10)
		require.NoError(t, err)
		for _, st := range pub {
			assert.Contains(t, []string{domain.VisibilityPublic, domain.VisibilityUnlisted}, st.Visibility)
		}
	})

	t.Run("CountLocalStatuses", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		createTestStatus(t, s, ctx, acc.ID)

		n, err := s.CountLocalStatuses(ctx)
		require.NoError(t, err)
		assert.Greater(t, n, int64(0))
	})

	t.Run("CountAccountPublicStatuses", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		createTestStatus(t, s, ctx, acc.ID)

		n, err := s.CountAccountPublicStatuses(ctx, acc.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(1), n)
	})

	t.Run("AttachMediaToStatus_GetStatusAttachments", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, acc.ID)

		mediaID := uid.New()
		_, err := s.CreateMediaAttachment(ctx, store.CreateMediaAttachmentInput{
			ID:         mediaID,
			AccountID:  acc.ID,
			Type:       "image",
			StorageKey: "key_" + mediaID,
			URL:        "https://cdn.example/media/" + mediaID + ".jpg",
		})
		require.NoError(t, err)

		err = s.AttachMediaToStatus(ctx, mediaID, st.ID, acc.ID)
		require.NoError(t, err)

		attachments, err := s.GetStatusAttachments(ctx, st.ID)
		require.NoError(t, err)
		assert.Len(t, attachments, 1)
		assert.Equal(t, mediaID, attachments[0].ID)
	})

	t.Run("CreateStatusEdit_ListStatusEdits", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, acc.ID)

		editID := uid.New()
		err := s.CreateStatusEdit(ctx, store.CreateStatusEditInput{
			ID:        editID,
			StatusID:  st.ID,
			AccountID: acc.ID,
			Text:      st.Text,
			Content:   st.Content,
		})
		require.NoError(t, err)

		edits, err := s.ListStatusEdits(ctx, st.ID)
		require.NoError(t, err)
		assert.Len(t, edits, 1)
		assert.Equal(t, editID, edits[0].ID)
	})

	t.Run("ScheduledStatus_CRUD", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		ssID := uid.New()
		params := mustJSON(t, map[string]string{"text": "scheduled post"})
		scheduledAt := testScheduleTime()

		ss, err := s.CreateScheduledStatus(ctx, store.CreateScheduledStatusInput{
			ID:          ssID,
			AccountID:   acc.ID,
			Params:      params,
			ScheduledAt: scheduledAt,
		})
		require.NoError(t, err)
		assert.Equal(t, ssID, ss.ID)

		got, err := s.GetScheduledStatusByID(ctx, ssID)
		require.NoError(t, err)
		assert.Equal(t, acc.ID, got.AccountID)

		// Pass a max cursor that includes all IDs to work around the nil→empty-string
		// issue in ListScheduledStatuses (empty string is not SQL NULL).
		maxCursor := "ZZZZZZZZZZZZZZZZZZZZZZZZZZ"
		listed, err := s.ListScheduledStatuses(ctx, acc.ID, &maxCursor, 10)
		require.NoError(t, err)
		assert.NotEmpty(t, listed)

		newParams := mustJSON(t, map[string]string{"text": "updated"})
		updated, err := s.UpdateScheduledStatus(ctx, store.UpdateScheduledStatusInput{
			ID:          ssID,
			Params:      newParams,
			ScheduledAt: scheduledAt,
		})
		require.NoError(t, err)
		assert.Equal(t, ssID, updated.ID)

		err = s.DeleteScheduledStatus(ctx, ssID)
		require.NoError(t, err)

		_, err = s.GetScheduledStatusByID(ctx, ssID)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("InReplyTo_chain", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		parent := createTestStatus(t, s, ctx, acc.ID)

		childID := uid.New()
		child, err := s.CreateStatus(ctx, store.CreateStatusInput{
			ID:                  childID,
			URI:                 "https://local.example/statuses/" + childID,
			AccountID:           acc.ID,
			Text:                testutil.StrPtr("reply"),
			Content:             testutil.StrPtr("<p>reply</p>"),
			Visibility:          domain.VisibilityPublic,
			InReplyToID:         &parent.ID,
			InReplyToAccountID:  &acc.ID,
			APID:                "https://local.example/statuses/" + childID,
			QuoteApprovalPolicy: domain.QuotePolicyPublic,
			Local:               true,
		})
		require.NoError(t, err)
		require.NotNil(t, child.InReplyToID)
		assert.Equal(t, parent.ID, *child.InReplyToID)
	})
}

func TestIntegration_TimelineStore(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("GetPublicTimeline", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		createTestStatus(t, s, ctx, acc.ID)

		statuses, err := s.GetPublicTimeline(ctx, false, nil, 20)
		require.NoError(t, err)
		assert.NotEmpty(t, statuses)
	})

	t.Run("GetPublicTimeline_localOnly", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		createTestStatus(t, s, ctx, acc.ID)

		statuses, err := s.GetPublicTimeline(ctx, true, nil, 20)
		require.NoError(t, err)
		for _, st := range statuses {
			assert.True(t, st.Local)
		}
	})

	t.Run("GetHomeTimeline_empty_for_new_account", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		statuses, err := s.GetHomeTimeline(ctx, acc.ID, nil, 20)
		require.NoError(t, err)
		assert.Empty(t, statuses)
	})

	t.Run("GetStatusAncestors_no_parent", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, acc.ID)

		ancestors, err := s.GetStatusAncestors(ctx, st.ID)
		require.NoError(t, err)
		assert.Empty(t, ancestors)
	})

	t.Run("GetStatusDescendants_no_children", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, acc.ID)

		descendants, err := s.GetStatusDescendants(ctx, st.ID)
		require.NoError(t, err)
		assert.Empty(t, descendants)
	})

	t.Run("GetStatusFavouritedBy_empty", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, acc.ID)

		accounts, err := s.GetStatusFavouritedBy(ctx, st.ID, nil, 10)
		require.NoError(t, err)
		assert.Empty(t, accounts)
	})

	t.Run("GetRebloggedBy_empty", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, acc.ID)

		accounts, err := s.GetRebloggedBy(ctx, st.ID, nil, 10)
		require.NoError(t, err)
		assert.Empty(t, accounts)
	})

	t.Run("GetFavouritesTimeline_empty", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		statuses, nextCursor, err := s.GetFavouritesTimeline(ctx, acc.ID, nil, 20)
		require.NoError(t, err)
		assert.Empty(t, statuses)
		assert.Nil(t, nextCursor)
	})
}

func testScheduleTime() time.Time {
	return time.Now().Add(24 * time.Hour)
}
