package oauthhandler

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"github.com/c-premus/documcp/internal/auth/oauth"
	authscope "github.com/c-premus/documcp/internal/auth/scope"
)

// DeviceAuthorization handles POST /oauth/device/code — issue device_code + user_code.
func (h *Handler) DeviceAuthorization(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientID string `json:"client_id"`
		Scope    string `json:"scope"`
	}

	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if err := r.ParseForm(); err != nil {
			oauthError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
			return
		}
		req.ClientID = r.FormValue("client_id")
		req.Scope = r.FormValue("scope")
	} else {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			oauthError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
			return
		}
	}

	if req.ClientID == "" {
		oauthError(w, http.StatusBadRequest, "invalid_request", "The client id field is required.")
		return
	}

	result, err := h.service.GenerateDeviceCode(r.Context(), oauth.DeviceAuthorizationParams{
		ClientID: req.ClientID,
		Scope:    req.Scope,
	})
	if err != nil {
		h.logger.Error("generating device code", "error", err)
		// Check for specific client errors
		msg := err.Error()
		if msg == "invalid or inactive client" || msg == "client does not support device_code grant type" {
			oauthError(w, http.StatusBadRequest, "invalid_client", msg)
			return
		}
		oauthError(w, http.StatusInternalServerError, "server_error", "An internal error occurred while processing the device authorization request")
		return
	}

	jsonResponse(w, http.StatusOK, result)
}

// DeviceVerification handles GET /oauth/device — user verification page.
func (h *Handler) DeviceVerification(w http.ResponseWriter, r *http.Request) {
	// Check user is authenticated
	session, err := h.store.Get(r, sessionName)
	if err != nil {
		// Stale/corrupt cookie — redirect to login.
		h.logger.Warn("session decode error in device verification", "error", err)
	}
	userID, ok := session.Values["user_id"].(int64)
	if !ok || userID == 0 {
		http.Redirect(w, r, "/auth/login?redirect="+r.URL.RequestURI(), http.StatusFound)
		return
	}

	userCode := r.URL.Query().Get("user_code")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, deviceVerificationHTML, html.EscapeString(userCode))
}

// DeviceVerificationSubmit handles POST /oauth/device — user submits user_code.
func (h *Handler) DeviceVerificationSubmit(w http.ResponseWriter, r *http.Request) {
	// Check user is authenticated
	session, sessErr := h.store.Get(r, sessionName)
	if sessErr != nil {
		// Stale/corrupt cookie — redirect to login.
		h.logger.Warn("session decode error in device submit", "error", sessErr)
	}
	userID, ok := session.Values["user_id"].(int64)
	if !ok || userID == 0 {
		http.Redirect(w, r, "/auth/login?redirect=/oauth/device", http.StatusFound)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Brute force protection: track failed verification attempts per session (RFC 8628 §3.5).
	failedAttempts, _ := session.Values["device_failed_attempts"].(int)
	if failedAttempts >= 5 {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, deviceErrorHTML, "Too many failed attempts. Please log out and try again.")
		return
	}

	userCode := r.FormValue("user_code")
	if userCode == "" || len(userCode) > 9 {
		session.Values["device_failed_attempts"] = failedAttempts + 1
		_ = session.Save(r, w)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, deviceErrorHTML, "Invalid or expired user code. Please check the code and try again.")
		return
	}

	// Look up the device code
	dc, err := h.service.FindDeviceCodeByUserCode(r.Context(), userCode)
	if err != nil {
		session.Values["device_failed_attempts"] = failedAttempts + 1
		_ = session.Save(r, w)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, deviceErrorHTML, "Invalid or expired user code. Please check the code and try again.")
		return
	}

	if time.Now().After(dc.ExpiresAt) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, deviceErrorHTML, "This code has expired. Please request a new code from your device.")
		return
	}

	if dc.Status != "pending" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, deviceErrorHTML, "This code has already been used. Please request a new code from your device.")
		return
	}

	// Store pending device code in session
	session.Values["device_code_pending"] = map[string]any{
		"user_code": userCode,
		"timestamp": time.Now().Unix(),
	}
	_ = session.Save(r, w)

	// Look up client for the consent screen
	client, err := h.service.FindClientByInternalID(r.Context(), dc.ClientID)
	if err != nil {
		h.logger.Error("finding client for device code", "error", err)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, deviceErrorHTML, "An error occurred while processing your authorization.")
		return
	}

	// Show the scope the user can actually grant (narrowed to their entitlements).
	scope := ""
	if dc.Scope.Valid {
		scope = dc.Scope.String
	}
	if scope != "" {
		user, err := h.service.FindUserByID(r.Context(), userID)
		if err != nil {
			h.logger.Error("looking up user for device consent", "error", err)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprintf(w, deviceErrorHTML, "An error occurred while processing your authorization.")
			return
		}
		// Expand the client's registered scope with the user's entitlements so
		// auto-registered clients gain write/admin scopes when an admin approves.
		userEntitlements := authscope.UserScopes(user.IsAdmin)
		if entitled := authscope.Intersect(scope, userEntitlements); entitled != "" {
			if _, expandErr := h.service.ExpandClientScope(r.Context(), dc.ClientID, entitled); expandErr != nil {
				h.logger.Error("expanding client scope for device consent", "error", expandErr)
			}
		}

		scope = authscope.Intersect(scope, userEntitlements)
		if scope == "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprintf(w, deviceErrorHTML, "None of the requested scopes are available to your account.")
			return
		}
	}

	// Show consent screen
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, deviceConsentHTML, html.EscapeString(client.ClientName), html.EscapeString(scope), html.EscapeString(userCode))
}

// DeviceApprove handles POST /oauth/device/approve — user approves/denies.
func (h *Handler) DeviceApprove(w http.ResponseWriter, r *http.Request) {
	// Check user is authenticated
	session, err := h.store.Get(r, sessionName)
	if err != nil {
		// Stale/corrupt cookie — approval requires valid session state.
		h.logger.Warn("session decode error in device approve", "error", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID, ok := session.Values["user_id"].(int64)
	if !ok || userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	userCode := r.FormValue("user_code")
	action := r.FormValue("approve")

	// Validate pending device code in session
	pendingRaw, exists := session.Values["device_code_pending"]
	if !exists || pendingRaw == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, deviceErrorHTML, "No pending device authorization. Please restart the authorization flow.")
		return
	}

	pending, ok := pendingRaw.(map[string]any)
	if !ok {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, deviceErrorHTML, "No pending device authorization. Please restart the authorization flow.")
		return
	}

	// Validate user code matches
	pendingUserCode, _ := pending["user_code"].(string)
	if oauth.NormalizeUserCode(userCode) != oauth.NormalizeUserCode(pendingUserCode) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, deviceErrorHTML, "User code mismatch. Please restart the authorization flow.")
		return
	}

	// Validate timestamp (10 minute expiry)
	pendingTimestamp, _ := pending["timestamp"].(int64)
	if time.Now().Unix()-pendingTimestamp > pendingStateMaxAge {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, deviceErrorHTML, "Authorization request expired. Please restart the authorization flow.")
		return
	}

	// Clear pending from session
	delete(session.Values, "device_code_pending")
	_ = session.Save(r, w)

	approved := action == "approve"

	if err := h.service.AuthorizeDeviceCode(r.Context(), userCode, userID, approved); err != nil {
		h.logger.Error("authorizing device code", "error", err)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, deviceErrorHTML, "An error occurred while processing your authorization.")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if approved {
		_, _ = fmt.Fprint(w, deviceSuccessHTML)
	} else {
		_, _ = fmt.Fprint(w, deviceDeniedHTML)
	}
}

const deviceVerificationHTML = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Device Authorization</title>
<style>
body{font-family:system-ui,sans-serif;max-width:480px;margin:60px auto;padding:0 20px;color:#0f172a;background:#ffffff}
h1{font-size:1.5em}
label{display:block;margin-bottom:8px}
input[type=text]{font-size:1.5em;padding:10px;width:200px;text-align:center;letter-spacing:4px;text-transform:uppercase;border:2px solid #94a3b8;border-radius:6px;background:#ffffff;color:#0f172a}
input[type=text]:focus-visible{outline:2px solid #4f46e5;outline-offset:2px;border-color:#4f46e5}
button{padding:10px 24px;font-size:1em;border:none;border-radius:6px;cursor:pointer;background:#2563eb;color:white}
button:focus-visible{outline:2px solid #4f46e5;outline-offset:2px}
@media(prefers-color-scheme:dark){
body{color:#e2e8f0;background:#030712}
input[type=text]{background:#111827;color:#e2e8f0;border-color:#6b7280}
input[type=text]:focus-visible{outline-color:#818cf8;border-color:#818cf8}
button:focus-visible{outline-color:#818cf8}
}
</style>
</head>
<body>
<h1>Device Authorization</h1>
<form method="POST" action="/oauth/device">
<label for="user_code">Enter the code shown on your device:</label>
<input id="user_code" type="text" name="user_code" value="%s" maxlength="9" placeholder="XXXX-XXXX" autocomplete="off" required>
<br><br>
<button type="submit">Continue</button>
</form>
</body>
</html>`

const deviceConsentHTML = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Authorize Device</title>
<style>
body{font-family:system-ui,sans-serif;max-width:480px;margin:60px auto;padding:0 20px;color:#0f172a;background:#ffffff}
h1{font-size:1.5em}
.client-name{font-weight:bold;color:#2563eb}
.scope{background:#f1f5f9;padding:4px 8px;border-radius:4px;font-family:monospace}
button{padding:10px 24px;font-size:1em;border:none;border-radius:6px;cursor:pointer;margin-right:8px}
.approve{background:#2563eb;color:white}
.deny{background:#e2e8f0;color:#334155}
button:focus-visible{outline:2px solid #4f46e5;outline-offset:2px}
@media(prefers-color-scheme:dark){
body{color:#e2e8f0;background:#030712}
.client-name{color:#60a5fa}
.scope{background:#111827;color:#e2e8f0}
.deny{background:#334155;color:#e2e8f0}
button:focus-visible{outline-color:#818cf8}
}
</style>
</head>
<body>
<h1>Authorize Device</h1>
<p><span class="client-name">%s</span> is requesting access to your account.</p>
<p>Scope: <span class="scope">%s</span></p>
<form method="POST" action="/oauth/device/approve">
<input type="hidden" name="user_code" value="%s">
<button type="submit" name="approve" value="approve" class="approve">Authorize</button>
<button type="submit" name="approve" value="deny" class="deny">Deny</button>
</form>
</body>
</html>`

const deviceSuccessHTML = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Authorization Successful</title>
<style>
body{font-family:system-ui,sans-serif;max-width:480px;margin:60px auto;padding:0 20px;text-align:center;color:#0f172a;background:#ffffff}
h1{color:#16a34a}
@media(prefers-color-scheme:dark){body{color:#e2e8f0;background:#030712}h1{color:#4ade80}}
</style>
</head>
<body>
<h1>Authorization Successful!</h1>
<p>You can close this window and return to your device.</p>
</body>
</html>`

const deviceDeniedHTML = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Authorization Denied</title>
<style>
body{font-family:system-ui,sans-serif;max-width:480px;margin:60px auto;padding:0 20px;text-align:center;color:#0f172a;background:#ffffff}
h1{color:#dc2626}
@media(prefers-color-scheme:dark){body{color:#e2e8f0;background:#030712}h1{color:#f87171}}
</style>
</head>
<body>
<h1>Authorization Denied</h1>
<p>You can close this window.</p>
</body>
</html>`

const deviceErrorHTML = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Error</title>
<style>
body{font-family:system-ui,sans-serif;max-width:480px;margin:60px auto;padding:0 20px;text-align:center;color:#0f172a;background:#ffffff}
h1{color:#dc2626}
@media(prefers-color-scheme:dark){body{color:#e2e8f0;background:#030712}h1{color:#f87171}}
</style>
</head>
<body>
<h1>Error</h1>
<p role="alert">%s</p>
</body>
</html>`
