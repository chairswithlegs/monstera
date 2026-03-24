package service

import (
	"context"
	"testing"
	"time"

	"encoding/json"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/chairswithlegs/monstera/internal/uid"
)

func TestRemoteStatusWriteService_CreateRemote_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := NewAccountService(st, "https://example.com")
	acc, err := accountSvc.Register(ctx, RegisterInput{
		Username: "remote",
		Email:    "r@example.com",
		Password: "p",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)
	require.NotNil(t, acc)

	statusSvc := NewStatusService(st, "https://example.com", "example.com", 5000)
	convSvc := NewConversationService(st, statusSvc)
	mediaSvc := NewMediaService(st, nil, 0)
	svc := NewRemoteStatusWriteService(st, convSvc, mediaSvc, "https://example.com")

	in := CreateRemoteStatusInput{
		AccountID:  acc.ID,
		URI:        "https://remote.example/statuses/1",
		Content:    testutil.StrPtr("hello"),
		Visibility: domain.VisibilityPublic,
		APID:       "https://remote.example/statuses/1",
	}
	st2, err := svc.CreateRemote(ctx, in)
	require.NoError(t, err)
	require.NotNil(t, st2)
	assert.Equal(t, acc.ID, st2.AccountID)
	assert.Equal(t, "https://remote.example/statuses/1", st2.URI)
	assert.Equal(t, "https://remote.example/statuses/1", st2.APID)
	assert.False(t, st2.Local)
}

func TestRemoteStatusWriteService_CreateRemote_PreservesPublishedAt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := NewAccountService(st, "https://example.com")
	acc, err := accountSvc.Register(ctx, RegisterInput{
		Username: "remote2",
		Email:    "r2@example.com",
		Password: "p",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	statusSvc := NewStatusService(st, "https://example.com", "example.com", 5000)
	convSvc := NewConversationService(st, statusSvc)
	mediaSvc := NewMediaService(st, nil, 0)
	svc := NewRemoteStatusWriteService(st, convSvc, mediaSvc, "https://example.com")

	published := time.Date(2023, 6, 15, 12, 0, 0, 0, time.UTC)
	in := CreateRemoteStatusInput{
		AccountID:   acc.ID,
		URI:         "https://remote.example/statuses/old",
		Content:     testutil.StrPtr("old post"),
		Visibility:  domain.VisibilityPublic,
		APID:        "https://remote.example/statuses/old",
		PublishedAt: &published,
	}
	created, err := svc.CreateRemote(ctx, in)
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, published, created.CreatedAt, "CreatedAt should match the AP Note Published timestamp")
}

func TestRemoteStatusWriteService_DeleteRemote_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	statusSvc := NewStatusService(st, "https://example.com", "example.com", 5000)
	convSvc := NewConversationService(st, statusSvc)
	mediaSvc := NewMediaService(st, nil, 0)
	svc := NewRemoteStatusWriteService(st, convSvc, mediaSvc, "https://example.com")

	err := svc.DeleteRemote(ctx, "01nonexistent-status-id")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestRemoteStatusWriteService_DeleteRemote_ForbiddenLocal(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := NewAccountService(st, "https://example.com")
	acc, err := accountSvc.Register(ctx, RegisterInput{
		Username: "local",
		Email:    "l@example.com",
		Password: "p",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)
	require.NotNil(t, acc)

	localStatusID := uid.New()
	_, err = st.CreateStatus(ctx, store.CreateStatusInput{
		ID:                  localStatusID,
		URI:                 "https://example.com/statuses/" + localStatusID,
		AccountID:           acc.ID,
		Content:             testutil.StrPtr("hello"),
		Visibility:          domain.VisibilityPublic,
		APID:                "https://example.com/statuses/" + localStatusID,
		Local:               true,
		QuoteApprovalPolicy: domain.QuotePolicyPublic,
	})
	require.NoError(t, err)

	statusSvc := NewStatusService(st, "https://example.com", "example.com", 5000)
	convSvc := NewConversationService(st, statusSvc)
	mediaSvc := NewMediaService(st, nil, 0)
	svc := NewRemoteStatusWriteService(st, convSvc, mediaSvc, "https://example.com")

	err = svc.DeleteRemote(ctx, localStatusID)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestRemoteStatusWriteService_CreateRemote_DuplicateAPID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := NewAccountService(st, "https://example.com")
	acc, err := accountSvc.Register(ctx, RegisterInput{
		Username: "remote3",
		Email:    "r3@example.com",
		Password: "p",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	statusSvc := NewStatusService(st, "https://example.com", "example.com", 5000)
	convSvc := NewConversationService(st, statusSvc)
	mediaSvc := NewMediaService(st, nil, 0)
	svc := NewRemoteStatusWriteService(st, convSvc, mediaSvc, "https://example.com")

	in := CreateRemoteStatusInput{
		AccountID:  acc.ID,
		URI:        "https://remote.example/statuses/dup",
		Content:    testutil.StrPtr("hello"),
		Visibility: domain.VisibilityPublic,
		APID:       "https://remote.example/statuses/dup",
	}
	_, err = svc.CreateRemote(ctx, in)
	require.NoError(t, err)

	// Second call with the same ap_id should return ErrConflict.
	_, err = svc.CreateRemote(ctx, in)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrConflict)
}

func TestRemoteStatusWriteService_CreateRemote_NoMediaOnConflict(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := NewAccountService(st, "https://example.com")
	acc, err := accountSvc.Register(ctx, RegisterInput{
		Username: "remote4",
		Email:    "r4@example.com",
		Password: "p",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	statusSvc := NewStatusService(st, "https://example.com", "example.com", 5000)
	convSvc := NewConversationService(st, statusSvc)
	mediaSvc := NewMediaService(st, nil, 0)
	svc := NewRemoteStatusWriteService(st, convSvc, mediaSvc, "https://example.com")

	attachment := CreateRemoteMediaInput{
		AccountID: acc.ID,
		RemoteURL: "https://remote.example/image.jpg",
		MediaType: "image/jpeg",
	}
	in := CreateRemoteStatusInput{
		AccountID:   acc.ID,
		URI:         "https://remote.example/statuses/media",
		Content:     testutil.StrPtr("post with media"),
		Visibility:  domain.VisibilityPublic,
		APID:        "https://remote.example/statuses/media",
		Attachments: []CreateRemoteMediaInput{attachment},
	}

	// First call: status and media are created.
	created, err := svc.CreateRemote(ctx, in)
	require.NoError(t, err)
	atts, err := st.GetStatusAttachments(ctx, created.ID)
	require.NoError(t, err)
	assert.Len(t, atts, 1, "media should be attached after successful create")

	// Second call: ErrConflict — no additional media records should be created.
	_, err = svc.CreateRemote(ctx, in)
	require.ErrorIs(t, err, domain.ErrConflict)
	atts2, _ := st.GetStatusAttachments(ctx, created.ID)
	assert.Len(t, atts2, 1, "no extra media should be created when CreateStatus conflicts")
}

func TestRemoteStatusWriteService_CreateRemoteFavourite_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := NewAccountService(st, "https://example.com")
	acc, err := accountSvc.Register(ctx, RegisterInput{
		Username: "remote",
		Email:    "r@example.com",
		Password: "p",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)
	require.NotNil(t, acc)

	statusSvc := NewStatusService(st, "https://example.com", "example.com", 5000)
	convSvc := NewConversationService(st, statusSvc)
	mediaSvc := NewMediaService(st, nil, 0)
	svc := NewRemoteStatusWriteService(st, convSvc, mediaSvc, "https://example.com")

	in := CreateRemoteStatusInput{
		AccountID:  acc.ID,
		URI:        "https://remote.example/statuses/1",
		Content:    testutil.StrPtr("hello"),
		Visibility: domain.VisibilityPublic,
		APID:       "https://remote.example/statuses/1",
	}
	st2, err := svc.CreateRemote(ctx, in)
	require.NoError(t, err)
	require.NotNil(t, st2)

	fav, err := svc.CreateRemoteFavourite(ctx, acc.ID, st2.ID, nil)
	require.NoError(t, err)
	require.NotNil(t, fav)
	assert.Equal(t, acc.ID, fav.AccountID)
	assert.Equal(t, st2.ID, fav.StatusID)
}

func TestRemoteStatusWriteService_CreateRemote_EventPayloadIncludesMediaMentionsTags(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fs := testutil.NewFakeStore()
	accountSvc := NewAccountService(fs, "https://example.com")
	remoteAuthor, err := accountSvc.Register(ctx, RegisterInput{
		Username: "remoteauthor",
		Email:    "ra@example.com",
		Password: "p",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)
	mentioned, err := accountSvc.Register(ctx, RegisterInput{
		Username: "mentioned",
		Email:    "m@example.com",
		Password: "p",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	statusSvc := NewStatusService(fs, "https://example.com", "example.com", 5000)
	convSvc := NewConversationService(fs, statusSvc)
	mediaSvc := NewMediaService(fs, nil, 0)
	svc := NewRemoteStatusWriteService(fs, convSvc, mediaSvc, "https://example.com")

	in := CreateRemoteStatusInput{
		AccountID:  remoteAuthor.ID,
		URI:        "https://remote.example/statuses/full",
		Content:    testutil.StrPtr("<p>hello @mentioned</p>"),
		Visibility: domain.VisibilityPublic,
		APID:       "https://remote.example/statuses/full",
		Attachments: []CreateRemoteMediaInput{
			{AccountID: remoteAuthor.ID, RemoteURL: "https://remote.example/image.jpg", MediaType: "image/jpeg"},
		},
		HashtagNames: []string{"golang", "fediverse"},
		MentionIRIs:  []string{mentioned.APID},
	}
	created, err := svc.CreateRemote(ctx, in)
	require.NoError(t, err)
	require.NotNil(t, created)

	// Find the status.created.remote event.
	var foundEvent *domain.DomainEvent
	for i := range fs.OutboxEvents {
		if fs.OutboxEvents[i].EventType == domain.EventStatusCreatedRemote {
			foundEvent = &fs.OutboxEvents[i]
			break
		}
	}
	require.NotNil(t, foundEvent, "expected a status.created.remote event")

	var payload domain.StatusCreatedPayload
	require.NoError(t, json.Unmarshal(foundEvent.Payload, &payload))

	assert.NotNil(t, payload.Author, "event payload should include author")
	assert.Len(t, payload.Media, 1, "event payload should include media")
	assert.Len(t, payload.Tags, 2, "event payload should include tags")
	assert.Len(t, payload.Mentions, 1, "event payload should include mentions")
	assert.Equal(t, mentioned.ID, payload.Mentions[0].ID)
	assert.Contains(t, payload.MentionedAccountIDs, mentioned.ID)
	assert.False(t, payload.Local)
}

func TestRemoteStatusWriteService_UpdateRemote_EventPayloadIncludesMediaMentionsTags(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fs := testutil.NewFakeStore()
	accountSvc := NewAccountService(fs, "https://example.com")
	remoteAuthor, err := accountSvc.Register(ctx, RegisterInput{
		Username: "updateauthor",
		Email:    "ua@example.com",
		Password: "p",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)
	mentioned, err := accountSvc.Register(ctx, RegisterInput{
		Username: "updatementioned",
		Email:    "um@example.com",
		Password: "p",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	statusSvc := NewStatusService(fs, "https://example.com", "example.com", 5000)
	convSvc := NewConversationService(fs, statusSvc)
	mediaSvc := NewMediaService(fs, nil, 0)
	svc := NewRemoteStatusWriteService(fs, convSvc, mediaSvc, "https://example.com")

	// Create the initial remote status with media, tags, mentions.
	in := CreateRemoteStatusInput{
		AccountID:  remoteAuthor.ID,
		URI:        "https://remote.example/statuses/upd",
		Content:    testutil.StrPtr("<p>original</p>"),
		Visibility: domain.VisibilityPublic,
		APID:       "https://remote.example/statuses/upd",
		Attachments: []CreateRemoteMediaInput{
			{AccountID: remoteAuthor.ID, RemoteURL: "https://remote.example/pic.jpg", MediaType: "image/jpeg"},
		},
		HashtagNames: []string{"testing"},
		MentionIRIs:  []string{mentioned.APID},
	}
	created, err := svc.CreateRemote(ctx, in)
	require.NoError(t, err)

	// Clear events from create.
	fs.OutboxEvents = nil

	// Update the remote status.
	updatedContent := "<p>edited</p>"
	err = svc.UpdateRemote(ctx, created.ID, created, UpdateRemoteStatusInput{
		Content: &updatedContent,
	})
	require.NoError(t, err)

	require.Len(t, fs.OutboxEvents, 1)
	ev := fs.OutboxEvents[0]
	assert.Equal(t, domain.EventStatusUpdatedRemote, ev.EventType)

	var payload domain.StatusUpdatedPayload
	require.NoError(t, json.Unmarshal(ev.Payload, &payload))

	assert.NotNil(t, payload.Author, "event payload should include author")
	assert.Len(t, payload.Media, 1, "event payload should include existing media")
	assert.Len(t, payload.Tags, 1, "event payload should include existing tags")
	assert.Len(t, payload.Mentions, 1, "event payload should include existing mentions")
	assert.Equal(t, mentioned.ID, payload.Mentions[0].ID)
	assert.Contains(t, payload.MentionedAccountIDs, mentioned.ID, "event payload should include mentioned account IDs")
	assert.False(t, payload.Local)
}

func TestRemoteStatusWriteService_DeleteRemoteReblog_EmitsEvent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fs := testutil.NewFakeStore()
	accountSvc := NewAccountService(fs, "https://example.com")
	reblogger, err := accountSvc.Register(ctx, RegisterInput{
		Username: "reblogger",
		Email:    "rb@example.com",
		Password: "p",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)
	author, err := accountSvc.Register(ctx, RegisterInput{
		Username: "author",
		Email:    "a@example.com",
		Password: "p",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	statusSvc := NewStatusService(fs, "https://example.com", "example.com", 5000)
	convSvc := NewConversationService(fs, statusSvc)
	mediaSvc := NewMediaService(fs, nil, 0)
	svc := NewRemoteStatusWriteService(fs, convSvc, mediaSvc, "https://example.com")

	// Create the original status.
	originalID := uid.New()
	_, err = fs.CreateStatus(ctx, store.CreateStatusInput{
		ID:                  originalID,
		URI:                 "https://example.com/statuses/" + originalID,
		AccountID:           author.ID,
		Content:             testutil.StrPtr("original"),
		Visibility:          domain.VisibilityPublic,
		APID:                "https://example.com/statuses/" + originalID,
		Local:               true,
		QuoteApprovalPolicy: domain.QuotePolicyPublic,
	})
	require.NoError(t, err)

	// Create a remote reblog of that status.
	_, err = svc.CreateRemoteReblog(ctx, CreateRemoteReblogInput{
		AccountID:        reblogger.ID,
		ObjectStatusAPID: "https://example.com/statuses/" + originalID,
		ActivityAPID:     "https://remote.example/activities/announce-1",
	})
	require.NoError(t, err)

	// Clear events from create so we only see the delete event.
	fs.OutboxEvents = nil

	err = svc.DeleteRemoteReblog(ctx, reblogger.ID, originalID)
	require.NoError(t, err)

	require.Len(t, fs.OutboxEvents, 1)
	ev := fs.OutboxEvents[0]
	assert.Equal(t, domain.EventReblogRemoved, ev.EventType)
	assert.Equal(t, "status", ev.AggregateType)

	var payload domain.ReblogRemovedPayload
	require.NoError(t, json.Unmarshal(ev.Payload, &payload))
	assert.Equal(t, reblogger.ID, payload.AccountID)
	assert.Equal(t, originalID, payload.OriginalStatusID)
	assert.Equal(t, author.ID, payload.OriginalAuthorID)
	assert.NotEmpty(t, payload.ReblogStatusID)
}

func TestRemoteStatusWriteService_DeleteRemoteFavourite_EmitsEvent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fs := testutil.NewFakeStore()
	accountSvc := NewAccountService(fs, "https://example.com")
	favouriter, err := accountSvc.Register(ctx, RegisterInput{
		Username: "favouriter",
		Email:    "f@example.com",
		Password: "p",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)
	author, err := accountSvc.Register(ctx, RegisterInput{
		Username: "author2",
		Email:    "a2@example.com",
		Password: "p",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	statusSvc := NewStatusService(fs, "https://example.com", "example.com", 5000)
	convSvc := NewConversationService(fs, statusSvc)
	mediaSvc := NewMediaService(fs, nil, 0)
	svc := NewRemoteStatusWriteService(fs, convSvc, mediaSvc, "https://example.com")

	// Create a status.
	statusID := uid.New()
	_, err = fs.CreateStatus(ctx, store.CreateStatusInput{
		ID:                  statusID,
		URI:                 "https://example.com/statuses/" + statusID,
		AccountID:           author.ID,
		Content:             testutil.StrPtr("original"),
		Visibility:          domain.VisibilityPublic,
		APID:                "https://example.com/statuses/" + statusID,
		Local:               true,
		QuoteApprovalPolicy: domain.QuotePolicyPublic,
	})
	require.NoError(t, err)

	// Create a remote favourite.
	_, err = svc.CreateRemoteFavourite(ctx, favouriter.ID, statusID, nil)
	require.NoError(t, err)

	// Clear events from create.
	fs.OutboxEvents = nil

	err = svc.DeleteRemoteFavourite(ctx, favouriter.ID, statusID)
	require.NoError(t, err)

	require.Len(t, fs.OutboxEvents, 1)
	ev := fs.OutboxEvents[0]
	assert.Equal(t, domain.EventFavouriteRemoved, ev.EventType)
	assert.Equal(t, "favourite", ev.AggregateType)
	assert.Equal(t, statusID, ev.AggregateID)

	var payload domain.FavouriteRemovedPayload
	require.NoError(t, json.Unmarshal(ev.Payload, &payload))
	assert.Equal(t, favouriter.ID, payload.AccountID)
	assert.Equal(t, statusID, payload.StatusID)
	assert.Equal(t, author.ID, payload.StatusAuthorID)
}
