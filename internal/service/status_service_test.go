package service

import (
	"context"
	"testing"
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/chairswithlegs/monstera/internal/uid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const privatePostText = "private post"

func TestStatusService_GetByID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	created, err := statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  acc.ID,
		Text:       "Hello",
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)

	got, err := statusSvc.GetByID(ctx, created.Status.ID)
	require.NoError(t, err)
	assert.Equal(t, created.Status.ID, got.ID)
	assert.Equal(t, "Hello", *got.Text)
}

func TestStatusService_GetByID_not_found(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)

	_, err := statusSvc.GetByID(ctx, "01nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestStatusService_GetByAPID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	result, err := statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  acc.ID,
		Username:   acc.Username,
		Text:       "Hello",
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)

	t.Run("found by APID", func(t *testing.T) {
		got, err := statusSvc.GetByAPID(ctx, result.Status.APID)
		require.NoError(t, err)
		assert.Equal(t, result.Status.ID, got.ID)
	})

	t.Run("not found returns ErrNotFound", func(t *testing.T) {
		_, err := statusSvc.GetByAPID(ctx, "https://example.com/nonexistent")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestStatusService_IsBookmarked(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	result, err := statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  acc.ID,
		Text:       "bookmarkable",
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)
	statusID := result.Status.ID

	t.Run("not bookmarked returns false", func(t *testing.T) {
		ok, err := statusSvc.IsBookmarked(ctx, acc.ID, statusID)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("after bookmark returns true", func(t *testing.T) {
		err := fake.CreateBookmark(ctx, store.CreateBookmarkInput{
			ID:        uid.New(),
			AccountID: acc.ID,
			StatusID:  statusID,
		})
		require.NoError(t, err)
		ok, err := statusSvc.IsBookmarked(ctx, acc.ID, statusID)
		require.NoError(t, err)
		assert.True(t, ok)
	})
}

func TestStatusService_GetPoll(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)

	acc, err := accountSvc.Register(ctx, RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	result, err := statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:         acc.ID,
		Username:          acc.Username,
		Text:              "Poll?",
		Visibility:        domain.VisibilityPublic,
		DefaultVisibility: domain.VisibilityPublic,
		Poll: &PollInput{
			Options:          []string{"Yes", "No"},
			ExpiresInSeconds: 3600,
			Multiple:         false,
		},
		PollLimits: &PollLimits{
			MaxOptions:    4,
			MinExpiration: 300,
			MaxExpiration: 2629746,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, result.Poll)
	pollID := result.Poll.Poll.ID

	t.Run("returns enriched poll with options", func(t *testing.T) {
		poll, err := statusSvc.GetPoll(ctx, pollID, nil)
		require.NoError(t, err)
		require.NotNil(t, poll)
		assert.Equal(t, pollID, poll.Poll.ID)
		assert.Len(t, poll.Options, 2)
		assert.False(t, poll.Voted)
	})

	t.Run("with viewer sets voted field", func(t *testing.T) {
		poll, err := statusSvc.GetPoll(ctx, pollID, &acc.ID)
		require.NoError(t, err)
		assert.False(t, poll.Voted)
	})

	t.Run("not found returns ErrNotFound", func(t *testing.T) {
		_, err := statusSvc.GetPoll(ctx, "01H0000000000000000000000", nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestStatusService_GetScheduledStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)

	alice, err := accountSvc.Register(ctx, RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	bob, err := accountSvc.Register(ctx, RegisterInput{
		Username:     "bob",
		Email:        "bob@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	schedID := uid.New()
	_, err = fake.CreateScheduledStatus(ctx, store.CreateScheduledStatusInput{
		ID:          schedID,
		AccountID:   alice.ID,
		Params:      []byte(`{"text":"scheduled"}`),
		ScheduledAt: time.Now().Add(1 * time.Hour),
	})
	require.NoError(t, err)

	t.Run("owner gets scheduled status", func(t *testing.T) {
		s, err := statusSvc.GetScheduledStatus(ctx, schedID, alice.ID)
		require.NoError(t, err)
		assert.Equal(t, schedID, s.ID)
		assert.Equal(t, alice.ID, s.AccountID)
	})

	t.Run("other account returns ErrNotFound", func(t *testing.T) {
		_, err := statusSvc.GetScheduledStatus(ctx, schedID, bob.ID)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("nonexistent returns ErrNotFound", func(t *testing.T) {
		_, err := statusSvc.GetScheduledStatus(ctx, "01H0000000000000000000000", alice.ID)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestStatusService_GetByIDEnriched_private_returns_ErrNotFound_when_unauthenticated(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	st, err := statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  acc.ID,
		Text:       privatePostText,
		Visibility: domain.VisibilityPrivate,
	})
	require.NoError(t, err)

	_, err = statusSvc.GetByIDEnriched(ctx, st.Status.ID, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestStatusService_GetByIDEnriched_private_returns_success_when_viewer_is_author(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	st, err := statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  acc.ID,
		Text:       privatePostText,
		Visibility: domain.VisibilityPrivate,
	})
	require.NoError(t, err)
	viewerID := acc.ID

	result, err := statusSvc.GetByIDEnriched(ctx, st.Status.ID, &viewerID)
	require.NoError(t, err)
	require.NotNil(t, result.Status)
	assert.Equal(t, domain.VisibilityPrivate, result.Status.Visibility)
	assert.Equal(t, privatePostText, *result.Status.Text)
}

func TestStatusService_GetByIDEnriched_returns_ErrNotFound_when_viewer_blocked_by_author(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)

	author, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	viewer, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)
	st, err := statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  author.ID,
		Text:       "public post",
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)
	err = fake.CreateBlock(ctx, store.CreateBlockInput{ID: "01block", AccountID: author.ID, TargetID: viewer.ID})
	require.NoError(t, err)

	_, err = statusSvc.GetByIDEnriched(ctx, st.Status.ID, &viewer.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestStatusService_ListQuotesOfStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	conversationSvc := NewConversationService(fake, statusSvc)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, conversationSvc, "https://example.com", "example.com", 500)

	acc, err := accountSvc.Register(ctx, RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	quotedID := uid.New()
	_, err = fake.CreateStatus(ctx, store.CreateStatusInput{
		ID:         quotedID,
		URI:        "https://example.com/statuses/" + quotedID,
		AccountID:  acc.ID,
		Text:       testutil.StrPtr("original"),
		Content:    testutil.StrPtr("<p>original</p>"),
		Visibility: domain.VisibilityPublic,
		APID:       "https://example.com/statuses/" + quotedID,
		Local:      true,
	})
	require.NoError(t, err)

	t.Run("no quotes returns empty list", func(t *testing.T) {
		list, err := statusSvc.ListQuotesOfStatus(ctx, quotedID, nil, 20, &acc.ID)
		require.NoError(t, err)
		assert.Empty(t, list)
	})

	t.Run("after creating quote returns one status", func(t *testing.T) {
		enriched, err := statusWriteSvc.Create(ctx, CreateStatusInput{
			AccountID:      acc.ID,
			Username:       "alice",
			Text:           "a quote",
			Visibility:     domain.VisibilityPublic,
			QuotedStatusID: &quotedID,
		})
		require.NoError(t, err)
		list, err := statusSvc.ListQuotesOfStatus(ctx, quotedID, nil, 20, &acc.ID)
		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.Equal(t, enriched.Status.ID, list[0].Status.ID)
	})

	t.Run("nonexistent status returns ErrNotFound", func(t *testing.T) {
		_, err := statusSvc.ListQuotesOfStatus(ctx, "01H0000000000000000000000", nil, 20, &acc.ID)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}
