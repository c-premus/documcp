package oauthhandler

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"

	"github.com/c-premus/documcp/internal/auth/oauth"
	authscope "github.com/c-premus/documcp/internal/auth/scope"
)

// Authorize handles GET /oauth/authorize — shows the consent screen.
func (h *Handler) Authorize(w http.ResponseWriter, r *http.Request) {
	// Parse and validate query parameters
	responseType := r.URL.Query().Get("response_type")
	clientID := r.URL.Query().Get("client_id")
	redirectURI := r.URL.Query().Get("redirect_uri")
	state := r.URL.Query().Get("state")
	scope := r.URL.Query().Get("scope")
	codeChallenge := r.URL.Query().Get("code_challenge")
	codeChallengeMethod := r.URL.Query().Get("code_challenge_method")

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

	// Generate consent nonce
	nonce := uuid.New().String()

	// Clear any completed redirect from a previous flow.
	delete(session.Values, "oauth_completed_redirect")
	delete(session.Values, "oauth_completed_nonce")

	// Store pending request in session
	session.Values["oauth_pending_request"] = map[string]any{
		"nonce":                 nonce,
		"client_id":             clientID,
		"state":                 state,
		"redirect_uri":          redirectURI,
		"code_challenge":        codeChallenge,
		"code_challenge_method": codeChallengeMethod,
		"scope":                 effectiveScope,
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
	_, _ = fmt.Fprintf(w, consentHTML,
		html.EscapeString(client.ClientName),
		html.EscapeString(effectiveScope),
		html.EscapeString(clientID),
		html.EscapeString(redirectURI),
		html.EscapeString(state),
		html.EscapeString(effectiveScope),
		html.EscapeString(codeChallenge),
		html.EscapeString(codeChallengeMethod),
		html.EscapeString(nonce),
	)
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
			// Override CSP for cross-origin redirect (see primary path below).
			w.Header().Set("Content-Security-Policy",
				"default-src 'none'; form-action *; frame-ancestors 'none'")
			// 303 See Other: tells the browser to follow the redirect with GET,
			// preventing further POST replays.
			http.Redirect(w, r, completedURL, http.StatusSeeOther)
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

	// Defense-in-depth: verify POST body values match session state.
	// An attacker who tampers with the POST body cannot alter the grant.
	if reqRedirectURI != pendingRedirectURI {
		http.Error(w, "redirect_uri mismatch", http.StatusBadRequest)
		return
	}
	if reqScope != pendingScope {
		h.logger.Warn("scope mismatch between POST body and session", "post", reqScope, "session", pendingScope)
	}
	if reqCodeChallenge != pendingCodeChallenge {
		h.logger.Warn("code_challenge mismatch between POST body and session", "post", reqCodeChallenge, "session", pendingCodeChallenge)
	}
	if reqCodeChallengeMethod != pendingCodeChallengeMethod {
		h.logger.Warn("code_challenge_method mismatch between POST body and session", "post", reqCodeChallengeMethod, "session", pendingCodeChallengeMethod)
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
	})
	if err != nil {
		h.logger.Error("generating authorization code", "error", err)
		http.Error(w, "Failed to generate authorization code", http.StatusInternalServerError)
		return
	}

	// Build redirect URL using session-validated URI — not user-supplied POST body.
	redirectURL := pendingRedirectURI + "?code=" + url.QueryEscape(code)
	if reqState != "" {
		redirectURL += "&state=" + url.QueryEscape(reqState)
	}

	// Replace pending state with completed state so retries re-redirect
	// instead of returning 400. The auth code is single-use at the token
	// endpoint, so replaying this redirect is safe.
	delete(session.Values, "oauth_pending_request")
	session.Values["oauth_completed_redirect"] = redirectURL
	session.Values["oauth_completed_nonce"] = reqNonce
	_ = session.Save(r, w)

	// Override the global CSP: the redirect target is the client's
	// redirect_uri (different origin). Safari checks form-action on the
	// 303 response as part of the form submission chain.
	w.Header().Set("Content-Security-Policy",
		"default-src 'none'; form-action *; frame-ancestors 'none'")

	// 303 See Other: tells the browser to follow the redirect with GET,
	// preventing POST replay. 302 Found is ambiguous after POST — Safari
	// may re-POST instead of switching to GET.
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
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
		q := url.Values{}
		q.Set("error", "access_denied")
		q.Set("error_description", "The resource owner denied the request")
		if state != "" {
			q.Set("state", state)
		}
		redirectURL := redirectURI + "?" + q.Encode()
		// Override CSP for cross-origin redirect (see AuthorizeApprove).
		w.Header().Set("Content-Security-Policy",
			"default-src 'none'; form-action *; frame-ancestors 'none'")
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	// No redirect_uri — render a simple denial page
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, denyHTML)
}

const denyHTML = `<!DOCTYPE html>
<html>
<head><title>Authorization Denied</title>
<style>
body{font-family:system-ui,sans-serif;max-width:480px;margin:60px auto;padding:0 20px}
h1{font-size:1.5em}
</style>
</head>
<body>
<h1>Authorization Denied</h1>
<p>Authorization denied. You may close this window.</p>
</body>
</html>`

// pendingStateMaxAge is the maximum age (in seconds) for pending OAuth state
// stored in the session (authorization code flow and device code flow).
const pendingStateMaxAge = 600 // 10 minutes

const consentHTML = `<!DOCTYPE html>
<html>
<head><title>Authorize Application</title>
<style>
body{font-family:system-ui,sans-serif;max-width:480px;margin:60px auto;padding:0 20px}
h1{font-size:1.5em}
.client-name{font-weight:bold;color:#2563eb}
.scope{background:#f1f5f9;padding:4px 8px;border-radius:4px;font-family:monospace}
form{margin-top:24px}
button{padding:10px 24px;font-size:1em;border:none;border-radius:6px;cursor:pointer;margin-right:8px}
.approve{background:#2563eb;color:white}
.deny{background:#e2e8f0;color:#334155}
</style>
</head>
<body>
<h1>Authorize Application</h1>
<p><span class="client-name">%s</span> is requesting access to your account.</p>
<p>Scope: <span class="scope">%s</span></p>
<form method="POST" action="/oauth/authorize/approve">
<input type="hidden" name="client_id" value="%s">
<input type="hidden" name="redirect_uri" value="%s">
<input type="hidden" name="state" value="%s">
<input type="hidden" name="scope" value="%s">
<input type="hidden" name="code_challenge" value="%s">
<input type="hidden" name="code_challenge_method" value="%s">
<input type="hidden" name="nonce" value="%s">
<button type="submit" class="approve">Authorize</button>
<button type="submit" class="deny" formaction="/oauth/authorize/deny">Deny</button>
</form>
</body>
</html>`
