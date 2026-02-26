package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/chairswithlegs/monstera-fed/internal/config"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/observability"
	"github.com/chairswithlegs/monstera-fed/internal/service"
	"github.com/chairswithlegs/monstera-fed/internal/store"
	"github.com/chairswithlegs/monstera-fed/internal/store/postgres"
)

type seedUser struct {
	Username    string
	Email       string
	Password    string
	DisplayName string
	Role        string
}

var seedUsers = []seedUser{
	{Username: "admin", Email: "admin@example.com", Password: "password", DisplayName: "Admin", Role: domain.RoleAdmin},
	{Username: "alice", Email: "alice@example.com", Password: "password", DisplayName: "Alice", Role: domain.RoleUser},
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "seed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	logger := observability.NewLogger(cfg.AppEnv, cfg.LogLevel)
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("database ping: %w", err)
	}

	if err := store.RunUp(cfg.DatabaseURL); err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}

	s := postgres.New(pool)
	instanceBaseURL := "https://" + cfg.InstanceDomain
	accountSvc := service.NewAccountService(s, instanceBaseURL)

	for _, u := range seedUsers {
		if err := runSeedUser(ctx, logger, s, accountSvc, u); err != nil {
			return fmt.Errorf("seed user %s: %w", u.Username, err)
		}
	}

	logger.Info("seed complete")
	return nil
}

func runSeedUser(ctx context.Context, logger *slog.Logger, s store.Store, accountSvc *service.AccountService, u seedUser) error {
	acc, err := s.GetLocalAccountByUsername(ctx, u.Username)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return fmt.Errorf("get account: %w", err)
	}
	if acc != nil {
		user, err := s.GetUserByAccountID(ctx, acc.ID)
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}
		if user.ConfirmedAt == nil {
			if err := s.ConfirmUser(ctx, user.ID); err != nil {
				return fmt.Errorf("confirm user: %w", err)
			}
			logger.Info("user already existed, confirmed", slog.String("username", u.Username))
		} else {
			logger.Info("user already exists", slog.String("username", u.Username))
		}
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("bcrypt: %w", err)
	}
	displayName := u.DisplayName
	created, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     u.Username,
		Email:        u.Email,
		PasswordHash: string(hash),
		DisplayName:  &displayName,
		Role:         u.Role,
	})
	if err != nil {
		return fmt.Errorf("register: %w", err)
	}
	user, err := s.GetUserByAccountID(ctx, created.ID)
	if err != nil {
		return fmt.Errorf("get user after register: %w", err)
	}
	if err := s.ConfirmUser(ctx, user.ID); err != nil {
		return fmt.Errorf("confirm user: %w", err)
	}
	logger.Info("created user", slog.String("username", u.Username), slog.String("email", u.Email))
	return nil
}
