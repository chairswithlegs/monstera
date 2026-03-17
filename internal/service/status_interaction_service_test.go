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

func TestStatusInteractionService_CreateReblog(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	interactionSvc := NewStatusInteractionService(fake, statusSvc, "https://example.com")
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)

	alice, err := accountSvc.Register(ctx, RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)
	bob, err := accountSvc.Register(ctx, RegisterInput{
		Username: "bob",
		Email:    "bob@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	result, err := statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  alice.ID,
		Username:   alice.Username,
		Text:       "rebloggable",
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)
	statusID := result.Status.ID

	t.Run("success creates reblog and increments count", func(t *testing.T) {
		enriched, err := interactionSvc.CreateReblog(ctx, bob.ID, bob.Username, statusID)
		require.NoError(t, err)
		require.NotNil(t, enriched.Status)
		assert.Equal(t, &statusID, enriched.Status.ReblogOfID)
		st, err := fake.GetStatusByID(ctx, statusID)
		require.NoError(t, err)
		assert.Equal(t, 1, st.ReblogsCount)
	})

	t.Run("duplicate reblog returns ErrConflict", func(t *testing.T) {
		_, err := interactionSvc.CreateReblog(ctx, bob.ID, bob.Username, statusID)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrConflict)
	})

	t.Run("nonexistent status returns ErrNotFound", func(t *testing.T) {
		_, err := interactionSvc.CreateReblog(ctx, bob.ID, bob.Username, "01H0000000000000000000000")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestStatusInteractionService_DeleteReblog(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	interactionSvc := NewStatusInteractionService(fake, statusSvc, "https://example.com")
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)

	alice, err := accountSvc.Register(ctx, RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)
	bob, err := accountSvc.Register(ctx, RegisterInput{
		Username: "bob",
		Email:    "bob@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	result, err := statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  alice.ID,
		Username:   alice.Username,
		Text:       "rebloggable",
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)
	statusID := result.Status.ID

	_, err = interactionSvc.CreateReblog(ctx, bob.ID, bob.Username, statusID)
	require.NoError(t, err)

	t.Run("delete removes reblog and decrements count", func(t *testing.T) {
		err := interactionSvc.DeleteReblog(ctx, bob.ID, statusID)
		require.NoError(t, err)
		st, err := fake.GetStatusByID(ctx, statusID)
		require.NoError(t, err)
		assert.Equal(t, 0, st.ReblogsCount)
	})

	t.Run("idempotent: delete nonexistent reblog succeeds", func(t *testing.T) {
		err := interactionSvc.DeleteReblog(ctx, bob.ID, statusID)
		require.NoError(t, err)
	})
}

func TestStatusInteractionService_CreateFavourite(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	interactionSvc := NewStatusInteractionService(fake, statusSvc, "https://example.com")
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)

	alice, err := accountSvc.Register(ctx, RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)
	bob, err := accountSvc.Register(ctx, RegisterInput{
		Username: "bob",
		Email:    "bob@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	result, err := statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  alice.ID,
		Username:   alice.Username,
		Text:       "favouritable",
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)
	statusID := result.Status.ID

	t.Run("success returns status and increments count", func(t *testing.T) {
		enriched, err := interactionSvc.CreateFavourite(ctx, bob.ID, statusID)
		require.NoError(t, err)
		assert.Equal(t, statusID, enriched.Status.ID)
		st, err := fake.GetStatusByID(ctx, statusID)
		require.NoError(t, err)
		assert.Equal(t, 1, st.FavouritesCount)
	})

	t.Run("not found returns ErrNotFound", func(t *testing.T) {
		_, err := interactionSvc.CreateFavourite(ctx, bob.ID, "01H0000000000000000000000")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestStatusInteractionService_DeleteFavourite(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	interactionSvc := NewStatusInteractionService(fake, statusSvc, "https://example.com")
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)

	acc, err := accountSvc.Register(ctx, RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	result, err := statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  acc.ID,
		Username:   acc.Username,
		Text:       "unfavouritable",
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)
	statusID := result.Status.ID

	_, err = interactionSvc.CreateFavourite(ctx, acc.ID, statusID)
	require.NoError(t, err)

	t.Run("delete returns status", func(t *testing.T) {
		enriched, err := interactionSvc.DeleteFavourite(ctx, acc.ID, statusID)
		require.NoError(t, err)
		assert.Equal(t, statusID, enriched.Status.ID)
	})

	t.Run("idempotent: delete again succeeds", func(t *testing.T) {
		_, err := interactionSvc.DeleteFavourite(ctx, acc.ID, statusID)
		require.NoError(t, err)
	})
}

func TestStatusInteractionService_Bookmark(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	interactionSvc := NewStatusInteractionService(fake, statusSvc, "https://example.com")
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	result, err := statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  acc.ID,
		Username:   acc.Username,
		Text:       "bookmarkable",
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)
	statusID := result.Status.ID

	t.Run("bookmark returns status with bookmarked true", func(t *testing.T) {
		enriched, err := interactionSvc.Bookmark(ctx, acc.ID, statusID)
		require.NoError(t, err)
		assert.Equal(t, statusID, enriched.Status.ID)
		assert.True(t, enriched.Bookmarked)
	})

	t.Run("idempotent: double bookmark succeeds", func(t *testing.T) {
		enriched, err := interactionSvc.Bookmark(ctx, acc.ID, statusID)
		require.NoError(t, err)
		assert.True(t, enriched.Bookmarked)
	})

	t.Run("nonexistent status returns ErrNotFound", func(t *testing.T) {
		_, err := interactionSvc.Bookmark(ctx, acc.ID, "01H0000000000000000000000")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestStatusInteractionService_Unbookmark(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	interactionSvc := NewStatusInteractionService(fake, statusSvc, "https://example.com")
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	result, err := statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  acc.ID,
		Username:   acc.Username,
		Text:       "unbookmarkable",
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)
	statusID := result.Status.ID

	_, err = interactionSvc.Bookmark(ctx, acc.ID, statusID)
	require.NoError(t, err)

	t.Run("unbookmark returns status with bookmarked false", func(t *testing.T) {
		enriched, err := interactionSvc.Unbookmark(ctx, acc.ID, statusID)
		require.NoError(t, err)
		assert.Equal(t, statusID, enriched.Status.ID)
		assert.False(t, enriched.Bookmarked)
	})

	t.Run("idempotent: unbookmark again succeeds", func(t *testing.T) {
		_, err := interactionSvc.Unbookmark(ctx, acc.ID, statusID)
		require.NoError(t, err)
	})
}

func TestStatusInteractionService_Pin(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	interactionSvc := NewStatusInteractionService(fake, statusSvc, "https://example.com")
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)

	alice, err := accountSvc.Register(ctx, RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)
	bob, err := accountSvc.Register(ctx, RegisterInput{
		Username: "bob",
		Email:    "bob@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	result, err := statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  alice.ID,
		Username:   alice.Username,
		Text:       "pinnable",
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)
	statusID := result.Status.ID

	t.Run("owner pins own public status", func(t *testing.T) {
		enriched, err := interactionSvc.Pin(ctx, alice.ID, statusID)
		require.NoError(t, err)
		assert.Equal(t, statusID, enriched.Status.ID)
		assert.True(t, enriched.Pinned)
	})

	t.Run("non-owner returns ErrForbidden", func(t *testing.T) {
		_, err := interactionSvc.Pin(ctx, bob.ID, statusID)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("nonexistent status returns ErrNotFound", func(t *testing.T) {
		_, err := interactionSvc.Pin(ctx, alice.ID, "01H0000000000000000000000")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestStatusInteractionService_Unpin(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	interactionSvc := NewStatusInteractionService(fake, statusSvc, "https://example.com")
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)

	alice, err := accountSvc.Register(ctx, RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)
	bob, err := accountSvc.Register(ctx, RegisterInput{
		Username: "bob",
		Email:    "bob@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	result, err := statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  alice.ID,
		Username:   alice.Username,
		Text:       "pinnable",
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)
	statusID := result.Status.ID

	_, err = interactionSvc.Pin(ctx, alice.ID, statusID)
	require.NoError(t, err)

	t.Run("owner unpins", func(t *testing.T) {
		enriched, err := interactionSvc.Unpin(ctx, alice.ID, statusID)
		require.NoError(t, err)
		assert.Equal(t, statusID, enriched.Status.ID)
		assert.False(t, enriched.Pinned)
	})

	t.Run("non-owner returns ErrForbidden", func(t *testing.T) {
		_, err := interactionSvc.Unpin(ctx, bob.ID, statusID)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func TestStatusInteractionService_RecordVote(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	interactionSvc := NewStatusInteractionService(fake, statusSvc, "https://example.com")
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)

	acc, err := accountSvc.Register(ctx, RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	result, err := statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:         acc.ID,
		Username:          acc.Username,
		Text:              "Vote?",
		Visibility:        domain.VisibilityPublic,
		DefaultVisibility: domain.VisibilityPublic,
		Poll: &PollInput{
			Options:          []string{"A", "B", "C"},
			ExpiresInSeconds: 3600,
			Multiple:         true,
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

	t.Run("records vote returns enriched poll with voted=true", func(t *testing.T) {
		poll, err := interactionSvc.RecordVote(ctx, pollID, acc.ID, []int{0, 2})
		require.NoError(t, err)
		assert.True(t, poll.Voted)
		assert.Equal(t, []int{0, 2}, poll.OwnVotes)
	})

	t.Run("empty choices returns ErrValidation", func(t *testing.T) {
		_, err := interactionSvc.RecordVote(ctx, pollID, acc.ID, []int{})
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrValidation)
	})

	t.Run("out-of-range index returns ErrValidation", func(t *testing.T) {
		_, err := interactionSvc.RecordVote(ctx, pollID, acc.ID, []int{99})
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrValidation)
	})

	t.Run("nonexistent poll returns ErrNotFound", func(t *testing.T) {
		_, err := interactionSvc.RecordVote(ctx, "01H0000000000000000000000", acc.ID, []int{0})
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("expired poll returns ErrUnprocessable", func(t *testing.T) {
		expiredAt := time.Now().Add(-time.Hour)
		expiredStatusID := uid.New()
		_, err := fake.CreateStatus(ctx, store.CreateStatusInput{
			ID:         expiredStatusID,
			URI:        "https://example.com/statuses/" + expiredStatusID,
			AccountID:  acc.ID,
			Text:       testutil.StrPtr("Expired poll"),
			Content:    testutil.StrPtr("<p>Expired poll</p>"),
			Visibility: domain.VisibilityPublic,
			APID:       "https://example.com/statuses/" + expiredStatusID,
			Local:      true,
		})
		require.NoError(t, err)
		expiredPollID := uid.New()
		_, err = fake.CreatePoll(ctx, store.CreatePollInput{
			ID:        expiredPollID,
			StatusID:  expiredStatusID,
			ExpiresAt: &expiredAt,
			Multiple:  false,
		})
		require.NoError(t, err)
		_, err = fake.CreatePollOption(ctx, store.CreatePollOptionInput{ID: uid.New(), PollID: expiredPollID, Title: "A", Position: 0})
		require.NoError(t, err)

		_, err = interactionSvc.RecordVote(ctx, expiredPollID, acc.ID, []int{0})
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrUnprocessable)
	})
}
