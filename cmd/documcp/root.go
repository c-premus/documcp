package main

import (
	"github.com/spf13/cobra"

	"github.com/c-premus/documcp/internal/config"
)

var (
	cfgFile string
	cfg     *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "documcp",
	Short: "DocuMCP — documentation server with MCP, OAuth 2.1, and full-text search",
	Long: `DocuMCP exposes documentation and knowledge bases through the Model Context
Protocol (MCP), enabling AI agents to search, read, and manage documentation.
It also provides a REST API and web-based admin panel.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		// Skip config loading for commands that don't need it.
		if cmd.Name() == "version" || cmd.Name() == "health" {
			return nil
		}

		var err error
		cfg, err = config.Load()
		if err != nil {
			return err
		}

		// Override version from build ldflags if set.
		if version != "dev" {
			cfg.DocuMCP.ServerVersion = version

			// Fall back OTEL service version to build version if not explicitly set.
			if cfg.OTEL.Version == "" {
				cfg.OTEL.Version = version
			}
		}

		return cfg.Validate()
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path (optional)")
}
