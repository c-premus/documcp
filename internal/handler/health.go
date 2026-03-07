package handler

import (
	"encoding/json"
	"net/http"
)

// HealthResponse is the JSON payload returned by the health endpoint.
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// HealthHandler serves the health check endpoint.
type HealthHandler struct {
	version string
}

// NewHealthHandler creates a HealthHandler with the given application version.
func NewHealthHandler(version string) *HealthHandler {
	return &HealthHandler{version: version}
}

// ServeHTTP writes a JSON health response with status 200.
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	resp := HealthResponse{
		Status:  "ok",
		Version: h.version,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(resp)
}
