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
	apihandler "git.999.haus/chris/DocuMCP-go/internal/handler/api"
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

	// Phase 3: Document pipeline & search
	DocumentHandler *apihandler.DocumentHandler // nil if not configured
	SearchHandler   *apihandler.SearchHandler   // nil if Meilisearch not configured

	// Phase 4: External service clients & REST API
	ZimHandler             *apihandler.ZimHandler
	ConfluenceHandler      *apihandler.ConfluenceHandler
	GitTemplateHandler     *apihandler.GitTemplateHandler
	ExternalServiceHandler *apihandler.ExternalServiceHandler
	UserHandler            *apihandler.UserHandler
	OAuthClientHandler     *apihandler.OAuthClientHandler
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

	// REST API
	r.Route("/api", func(r chi.Router) {
		// Document endpoints
		if deps.DocumentHandler != nil {
			r.Route("/documents", func(r chi.Router) {
				r.Get("/", deps.DocumentHandler.List)
				r.Post("/", deps.DocumentHandler.Upload)
				r.Get("/{uuid}", deps.DocumentHandler.Show)
				r.Put("/{uuid}", deps.DocumentHandler.Update)
				r.Delete("/{uuid}", deps.DocumentHandler.Delete)
			})
			s.logger.Info("document API endpoints registered")
		}

		// Search endpoints
		if deps.SearchHandler != nil {
			r.Get("/search", deps.SearchHandler.Search)
			r.Get("/search/unified", deps.SearchHandler.FederatedSearch)
			s.logger.Info("search API endpoints registered")
		}

		// ZIM archive endpoints
		if deps.ZimHandler != nil {
			r.Route("/zim/archives", func(r chi.Router) {
				r.Get("/", deps.ZimHandler.List)
				r.Get("/{archive}", deps.ZimHandler.Show)
				r.Get("/{archive}/search", deps.ZimHandler.Search)
				r.Get("/{archive}/suggest", deps.ZimHandler.Suggest)
				r.Get("/{archive}/articles/*", deps.ZimHandler.ReadArticle)
			})
			s.logger.Info("ZIM API endpoints registered")
		}

		// Confluence endpoints
		if deps.ConfluenceHandler != nil {
			r.Route("/confluence", func(r chi.Router) {
				r.Get("/spaces", deps.ConfluenceHandler.ListSpaces)
				r.Get("/spaces/{key}", deps.ConfluenceHandler.ShowSpace)
				r.Get("/pages/search", deps.ConfluenceHandler.SearchPages)
				r.Get("/pages/{id}", deps.ConfluenceHandler.ReadPage)
			})
			s.logger.Info("Confluence API endpoints registered")
		}

		// Git template endpoints
		if deps.GitTemplateHandler != nil {
			r.Route("/git-templates", func(r chi.Router) {
				r.Get("/", deps.GitTemplateHandler.List)
				r.Post("/", deps.GitTemplateHandler.Create)
				r.Get("/search", deps.GitTemplateHandler.Search)
				r.Get("/{uuid}", deps.GitTemplateHandler.Show)
				r.Put("/{uuid}", deps.GitTemplateHandler.Update)
				r.Delete("/{uuid}", deps.GitTemplateHandler.Delete)
				r.Get("/{uuid}/structure", deps.GitTemplateHandler.Structure)
				r.Get("/{uuid}/files/*", deps.GitTemplateHandler.ReadFile)
				r.Get("/{uuid}/deployment-guide", deps.GitTemplateHandler.DeploymentGuide)
				r.Post("/{uuid}/download", deps.GitTemplateHandler.Download)
			})
			s.logger.Info("Git template API endpoints registered")
		}

		// External service endpoints
		if deps.ExternalServiceHandler != nil {
			r.Route("/external-services", func(r chi.Router) {
				r.Get("/", deps.ExternalServiceHandler.List)
				r.Post("/", deps.ExternalServiceHandler.Create)
				r.Get("/{uuid}", deps.ExternalServiceHandler.Show)
				r.Put("/{uuid}", deps.ExternalServiceHandler.Update)
				r.Delete("/{uuid}", deps.ExternalServiceHandler.Delete)
				r.Post("/{uuid}/health-check", deps.ExternalServiceHandler.HealthCheck)
			})
			s.logger.Info("External service API endpoints registered")
		}
	})

	// Admin UI and management
	r.Route("/admin", func(r chi.Router) {
		// Admin API endpoints
		if deps.UserHandler != nil {
			r.Route("/users", func(r chi.Router) {
				r.Get("/", deps.UserHandler.List)
				r.Get("/{uuid}", deps.UserHandler.Show)
			})
			s.logger.Info("Admin user endpoints registered")
		}
		if deps.OAuthClientHandler != nil {
			r.Route("/oauth-clients", func(r chi.Router) {
				r.Get("/", deps.OAuthClientHandler.List)
				r.Get("/{id}", deps.OAuthClientHandler.Show)
			})
			s.logger.Info("Admin OAuth client endpoints registered")
		}
	})
}
