package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/sessions"

	"git.999.haus/chris/DocuMCP-go/internal/auth/oauth"
	"git.999.haus/chris/DocuMCP-go/internal/auth/oidc"
	"git.999.haus/chris/DocuMCP-go/internal/handler"
	oauthhandler "git.999.haus/chris/DocuMCP-go/internal/handler/oauth"
)

// Deps holds handler dependencies injected from the app layer.
type Deps struct {
	Version      string
	MCPHandler   http.Handler          // nil if MCP is not configured
	OAuthHandler *oauthhandler.Handler // nil if OAuth is not configured
	OIDCHandler  *oidc.Handler         // nil if OIDC is not configured
	OAuthService *oauth.Service        // for middleware (nil if OAuth not configured)
	SessionStore sessions.Store        // for middleware (nil if sessions not configured)
}

// RegisterRoutes configures all middleware and route groups on the server.
func (s *Server) RegisterRoutes(deps Deps) {
	r := s.router

	// Built-in chi middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Application middleware
	r.Use(SecurityHeaders)
	r.Use(RequestLogger(s.logger))

	// Health check
	health := handler.NewHealthHandler(deps.Version)
	r.Method(http.MethodGet, "/health", health)

	// MCP endpoint
	if deps.MCPHandler != nil {
		r.Route("/documcp", func(r chi.Router) {
			r.Handle("/*", deps.MCPHandler)
			r.Handle("/", deps.MCPHandler)
		})
		s.logger.Info("MCP endpoint registered", "path", "/documcp")
	}

	// Well-known discovery endpoints
	if deps.OAuthHandler != nil {
		r.Get("/.well-known/oauth-authorization-server", deps.OAuthHandler.AuthorizationServerMetadata)
		r.Get("/.well-known/oauth-protected-resource", deps.OAuthHandler.ProtectedResourceMetadata)
		r.Get("/.well-known/oauth-protected-resource/*", deps.OAuthHandler.ProtectedResourceMetadata)
		s.logger.Info("OAuth well-known endpoints registered")
	}

	// OAuth endpoints
	if deps.OAuthHandler != nil {
		r.Route("/oauth", func(r chi.Router) {
			r.Get("/authorize", deps.OAuthHandler.Authorize)
			r.Post("/authorize/approve", deps.OAuthHandler.AuthorizeApprove)
			r.Post("/token", deps.OAuthHandler.Token)
			r.Post("/revoke", deps.OAuthHandler.Revoke)
			r.Post("/register", deps.OAuthHandler.Register)
			r.Post("/device/code", deps.OAuthHandler.DeviceAuthorization)
			r.Get("/device", deps.OAuthHandler.DeviceVerification)
			r.Post("/device", deps.OAuthHandler.DeviceVerificationSubmit)
			r.Post("/device/approve", deps.OAuthHandler.DeviceApprove)
		})
		s.logger.Info("OAuth endpoints registered")
	}

	// OIDC auth endpoints
	if deps.OIDCHandler != nil {
		r.Route("/auth", func(r chi.Router) {
			r.Get("/login", deps.OIDCHandler.Login)
			r.Get("/callback", deps.OIDCHandler.Callback)
			r.Post("/logout", deps.OIDCHandler.Logout)
		})
		s.logger.Info("OIDC auth endpoints registered")
	}

	// REST API (TODO)
	r.Route("/api", func(r chi.Router) {
		// TODO: register API handlers
	})

	// Admin UI (TODO)
	r.Route("/admin", func(r chi.Router) {
		// TODO: register admin handlers
	})
}
