//go:build integration

package service

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/chairswithlegs/monstera/internal/config"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/store/postgres"
	"github.com/stretchr/testify/require"
)

func TestIntegration_RegisterUser_CreateStatus_HomeTimeline(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	connString := store.DatabaseConnectionString(cfg, false)
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, connString)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	s := postgres.New(pool)
	instanceBaseURL := "https://test.example.com"
	accountSvc := NewAccountService(s, instanceBaseURL)
	statusSvc := NewStatusService(s, instanceBaseURL, "test.example.com", 500)
	timelineSvc := NewTimelineService(s, accountSvc, statusSvc)
	statusWriteSvc := NewStatusWriteService(s, statusSvc, NewConversationService(s, statusSvc), instanceBaseURL, "test.example.com", 500)

	acc, err := accountSvc.Register(ctx, RegisterInput{
		Username:     "integration_user",
		Email:        "integration@test.example.com",
		PasswordHash: "hashedpassword",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	require.NotNil(t, acc)

	text := "Hello from integration test"
	st, err := statusWriteSvc.Create(ctx, CreateStatusInput{
		AccountID:  acc.ID,
		Text:       text,
		Visibility: domain.VisibilityPublic,
	})
	require.NoError(t, err)
	require.NotNil(t, st)

	home, err := timelineSvc.Home(ctx, acc.ID, nil, 10)
	require.NoError(t, err)
	require.NotEmpty(t, home, "home timeline should contain the created status")
	var found bool
	for i := range home {
		if home[i].ID == st.Status.ID {
			found = true
			require.Equal(t, text, *home[i].Text)
			require.Equal(t, acc.ID, home[i].AccountID)
			break
		}
	}
	require.True(t, found, "created status should appear in home timeline")
}
