package oauthhandler

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/c-premus/documcp/internal/auth/oauth"
	authscope "github.com/c-premus/documcp/internal/auth/scope"
)

// Token handles POST /oauth/token — the token endpoint.
//
// Content-Type handling follows RFC 6749 §3.2: only
// `application/x-www-form-urlencoded` is accepted. JSON bodies were supported
// in prior releases and are now rejected with 415 Unsupported Media Type —
// noted in security.md M4 (spec deviation; JSON support also muddied the
// CSRF analysis).
//
// Client authentication accepts either `Authorization: Basic ...`
// (client_secret_basic, the RFC 6749 REQUIRED auth method) or credentials in
// the request body (client_secret_post). Callers MUST NOT supply both —
// RFC 6749 §2.3.1 forbids dual-auth. Security.md M3 fix: `wellknown.go`
// advertises `client_secret_basic` but prior releases only parsed body
// credentials, so a spec-compliant client using Basic auth silently failed.
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
		Resource     string `json:"resource"`
	}

	contentType := r.Header.Get("Content-Type")
	mediaType := contentType
	if idx := strings.Index(mediaType, ";"); idx != -1 {
		mediaType = strings.TrimSpace(mediaType[:idx])
	}
	mediaType = strings.ToLower(mediaType)
	if mediaType != "application/x-www-form-urlencoded" {
		// RFC 6749 §3.2 mandates form encoding at the token endpoint.
		w.Header().Set("Accept", "application/x-www-form-urlencoded")
		oauthError(w, http.StatusUnsupportedMediaType, "invalid_request",
			"The token endpoint requires Content-Type: application/x-www-form-urlencoded")
		return
	}

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
	req.Resource = r.FormValue("resource")

	// HTTP Basic auth overrides body credentials. Presenting both is a
	// protocol violation; reject per RFC 6749 §2.3.1.
	if basicID, basicSecret, hasBasic := r.BasicAuth(); hasBasic {
		if req.ClientID != "" || req.ClientSecret != "" {
			oauthError(w, http.StatusBadRequest, "invalid_request",
				"client credentials must be supplied via either HTTP Basic or request body, not both")
			return
		}
		req.ClientID = basicID
		req.ClientSecret = basicSecret
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
		h.tokenAuthorizationCode(w, r, req.ClientID, req.ClientSecret, req.Code, req.RedirectURI, req.CodeVerifier, req.Resource)
	case "refresh_token":
		h.tokenRefreshToken(w, r, req.ClientID, req.ClientSecret, req.RefreshToken, req.Scope, req.Resource)
	case "urn:ietf:params:oauth:grant-type:device_code":
		h.tokenDeviceCode(w, r, req.ClientID, req.ClientSecret, req.DeviceCode)
	default:
		oauthError(w, http.StatusBadRequest, "unsupported_grant_type",
			fmt.Sprintf("Grant type %s is not supported", req.GrantType))
	}
}

func (h *Handler) tokenAuthorizationCode(w http.ResponseWriter, r *http.Request, clientID, clientSecret, code, redirectURI, codeVerifier, resource string) {
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
		Resource:     resource,
	})
	if err != nil {
		h.logger.Warn("oauth token failed: invalid authorization code",
			"client_ip", r.RemoteAddr,
			"client_id", clientID,
			"error", err,
		)
		if errors.Is(err, oauth.ErrUnsupportedGrant) {
			oauthError(w, http.StatusBadRequest, "unauthorized_client", "This client is not authorized for the authorization_code grant type")
			return
		}
		oauthError(w, http.StatusBadRequest, "invalid_grant", "The authorization code is invalid, expired, or has already been used")
		return
	}

	jsonResponse(w, http.StatusOK, result)
}

func (h *Handler) tokenRefreshToken(w http.ResponseWriter, r *http.Request, clientID, clientSecret, refreshToken, scope, resource string) {
	if refreshToken == "" {
		oauthError(w, http.StatusBadRequest, "invalid_request", "The refresh token field is required when grant type is refresh_token.")
		return
	}

	result, err := h.service.RefreshAccessToken(r.Context(), oauth.RefreshTokenParams{
		RefreshToken: refreshToken,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scope:        scope,
		Resource:     resource,
	})
	if err != nil {
		h.logger.Warn("oauth token failed: invalid refresh token",
			"client_ip", r.RemoteAddr,
			"client_id", clientID,
			"error", err,
		)
		if errors.Is(err, oauth.ErrUnsupportedGrant) {
			oauthError(w, http.StatusBadRequest, "unauthorized_client", "This client is not authorized for the refresh_token grant type")
			return
		}
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
