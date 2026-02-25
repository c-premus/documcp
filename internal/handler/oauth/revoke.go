package oauthhandler

import (
	"encoding/json"
	"net/http"

	"git.999.haus/chris/DocuMCP-go/internal/auth/oauth"
)

// Revoke handles POST /oauth/revoke — token revocation (RFC 7009).
func (h *Handler) Revoke(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token         string `json:"token"`
		ClientID      string `json:"client_id"`
		ClientSecret  string `json:"client_secret"`
		TokenTypeHint string `json:"token_type_hint"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if req.Token == "" {
		oauthError(w, http.StatusBadRequest, "invalid_request", "The token field is required.")
		return
	}
	if req.ClientID == "" {
		oauthError(w, http.StatusBadRequest, "invalid_request", "The client id field is required.")
		return
	}
	if req.TokenTypeHint != "" && req.TokenTypeHint != "access_token" && req.TokenTypeHint != "refresh_token" {
		oauthError(w, http.StatusBadRequest, "invalid_request", "The selected token type hint is invalid.")
		return
	}

	err := h.service.RevokeToken(r.Context(), oauth.RevokeTokenParams{
		Token:         req.Token,
		ClientID:      req.ClientID,
		ClientSecret:  req.ClientSecret,
		TokenTypeHint: req.TokenTypeHint,
	})
	if err != nil {
		h.logger.Error("revoking token", "error", err)
		oauthError(w, http.StatusInternalServerError, "server_error", "An internal error occurred while processing the revocation request")
		return
	}

	// Per RFC 7009, always return 200 OK with empty array
	jsonResponse(w, http.StatusOK, []any{})
}
