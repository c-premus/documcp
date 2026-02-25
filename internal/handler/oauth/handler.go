// Package oauthhandler implements HTTP handlers for the OAuth 2.1 authorization server.
package oauthhandler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gorilla/sessions"

	"git.999.haus/chris/DocuMCP-go/internal/auth/oauth"
	"git.999.haus/chris/DocuMCP-go/internal/config"
)

// Handler holds dependencies for all OAuth HTTP handlers.
type Handler struct {
	service      *oauth.Service
	store        sessions.Store
	oauthCfg     config.OAuthConfig
	appURL       string
	logger       *slog.Logger
}

// Config holds the dependencies for creating a new Handler.
type Config struct {
	Service      *oauth.Service
	SessionStore sessions.Store
	OAuthCfg     config.OAuthConfig
	AppURL       string
	Logger       *slog.Logger
}

// New creates a new OAuth handler.
func New(cfg Config) *Handler {
	return &Handler{
		service:      cfg.Service,
		store:        cfg.SessionStore,
		oauthCfg:     cfg.OAuthCfg,
		appURL:       cfg.AppURL,
		logger:       cfg.Logger,
	}
}

// oauthError writes an OAuth-format JSON error response.
func oauthError(w http.ResponseWriter, status int, errCode, description string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":             errCode,
		"error_description": description,
	})
}

// jsonResponse writes a JSON response.
func jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

const sessionName = "documcp_session"
