package api

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"git.999.haus/chris/DocuMCP-go/internal/queue"
)

// QueueHandler provides REST API for queue management.
type QueueHandler struct {
	riverClient *queue.RiverClient
}

// NewQueueHandler creates a QueueHandler.
func NewQueueHandler(riverClient *queue.RiverClient) *QueueHandler {
	return &QueueHandler{riverClient: riverClient}
}

// Stats handles GET /api/admin/queue/stats — queue depth and configuration.
func (h *QueueHandler) Stats(w http.ResponseWriter, r *http.Request) {
	counts, err := h.riverClient.QueueStats(r.Context())
	if err != nil {
		slog.Error("queue stats query failed", "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to fetch queue stats")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]int{
		"available": counts["available"],
		"running":   counts["running"],
		"retryable": counts["retryable"],
		"discarded": counts["discarded"],
		"cancelled": counts["cancelled"],
	})
}

// ListFailed handles GET /api/admin/queue/failed — paginated list of failed jobs.
func (h *QueueHandler) ListFailed(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	params := river.NewJobListParams().
		States(rivertype.JobStateDiscarded, rivertype.JobStateCancelled, rivertype.JobStateRetryable).
		First(limit)

	result, err := h.riverClient.Client().JobList(ctx, params)
	if err != nil {
		slog.Error("queue job list failed", "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to list queue jobs")
		return
	}

	type failedJob struct {
		ID          int64                   `json:"id"`
		Kind        string                  `json:"kind"`
		Queue       string                  `json:"queue"`
		State       rivertype.JobState      `json:"state"`
		Attempt     int                     `json:"attempt"`
		MaxAttempts int                     `json:"max_attempts"`
		CreatedAt   string                  `json:"created_at"`
		Errors      []rivertype.AttemptError `json:"errors,omitempty"`
	}

	jobs := make([]failedJob, 0, len(result.Jobs))
	for _, j := range result.Jobs {
		jobs = append(jobs, failedJob{
			ID:          j.ID,
			Kind:        j.Kind,
			Queue:       j.Queue,
			State:       j.State,
			Attempt:     j.Attempt,
			MaxAttempts: j.MaxAttempts,
			CreatedAt:   j.CreatedAt.Format("2006-01-02T15:04:05Z"),
			Errors:      j.Errors,
		})
	}

	jsonResponse(w, http.StatusOK, map[string]any{"jobs": jobs, "count": len(jobs)})
}

// RetryFailed handles POST /api/admin/queue/failed/{id}/retry — retries a specific job.
func (h *QueueHandler) RetryFailed(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid job id")
		return
	}

	job, err := h.riverClient.Client().JobRetry(r.Context(), id)
	if err != nil {
		slog.Error("queue job retry failed", "error", err, "job_id", id)
		errorResponse(w, http.StatusInternalServerError, "failed to retry job")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{"id": job.ID, "state": job.State})
}

// DeleteFailed handles DELETE /api/admin/queue/failed/{id} — cancels a failed job.
func (h *QueueHandler) DeleteFailed(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid job id")
		return
	}

	job, err := h.riverClient.Client().JobCancel(r.Context(), id)
	if err != nil {
		slog.Error("queue job cancel failed", "error", err, "job_id", id)
		errorResponse(w, http.StatusInternalServerError, "failed to cancel job")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{"id": job.ID, "state": job.State})
}
