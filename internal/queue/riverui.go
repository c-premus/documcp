package queue

import (
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"riverqueue.com/riverui"
)

// NewRiverUIHandler creates a River UI HTTP handler for the admin dashboard.
// The prefix must match the path where the handler is mounted in the router
// (e.g., "/admin/river").
func NewRiverUIHandler(client *river.Client[pgx.Tx], logger *slog.Logger, prefix string) (*riverui.Handler, error) {
	return riverui.NewHandler(&riverui.HandlerOpts{
		Endpoints: riverui.NewEndpoints(client, nil),
		Logger:    logger,
		Prefix:    prefix,
	})
}
