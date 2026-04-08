package api

import (
	"log/slog"
	"net/http"

	authmiddleware "github.com/c-premus/documcp/internal/auth/middleware"
)

// AuthHandler handles auth status endpoints for the Vue SPA.
type AuthHandler struct {
	logger *slog.Logger
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(logger *slog.Logger) *AuthHandler {
	return &AuthHandler{logger: logger}
}

// Me handles GET /api/auth/me — returns the currently authenticated user.
// Relies on BearerOrSession middleware to populate the user in context.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := authmiddleware.UserFromContext(r.Context())
	if !ok {
		errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"id":       user.ID,
			"email":    user.Email,
			"name":     user.Name,
			"is_admin": user.IsAdmin,
		},
	})
}
