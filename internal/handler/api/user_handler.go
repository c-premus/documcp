package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"git.999.haus/chris/DocuMCP-go/internal/model"
	"git.999.haus/chris/DocuMCP-go/internal/repository"
)

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
	r := userResponse{
		ID:      u.ID,
		Name:    u.Name,
		Email:   u.Email,
		IsAdmin: u.IsAdmin,
	}
	if u.OIDCSub.Valid {
		r.OIDCSub = u.OIDCSub.String
	}
	if u.OIDCProvider.Valid {
		r.OIDCProvider = u.OIDCProvider.String
	}
	if u.EmailVerifiedAt.Valid {
		r.EmailVerifiedAt = u.EmailVerifiedAt.Time.Format(time.RFC3339)
	}
	if u.CreatedAt.Valid {
		r.CreatedAt = u.CreatedAt.Time.Format(time.RFC3339)
	}
	if u.UpdatedAt.Valid {
		r.UpdatedAt = u.UpdatedAt.Time.Format(time.RFC3339)
	}
	return r
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

// List handles GET /api/admin/users — list users with optional search, limit, offset.
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")

	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit <= 0 {
		limit = 20
	}

	offset, err := strconv.Atoi(r.URL.Query().Get("offset"))
	if err != nil || offset < 0 {
		offset = 0
	}

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

// createUserRequest is the JSON body for creating a user.
type createUserRequest struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	IsAdmin bool   `json:"is_admin"`
}

// Create handles POST /api/admin/users — create a new user.
func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Name == "" || req.Email == "" {
		errorResponse(w, http.StatusBadRequest, "name and email are required")
		return
	}

	user := &model.User{
		Name:    req.Name,
		Email:   req.Email,
		IsAdmin: req.IsAdmin,
	}

	if err := h.repo.CreateUser(r.Context(), user); err != nil {
		h.logger.Error("creating user", "email", req.Email, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	jsonResponse(w, http.StatusCreated, map[string]any{
		"data": newUserResponse(user),
	})
}

// updateUserRequest is the JSON body for updating a user.
type updateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Update handles PUT /api/admin/users/{id} — update an existing user.
func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseID(w, r)
	if !ok {
		return
	}

	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	user, err := h.repo.FindUserByID(r.Context(), id)
	if err != nil {
		h.logger.Error("finding user for update", "id", id, "error", err)
		errorResponse(w, http.StatusNotFound, "user not found")
		return
	}

	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Email != "" {
		user.Email = req.Email
	}

	if err := h.repo.UpdateUser(r.Context(), user); err != nil {
		h.logger.Error("updating user", "id", id, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to update user")
		return
	}

	// Re-fetch to get the updated_at timestamp from the database.
	user, err = h.repo.FindUserByID(r.Context(), id)
	if err != nil {
		h.logger.Error("re-fetching user after update", "id", id, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to fetch updated user")
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
func (h *UserHandler) ToggleAdmin(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseID(w, r)
	if !ok {
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
	raw := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid user ID")
		return 0, false
	}
	return id, true
}
