package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
)

var (
	setupDBURL    string
	setupUsername string
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Provision an OAuth app and access token for load testing",
	RunE:  runSetup,
}

func init() {
	setupCmd.Flags().StringVar(&setupDBURL, "db-url", "", "PostgreSQL connection URL (required)")
	setupCmd.Flags().StringVar(&setupUsername, "username", "alice", "local account username to bind the token to")
	_ = setupCmd.MarkFlagRequired("db-url")
}

func runSetup(cmd *cobra.Command, _ []string) error {
	ctx := context.Background()

	seeder, err := NewFanoutSeeder(ctx, setupDBURL)
	if err != nil {
		return fmt.Errorf("setup: connect: %w", err)
	}

	acc, err := seeder.GetLocalAccount(ctx, setupUsername)
	if err != nil {
		return fmt.Errorf("setup: lookup account: %w", err)
	}

	appID := uid.New()
	clientID := uid.New()
	clientSecret := uid.New()
	app, err := seeder.s.CreateApplication(ctx, store.CreateApplicationInput{
		ID:           appID,
		Name:         "loadtest",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURIs: "urn:ietf:wg:oauth:2.0:oob",
		Scopes:       "read write",
	})
	if err != nil {
		return fmt.Errorf("setup: create application: %w", err)
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Errorf("setup: generate token bytes: %w", err)
	}
	token := hex.EncodeToString(raw)

	accID := acc.ID
	_, err = seeder.s.CreateAccessToken(ctx, store.CreateAccessTokenInput{
		ID:            uid.New(),
		ApplicationID: app.ID,
		AccountID:     &accID,
		Token:         token,
		Scopes:        "read write",
		ExpiresAt:     nil,
	})
	if err != nil {
		return fmt.Errorf("setup: create access token: %w", err)
	}

	fmt.Println(token)
	return nil
}
