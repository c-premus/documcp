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

// PoolHealthy reports whether the connection pool has live connections.
// This avoids calling pgxPool.Ping() which generates otelpgx traces
// (ping + pool.acquire spans) on every health check.
type PoolHealthy interface {
	IsHealthy() bool
}

// RedisPinger checks Redis connectivity.
type RedisPinger interface {
	Ping(ctx context.Context) error
}

// ReadinessHandler checks that critical dependencies are reachable.
type ReadinessHandler struct {
	version string
	db      PoolHealthy
	redis   RedisPinger
}

// NewReadinessHandler creates a ReadinessHandler with the given dependency checkers.
func NewReadinessHandler(version string, db PoolHealthy, redisPinger RedisPinger) *ReadinessHandler {
	return &ReadinessHandler{version: version, db: db, redis: redisPinger}
}

// ServeHTTP checks Postgres pool health and Redis connectivity,
// returning 200 if all services are healthy, 503 otherwise.
func (h *ReadinessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	services := make(map[string]string)
	allHealthy := true

	// Check Postgres via pool stats — avoids otelpgx ping/pool.acquire spans.
	if h.db != nil {
		if !h.db.IsHealthy() {
			services["postgres"] = "unhealthy"
			allHealthy = false
		} else {
			services["postgres"] = "healthy"
		}
	}

	// Check Redis via uninstrumented client — avoids redisotel spans.
	if h.redis != nil {
		if err := h.redis.Ping(r.Context()); err != nil {
			services["redis"] = "unhealthy"
			allHealthy = false
		} else {
			services["redis"] = "healthy"
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
