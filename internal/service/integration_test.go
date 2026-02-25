//go:build integration

package service

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/store"
	"github.com/chairswithlegs/monstera-fed/internal/store/postgres"
	"github.com/stretchr/testify/require"
)

func TestIntegration_RegisterUser_CreateStatus_HomeTimeline(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	require.NotEmpty(t, url, "DATABASE_URL must be set for integration test")
	ctx := context.Background()
	require.NoError(t, store.RunUp(url))
	t.Cleanup(func() {
		_ = store.RunDownAll(url)
	})

	pool, err := pgxpool.New(ctx, url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	s := postgres.New(pool)
	instanceBaseURL := "https://test.example.com"
	accountSvc := NewAccountService(s, instanceBaseURL)
	statusSvc := NewStatusService(s, instanceBaseURL, "test.example.com", 500)
	timelineSvc := NewTimelineService(s)

	acc, err := accountSvc.Register(ctx, RegisterInput{
		Username:     "integration_user",
		Email:        "integration@test.example.com",
		PasswordHash: "hashedpassword",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	require.NotNil(t, acc)

	text := "Hello from integration test"
	st, err := statusSvc.Create(ctx, CreateStatusInput{
		AccountID:  acc.ID,
		Text:       &text,
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)
	require.NotNil(t, st)

	home, err := timelineSvc.Home(ctx, acc.ID, nil, 10)
	require.NoError(t, err)
	require.NotEmpty(t, home, "home timeline should contain the created status")
	var found bool
	for i := range home {
		if home[i].ID == st.ID {
			found = true
			require.Equal(t, text, *home[i].Text)
			require.Equal(t, acc.ID, home[i].AccountID)
			break
		}
	}
	require.True(t, found, "created status should appear in home timeline")
}
