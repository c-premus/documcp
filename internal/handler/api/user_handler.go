package api

import (
	"context"
	"log/slog"
	"net/http"

	authmiddleware "github.com/c-premus/documcp/internal/auth/middleware"
	"github.com/c-premus/documcp/internal/model"
)

// userRepo defines the methods used by UserHandler — defined where consumed.
//
// Create/Update are intentionally absent. DocuMCP is OIDC-only: user rows are
// created on first OIDC login, profile fields (name/email) sync from claims
// on every login, and IsAdmin derives from OIDC_ADMIN_GROUPS or the one-time
// OIDC_BOOTSTRAP_ADMIN_EMAIL. Admin-side user management is limited to
// listing, viewing, toggling admin, and deleting — none of which can produce
// a new authn-linked record. This was a deliberate v0.21.0 hardening: the
// previous POST/PUT endpoints were the admin-takeover vector in security.md
// finding H1.
type userRepo interface {
	ListUsers(ctx context.Context, query string, limit, offset int) ([]model.User, int, error)
	FindUserByID(ctx context.Context, id int64) (*model.User, error)
	DeleteUser(ctx context.Context, id int64) error
	ToggleAdmin(ctx context.Context, id int64) error
}

// userResponse is a clean JSON representation of a User,
// mapping sql.NullString/sql.NullTime to plain values.
type userResponse struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	Email           string `json:"email"`
	OIDCSub         string `json:"oidc_sub"`
	OIDCProvider    string `json:"oidc_provider"`
	EmailVerifiedAt string `json:"email_verified_at"`
	IsAdmin         bool   `json:"is_admin"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

func newUserResponse(u *model.User) userResponse {
	return userResponse{
		ID:              u.ID,
		Name:            u.Name,
		Email:           u.Email,
		IsAdmin:         u.IsAdmin,
		OIDCSub:         nullStringValue(u.OIDCSub),
		OIDCProvider:    nullStringValue(u.OIDCProvider),
		EmailVerifiedAt: nullTimeToString(u.EmailVerifiedAt),
		CreatedAt:       nullTimeToString(u.CreatedAt),
		UpdatedAt:       nullTimeToString(u.UpdatedAt),
	}
}

func newUserResponseList(users []model.User) []userResponse {
	out := make([]userResponse, len(users))
	for i := range users {
		out[i] = newUserResponse(&users[i])
	}
	return out
}

// UserHandler handles REST API endpoints for user administration.
type UserHandler struct {
	repo   userRepo
	logger *slog.Logger
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(
	repo userRepo,
	logger *slog.Logger,
) *UserHandler {
	return &UserHandler{
		repo:   repo,
		logger: logger,
	}
}

// List handles GET /api/admin/users — list users with optional search, limit, offset.
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")

	limit, offset := parsePagination(r, 20, 100)

	users, total, err := h.repo.ListUsers(r.Context(), q, limit, offset)
	if err != nil {
		h.logger.Error("listing users", "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to list users")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": newUserResponseList(users),
		"meta": map[string]any{
			"total": total,
		},
	})
}

// Show handles GET /api/admin/users/{id} — get a single user by ID.
func (h *UserHandler) Show(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseID(w, r)
	if !ok {
		return
	}

	user, err := h.repo.FindUserByID(r.Context(), id)
	if err != nil {
		h.logger.Error("finding user", "id", id, "error", err)
		errorResponse(w, http.StatusNotFound, "user not found")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": newUserResponse(user),
	})
}

// Delete handles DELETE /api/admin/users/{id} — hard-delete a user.
func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseID(w, r)
	if !ok {
		return
	}

	// Prevent admin from deleting their own account.
	if currentUser, ok := authmiddleware.UserFromContext(r.Context()); ok && currentUser.ID == id {
		errorResponse(w, http.StatusBadRequest, "cannot delete your own account")
		return
	}

	if err := h.repo.DeleteUser(r.Context(), id); err != nil {
		h.logger.Error("deleting user", "id", id, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to delete user")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"message": "user deleted",
	})
}

// ToggleAdmin handles POST /api/admin/users/{id}/toggle-admin — toggle admin flag.
//
// When OIDC_ADMIN_GROUPS is configured, this toggle is effectively read-only:
// group membership re-syncs IsAdmin on every login. The toggle is useful in
// bootstrap-email mode (no group claim support at the IdP) for promoting or
// demoting users after the initial admin is provisioned.
func (h *UserHandler) ToggleAdmin(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseID(w, r)
	if !ok {
		return
	}

	// Prevent admin from demoting themselves.
	if currentUser, ok := authmiddleware.UserFromContext(r.Context()); ok && currentUser.ID == id {
		errorResponse(w, http.StatusBadRequest, "cannot change your own admin status")
		return
	}

	if err := h.repo.ToggleAdmin(r.Context(), id); err != nil {
		h.logger.Error("toggling admin", "id", id, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to toggle admin")
		return
	}

	user, err := h.repo.FindUserByID(r.Context(), id)
	if err != nil {
		h.logger.Error("finding user after toggle", "id", id, "error", err)
		errorResponse(w, http.StatusNotFound, "user not found")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": newUserResponse(user),
	})
}

// parseID extracts and validates the {id} URL parameter.
// Returns the parsed ID and true on success, or writes an error response and returns false.
func (h *UserHandler) parseID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	return parseIDParam(w, r, "id", "user ID")
}
