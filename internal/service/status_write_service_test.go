package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/chairswithlegs/monstera/internal/uid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusWriteService_Delete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	conversationSvc := NewConversationService(fake, statusSvc)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, conversationSvc, "https://example.com", "example.com", 500)

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	st, err := statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  acc.ID,
		Text:       "To be deleted",
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)

	err = statusWriteSvc.Delete(ctx, st.Status.ID)
	require.NoError(t, err)

	_, err = statusSvc.GetByID(ctx, st.Status.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestStatusWriteService_Create_empty_text_returns_validation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	conversationSvc := NewConversationService(fake, statusSvc)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, conversationSvc, "https://example.com", "example.com", 500)

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)

	_, err = statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  acc.ID,
		Username:   acc.Username,
		Text:       "   ",
		Visibility: domain.VisibilityPublic,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestStatusWriteService_Create_over_char_limit_returns_validation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 10)
	conversationSvc := NewConversationService(fake, statusSvc)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, conversationSvc, "https://example.com", "example.com", 10)

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)

	_, err = statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  acc.ID,
		Username:   acc.Username,
		Text:       "this is way over ten characters",
		Visibility: domain.VisibilityPublic,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestStatusWriteService_Create_success_returns_result_with_author(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	conversationSvc := NewConversationService(fake, statusSvc)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, conversationSvc, "https://example.com", "example.com", 500)

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)

	result, err := statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  acc.ID,
		Username:   acc.Username,
		Text:       "Hello world",
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)
	require.NotNil(t, result.Status)
	require.NotNil(t, result.Author)
	assert.Equal(t, acc.ID, result.Status.AccountID)
	assert.Equal(t, acc.ID, result.Author.ID)
	assert.Equal(t, "Hello world", *result.Status.Text)
	assert.Empty(t, result.Mentions)
	assert.Empty(t, result.Tags)
	assert.Empty(t, result.Media)
	assert.Contains(t, result.Status.URI, "/users/alice/statuses/")
}

func TestStatusWriteService_Create_no_mention_notification_when_mentionee_muted_conversation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	conversationSvc := NewConversationService(fake, statusSvc)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, conversationSvc, "https://example.com", "example.com", 500)

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

	root, err := statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  alice.ID,
		Username:   alice.Username,
		Text:       "root post",
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)

	err = statusWriteSvc.MuteConversation(ctx, bob.ID, root.Status.ID)
	require.NoError(t, err)

	_, err = statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:   alice.ID,
		Username:    alice.Username,
		Text:        "hey @bob",
		Visibility:  domain.VisibilityPublic,
		InReplyToID: &root.Status.ID,
	})
	require.NoError(t, err)

	notifs, err := fake.ListNotifications(ctx, bob.ID, nil, 10)
	require.NoError(t, err)
	assert.Empty(t, notifs, "bob should receive no mention notification when conversation is muted")
}

func TestStatusWriteService_CreateReblog(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)

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

	result, err := statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  alice.ID,
		Username:   alice.Username,
		Text:       "rebloggable",
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)
	statusID := result.Status.ID

	t.Run("success creates reblog and increments count", func(t *testing.T) {
		enriched, err := statusWriteSvc.CreateReblog(ctx, bob.ID, bob.Username, statusID)
		require.NoError(t, err)
		require.NotNil(t, enriched.Status)
		assert.Equal(t, &statusID, enriched.Status.ReblogOfID)
		st, err := fake.GetStatusByID(ctx, statusID)
		require.NoError(t, err)
		assert.Equal(t, 1, st.ReblogsCount)
	})

	t.Run("duplicate reblog returns ErrConflict", func(t *testing.T) {
		_, err := statusWriteSvc.CreateReblog(ctx, bob.ID, bob.Username, statusID)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrConflict)
	})

	t.Run("nonexistent status returns ErrNotFound", func(t *testing.T) {
		_, err := statusWriteSvc.CreateReblog(ctx, bob.ID, bob.Username, "01H0000000000000000000000")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestStatusWriteService_DeleteReblog(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)

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

	result, err := statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  alice.ID,
		Username:   alice.Username,
		Text:       "rebloggable",
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)
	statusID := result.Status.ID

	_, err = statusWriteSvc.CreateReblog(ctx, bob.ID, bob.Username, statusID)
	require.NoError(t, err)

	t.Run("delete removes reblog and decrements count", func(t *testing.T) {
		err := statusWriteSvc.DeleteReblog(ctx, bob.ID, statusID)
		require.NoError(t, err)
		st, err := fake.GetStatusByID(ctx, statusID)
		require.NoError(t, err)
		assert.Equal(t, 0, st.ReblogsCount)
	})

	t.Run("idempotent: delete nonexistent reblog succeeds", func(t *testing.T) {
		err := statusWriteSvc.DeleteReblog(ctx, bob.ID, statusID)
		require.NoError(t, err)
	})
}

func TestStatusWriteService_CreateFavourite(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)

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

	result, err := statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  alice.ID,
		Username:   alice.Username,
		Text:       "favouritable",
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)
	statusID := result.Status.ID

	t.Run("success returns status and increments count", func(t *testing.T) {
		enriched, err := statusWriteSvc.CreateFavourite(ctx, bob.ID, statusID)
		require.NoError(t, err)
		assert.Equal(t, statusID, enriched.Status.ID)
		st, err := fake.GetStatusByID(ctx, statusID)
		require.NoError(t, err)
		assert.Equal(t, 1, st.FavouritesCount)
	})

	t.Run("not found returns ErrNotFound", func(t *testing.T) {
		_, err := statusWriteSvc.CreateFavourite(ctx, bob.ID, "01H0000000000000000000000")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestStatusWriteService_DeleteFavourite(t *testing.T) {
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
		AccountID:  acc.ID,
		Username:   acc.Username,
		Text:       "unfavouritable",
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)
	statusID := result.Status.ID

	_, err = statusWriteSvc.CreateFavourite(ctx, acc.ID, statusID)
	require.NoError(t, err)

	t.Run("delete returns status", func(t *testing.T) {
		enriched, err := statusWriteSvc.DeleteFavourite(ctx, acc.ID, statusID)
		require.NoError(t, err)
		assert.Equal(t, statusID, enriched.Status.ID)
	})

	t.Run("idempotent: delete again succeeds", func(t *testing.T) {
		_, err := statusWriteSvc.DeleteFavourite(ctx, acc.ID, statusID)
		require.NoError(t, err)
	})
}

func TestStatusWriteService_Bookmark(t *testing.T) {
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

	t.Run("bookmark returns status with bookmarked true", func(t *testing.T) {
		enriched, err := statusWriteSvc.Bookmark(ctx, acc.ID, statusID)
		require.NoError(t, err)
		assert.Equal(t, statusID, enriched.Status.ID)
		assert.True(t, enriched.Bookmarked)
	})

	t.Run("idempotent: double bookmark succeeds", func(t *testing.T) {
		enriched, err := statusWriteSvc.Bookmark(ctx, acc.ID, statusID)
		require.NoError(t, err)
		assert.True(t, enriched.Bookmarked)
	})

	t.Run("nonexistent status returns ErrNotFound", func(t *testing.T) {
		_, err := statusWriteSvc.Bookmark(ctx, acc.ID, "01H0000000000000000000000")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestStatusWriteService_Unbookmark(t *testing.T) {
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
		Text:       "unbookmarkable",
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)
	statusID := result.Status.ID

	_, err = statusWriteSvc.Bookmark(ctx, acc.ID, statusID)
	require.NoError(t, err)

	t.Run("unbookmark returns status with bookmarked false", func(t *testing.T) {
		enriched, err := statusWriteSvc.Unbookmark(ctx, acc.ID, statusID)
		require.NoError(t, err)
		assert.Equal(t, statusID, enriched.Status.ID)
		assert.False(t, enriched.Bookmarked)
	})

	t.Run("idempotent: unbookmark again succeeds", func(t *testing.T) {
		_, err := statusWriteSvc.Unbookmark(ctx, acc.ID, statusID)
		require.NoError(t, err)
	})
}

func TestStatusWriteService_Pin(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)

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

	result, err := statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  alice.ID,
		Username:   alice.Username,
		Text:       "pinnable",
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)
	statusID := result.Status.ID

	t.Run("owner pins own public status", func(t *testing.T) {
		enriched, err := statusWriteSvc.Pin(ctx, alice.ID, statusID)
		require.NoError(t, err)
		assert.Equal(t, statusID, enriched.Status.ID)
		assert.True(t, enriched.Pinned)
	})

	t.Run("non-owner returns ErrForbidden", func(t *testing.T) {
		_, err := statusWriteSvc.Pin(ctx, bob.ID, statusID)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("nonexistent status returns ErrNotFound", func(t *testing.T) {
		_, err := statusWriteSvc.Pin(ctx, alice.ID, "01H0000000000000000000000")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestStatusWriteService_Unpin(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)

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

	result, err := statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  alice.ID,
		Username:   alice.Username,
		Text:       "pinnable",
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)
	statusID := result.Status.ID

	_, err = statusWriteSvc.Pin(ctx, alice.ID, statusID)
	require.NoError(t, err)

	t.Run("owner unpins", func(t *testing.T) {
		enriched, err := statusWriteSvc.Unpin(ctx, alice.ID, statusID)
		require.NoError(t, err)
		assert.Equal(t, statusID, enriched.Status.ID)
		assert.False(t, enriched.Pinned)
	})

	t.Run("non-owner returns ErrForbidden", func(t *testing.T) {
		_, err := statusWriteSvc.Unpin(ctx, bob.ID, statusID)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func TestStatusWriteService_RecordVote(t *testing.T) {
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
		poll, err := statusWriteSvc.RecordVote(ctx, pollID, acc.ID, []int{0, 2})
		require.NoError(t, err)
		assert.True(t, poll.Voted)
		assert.Equal(t, []int{0, 2}, poll.OwnVotes)
	})

	t.Run("empty choices returns ErrValidation", func(t *testing.T) {
		_, err := statusWriteSvc.RecordVote(ctx, pollID, acc.ID, []int{})
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrValidation)
	})

	t.Run("out-of-range index returns ErrValidation", func(t *testing.T) {
		_, err := statusWriteSvc.RecordVote(ctx, pollID, acc.ID, []int{99})
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrValidation)
	})

	t.Run("nonexistent poll returns ErrNotFound", func(t *testing.T) {
		_, err := statusWriteSvc.RecordVote(ctx, "01H0000000000000000000000", acc.ID, []int{0})
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

		_, err = statusWriteSvc.RecordVote(ctx, expiredPollID, acc.ID, []int{0})
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrUnprocessable)
	})
}

func TestStatusWriteService_MuteConversation(t *testing.T) {
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
		AccountID:  acc.ID,
		Username:   acc.Username,
		Text:       "conversational",
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)
	statusID := result.Status.ID

	t.Run("mute makes IsConversationMutedForViewer return true", func(t *testing.T) {
		err := statusWriteSvc.MuteConversation(ctx, acc.ID, statusID)
		require.NoError(t, err)
		muted, err := statusSvc.IsConversationMutedForViewer(ctx, acc.ID, statusID)
		require.NoError(t, err)
		assert.True(t, muted)
	})

	t.Run("unmute makes IsConversationMutedForViewer return false", func(t *testing.T) {
		err := statusWriteSvc.UnmuteConversation(ctx, acc.ID, statusID)
		require.NoError(t, err)
		muted, err := statusSvc.IsConversationMutedForViewer(ctx, acc.ID, statusID)
		require.NoError(t, err)
		assert.False(t, muted)
	})
}

func TestStatusWriteService_CreateScheduledStatus(t *testing.T) {
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

	t.Run("success returns scheduled status", func(t *testing.T) {
		params := []byte(`{"text":"scheduled post","visibility":"public"}`)
		scheduledAt := time.Now().Add(1 * time.Hour)
		s, err := statusWriteSvc.CreateScheduledStatus(ctx, acc.ID, params, scheduledAt)
		require.NoError(t, err)
		require.NotNil(t, s)
		assert.Equal(t, acc.ID, s.AccountID)
		assert.False(t, s.ScheduledAt.IsZero())
	})

	t.Run("past scheduled_at returns ErrValidation", func(t *testing.T) {
		params := []byte(`{"text":"late"}`)
		_, err := statusWriteSvc.CreateScheduledStatus(ctx, acc.ID, params, time.Now().Add(-1*time.Hour))
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrValidation)
	})

	t.Run("invalid params JSON returns ErrValidation", func(t *testing.T) {
		_, err := statusWriteSvc.CreateScheduledStatus(ctx, acc.ID, []byte("not json"), time.Now().Add(1*time.Hour))
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrValidation)
	})
}

func TestStatusWriteService_UpdateScheduledStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)

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
	params := []byte(`{"text":"original"}`)
	_, err = fake.CreateScheduledStatus(ctx, store.CreateScheduledStatusInput{
		ID:          schedID,
		AccountID:   alice.ID,
		Params:      params,
		ScheduledAt: time.Now().Add(1 * time.Hour),
	})
	require.NoError(t, err)

	newTime := time.Now().Add(3 * time.Hour)
	newParams := []byte(`{"text":"updated"}`)

	t.Run("success updates scheduled_at and params", func(t *testing.T) {
		updated, err := statusWriteSvc.UpdateScheduledStatus(ctx, schedID, alice.ID, newParams, newTime)
		require.NoError(t, err)
		require.NotNil(t, updated)
		assert.Equal(t, schedID, updated.ID)
	})

	t.Run("other account returns ErrNotFound", func(t *testing.T) {
		_, err := statusWriteSvc.UpdateScheduledStatus(ctx, schedID, bob.ID, newParams, newTime)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("past scheduled_at returns ErrValidation", func(t *testing.T) {
		_, err := statusWriteSvc.UpdateScheduledStatus(ctx, schedID, alice.ID, newParams, time.Now().Add(-1*time.Hour))
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrValidation)
	})

	t.Run("nonexistent returns ErrNotFound", func(t *testing.T) {
		_, err := statusWriteSvc.UpdateScheduledStatus(ctx, "01H0000000000000000000000", alice.ID, newParams, newTime)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestStatusWriteService_DeleteScheduledStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)

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
		Params:      []byte(`{"text":"delete me"}`),
		ScheduledAt: time.Now().Add(1 * time.Hour),
	})
	require.NoError(t, err)

	t.Run("other account returns ErrNotFound", func(t *testing.T) {
		err := statusWriteSvc.DeleteScheduledStatus(ctx, schedID, bob.ID)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("owner deletes successfully", func(t *testing.T) {
		err := statusWriteSvc.DeleteScheduledStatus(ctx, schedID, alice.ID)
		require.NoError(t, err)
		_, err = fake.GetScheduledStatusByID(ctx, schedID)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("nonexistent returns ErrNotFound", func(t *testing.T) {
		err := statusWriteSvc.DeleteScheduledStatus(ctx, "01H0000000000000000000000", alice.ID)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestStatusWriteService_PublishScheduled(t *testing.T) {
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

	params := domain.ScheduledStatusParams{Text: "published by worker", Language: "en"}
	paramsJSON, err := json.Marshal(params)
	require.NoError(t, err)

	schedID := uid.New()
	_, err = fake.CreateScheduledStatus(ctx, store.CreateScheduledStatusInput{
		ID:          schedID,
		AccountID:   acc.ID,
		Params:      paramsJSON,
		ScheduledAt: time.Now().Add(-1 * time.Minute),
	})
	require.NoError(t, err)

	err = statusWriteSvc.PublishScheduled(ctx, schedID)
	require.NoError(t, err)

	_, err = fake.GetScheduledStatusByID(ctx, schedID)
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrNotFound)

	statuses, err := fake.GetAccountPublicStatuses(ctx, acc.ID, nil, 10)
	require.NoError(t, err)
	require.Len(t, statuses, 1)
	assert.Contains(t, *statuses[0].Content, "published by worker")
}

func TestStatusWriteService_workerPublishScheduled(t *testing.T) {
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

	dueParams, err := json.Marshal(domain.ScheduledStatusParams{Text: "due post", Language: "en"})
	require.NoError(t, err)
	futureParams, err := json.Marshal(domain.ScheduledStatusParams{Text: "future post", Language: "en"})
	require.NoError(t, err)

	dueID := uid.New()
	futureID := uid.New()
	_, err = fake.CreateScheduledStatus(ctx, store.CreateScheduledStatusInput{
		ID:          dueID,
		AccountID:   acc.ID,
		Params:      dueParams,
		ScheduledAt: time.Now().Add(-1 * time.Hour),
	})
	require.NoError(t, err)
	_, err = fake.CreateScheduledStatus(ctx, store.CreateScheduledStatusInput{
		ID:          futureID,
		AccountID:   acc.ID,
		Params:      futureParams,
		ScheduledAt: time.Now().Add(2 * time.Hour),
	})
	require.NoError(t, err)

	dueList, err := fake.ListScheduledStatusesDue(ctx, 20)
	require.NoError(t, err)
	require.Len(t, dueList, 1)
	assert.Equal(t, dueID, dueList[0].ID)

	for i := range dueList {
		err = statusWriteSvc.PublishScheduled(ctx, dueList[i].ID)
		require.NoError(t, err)
	}

	_, err = fake.GetScheduledStatusByID(ctx, dueID)
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = fake.GetScheduledStatusByID(ctx, futureID)
	require.NoError(t, err)

	statuses, err := fake.GetAccountPublicStatuses(ctx, acc.ID, nil, 10)
	require.NoError(t, err)
	require.Len(t, statuses, 1)
	assert.Contains(t, *statuses[0].Content, "due post")
}

func TestStatusWriteService_Create_quote(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	conversationSvc := NewConversationService(fake, statusSvc)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, conversationSvc, "https://example.com", "example.com", 500)

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

	quotedID := uid.New()
	_, err = fake.CreateStatus(ctx, store.CreateStatusInput{
		ID:                  quotedID,
		URI:                 "https://example.com/statuses/" + quotedID,
		AccountID:           alice.ID,
		Text:                testutil.StrPtr("original"),
		Content:             testutil.StrPtr("<p>original</p>"),
		Visibility:          domain.VisibilityPublic,
		QuoteApprovalPolicy: domain.QuotePolicyPublic,
		APID:                "https://example.com/statuses/" + quotedID,
		Local:               true,
	})
	require.NoError(t, err)

	t.Run("success creates quote and approval", func(t *testing.T) {
		enriched, err := statusWriteSvc.Create(ctx, CreateStatusInput{
			AccountID:      bob.ID,
			Username:       "bob",
			Text:           "quoting alice",
			Visibility:     domain.VisibilityPublic,
			QuotedStatusID: &quotedID,
		})
		require.NoError(t, err)
		require.NotNil(t, enriched.Status)
		assert.Equal(t, quotedID, *enriched.Status.QuotedStatusID)
		assert.Equal(t, domain.QuotePolicyPublic, enriched.Status.QuoteApprovalPolicy)
		approval, err := fake.GetQuoteApproval(ctx, enriched.Status.ID)
		require.NoError(t, err)
		require.NotNil(t, approval)
		assert.Nil(t, approval.RevokedAt)
		quoted, _ := fake.GetStatusByID(ctx, quotedID)
		require.NotNil(t, quoted)
		assert.Equal(t, 1, quoted.QuotesCount)
	})

	t.Run("quote_approval_policy nobody returns ErrForbidden for non-author", func(t *testing.T) {
		nobodyID := uid.New()
		_, err := fake.CreateStatus(ctx, store.CreateStatusInput{
			ID:                  nobodyID,
			URI:                 "https://example.com/statuses/" + nobodyID,
			AccountID:           alice.ID,
			Text:                testutil.StrPtr("nobody"),
			Content:             testutil.StrPtr("<p>nobody</p>"),
			Visibility:          domain.VisibilityPublic,
			QuoteApprovalPolicy: domain.QuotePolicyNobody,
			APID:                "https://example.com/statuses/" + nobodyID,
			Local:               true,
		})
		require.NoError(t, err)
		_, err = statusWriteSvc.Create(ctx, CreateStatusInput{
			AccountID:      bob.ID,
			Username:       "bob",
			Text:           "trying to quote",
			Visibility:     domain.VisibilityPublic,
			QuotedStatusID: &nobodyID,
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func TestStatusWriteService_RevokeQuote(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)

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

	quotedID := uid.New()
	_, err = fake.CreateStatus(ctx, store.CreateStatusInput{
		ID:         quotedID,
		URI:        "https://example.com/statuses/" + quotedID,
		AccountID:  alice.ID,
		Text:       testutil.StrPtr("alice post"),
		Content:    testutil.StrPtr("<p>alice</p>"),
		Visibility: domain.VisibilityPublic,
		APID:       "https://example.com/statuses/" + quotedID,
		Local:      true,
	})
	require.NoError(t, err)
	quotingID := uid.New()
	_, err = fake.CreateStatus(ctx, store.CreateStatusInput{
		ID:                  quotingID,
		URI:                 "https://example.com/statuses/" + quotingID,
		AccountID:           bob.ID,
		Text:                testutil.StrPtr("bob quote"),
		Content:             testutil.StrPtr("<p>bob quote</p>"),
		Visibility:          domain.VisibilityPublic,
		QuotedStatusID:      &quotedID,
		QuoteApprovalPolicy: domain.QuotePolicyPublic,
		APID:                "https://example.com/statuses/" + quotingID,
		Local:               true,
	})
	require.NoError(t, err)
	require.NoError(t, fake.CreateQuoteApproval(ctx, quotingID, quotedID))
	require.NoError(t, fake.IncrementQuotesCount(ctx, quotedID))

	t.Run("non-owner returns ErrForbidden", func(t *testing.T) {
		err := statusWriteSvc.RevokeQuote(ctx, bob.ID, quotedID, quotingID)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("owner revokes success", func(t *testing.T) {
		err := statusWriteSvc.RevokeQuote(ctx, alice.ID, quotedID, quotingID)
		require.NoError(t, err)
		approval, err := fake.GetQuoteApproval(ctx, quotingID)
		require.NoError(t, err)
		require.NotNil(t, approval)
		assert.NotNil(t, approval.RevokedAt)
		quoted, _ := fake.GetStatusByID(ctx, quotedID)
		require.NotNil(t, quoted)
		assert.Equal(t, 0, quoted.QuotesCount)
	})

	t.Run("nonexistent quoting status returns ErrNotFound", func(t *testing.T) {
		err := statusWriteSvc.RevokeQuote(ctx, alice.ID, quotedID, "01H0000000000000000000000")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestStatusWriteService_UpdateQuoteApprovalPolicy(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)

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

	statusID := uid.New()
	_, err = fake.CreateStatus(ctx, store.CreateStatusInput{
		ID:         statusID,
		URI:        "https://example.com/statuses/" + statusID,
		AccountID:  alice.ID,
		Text:       testutil.StrPtr("my status"),
		Content:    testutil.StrPtr("<p>my status</p>"),
		Visibility: domain.VisibilityPublic,
		APID:       "https://example.com/statuses/" + statusID,
		Local:      true,
	})
	require.NoError(t, err)

	t.Run("owner updates policy success", func(t *testing.T) {
		err := statusWriteSvc.UpdateQuoteApprovalPolicy(ctx, alice.ID, statusID, domain.QuotePolicyFollowers)
		require.NoError(t, err)
		st, _ := fake.GetStatusByID(ctx, statusID)
		require.NotNil(t, st)
		assert.Equal(t, domain.QuotePolicyFollowers, st.QuoteApprovalPolicy)
	})

	t.Run("non-owner returns ErrForbidden", func(t *testing.T) {
		err := statusWriteSvc.UpdateQuoteApprovalPolicy(ctx, bob.ID, statusID, domain.QuotePolicyNobody)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("invalid policy returns ErrValidation", func(t *testing.T) {
		err := statusWriteSvc.UpdateQuoteApprovalPolicy(ctx, alice.ID, statusID, "invalid")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrValidation)
	})

	t.Run("nonexistent status returns ErrNotFound", func(t *testing.T) {
		err := statusWriteSvc.UpdateQuoteApprovalPolicy(ctx, alice.ID, "01H0000000000000000000000", domain.QuotePolicyPublic)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestStatusWriteService_Update_quoted_update_notification(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	convSvc := NewConversationService(fake, statusSvc)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, convSvc, "https://example.com", "example.com", 500)

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

	quotedID := uid.New()
	_, err = fake.CreateStatus(ctx, store.CreateStatusInput{
		ID:         quotedID,
		URI:        "https://example.com/statuses/" + quotedID,
		AccountID:  alice.ID,
		Text:       testutil.StrPtr("original"),
		Content:    testutil.StrPtr("<p>original</p>"),
		Visibility: domain.VisibilityPublic,
		APID:       "https://example.com/statuses/" + quotedID,
		Local:      true,
	})
	require.NoError(t, err)
	quotingID := uid.New()
	_, err = fake.CreateStatus(ctx, store.CreateStatusInput{
		ID:                  quotingID,
		URI:                 "https://example.com/statuses/" + quotingID,
		AccountID:           bob.ID,
		Text:                testutil.StrPtr("bob quote"),
		Content:             testutil.StrPtr("<p>bob quote</p>"),
		Visibility:          domain.VisibilityPublic,
		QuotedStatusID:      &quotedID,
		QuoteApprovalPolicy: domain.QuotePolicyPublic,
		APID:                "https://example.com/statuses/" + quotingID,
		Local:               true,
	})
	require.NoError(t, err)
	require.NoError(t, fake.CreateQuoteApproval(ctx, quotingID, quotedID))

	_, err = statusWriteSvc.Update(ctx, UpdateStatusInput{AccountID: alice.ID, StatusID: quotedID, Text: "edited text"})
	require.NoError(t, err)

	notifications, err := fake.ListNotifications(ctx, bob.ID, nil, 20)
	require.NoError(t, err)
	var quotedUpdate *domain.Notification
	for i := range notifications {
		if notifications[i].Type == domain.NotificationTypeQuotedUpdate {
			quotedUpdate = &notifications[i]
			break
		}
	}
	require.NotNil(t, quotedUpdate, "quoter should receive quoted_update notification")
	assert.Equal(t, alice.ID, quotedUpdate.FromID)
	assert.Equal(t, quotingID, *quotedUpdate.StatusID)
}
