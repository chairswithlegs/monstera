package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/chairswithlegs/monstera/internal/config"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/store/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"
)

var userCmd = &cobra.Command{
	Use:   "user",
	Short: "User management commands",
}

var userCreateCmd = &cobra.Command{
	Use:       "create",
	Short:     "Create a new user",
	RunE:      runUserCreate,
	Args:      cobra.ExactArgs(5),
	ValidArgs: []string{"username", "email", "password", "display_name", "role"},
}

func init() {
	userCmd.AddCommand(userCreateCmd)
}

func runUserCreate(cmd *cobra.Command, args []string) error {
	username := args[0]
	email := args[1]
	password := args[2]
	displayName := args[3]
	role := args[4]

	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	connString := store.DatabaseConnectionString(cfg, false)
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("database ping: %w", err)
	}

	s := postgres.New(pool)
	instanceBaseURL := "https://" + cfg.MonsteraInstanceDomain
	accountSvc := service.NewAccountService(s, instanceBaseURL)
	registrationSvc := service.NewRegistrationService(s, nil, nil, instanceBaseURL, nil)

	// Validate that the user doesn't already exist
	acc, err := accountSvc.GetByUsername(ctx, username, nil)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return fmt.Errorf("get account: %w", err)
	}

	if acc != nil {
		return errors.New("user already exists")
	}

	created, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:    username,
		Email:       email,
		Password:    password,
		DisplayName: &displayName,
		Role:        role,
	})
	if err != nil {
		return fmt.Errorf("register: %w", err)
	}

	// Mark the user as confirmed
	_, user, err := accountSvc.GetAccountWithUser(ctx, created.ID)
	if err != nil {
		return fmt.Errorf("get user after register: %w", err)
	}

	if err := registrationSvc.Confirm(ctx, user.ID); err != nil {
		return fmt.Errorf("confirm user: %w", err)
	}

	slog.InfoContext(ctx, "created user", slog.String("username", username), slog.String("email", email), slog.String("role", role))
	return nil
}
