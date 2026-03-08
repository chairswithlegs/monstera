package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/events"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/chairswithlegs/monstera/internal/uid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const privatePostText = "private post"

func TestStatusService_Create(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)

	text := "Hello world"
	st, err := statusSvc.Create(ctx, CreateStatusInput{
		AccountID:  acc.ID,
		Text:       &text,
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)
	require.NotNil(t, st)
	assert.Equal(t, acc.ID, st.AccountID)
	assert.Equal(t, "Hello world", *st.Text)
	assert.Equal(t, domain.VisibilityPublic, st.Visibility)
	assert.Contains(t, st.URI, "statuses/")
	assert.True(t, st.Local)
}

func TestStatusService_Create_nil_text_returns_validation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)

	_, err = statusSvc.Create(ctx, CreateStatusInput{
		AccountID:  acc.ID,
		Text:       nil,
		Visibility: domain.VisibilityPublic,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestStatusService_Create_invalid_visibility_returns_validation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)

	text := "Hello"
	_, err = statusSvc.Create(ctx, CreateStatusInput{
		AccountID:  acc.ID,
		Text:       &text,
		Visibility: "invalid",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestStatusService_GetByID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	text := "Hello"
	created, err := statusSvc.Create(ctx, CreateStatusInput{
		AccountID:  acc.ID,
		Text:       &text,
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)

	got, err := statusSvc.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, "Hello", *got.Text)
}

func TestStatusService_GetByID_not_found(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	statusSvc := NewStatusService(fake, NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())

	_, err := statusSvc.GetByID(ctx, "01nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestStatusService_Delete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	text := "To be deleted"
	st, err := statusSvc.Create(ctx, CreateStatusInput{
		AccountID:  acc.ID,
		Text:       &text,
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)

	err = statusSvc.Delete(ctx, st.ID)
	require.NoError(t, err)

	_, err = statusSvc.GetByID(ctx, st.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestStatusService_CreateWithContent_empty_text_returns_validation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)

	_, err = statusSvc.CreateWithContent(ctx, CreateWithContentInput{
		AccountID:  acc.ID,
		Username:   acc.Username,
		Text:       "   ",
		Visibility: domain.VisibilityPublic,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestStatusService_CreateWithContent_over_char_limit_returns_validation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 10, slog.Default())

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)

	_, err = statusSvc.CreateWithContent(ctx, CreateWithContentInput{
		AccountID:  acc.ID,
		Username:   acc.Username,
		Text:       "this is way over ten characters",
		Visibility: domain.VisibilityPublic,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestStatusService_CreateWithContent_success_returns_result_with_author(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)

	result, err := statusSvc.CreateWithContent(ctx, CreateWithContentInput{
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

func TestStatusService_CreateWithContent_no_mention_notification_when_mentionee_muted_conversation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())

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

	root, err := statusSvc.CreateWithContent(ctx, CreateWithContentInput{
		AccountID:  alice.ID,
		Username:   alice.Username,
		Text:       "root post",
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)

	err = statusSvc.MuteConversation(ctx, bob.ID, root.Status.ID)
	require.NoError(t, err)

	_, err = statusSvc.CreateWithContent(ctx, CreateWithContentInput{
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

func TestStatusService_GetByIDEnriched_private_returns_ErrNotFound_when_unauthenticated(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	text := privatePostText
	st, err := statusSvc.Create(ctx, CreateStatusInput{
		AccountID:  acc.ID,
		Text:       &text,
		Visibility: domain.VisibilityPrivate,
	})
	require.NoError(t, err)

	_, err = statusSvc.GetByIDEnriched(ctx, st.ID, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestStatusService_GetByIDEnriched_private_returns_success_when_viewer_is_author(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	text := privatePostText
	st, err := statusSvc.Create(ctx, CreateStatusInput{
		AccountID:  acc.ID,
		Text:       &text,
		Visibility: domain.VisibilityPrivate,
	})
	require.NoError(t, err)
	viewerID := acc.ID

	result, err := statusSvc.GetByIDEnriched(ctx, st.ID, &viewerID)
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
	statusSvc := NewStatusService(fake, NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())

	author, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	viewer, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)
	text := "public post"
	st, err := statusSvc.Create(ctx, CreateStatusInput{
		AccountID:  author.ID,
		Text:       &text,
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)
	err = fake.CreateBlock(ctx, store.CreateBlockInput{ID: "01block", AccountID: author.ID, TargetID: viewer.ID})
	require.NoError(t, err)

	_, err = statusSvc.GetByIDEnriched(ctx, st.ID, &viewer.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestStatusService_PublishScheduled(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())

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

	err = statusSvc.PublishScheduled(ctx, schedID)
	require.NoError(t, err)

	_, err = fake.GetScheduledStatusByID(ctx, schedID)
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrNotFound)

	statuses, err := fake.GetAccountPublicStatuses(ctx, acc.ID, nil, 10)
	require.NoError(t, err)
	require.Len(t, statuses, 1)
	assert.Contains(t, *statuses[0].Content, "published by worker")
}

// TestStatusService_workerPublishScheduled simulates the scheduled-status worker: list due, then publish each.
func TestStatusService_workerPublishScheduled(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())

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
		err = statusSvc.PublishScheduled(ctx, dueList[i].ID)
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
