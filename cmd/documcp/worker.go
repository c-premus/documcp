package main

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/c-premus/documcp/internal/app"
)

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Run queue workers and periodic jobs",
	Long: `Start River queue workers to process background jobs (document extraction,
indexing, sync, cleanup) and periodic scheduled tasks.

A minimal health HTTP endpoint is exposed for Kubernetes liveness and readiness
probes (default port 9090, configurable via WORKER_HEALTH_PORT).

Examples:
  documcp worker                           # Process all queues
  QUEUE_HIGH_WORKERS=20 documcp worker     # Scale up high-priority workers`,
	RunE: func(_ *cobra.Command, _ []string) error {
		foundation, err := app.NewFoundation(cfg)
		if err != nil {
			return err
		}
		defer foundation.Close()

		workerApp, err := app.NewWorkerApp(foundation)
		if err != nil {
			return err
		}
		defer func() {
			if closeErr := workerApp.Close(); closeErr != nil {
				foundation.Logger.Error("closing worker app", "error", closeErr)
			}
		}()

		foundation.Logger.Info("DocuMCP worker starting",
			"version", cfg.DocuMCP.ServerVersion,
			"build_time", buildTime,
			"env", cfg.App.Env,
			"high_workers", cfg.Queue.HighWorkers,
			"default_workers", cfg.Queue.DefaultWorkers,
			"low_workers", cfg.Queue.LowWorkers,
		)

		return workerApp.Start(context.Background())
	},
}

func init() {
	rootCmd.AddCommand(workerCmd)
}
