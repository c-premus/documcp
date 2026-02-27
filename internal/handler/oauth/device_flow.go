package oauthhandler

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"git.999.haus/chris/DocuMCP-go/internal/auth/oauth"
)

// DeviceAuthorization handles POST /oauth/device/code — issue device_code + user_code.
func (h *Handler) DeviceAuthorization(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientID string `json:"client_id"`
		Scope    string `json:"scope"`
	}

	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			oauthError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
			return
		}
		req.ClientID = r.FormValue("client_id")
		req.Scope = r.FormValue("scope")
	} else {
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
	session, _ := h.store.Get(r, sessionName)
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
	session, _ := h.store.Get(r, sessionName)
	userID, ok := session.Values["user_id"].(int64)
	if !ok || userID == 0 {
		http.Redirect(w, r, "/auth/login?redirect=/oauth/device", http.StatusFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	userCode := r.FormValue("user_code")
	if userCode == "" || len(userCode) > 9 {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, deviceErrorHTML, "Invalid or expired user code. Please check the code and try again.")
		return
	}

	// Look up the device code
	dc, err := h.service.FindDeviceCodeByUserCode(r.Context(), userCode)
	if err != nil {
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

	scope := ""
	if dc.Scope.Valid {
		scope = dc.Scope.String
	}

	// Show consent screen
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, deviceConsentHTML, html.EscapeString(client.ClientName), html.EscapeString(scope), html.EscapeString(userCode), userID)
}

// DeviceApprove handles POST /oauth/device/approve — user approves/denies.
func (h *Handler) DeviceApprove(w http.ResponseWriter, r *http.Request) {
	// Check user is authenticated
	session, _ := h.store.Get(r, sessionName)
	userID, ok := session.Values["user_id"].(int64)
	if !ok || userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

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
	if time.Now().Unix()-pendingTimestamp > 600 {
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
<html>
<head><title>Device Authorization</title>
<style>
body{font-family:system-ui,sans-serif;max-width:480px;margin:60px auto;padding:0 20px}
h1{font-size:1.5em}
input[type=text]{font-size:1.5em;padding:10px;width:200px;text-align:center;letter-spacing:4px;text-transform:uppercase}
button{padding:10px 24px;font-size:1em;border:none;border-radius:6px;cursor:pointer;background:#2563eb;color:white}
</style>
</head>
<body>
<h1>Device Authorization</h1>
<p>Enter the code shown on your device:</p>
<form method="POST" action="/oauth/device">
<input type="text" name="user_code" value="%s" maxlength="9" placeholder="XXXX-XXXX" required>
<br><br>
<button type="submit">Continue</button>
</form>
</body>
</html>`

const deviceConsentHTML = `<!DOCTYPE html>
<html>
<head><title>Authorize Device</title>
<style>
body{font-family:system-ui,sans-serif;max-width:480px;margin:60px auto;padding:0 20px}
h1{font-size:1.5em}
.client-name{font-weight:bold;color:#2563eb}
.scope{background:#f1f5f9;padding:4px 8px;border-radius:4px;font-family:monospace}
button{padding:10px 24px;font-size:1em;border:none;border-radius:6px;cursor:pointer;margin-right:8px}
.approve{background:#2563eb;color:white}
.deny{background:#e2e8f0;color:#334155}
</style>
</head>
<body>
<h1>Authorize Device</h1>
<p><span class="client-name">%s</span> is requesting access to your account.</p>
<p>Scope: <span class="scope">%s</span></p>
<form method="POST" action="/oauth/device/approve">
<input type="hidden" name="user_code" value="%s">
<input type="hidden" name="user_id" value="%d">
<button type="submit" name="approve" value="approve" class="approve">Authorize</button>
<button type="submit" name="approve" value="deny" class="deny">Deny</button>
</form>
</body>
</html>`

const deviceSuccessHTML = `<!DOCTYPE html>
<html>
<head><title>Authorization Successful</title>
<style>body{font-family:system-ui,sans-serif;max-width:480px;margin:60px auto;padding:0 20px;text-align:center}h1{color:#16a34a}</style>
</head>
<body>
<h1>Authorization Successful!</h1>
<p>You can close this window and return to your device.</p>
</body>
</html>`

const deviceDeniedHTML = `<!DOCTYPE html>
<html>
<head><title>Authorization Denied</title>
<style>body{font-family:system-ui,sans-serif;max-width:480px;margin:60px auto;padding:0 20px;text-align:center}h1{color:#dc2626}</style>
</head>
<body>
<h1>Authorization Denied</h1>
<p>You can close this window.</p>
</body>
</html>`

const deviceErrorHTML = `<!DOCTYPE html>
<html>
<head><title>Error</title>
<style>body{font-family:system-ui,sans-serif;max-width:480px;margin:60px auto;padding:0 20px;text-align:center}h1{color:#dc2626}</style>
</head>
<body>
<h1>Error</h1>
<p>%s</p>
</body>
</html>`
