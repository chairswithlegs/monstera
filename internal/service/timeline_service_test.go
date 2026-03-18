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

func TestTimelineService_HomeEnriched_empty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	timelineSvc := NewTimelineService(fake, accountSvc, statusSvc)

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)

	enriched, err := timelineSvc.HomeEnriched(ctx, acc.ID, nil, 10)
	require.NoError(t, err)
	assert.Empty(t, enriched)
}

func TestTimelineService_HomeEnriched_one_status(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)
	timelineSvc := NewTimelineService(fake, accountSvc, statusSvc)

	acc, err := accountSvc.Register(ctx, RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
	})
	require.NoError(t, err)

	text := "Hello world"
	_, err = statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  acc.ID,
		Text:       text,
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)

	enriched, err := timelineSvc.HomeEnriched(ctx, acc.ID, nil, 10)
	require.NoError(t, err)
	require.Len(t, enriched, 1)
	assert.Equal(t, "Hello world", *enriched[0].Status.Text)
	assert.Equal(t, acc.ID, enriched[0].Status.AccountID)
	require.NotNil(t, enriched[0].Author)
	assert.Equal(t, "alice", enriched[0].Author.Username)
	assert.Empty(t, enriched[0].Mentions)
	assert.Empty(t, enriched[0].Tags)
	assert.Empty(t, enriched[0].Media)
}

func TestTimelineService_ListTimelineEnriched_excludes_private_status_when_list_owner_does_not_follow_author(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)
	timelineSvc := NewTimelineService(fake, accountSvc, statusSvc)

	alice, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	bob, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	listID := uid.New()
	_, err = fake.CreateList(ctx, store.CreateListInput{
		ID:            listID,
		AccountID:     alice.ID,
		Title:         "My list",
		RepliesPolicy: "list",
		Exclusive:     false,
	})
	require.NoError(t, err)
	err = fake.AddAccountToList(ctx, listID, bob.ID)
	require.NoError(t, err)

	privText := "private post"
	_, err = statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  bob.ID,
		Text:       privText,
		Visibility: domain.VisibilityPrivate,
	})
	require.NoError(t, err)

	enriched, err := timelineSvc.ListTimelineEnriched(ctx, alice.ID, listID, nil, 10)
	require.NoError(t, err)
	assert.Empty(t, enriched, "list owner should not see private status from list member they do not follow")
}

func TestTimelineService_ListTimelineEnriched_includes_private_status_when_list_owner_follows_author(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	statusWriteSvc := NewStatusWriteService(fake, statusSvc, NewConversationService(fake, statusSvc), "https://example.com", "example.com", 500)
	timelineSvc := NewTimelineService(fake, accountSvc, statusSvc)

	alice, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	bob, err := accountSvc.Create(ctx, CreateAccountInput{Username: "bob"})
	require.NoError(t, err)

	listID := uid.New()
	_, err = fake.CreateList(ctx, store.CreateListInput{
		ID:            listID,
		AccountID:     alice.ID,
		Title:         "My list",
		RepliesPolicy: "list",
		Exclusive:     false,
	})
	require.NoError(t, err)
	err = fake.AddAccountToList(ctx, listID, bob.ID)
	require.NoError(t, err)

	_, err = fake.CreateFollow(ctx, store.CreateFollowInput{
		ID:        uid.New(),
		AccountID: alice.ID,
		TargetID:  bob.ID,
		State:     domain.FollowStateAccepted,
		APID:      nil,
	})
	require.NoError(t, err)

	privText := "private post"
	_, err = statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  bob.ID,
		Text:       privText,
		Visibility: domain.VisibilityPrivate,
	})
	require.NoError(t, err)

	enriched, err := timelineSvc.ListTimelineEnriched(ctx, alice.ID, listID, nil, 10)
	require.NoError(t, err)
	require.Len(t, enriched, 1)
	assert.Equal(t, "private post", *enriched[0].Status.Text)
	assert.Equal(t, domain.VisibilityPrivate, enriched[0].Status.Visibility)
	assert.Equal(t, bob.ID, enriched[0].Status.AccountID)
}
