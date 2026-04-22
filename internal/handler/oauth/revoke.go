package oauthhandler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/c-premus/documcp/internal/auth/oauth"
)

// Revoke handles POST /oauth/revoke — token revocation (RFC 7009).
func (h *Handler) Revoke(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token         string `json:"token"`
		ClientID      string `json:"client_id"`
		ClientSecret  string `json:"client_secret"`
		TokenTypeHint string `json:"token_type_hint"`
	}

	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if err := r.ParseForm(); err != nil {
			oauthError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
			return
		}
		req.Token = r.FormValue("token")
		req.ClientID = r.FormValue("client_id")
		req.ClientSecret = r.FormValue("client_secret")
		req.TokenTypeHint = r.FormValue("token_type_hint")
	} else {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			oauthError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
			return
		}
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
	switch {
	case errors.Is(err, oauth.ErrInvalidClientCredentials):
		// RFC 6749 §5.2 — bad client auth is `invalid_client` / 401.
		w.Header().Set("WWW-Authenticate", `Basic realm="oauth"`)
		oauthError(w, http.StatusUnauthorized, "invalid_client", "Client authentication failed")
		return
	case err != nil:
		// RFC 7009 §2.2 — every other failure is swallowed: invalid token,
		// unknown token, internal error on the revoke write, etc. The client
		// cannot distinguish "already revoked" from "never existed" from
		// "storage hiccup", and that's the intended spec behavior.
		h.logger.Warn("swallowed revoke error per RFC 7009 §2.2",
			"error", err,
			"client_id", req.ClientID,
			"token_type_hint", req.TokenTypeHint,
		)
	}

	jsonResponse(w, http.StatusOK, []any{})
}
