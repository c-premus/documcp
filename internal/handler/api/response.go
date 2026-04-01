// Package api provides REST API handlers for documents and search.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// jsonResponse writes a JSON response with the given status code.
// Note: the error path uses the global slog logger because this function has
// no context.Context parameter. This is acceptable — encoding failures are
// extremely rare (only on broken io.Writer or unmarshalable types), and
// threading context through every call site is not worth the churn.
func jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Warn("failed to encode JSON response", "error", err)
	}
}

// errorResponse writes a JSON error response.
func errorResponse(w http.ResponseWriter, status int, message string) {
	jsonResponse(w, status, map[string]any{
		"error":   http.StatusText(status),
		"message": message,
	})
}
