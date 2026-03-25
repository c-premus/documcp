package api

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"git.999.haus/chris/DocuMCP-go/internal/model"
)

// oauthClientRepo defines the methods used by OAuthClientHandler.
type oauthClientRepo interface {
	ListClients(ctx context.Context, query string, limit, offset int) ([]model.OAuthClient, int, error)
	CreateClient(ctx context.Context, client *model.OAuthClient) error
	FindClientByID(ctx context.Context, id int64) (*model.OAuthClient, error)
	DeactivateClient(ctx context.Context, id int64) error
}

// OAuthClientHandler handles REST API endpoints for OAuth client administration.
type OAuthClientHandler struct {
	repo   oauthClientRepo
	logger *slog.Logger
}

// NewOAuthClientHandler creates a new OAuthClientHandler.
func NewOAuthClientHandler(
	repo oauthClientRepo,
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
	LastUsedAt              *string  `json:"last_used_at,omitempty"`
	CreatedAt               string   `json:"created_at,omitempty"`
	UpdatedAt               string   `json:"updated_at,omitempty"`
}

// List handles GET /api/admin/oauth-clients -- list OAuth clients with pagination.
func (h *OAuthClientHandler) List(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")

	limit, offset := parsePagination(r, 20, 100)

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

	jsonResponse(w, http.StatusOK, listResponse(items, total, limit, offset))
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
	id, ok := parseIDParam(w, r, "id", "client id")
	if !ok {
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

// Show handles GET /api/admin/oauth-clients/{id} -- get a single OAuth client.
func (h *OAuthClientHandler) Show(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r, "id", "client id")
	if !ok {
		return
	}

	client, err := h.repo.FindClientByID(r.Context(), id)
	if err != nil {
		h.logger.Error("finding oauth client", "id", id, "error", err)
		errorResponse(w, http.StatusNotFound, "oauth client not found")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": toOAuthClientResponse(client),
	})
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
		Scope:                   nullStringValue(c.Scope),
		LastUsedAt:              nullTimePtr(c.LastUsedAt),
		CreatedAt:               nullTimeToString(c.CreatedAt),
		UpdatedAt:               nullTimeToString(c.UpdatedAt),
	}

	return resp
}
