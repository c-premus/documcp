package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gorilla/sessions"

	"git.999.haus/chris/DocuMCP-go/internal/model"
)

// authUserFinder looks up a user by ID for session-based auth.
type authUserFinder interface {
	FindUserByID(ctx context.Context, id int64) (*model.User, error)
}

// AuthHandler handles session-based auth endpoints for the Vue SPA.
type AuthHandler struct {
	sessionStore sessions.Store
	userFinder   authUserFinder
	logger       *slog.Logger
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(store sessions.Store, finder authUserFinder, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{sessionStore: store, userFinder: finder, logger: logger}
}

// Me handles GET /api/auth/me — returns the currently authenticated user.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	session, err := h.sessionStore.Get(r, "documcp_session")
	if err != nil {
		errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	userID, ok := session.Values["user_id"].(int64)
	if !ok {
		errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	user, err := h.userFinder.FindUserByID(r.Context(), userID)
	if err != nil {
		h.logger.Error("finding user for auth/me", "user_id", userID, "error", err)
		errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"id":       user.ID,
		"email":    user.Email,
		"name":     user.Name,
		"is_admin": user.IsAdmin,
	})
}
