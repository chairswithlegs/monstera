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

func TestScheduledStatusService_CreateScheduledStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)
	scheduledSvc := NewScheduledStatusService(fake, statusWriteSvc)

	acc, err := accountSvc.Register(ctx, RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("success returns scheduled status", func(t *testing.T) {
		params := []byte(`{"text":"scheduled post","visibility":"public"}`)
		scheduledAt := time.Now().Add(1 * time.Hour)
		s, err := scheduledSvc.CreateScheduledStatus(ctx, acc.ID, params, scheduledAt)
		require.NoError(t, err)
		require.NotNil(t, s)
		assert.Equal(t, acc.ID, s.AccountID)
		assert.False(t, s.ScheduledAt.IsZero())
	})

	t.Run("past scheduled_at returns ErrValidation", func(t *testing.T) {
		params := []byte(`{"text":"late"}`)
		_, err := scheduledSvc.CreateScheduledStatus(ctx, acc.ID, params, time.Now().Add(-1*time.Hour))
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrValidation)
	})

	t.Run("invalid params JSON returns ErrValidation", func(t *testing.T) {
		_, err := scheduledSvc.CreateScheduledStatus(ctx, acc.ID, []byte("not json"), time.Now().Add(1*time.Hour))
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrValidation)
	})
}

func TestScheduledStatusService_UpdateScheduledStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)
	scheduledSvc := NewScheduledStatusService(fake, statusWriteSvc)

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
		updated, err := scheduledSvc.UpdateScheduledStatus(ctx, schedID, alice.ID, newParams, newTime)
		require.NoError(t, err)
		require.NotNil(t, updated)
		assert.Equal(t, schedID, updated.ID)
	})

	t.Run("other account returns ErrNotFound", func(t *testing.T) {
		_, err := scheduledSvc.UpdateScheduledStatus(ctx, schedID, bob.ID, newParams, newTime)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("past scheduled_at returns ErrValidation", func(t *testing.T) {
		_, err := scheduledSvc.UpdateScheduledStatus(ctx, schedID, alice.ID, newParams, time.Now().Add(-1*time.Hour))
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrValidation)
	})

	t.Run("nonexistent returns ErrNotFound", func(t *testing.T) {
		_, err := scheduledSvc.UpdateScheduledStatus(ctx, "01H0000000000000000000000", alice.ID, newParams, newTime)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestScheduledStatusService_DeleteScheduledStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)
	scheduledSvc := NewScheduledStatusService(fake, statusWriteSvc)

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

	schedID := uid.New()
	_, err = fake.CreateScheduledStatus(ctx, store.CreateScheduledStatusInput{
		ID:          schedID,
		AccountID:   alice.ID,
		Params:      []byte(`{"text":"delete me"}`),
		ScheduledAt: time.Now().Add(1 * time.Hour),
	})
	require.NoError(t, err)

	t.Run("other account returns ErrNotFound", func(t *testing.T) {
		err := scheduledSvc.DeleteScheduledStatus(ctx, schedID, bob.ID)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("owner deletes successfully", func(t *testing.T) {
		err := scheduledSvc.DeleteScheduledStatus(ctx, schedID, alice.ID)
		require.NoError(t, err)
		_, err = fake.GetScheduledStatusByID(ctx, schedID)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("nonexistent returns ErrNotFound", func(t *testing.T) {
		err := scheduledSvc.DeleteScheduledStatus(ctx, "01H0000000000000000000000", alice.ID)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestScheduledStatusService_PublishDueStatuses(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	conversationSvc := NewConversationService(fake, statusSvc)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, conversationSvc, "https://example.com", "example.com", 500)
	scheduledSvc := NewScheduledStatusService(fake, statusWriteSvc)

	acc, err := accountSvc.Register(ctx, RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
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

	published, err := scheduledSvc.PublishDueStatuses(ctx, 20)
	require.NoError(t, err)
	assert.Equal(t, 1, published)

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
