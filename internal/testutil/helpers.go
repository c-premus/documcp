package testutil

import "log/slog"

// DiscardLogger returns a logger that discards all output.
// Use in tests to suppress log noise.
func DiscardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}
