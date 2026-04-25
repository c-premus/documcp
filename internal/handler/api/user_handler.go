package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

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

// sessionRevoker is the narrow slice of internal/auth/session.Store that
// UserHandler needs. List + revoke-one + revoke-all back the admin sessions
// surface; auto-revoke on Delete and on demote piggyback on RevokeUserSessions.
// Defined where consumed so production wires *session.Store and tests pass a
// recording stub. Nil revoker disables the auto-revoke + admin endpoints —
// preserves the pre-Redis-store behavior for tests that don't care.
type sessionRevoker interface {
	ListUserSessions(ctx context.Context, userID int64) ([]string, error)
	RevokeSession(ctx context.Context, sessionID string) error
	RevokeUserSessions(ctx context.Context, userID int64) (int, error)
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
	repo     userRepo
	sessions sessionRevoker
	logger   *slog.Logger
}

// NewUserHandler creates a new UserHandler. sessions may be nil when the
// caller hasn't wired a session store (tests, or future deployments that opt
// out of Redis sessions); the auto-revoke calls and the admin sessions
// endpoints become no-ops in that case.
func NewUserHandler(
	repo userRepo,
	sessions sessionRevoker,
	logger *slog.Logger,
) *UserHandler {
	return &UserHandler{
		repo:     repo,
		sessions: sessions,
		logger:   logger,
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

	h.revokeSessionsBestEffort(r.Context(), id, "user_deleted")

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

	// Demote-to-non-admin invalidates every existing session for this user so
	// the demoted user can't continue using their tab until the cookie expires.
	// Promote-to-admin needs no revocation — middleware re-reads is_admin per
	// request, so the next request sees the elevated role.
	if !user.IsAdmin {
		h.revokeSessionsBestEffort(r.Context(), id, "admin_demoted")
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": newUserResponse(user),
	})
}

// ListSessions handles GET /api/admin/users/{id}/sessions — return the IDs
// of every Redis-backed session attached to the user. Empty list is a 200,
// not a 404, so the UI doesn't have to disambiguate "no sessions" from
// "user not found" (Show covers user existence).
func (h *UserHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseID(w, r)
	if !ok {
		return
	}
	if h.sessions == nil {
		jsonResponse(w, http.StatusOK, map[string]any{"data": []string{}})
		return
	}
	ids, err := h.sessions.ListUserSessions(r.Context(), id)
	if err != nil {
		h.logger.Error("listing user sessions", "user_id", id, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to list sessions")
		return
	}
	if ids == nil {
		ids = []string{}
	}
	jsonResponse(w, http.StatusOK, map[string]any{"data": ids})
}

// RevokeSession handles DELETE /api/admin/users/{id}/sessions/{sessionID}.
// Session IDs are server-issued opaque strings; we accept whatever the caller
// passes and rely on RevokeSession's "missing key is harmless" contract.
func (h *UserHandler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.parseID(w, r); !ok {
		return
	}
	if h.sessions == nil {
		errorResponse(w, http.StatusServiceUnavailable, "session store not available")
		return
	}
	sessionID := chi.URLParam(r, "sessionID")
	if sessionID == "" {
		errorResponse(w, http.StatusBadRequest, "session ID required")
		return
	}
	if err := h.sessions.RevokeSession(r.Context(), sessionID); err != nil {
		h.logger.Error("revoking session", "session_id", sessionID, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to revoke session")
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"message": "session revoked"})
}

// RevokeAllSessions handles DELETE /api/admin/users/{id}/sessions — drop
// every session attached to the user. Returns the number revoked so the UI
// can render a confirmation toast.
func (h *UserHandler) RevokeAllSessions(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseID(w, r)
	if !ok {
		return
	}
	if h.sessions == nil {
		jsonResponse(w, http.StatusOK, map[string]any{"data": map[string]int{"revoked": 0}})
		return
	}
	count, err := h.sessions.RevokeUserSessions(r.Context(), id)
	if err != nil {
		h.logger.Error("revoking user sessions", "user_id", id, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to revoke sessions")
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"data": map[string]int{"revoked": count}})
}

// revokeSessionsBestEffort fires RevokeUserSessions in the background of a
// successful Delete / demote and Warn-logs on failure. The user op already
// succeeded — a Redis blip on the cleanup hook should not turn a 200 into a
// 500. Middleware re-reads the user row each request, so even if revocation
// drops the demoted user still loses admin scope on their next request.
func (h *UserHandler) revokeSessionsBestEffort(ctx context.Context, userID int64, reason string) {
	if h.sessions == nil {
		return
	}
	count, err := h.sessions.RevokeUserSessions(ctx, userID)
	if err != nil {
		h.logger.Warn("revoking sessions on user op", "user_id", userID, "reason", reason, "error", err)
		return
	}
	if count > 0 {
		h.logger.Info("revoked user sessions", "user_id", userID, "reason", reason, "count", count)
	}
}

// parseID extracts and validates the {id} URL parameter.
// Returns the parsed ID and true on success, or writes an error response and returns false.
func (h *UserHandler) parseID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	return parseIDParam(w, r, "id", "user ID")
}
