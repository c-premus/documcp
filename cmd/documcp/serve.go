package main

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/c-premus/documcp/internal/app"
)

var withWorker bool

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the HTTP server and MCP endpoint",
	Long: `Start the HTTP server, REST API, and MCP endpoint.

By default, the server runs in insert-only mode: it enqueues jobs into the
River queue but does not process them. Use --with-worker to also run queue
workers in the same process (combined mode).

Examples:
  documcp serve                  # HTTP server only (scale behind a load balancer)
  documcp serve --with-worker    # HTTP server + queue workers (single-process mode)`,
	RunE: func(_ *cobra.Command, _ []string) error {
		foundation, err := app.NewFoundation(cfg)
		if err != nil {
			return err
		}
		defer foundation.Close()

		serverApp, err := app.NewServerApp(foundation, withWorker)
		if err != nil {
			return err
		}
		defer func() {
			if closeErr := serverApp.Close(); closeErr != nil {
				foundation.Logger.Error("closing server app", "error", closeErr)
			}
		}()

		foundation.Logger.Info("DocuMCP starting",
			"version", cfg.DocuMCP.ServerVersion,
			"build_time", buildTime,
			"env", cfg.App.Env,
			"mode", serverMode(),
		)

		if !withWorker {
			foundation.Logger.Warn(
				"serve is running without --with-worker; periodic jobs require at least one worker replica in the fleet",
				"periodic_jobs", "oauth_token_cleanup,orphan_file_cleanup,soft_delete_purge,external_service_sync,scope_grant_cleanup,stuck_document_recovery",
				"monitor", "documcp_river_leader_active Prometheus gauge",
			)
		}

		return serverApp.Start(context.Background())
	},
}

func init() {
	serveCmd.Flags().BoolVar(&withWorker, "with-worker", false, "also run queue workers in the same process")
	rootCmd.AddCommand(serveCmd)
}

func serverMode() string {
	if withWorker {
		return "combined"
	}
	return "serve-only"
}
