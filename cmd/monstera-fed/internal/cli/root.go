package cli

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "monstera-fed",
	Short: "Monstera-fed — Mastodon-compatible ActivityPub server",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(migrateCmd, serveCmd)
}
