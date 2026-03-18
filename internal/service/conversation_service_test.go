package service

import (
	"context"
	"testing"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/chairswithlegs/monstera/internal/uid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConversationService_ListConversations_empty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	convSvc := NewConversationService(fake, statusSvc)

	acc, err := accountSvc.Register(ctx, RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	list, next, err := convSvc.ListConversations(ctx, acc.ID, nil, 40)
	require.NoError(t, err)
	assert.Empty(t, list)
	assert.Nil(t, next)
}

func TestConversationService_UpdateForDirectStatus_creates_conversation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	convSvc := NewConversationService(fake, statusSvc)

	alice, err := accountSvc.Register(ctx, RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)
	bob, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	st := &domain.Status{
		ID:         uid.New(),
		AccountID:  alice.ID,
		Visibility: domain.VisibilityDirect,
	}
	_, err = fake.CreateStatus(ctx, store.CreateStatusInput{
		ID:         st.ID,
		URI:        "https://example.com/statuses/" + st.ID,
		AccountID:  st.AccountID,
		Visibility: st.Visibility,
		APID:       "https://example.com/statuses/" + st.ID,
		Local:      true,
	})
	require.NoError(t, err)
	require.NoError(t, fake.CreateStatusMention(ctx, st.ID, bob.ID))

	err = convSvc.UpdateForDirectStatus(ctx, st, alice.ID, []string{bob.ID})
	require.NoError(t, err)

	list, _, err := convSvc.ListConversations(ctx, alice.ID, nil, 40)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, alice.ID, list[0].AccountConversation.AccountID)
	assert.False(t, list[0].AccountConversation.Unread)
	assert.Len(t, list[0].Participants, 1)
	assert.Equal(t, bob.ID, list[0].Participants[0].ID)

	listBob, _, err := convSvc.ListConversations(ctx, bob.ID, nil, 40)
	require.NoError(t, err)
	require.Len(t, listBob, 1)
	assert.True(t, listBob[0].AccountConversation.Unread)
}

func TestConversationService_MarkConversationRead(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	convSvc := NewConversationService(fake, statusSvc)

	acc, err := accountSvc.Register(ctx, RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)
	convID := uid.New()
	require.NoError(t, fake.CreateConversation(ctx, convID))
	require.NoError(t, fake.UpsertAccountConversation(ctx, store.UpsertAccountConversationInput{
		ID:             uid.New(),
		AccountID:      acc.ID,
		ConversationID: convID,
		LastStatusID:   "",
		Unread:         true,
	}))

	result, err := convSvc.MarkConversationRead(ctx, acc.ID, convID)
	require.NoError(t, err)
	assert.False(t, result.AccountConversation.Unread)
	assert.Equal(t, convID, result.AccountConversation.ConversationID)
}

func TestConversationService_RemoveConversation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	convSvc := NewConversationService(fake, statusSvc)

	acc, err := accountSvc.Register(ctx, RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)
	convID := uid.New()
	require.NoError(t, fake.CreateConversation(ctx, convID))
	require.NoError(t, fake.UpsertAccountConversation(ctx, store.UpsertAccountConversationInput{
		ID:             uid.New(),
		AccountID:      acc.ID,
		ConversationID: convID,
		LastStatusID:   "",
		Unread:         true,
	}))

	err = convSvc.RemoveConversation(ctx, acc.ID, convID)
	require.NoError(t, err)

	list, _, err := convSvc.ListConversations(ctx, acc.ID, nil, 40)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestConversationService_MarkConversationRead_not_found(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	convSvc := NewConversationService(fake, statusSvc)

	acc, err := accountSvc.Register(ctx, RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	_, err = convSvc.MarkConversationRead(ctx, acc.ID, "nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestConversationService_MuteConversation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	convSvc := NewConversationService(fake, statusSvc)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, convSvc, "https://example.com", "example.com", 500)

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
		Text:       "conversational",
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)
	statusID := result.Status.ID

	t.Run("mute makes conversation muted in store", func(t *testing.T) {
		err := convSvc.MuteConversation(ctx, acc.ID, statusID)
		require.NoError(t, err)
		root, err := fake.GetConversationRoot(ctx, statusID)
		require.NoError(t, err)
		muted, err := fake.IsConversationMuted(ctx, acc.ID, root)
		require.NoError(t, err)
		assert.True(t, muted)
	})

	t.Run("unmute makes conversation unmuted in store", func(t *testing.T) {
		err := convSvc.UnmuteConversation(ctx, acc.ID, statusID)
		require.NoError(t, err)
		root, err := fake.GetConversationRoot(ctx, statusID)
		require.NoError(t, err)
		muted, err := fake.IsConversationMuted(ctx, acc.ID, root)
		require.NoError(t, err)
		assert.False(t, muted)
	})
}
