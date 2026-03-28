package main

import (
	"github.com/spf13/cobra"

	"github.com/c-premus/documcp/internal/app"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations and exit",
	Long: `Run all pending database migrations (goose) and River schema migrations,
then exit. Useful for CI/CD pipelines and init containers.

Examples:
  documcp migrate    # Apply all pending migrations`,
	RunE: func(_ *cobra.Command, _ []string) error {
		return app.RunMigrationsOnly(cfg)
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}
