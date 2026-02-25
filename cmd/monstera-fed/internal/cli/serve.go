package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the API server (stub for Stage 2)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		fmt.Println("serve: not implemented in Stage 2")
		return nil
	},
}
