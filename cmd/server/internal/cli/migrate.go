package cli

import (
	"errors"
	"fmt"
	"os"

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
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return errors.New("DATABASE_URL is required")
	}
	if err := store.RunUp(url); err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

func runMigrateDown(cmd *cobra.Command, _ []string) error {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return errors.New("DATABASE_URL is required")
	}
	if err := store.RunDown(url); err != nil {
		return fmt.Errorf("migrate down: %w", err)
	}
	return nil
}

func runMigrateDownAll(cmd *cobra.Command, _ []string) error {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return errors.New("DATABASE_URL is required")
	}
	if err := store.RunDownAll(url); err != nil {
		return fmt.Errorf("migrate down-all: %w", err)
	}
	return nil
}
