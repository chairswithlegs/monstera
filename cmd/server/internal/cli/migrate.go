package cli

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/chairswithlegs/monstera/internal/config"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Database migration commands",
}

var migrateUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Apply all pending migrations",
	RunE:  runMigrateUp,
}

var migrateDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Roll back the most recent migration",
	RunE:  runMigrateDown,
}

var migrateDownAllCmd = &cobra.Command{
	Use:   "down-all",
	Short: "Roll back all migrations",
	RunE:  runMigrateDownAll,
}

func init() {
	migrateCmd.AddCommand(migrateUpCmd, migrateDownCmd, migrateDownAllCmd)
}

func runMigrateUp(cmd *cobra.Command, _ []string) error {
	ctx := context.Background()
	connString := getDatabaseConnectionString()
	slog.InfoContext(ctx, "Applying database migrations")
	if err := store.RunUp(connString); err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}
	slog.InfoContext(ctx, "Database migrations applied")
	return nil
}

func runMigrateDown(cmd *cobra.Command, _ []string) error {
	ctx := context.Background()
	connString := getDatabaseConnectionString()
	slog.InfoContext(ctx, "Rolling back database migrations")
	if err := store.RunDown(connString); err != nil {
		return fmt.Errorf("migrate down: %w", err)
	}
	slog.InfoContext(ctx, "Database migrations rolled back")
	return nil
}

func runMigrateDownAll(cmd *cobra.Command, _ []string) error {
	ctx := context.Background()
	connString := getDatabaseConnectionString()
	slog.InfoContext(ctx, "Rolling back all database migrations")
	if err := store.RunDownAll(connString); err != nil {
		return fmt.Errorf("migrate down-all: %w", err)
	}
	slog.InfoContext(ctx, "Database migrations rolled back")
	return nil
}

func getDatabaseConnectionString() string {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	return store.DatabaseConnectionString(cfg, false)
}
