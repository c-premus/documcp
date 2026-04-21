// Package oidc implements OIDC authentication for admin login via an external identity provider.
package oidc

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/gorilla/sessions"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/oauth2"

	authmiddleware "github.com/c-premus/documcp/internal/auth/middleware"
	"github.com/c-premus/documcp/internal/config"
	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/security"
)

// TokenRevoker revokes OAuth tokens issued during a session. Implemented by
// *oauth.Service; declared here as a narrow interface so the OIDC package
// keeps its consumer-side dependency minimal and tests can substitute a stub.
type TokenRevoker interface {
	RevokeUserTokensSince(ctx context.Context, userID int64, since time.Time) (int64, error)
}

// UserRepo defines the repository interface consumed by the OIDC handler.
// There is no FindUserByEmail method by design — email is display-only here.
// Identity is `sub`. Looking up by email and silently linking a pre-created
// local record to an OIDC identity was the admin-takeover vector removed in
// v0.21.0.
type UserRepo interface {
	FindUserByOIDCSub(ctx context.Context, sub string) (*model.User, error)
	CreateUser(ctx context.Context, user *model.User) error
	UpdateUser(ctx context.Context, user *model.User) error
}

// Handler provides HTTP handlers for OIDC login/callback/logout.
type Handler struct {
	provider            *gooidc.Provider // nil in manual-endpoint mode
	oauth2Config        oauth2.Config
	verifier            *gooidc.IDTokenVerifier
	httpClient          *http.Client // instrumented with otelhttp for tracing
	store               sessions.Store
	repo                UserRepo
	tokenRevoker        TokenRevoker // nil disables session-logout token revocation
	logger              *slog.Logger
	adminGroups         []string
	bootstrapAdminEmail string // normalized lower-case; empty = disabled
	providerURL         string // stored for OIDCProvider field in user records
	endSessionEndpoint  string // RP-Initiated Logout endpoint (from discovery or manual config)
	appURL              string // post_logout_redirect_uri target
}

// Config holds the dependencies for creating a new OIDC Handler.
type Config struct {
	OIDCCfg      config.OIDCConfig
	SessionStore sessions.Store
	Repo         UserRepo
	TokenRevoker TokenRevoker // optional; enables security L7 on logout
	Logger       *slog.Logger
	AppURL       string // used as post_logout_redirect_uri
}

// gobRegisterOnce ensures session-stored types are registered with
// encoding/gob exactly once before any handler is created. Registration is an
// explicit side of New rather than a package init() so the registration is
// tied to OIDC being actually configured.
var gobRegisterOnce sync.Once

// New creates a new OIDC Handler. It discovers the provider configuration
// from the well-known endpoint, or uses manually configured endpoints when
// OIDC_AUTHORIZATION_URL and OIDC_TOKEN_URL are set (REQ-AUTH-003).
// Returns nil if OIDC is not configured.
func New(ctx context.Context, cfg Config) (*Handler, error) {
	if cfg.OIDCCfg.ProviderURL == "" || cfg.OIDCCfg.ClientID == "" {
		return nil, nil
	}

	// gorilla/sessions uses gob encoding. Register types stored in sessions.
	gobRegisterOnce.Do(func() {
		gob.Register(map[string]any{})
	})

	// Validate operator-configured URLs before any outbound HTTP. Private RFC-1918
	// ranges are allowed for homelab / internal Authentik deployments, matching
	// the SSRF policy used for Kiwix and Git template URLs.
	if err := validateOIDCURL(cfg.OIDCCfg.ProviderURL); err != nil {
		return nil, fmt.Errorf("validating OIDC_PROVIDER_URL: %w", err)
	}
	if cfg.OIDCCfg.ManualEndpoints() {
		if err := validateOIDCURL(cfg.OIDCCfg.AuthorizationURL); err != nil {
			return nil, fmt.Errorf("validating OIDC_AUTHORIZATION_URL: %w", err)
		}
		if err := validateOIDCURL(cfg.OIDCCfg.TokenURL); err != nil {
			return nil, fmt.Errorf("validating OIDC_TOKEN_URL: %w", err)
		}
		if cfg.OIDCCfg.JWKSURL != "" {
			if err := validateOIDCURL(cfg.OIDCCfg.JWKSURL); err != nil {
				return nil, fmt.Errorf("validating OIDC_JWKS_URL: %w", err)
			}
		}
	}
	if cfg.OIDCCfg.EndSessionURL != "" {
		if err := validateOIDCURL(cfg.OIDCCfg.EndSessionURL); err != nil {
			return nil, fmt.Errorf("validating OIDC_END_SESSION_URL: %w", err)
		}
	}

	scopes := cfg.OIDCCfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{gooidc.ScopeOpenID, "profile", "email"}
	}

	// Instrumented HTTP client for all outbound OIDC calls (discovery, JWKS, token exchange).
	// The base transport re-validates resolved IPs at dial time to prevent DNS
	// rebinding attacks against the configured OIDC endpoints (see oidcBaseTransport).
	httpClient := &http.Client{
		Timeout:   30 * time.Second,
		Transport: otelhttp.NewTransport(oidcBaseTransport()),
	}
	// go-oidc and golang.org/x/oauth2 both read the HTTP client from context.
	ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)

	var (
		provider           *gooidc.Provider
		verifier           *gooidc.IDTokenVerifier
		endpoint           oauth2.Endpoint
		endSessionEndpoint string
	)

	if cfg.OIDCCfg.ManualEndpoints() {
		// Manual endpoint configuration — skip auto-discovery.
		endpoint = oauth2.Endpoint{
			AuthURL:  cfg.OIDCCfg.AuthorizationURL,
			TokenURL: cfg.OIDCCfg.TokenURL,
		}
		endSessionEndpoint = cfg.OIDCCfg.EndSessionURL

		// JWKS URL is required for token verification in manual mode.
		if cfg.OIDCCfg.JWKSURL == "" {
			return nil, errors.New("OIDC_JWKS_URL is required when using manual OIDC endpoints")
		}
		keySet := gooidc.NewRemoteKeySet(ctx, cfg.OIDCCfg.JWKSURL)
		verifier = gooidc.NewVerifier(cfg.OIDCCfg.ProviderURL, keySet, &gooidc.Config{
			ClientID: cfg.OIDCCfg.ClientID,
		})
	} else {
		// Auto-discovery from well-known endpoint with retry.
		// Transient network failures (e.g. identity provider restarting) should
		// not permanently disable OIDC login until the next container restart.
		var lastErr error
		for attempt := range discoveryMaxRetries {
			provider, lastErr = gooidc.NewProvider(ctx, cfg.OIDCCfg.ProviderURL)
			if lastErr == nil {
				break
			}
			if attempt < discoveryMaxRetries-1 {
				backoff := discoveryBaseDelay << attempt // 1s, 2s, 4s
				cfg.Logger.Warn("OIDC provider discovery failed, retrying",
					"error", lastErr, "attempt", attempt+1, "backoff", backoff)
				select {
				case <-time.After(backoff):
				case <-ctx.Done():
					return nil, fmt.Errorf("discovering OIDC provider: %w", ctx.Err())
				}
			}
		}
		if lastErr != nil {
			return nil, fmt.Errorf("discovering OIDC provider after %d attempts: %w", discoveryMaxRetries, lastErr)
		}
		endpoint = provider.Endpoint()
		verifier = provider.Verifier(&gooidc.Config{ClientID: cfg.OIDCCfg.ClientID})

		// Extract end_session_endpoint from discovery document for RP-Initiated Logout.
		// Manual config override takes precedence if set.
		if cfg.OIDCCfg.EndSessionURL != "" {
			endSessionEndpoint = cfg.OIDCCfg.EndSessionURL
		} else {
			var discoveryClaims struct {
				EndSessionEndpoint string `json:"end_session_endpoint"`
			}
			if err := provider.Claims(&discoveryClaims); err == nil && discoveryClaims.EndSessionEndpoint != "" {
				endSessionEndpoint = discoveryClaims.EndSessionEndpoint
			}
		}
	}

	oauth2Config := oauth2.Config{
		ClientID:     cfg.OIDCCfg.ClientID,
		ClientSecret: cfg.OIDCCfg.ClientSecret,
		RedirectURL:  cfg.OIDCCfg.RedirectURL,
		Endpoint:     endpoint,
		Scopes:       scopes,
	}

	if endSessionEndpoint != "" {
		cfg.Logger.Info("OIDC RP-Initiated Logout enabled", "end_session_endpoint", endSessionEndpoint)
	}

	return &Handler{
		provider:            provider,
		oauth2Config:        oauth2Config,
		verifier:            verifier,
		httpClient:          httpClient,
		store:               cfg.SessionStore,
		repo:                cfg.Repo,
		tokenRevoker:        cfg.TokenRevoker,
		logger:              cfg.Logger,
		adminGroups:         cfg.OIDCCfg.AdminGroups,
		bootstrapAdminEmail: strings.ToLower(strings.TrimSpace(cfg.OIDCCfg.BootstrapAdminEmail)),
		providerURL:         cfg.OIDCCfg.ProviderURL,
		endSessionEndpoint:  endSessionEndpoint,
		appURL:              cfg.AppURL,
	}, nil
}

const sessionName = "documcp_session"

// idTokenSessionName is a separate cookie for the raw ID token used by
// RP-Initiated Logout. Stored separately because the ID token JWT (~1500
// bytes) combined with OAuth pending request state would exceed the browser's
// ~4096-byte per-cookie limit and get silently dropped.
const idTokenSessionName = "documcp_idt" //nolint:gosec // session cookie name, not a credential

// discoveryMaxRetries and discoveryBaseDelay control OIDC provider discovery
// retry behavior. With exponential backoff (1s, 2s, 4s) the total wait is ~7s
// before giving up. Variables (not constants) so tests can override them.
var (
	discoveryMaxRetries = 4
	discoveryBaseDelay  = 1 * time.Second
)

// validateOIDCURL checks an operator-configured OIDC URL against the SSRF
// block-list. It is a package-level variable so tests that spin up httptest
// servers (bound to 127.0.0.1) can replace it with a no-op. Production code
// must never reassign this — the default enforces loopback/link-local blocks
// while still permitting private RFC-1918 ranges for homelab deployments.
var validateOIDCURL = func(url string) error {
	return security.ValidateExternalURL(url, true)
}

// oidcBaseTransport returns the base http.RoundTripper used by the OIDC HTTP
// client. It is wrapped with otelhttp for tracing inside New(). The default
// is SafeTransportAllowPrivate which re-validates resolved IPs at dial time
// to prevent DNS rebinding. Tests override this so they can reach httptest
// servers bound to 127.0.0.1.
var oidcBaseTransport = func() http.RoundTripper {
	return security.SafeTransportAllowPrivate(10 * time.Second)
}

// Login handles GET /auth/login — redirects to the OIDC provider.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	state, err := generateState()
	if err != nil {
		h.logger.Error("generating OIDC state", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	session, err := h.store.Get(r, sessionName)
	if err != nil {
		// Stale/corrupt cookie — gorilla returns a fresh empty session; proceed
		// so the user can log in (the Save below will overwrite the bad cookie).
		h.logger.Warn("session decode error in login, using fresh session", "error", err)
	}
	nonce, err := generateState() // reuse same 32-byte random generation
	if err != nil {
		h.logger.Error("generating OIDC nonce", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	session.Values["oidc_state"] = state
	session.Values["oidc_nonce"] = nonce

	// Preserve redirect destination (validated to prevent open redirect).
	redirect := r.URL.Query().Get("redirect")
	if redirect != "" && isSafeRedirect(redirect) {
		session.Values["oidc_redirect"] = redirect
	}

	if err := session.Save(r, w); err != nil {
		h.logger.Error("saving session", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, h.oauth2Config.AuthCodeURL(state, oauth2.SetAuthURLParam("nonce", nonce)), http.StatusFound)
}

// Callback handles GET /auth/callback — processes the OIDC callback.
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	session, err := h.store.Get(r, sessionName)
	if err != nil {
		// Stale/corrupt cookie — state will be empty, verification below fails with 400.
		h.logger.Warn("session decode error in callback", "error", err)
	}

	// Verify state (timing-safe comparison to prevent timing attacks).
	expectedState, _ := session.Values["oidc_state"].(string)
	actualState := r.URL.Query().Get("state")
	if expectedState == "" || subtle.ConstantTimeCompare([]byte(actualState), []byte(expectedState)) != 1 {
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}

	// RFC 9207: validate issuer parameter to prevent mix-up attacks.
	if iss := r.URL.Query().Get("iss"); iss != "" && iss != h.providerURL {
		h.logger.Warn("OIDC callback issuer mismatch", "expected", h.providerURL, "got", iss)
		http.Error(w, "Issuer mismatch", http.StatusBadRequest)
		return
	}

	expectedNonce, _ := session.Values["oidc_nonce"].(string)
	delete(session.Values, "oidc_state")
	delete(session.Values, "oidc_nonce")

	// Exchange code for token (use instrumented HTTP client for tracing).
	exchangeCtx := context.WithValue(r.Context(), oauth2.HTTPClient, h.httpClient)
	oauth2Token, err := h.oauth2Config.Exchange(exchangeCtx, r.URL.Query().Get("code"))
	if err != nil {
		h.logger.Error("exchanging OIDC code", "error", err)
		http.Error(w, "Authentication failed", http.StatusUnauthorized)
		return
	}

	// Verify ID token
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		h.logger.Error("no id_token in OIDC response")
		http.Error(w, "Authentication failed", http.StatusUnauthorized)
		return
	}

	idToken, err := h.verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		h.logger.Error("verifying ID token", "error", err)
		http.Error(w, "Authentication failed", http.StatusUnauthorized)
		return
	}

	// Extract claims. EmailVerified is consulted only for bootstrap-admin
	// promotion (see findOrCreateUser). Identity is `sub`; email is display.
	var claims struct {
		Sub           string   `json:"sub"`
		Email         string   `json:"email"`
		EmailVerified bool     `json:"email_verified"`
		Name          string   `json:"name"`
		Groups        []string `json:"groups"`
		Nonce         string   `json:"nonce"`
	}
	if err = idToken.Claims(&claims); err != nil {
		h.logger.Error("parsing ID token claims", "error", err)
		http.Error(w, "Authentication failed", http.StatusUnauthorized)
		return
	}

	// Verify nonce to prevent ID token replay attacks (OIDC Core 3.1.3.7).
	if expectedNonce == "" || subtle.ConstantTimeCompare([]byte(claims.Nonce), []byte(expectedNonce)) != 1 {
		h.logger.Error("OIDC nonce mismatch", "expected_present", expectedNonce != "", "got_present", claims.Nonce != "")
		http.Error(w, "Authentication failed", http.StatusUnauthorized)
		return
	}

	if claims.Sub == "" {
		h.logger.Error("OIDC token missing sub claim")
		http.Error(w, "Authentication failed", http.StatusBadRequest)
		return
	}

	// Find or create user — lookup strictly by sub; no email fallback.
	user, err := h.findOrCreateUser(r.Context(), claims.Sub, claims.Email, claims.EmailVerified, claims.Name, claims.Groups)
	if err != nil {
		h.logger.Error("finding or creating user", "error", err)
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		return
	}

	// Regenerate session to prevent session fixation attacks:
	// preserve redirect, expire old session, then create a new one.
	redirect, _ := session.Values["oidc_redirect"].(string)

	session.Options.MaxAge = -1
	if err := session.Save(r, w); err != nil {
		h.logger.Error("expiring old session", "error", err)
	}

	session, _ = h.store.New(r, sessionName)
	session.Values["user_id"] = user.ID
	session.Values["is_admin"] = user.IsAdmin
	session.Values["user_email"] = user.Email
	// login_at anchors the session's absolute lifetime (security M1).
	// BearerOrSession / SessionAuth compare against OAUTH_SESSION_ABSOLUTE_MAX_AGE
	// to reject stale sessions regardless of sliding-cookie activity.
	session.Values[authmiddleware.LoginAtSessionKey] = time.Now().Unix()

	if err := session.Save(r, w); err != nil {
		h.logger.Error("saving session", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Store the raw ID token in a separate cookie to keep the main session
	// small. The ID token JWT (~1500 bytes) is only needed for RP-Initiated
	// Logout (id_token_hint parameter).
	idtSession, _ := h.store.New(r, idTokenSessionName)
	idtSession.Values["id_token"] = rawIDToken
	if err := idtSession.Save(r, w); err != nil {
		// Non-fatal — logout will work without id_token_hint, just won't
		// terminate the provider session.
		h.logger.Warn("saving id_token session", "error", err)
	}

	if redirect == "" || !isSafeRedirect(redirect) {
		redirect = "/admin"
	}
	http.Redirect(w, r, redirect, http.StatusFound)
}

// Logout handles POST /auth/logout — clears the local session and returns
// JSON with a redirect URL. When RP-Initiated Logout is configured (the OIDC
// provider exposes end_session_endpoint), the redirect URL points to the
// provider's logout endpoint with id_token_hint and post_logout_redirect_uri
// so the provider session is also terminated. Otherwise falls back to "/".
//
// Query param `revoke_oauth=true` opts into session-L7 behavior: every live
// OAuth access + refresh token for the session user minted at or after
// login_at is revoked before the cookie is cleared. Tokens minted before the
// current session are left alone — they belong to earlier, still-valid grants.
// Requires TokenRevoker to be configured (nil disables the feature).
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	session, err := h.store.Get(r, sessionName)
	if err != nil {
		h.logger.Warn("session decode error in logout, clearing cookie", "error", err)
	}

	// Retrieve the ID token from its dedicated cookie — needed for id_token_hint.
	var rawIDToken string
	if idtSession, idtErr := h.store.Get(r, idTokenSessionName); idtErr == nil {
		rawIDToken, _ = idtSession.Values["id_token"].(string)
		idtSession.Options.MaxAge = -1
		_ = idtSession.Save(r, w)
	}

	if r.URL.Query().Get("revoke_oauth") == "true" {
		h.revokeSessionTokens(r.Context(), session)
	}

	session.Options.MaxAge = -1
	_ = session.Save(r, w)

	redirectURL := "/"
	if h.endSessionEndpoint != "" {
		redirectURL = h.buildEndSessionURL(rawIDToken)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"redirect_url": redirectURL})
}

// revokeSessionTokens implements security L7: revoke live OAuth tokens minted
// by the session user since login_at. Silent when prerequisites are missing
// (no revoker configured, no login_at anchor, no user_id) — opt-in feature
// that must not block logout.
func (h *Handler) revokeSessionTokens(ctx context.Context, session *sessions.Session) {
	if h.tokenRevoker == nil || session == nil {
		return
	}
	userID, hasUser := session.Values["user_id"].(int64)
	loginAt, hasAnchor := session.Values[authmiddleware.LoginAtSessionKey].(int64)
	if !hasUser || !hasAnchor || userID == 0 {
		return
	}
	revoked, err := h.tokenRevoker.RevokeUserTokensSince(ctx, userID, time.Unix(loginAt, 0))
	if err != nil {
		h.logger.Warn("revoking session-minted oauth tokens on logout",
			"user_id", userID, "error", err)
		return
	}
	if revoked > 0 {
		h.logger.Info("revoked session-minted oauth tokens on logout",
			"user_id", userID, "access_tokens_revoked", revoked)
	}
}

// buildEndSessionURL constructs the RP-Initiated Logout URL per
// OpenID Connect RP-Initiated Logout 1.0.
func (h *Handler) buildEndSessionURL(idTokenHint string) string {
	u, err := url.Parse(h.endSessionEndpoint)
	if err != nil {
		h.logger.Error("parsing end_session_endpoint", "error", err)
		return "/"
	}

	q := u.Query()
	if idTokenHint != "" {
		q.Set("id_token_hint", idTokenHint)
	}
	if h.appURL != "" {
		q.Set("post_logout_redirect_uri", h.appURL)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// findOrCreateUser looks up a user by OIDC sub, syncing profile fields from
// current claims. When the sub is not found a new user is created.
//
// Admin resolution order on the create-new branch:
//  1. AdminGroups — if configured and the user is in one, they are admin.
//  2. BootstrapAdminEmail — if groups did not grant admin and the user's
//     verified email matches the bootstrap address, they are admin. Fires
//     only on first creation; returning users are never re-bootstrapped.
//  3. Otherwise the user is a regular non-admin.
//
// Email-based lookup of existing local records is intentionally absent — that
// was the admin-takeover vector closed in v0.21.0.
func (h *Handler) findOrCreateUser(ctx context.Context, sub, email string, emailVerified bool, name string, groups []string) (*model.User, error) {
	// Sync-by-sub path.
	user, err := h.repo.FindUserByOIDCSub(ctx, sub)
	if err == nil {
		needsUpdate := user.Name != name || user.Email != email
		if isAdmin, shouldSync := h.resolveAdmin(groups); shouldSync && user.IsAdmin != isAdmin {
			h.logger.Info("synced admin status from OIDC groups",
				"user_id", user.ID, "email", user.Email,
				"is_admin", isAdmin, "groups", groups,
			)
			user.IsAdmin = isAdmin
			needsUpdate = true
		}
		if needsUpdate {
			user.Name = name
			user.Email = email
			if updateErr := h.repo.UpdateUser(ctx, user); updateErr != nil {
				// Admin demotion/promotion + identity-field sync not persisted.
				// Not fatal — session still works with the stale row — but next
				// login will try again. Warn so operators see a persistent
				// failure rather than silent drift.
				h.logger.Warn("updating user on oidc callback",
					"user_id", user.ID, "error", updateErr)
			}
		}
		return user, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("looking up user by oidc_sub: %w", err)
	}

	// Create-new path — decide IsAdmin from groups first, bootstrap second.
	isAdmin := false
	adminSource := "default"
	if admin, shouldSync := h.resolveAdmin(groups); shouldSync && admin {
		isAdmin = true
		adminSource = "groups"
	} else if h.bootstrapAdminEmail != "" && emailVerified &&
		strings.EqualFold(strings.TrimSpace(email), h.bootstrapAdminEmail) {
		isAdmin = true
		adminSource = "bootstrap_email"
	}

	user = &model.User{
		Name:            name,
		Email:           email,
		OIDCSub:         sql.NullString{String: sub, Valid: true},
		OIDCProvider:    sql.NullString{String: h.providerURL, Valid: true},
		EmailVerifiedAt: sql.NullTime{Time: time.Now(), Valid: emailVerified},
		IsAdmin:         isAdmin,
	}
	if err := h.repo.CreateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}

	h.logger.Info("created new user from OIDC login",
		"email", email,
		"user_id", user.ID,
		"is_admin", isAdmin,
		"admin_source", adminSource,
	)

	return user, nil
}

// resolveAdmin determines if a user should be admin based on their OIDC groups.
// Returns (isAdmin, shouldSync). ShouldSync is false when adminGroups is not
// configured, meaning IsAdmin should not be touched (manual assignment preserved).
func (h *Handler) resolveAdmin(groups []string) (isAdmin, shouldSync bool) {
	if len(h.adminGroups) == 0 {
		return false, false
	}
	for _, g := range groups {
		if slices.Contains(h.adminGroups, g) {
			return true, true
		}
	}
	return false, true
}

// isSafeRedirect validates that a redirect path is a same-origin relative path
// to prevent open redirect attacks.
func isSafeRedirect(redirect string) bool {
	if redirect == "" {
		return false
	}
	// Must start with / and second char must not be / or \ (prevents
	// protocol-relative URLs like //evil.com and \/evil.com). Explicit
	// second-char check satisfies static analysis (CodeQL go/bad-redirect-check)
	// in addition to the broader Contains checks below.
	if redirect[0] != '/' {
		return false
	}
	if len(redirect) > 1 && (redirect[1] == '/' || redirect[1] == '\\') {
		return false
	}
	// Must not contain // anywhere (protocol-relative URL)
	if strings.Contains(redirect, "//") {
		return false
	}
	// Must not contain backslash anywhere (some browsers normalize \\ to //)
	if strings.Contains(redirect, "\\") {
		return false
	}
	return true
}

func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
