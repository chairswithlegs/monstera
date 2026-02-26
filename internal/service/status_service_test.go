package service

import (
	"context"
	"testing"

	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusService_Create(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)

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
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)

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
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)

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
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)

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
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)

	_, err := statusSvc.GetByID(ctx, "01nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestStatusService_Delete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)

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
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)

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
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 10)

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
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)

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
