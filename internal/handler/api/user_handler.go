package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"git.999.haus/chris/DocuMCP-go/internal/repository"
)

// UserHandler handles REST API endpoints for user administration.
type UserHandler struct {
	repo   *repository.OAuthRepository
	logger *slog.Logger
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(
	repo *repository.OAuthRepository,
	logger *slog.Logger,
) *UserHandler {
	return &UserHandler{
		repo:   repo,
		logger: logger,
	}
}

// List handles GET /api/admin/users -- list users (stub).
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]any{
		"data": []any{},
		"meta": map[string]any{
			"total": 0,
		},
	})
}

// Show handles GET /api/admin/users/{id} -- get a single user (stub).
func (h *UserHandler) Show(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	_ = userID

	errorResponse(w, http.StatusNotImplemented, "user detail endpoint not yet implemented")
}
