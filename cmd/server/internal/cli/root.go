package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "monstera",
	Short: "Monstera — Mastodon-compatible ActivityPub server",
}

func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		return fmt.Errorf("execute: %w", err)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(migrateCmd, serveCmd)
}
