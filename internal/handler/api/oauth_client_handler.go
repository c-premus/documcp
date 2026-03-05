package api

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"git.999.haus/chris/DocuMCP-go/internal/model"
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

// oauthClientResponse is the JSON representation of an OAuth client.
type oauthClientResponse struct {
	ID                      int64    `json:"id"`
	ClientID                string   `json:"client_id"`
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	Scope                   string   `json:"scope,omitempty"`
	IsActive                bool     `json:"is_active"`
	CreatedAt               string   `json:"created_at,omitempty"`
	UpdatedAt               string   `json:"updated_at,omitempty"`
}

// List handles GET /api/admin/oauth-clients -- list OAuth clients with pagination.
func (h *OAuthClientHandler) List(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 20
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	clients, total, err := h.repo.ListClients(r.Context(), query, limit, offset)
	if err != nil {
		h.logger.Error("listing oauth clients", "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to list oauth clients")
		return
	}

	items := make([]oauthClientResponse, 0, len(clients))
	for i := range clients {
		items = append(items, toOAuthClientResponse(&clients[i]))
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": items,
		"meta": map[string]any{
			"total":  total,
			"limit":  limit,
			"offset": offset,
		},
	})
}

// Create handles POST /api/admin/oauth-clients -- create a new OAuth client.
func (h *OAuthClientHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ClientName              string   `json:"client_name"`
		RedirectURIs            []string `json:"redirect_uris"`
		GrantTypes              []string `json:"grant_types"`
		TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
		Scope                   string   `json:"scope"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if body.ClientName == "" {
		errorResponse(w, http.StatusBadRequest, "client_name is required")
		return
	}

	// Generate client credentials.
	clientID := uuid.New().String()
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		h.logger.Error("generating client secret", "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to generate client secret")
		return
	}
	plaintextSecret := hex.EncodeToString(secretBytes)

	hashedSecret, err := bcrypt.GenerateFromPassword([]byte(plaintextSecret), bcrypt.DefaultCost)
	if err != nil {
		h.logger.Error("hashing client secret", "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to hash client secret")
		return
	}

	// Default values.
	redirectURIs := body.RedirectURIs
	if redirectURIs == nil {
		redirectURIs = []string{}
	}
	grantTypes := body.GrantTypes
	if grantTypes == nil {
		grantTypes = []string{"authorization_code"}
	}
	responseTypes := []string{"code"}
	authMethod := body.TokenEndpointAuthMethod
	if authMethod == "" {
		authMethod = "client_secret_post"
	}

	redirectURIsJSON, _ := json.Marshal(redirectURIs)
	grantTypesJSON, _ := json.Marshal(grantTypes)
	responseTypesJSON, _ := json.Marshal(responseTypes)

	client := &model.OAuthClient{
		ClientID:                clientID,
		ClientSecret:            sql.NullString{String: string(hashedSecret), Valid: true},
		ClientName:              body.ClientName,
		RedirectURIs:            string(redirectURIsJSON),
		GrantTypes:              string(grantTypesJSON),
		ResponseTypes:           string(responseTypesJSON),
		TokenEndpointAuthMethod: authMethod,
		IsActive:                true,
	}
	if body.Scope != "" {
		client.Scope = sql.NullString{String: body.Scope, Valid: true}
	}

	if err := h.repo.CreateClient(r.Context(), client); err != nil {
		h.logger.Error("creating oauth client", "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to create oauth client")
		return
	}

	jsonResponse(w, http.StatusCreated, map[string]any{
		"data": map[string]any{
			"id":            client.ID,
			"client_id":     clientID,
			"client_secret": plaintextSecret,
			"client_name":   body.ClientName,
		},
		"message": "OAuth client created. Copy the client_secret now — it will not be shown again.",
	})
}

// Revoke handles POST /api/admin/oauth-clients/{id}/revoke -- deactivate a client.
func (h *OAuthClientHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid client id")
		return
	}

	if err := h.repo.DeactivateClient(r.Context(), id); err != nil {
		h.logger.Error("revoking oauth client", "id", id, "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to revoke oauth client")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"message": "OAuth client revoked successfully.",
	})
}

// Show handles GET /api/admin/oauth-clients/{clientId} -- get a single OAuth client.
func (h *OAuthClientHandler) Show(w http.ResponseWriter, r *http.Request) {
	clientID := chi.URLParam(r, "clientId")
	_ = clientID

	errorResponse(w, http.StatusNotImplemented, "OAuth client detail endpoint not yet implemented")
}

// toOAuthClientResponse converts an OAuthClient model to its JSON response DTO.
func toOAuthClientResponse(c *model.OAuthClient) oauthClientResponse {
	redirectURIs, _ := c.ParseRedirectURIs()
	if redirectURIs == nil {
		redirectURIs = []string{}
	}
	grantTypes, _ := c.ParseGrantTypes()
	if grantTypes == nil {
		grantTypes = []string{}
	}
	var responseTypes []string
	if err := json.Unmarshal([]byte(c.ResponseTypes), &responseTypes); err != nil {
		responseTypes = []string{}
	}

	resp := oauthClientResponse{
		ID:                      c.ID,
		ClientID:                c.ClientID,
		ClientName:              c.ClientName,
		RedirectURIs:            redirectURIs,
		GrantTypes:              grantTypes,
		ResponseTypes:           responseTypes,
		TokenEndpointAuthMethod: c.TokenEndpointAuthMethod,
		IsActive:                c.IsActive,
	}

	if c.Scope.Valid {
		resp.Scope = c.Scope.String
	}
	if c.CreatedAt.Valid {
		resp.CreatedAt = c.CreatedAt.Time.Format(time.RFC3339)
	}
	if c.UpdatedAt.Valid {
		resp.UpdatedAt = c.UpdatedAt.Time.Format(time.RFC3339)
	}

	return resp
}

