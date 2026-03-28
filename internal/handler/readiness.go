package handler

import (
	"context"
	"encoding/json"
	"net/http"
)

// ReadinessResponse is the JSON payload returned by the readiness endpoint.
type ReadinessResponse struct {
	Status   string            `json:"status"`
	Version  string            `json:"version"`
	Services map[string]string `json:"services"`
}

// searchHealthChecker checks if the search engine is reachable.
type searchHealthChecker interface {
	Healthy() bool
}

// DBPinger checks database connectivity.
type DBPinger interface {
	Ping(ctx context.Context) error
}

// ReadinessHandler checks that critical dependencies are reachable.
type ReadinessHandler struct {
	version string
	db      DBPinger
	search  searchHealthChecker
}

// NewReadinessHandler creates a ReadinessHandler with the given DB pinger and version.
func NewReadinessHandler(version string, db DBPinger, search searchHealthChecker) *ReadinessHandler {
	return &ReadinessHandler{version: version, db: db, search: search}
}

// ServeHTTP pings Postgres and returns 200 if all services are healthy, 503 otherwise.
func (h *ReadinessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	services := make(map[string]string)
	allHealthy := true

	// Check Postgres
	if h.db != nil {
		if err := h.db.Ping(r.Context()); err != nil {
			services["postgres"] = "unhealthy"
			allHealthy = false
		} else {
			services["postgres"] = "healthy"
		}
	}

	// Check Meilisearch
	if h.search != nil {
		if h.search.Healthy() {
			services["meilisearch"] = "healthy"
		} else {
			services["meilisearch"] = "unhealthy"
			allHealthy = false
		}
	}

	status := "ready"
	httpStatus := http.StatusOK
	if !allHealthy {
		status = "not_ready"
		httpStatus = http.StatusServiceUnavailable
	}

	resp := ReadinessResponse{
		Status:   status,
		Version:  h.version,
		Services: services,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)

	_ = json.NewEncoder(w).Encode(resp)
}
