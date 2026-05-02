package main

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

var healthPort int

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check server readiness (for Docker healthchecks)",
	Long: `Makes an HTTP GET request to the /health/ready endpoint and exits 0 if the
server responds with 200 OK, or 1 otherwise. Designed for use in Docker
healthchecks:

  HEALTHCHECK CMD ["/documcp", "health"]

The default port (8080) targets serve-mode. For worker-only containers, pass
--port to match WORKER_HEALTH_PORT (default 9090):

  HEALTHCHECK CMD ["/documcp", "health", "--port", "9090"]`,
	RunE: func(_ *cobra.Command, _ []string) error {
		url := fmt.Sprintf("http://localhost:%d/health/ready", healthPort)

		resp, err := http.Get(url) //nolint:gosec,noctx // localhost-only, no user input
		if err != nil {
			fmt.Fprintf(os.Stderr, "health check failed: %v\n", err)
			os.Exit(1)
		}
		defer func() { _ = resp.Body.Close() }()

		body, _ := io.ReadAll(resp.Body)
		fmt.Print(string(body))

		if resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		return nil
	},
}

func init() {
	healthCmd.Flags().IntVar(&healthPort, "port", 8080, "port of the health endpoint")
	rootCmd.AddCommand(healthCmd)
}
