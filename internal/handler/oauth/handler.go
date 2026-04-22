// Package oauthhandler implements HTTP handlers for the OAuth 2.1 authorization server.
package oauthhandler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gorilla/sessions"

	"github.com/c-premus/documcp/internal/auth/oauth"
	"github.com/c-premus/documcp/internal/config"
)

// Handler holds dependencies for all OAuth HTTP handlers.
type Handler struct {
	service        *oauth.Service
	store          sessions.Store
	oauthCfg       config.OAuthConfig
	appURL         string
	logger         *slog.Logger
	deviceFailures *oauth.DeviceFailureLimiter
}

// Config holds the dependencies for creating a new Handler.
type Config struct {
	Service      *oauth.Service
	SessionStore sessions.Store
	OAuthCfg     config.OAuthConfig
	AppURL       string
	Logger       *slog.Logger
	// DeviceFailureLimiter enforces per-user brute-force limits on the
	// device-verification submit endpoint (security L6). Nil collapses to a
	// no-op limiter for tests that don't exercise the counter.
	DeviceFailureLimiter *oauth.DeviceFailureLimiter
}

// New creates a new OAuth handler.
func New(cfg Config) *Handler {
	limiter := cfg.DeviceFailureLimiter
	if limiter == nil {
		limiter = oauth.NewDeviceFailureLimiter(nil, 0, 0)
	}
	return &Handler{
		service:        cfg.Service,
		store:          cfg.SessionStore,
		oauthCfg:       cfg.OAuthCfg,
		appURL:         cfg.AppURL,
		logger:         cfg.Logger,
		deviceFailures: limiter,
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
