package service

import (
	"context"
	"testing"

	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimelineService_Home(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	timelineSvc := NewTimelineService(fake)

	acc, err := accountSvc.Register(ctx, RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
	})
	require.NoError(t, err)

	text := "My first post"
	_, err = statusSvc.Create(ctx, CreateStatusInput{
		AccountID:  acc.ID,
		Text:       &text,
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)

	home, err := timelineSvc.Home(ctx, acc.ID, nil, 10)
	require.NoError(t, err)
	require.Len(t, home, 1)
	assert.Equal(t, "My first post", *home[0].Text)
	assert.Equal(t, acc.ID, home[0].AccountID)
}

func TestTimelineService_Home_empty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	timelineSvc := NewTimelineService(fake)

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)

	home, err := timelineSvc.Home(ctx, acc.ID, nil, 10)
	require.NoError(t, err)
	assert.Empty(t, home)
}

func TestTimelineService_Home_respects_limit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	timelineSvc := NewTimelineService(fake)

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		text := "post"
		_, err = statusSvc.Create(ctx, CreateStatusInput{
			AccountID:  acc.ID,
			Text:       &text,
			Visibility: domain.VisibilityPublic,
		})
		require.NoError(t, err)
	}

	home, err := timelineSvc.Home(ctx, acc.ID, nil, 2)
	require.NoError(t, err)
	assert.Len(t, home, 2)
}

func TestTimelineService_PublicLocal(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	statusSvc := NewStatusService(fake, "https://example.com", "example.com", 500)
	timelineSvc := NewTimelineService(fake)

	acc, err := accountSvc.Create(ctx, CreateAccountInput{Username: "alice"})
	require.NoError(t, err)
	text := "Public post"
	_, err = statusSvc.Create(ctx, CreateStatusInput{
		AccountID:  acc.ID,
		Text:       &text,
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)

	pub, err := timelineSvc.PublicLocal(ctx, true, nil, 10)
	require.NoError(t, err)
	require.Len(t, pub, 1)
	assert.Equal(t, "Public post", *pub[0].Text)
}

func TestTimelineService_PublicLocal_default_limit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	timelineSvc := NewTimelineService(fake)

	pub, err := timelineSvc.PublicLocal(ctx, false, nil, 0)
	require.NoError(t, err)
	assert.Empty(t, pub)
}

func TestTimelineService_HomeEnriched_empty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	accountSvc := NewAccountService(fake, "https://example.com")
	timelineSvc := NewTimelineService(fake)

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
	timelineSvc := NewTimelineService(fake)

	acc, err := accountSvc.Register(ctx, RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
	})
	require.NoError(t, err)

	text := "Hello world"
	_, err = statusSvc.Create(ctx, CreateStatusInput{
		AccountID:  acc.ID,
		Text:       &text,
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
