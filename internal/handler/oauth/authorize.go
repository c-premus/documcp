package oauthhandler

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"

	"github.com/c-premus/documcp/internal/auth/oauth"
	authscope "github.com/c-premus/documcp/internal/auth/scope"
)

// Authorize handles GET /oauth/authorize — shows the consent screen.
//
//nolint:gocyclo // OAuth authorize is a sequence of independent validation steps; splitting them obscures the flow.
func (h *Handler) Authorize(w http.ResponseWriter, r *http.Request) {
	// Parse and validate query parameters
	responseType := r.URL.Query().Get("response_type")
	clientID := r.URL.Query().Get("client_id")
	redirectURI := r.URL.Query().Get("redirect_uri")
	state := r.URL.Query().Get("state")
	scope := authscope.Normalize(r.URL.Query().Get("scope"))
	codeChallenge := r.URL.Query().Get("code_challenge")
	codeChallengeMethod := r.URL.Query().Get("code_challenge_method")
	resource := r.URL.Query().Get("resource")

	// Validate required parameters
	if responseType == "" {
		oauthError(w, http.StatusBadRequest, "invalid_request", "The response type field is required.")
		return
	}
	if responseType != "code" {
		oauthError(w, http.StatusBadRequest, "invalid_request", "The selected response type is invalid.")
		return
	}
	if clientID == "" {
		oauthError(w, http.StatusBadRequest, "invalid_request", "The client id field is required.")
		return
	}
	if redirectURI == "" {
		oauthError(w, http.StatusBadRequest, "invalid_request", "The redirect uri field is required.")
		return
	}
	if state == "" || len(state) < 8 {
		oauthError(w, http.StatusBadRequest, "invalid_request", "The state field must be at least 8 characters.")
		return
	}
	if codeChallengeMethod != "" && codeChallengeMethod != "S256" {
		oauthError(w, http.StatusBadRequest, "invalid_request", "The selected code challenge method is invalid.")
		return
	}

	// RFC 8707 resource indicator. Optional at this layer (the resource
	// server enforces audience binding); validated against the configured
	// allowlist when present so callers can't bind tokens to unknown audiences.
	if resource != "" {
		canonical, err := oauth.ValidateResource(resource, h.oauthCfg.AllowedResources)
		if err != nil {
			oauthError(w, http.StatusBadRequest, "invalid_target", "The requested resource is not recognized.")
			return
		}
		resource = canonical
	}

	// Check user is authenticated
	session, err := h.store.Get(r, sessionName)
	if err != nil {
		// Stale/corrupt cookie — gorilla returns fresh empty session;
		// userID check below will redirect to login.
		h.logger.Warn("session decode error in authorize", "error", err)
	}
	userID, ok := session.Values["user_id"].(int64)
	if !ok || userID == 0 {
		// Redirect to login with return URL (escape to prevent open redirect)
		http.Redirect(w, r, "/auth/login?redirect="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
		return
	}

	// Look up the client
	client, err := h.service.FindClient(r.Context(), clientID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			oauthError(w, http.StatusBadRequest, "invalid_client", "Client not found or inactive")
			return
		}
		oauthError(w, http.StatusBadRequest, "invalid_client", "Client not found or inactive")
		return
	}

	// Validate redirect URI
	registeredURIs, err := client.ParseRedirectURIs()
	if err != nil || !oauth.MatchRedirectURI(redirectURI, registeredURIs) {
		oauthError(w, http.StatusBadRequest, "invalid_request", "Invalid redirect_uri")
		return
	}

	// PKCE is always required per OAuth 2.1 (RFC 9700, Section 7.5.2).
	if codeChallenge == "" {
		oauthError(w, http.StatusBadRequest, "invalid_request", "PKCE code_challenge required")
		return
	}
	if codeChallengeMethod == "" {
		oauthError(w, http.StatusBadRequest, "invalid_request", "PKCE code_challenge_method required")
		return
	}

	// Narrow requested scope to what this user is entitled to grant.
	// Admins can grant all scopes; regular users only DefaultScopes.
	user, err := h.service.FindUserByID(r.Context(), userID)
	if err != nil {
		h.logger.Error("looking up user for scope computation", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	effectiveScope := scope
	if scope != "" {
		effectiveScope = authscope.Intersect(scope, authscope.UserScopes(user.IsAdmin))
	}
	if effectiveScope == "" && scope != "" {
		oauthError(w, http.StatusBadRequest, "invalid_scope", "None of the requested scopes are available to your account.")
		return
	}

	// Record a time-bounded scope grant so the client can use these scopes
	// in future consent flows (replaces permanent scope widening).
	if effectiveScope != "" {
		if grantErr := h.service.GrantClientScope(r.Context(), client.ID, effectiveScope, userID); grantErr != nil {
			h.logger.Error("granting client scope", "error", grantErr)
			// Non-fatal: proceed with base client scope.
		}
	}

	// Narrow to the client's effective scope (base registration + active grants).
	// A client should not receive scopes beyond what it has been granted.
	if effectiveScope != "" {
		baseScope := ""
		if client.Scope.Valid {
			baseScope = client.Scope.String
		}
		clientEffective, effErr := h.service.EffectiveClientScope(r.Context(), client.ID, baseScope)
		if effErr != nil {
			h.logger.Error("computing effective client scope", "error", effErr)
			clientEffective = baseScope
		}
		if clientEffective != "" {
			effectiveScope = authscope.Intersect(effectiveScope, clientEffective)
			if effectiveScope == "" {
				oauthError(w, http.StatusBadRequest, "invalid_scope", "None of the requested scopes are available for this client.")
				return
			}
		}
	}

	// Generate consent nonce
	nonce := uuid.New().String()

	// Clear any completed redirect from a previous flow.
	delete(session.Values, "oauth_completed_redirect")
	delete(session.Values, "oauth_completed_nonce")

	// Remove legacy id_token from pre-v0.18 sessions that stored it in the
	// main cookie. It now lives in a separate cookie (documcp_idt). Without
	// this, the combined size exceeds the browser's ~4096-byte cookie limit.
	delete(session.Values, "id_token")

	// Store pending request in session
	session.Values["oauth_pending_request"] = map[string]any{
		"nonce":                 nonce,
		"client_id":             clientID,
		"state":                 state,
		"redirect_uri":          redirectURI,
		"code_challenge":        codeChallenge,
		"code_challenge_method": codeChallengeMethod,
		"scope":                 effectiveScope,
		"resource":              resource,
		"timestamp":             time.Now().Unix(),
	}
	if err := session.Save(r, w); err != nil {
		h.logger.Error("saving session", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Render simple consent screen.
	// Override the global CSP: the form POSTs to 'self' but the server responds
	// with a 302 to the client's redirect_uri (different origin). Chrome checks
	// the redirect destination against form-action, so we must allow any target.
	// This page is server-rendered with html.EscapeString — no injection risk.
	w.Header().Set("Content-Security-Policy",
		"default-src 'none'; style-src 'unsafe-inline'; form-action *; frame-ancestors 'none'")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = consentTmpl.Execute(w, struct {
		ClientName          string
		Scope               string
		ClientID            string
		RedirectURI         string
		State               string
		CodeChallenge       string
		CodeChallengeMethod string
		Nonce               string
	}{
		ClientName:          client.ClientName,
		Scope:               effectiveScope,
		ClientID:            clientID,
		RedirectURI:         redirectURI,
		State:               state,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: codeChallengeMethod,
		Nonce:               nonce,
	})
}

// AuthorizeApprove handles POST /oauth/authorize/approve — processes consent.
func (h *Handler) AuthorizeApprove(w http.ResponseWriter, r *http.Request) {
	// Check user is authenticated
	session, sessErr := h.store.Get(r, sessionName)
	if sessErr != nil {
		// Stale/corrupt cookie — consent requires valid session state.
		h.logger.Warn("session decode error in authorize/approve", "error", sessErr)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID, ok := session.Values["user_id"].(int64)
	if !ok || userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse form or JSON body
	var reqClientID, reqRedirectURI, reqState, reqScope string
	var reqCodeChallenge, reqCodeChallengeMethod, reqNonce string

	contentType := r.Header.Get("Content-Type")
	if contentType == "application/json" {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var body struct {
			ClientID            string `json:"client_id"`
			RedirectURI         string `json:"redirect_uri"`
			State               string `json:"state"`
			Scope               string `json:"scope"`
			CodeChallenge       string `json:"code_challenge"`
			CodeChallengeMethod string `json:"code_challenge_method"`
			Nonce               string `json:"nonce"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		reqClientID = body.ClientID
		reqRedirectURI = body.RedirectURI
		reqState = body.State
		reqScope = body.Scope
		reqCodeChallenge = body.CodeChallenge
		reqCodeChallengeMethod = body.CodeChallengeMethod
		reqNonce = body.Nonce
	} else {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		reqClientID = r.FormValue("client_id")
		reqRedirectURI = r.FormValue("redirect_uri")
		reqState = r.FormValue("state")
		reqScope = r.FormValue("scope")
		reqCodeChallenge = r.FormValue("code_challenge")
		reqCodeChallengeMethod = r.FormValue("code_challenge_method")
		reqNonce = r.FormValue("nonce")
	}

	// Check for idempotent retry: if we already approved this nonce and stored
	// the redirect URL, re-issue the same redirect. This handles Safari's
	// tendency to replay POST requests after a cross-origin redirect.
	completedURL, hasCompleted := session.Values["oauth_completed_redirect"].(string)
	if hasCompleted {
		completedNonce, _ := session.Values["oauth_completed_nonce"].(string)
		if completedNonce != "" && subtle.ConstantTimeCompare([]byte(reqNonce), []byte(completedNonce)) == 1 {
			jsRedirect(w, completedURL)
			return
		}
	}

	// Get pending request from session
	pendingRaw, exists := session.Values["oauth_pending_request"]
	if !exists || pendingRaw == nil {
		http.Error(w, "No pending OAuth request. Please restart the authorization flow.", http.StatusBadRequest)
		return
	}

	pending, ok := pendingRaw.(map[string]any)
	if !ok {
		http.Error(w, "No pending OAuth request. Please restart the authorization flow.", http.StatusBadRequest)
		return
	}

	// Validate nonce
	pendingNonce, _ := pending["nonce"].(string)
	if reqNonce == "" || subtle.ConstantTimeCompare([]byte(reqNonce), []byte(pendingNonce)) != 1 {
		http.Error(w, "Invalid authorization request. Please restart the authorization flow.", http.StatusBadRequest)
		return
	}

	// Validate client_id matches
	pendingClientID, _ := pending["client_id"].(string)
	if reqClientID != pendingClientID {
		http.Error(w, "OAuth request mismatch. This may happen if you have multiple authorization tabs open. Please close all tabs and try again.", http.StatusBadRequest)
		return
	}

	// Validate state matches
	pendingState, _ := pending["state"].(string)
	if reqState != pendingState {
		http.Error(w, "OAuth state mismatch. Please restart the authorization flow.", http.StatusBadRequest)
		return
	}

	// Validate timestamp (10 minute expiry)
	pendingTimestamp, _ := pending["timestamp"].(int64)
	if time.Now().Unix()-pendingTimestamp > pendingStateMaxAge {
		http.Error(w, "OAuth request expired. Please restart the authorization flow.", http.StatusBadRequest)
		return
	}

	// Validate state format
	if reqState != "" && !oauth.ValidateState(reqState) {
		http.Error(w, "Invalid state parameter format", http.StatusBadRequest)
		return
	}

	// Use server-controlled session state (validated in the GET handler) rather
	// than POST body values. This eliminates user-controlled data from the
	// authorization code grant entirely.
	pendingRedirectURI, _ := pending["redirect_uri"].(string)
	pendingScope, _ := pending["scope"].(string)
	pendingCodeChallenge, _ := pending["code_challenge"].(string)
	pendingCodeChallengeMethod, _ := pending["code_challenge_method"].(string)
	pendingResource, _ := pending["resource"].(string)

	// Defense-in-depth: verify POST body values match session state.
	// An attacker who tampers with the POST body cannot alter the grant.
	if reqRedirectURI != pendingRedirectURI {
		http.Error(w, "redirect_uri mismatch", http.StatusBadRequest)
		return
	}
	if reqScope != pendingScope {
		h.logger.Warn("scope mismatch between POST body and session", "post", reqScope, "session", pendingScope)
		http.Error(w, "scope mismatch", http.StatusBadRequest)
		return
	}
	if reqCodeChallenge != pendingCodeChallenge {
		h.logger.Warn("code_challenge mismatch between POST body and session", "post", reqCodeChallenge, "session", pendingCodeChallenge)
		http.Error(w, "code_challenge mismatch", http.StatusBadRequest)
		return
	}
	if reqCodeChallengeMethod != pendingCodeChallengeMethod {
		h.logger.Warn("code_challenge_method mismatch between POST body and session", "post", reqCodeChallengeMethod, "session", pendingCodeChallengeMethod)
		http.Error(w, "code_challenge_method mismatch", http.StatusBadRequest)
		return
	}

	// Look up client to get internal ID.
	client, err := h.service.FindClient(r.Context(), reqClientID)
	if err != nil {
		http.Error(w, "Failed to generate authorization code", http.StatusInternalServerError)
		return
	}

	// Generate authorization code using session-validated values.
	// Scope was narrowed to user entitlements in GET /oauth/authorize;
	// PKCE challenge was validated there as well.
	code, err := h.service.GenerateAuthorizationCode(r.Context(), oauth.GenerateAuthorizationCodeParams{
		ClientID:            client.ID,
		UserID:              userID,
		RedirectURI:         pendingRedirectURI,
		Scope:               pendingScope,
		CodeChallenge:       pendingCodeChallenge,
		CodeChallengeMethod: pendingCodeChallengeMethod,
		Resource:            pendingResource,
	})
	if err != nil {
		h.logger.Error("generating authorization code", "error", err)
		http.Error(w, "Failed to generate authorization code", http.StatusInternalServerError)
		return
	}

	// Build redirect URL using session-validated URI — not user-supplied POST body.
	// Use url.Parse to correctly append query params even if the redirect URI
	// already contains a query string (RFC 6749 §4.1.2).
	parsedRedirect, parseErr := url.Parse(pendingRedirectURI)
	if parseErr != nil {
		http.Error(w, "Invalid redirect URI", http.StatusBadRequest)
		return
	}
	q := parsedRedirect.Query()
	q.Set("code", code)
	if reqState != "" {
		q.Set("state", reqState)
	}
	parsedRedirect.RawQuery = q.Encode()
	redirectURL := parsedRedirect.String()

	// Replace pending state with completed state so retries re-redirect
	// instead of returning 400. The auth code is single-use at the token
	// endpoint, so replaying this redirect is safe.
	delete(session.Values, "oauth_pending_request")
	session.Values["oauth_completed_redirect"] = redirectURL
	session.Values["oauth_completed_nonce"] = reqNonce
	_ = session.Save(r, w)

	// Use a JavaScript redirect instead of HTTP 303. Safari and embedded
	// browsers (VS Code) do not reliably follow cross-origin 303 redirects
	// in popup windows opened by Claude.ai's MCP integration.
	jsRedirect(w, redirectURL)
}

// AuthorizeDeny handles POST /oauth/authorize/deny — user denies consent.
func (h *Handler) AuthorizeDeny(w http.ResponseWriter, r *http.Request) {
	// Check user is authenticated
	session, sessErr := h.store.Get(r, sessionName)
	if sessErr != nil {
		h.logger.Warn("session decode error in authorize/deny", "error", sessErr)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID, ok := session.Values["user_id"].(int64)
	if !ok || userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse form or JSON body — only need nonce
	var reqNonce string

	contentType := r.Header.Get("Content-Type")
	if contentType == "application/json" {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var body struct {
			Nonce string `json:"nonce"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		reqNonce = body.Nonce
	} else {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		reqNonce = r.FormValue("nonce")
	}

	// Get pending request from session
	pendingRaw, exists := session.Values["oauth_pending_request"]
	if !exists || pendingRaw == nil {
		http.Error(w, "No pending OAuth request. Please restart the authorization flow.", http.StatusBadRequest)
		return
	}

	pending, ok := pendingRaw.(map[string]any)
	if !ok {
		http.Error(w, "No pending OAuth request. Please restart the authorization flow.", http.StatusBadRequest)
		return
	}

	// Validate nonce
	pendingNonce, _ := pending["nonce"].(string)
	if reqNonce == "" || subtle.ConstantTimeCompare([]byte(reqNonce), []byte(pendingNonce)) != 1 {
		http.Error(w, "Invalid authorization request. Please restart the authorization flow.", http.StatusBadRequest)
		return
	}

	// Clear pending request from session
	delete(session.Values, "oauth_pending_request")
	_ = session.Save(r, w)

	// Extract redirect_uri and state from pending request
	redirectURI, _ := pending["redirect_uri"].(string)
	state, _ := pending["state"].(string)

	if redirectURI != "" {
		parsedRedirect, parseErr := url.Parse(redirectURI)
		if parseErr != nil {
			http.Error(w, "Invalid redirect URI", http.StatusBadRequest)
			return
		}
		q := parsedRedirect.Query()
		q.Set("error", "access_denied")
		q.Set("error_description", "The resource owner denied the request")
		if state != "" {
			q.Set("state", state)
		}
		parsedRedirect.RawQuery = q.Encode()
		jsRedirect(w, parsedRedirect.String())
		return
	}

	// No redirect_uri — render a simple denial page
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(denyHTML))
}

// jsRedirect renders an HTML page that performs a client-side redirect via
// JavaScript. Safari and embedded browsers (VS Code) do not reliably follow
// HTTP 303 redirects to a different origin in popup windows. A JS redirect
// works universally. The URL is injected as a JSON string to prevent XSS —
// the caller must pass a fully-constructed URL (not user input).
func jsRedirect(w http.ResponseWriter, redirectURL string) {
	w.Header().Set("Content-Security-Policy",
		"default-src 'none'; script-src 'unsafe-inline'; frame-ancestors 'none'")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	// JSON-encode the URL to safely embed it in a <script> tag.
	// template.JS marks the value as safe for JavaScript context —
	// json.Marshal already produces a safely-quoted string literal.
	urlJSON, _ := json.Marshal(redirectURL)
	_ = jsRedirectTmpl.Execute(w, template.JS(urlJSON)) //nolint:gosec // G203: urlJSON is json.Marshal output, safe for JS context
}

// pendingStateMaxAge is the maximum age (in seconds) for pending OAuth state
// stored in the session (authorization code flow and device code flow).
const pendingStateMaxAge = 600 // 10 minutes
