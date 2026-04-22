package oauthhandler

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/c-premus/documcp/internal/auth/oauth"
	authscope "github.com/c-premus/documcp/internal/auth/scope"
	"github.com/c-premus/documcp/internal/model"
)

// DeviceAuthorization handles POST /oauth/device/code — issue device_code + user_code.
func (h *Handler) DeviceAuthorization(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientID string `json:"client_id"`
		Scope    string `json:"scope"`
		Resource string `json:"resource"`
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
		req.Resource = r.FormValue("resource")
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

	if req.Resource != "" {
		canonical, err := oauth.ValidateResource(req.Resource, h.oauthCfg.AllowedResources)
		if err != nil {
			oauthError(w, http.StatusBadRequest, "invalid_target", "The requested resource is not recognized.")
			return
		}
		req.Resource = canonical
	}

	result, err := h.service.GenerateDeviceCode(r.Context(), oauth.DeviceAuthorizationParams{
		ClientID: req.ClientID,
		Scope:    authscope.Normalize(req.Scope),
		Resource: req.Resource,
	})
	if err != nil {
		h.logger.Error("generating device code", "error", err)
		if errors.Is(err, oauth.ErrInvalidClient) || errors.Is(err, oauth.ErrUnsupportedGrant) {
			oauthError(w, http.StatusBadRequest, "invalid_client", err.Error())
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
		http.Redirect(w, r, "/auth/login?redirect="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
		return
	}

	userCode := r.URL.Query().Get("user_code")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = deviceVerificationTmpl.Execute(w, userCode)
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

	// Brute-force protection keyed on user_id in Redis (security L6). The
	// counter survives session-cookie resets — the prior session-scoped
	// implementation was defeated by clearing cookies. Fail-open on Redis
	// errors: a Redis blip shouldn't break legitimate device flows.
	allowed, failErr := h.deviceFailures.Allowed(r.Context(), userID)
	if failErr != nil {
		h.logger.Warn("device failure limiter check failed", "error", failErr, "user_id", userID)
	}
	if !allowed {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = deviceErrorTmpl.Execute(w, "Too many failed attempts. Please try again later.")
		return
	}

	userCode := r.FormValue("user_code")
	if userCode == "" || len(userCode) > 9 {
		if err := h.deviceFailures.Record(r.Context(), userID); err != nil {
			h.logger.Warn("device failure limiter record failed", "error", err, "user_id", userID)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = deviceErrorTmpl.Execute(w, "Invalid or expired user code. Please check the code and try again.")
		return
	}

	// Look up the device code
	dc, err := h.service.FindDeviceCodeByUserCode(r.Context(), userCode)
	if err != nil {
		if recErr := h.deviceFailures.Record(r.Context(), userID); recErr != nil {
			h.logger.Warn("device failure limiter record failed", "error", recErr, "user_id", userID)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = deviceErrorTmpl.Execute(w, "Invalid or expired user code. Please check the code and try again.")
		return
	}

	if time.Now().After(dc.ExpiresAt) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = deviceErrorTmpl.Execute(w, "This code has expired. Please request a new code from your device.")
		return
	}

	if dc.Status != model.DeviceCodeStatusPending {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = deviceErrorTmpl.Execute(w, "This code has already been used. Please request a new code from your device.")
		return
	}

	// Successful lookup: clear the failure counter so a typo earlier in the
	// window doesn't eat into a legit user's budget.
	if clearErr := h.deviceFailures.Clear(r.Context(), userID); clearErr != nil {
		h.logger.Warn("device failure limiter clear failed", "error", clearErr, "user_id", userID)
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
		_ = deviceErrorTmpl.Execute(w, "An error occurred while processing your authorization.")
		return
	}

	// Show the scope the user can actually grant (narrowed to their
	// entitlement ceiling via ThirdPartyGrantable — filters out `admin` and
	// `services:write`, closing security.md H2).
	//
	// No GrantClientScope call here (security.md H3): the grant is recorded
	// in AuthorizeDeviceCode only when the user clicks Approve, not on
	// consent render.
	scope := ""
	if dc.Scope.Valid {
		scope = dc.Scope.String
	}
	if scope != "" {
		user, err := h.service.FindUserByID(r.Context(), userID)
		if err != nil {
			h.logger.Error("looking up user for device consent", "error", err)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_ = deviceErrorTmpl.Execute(w, "An error occurred while processing your authorization.")
			return
		}
		scope = authscope.Intersect(scope, authscope.ThirdPartyGrantable(user.IsAdmin))
		if scope == "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_ = deviceErrorTmpl.Execute(w, "None of the requested scopes are available to your account.")
			return
		}
	}

	// Show consent screen
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = deviceConsentTmpl.Execute(w, struct {
		ClientName string
		Scope      string
		UserCode   string
	}{
		ClientName: client.ClientName,
		Scope:      scope,
		UserCode:   userCode,
	})
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
		_ = deviceErrorTmpl.Execute(w, "No pending device authorization. Please restart the authorization flow.")
		return
	}

	pending, ok := pendingRaw.(map[string]any)
	if !ok {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = deviceErrorTmpl.Execute(w, "No pending device authorization. Please restart the authorization flow.")
		return
	}

	// Validate user code matches
	pendingUserCode, _ := pending["user_code"].(string)
	if subtle.ConstantTimeCompare([]byte(oauth.NormalizeUserCode(userCode)), []byte(oauth.NormalizeUserCode(pendingUserCode))) != 1 {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = deviceErrorTmpl.Execute(w, "User code mismatch. Please restart the authorization flow.")
		return
	}

	// Validate timestamp (10 minute expiry)
	pendingTimestamp, _ := pending["timestamp"].(int64)
	if time.Now().Unix()-pendingTimestamp > pendingStateMaxAge {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = deviceErrorTmpl.Execute(w, "Authorization request expired. Please restart the authorization flow.")
		return
	}

	// Clear pending from session
	delete(session.Values, "device_code_pending")
	_ = session.Save(r, w)

	approved := action == "approve"

	if err := h.service.AuthorizeDeviceCode(r.Context(), userCode, userID, approved); err != nil {
		h.logger.Error("authorizing device code", "error", err)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = deviceErrorTmpl.Execute(w, "An error occurred while processing your authorization.")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if approved {
		_, _ = w.Write([]byte(deviceSuccessHTML))
	} else {
		_, _ = w.Write([]byte(deviceDeniedHTML))
	}
}
