// Package api provides REST API handlers for documents and search.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// jsonResponse writes a JSON response with the given status code.
func jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("encoding JSON response", "error", err)
	}
}

// errorResponse writes a JSON error response.
func errorResponse(w http.ResponseWriter, status int, message string) {
	jsonResponse(w, status, map[string]any{
		"error":   http.StatusText(status),
		"message": message,
	})
}
