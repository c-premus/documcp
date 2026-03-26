package oauthhandler

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/c-premus/documcp/internal/auth/oauth"
)

// Register handles POST /oauth/register — dynamic client registration (RFC 7591).
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	// Check if registration is enabled
	if !h.oauthCfg.RegistrationEnabled {
		http.NotFound(w, r)
		return
	}

	// Check auth requirement — verify admin status from DB, not session cache.
	if h.oauthCfg.RegistrationRequireAuth {
		session, err := h.store.Get(r, sessionName)
		if err != nil {
			// Stale/corrupt cookie — treat as unauthenticated.
			h.logger.Warn("session decode error in register", "error", err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		userID, ok := session.Values["user_id"].(int64)
		if !ok || userID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		user, err := h.service.FindUserByID(r.Context(), userID)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if !user.IsAdmin {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	}

	var req oauth.RegisterClientParams
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "Invalid request body")
		return
	}

	// Validate required fields
	if req.ClientName == "" {
		oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "The client name field is required.")
		return
	}
	if len(req.ClientName) > 255 {
		oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "The client name field must not be greater than 255 characters.")
		return
	}
	if len(req.RedirectURIs) == 0 {
		oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "The redirect uris field is required.")
		return
	}

	// Validate each redirect URI
	for _, uri := range req.RedirectURIs {
		parsed, err := url.ParseRequestURI(uri)
		if err != nil {
			oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "The redirect_uris.0 field must be a valid URL.")
			return
		}
		// OAuth 2.1: redirect URIs must use HTTPS unless targeting a loopback address.
		if parsed.Scheme != "https" && !oauth.IsLoopbackHost(parsed.Hostname()) {
			oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "Redirect URIs must use HTTPS for non-loopback hosts.")
			return
		}
	}

	// Validate grant_types
	validGrantTypes := map[string]bool{
		"authorization_code": true,
		"refresh_token":      true,
		"urn:ietf:params:oauth:grant-type:device_code": true,
	}
	for _, gt := range req.GrantTypes {
		if !validGrantTypes[gt] {
			oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "The selected grant_types.0 is invalid.")
			return
		}
	}

	// Validate response_types
	for _, rt := range req.ResponseTypes {
		if rt != "code" {
			oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "The selected response_types.0 is invalid.")
			return
		}
	}

	// Validate token_endpoint_auth_method
	if req.TokenEndpointAuthMethod != "" {
		validMethods := map[string]bool{"none": true, "client_secret_basic": true, "client_secret_post": true}
		if !validMethods[req.TokenEndpointAuthMethod] {
			oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "The selected token endpoint auth method is invalid.")
			return
		}
	}

	// Validate optional field lengths
	if len(req.SoftwareID) > 255 {
		oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "The software id field must not be greater than 255 characters.")
		return
	}
	if len(req.SoftwareVersion) > 100 {
		oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "The software version field must not be greater than 100 characters.")
		return
	}

	result, err := h.service.RegisterClient(r.Context(), req)
	if err != nil {
		h.logger.Error("registering oauth client", "error", err)
		oauthError(w, http.StatusInternalServerError, "server_error", "An internal error occurred while registering the client")
		return
	}

	jsonResponse(w, http.StatusCreated, result)
}
