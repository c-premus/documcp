package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	httprateredis "github.com/go-chi/httprate-redis"
	"github.com/gorilla/sessions"
	"github.com/jackc/pgx/v5/pgxpool"
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

// Handlers groups every HTTP handler mounted on the router. Any field may be
// nil when the corresponding feature is not configured. Field names preserve
// the "*Handler" suffix so call sites and tests don't need to distinguish
// between handlers and other dependencies by context alone.
type Handlers struct {
	MCPHandler             http.Handler
	DocumentHandler        *apihandler.DocumentHandler
	SearchHandler          *apihandler.SearchHandler
	ZimHandler             *apihandler.ZimHandler
	GitTemplateHandler     *apihandler.GitTemplateHandler
	ExternalServiceHandler *apihandler.ExternalServiceHandler
	UserHandler            *apihandler.UserHandler
	OAuthClientHandler     *apihandler.OAuthClientHandler
	SSEHandler             *apihandler.SSEHandler
	QueueHandler           *apihandler.QueueHandler
	DashboardHandler       *apihandler.DashboardHandler
	RiverUIHandler         http.Handler
	AuthHandler            *apihandler.AuthHandler
	SPAHandler             http.Handler
	RootAssetHandler       http.Handler
}

// Auth groups everything the auth middleware stack reads: the OAuth/OIDC
// handlers mounted under /oauth + /auth, the OAuth service used by bearer
// and session middleware, the session store, and the RFC 8707 audience
// strings that /documcp and /api enforce.
type Auth struct {
	OAuthHandler *oauthhandler.Handler
	OIDCHandler  *oidc.Handler
	OAuthService *oauth.Service
	SessionStore sessions.Store
	MCPResource  string // expected audience for the /documcp MCP endpoint
	APIResource  string // expected audience for /api/* bearer-token requests
}

// Tuning groups operator-configured numeric limits applied to HTTP requests.
type Tuning struct {
	MaxBodySize      int64         // max request body size (excludes multipart)
	RequestTimeout   time.Duration // context timeout for non-streaming requests
	HSTSMaxAge       int           // HSTS max-age in seconds (0 to disable)
	InternalAPIToken string        // protects /metrics + /health/ready (empty = unrestricted)
}

// Deps holds handler dependencies injected from the app layer. The big
// groups (Handlers, Auth, Tuning) are bundled to keep the top-level shape
// navigable; the remaining flat fields are truly standalone singletons.
type Deps struct {
	Version  string
	Handlers Handlers
	Auth     Auth
	Tuning   Tuning

	// Observability
	Metrics     *observability.Metrics // nil disables Prometheus metrics
	OTELEnabled bool                   // enables tracing middleware

	// Infrastructure
	BareRedisClient *redis.Client            // uninstrumented (rate limit + readiness)
	RedisClient     *redis.Client            // instrumented (EventBus, app queries)
	DB              handler.DependencyPinger // readiness (nil disables /health/ready)
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
	r.Use(SecurityHeaders(deps.Tuning.HSTSMaxAge))

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
	r.Use(MaxBodySize(deps.Tuning.MaxBodySize))
	r.Use(TimeoutExcept(deps.Tuning.RequestTimeout, "/documcp", "/api/admin/events/stream", "/api/events/stream"))

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
	if deps.Handlers.MCPHandler != nil && deps.Auth.OAuthService != nil {
		r.Group(func(r chi.Router) {
			r.Use(rateLimitByIP(60, time.Minute, deps.BareRedisClient))
			r.Use(authmiddleware.BearerTokenWithAudience(deps.Auth.OAuthService, s.logger, deps.Auth.MCPResource))
			r.Use(authmiddleware.RequireScope("mcp:access", s.logger))
			r.Handle("/documcp/*", deps.Handlers.MCPHandler)
			r.Handle("/documcp", deps.Handlers.MCPHandler)
		})
		s.logger.Info("MCP endpoint registered", "path", "/documcp")
	} else if deps.Handlers.MCPHandler != nil {
		s.logger.Warn("MCP endpoint NOT registered: OAuth service not configured")
	}

	// Health check (liveness — cheap, no I/O)
	health := handler.NewHealthHandler(deps.Version)
	r.Method(http.MethodGet, "/health", health)

	// Readiness probe (checks dependencies like Postgres and Redis)
	if deps.DB != nil {
		var redisPinger handler.DependencyPinger
		if deps.BareRedisClient != nil {
			redisPinger = &redisClientPinger{deps.BareRedisClient}
		}
		readiness := handler.NewReadinessHandler(deps.Version, deps.DB, redisPinger)
		r.Method(http.MethodGet, "/health/ready", readiness)
	}

	// Prometheus metrics endpoint (protected by internal API token if configured)
	if deps.Metrics != nil {
		r.Group(func(r chi.Router) {
			if deps.Tuning.InternalAPIToken != "" {
				r.Use(internalTokenAuth(deps.Tuning.InternalAPIToken))
			} else {
				s.logger.Warn("metrics endpoint exposed without authentication (INTERNAL_API_TOKEN not set)")
			}
			r.Method(http.MethodGet, "/metrics", observability.MetricsHandler())
		})
		s.logger.Info("Prometheus metrics endpoint registered", "path", "/metrics")
	}

	// Well-known discovery endpoints
	if deps.Auth.OAuthHandler != nil {
		r.Get("/.well-known/oauth-authorization-server", deps.Auth.OAuthHandler.AuthorizationServerMetadata)
		r.Get("/.well-known/oauth-protected-resource", deps.Auth.OAuthHandler.ProtectedResourceMetadata)
		r.Get("/.well-known/oauth-protected-resource/*", deps.Auth.OAuthHandler.ProtectedResourceMetadata)
		s.logger.Info("OAuth well-known endpoints registered")
	}
}

// registerAuthRoutes registers OAuth, OIDC, and session auth endpoints.
func (s *Server) registerAuthRoutes(deps Deps) {
	r := s.router

	if deps.Auth.OAuthHandler != nil {
		r.Route("/oauth", func(r chi.Router) {
			// Browser-rendered form endpoints — protected by CrossOriginProtection
			// (global middleware) plus SameSite=Lax session cookies. Per-IP rate
			// limit caps consent-form flooding (burns session state and DB write
			// traffic via GrantClientScope / POST approve).
			r.Group(func(r chi.Router) {
				r.Use(rateLimitByIP(30, time.Minute, deps.BareRedisClient))
				r.Get("/authorize", deps.Auth.OAuthHandler.Authorize)
				r.Post("/authorize/approve", deps.Auth.OAuthHandler.AuthorizeApprove)
				r.Post("/authorize/deny", deps.Auth.OAuthHandler.AuthorizeDeny)
			})
			r.Get("/device", deps.Auth.OAuthHandler.DeviceVerification)
			r.Group(func(r chi.Router) {
				r.Use(rateLimitByIP(10, time.Minute, deps.BareRedisClient))
				r.Post("/device", deps.Auth.OAuthHandler.DeviceVerificationSubmit)
				r.Post("/device/approve", deps.Auth.OAuthHandler.DeviceApprove)
			})

			// Machine-to-machine endpoints — no CSRF (clients don't have browser cookies).
			// Two rateLimitByIP calls stack: the first bounds short bursts, the
			// second caps sustained abuse over a longer window.
			r.Group(func(r chi.Router) {
				r.Use(rateLimitByIP(30, time.Minute, deps.BareRedisClient))
				r.Use(rateLimitByIP(100, time.Hour, deps.BareRedisClient))
				r.Post("/token", deps.Auth.OAuthHandler.Token)
				r.Post("/revoke", deps.Auth.OAuthHandler.Revoke)
			})

			// Dynamic client registration — tighter per-hour + per-day caps.
			r.Group(func(r chi.Router) {
				r.Use(rateLimitByIP(10, time.Hour, deps.BareRedisClient))
				r.Use(rateLimitByIP(50, 24*time.Hour, deps.BareRedisClient))
				r.Post("/register", deps.Auth.OAuthHandler.Register)
			})

			r.Group(func(r chi.Router) {
				r.Use(rateLimitByIP(30, time.Minute, deps.BareRedisClient))
				r.Post("/device/code", deps.Auth.OAuthHandler.DeviceAuthorization)
			})
		})
		s.logger.Info("OAuth endpoints registered")
	}

	if deps.Auth.OIDCHandler != nil {
		r.Route("/auth", func(r chi.Router) {
			r.Use(rateLimitByIP(30, time.Minute, deps.BareRedisClient))
			r.Get("/login", deps.Auth.OIDCHandler.Login)
			r.Get("/callback", deps.Auth.OIDCHandler.Callback)
			r.Post("/logout", deps.Auth.OIDCHandler.Logout)
		})
		s.logger.Info("OIDC auth endpoints registered")
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
		// Root-level IP rate limit applied BEFORE the bearer token DB lookup.
		// Without this gate, an unauthenticated attacker could force one DB
		// hash-lookup per request by sending arbitrary bearer tokens — per-route
		// rate limits inside nested groups kick in too late for that. Cap is
		// a generous 300 req/min/IP so it doesn't interfere with legitimate SSE
		// reconnects or admin-panel bursts; the per-route caps stay stricter.
		r.Use(rateLimitByIP(300, time.Minute, deps.BareRedisClient))
		switch {
		case deps.Auth.OAuthService != nil:
			r.Use(authmiddleware.BearerOrSessionWithAudience(deps.Auth.OAuthService, deps.Auth.SessionStore, s.logger, deps.Auth.APIResource))
		default:
			r.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusServiceUnavailable)
					_, _ = w.Write([]byte(`{"error":"Service Unavailable","message":"authentication not configured"}`))
				})
			})
		}

		// Auth status endpoint (no scope requirement — any authenticated user)
		if deps.Handlers.AuthHandler != nil {
			r.Group(func(r chi.Router) {
				r.Use(rateLimitByIP(60, time.Minute, deps.BareRedisClient))
				r.Get("/auth/me", deps.Handlers.AuthHandler.Me)
			})
		}

		// User SSE endpoint (any authenticated user — server-side filtering by role)
		if deps.Handlers.SSEHandler != nil {
			r.Group(func(r chi.Router) {
				r.Use(authmiddleware.RequireScope(authscope.DocumentsRead, s.logger))
				r.Get("/events/stream", deps.Handlers.SSEHandler.UserStream)
			})
		}

		// Document endpoints
		if deps.Handlers.DocumentHandler != nil {
			r.Route("/documents", func(r chi.Router) {
				r.Group(func(r chi.Router) {
					r.Use(rateLimitByIP(60, time.Minute, deps.BareRedisClient))
					r.Use(authmiddleware.RequireScope(authscope.DocumentsRead, s.logger))
					r.Get("/", deps.Handlers.DocumentHandler.List)
					r.Get("/tags", deps.Handlers.DocumentHandler.ListTags)
					r.Get("/trash", deps.Handlers.DocumentHandler.ListDeleted)
					r.Get("/{uuid}", deps.Handlers.DocumentHandler.Show)
					r.Get("/{uuid}/download", deps.Handlers.DocumentHandler.Download)
				})

				// Write operations: any authenticated user with documents:write scope.
				// Handlers enforce ownership via checkOwnership (admins bypass).
				r.Group(func(r chi.Router) {
					r.Use(rateLimitByIP(30, time.Minute, deps.BareRedisClient))
					r.Use(authmiddleware.RequireScope(authscope.DocumentsWrite, s.logger))
					r.Post("/", deps.Handlers.DocumentHandler.Upload)
					r.Post("/analyze", deps.Handlers.DocumentHandler.Analyze)
					r.Put("/{uuid}", deps.Handlers.DocumentHandler.Update)
					r.Delete("/{uuid}", deps.Handlers.DocumentHandler.Delete)
					r.Post("/{uuid}/content", deps.Handlers.DocumentHandler.ReplaceContent)
					r.Post("/{uuid}/restore", deps.Handlers.DocumentHandler.Restore)
				})

				// Purge: admin-only (irreversible permanent deletion)
				r.Group(func(r chi.Router) {
					r.Use(rateLimitByIP(30, time.Minute, deps.BareRedisClient))
					r.Use(authmiddleware.RequireScope(authscope.DocumentsWrite, s.logger))
					r.Use(authmiddleware.RequireAdmin)
					r.Delete("/{uuid}/purge", deps.Handlers.DocumentHandler.Purge)
				})
			})
			s.logger.Info("document API endpoints registered")
		}

		// Search endpoints (120/min per IP)
		if deps.Handlers.SearchHandler != nil {
			r.Group(func(r chi.Router) {
				r.Use(rateLimitByIP(120, time.Minute, deps.BareRedisClient))
				r.Use(authmiddleware.RequireScope(authscope.SearchRead, s.logger))
				r.Get("/search", deps.Handlers.SearchHandler.Search)
				r.Get("/search/unified", deps.Handlers.SearchHandler.FederatedSearch)
				r.Get("/search/autocomplete", deps.Handlers.SearchHandler.Autocomplete)
			})
			// Popular searches are admin-only: the aggregation is global
			// across all users (currently has no per-user scoping), so
			// exposing it to any search:read bearer leaks what other
			// users and third-party clients are searching for — project
			// names, client names, internal-tool terms. Admin-gate until
			// a proper per-caller scoping or anonymizing rollup lands.
			r.Group(func(r chi.Router) {
				r.Use(rateLimitByIP(120, time.Minute, deps.BareRedisClient))
				r.Use(authmiddleware.RequireAdmin)
				r.Get("/search/popular", deps.Handlers.SearchHandler.Popular)
			})
			s.logger.Info("search API endpoints registered")
		}

		// ZIM archive endpoints (60/min per IP)
		if deps.Handlers.ZimHandler != nil {
			r.Route("/zim/archives", func(r chi.Router) {
				r.Use(rateLimitByIP(60, time.Minute, deps.BareRedisClient))
				r.Use(authmiddleware.RequireScope(authscope.ZIMRead, s.logger))
				r.Get("/", deps.Handlers.ZimHandler.List)
				r.Get("/{archive}", deps.Handlers.ZimHandler.Show)
				r.Get("/{archive}/search", deps.Handlers.ZimHandler.Search)
				r.Get("/{archive}/suggest", deps.Handlers.ZimHandler.Suggest)
				r.Get("/{archive}/articles/*", deps.Handlers.ZimHandler.ReadArticle)
			})
			s.logger.Info("ZIM API endpoints registered")
		}

		// Git template endpoints
		if deps.Handlers.GitTemplateHandler != nil {
			r.Route("/git-templates", func(r chi.Router) {
				r.Group(func(r chi.Router) {
					r.Use(rateLimitByIP(60, time.Minute, deps.BareRedisClient))
					r.Use(authmiddleware.RequireScope(authscope.TemplatesRead, s.logger))
					r.Get("/", deps.Handlers.GitTemplateHandler.List)
					r.Get("/search", deps.Handlers.GitTemplateHandler.Search)
					r.Get("/{uuid}", deps.Handlers.GitTemplateHandler.Show)
					r.Get("/{uuid}/structure", deps.Handlers.GitTemplateHandler.Structure)
					r.Get("/{uuid}/files/*", deps.Handlers.GitTemplateHandler.ReadFile)
					r.Get("/{uuid}/deployment-guide", deps.Handlers.GitTemplateHandler.DeploymentGuide)
				})

				r.Group(func(r chi.Router) {
					r.Use(rateLimitByIP(30, time.Minute, deps.BareRedisClient))
					r.Use(authmiddleware.RequireScope(authscope.TemplatesWrite, s.logger))
					r.Use(authmiddleware.RequireAdmin)
					r.Post("/", deps.Handlers.GitTemplateHandler.Create)
					r.Put("/{uuid}", deps.Handlers.GitTemplateHandler.Update)
					r.Delete("/{uuid}", deps.Handlers.GitTemplateHandler.Delete)
					r.Post("/{uuid}/sync", deps.Handlers.GitTemplateHandler.Sync)
					r.Post("/{uuid}/download", deps.Handlers.GitTemplateHandler.Download)
				})
			})
			s.logger.Info("Git template API endpoints registered")
		}

		// External service endpoints
		if deps.Handlers.ExternalServiceHandler != nil {
			r.Route("/external-services", func(r chi.Router) {
				r.Group(func(r chi.Router) {
					r.Use(rateLimitByIP(60, time.Minute, deps.BareRedisClient))
					r.Use(authmiddleware.RequireScope(authscope.ServicesRead, s.logger))
					r.Get("/", deps.Handlers.ExternalServiceHandler.List)
					r.Get("/{uuid}", deps.Handlers.ExternalServiceHandler.Show)
				})

				r.Group(func(r chi.Router) {
					r.Use(rateLimitByIP(30, time.Minute, deps.BareRedisClient))
					r.Use(authmiddleware.RequireScope(authscope.ServicesWrite, s.logger))
					r.Use(authmiddleware.RequireAdmin)
					r.Post("/", deps.Handlers.ExternalServiceHandler.Create)
					r.Put("/{uuid}", deps.Handlers.ExternalServiceHandler.Update)
					r.Delete("/{uuid}", deps.Handlers.ExternalServiceHandler.Delete)
					r.Post("/{uuid}/health-check", deps.Handlers.ExternalServiceHandler.HealthCheck)
					r.Post("/{uuid}/sync", deps.Handlers.ExternalServiceHandler.Sync)
				})
			})
			s.logger.Info("External service API endpoints registered")
		}

		// Admin API endpoints (60/min, requires admin scope + role)
		r.Route("/admin", func(r chi.Router) {
			r.Use(rateLimitByIP(60, time.Minute, deps.BareRedisClient))
			r.Use(authmiddleware.RequireScope(authscope.Admin, s.logger))
			r.Use(authmiddleware.RequireAdmin)

			if deps.Handlers.SSEHandler != nil {
				r.Get("/events/stream", deps.Handlers.SSEHandler.Stream)
			}
			if deps.Handlers.DashboardHandler != nil {
				r.Get("/dashboard/stats", deps.Handlers.DashboardHandler.Stats)
			}

			if deps.Handlers.UserHandler != nil {
				// Create/Update are intentionally absent — DocuMCP is OIDC-only.
				// User rows are provisioned by the OIDC callback on first login
				// and synced from claims thereafter. See security.md H1.
				r.Route("/users", func(r chi.Router) {
					r.Get("/", deps.Handlers.UserHandler.List)
					r.Get("/{id}", deps.Handlers.UserHandler.Show)
					r.Delete("/{id}", deps.Handlers.UserHandler.Delete)
					r.Post("/{id}/toggle-admin", deps.Handlers.UserHandler.ToggleAdmin)
				})
			}

			if deps.Handlers.DocumentHandler != nil {
				r.Delete("/documents/purge", deps.Handlers.DocumentHandler.BulkPurge)
			}
			if deps.Handlers.ExternalServiceHandler != nil {
				r.Put("/external-services/reorder", deps.Handlers.ExternalServiceHandler.Reorder)
			}
			if deps.Handlers.GitTemplateHandler != nil {
				r.Post("/git-templates/validate-url", deps.Handlers.GitTemplateHandler.ValidateURL)
			}

			if deps.Handlers.OAuthClientHandler != nil {
				r.Route("/oauth-clients", func(r chi.Router) {
					r.Get("/", deps.Handlers.OAuthClientHandler.List)
					r.Post("/", deps.Handlers.OAuthClientHandler.Create)
					r.Get("/{id}", deps.Handlers.OAuthClientHandler.Show)
					r.Delete("/{id}", deps.Handlers.OAuthClientHandler.Delete)
					r.Get("/{id}/scope-grants", deps.Handlers.OAuthClientHandler.ListScopeGrants)
					r.Delete("/{id}/scope-grants/{grantId}", deps.Handlers.OAuthClientHandler.RevokeScopeGrant)
				})
			}

			if deps.Handlers.QueueHandler != nil {
				r.Route("/queue", func(r chi.Router) {
					r.Get("/stats", deps.Handlers.QueueHandler.Stats)
					r.Get("/failed", deps.Handlers.QueueHandler.ListFailed)
					r.Post("/failed/{id}/retry", deps.Handlers.QueueHandler.RetryFailed)
					r.Delete("/failed/{id}", deps.Handlers.QueueHandler.DeleteFailed)
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

	if deps.Handlers.RootAssetHandler != nil {
		r.Get("/favicon.ico", deps.Handlers.RootAssetHandler.ServeHTTP)
		r.Get("/favicon.svg", deps.Handlers.RootAssetHandler.ServeHTTP)
		r.Get("/favicon-96x96.png", deps.Handlers.RootAssetHandler.ServeHTTP)
		r.Get("/apple-touch-icon.png", deps.Handlers.RootAssetHandler.ServeHTTP)
		r.Get("/site.webmanifest", deps.Handlers.RootAssetHandler.ServeHTTP)
		r.Get("/web-app-manifest-192x192.png", deps.Handlers.RootAssetHandler.ServeHTTP)
		r.Get("/web-app-manifest-512x512.png", deps.Handlers.RootAssetHandler.ServeHTTP)
		r.Get("/openapi.yaml", deps.Handlers.RootAssetHandler.ServeHTTP)
	}

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusFound)
	})

	// River UI — must be registered before the SPA catch-all so chi matches it first.
	if deps.Handlers.RiverUIHandler != nil {
		r.Route("/admin/river", func(r chi.Router) {
			r.Use(authmiddleware.BearerOrSessionWithAudience(deps.Auth.OAuthService, deps.Auth.SessionStore, s.logger, deps.Auth.APIResource))
			r.Use(authmiddleware.RequireScope(authscope.Admin, s.logger))
			r.Use(authmiddleware.RequireAdmin)
			r.Use(riverUICSP)
			r.Handle("/*", deps.Handlers.RiverUIHandler)
		})
		s.logger.Info("River UI mounted", "path", "/admin/river/*")
	}

	if deps.Handlers.SPAHandler != nil {
		r.Get("/admin", http.RedirectHandler("/admin/", http.StatusMovedPermanently).ServeHTTP)
		r.Mount("/admin/", http.StripPrefix("/admin", deps.Handlers.SPAHandler))
		s.logger.Info("SPA handler registered", "path", "/admin/*")
	}
}

// riverUICSP applies a relaxed Content-Security-Policy for the River UI React app,
// which requires inline scripts and styles. This overrides the stricter default CSP
// set by the SecurityHeaders middleware.
func riverUICSP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline' 'unsafe-eval'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data:; "+
				"connect-src 'self'; "+
				"font-src 'self' data:")
		next.ServeHTTP(w, r)
	})
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
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   http.StatusText(status),
		"message": message,
	})
}

// PgxPoolPinger adapts *pgxpool.Pool to handler.DependencyPinger. Wrap the
// uninstrumented BarePgxPool — Ping on the instrumented pool emits
// otelpgx ping + pool.acquire spans on every probe.
type PgxPoolPinger struct {
	Pool *pgxpool.Pool
}

// Ping delegates to pgxpool's Ping. The caller's context carries the probe
// budget; readiness handlers cap this before invoking.
func (p *PgxPoolPinger) Ping(ctx context.Context) error {
	return p.Pool.Ping(ctx)
}

// redisClientPinger adapts *redis.Client to handler.DependencyPinger.
type redisClientPinger struct {
	client *redis.Client
}

// Ping checks Redis connectivity using the caller's context. Readiness
// handlers cap the probe budget before invoking, so no additional timeout
// is layered here.
func (p *redisClientPinger) Ping(ctx context.Context) error {
	return p.client.Ping(ctx).Err()
}
