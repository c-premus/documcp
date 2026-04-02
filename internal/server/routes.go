package server

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	httprateredis "github.com/go-chi/httprate-redis"
	"github.com/gorilla/sessions"
	"github.com/redis/go-redis/v9"

	authmiddleware "github.com/c-premus/documcp/internal/auth/middleware"
	"github.com/c-premus/documcp/internal/auth/oauth"
	"github.com/c-premus/documcp/internal/auth/oidc"
	authscope "github.com/c-premus/documcp/internal/auth/scope"
	"github.com/c-premus/documcp/internal/handler"
	apihandler "github.com/c-premus/documcp/internal/handler/api"
	oauthhandler "github.com/c-premus/documcp/internal/handler/oauth"
	"github.com/c-premus/documcp/internal/observability"
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
	SearchHandler   *apihandler.SearchHandler   // nil if search not configured

	// Phase 4: External service clients & REST API
	ZimHandler             *apihandler.ZimHandler
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
	SPAHandler  http.Handler            // nil if not configured

	// Observability
	Metrics     *observability.Metrics // nil disables Prometheus metrics
	OTELEnabled bool                   // enables tracing middleware

	// Security
	IsSecure bool // true when running behind TLS (reserved for future use)

	// Infrastructure
	RateLimitRedisClient *redis.Client    // dedicated client for httprate-redis (no retries, no otel)
	RedisClient          *redis.Client    // main client for readiness checks
	DB               handler.DBPinger // for readiness checks (nil disables /health/ready)
	InternalAPIToken string           // protects /metrics and /health/ready (empty = unrestricted)

	// Server tuning (populated from config)
	MaxBodySize    int64         // max request body size in bytes (excludes multipart)
	RequestTimeout time.Duration // context timeout for non-streaming requests
	HSTSMaxAge     int           // HSTS max-age in seconds (0 to disable)
}

// RegisterRoutes configures all middleware and route groups on the server.
func (s *Server) RegisterRoutes(deps Deps) {
	s.registerGlobalMiddleware(deps)
	s.registerInfraRoutes(deps)
	s.registerAuthRoutes(deps)
	s.registerAPIRoutes(deps)
	s.registerSPARoutes(deps)
}

// registerGlobalMiddleware installs the middleware stack applied to all routes.
func (s *Server) registerGlobalMiddleware(deps Deps) {
	r := s.router

	r.Use(middleware.RequestID)
	r.Use(RealIP(s.trustedProxies))
	r.Use(SafeRecoverer(s.logger))
	r.Use(SecurityHeaders(deps.HSTSMaxAge))

	// OpenTelemetry tracing middleware — must run before RequestLogger so the
	// span context is available for trace_id/span_id injection into logs.
	if deps.OTELEnabled {
		r.Use(observability.Tracing("documcp"))
	}

	r.Use(RequestLogger(s.logger))

	// Prometheus metrics middleware (before application middleware so it
	// captures the full request lifecycle including logging overhead).
	if deps.Metrics != nil {
		r.Use(observability.MetricsMiddleware(deps.Metrics))
	}

	r.Use(BlockSensitiveFiles)
	r.Use(MaxBodySize(deps.MaxBodySize))
	r.Use(TimeoutExcept(deps.RequestTimeout, "/documcp", "/api/admin/events/stream"))

	// Cross-origin protection: blocks cross-origin POST/PUT/DELETE/PATCH using
	// Sec-Fetch-Site (all modern browsers) with Origin fallback. GET/HEAD/OPTIONS
	// are exempt. API clients (curl, OAuth M2M) pass through (no Origin header).
	cop := http.NewCrossOriginProtection()
	r.Use(cop.Handler)
}

// registerInfraRoutes registers MCP, health, metrics, and well-known discovery endpoints.
func (s *Server) registerInfraRoutes(deps Deps) {
	r := s.router

	// MCP endpoint — timeout excluded in global middleware (SSE streams must stay open).
	// OAuth MUST be configured; without it, all MCP tools would be unauthenticated.
	if deps.MCPHandler != nil && deps.OAuthService != nil {
		r.Group(func(r chi.Router) {
			r.Use(rateLimitByIP(60, time.Minute, deps.RateLimitRedisClient))
			r.Use(authmiddleware.BearerToken(deps.OAuthService, s.logger))
			r.Use(authmiddleware.RequireScope("mcp:access", s.logger))
			r.Handle("/documcp/*", deps.MCPHandler)
			r.Handle("/documcp", deps.MCPHandler)
		})
		s.logger.Info("MCP endpoint registered", "path", "/documcp")
	} else if deps.MCPHandler != nil {
		s.logger.Warn("MCP endpoint NOT registered: OAuth service not configured")
	}

	// Health check (liveness — cheap, no I/O)
	health := handler.NewHealthHandler(deps.Version)
	r.Method(http.MethodGet, "/health", health)

	// Readiness probe (checks dependencies like Postgres and Redis)
	if deps.DB != nil {
		var redisPinger handler.RedisPinger
		if deps.RedisClient != nil {
			redisPinger = &redisClientPinger{deps.RedisClient}
		}
		readiness := handler.NewReadinessHandler(deps.Version, deps.DB, redisPinger)
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
}

// registerAuthRoutes registers OAuth, OIDC, and session auth endpoints.
func (s *Server) registerAuthRoutes(deps Deps) {
	r := s.router

	if deps.OAuthHandler != nil {
		r.Route("/oauth", func(r chi.Router) {
			// Browser-rendered form endpoints — protected by CrossOriginProtection
			// (global middleware) plus SameSite=Lax session cookies.
			r.Get("/authorize", deps.OAuthHandler.Authorize)
			r.Post("/authorize/approve", deps.OAuthHandler.AuthorizeApprove)
			r.Post("/authorize/deny", deps.OAuthHandler.AuthorizeDeny)
			r.Get("/device", deps.OAuthHandler.DeviceVerification)
			r.Post("/device", deps.OAuthHandler.DeviceVerificationSubmit)
			r.Post("/device/approve", deps.OAuthHandler.DeviceApprove)

			// Machine-to-machine endpoints — no CSRF (clients don't have browser cookies).
			r.Group(func(r chi.Router) {
				r.Use(rateLimitByIP(30, time.Minute, deps.RateLimitRedisClient))
				r.Use(rateLimitByIP(100, time.Hour, deps.RateLimitRedisClient))
				r.Post("/token", deps.OAuthHandler.Token)
				r.Post("/revoke", deps.OAuthHandler.Revoke)
			})

			r.Group(func(r chi.Router) {
				r.Use(rateLimitByIP(10, time.Hour, deps.RateLimitRedisClient))
				r.Use(rateLimitByIP(50, 24*time.Hour, deps.RateLimitRedisClient))
				r.Post("/register", deps.OAuthHandler.Register)
			})

			r.Group(func(r chi.Router) {
				r.Use(rateLimitByIP(30, time.Minute, deps.RateLimitRedisClient))
				r.Post("/device/code", deps.OAuthHandler.DeviceAuthorization)
			})
		})
		s.logger.Info("OAuth endpoints registered")
	}

	if deps.OIDCHandler != nil {
		r.Route("/auth", func(r chi.Router) {
			r.Use(rateLimitByIP(30, time.Minute, deps.RateLimitRedisClient))
			r.Get("/login", deps.OIDCHandler.Login)
			r.Get("/callback", deps.OIDCHandler.Callback)
			r.Post("/logout", deps.OIDCHandler.Logout)
		})
		s.logger.Info("OIDC auth endpoints registered")
	}

	if deps.AuthHandler != nil {
		r.Group(func(r chi.Router) {
			r.Use(rateLimitByIP(60, time.Minute, deps.RateLimitRedisClient))
			r.Get("/api/auth/me", deps.AuthHandler.Me)
		})
		s.logger.Info("auth/me endpoint registered", "path", "/api/auth/me")
	}
}

// registerAPIRoutes registers the REST API route group with dual-auth model.
// All routes under /api require authentication:
//   - OAuth tokens: scoped via RequireScope middleware
//   - Session cookies: scoped via handler-level ownership checks (see RequireScope doc)
//
// When OAuthService is nil, the entire /api group returns 503. All nested
// RequireScope calls are therefore guaranteed to have a non-nil OAuthService.
func (s *Server) registerAPIRoutes(deps Deps) {
	s.router.Route("/api", func(r chi.Router) {
		switch {
		case deps.OAuthService != nil:
			r.Use(authmiddleware.BearerOrSession(deps.OAuthService, deps.SessionStore, s.logger))
		default:
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
				r.Group(func(r chi.Router) {
					r.Use(rateLimitByIP(60, time.Minute, deps.RateLimitRedisClient))
					r.Use(authmiddleware.RequireScope(authscope.DocumentsRead, s.logger))
					r.Get("/", deps.DocumentHandler.List)
					r.Get("/trash", deps.DocumentHandler.ListDeleted)
					r.Get("/{uuid}", deps.DocumentHandler.Show)
					r.Get("/{uuid}/download", deps.DocumentHandler.Download)
				})

				r.Group(func(r chi.Router) {
					r.Use(rateLimitByIP(30, time.Minute, deps.RateLimitRedisClient))
					r.Use(authmiddleware.RequireScope(authscope.DocumentsWrite, s.logger))
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
				r.Use(rateLimitByIP(120, time.Minute, deps.RateLimitRedisClient))
				r.Use(authmiddleware.RequireScope(authscope.SearchRead, s.logger))
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
				r.Use(rateLimitByIP(60, time.Minute, deps.RateLimitRedisClient))
				r.Use(authmiddleware.RequireScope(authscope.ZIMRead, s.logger))
				r.Get("/", deps.ZimHandler.List)
				r.Get("/{archive}", deps.ZimHandler.Show)
				r.Get("/{archive}/search", deps.ZimHandler.Search)
				r.Get("/{archive}/suggest", deps.ZimHandler.Suggest)
				r.Get("/{archive}/articles/*", deps.ZimHandler.ReadArticle)
			})
			s.logger.Info("ZIM API endpoints registered")
		}

		// Git template endpoints
		if deps.GitTemplateHandler != nil {
			r.Route("/git-templates", func(r chi.Router) {
				r.Group(func(r chi.Router) {
					r.Use(rateLimitByIP(60, time.Minute, deps.RateLimitRedisClient))
					r.Use(authmiddleware.RequireScope(authscope.TemplatesRead, s.logger))
					r.Get("/", deps.GitTemplateHandler.List)
					r.Get("/search", deps.GitTemplateHandler.Search)
					r.Get("/{uuid}", deps.GitTemplateHandler.Show)
					r.Get("/{uuid}/structure", deps.GitTemplateHandler.Structure)
					r.Get("/{uuid}/files/*", deps.GitTemplateHandler.ReadFile)
					r.Get("/{uuid}/deployment-guide", deps.GitTemplateHandler.DeploymentGuide)
				})

				r.Group(func(r chi.Router) {
					r.Use(rateLimitByIP(30, time.Minute, deps.RateLimitRedisClient))
					r.Use(authmiddleware.RequireScope(authscope.TemplatesWrite, s.logger))
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
				r.Group(func(r chi.Router) {
					r.Use(rateLimitByIP(60, time.Minute, deps.RateLimitRedisClient))
					r.Use(authmiddleware.RequireScope(authscope.ServicesRead, s.logger))
					r.Get("/", deps.ExternalServiceHandler.List)
					r.Get("/{uuid}", deps.ExternalServiceHandler.Show)
				})

				r.Group(func(r chi.Router) {
					r.Use(rateLimitByIP(30, time.Minute, deps.RateLimitRedisClient))
					r.Use(authmiddleware.RequireScope(authscope.ServicesWrite, s.logger))
					r.Use(authmiddleware.RequireAdmin)
					r.Post("/", deps.ExternalServiceHandler.Create)
					r.Put("/{uuid}", deps.ExternalServiceHandler.Update)
					r.Delete("/{uuid}", deps.ExternalServiceHandler.Delete)
					r.Post("/{uuid}/health-check", deps.ExternalServiceHandler.HealthCheck)
					r.Post("/{uuid}/sync", deps.ExternalServiceHandler.Sync)
				})
			})
			s.logger.Info("External service API endpoints registered")
		}

		// Admin API endpoints (60/min, requires admin scope + role)
		r.Route("/admin", func(r chi.Router) {
			r.Use(rateLimitByIP(60, time.Minute, deps.RateLimitRedisClient))
			r.Use(authmiddleware.RequireScope(authscope.Admin, s.logger))
			r.Use(authmiddleware.RequireAdmin)

			if deps.SSEHandler != nil {
				r.Get("/events/stream", deps.SSEHandler.Stream)
			}
			if deps.DashboardHandler != nil {
				r.Get("/dashboard/stats", deps.DashboardHandler.Stats)
			}

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

			if deps.DocumentHandler != nil {
				r.Delete("/documents/purge", deps.DocumentHandler.BulkPurge)
			}
			if deps.ExternalServiceHandler != nil {
				r.Put("/external-services/reorder", deps.ExternalServiceHandler.Reorder)
			}
			if deps.GitTemplateHandler != nil {
				r.Post("/git-templates/validate-url", deps.GitTemplateHandler.ValidateURL)
			}

			if deps.OAuthClientHandler != nil {
				r.Route("/oauth-clients", func(r chi.Router) {
					r.Get("/", deps.OAuthClientHandler.List)
					r.Post("/", deps.OAuthClientHandler.Create)
					r.Get("/{id}", deps.OAuthClientHandler.Show)
					r.Post("/{id}/revoke", deps.OAuthClientHandler.Revoke)
				})
			}

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
}

// registerSPARoutes registers the Vue SPA mount, backward-compat redirects, and root redirect.
func (s *Server) registerSPARoutes(deps Deps) {
	r := s.router

	r.Get("/admin/login", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/auth/login", http.StatusMovedPermanently)
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusFound)
	})

	if deps.SPAHandler != nil {
		r.Get("/admin", http.RedirectHandler("/admin/", http.StatusMovedPermanently).ServeHTTP)
		r.Mount("/admin/", http.StripPrefix("/admin", deps.SPAHandler))
		s.logger.Info("SPA handler registered", "path", "/admin/*")
	}
}

// rateLimitByIP creates an IP-based rate limiter. When a Redis client is
// provided, counters are shared across all server instances. When nil (tests),
// it falls back to the default in-memory counter.
func rateLimitByIP(count int, window time.Duration, rc *redis.Client) func(http.Handler) http.Handler {
	if rc == nil {
		return httprate.LimitByIP(count, window)
	}
	return httprate.Limit(count, window,
		httprate.WithKeyByIP(),
		httprateredis.WithRedisLimitCounter(&httprateredis.Config{
			Client:    rc,
			PrefixKey: "documcp:rate",
		}),
	)
}

// internalTokenAuth returns a middleware that requires a bearer token matching
// the configured internal API token. Used to protect operational endpoints
// like /metrics and /health/ready.
func internalTokenAuth(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				jsonErrorResponse(w, http.StatusUnauthorized, "Bearer token required")
				return
			}
			provided := strings.TrimPrefix(auth, "Bearer ")
			if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
				jsonErrorResponse(w, http.StatusUnauthorized, "Invalid token")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// jsonErrorResponse writes a JSON error response consistent with the app's error format.
func jsonErrorResponse(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":"` + http.StatusText(status) + `","message":"` + message + `"}`))
}

// redisClientPinger adapts *redis.Client to handler.RedisPinger.
type redisClientPinger struct {
	client *redis.Client
}

// Ping checks Redis connectivity using an independent context so that
// HTTP request cancellation cannot leave unread data on the connection.
func (p *redisClientPinger) Ping(_ context.Context) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return p.client.Ping(ctx).Err()
}
