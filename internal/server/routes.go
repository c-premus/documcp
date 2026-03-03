package server

import (
	"crypto/subtle"
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"

	authmiddleware "git.999.haus/chris/DocuMCP-go/internal/auth/middleware"
	"git.999.haus/chris/DocuMCP-go/internal/auth/oauth"
	"git.999.haus/chris/DocuMCP-go/internal/auth/oidc"
	"git.999.haus/chris/DocuMCP-go/internal/handler"
	adminhandler "git.999.haus/chris/DocuMCP-go/internal/handler/admin"
	apihandler "git.999.haus/chris/DocuMCP-go/internal/handler/api"
	oauthhandler "git.999.haus/chris/DocuMCP-go/internal/handler/oauth"
	"git.999.haus/chris/DocuMCP-go/internal/observability"
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

	// Phase 5: Admin UI
	AdminHandler *adminhandler.Handler

	// Observability
	Metrics      *observability.Metrics // nil disables Prometheus metrics
	OTELEnabled  bool                   // enables tracing middleware

	// Security
	CSRFKey  []byte // 32-byte key for CSRF token generation (nil disables CSRF)
	IsSecure bool   // true when running behind TLS (sets Secure cookie flag)

	// Infrastructure
	DB               *sql.DB // for readiness checks (nil disables /health/ready)
	InternalAPIToken string  // protects /metrics and /health/ready (empty = unrestricted)
}

// RegisterRoutes configures all middleware and route groups on the server.
func (s *Server) RegisterRoutes(deps Deps) {
	r := s.router

	// Built-in chi middleware
	r.Use(middleware.RequestID)
	r.Use(RealIP(s.trustedProxies))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// OpenTelemetry tracing middleware
	if deps.OTELEnabled {
		r.Use(observability.Tracing("documcp"))
	}

	// Prometheus metrics middleware (before application middleware so it
	// captures the full request lifecycle including logging overhead).
	if deps.Metrics != nil {
		r.Use(observability.MetricsMiddleware(deps.Metrics))
	}

	// Application middleware
	r.Use(SecurityHeaders)
	r.Use(MaxBodySize(1 * 1024 * 1024)) // 1 MB default body limit (excludes multipart)
	r.Use(RequestLogger(s.logger))

	// Health check (liveness — cheap, no I/O)
	health := handler.NewHealthHandler(deps.Version)
	r.Method(http.MethodGet, "/health", health)

	// Readiness probe (checks dependencies like Postgres)
	if deps.DB != nil {
		readiness := handler.NewReadinessHandler(deps.Version, deps.DB)
		r.Method(http.MethodGet, "/health/ready", readiness)
	}

	// Prometheus metrics endpoint (protected by internal API token if configured)
	if deps.Metrics != nil {
		r.Group(func(r chi.Router) {
			if deps.InternalAPIToken != "" {
				r.Use(internalTokenAuth(deps.InternalAPIToken))
			}
			r.Method(http.MethodGet, "/metrics", observability.MetricsHandler())
		})
		s.logger.Info("Prometheus metrics endpoint registered", "path", "/metrics")
	}

	// MCP endpoint (protected by bearer token when OAuth is configured)
	if deps.MCPHandler != nil {
		r.Route("/documcp", func(r chi.Router) {
			if deps.OAuthService != nil {
				r.Use(authmiddleware.BearerToken(deps.OAuthService))
			}
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
			// CSRF protection for state-changing OAuth forms (consent, device approval).
			if len(deps.CSRFKey) > 0 {
				r.Use(csrf.Protect(
					deps.CSRFKey,
					csrf.Secure(deps.IsSecure),
					csrf.Path("/oauth"),
					csrf.RequestHeader("X-CSRF-Token"),
				))
			}
			r.Get("/authorize", deps.OAuthHandler.Authorize)
			r.Post("/authorize/approve", deps.OAuthHandler.AuthorizeApprove)

			// Rate-limited token endpoint (brute force prevention)
			r.Group(func(r chi.Router) {
				r.Use(httprate.LimitByIP(100, time.Minute))
				r.Post("/token", deps.OAuthHandler.Token)
				r.Post("/revoke", deps.OAuthHandler.Revoke)
			})

			// Rate-limited registration endpoint
			r.Group(func(r chi.Router) {
				r.Use(httprate.LimitByIP(10, time.Minute))
				r.Post("/register", deps.OAuthHandler.Register)
			})

			// Rate-limited device code endpoint
			r.Group(func(r chi.Router) {
				r.Use(httprate.LimitByIP(20, time.Minute))
				r.Post("/device/code", deps.OAuthHandler.DeviceAuthorization)
			})

			r.Get("/device", deps.OAuthHandler.DeviceVerification)
			r.Post("/device", deps.OAuthHandler.DeviceVerificationSubmit)
			r.Post("/device/approve", deps.OAuthHandler.DeviceApprove)
		})
		s.logger.Info("OAuth endpoints registered")
	}

	// OIDC auth endpoints
	if deps.OIDCHandler != nil {
		r.Route("/auth", func(r chi.Router) {
			r.Use(httprate.LimitByIP(30, time.Minute))
			r.Get("/login", deps.OIDCHandler.Login)
			r.Get("/callback", deps.OIDCHandler.Callback)
			r.Post("/logout", deps.OIDCHandler.Logout)
		})
		s.logger.Info("OIDC auth endpoints registered")
	}

	// REST API (protected by bearer token when OAuth is configured)
	r.Route("/api", func(r chi.Router) {
		r.Use(httprate.LimitByIP(300, time.Minute))
		if deps.OAuthService != nil {
			r.Use(authmiddleware.BearerToken(deps.OAuthService))
		}

		// Document endpoints
		if deps.DocumentHandler != nil {
			r.Route("/documents", func(r chi.Router) {
				r.Get("/", deps.DocumentHandler.List)
				r.Post("/", deps.DocumentHandler.Upload)
				r.Post("/analyze", deps.DocumentHandler.Analyze)
				r.Get("/{uuid}", deps.DocumentHandler.Show)
				r.Put("/{uuid}", deps.DocumentHandler.Update)
				r.Delete("/{uuid}", deps.DocumentHandler.Delete)
				r.Get("/{uuid}/download", deps.DocumentHandler.Download)
			})
			s.logger.Info("document API endpoints registered")
		}

		// Search endpoints
		if deps.SearchHandler != nil {
			r.Get("/search", deps.SearchHandler.Search)
			r.Get("/search/unified", deps.SearchHandler.FederatedSearch)
			r.Get("/search/popular", deps.SearchHandler.Popular)
			r.Get("/search/autocomplete", deps.SearchHandler.Autocomplete)
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
				r.Post("/{uuid}/sync", deps.GitTemplateHandler.Sync)
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

	// Admin UI
	if deps.AdminHandler != nil {
		// Login page — no auth required
		r.Get("/admin/login", deps.AdminHandler.Login)

		r.Route("/admin", func(r chi.Router) {
			// Protect all admin routes with session auth + admin check
			if deps.SessionStore != nil && deps.OAuthService != nil {
				r.Use(authmiddleware.SessionAuth(deps.SessionStore, deps.OAuthService))
				r.Use(authmiddleware.RequireAdmin)
			}

			// CSRF protection for all admin state-changing requests.
			// htmx sends the token via X-CSRF-Token header (configured in layout).
			if len(deps.CSRFKey) > 0 {
				r.Use(csrf.Protect(
					deps.CSRFKey,
					csrf.Secure(deps.IsSecure),
					csrf.Path("/admin"),
					csrf.RequestHeader("X-CSRF-Token"),
				))
				// Inject CSRF token into every admin response via a cookie that
				// the layout template reads and sets as an htmx header.
				r.Use(func(next http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("X-CSRF-Token", csrf.Token(r))
						next.ServeHTTP(w, r)
					})
				})
			}

			// Dashboard
			r.Get("/", deps.AdminHandler.Dashboard)

			// Documents
			r.Route("/documents", func(r chi.Router) {
				r.Get("/", deps.AdminHandler.DocumentList)
				r.Post("/", deps.AdminHandler.DocumentUpload)
				r.Get("/{uuid}", deps.AdminHandler.DocumentDetail)
				r.Delete("/{uuid}", deps.AdminHandler.DocumentDelete)
			})

			// Users
			r.Route("/users", func(r chi.Router) {
				r.Get("/", deps.AdminHandler.UserList)
				r.Get("/{id}", deps.AdminHandler.UserDetail)
				r.Post("/{id}/toggle-admin", deps.AdminHandler.UserToggleAdmin)
			})

			// OAuth Clients
			r.Route("/oauth-clients", func(r chi.Router) {
				r.Get("/", deps.AdminHandler.OAuthClientList)
				r.Post("/", deps.AdminHandler.OAuthClientCreate)
				r.Get("/{id}", deps.AdminHandler.OAuthClientDetail)
				r.Post("/{id}/revoke", deps.AdminHandler.OAuthClientRevoke)
			})

			// External Services
			r.Route("/external-services", func(r chi.Router) {
				r.Get("/", deps.AdminHandler.ExternalServiceList)
				r.Post("/", deps.AdminHandler.ExternalServiceCreate)
				r.Get("/{uuid}", deps.AdminHandler.ExternalServiceDetail)
				r.Post("/{uuid}/health-check", deps.AdminHandler.ExternalServiceHealthCheck)
				r.Delete("/{uuid}", deps.AdminHandler.ExternalServiceDelete)
			})

			// ZIM Archives
			r.Route("/zim-archives", func(r chi.Router) {
				r.Get("/", deps.AdminHandler.ZimArchiveList)
				r.Post("/{uuid}/toggle-enabled", deps.AdminHandler.ZimArchiveToggleEnabled)
				r.Post("/{uuid}/toggle-searchable", deps.AdminHandler.ZimArchiveToggleSearchable)
			})

			// Confluence Spaces
			r.Route("/confluence-spaces", func(r chi.Router) {
				r.Get("/", deps.AdminHandler.ConfluenceSpaceList)
				r.Post("/{uuid}/toggle-enabled", deps.AdminHandler.ConfluenceSpaceToggleEnabled)
				r.Post("/{uuid}/toggle-searchable", deps.AdminHandler.ConfluenceSpaceToggleSearchable)
			})

			// Git Templates
			r.Route("/git-templates", func(r chi.Router) {
				r.Get("/", deps.AdminHandler.GitTemplateList)
				r.Post("/", deps.AdminHandler.GitTemplateCreate)
				r.Get("/{uuid}", deps.AdminHandler.GitTemplateDetail)
				r.Delete("/{uuid}", deps.AdminHandler.GitTemplateDelete)
			})
		})
		s.logger.Info("Admin UI endpoints registered")
	}
}

// internalTokenAuth returns a middleware that requires a bearer token matching
// the configured internal API token. Used to protect operational endpoints
// like /metrics and /health/ready.
func internalTokenAuth(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			provided := strings.TrimPrefix(auth, "Bearer ")
			if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
