// Package oidc implements OIDC authentication for admin login via an external identity provider.
package oidc

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/gob"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"

	"github.com/c-premus/documcp/internal/config"
	"github.com/c-premus/documcp/internal/model"
)

// UserRepo defines the repository interface consumed by the OIDC handler.
type UserRepo interface {
	FindUserByOIDCSub(ctx context.Context, sub string) (*model.User, error)
	FindUserByEmail(ctx context.Context, email string) (*model.User, error)
	CreateUser(ctx context.Context, user *model.User) error
	UpdateUser(ctx context.Context, user *model.User) error
}

// Handler provides HTTP handlers for OIDC login/callback/logout.
type Handler struct {
	provider     *gooidc.Provider // nil in manual-endpoint mode
	oauth2Config oauth2.Config
	verifier     *gooidc.IDTokenVerifier
	store        sessions.Store
	repo         UserRepo
	logger       *slog.Logger
	adminGroups  []string
	providerURL  string // stored for OIDCProvider field in user records
}

// Config holds the dependencies for creating a new OIDC Handler.
type Config struct {
	OIDCCfg      config.OIDCConfig
	SessionStore sessions.Store
	Repo         UserRepo
	Logger       *slog.Logger
}

// New creates a new OIDC Handler. It discovers the provider configuration
// from the well-known endpoint, or uses manually configured endpoints when
// OIDC_AUTHORIZATION_URL and OIDC_TOKEN_URL are set (REQ-AUTH-003).
// Returns nil if OIDC is not configured.
func New(ctx context.Context, cfg Config) (*Handler, error) {
	if cfg.OIDCCfg.ProviderURL == "" || cfg.OIDCCfg.ClientID == "" {
		return nil, nil
	}

	scopes := cfg.OIDCCfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{gooidc.ScopeOpenID, "profile", "email"}
	}

	var (
		provider *gooidc.Provider
		verifier *gooidc.IDTokenVerifier
		endpoint oauth2.Endpoint
	)

	if cfg.OIDCCfg.ManualEndpoints() {
		// Manual endpoint configuration — skip auto-discovery.
		endpoint = oauth2.Endpoint{
			AuthURL:  cfg.OIDCCfg.AuthorizationURL,
			TokenURL: cfg.OIDCCfg.TokenURL,
		}

		// JWKS URL is required for token verification in manual mode.
		if cfg.OIDCCfg.JWKSURL == "" {
			return nil, errors.New("OIDC_JWKS_URL is required when using manual OIDC endpoints")
		}
		keySet := gooidc.NewRemoteKeySet(ctx, cfg.OIDCCfg.JWKSURL)
		verifier = gooidc.NewVerifier(cfg.OIDCCfg.ProviderURL, keySet, &gooidc.Config{
			ClientID: cfg.OIDCCfg.ClientID,
		})
	} else {
		// Auto-discovery from well-known endpoint.
		var err error
		provider, err = gooidc.NewProvider(ctx, cfg.OIDCCfg.ProviderURL)
		if err != nil {
			return nil, fmt.Errorf("discovering OIDC provider: %w", err)
		}
		endpoint = provider.Endpoint()
		verifier = provider.Verifier(&gooidc.Config{ClientID: cfg.OIDCCfg.ClientID})
	}

	oauth2Config := oauth2.Config{
		ClientID:     cfg.OIDCCfg.ClientID,
		ClientSecret: cfg.OIDCCfg.ClientSecret,
		RedirectURL:  cfg.OIDCCfg.RedirectURL,
		Endpoint:     endpoint,
		Scopes:       scopes,
	}

	return &Handler{
		provider:     provider,
		oauth2Config: oauth2Config,
		verifier:     verifier,
		store:        cfg.SessionStore,
		repo:         cfg.Repo,
		logger:       cfg.Logger,
		adminGroups:  cfg.OIDCCfg.AdminGroups,
		providerURL:  cfg.OIDCCfg.ProviderURL,
	}, nil
}

const sessionName = "documcp_session"

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
	expectedNonce, _ := session.Values["oidc_nonce"].(string)
	delete(session.Values, "oidc_state")
	delete(session.Values, "oidc_nonce")

	// Exchange code for token
	oauth2Token, err := h.oauth2Config.Exchange(r.Context(), r.URL.Query().Get("code"))
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

	// Extract claims
	var claims struct {
		Sub    string   `json:"sub"`
		Email  string   `json:"email"`
		Name   string   `json:"name"`
		Groups []string `json:"groups"`
		Nonce  string   `json:"nonce"`
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

	if claims.Email == "" {
		http.Error(w, "Email not provided by identity provider", http.StatusBadRequest)
		return
	}

	// Find or create user
	user, err := h.findOrCreateUser(r.Context(), claims.Sub, claims.Email, claims.Name, claims.Groups)
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

	if err := session.Save(r, w); err != nil {
		h.logger.Error("saving session", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if redirect == "" || !isSafeRedirect(redirect) {
		redirect = "/admin"
	}
	http.Redirect(w, r, redirect, http.StatusFound)
}

// Logout handles POST /auth/logout — clears the session.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	session, err := h.store.Get(r, sessionName)
	if err != nil {
		// Stale cookie — expire it and redirect.
		h.logger.Warn("session decode error in logout, clearing cookie", "error", err)
	}
	session.Options.MaxAge = -1
	_ = session.Save(r, w)
	http.Redirect(w, r, "/", http.StatusFound)
}

// findOrCreateUser finds a user by OIDC sub or email, or creates a new one.
// When adminGroups is configured, IsAdmin is synced from OIDC group membership on every login.
func (h *Handler) findOrCreateUser(ctx context.Context, sub, email, name string, groups []string) (*model.User, error) {
	// Try by OIDC sub first
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
			_ = h.repo.UpdateUser(ctx, user)
		}
		return user, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("looking up user by oidc_sub: %w", err)
	}

	// Try by email
	user, err = h.repo.FindUserByEmail(ctx, email)
	if err == nil {
		// Link OIDC identity
		user.OIDCSub = sql.NullString{String: sub, Valid: true}
		user.OIDCProvider = sql.NullString{String: h.providerURL, Valid: true}
		if name != "" {
			user.Name = name
		}
		if isAdmin, shouldSync := h.resolveAdmin(groups); shouldSync {
			user.IsAdmin = isAdmin
		}
		if err = h.repo.UpdateUser(ctx, user); err != nil {
			return nil, fmt.Errorf("linking OIDC identity: %w", err)
		}
		return user, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("looking up user by email: %w", err)
	}

	// Create new user
	isAdmin := false
	if admin, shouldSync := h.resolveAdmin(groups); shouldSync {
		isAdmin = admin
	}
	user = &model.User{
		Name:            name,
		Email:           email,
		OIDCSub:         sql.NullString{String: sub, Valid: true},
		OIDCProvider:    sql.NullString{String: h.providerURL, Valid: true},
		EmailVerifiedAt: sql.NullTime{Time: time.Now(), Valid: true},
		IsAdmin:         isAdmin,
	}
	if err := h.repo.CreateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}

	h.logger.Info("created new user from OIDC login",
		"email", email,
		"user_id", user.ID,
		"is_admin", isAdmin,
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

func init() {
	// gorilla/sessions uses gob encoding. Register types stored in sessions.
	gob.Register(map[string]any{})
}
