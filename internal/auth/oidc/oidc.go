// Package oidc implements OIDC authentication for admin login via an external identity provider.
package oidc

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/gob"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"

	"git.999.haus/chris/DocuMCP-go/internal/config"
	"git.999.haus/chris/DocuMCP-go/internal/model"
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
	provider     *gooidc.Provider
	oauth2Config oauth2.Config
	verifier     *gooidc.IDTokenVerifier
	store        sessions.Store
	repo         UserRepo
	logger       *slog.Logger
}

// Config holds the dependencies for creating a new OIDC Handler.
type Config struct {
	OIDCCfg      config.OIDCConfig
	SessionStore sessions.Store
	Repo         UserRepo
	Logger       *slog.Logger
}

// New creates a new OIDC Handler. It discovers the provider configuration
// from the well-known endpoint. Returns nil if OIDC is not configured.
func New(ctx context.Context, cfg Config) (*Handler, error) {
	if cfg.OIDCCfg.ProviderURL == "" || cfg.OIDCCfg.ClientID == "" {
		return nil, nil
	}

	provider, err := gooidc.NewProvider(ctx, cfg.OIDCCfg.ProviderURL)
	if err != nil {
		return nil, fmt.Errorf("discovering OIDC provider: %w", err)
	}

	scopes := cfg.OIDCCfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{gooidc.ScopeOpenID, "profile", "email"}
	}

	oauth2Config := oauth2.Config{
		ClientID:     cfg.OIDCCfg.ClientID,
		ClientSecret: cfg.OIDCCfg.ClientSecret,
		RedirectURL:  cfg.OIDCCfg.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       scopes,
	}

	verifier := provider.Verifier(&gooidc.Config{ClientID: cfg.OIDCCfg.ClientID})

	return &Handler{
		provider:     provider,
		oauth2Config: oauth2Config,
		verifier:     verifier,
		store:        cfg.SessionStore,
		repo:         cfg.Repo,
		logger:       cfg.Logger,
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

	session, _ := h.store.Get(r, sessionName)
	session.Values["oidc_state"] = state

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

	http.Redirect(w, r, h.oauth2Config.AuthCodeURL(state), http.StatusFound)
}

// Callback handles GET /auth/callback — processes the OIDC callback.
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	session, _ := h.store.Get(r, sessionName)

	// Verify state
	expectedState, _ := session.Values["oidc_state"].(string)
	if r.URL.Query().Get("state") != expectedState || expectedState == "" {
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}
	delete(session.Values, "oidc_state")

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
		Sub   string `json:"sub"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		h.logger.Error("parsing ID token claims", "error", err)
		http.Error(w, "Authentication failed", http.StatusUnauthorized)
		return
	}

	if claims.Email == "" {
		http.Error(w, "Email not provided by identity provider", http.StatusBadRequest)
		return
	}

	// Find or create user
	user, err := h.findOrCreateUser(r.Context(), claims.Sub, claims.Email, claims.Name)
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
	session, _ := h.store.Get(r, sessionName)
	session.Options.MaxAge = -1
	_ = session.Save(r, w)
	http.Redirect(w, r, "/", http.StatusFound)
}

// findOrCreateUser finds a user by OIDC sub or email, or creates a new one.
func (h *Handler) findOrCreateUser(ctx context.Context, sub, email, name string) (*model.User, error) {
	// Try by OIDC sub first
	user, err := h.repo.FindUserByOIDCSub(ctx, sub)
	if err == nil {
		// Update profile if changed
		if user.Name != name || user.Email != email {
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
		user.OIDCProvider = sql.NullString{String: h.provider.Endpoint().AuthURL, Valid: true}
		if name != "" {
			user.Name = name
		}
		if err := h.repo.UpdateUser(ctx, user); err != nil {
			return nil, fmt.Errorf("linking OIDC identity: %w", err)
		}
		return user, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("looking up user by email: %w", err)
	}

	// Create new user
	user = &model.User{
		Name:            name,
		Email:           email,
		OIDCSub:         sql.NullString{String: sub, Valid: true},
		OIDCProvider:    sql.NullString{String: h.provider.Endpoint().AuthURL, Valid: true},
		EmailVerifiedAt: sql.NullTime{Time: time.Now(), Valid: true},
		IsAdmin:         false,
	}
	if err := h.repo.CreateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}

	h.logger.Info("created new user from OIDC login",
		"email", email,
		"user_id", user.ID,
	)

	return user, nil
}

// isSafeRedirect validates that a redirect path is a same-origin relative path
// to prevent open redirect attacks.
func isSafeRedirect(redirect string) bool {
	if redirect == "" {
		return false
	}
	// Must start with /
	if !strings.HasPrefix(redirect, "/") {
		return false
	}
	// Must not contain // (protocol-relative URL)
	if strings.Contains(redirect, "//") {
		return false
	}
	// Must not contain backslash (some browsers normalize \\ to //)
	if strings.Contains(redirect, "\\") {
		return false
	}
	return true
}

func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func init() {
	// gorilla/sessions uses gob encoding. Register types stored in sessions.
	gob.Register(map[string]any{})
}
