package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"git.999.haus/chris/DocuMCP-go/internal/repository"
)

// OAuthClientHandler handles REST API endpoints for OAuth client administration.
type OAuthClientHandler struct {
	repo   *repository.OAuthRepository
	logger *slog.Logger
}

// NewOAuthClientHandler creates a new OAuthClientHandler.
func NewOAuthClientHandler(
	repo *repository.OAuthRepository,
	logger *slog.Logger,
) *OAuthClientHandler {
	return &OAuthClientHandler{
		repo:   repo,
		logger: logger,
	}
}

// List handles GET /api/admin/oauth-clients -- list OAuth clients (stub).
func (h *OAuthClientHandler) List(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]any{
		"data": []any{},
		"meta": map[string]any{
			"total": 0,
		},
	})
}

// Show handles GET /api/admin/oauth-clients/{clientId} -- get a single OAuth client (stub).
func (h *OAuthClientHandler) Show(w http.ResponseWriter, r *http.Request) {
	clientID := chi.URLParam(r, "clientId")
	_ = clientID

	errorResponse(w, http.StatusNotImplemented, "OAuth client detail endpoint not yet implemented")
}
