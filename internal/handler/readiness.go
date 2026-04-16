package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// ReadinessResponse is the JSON payload returned by the readiness endpoint.
type ReadinessResponse struct {
	Status   string            `json:"status"`
	Version  string            `json:"version"`
	Services map[string]string `json:"services"`
}

// DependencyPinger verifies that a dependency is reachable.
// Postgres and Redis both implement this shape (pgxpool.Pool.Ping,
// redis.Client.Ping(ctx).Err). The readiness probe should target
// uninstrumented clients so the probe path does not emit trace spans.
type DependencyPinger interface {
	Ping(ctx context.Context) error
}

// ReadinessHandler checks that critical dependencies respond to Ping.
// Nil dependencies are treated as absent (no check performed).
type ReadinessHandler struct {
	version string
	db      DependencyPinger
	redis   DependencyPinger
}

// NewReadinessHandler creates a ReadinessHandler wired with the given pingers.
func NewReadinessHandler(version string, db, redis DependencyPinger) *ReadinessHandler {
	return &ReadinessHandler{version: version, db: db, redis: redis}
}

// readinessProbeTimeout caps the total time spent probing dependencies.
// Matches the readiness gauge scrape budget so the HTTP endpoint and the
// Prometheus collector see consistent results.
const readinessProbeTimeout = 2 * time.Second

// Check runs Ping on each configured dependency with a bounded timeout.
// Returns per-service status and true when every configured dependency
// responded without error.
func (h *ReadinessHandler) Check(ctx context.Context) (services map[string]string, ready bool) {
	ctx, cancel := context.WithTimeout(ctx, readinessProbeTimeout)
	defer cancel()

	services = make(map[string]string, 2)
	ready = true

	if h.db != nil {
		if err := h.db.Ping(ctx); err != nil {
			services["postgres"] = "unhealthy"
			ready = false
		} else {
			services["postgres"] = "healthy"
		}
	}

	if h.redis != nil {
		if err := h.redis.Ping(ctx); err != nil {
			services["redis"] = "unhealthy"
			ready = false
		} else {
			services["redis"] = "healthy"
		}
	}

	return services, ready
}

// ServeHTTP runs the readiness check and writes a JSON response. Returns
// 200 when every configured dependency responded to Ping, 503 otherwise.
func (h *ReadinessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	services, ready := h.Check(r.Context())

	status := "ready"
	httpStatus := http.StatusOK
	if !ready {
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
