package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"github.com/go-chi/chi/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/queue"
	"github.com/c-premus/documcp/internal/service"
)

// externalServiceReorderer reorders external services by priority.
type externalServiceReorderer interface {
	ReorderPriorities(ctx context.Context, serviceIDs []int64) error
}

// externalServiceJobInserter enqueues background jobs. Defined where consumed.
type externalServiceJobInserter interface {
	Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// kiwixCacheInvalidator invalidates the cached Kiwix client. Defined where consumed.
type kiwixCacheInvalidator interface {
	Invalidate()
}

// ExternalServiceHandler handles REST API endpoints for external services.
type ExternalServiceHandler struct {
	svc        *service.ExternalServiceService
	reorderer  externalServiceReorderer
	inserter   externalServiceJobInserter
	kiwixCache kiwixCacheInvalidator
	logger     *slog.Logger
}

// NewExternalServiceHandler creates a new ExternalServiceHandler.
func NewExternalServiceHandler(
	svc *service.ExternalServiceService,
	reorderer externalServiceReorderer,
	inserter externalServiceJobInserter,
	kiwixCache kiwixCacheInvalidator,
	logger *slog.Logger,
) *ExternalServiceHandler {
	return &ExternalServiceHandler{
		svc:        svc,
		reorderer:  reorderer,
		inserter:   inserter,
		kiwixCache: kiwixCache,
		logger:     logger,
	}
}

// externalServiceResponse is the JSON representation of an external service.
type externalServiceResponse struct {
	UUID                string `json:"uuid"`
	Name                string `json:"name"`
	Slug                string `json:"slug"`
	Type                string `json:"type"`
	BaseURL             string `json:"base_url"`
	Priority            int    `json:"priority"`
	Status              string `json:"status"`
	IsEnabled           bool   `json:"is_enabled"`
	IsEnvManaged        bool   `json:"is_env_managed"`
	ErrorCount          int    `json:"error_count"`
	ConsecutiveFailures int    `json:"consecutive_failures"`
	LastError           string `json:"last_error,omitempty"`
	LastErrorAt         string `json:"last_error_at,omitempty"`
	LastCheckAt         string `json:"last_check_at,omitempty"`
	LastLatencyMS       int64  `json:"last_latency_ms,omitempty"`
	CreatedAt           string `json:"created_at,omitempty"`
	UpdatedAt           string `json:"updated_at,omitempty"`
}

// List handles GET /api/external-services -- list external services with filters.
func (h *ExternalServiceHandler) List(w http.ResponseWriter, r *http.Request) {
	serviceType := r.URL.Query().Get("type")
	status := r.URL.Query().Get("status")

	limit, offset := parsePagination(r, 50, 100)

	services, total, err := h.svc.List(r.Context(), serviceType, status, limit, offset)
	if err != nil {
		h.logger.Error("listing external services", "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to list external services")
		return
	}

	items := make([]externalServiceResponse, 0, len(services))
	for i := range services {
		items = append(items, toExternalServiceResponse(&services[i]))
	}

	jsonResponse(w, http.StatusOK, listResponse(items, total, limit, offset))
}

// Show handles GET /api/external-services/{uuid} -- get a single external service.
func (h *ExternalServiceHandler) Show(w http.ResponseWriter, r *http.Request) {
	svcUUID := chi.URLParam(r, "uuid")

	svc, err := h.svc.FindByUUID(r.Context(), svcUUID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			errorResponse(w, http.StatusNotFound, "external service not found")
			return
		}
		h.logger.Error("finding external service", "uuid", svcUUID, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to find external service")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": toExternalServiceResponse(svc),
	})
}

// Create handles POST /api/external-services -- create a new external service.
func (h *ExternalServiceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		BaseURL  string `json:"base_url"`
		APIKey   string `json:"api_key"`
		Config   string `json:"config"`
		Priority int    `json:"priority"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if body.Name == "" {
		errorResponse(w, http.StatusBadRequest, "name is required")
		return
	}
	if body.Type == "" {
		errorResponse(w, http.StatusBadRequest, "type is required")
		return
	}
	if body.BaseURL == "" {
		errorResponse(w, http.StatusBadRequest, "base_url is required")
		return
	}

	created, err := h.svc.Create(r.Context(), service.CreateExternalServiceParams{
		Name:     body.Name,
		Type:     body.Type,
		BaseURL:  body.BaseURL,
		APIKey:   body.APIKey,
		Config:   body.Config,
		Priority: body.Priority,
	})
	if err != nil {
		h.logger.Error("creating external service", "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to create external service")
		return
	}

	if created.Type == "kiwix" {
		if h.kiwixCache != nil {
			h.kiwixCache.Invalidate()
		}
		if h.inserter != nil {
			if _, jobErr := h.inserter.Insert(r.Context(), queue.SyncKiwixArgs{}, nil); jobErr != nil {
				h.logger.Warn("failed to enqueue sync after external service create", "type", created.Type, "error", jobErr)
			}
		}
	}

	jsonResponse(w, http.StatusCreated, map[string]any{
		"data":    toExternalServiceResponse(created),
		"message": "External service created successfully.",
	})
}

// Update handles PUT /api/external-services/{uuid} -- partial update of an external service.
func (h *ExternalServiceHandler) Update(w http.ResponseWriter, r *http.Request) {
	svcUUID := chi.URLParam(r, "uuid")

	var body struct {
		Name      string `json:"name"`
		BaseURL   string `json:"base_url"`
		APIKey    string `json:"api_key"`
		Config    string `json:"config"`
		Priority  *int   `json:"priority"`
		IsEnabled *bool  `json:"is_enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	updated, err := h.svc.Update(r.Context(), svcUUID, service.UpdateExternalServiceParams{
		Name:      body.Name,
		BaseURL:   body.BaseURL,
		APIKey:    body.APIKey,
		Config:    body.Config,
		Priority:  body.Priority,
		IsEnabled: body.IsEnabled,
	})
	if err != nil {
		h.logger.Error("updating external service", "uuid", svcUUID, "error", err)
		if errors.Is(err, service.ErrNotFound) {
			errorResponse(w, http.StatusNotFound, "external service not found")
			return
		}
		errorResponse(w, http.StatusInternalServerError, "failed to update external service")
		return
	}

	if updated.Type == "kiwix" {
		if h.kiwixCache != nil {
			h.kiwixCache.Invalidate()
		}
		if h.inserter != nil {
			if _, jobErr := h.inserter.Insert(r.Context(), queue.SyncKiwixArgs{}, nil); jobErr != nil {
				h.logger.Warn("failed to enqueue sync after external service update", "type", updated.Type, "error", jobErr)
			}
		}
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data":    toExternalServiceResponse(updated),
		"message": "External service updated successfully.",
	})
}

// Delete handles DELETE /api/external-services/{uuid} -- delete an external service.
func (h *ExternalServiceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	svcUUID := chi.URLParam(r, "uuid")

	// Look up service type before deleting so we can invalidate cache if needed.
	existing, lookupErr := h.svc.FindByUUID(r.Context(), svcUUID)

	if err := h.svc.Delete(r.Context(), svcUUID); err != nil {
		h.logger.Error("deleting external service", "uuid", svcUUID, "error", err)
		if errors.Is(err, service.ErrNotFound) {
			errorResponse(w, http.StatusNotFound, "external service not found")
			return
		}
		if errors.Is(err, service.ErrEnvManaged) {
			errorResponse(w, http.StatusForbidden, "cannot delete environment-managed external service")
			return
		}
		errorResponse(w, http.StatusInternalServerError, "failed to delete external service")
		return
	}

	if lookupErr == nil && existing.Type == "kiwix" && h.kiwixCache != nil {
		h.kiwixCache.Invalidate()
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"message": "External service deleted successfully.",
	})
}

// HealthCheck handles POST /api/external-services/{uuid}/health -- trigger a health check.
// Health checks run automatically via the scheduler. This endpoint is reserved for future
// on-demand health checks once per-type client resolution is wired.
func (h *ExternalServiceHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	svcUUID := chi.URLParam(r, "uuid")

	svc, err := h.svc.FindByUUID(r.Context(), svcUUID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			errorResponse(w, http.StatusNotFound, "external service not found")
			return
		}
		h.logger.Error("finding external service for health check", "uuid", svcUUID, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to find external service")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data":    toExternalServiceResponse(svc),
		"message": "Health checks run automatically via the scheduler.",
	})
}

// Sync handles POST /api/external-services/{uuid}/sync -- trigger an on-demand sync.
func (h *ExternalServiceHandler) Sync(w http.ResponseWriter, r *http.Request) {
	svcUUID := chi.URLParam(r, "uuid")

	svc, err := h.svc.FindByUUID(r.Context(), svcUUID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			errorResponse(w, http.StatusNotFound, "external service not found")
			return
		}
		h.logger.Error("finding external service for sync", "uuid", svcUUID, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to find external service")
		return
	}

	if h.inserter == nil {
		errorResponse(w, http.StatusServiceUnavailable, "job queue not available")
		return
	}

	var jobErr error
	switch svc.Type {
	case "kiwix":
		_, jobErr = h.inserter.Insert(r.Context(), queue.SyncKiwixArgs{}, nil)
	default:
		errorResponse(w, http.StatusBadRequest, "sync not supported for service type: "+svc.Type)
		return
	}
	if jobErr != nil {
		h.logger.Error("failed to enqueue sync job", "uuid", svcUUID, "type", svc.Type, "error", jobErr)
		errorResponse(w, http.StatusInternalServerError, "failed to enqueue sync job")
		return
	}

	jsonResponse(w, http.StatusAccepted, map[string]any{
		"message": "Sync queued",
	})
}

// Reorder handles PUT /api/admin/external-services/reorder -- update priority ordering.
func (h *ExternalServiceHandler) Reorder(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ServiceIDs []int64 `json:"service_ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if len(body.ServiceIDs) == 0 {
		errorResponse(w, http.StatusBadRequest, "service_ids is required")
		return
	}

	if err := h.reorderer.ReorderPriorities(r.Context(), body.ServiceIDs); err != nil {
		h.logger.Error("reordering external services", "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to reorder external services")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"message": "External services reordered successfully.",
	})
}

// toExternalServiceResponse converts an ExternalService model to its JSON response DTO.
func toExternalServiceResponse(es *model.ExternalService) externalServiceResponse {
	resp := externalServiceResponse{
		UUID:                es.UUID,
		Name:                es.Name,
		Slug:                es.Slug,
		Type:                es.Type,
		BaseURL:             es.BaseURL,
		Priority:            es.Priority,
		Status:              es.Status,
		IsEnabled:           es.IsEnabled,
		IsEnvManaged:        es.IsEnvManaged,
		ErrorCount:          es.ErrorCount,
		ConsecutiveFailures: es.ConsecutiveFailures,
	}

	resp.LastError = nullStringValue(es.LastError)
	resp.LastErrorAt = nullTimeToString(es.LastErrorAt)
	resp.LastCheckAt = nullTimeToString(es.LastCheckAt)
	resp.LastLatencyMS = nullInt64Value(es.LastLatencyMS)
	resp.CreatedAt = nullTimeToString(es.CreatedAt)
	resp.UpdatedAt = nullTimeToString(es.UpdatedAt)

	return resp
}
