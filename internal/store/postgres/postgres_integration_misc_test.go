//go:build integration

package postgres

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/chairswithlegs/monstera/internal/uid"
)

// ---------------------------------------------------------------------------
// ListStore
// ---------------------------------------------------------------------------

func TestIntegration_ListStore(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("CRUD", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		listID := uid.New()

		lst, err := s.CreateList(ctx, store.CreateListInput{
			ID:            listID,
			AccountID:     acc.ID,
			Title:         "My List " + listID[:8],
			RepliesPolicy: domain.ListRepliesPolicyFollowed,
			Exclusive:     false,
		})
		require.NoError(t, err)
		assert.Equal(t, listID, lst.ID)

		got, err := s.GetListByID(ctx, listID)
		require.NoError(t, err)
		assert.Equal(t, lst.Title, got.Title)

		lists, err := s.ListLists(ctx, acc.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, lists)

		updatedTitle := "Updated " + uid.New()[:8]
		updated, err := s.UpdateList(ctx, store.UpdateListInput{
			ID:            listID,
			Title:         updatedTitle,
			RepliesPolicy: domain.ListRepliesPolicyNone,
			Exclusive:     true,
		})
		require.NoError(t, err)
		assert.Equal(t, updatedTitle, updated.Title)
		assert.True(t, updated.Exclusive)

		err = s.DeleteList(ctx, listID)
		require.NoError(t, err)

		_, err = s.GetListByID(ctx, listID)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("GetListByID_not_found", func(t *testing.T) {
		_, err := s.GetListByID(ctx, "nonexistent_"+uid.New())
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("AddAccountToList_ListAccountIDs_RemoveAccount", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		member := createTestLocalAccount(t, s, ctx)
		listID := uid.New()

		_, err := s.CreateList(ctx, store.CreateListInput{
			ID:            listID,
			AccountID:     acc.ID,
			Title:         "Members " + listID[:8],
			RepliesPolicy: domain.ListRepliesPolicyFollowed,
		})
		require.NoError(t, err)

		err = s.AddAccountToList(ctx, listID, member.ID)
		require.NoError(t, err)

		ids, err := s.ListListAccountIDs(ctx, listID)
		require.NoError(t, err)
		assert.Contains(t, ids, member.ID)

		err = s.RemoveAccountFromList(ctx, listID, member.ID)
		require.NoError(t, err)

		ids, err = s.ListListAccountIDs(ctx, listID)
		require.NoError(t, err)
		assert.NotContains(t, ids, member.ID)
	})

	t.Run("GetListTimeline_empty", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		listID := uid.New()
		_, err := s.CreateList(ctx, store.CreateListInput{
			ID:            listID,
			AccountID:     acc.ID,
			Title:         "Timeline " + listID[:8],
			RepliesPolicy: domain.ListRepliesPolicyFollowed,
		})
		require.NoError(t, err)

		statuses, err := s.GetListTimeline(ctx, listID, nil, 20)
		require.NoError(t, err)
		assert.Empty(t, statuses)
	})
}

// ---------------------------------------------------------------------------
// FilterStore
// ---------------------------------------------------------------------------

func TestIntegration_FilterStore(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("CRUD", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		filterID := uid.New()

		f, err := s.CreateUserFilter(ctx, store.CreateUserFilterInput{
			ID:        filterID,
			AccountID: acc.ID,
			Phrase:    "spoiler_" + uid.New()[:8],
			Context:   []string{domain.FilterContextHome, domain.FilterContextPublic},
			WholeWord: true,
		})
		require.NoError(t, err)
		assert.Equal(t, filterID, f.ID)
		assert.True(t, f.WholeWord)

		got, err := s.GetUserFilterByID(ctx, filterID)
		require.NoError(t, err)
		assert.Equal(t, f.Phrase, got.Phrase)
		assert.ElementsMatch(t, []string{domain.FilterContextHome, domain.FilterContextPublic}, got.Context)

		filters, err := s.ListUserFilters(ctx, acc.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, filters)

		newPhrase := "updated_" + uid.New()[:8]
		updated, err := s.UpdateUserFilter(ctx, store.UpdateUserFilterInput{
			ID:        filterID,
			Phrase:    newPhrase,
			Context:   []string{domain.FilterContextNotifications},
			WholeWord: false,
		})
		require.NoError(t, err)
		assert.Equal(t, newPhrase, updated.Phrase)
		assert.False(t, updated.WholeWord)

		err = s.DeleteUserFilter(ctx, filterID)
		require.NoError(t, err)

		_, err = s.GetUserFilterByID(ctx, filterID)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("GetUserFilterByID_not_found", func(t *testing.T) {
		_, err := s.GetUserFilterByID(ctx, "nonexistent_"+uid.New())
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("GetActiveUserFiltersByContext", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)

		_, err := s.CreateUserFilter(ctx, store.CreateUserFilterInput{
			ID:        uid.New(),
			AccountID: acc.ID,
			Phrase:    "ctxfilter_" + uid.New()[:8],
			Context:   []string{domain.FilterContextHome},
			WholeWord: false,
		})
		require.NoError(t, err)

		active, err := s.GetActiveUserFiltersByContext(ctx, acc.ID, domain.FilterContextHome)
		require.NoError(t, err)
		assert.NotEmpty(t, active)
	})
}

// ---------------------------------------------------------------------------
// MarkerStore
// ---------------------------------------------------------------------------

func TestIntegration_MarkerStore(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("SetMarker_GetMarkers", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		lastReadID := uid.New()

		err := s.SetMarker(ctx, acc.ID, "home", lastReadID)
		require.NoError(t, err)

		markers, err := s.GetMarkers(ctx, acc.ID, []string{"home"})
		require.NoError(t, err)
		m, ok := markers["home"]
		require.True(t, ok)
		assert.Equal(t, lastReadID, m.LastReadID)
		assert.Equal(t, 0, m.Version)
	})

	t.Run("SetMarker_increments_version", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		err := s.SetMarker(ctx, acc.ID, "notifications", uid.New())
		require.NoError(t, err)

		err = s.SetMarker(ctx, acc.ID, "notifications", uid.New())
		require.NoError(t, err)

		markers, err := s.GetMarkers(ctx, acc.ID, []string{"notifications"})
		require.NoError(t, err)
		m := markers["notifications"]
		assert.Equal(t, 1, m.Version)
	})

	t.Run("GetMarkers_empty", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		markers, err := s.GetMarkers(ctx, acc.ID, []string{"home"})
		require.NoError(t, err)
		assert.Empty(t, markers)
	})
}

// ---------------------------------------------------------------------------
// PollStore
// ---------------------------------------------------------------------------

func TestIntegration_PollStore(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("CreatePoll_options_vote", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, acc.ID)
		pollID := uid.New()
		expires := time.Now().Add(24 * time.Hour)

		poll, err := s.CreatePoll(ctx, store.CreatePollInput{
			ID:        pollID,
			StatusID:  st.ID,
			ExpiresAt: &expires,
			Multiple:  false,
		})
		require.NoError(t, err)
		assert.Equal(t, pollID, poll.ID)

		opt1ID := uid.New()
		opt1, err := s.CreatePollOption(ctx, store.CreatePollOptionInput{
			ID:       opt1ID,
			PollID:   pollID,
			Title:    "Option A",
			Position: 0,
		})
		require.NoError(t, err)
		assert.Equal(t, "Option A", opt1.Title)

		opt2ID := uid.New()
		_, err = s.CreatePollOption(ctx, store.CreatePollOptionInput{
			ID:       opt2ID,
			PollID:   pollID,
			Title:    "Option B",
			Position: 1,
		})
		require.NoError(t, err)

		options, err := s.ListPollOptions(ctx, pollID)
		require.NoError(t, err)
		assert.Len(t, options, 2)

		voter := createTestLocalAccount(t, s, ctx)
		err = s.CreatePollVote(ctx, uid.New(), pollID, voter.ID, opt1ID)
		require.NoError(t, err)

		voted, err := s.HasVotedOnPoll(ctx, pollID, voter.ID)
		require.NoError(t, err)
		assert.True(t, voted)

		ownVotes, err := s.GetOwnVoteOptionIDs(ctx, pollID, voter.ID)
		require.NoError(t, err)
		assert.Contains(t, ownVotes, opt1ID)

		counts, err := s.GetVoteCountsByPoll(ctx, pollID)
		require.NoError(t, err)
		assert.Equal(t, 1, counts[opt1ID])
		assert.Equal(t, 0, counts[opt2ID])
	})

	t.Run("GetPollByID_not_found", func(t *testing.T) {
		_, err := s.GetPollByID(ctx, "nonexistent_"+uid.New())
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("GetPollByStatusID", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, acc.ID)
		pollID := uid.New()
		_, err := s.CreatePoll(ctx, store.CreatePollInput{
			ID:       pollID,
			StatusID: st.ID,
			Multiple: true,
		})
		require.NoError(t, err)

		got, err := s.GetPollByStatusID(ctx, st.ID)
		require.NoError(t, err)
		assert.Equal(t, pollID, got.ID)
		assert.True(t, got.Multiple)
	})

	t.Run("DeletePollVotesByAccount", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, acc.ID)
		pollID := uid.New()
		_, err := s.CreatePoll(ctx, store.CreatePollInput{
			ID:       pollID,
			StatusID: st.ID,
			Multiple: true,
		})
		require.NoError(t, err)

		optID := uid.New()
		_, err = s.CreatePollOption(ctx, store.CreatePollOptionInput{
			ID: optID, PollID: pollID, Title: "X", Position: 0,
		})
		require.NoError(t, err)

		voter := createTestLocalAccount(t, s, ctx)
		err = s.CreatePollVote(ctx, uid.New(), pollID, voter.ID, optID)
		require.NoError(t, err)

		err = s.DeletePollVotesByAccount(ctx, pollID, voter.ID)
		require.NoError(t, err)

		voted, err := s.HasVotedOnPoll(ctx, pollID, voter.ID)
		require.NoError(t, err)
		assert.False(t, voted)
	})
}

// ---------------------------------------------------------------------------
// CardStore
// ---------------------------------------------------------------------------

func TestIntegration_CardStore(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("UpsertStatusCard_GetStatusCard", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, acc.ID)

		err := s.UpsertStatusCard(ctx, store.UpsertStatusCardInput{
			StatusID:        st.ID,
			ProcessingState: domain.CardStateFetched,
			URL:             "https://example.com/article",
			Title:           "Test Article",
			Description:     "A test card",
			CardType:        "link",
			ProviderName:    "Example",
			ProviderURL:     "https://example.com",
		})
		require.NoError(t, err)

		card, err := s.GetStatusCard(ctx, st.ID)
		require.NoError(t, err)
		assert.Equal(t, "Test Article", card.Title)
		assert.Equal(t, domain.CardStateFetched, card.ProcessingState)
	})

	t.Run("GetStatusCard_not_found", func(t *testing.T) {
		_, err := s.GetStatusCard(ctx, "nonexistent_"+uid.New())
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

}

// ---------------------------------------------------------------------------
// AnnouncementStore
// ---------------------------------------------------------------------------

func TestIntegration_AnnouncementStore(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("CRUD", func(t *testing.T) {
		annID := uid.New()
		now := time.Now()

		ann, err := s.CreateAnnouncement(ctx, store.CreateAnnouncementInput{
			ID:          annID,
			Content:     "Hello everyone! " + uid.New()[:8],
			AllDay:      false,
			PublishedAt: now,
		})
		require.NoError(t, err)
		assert.Equal(t, annID, ann.ID)

		got, err := s.GetAnnouncementByID(ctx, annID)
		require.NoError(t, err)
		assert.Equal(t, ann.Content, got.Content)

		active, err := s.ListActiveAnnouncements(ctx)
		require.NoError(t, err)
		found := false
		for _, a := range active {
			if a.ID == annID {
				found = true
			}
		}
		assert.True(t, found, "announcement not in active list")

		all, err := s.ListAllAnnouncements(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, all)

		newContent := "Updated announcement " + uid.New()[:8]
		err = s.UpdateAnnouncement(ctx, store.UpdateAnnouncementInput{
			ID:          annID,
			Content:     newContent,
			PublishedAt: now,
		})
		require.NoError(t, err)

		got, err = s.GetAnnouncementByID(ctx, annID)
		require.NoError(t, err)
		assert.Equal(t, newContent, got.Content)
	})

	t.Run("GetAnnouncementByID_not_found", func(t *testing.T) {
		_, err := s.GetAnnouncementByID(ctx, "nonexistent_"+uid.New())
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("DismissAnnouncement_ListReadAnnouncementIDs", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		annID := uid.New()
		_, err := s.CreateAnnouncement(ctx, store.CreateAnnouncementInput{
			ID:          annID,
			Content:     "Dismiss test " + uid.New()[:8],
			PublishedAt: time.Now(),
		})
		require.NoError(t, err)

		err = s.DismissAnnouncement(ctx, acc.ID, annID)
		require.NoError(t, err)

		readIDs, err := s.ListReadAnnouncementIDs(ctx, acc.ID)
		require.NoError(t, err)
		assert.Contains(t, readIDs, annID)
	})

	t.Run("Reactions", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		annID := uid.New()
		_, err := s.CreateAnnouncement(ctx, store.CreateAnnouncementInput{
			ID:          annID,
			Content:     "Reaction test " + uid.New()[:8],
			PublishedAt: time.Now(),
		})
		require.NoError(t, err)

		err = s.AddAnnouncementReaction(ctx, annID, acc.ID, "thumbsup")
		require.NoError(t, err)

		counts, err := s.ListAnnouncementReactionCounts(ctx, annID)
		require.NoError(t, err)
		found := false
		for _, c := range counts {
			if c.Name == "thumbsup" {
				found = true
				assert.Equal(t, 1, c.Count)
			}
		}
		assert.True(t, found, "reaction not in counts")

		names, err := s.ListAccountAnnouncementReactionNames(ctx, annID, acc.ID)
		require.NoError(t, err)
		assert.Contains(t, names, "thumbsup")

		err = s.RemoveAnnouncementReaction(ctx, annID, acc.ID, "thumbsup")
		require.NoError(t, err)

		counts, err = s.ListAnnouncementReactionCounts(ctx, annID)
		require.NoError(t, err)
		for _, c := range counts {
			if c.Name == "thumbsup" {
				assert.Equal(t, 0, c.Count)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// NotificationStore
// ---------------------------------------------------------------------------

func TestIntegration_NotificationStore(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("CreateNotification_GetNotification_List", func(t *testing.T) {
		recipient := createTestLocalAccount(t, s, ctx)
		sender := createTestLocalAccount(t, s, ctx)
		notifID := uid.New()

		notif, err := s.CreateNotification(ctx, store.CreateNotificationInput{
			ID:        notifID,
			AccountID: recipient.ID,
			FromID:    sender.ID,
			Type:      domain.NotificationTypeFollow,
		})
		require.NoError(t, err)
		assert.Equal(t, notifID, notif.ID)

		got, err := s.GetNotification(ctx, notifID, recipient.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.NotificationTypeFollow, got.Type)
		assert.False(t, got.Read)

		list, err := s.ListNotifications(ctx, recipient.ID, nil, 10)
		require.NoError(t, err)
		assert.NotEmpty(t, list)
	})

	t.Run("GetNotification_not_found", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		_, err := s.GetNotification(ctx, "nonexistent_"+uid.New(), acc.ID)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("DismissNotification", func(t *testing.T) {
		recipient := createTestLocalAccount(t, s, ctx)
		sender := createTestLocalAccount(t, s, ctx)
		notifID := uid.New()

		_, err := s.CreateNotification(ctx, store.CreateNotificationInput{
			ID:        notifID,
			AccountID: recipient.ID,
			FromID:    sender.ID,
			Type:      domain.NotificationTypeFavourite,
		})
		require.NoError(t, err)

		err = s.DismissNotification(ctx, notifID, recipient.ID)
		require.NoError(t, err)

		_, err = s.GetNotification(ctx, notifID, recipient.ID)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("ClearNotifications", func(t *testing.T) {
		recipient := createTestLocalAccount(t, s, ctx)
		sender := createTestLocalAccount(t, s, ctx)

		_, err := s.CreateNotification(ctx, store.CreateNotificationInput{
			ID:        uid.New(),
			AccountID: recipient.ID,
			FromID:    sender.ID,
			Type:      domain.NotificationTypeMention,
		})
		require.NoError(t, err)

		err = s.ClearNotifications(ctx, recipient.ID)
		require.NoError(t, err)

		list, err := s.ListNotifications(ctx, recipient.ID, nil, 10)
		require.NoError(t, err)
		assert.Empty(t, list)
	})

	t.Run("CreateNotification_with_status", func(t *testing.T) {
		recipient := createTestLocalAccount(t, s, ctx)
		sender := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, sender.ID)
		notifID := uid.New()

		notif, err := s.CreateNotification(ctx, store.CreateNotificationInput{
			ID:        notifID,
			AccountID: recipient.ID,
			FromID:    sender.ID,
			Type:      domain.NotificationTypeMention,
			StatusID:  &st.ID,
		})
		require.NoError(t, err)
		require.NotNil(t, notif.StatusID)
		assert.Equal(t, st.ID, *notif.StatusID)
	})
}

// ---------------------------------------------------------------------------
// OutboxStore
// ---------------------------------------------------------------------------

func TestIntegration_OutboxStore(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("InsertOutboxEvent_GetAndLock_MarkPublished", func(t *testing.T) {
		eventID := uid.New()
		payload := mustJSON(t, map[string]string{"status_id": uid.New()})

		err := s.InsertOutboxEvent(ctx, store.InsertOutboxEventInput{
			ID:            eventID,
			EventType:     domain.EventStatusCreated,
			AggregateType: "status",
			AggregateID:   uid.New(),
			Payload:       payload,
		})
		require.NoError(t, err)

		events, err := s.GetAndLockUnpublishedOutboxEvents(ctx, 10)
		require.NoError(t, err)
		found := false
		for _, e := range events {
			if e.ID == eventID {
				found = true
				assert.Equal(t, domain.EventStatusCreated, e.EventType)
			}
		}
		assert.True(t, found, "outbox event not in unpublished list")

		err = s.MarkOutboxEventsPublished(ctx, []string{eventID})
		require.NoError(t, err)

		events, err = s.GetAndLockUnpublishedOutboxEvents(ctx, 10)
		require.NoError(t, err)
		for _, e := range events {
			assert.NotEqual(t, eventID, e.ID, "published event still returned as unpublished")
		}
	})

	t.Run("DeletePublishedOutboxEventsBefore", func(t *testing.T) {
		err := s.DeletePublishedOutboxEventsBefore(ctx, time.Now().Add(1*time.Hour))
		require.NoError(t, err)
	})
}

// ---------------------------------------------------------------------------
// MediaStore
// ---------------------------------------------------------------------------

func TestIntegration_MediaStore(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("CreateMediaAttachment_GetMediaAttachment", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		mediaID := uid.New()
		url := "https://cdn.example/media/" + mediaID + ".jpg"

		m, err := s.CreateMediaAttachment(ctx, store.CreateMediaAttachmentInput{
			ID:          mediaID,
			AccountID:   acc.ID,
			Type:        domain.MediaTypeImage,
			StorageKey:  "key_" + mediaID,
			URL:         url,
			Description: testutil.StrPtr("A photo"),
		})
		require.NoError(t, err)
		assert.Equal(t, mediaID, m.ID)
		assert.Equal(t, url, m.URL)

		got, err := s.GetMediaAttachment(ctx, mediaID)
		require.NoError(t, err)
		assert.Equal(t, url, got.URL)
		require.NotNil(t, got.Description)
		assert.Equal(t, "A photo", *got.Description)
	})

	t.Run("GetMediaAttachment_not_found", func(t *testing.T) {
		_, err := s.GetMediaAttachment(ctx, "nonexistent_"+uid.New())
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("UpdateMediaAttachment", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		mediaID := uid.New()
		_, err := s.CreateMediaAttachment(ctx, store.CreateMediaAttachmentInput{
			ID:         mediaID,
			AccountID:  acc.ID,
			Type:       domain.MediaTypeImage,
			StorageKey: "upd_" + mediaID,
			URL:        "https://cdn.example/media/" + mediaID + ".jpg",
		})
		require.NoError(t, err)

		newDesc := "Updated description"
		meta := mustJSON(t, map[string]int{"width": 800, "height": 600})
		updated, err := s.UpdateMediaAttachment(ctx, store.UpdateMediaAttachmentInput{
			ID:          mediaID,
			AccountID:   acc.ID,
			Description: &newDesc,
			Meta:        json.RawMessage(meta),
		})
		require.NoError(t, err)
		require.NotNil(t, updated.Description)
		assert.Equal(t, "Updated description", *updated.Description)
	})
}

// ---------------------------------------------------------------------------
// InstanceStore
// ---------------------------------------------------------------------------

func TestIntegration_InstanceStore(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("GetMonsteraSettings_UpdateMonsteraSettings", func(t *testing.T) {
		settings, err := s.GetMonsteraSettings(ctx)
		require.NoError(t, err)
		require.NotNil(t, settings)

		err = s.UpdateMonsteraSettings(ctx, &domain.MonsteraSettings{
			RegistrationMode:   domain.MonsteraRegistrationModeOpen,
			TrendingLinksScope: domain.MonsteraTrendingLinksScopeDisabled,
		})
		require.NoError(t, err)

		got, err := s.GetMonsteraSettings(ctx)
		require.NoError(t, err)
		assert.Equal(t, domain.MonsteraRegistrationModeOpen, got.RegistrationMode)
	})
}

// ---------------------------------------------------------------------------
// TrendingStore
// ---------------------------------------------------------------------------

func TestIntegration_TrendingStore(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("GetTrendingStatusIDs_empty", func(t *testing.T) {
		statuses, err := s.GetTrendingStatusIDs(ctx, 10)
		require.NoError(t, err)
		_ = statuses
	})

	t.Run("GetTrendingTags_empty", func(t *testing.T) {
		tags, err := s.GetTrendingTags(ctx, 7, 10)
		require.NoError(t, err)
		_ = tags
	})

	t.Run("ReplaceTrendingStatuses", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		st := createTestStatus(t, s, ctx, acc.ID)

		err := s.ReplaceTrendingStatuses(ctx, []domain.TrendingStatus{
			{StatusID: st.ID, Score: 42.0, RankedAt: time.Now()},
		})
		require.NoError(t, err)

		trending, err := s.GetTrendingStatusIDs(ctx, 10)
		require.NoError(t, err)
		found := false
		for _, ts := range trending {
			if ts.StatusID == st.ID {
				found = true
			}
		}
		assert.True(t, found, "trending status not returned")
	})

	t.Run("GetTopScoredPublicStatuses", func(t *testing.T) {
		statuses, err := s.GetTopScoredPublicStatuses(ctx, time.Now().Add(-24*time.Hour), 10)
		require.NoError(t, err)
		_ = statuses
	})

	t.Run("UpsertTrendingTagHistory", func(t *testing.T) {
		h, err := s.GetOrCreateHashtag(ctx, "trending_"+uid.New()[:8])
		require.NoError(t, err)

		err = s.UpsertTrendingTagHistory(ctx, []domain.TrendingTagHistory{
			{HashtagID: h.ID, Day: time.Now().Truncate(24 * time.Hour), Uses: 10, Accounts: 5},
		})
		require.NoError(t, err)
	})
}
