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

	err = statusWriteSvc.Delete(ctx, st.Status.ID, acc.ID)
	require.NoError(t, err)

	_, err = statusSvc.GetByID(ctx, st.Status.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestStatusWriteService_Delete_forbidden_for_non_owner(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	conversationSvc := NewConversationService(fake, statusSvc)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, conversationSvc, "https://example.com", "example.com", 500)

	alice, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	bob, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)
	st, err := statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  alice.ID,
		Text:       "Alice's post",
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)

	err = statusWriteSvc.Delete(ctx, st.Status.ID, bob.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrForbidden)
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

	root, err := statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  alice.ID,
		Username:   alice.Username,
		Text:       "root post",
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)

	err = conversationSvc.MuteConversation(ctx, bob.ID, root.Status.ID)
	require.NoError(t, err)

	_, err = statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:   alice.ID,
		Username:    alice.Username,
		Text:        "hey @bob",
		Visibility:  domain.VisibilityPublic,
		InReplyToID: &root.Status.ID,
	})
	require.NoError(t, err)

	notifs, err := fake.ListNotifications(ctx, bob.ID, nil, 10, nil, nil)
	require.NoError(t, err)
	assert.Empty(t, notifs, "bob should receive no mention notification when conversation is muted")
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

	notifications, err := fake.ListNotifications(ctx, bob.ID, nil, 20, nil, nil)
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
