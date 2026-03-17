package server

import (
	"crypto/subtle"
	"database/sql"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"

	authmiddleware "git.999.haus/chris/DocuMCP-go/internal/auth/middleware"
	"git.999.haus/chris/DocuMCP-go/internal/auth/oauth"
	authscope "git.999.haus/chris/DocuMCP-go/internal/auth/scope"
	"git.999.haus/chris/DocuMCP-go/internal/auth/oidc"
	"git.999.haus/chris/DocuMCP-go/internal/handler"
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

	// Phase 7: River queue
	SSEHandler   *apihandler.SSEHandler   // nil if not configured
	QueueHandler *apihandler.QueueHandler // nil if not configured

	// Phase 9: Dashboard
	DashboardHandler *apihandler.DashboardHandler // nil if not configured

	// Vue SPA
	AuthHandler *apihandler.AuthHandler // nil if not configured
	SPAHandler  http.Handler           // nil if not configured

	// Search health
	SearchClient interface{ Healthy() bool } // for readiness checks (nil disables Meilisearch check)

	// Observability
	Metrics      *observability.Metrics // nil disables Prometheus metrics
	OTELEnabled  bool                   // enables tracing middleware

	// Security
	CSRFKey  []byte // 32-byte key for CSRF token generation (nil disables CSRF)
	IsSecure bool   // true when running behind TLS (sets Secure cookie flag)
	AppURL   string // APP_URL — used as CSRF trusted origin (e.g. "http://localhost:8080")

	// Infrastructure
	DB               *sql.DB // for readiness checks (nil disables /health/ready)
	InternalAPIToken string  // protects /metrics and /health/ready (empty = unrestricted)
}

// RegisterRoutes configures all middleware and route groups on the server.
func (s *Server) RegisterRoutes(deps Deps) {
	r := s.router

	// Built-in chi middleware (applied to all routes)
	r.Use(middleware.RequestID)
	r.Use(RealIP(s.trustedProxies))
	r.Use(middleware.Recoverer)
	r.Use(SecurityHeaders)
	r.Use(RequestLogger(s.logger))

	// OpenTelemetry tracing middleware
	if deps.OTELEnabled {
		r.Use(observability.Tracing("documcp"))
	}

	// Prometheus metrics middleware (before application middleware so it
	// captures the full request lifecycle including logging overhead).
	if deps.Metrics != nil {
		r.Use(observability.MetricsMiddleware(deps.Metrics))
	}

	// MCP endpoint — no timeout (SSE streams must stay open indefinitely).
	if deps.MCPHandler != nil {
		r.Group(func(r chi.Router) {
			if deps.OAuthService != nil {
				r.Use(authmiddleware.BearerToken(deps.OAuthService))
				r.Use(authmiddleware.RequireScope("mcp:access"))
			}
			r.Handle("/documcp/*", deps.MCPHandler)
			r.Handle("/documcp", deps.MCPHandler)
		})
		s.logger.Info("MCP endpoint registered", "path", "/documcp")
	}

	// Global middleware for all other routes
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(BlockSensitiveFiles)
	r.Use(MaxBodySize(1 * 1024 * 1024)) // 1 MB default body limit (excludes multipart)

	// Health check (liveness — cheap, no I/O)
	health := handler.NewHealthHandler(deps.Version)
	r.Method(http.MethodGet, "/health", health)

	// Readiness probe (checks dependencies like Postgres)
	if deps.DB != nil {
		readiness := handler.NewReadinessHandler(deps.Version, deps.DB, deps.SearchClient)
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
			// Browser-rendered form endpoints need CSRF protection.
			r.Group(func(r chi.Router) {
				if len(deps.CSRFKey) > 0 {
					csrfOpts := []csrf.Option{
						csrf.Secure(deps.IsSecure),
						csrf.Path("/oauth"),
						csrf.RequestHeader("X-CSRF-Token"),
					}
					if deps.AppURL != "" {
						// gorilla/csrf v1.7.3 compares TrustedOrigins against
						// parsedOrigin.Host (host:port), not the full URL.
						if parsed, err := url.Parse(deps.AppURL); err == nil && parsed.Host != "" {
							csrfOpts = append(csrfOpts, csrf.TrustedOrigins([]string{parsed.Host}))
						}
					}
					r.Use(csrf.Protect(deps.CSRFKey, csrfOpts...))
				}
				r.Get("/authorize", deps.OAuthHandler.Authorize)
				r.Post("/authorize/approve", deps.OAuthHandler.AuthorizeApprove)
				r.Get("/device", deps.OAuthHandler.DeviceVerification)
				r.Post("/device", deps.OAuthHandler.DeviceVerificationSubmit)
				r.Post("/device/approve", deps.OAuthHandler.DeviceApprove)
			})

			// Machine-to-machine endpoints — no CSRF (clients don't have browser cookies).
			// Rate-limited token endpoint (30/min + 100/hr per IP)
			r.Group(func(r chi.Router) {
				r.Use(httprate.LimitByIP(30, time.Minute))
				r.Use(httprate.LimitByIP(100, time.Hour))
				r.Post("/token", deps.OAuthHandler.Token)
				r.Post("/revoke", deps.OAuthHandler.Revoke)
			})

			// Rate-limited registration endpoint (10/hr + 50/day per IP)
			r.Group(func(r chi.Router) {
				r.Use(httprate.LimitByIP(10, time.Hour))
				r.Use(httprate.LimitByIP(50, 24*time.Hour))
				r.Post("/register", deps.OAuthHandler.Register)
			})

			// Rate-limited device code endpoint (30/min per IP)
			r.Group(func(r chi.Router) {
				r.Use(httprate.LimitByIP(30, time.Minute))
				r.Post("/device/code", deps.OAuthHandler.DeviceAuthorization)
			})
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

	// Session-based auth endpoint (no bearer token, uses session cookie)
	if deps.AuthHandler != nil {
		r.Get("/api/auth/me", deps.AuthHandler.Me)
		s.logger.Info("auth/me endpoint registered", "path", "/api/auth/me")
	}

	// REST API (always authenticated)
	r.Route("/api", func(r chi.Router) {
		switch {
		case deps.OAuthService != nil:
			r.Use(authmiddleware.BearerOrSession(deps.OAuthService, deps.SessionStore))
		default:
			// No authentication backend configured — block all API access.
			r.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusServiceUnavailable)
					_, _ = w.Write([]byte(`{"error":"Service Unavailable","message":"authentication not configured"}`))
				})
			})
		}

		// Document endpoints
		if deps.DocumentHandler != nil {
			r.Route("/documents", func(r chi.Router) {
				// Read-only: 60/min, requires documents:read scope
				r.Group(func(r chi.Router) {
					r.Use(httprate.LimitByIP(60, time.Minute))
					if deps.OAuthService != nil {
						r.Use(authmiddleware.RequireScope(authscope.DocumentsRead))
					}
					r.Get("/", deps.DocumentHandler.List)
					r.Get("/trash", deps.DocumentHandler.ListDeleted)
					r.Get("/{uuid}", deps.DocumentHandler.Show)
					r.Get("/{uuid}/download", deps.DocumentHandler.Download)
				})

				// Mutating: 30/min, requires documents:write scope + admin
				r.Group(func(r chi.Router) {
					r.Use(httprate.LimitByIP(30, time.Minute))
					if deps.OAuthService != nil {
						r.Use(authmiddleware.RequireScope(authscope.DocumentsWrite))
					}
					r.Use(authmiddleware.RequireAdmin)
					r.Post("/", deps.DocumentHandler.Upload)
					r.Post("/analyze", deps.DocumentHandler.Analyze)
					r.Put("/{uuid}", deps.DocumentHandler.Update)
					r.Delete("/{uuid}", deps.DocumentHandler.Delete)
					r.Post("/{uuid}/restore", deps.DocumentHandler.Restore)
					r.Delete("/{uuid}/purge", deps.DocumentHandler.Purge)
				})
			})
			s.logger.Info("document API endpoints registered")
		}

		// Search endpoints (120/min per IP)
		if deps.SearchHandler != nil {
			r.Group(func(r chi.Router) {
				r.Use(httprate.LimitByIP(120, time.Minute))
				if deps.OAuthService != nil {
					r.Use(authmiddleware.RequireScope(authscope.SearchRead))
				}
				r.Get("/search", deps.SearchHandler.Search)
				r.Get("/search/unified", deps.SearchHandler.FederatedSearch)
				r.Get("/search/popular", deps.SearchHandler.Popular)
				r.Get("/search/autocomplete", deps.SearchHandler.Autocomplete)
			})
			s.logger.Info("search API endpoints registered")
		}

		// ZIM archive endpoints (60/min per IP)
		if deps.ZimHandler != nil {
			r.Route("/zim/archives", func(r chi.Router) {
				r.Use(httprate.LimitByIP(60, time.Minute))
				if deps.OAuthService != nil {
					r.Use(authmiddleware.RequireScope(authscope.ZIMRead))
				}
				r.Get("/", deps.ZimHandler.List)
				r.Get("/{archive}", deps.ZimHandler.Show)
				r.Get("/{archive}/search", deps.ZimHandler.Search)
				r.Get("/{archive}/suggest", deps.ZimHandler.Suggest)
				r.Get("/{archive}/articles/*", deps.ZimHandler.ReadArticle)
			})
			s.logger.Info("ZIM API endpoints registered")
		}

		// Confluence endpoints (60/min per IP)
		if deps.ConfluenceHandler != nil {
			r.Route("/confluence", func(r chi.Router) {
				r.Use(httprate.LimitByIP(60, time.Minute))
				if deps.OAuthService != nil {
					r.Use(authmiddleware.RequireScope(authscope.ConfluenceRead))
				}
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
				// Read-only: 60/min, requires templates:read scope
				r.Group(func(r chi.Router) {
					r.Use(httprate.LimitByIP(60, time.Minute))
					if deps.OAuthService != nil {
						r.Use(authmiddleware.RequireScope(authscope.TemplatesRead))
					}
					r.Get("/", deps.GitTemplateHandler.List)
					r.Get("/search", deps.GitTemplateHandler.Search)
					r.Get("/{uuid}", deps.GitTemplateHandler.Show)
					r.Get("/{uuid}/structure", deps.GitTemplateHandler.Structure)
					r.Get("/{uuid}/files/*", deps.GitTemplateHandler.ReadFile)
					r.Get("/{uuid}/deployment-guide", deps.GitTemplateHandler.DeploymentGuide)
				})

				// Mutating: 30/min, requires templates:write scope + admin
				r.Group(func(r chi.Router) {
					r.Use(httprate.LimitByIP(30, time.Minute))
					if deps.OAuthService != nil {
						r.Use(authmiddleware.RequireScope(authscope.TemplatesWrite))
					}
					r.Use(authmiddleware.RequireAdmin)
					r.Post("/", deps.GitTemplateHandler.Create)
					r.Put("/{uuid}", deps.GitTemplateHandler.Update)
					r.Delete("/{uuid}", deps.GitTemplateHandler.Delete)
					r.Post("/{uuid}/sync", deps.GitTemplateHandler.Sync)
					r.Post("/{uuid}/download", deps.GitTemplateHandler.Download)
				})
			})
			s.logger.Info("Git template API endpoints registered")
		}

		// External service endpoints
		if deps.ExternalServiceHandler != nil {
			r.Route("/external-services", func(r chi.Router) {
				// Read-only: 60/min, requires services:read scope
				r.Group(func(r chi.Router) {
					r.Use(httprate.LimitByIP(60, time.Minute))
					if deps.OAuthService != nil {
						r.Use(authmiddleware.RequireScope(authscope.ServicesRead))
					}
					r.Get("/", deps.ExternalServiceHandler.List)
					r.Get("/{uuid}", deps.ExternalServiceHandler.Show)
				})

				// Mutating: 30/min, requires services:write scope + admin
				r.Group(func(r chi.Router) {
					r.Use(httprate.LimitByIP(30, time.Minute))
					if deps.OAuthService != nil {
						r.Use(authmiddleware.RequireScope(authscope.ServicesWrite))
					}
					r.Use(authmiddleware.RequireAdmin)
					r.Post("/", deps.ExternalServiceHandler.Create)
					r.Put("/{uuid}", deps.ExternalServiceHandler.Update)
					r.Delete("/{uuid}", deps.ExternalServiceHandler.Delete)
					r.Post("/{uuid}/health-check", deps.ExternalServiceHandler.HealthCheck)
				})
			})
			s.logger.Info("External service API endpoints registered")
		}

		// Admin API endpoints (60/min, requires admin scope + role)
		r.Route("/admin", func(r chi.Router) {
			r.Use(httprate.LimitByIP(60, time.Minute))
			if deps.OAuthService != nil {
				r.Use(authmiddleware.RequireScope(authscope.Admin))
			}
			r.Use(authmiddleware.RequireAdmin)

			// SSE events (admin-only: exposes queue operational data)
			if deps.SSEHandler != nil {
				r.Get("/events/stream", deps.SSEHandler.Stream)
			}
			// Dashboard stats
			if deps.DashboardHandler != nil {
				r.Get("/dashboard/stats", deps.DashboardHandler.Stats)
			}

			// User management
			if deps.UserHandler != nil {
				r.Route("/users", func(r chi.Router) {
					r.Get("/", deps.UserHandler.List)
					r.Post("/", deps.UserHandler.Create)
					r.Get("/{id}", deps.UserHandler.Show)
					r.Put("/{id}", deps.UserHandler.Update)
					r.Delete("/{id}", deps.UserHandler.Delete)
					r.Post("/{id}/toggle-admin", deps.UserHandler.ToggleAdmin)
				})
			}

			// Document bulk purge
			if deps.DocumentHandler != nil {
				r.Delete("/documents/purge", deps.DocumentHandler.BulkPurge)
			}

			// External service reordering
			if deps.ExternalServiceHandler != nil {
				r.Put("/external-services/reorder", deps.ExternalServiceHandler.Reorder)
			}

			// Git template URL validation
			if deps.GitTemplateHandler != nil {
				r.Post("/git-templates/validate-url", deps.GitTemplateHandler.ValidateURL)
			}

			// OAuth client management
			if deps.OAuthClientHandler != nil {
				r.Route("/oauth-clients", func(r chi.Router) {
					r.Get("/", deps.OAuthClientHandler.List)
					r.Post("/", deps.OAuthClientHandler.Create)
					r.Get("/{id}", deps.OAuthClientHandler.Show)
					r.Post("/{id}/revoke", deps.OAuthClientHandler.Revoke)
				})
			}

			// Queue management
			if deps.QueueHandler != nil {
				r.Route("/queue", func(r chi.Router) {
					r.Get("/stats", deps.QueueHandler.Stats)
					r.Get("/failed", deps.QueueHandler.ListFailed)
					r.Post("/failed/{id}/retry", deps.QueueHandler.RetryFailed)
					r.Delete("/failed/{id}", deps.QueueHandler.DeleteFailed)
				})
			}
		})
		s.logger.Info("admin API endpoints registered")
	})

	// Backward compatibility: /admin/login redirects to /auth/login
	r.Get("/admin/login", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/auth/login", http.StatusMovedPermanently)
	})

	// Vue SPA at /admin/* (must be registered last to avoid shadowing API routes)
	if deps.SPAHandler != nil {
		r.Get("/admin", http.RedirectHandler("/admin/", http.StatusMovedPermanently).ServeHTTP)
		r.Mount("/admin/", http.StripPrefix("/admin", deps.SPAHandler))
		s.logger.Info("SPA handler registered", "path", "/admin/*")
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
