package oauthhandler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/c-premus/documcp/internal/auth/oauth"
	authscope "github.com/c-premus/documcp/internal/auth/scope"
)

// Token handles POST /oauth/token — the token endpoint.
func (h *Handler) Token(w http.ResponseWriter, r *http.Request) {
	var req struct {
		GrantType    string `json:"grant_type"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		Code         string `json:"code"`
		RedirectURI  string `json:"redirect_uri"`
		CodeVerifier string `json:"code_verifier"`
		RefreshToken string `json:"refresh_token"`
		Scope        string `json:"scope"`
		DeviceCode   string `json:"device_code"`
	}

	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if err := r.ParseForm(); err != nil {
			oauthError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
			return
		}
		req.GrantType = r.FormValue("grant_type")
		req.ClientID = r.FormValue("client_id")
		req.ClientSecret = r.FormValue("client_secret")
		req.Code = r.FormValue("code")
		req.RedirectURI = r.FormValue("redirect_uri")
		req.CodeVerifier = r.FormValue("code_verifier")
		req.RefreshToken = r.FormValue("refresh_token")
		req.Scope = r.FormValue("scope")
		req.DeviceCode = r.FormValue("device_code")
	} else {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			oauthError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
			return
		}
	}

	req.Scope = authscope.Normalize(req.Scope)

	// Validate required fields
	if req.GrantType == "" {
		oauthError(w, http.StatusBadRequest, "invalid_request", "The grant type field is required.")
		return
	}
	if req.ClientID == "" {
		oauthError(w, http.StatusBadRequest, "invalid_request", "The client id field is required.")
		return
	}

	switch req.GrantType {
	case "authorization_code":
		h.tokenAuthorizationCode(w, r, req.ClientID, req.ClientSecret, req.Code, req.RedirectURI, req.CodeVerifier)
	case "refresh_token":
		h.tokenRefreshToken(w, r, req.ClientID, req.ClientSecret, req.RefreshToken, req.Scope)
	case "urn:ietf:params:oauth:grant-type:device_code":
		h.tokenDeviceCode(w, r, req.ClientID, req.ClientSecret, req.DeviceCode)
	default:
		oauthError(w, http.StatusBadRequest, "unsupported_grant_type",
			fmt.Sprintf("Grant type %s is not supported", req.GrantType))
	}
}

func (h *Handler) tokenAuthorizationCode(w http.ResponseWriter, r *http.Request, clientID, clientSecret, code, redirectURI, codeVerifier string) {
	if code == "" {
		oauthError(w, http.StatusBadRequest, "invalid_request", "The code field is required when grant type is authorization_code.")
		return
	}
	if redirectURI == "" {
		oauthError(w, http.StatusBadRequest, "invalid_request", "The redirect uri field is required when grant type is authorization_code.")
		return
	}

	result, err := h.service.ExchangeAuthorizationCode(r.Context(), oauth.ExchangeAuthorizationCodeParams{
		Code:         code,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
		CodeVerifier: codeVerifier,
	})
	if err != nil {
		h.logger.Warn("oauth token failed: invalid authorization code",
			"client_ip", r.RemoteAddr,
			"client_id", clientID,
			"error", err,
		)
		oauthError(w, http.StatusBadRequest, "invalid_grant", "The authorization code is invalid, expired, or has already been used")
		return
	}

	jsonResponse(w, http.StatusOK, result)
}

func (h *Handler) tokenRefreshToken(w http.ResponseWriter, r *http.Request, clientID, clientSecret, refreshToken, scope string) {
	if refreshToken == "" {
		oauthError(w, http.StatusBadRequest, "invalid_request", "The refresh token field is required when grant type is refresh_token.")
		return
	}

	result, err := h.service.RefreshAccessToken(r.Context(), oauth.RefreshTokenParams{
		RefreshToken: refreshToken,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scope:        scope,
	})
	if err != nil {
		h.logger.Warn("oauth token failed: invalid refresh token",
			"client_ip", r.RemoteAddr,
			"client_id", clientID,
			"error", err,
		)
		oauthError(w, http.StatusBadRequest, "invalid_grant", "The refresh token is invalid, expired, or has been revoked")
		return
	}

	jsonResponse(w, http.StatusOK, result)
}

func (h *Handler) tokenDeviceCode(w http.ResponseWriter, r *http.Request, clientID, clientSecret, deviceCode string) {
	if deviceCode == "" {
		oauthError(w, http.StatusBadRequest, "invalid_request", "The device code field is required when grant type is urn:ietf:params:oauth:grant-type:device_code.")
		return
	}

	result, err := h.service.ExchangeDeviceCode(r.Context(), oauth.ExchangeDeviceCodeParams{
		DeviceCode:   deviceCode,
		ClientID:     clientID,
		ClientSecret: clientSecret,
	})
	if err != nil {
		// Check for typed device code errors (authorization_pending, slow_down, expired_token).
		var dcErr *oauth.DeviceCodeError
		if errors.As(err, &dcErr) {
			// Only log non-pending errors — authorization_pending is normal polling.
			if dcErr.Code != "authorization_pending" {
				h.logger.Warn("oauth token failed: device code error",
					"client_ip", r.RemoteAddr,
					"client_id", clientID,
					"error_code", dcErr.Code,
				)
			}
			oauthError(w, http.StatusBadRequest, dcErr.Code, dcErr.Description)
			return
		}
		h.logger.Warn("oauth token failed: device code exchange error",
			"client_ip", r.RemoteAddr,
			"client_id", clientID,
			"error", err,
		)
		oauthError(w, http.StatusInternalServerError, "server_error", "An internal error occurred while processing the token request")
		return
	}

	jsonResponse(w, http.StatusOK, result)
}
