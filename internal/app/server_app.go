package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/gorilla/sessions"
	"github.com/riverqueue/river"

	"github.com/c-premus/documcp/internal/auth/oauth"
	"github.com/c-premus/documcp/internal/auth/oidc"
	"github.com/c-premus/documcp/internal/client/kiwix"
	"github.com/c-premus/documcp/internal/config"
	apihandler "github.com/c-premus/documcp/internal/handler/api"
	mcphandler "github.com/c-premus/documcp/internal/handler/mcp"
	oauthhandler "github.com/c-premus/documcp/internal/handler/oauth"
	"github.com/c-premus/documcp/internal/queue"
	"github.com/c-premus/documcp/internal/server"
	"github.com/c-premus/documcp/internal/service"
	frontend "github.com/c-premus/documcp/web/frontend"
)

// ServerApp manages the HTTP server lifecycle. It can run in two modes:
//   - serve-only: River client is insert-only (enqueues jobs but does not process them)
//   - combined: River client processes jobs and serves HTTP (equivalent to the old monolith)
type ServerApp struct {
	Foundation  *Foundation
	Server      *server.Server
	RiverClient *queue.RiverClient
	EventBus    queue.EventSubscriber
	MCPHandler  *mcphandler.Handler
	WithWorker  bool
}

// NewServerApp creates a ServerApp with HTTP handlers and routes.
// When withWorker is true, the River client also processes jobs (combined mode).
func NewServerApp(f *Foundation, withWorker bool) (*ServerApp, error) {
	cfg := f.Config
	logger := f.Logger

	// --- Session Store ---
	sessionStore, sessionSecret, err := buildSessionStore(cfg, logger)
	if err != nil {
		return nil, err
	}

	// --- Token HMAC key ---
	hmacKey, err := deriveKey([]byte(sessionSecret), cfg.OAuth.HKDFSalt, "oauth-token-hmac")
	if err != nil {
		return nil, fmt.Errorf("deriving HMAC key: %w", err)
	}
	oauth.SetTokenHMACKey(hmacKey)

	// --- EventBus (Redis-backed for cross-instance SSE delivery) ---
	eventBus := queue.NewRedisEventBus(context.Background(), f.RedisClient, logger)

	// --- River Workers + Client ---
	rs, err := buildRiverClient(f, eventBus, !withWorker)
	if err != nil {
		return nil, err
	}
	riverClient := rs.Client

	logger.Info("river queue client configured",
		"mode", riverMode(withWorker),
	)

	// --- Services ---
	documentService := service.NewDocumentService(f.DocumentRepo, logger)
	documentPipeline := service.NewDocumentPipeline(
		documentService,
		f.ExtractorRegistry,
		riverClient,
		f.StoragePath,
	)

	// Wire pipeline into document workers (resolves circular dependency).
	// Workers are registered even in insert-only mode for job kind validation.
	rs.ExtractWorker.Pipeline = documentPipeline
	rs.ExtractWorker.Metrics = f.Metrics

	oauthService := oauth.NewService(f.OAuthRepo, cfg.OAuth, cfg.App.URL, logger)
	externalServiceSvc := service.NewExternalServiceService(
		f.ExternalServiceRepo,
		f.ZimArchiveRepo,
		nil, // search index cleaning handled by PostgreSQL FTS
		logger,
	)

	// --- OAuth Handler ---
	oauthH := oauthhandler.New(oauthhandler.Config{
		Service:      oauthService,
		SessionStore: sessionStore,
		OAuthCfg:     cfg.OAuth,
		AppURL:       cfg.App.URL,
		Logger:       logger,
	})

	// --- OIDC Handler ---
	oidcH, err := oidc.New(context.Background(), oidc.Config{
		OIDCCfg:      cfg.OIDC,
		SessionStore: sessionStore,
		Repo:         f.OAuthRepo,
		Logger:       logger,
	})
	if err != nil {
		logger.Warn("OIDC provider discovery failed, OIDC login disabled", "error", err)
	} else if oidcH != nil {
		logger.Info("OIDC provider configured", "provider_url", cfg.OIDC.ProviderURL)
	}

	// --- API Handlers ---
	documentH := apihandler.NewDocumentHandler(documentPipeline, f.DocumentRepo, logger)
	searchH := apihandler.NewSearchHandler(f.Searcher, f.SearchQueryRepo, f.DocumentRepo, logger)
	zimH := apihandler.NewZimHandler(f.ZimArchiveRepo, &apihandler.KiwixFactoryAdapter{Factory: f.KiwixFactory}, logger)
	gitTemplateH := apihandler.NewGitTemplateHandler(f.GitTemplateRepo, riverClient, logger)
	externalServiceH := apihandler.NewExternalServiceHandler(externalServiceSvc, f.ExternalServiceRepo, riverClient, f.KiwixFactory, logger)
	userH := apihandler.NewUserHandler(f.OAuthRepo, logger)
	oauthClientH := apihandler.NewOAuthClientHandler(f.OAuthRepo, logger)

	// --- Auth & SPA Handlers ---
	authH := apihandler.NewAuthHandler(sessionStore, f.OAuthRepo, logger)
	spaHandler := frontend.Handler()
	rootAssetHandler := frontend.RootAssetHandler()

	// --- Dashboard Handler ---
	dashboardH := apihandler.NewDashboardHandler(
		f.DocumentRepo,
		f.OAuthRepo,
		f.OAuthRepo,
		f.ExternalServiceRepo,
		f.ZimArchiveRepo,
		f.GitTemplateRepo,
		riverClient,
		logger,
	)

	// --- SSE & Queue Handlers ---
	sseH := apihandler.NewSSEHandler(eventBus, cfg.Server.SSEHeartbeatInterval)
	queueH := apihandler.NewQueueHandler(riverClient, logger)

	// --- MCP Handler ---
	mcpCfg := mcphandler.Config{
		ServerName:          cfg.DocuMCP.ServerName,
		ServerVersion:       cfg.DocuMCP.ServerVersion,
		Logger:              logger,
		DocumentService:     documentService,
		DocumentRepo:        f.DocumentRepo,
		SearchQueryRepo:     f.SearchQueryRepo,
		ExternalServiceRepo: f.ExternalServiceRepo,
		ZimArchiveRepo:      f.ZimArchiveRepo,
		GitTemplateRepo:     f.GitTemplateRepo,
		ZimEnabled:          true,
		GitTemplatesEnabled: true,

		FederatedSearchTimeout:   cfg.Kiwix.FederatedSearchTimeout,
		FederatedMaxArchives:     cfg.Kiwix.FederatedMaxArchives,
		FederatedPerArchiveLimit: cfg.Kiwix.FederatedPerArchiveLimit,
	}
	mcpCfg.KiwixFactory = f.KiwixFactory
	mcpCfg.Searcher = f.Searcher
	mcpH := mcphandler.New(mcpCfg)

	// --- HTTP Server ---
	trustedProxies, err := config.ParseCIDRs(cfg.Server.TrustedProxies)
	if err != nil {
		return nil, fmt.Errorf("parsing TRUSTED_PROXIES: %w", err)
	}
	if len(trustedProxies) > 0 {
		names := make([]string, len(trustedProxies))
		for i, n := range trustedProxies {
			names[i] = n.String()
		}
		logger.Info("trusted proxies configured", "cidrs", strings.Join(names, ", "))
	}

	srv := server.New(server.Config{
		Host:              cfg.Server.Host,
		Port:              cfg.Server.Port,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		TrustedProxies:    trustedProxies,
	}, logger)

	srv.RegisterRoutes(server.Deps{
		RateLimitRedisClient:   f.RateLimitRedisClient,
		RedisClient:            f.RedisClient,
		Version:                cfg.DocuMCP.ServerVersion,
		MCPHandler:             mcpH,
		OAuthHandler:           oauthH,
		OIDCHandler:            oidcH,
		OAuthService:           oauthService,
		SessionStore:           sessionStore,
		DocumentHandler:        documentH,
		SearchHandler:          searchH,
		ZimHandler:             zimH,
		GitTemplateHandler:     gitTemplateH,
		ExternalServiceHandler: externalServiceH,
		UserHandler:            userH,
		OAuthClientHandler:     oauthClientH,
		AuthHandler:            authH,
		SPAHandler:             spaHandler,
		RootAssetHandler:       rootAssetHandler,
		DashboardHandler:       dashboardH,
		SSEHandler:             sseH,
		QueueHandler:           queueH,
		Metrics:                f.Metrics,
		OTELEnabled:            cfg.OTEL.Enabled,
		IsSecure:               cfg.App.Env == "production",
		DB:                     f.PgxPool,
		InternalAPIToken:       cfg.App.InternalAPIToken,
		MaxBodySize:            cfg.Server.MaxBodySize,
		RequestTimeout:         cfg.Server.RequestTimeout,
		HSTSMaxAge:             cfg.Server.HSTSMaxAge,
	})

	logger.Info("HTTP server configured",
		"name", cfg.DocuMCP.ServerName,
		"version", cfg.DocuMCP.ServerVersion,
		"mode", riverMode(withWorker),
	)

	return &ServerApp{
		Foundation:  f,
		Server:      srv,
		RiverClient: riverClient,
		EventBus:    eventBus,
		MCPHandler:  mcpH,
		WithWorker:  withWorker,
	}, nil
}

// Start runs the HTTP server and blocks until a shutdown signal is received.
func (s *ServerApp) Start(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start River client (no-op in insert-only mode).
	if err := s.RiverClient.Start(ctx); err != nil {
		return fmt.Errorf("starting river client: %w", err)
	}
	if s.WithWorker {
		s.Foundation.Logger.Info("river queue started (combined mode)")
		s.RiverClient.StartEventForwarding()
		queue.RecoverStuckDocuments(ctx, s.RiverClient, newDocStatusAdapter(s.Foundation.DocumentRepo), s.Foundation.Logger)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Server.Start()
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		s.Foundation.Logger.Info("shutdown signal received")
	}

	// Close EventBus and MCP sessions so SSE connections exit immediately.
	if s.EventBus != nil {
		s.EventBus.Close()
	}
	if s.MCPHandler != nil {
		s.MCPHandler.Close()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.Foundation.Config.Server.ShutdownTimeout)
	defer cancel()

	if err := s.Server.Shutdown(shutdownCtx); err != nil {
		s.Foundation.Logger.Warn("graceful shutdown timed out, forcing connection close")
		if closeErr := s.Server.Close(); closeErr != nil {
			s.Foundation.Logger.Error("force-closing server", "error", closeErr)
		}
	}

	s.Foundation.Logger.Info("server stopped")
	return nil
}

// Close releases ServerApp-specific resources.
func (s *ServerApp) Close() error {
	// Close EventBus first to stop Pub/Sub goroutine (matches WorkerApp pattern).
	if s.EventBus != nil {
		s.EventBus.Close()
	}
	if s.RiverClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), s.Foundation.Config.App.QueueStopTimeout)
		defer cancel()
		if err := s.RiverClient.Stop(ctx); err != nil {
			s.Foundation.Logger.Error("stopping river client", "error", err)
		}
	}
	return nil
}

// buildSessionStore creates a gorilla CookieStore with HKDF-derived encryption keys.
func buildSessionStore(cfg *config.Config, logger *slog.Logger) (*sessions.CookieStore, string, error) {
	sessionSecret := cfg.OAuth.SessionSecret
	if sessionSecret == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return nil, "", fmt.Errorf("generating session secret: %w", err)
		}
		sessionSecret = hex.EncodeToString(b)
		logger.Warn("no OAUTH_SESSION_SECRET configured, using random secret (sessions will not survive restarts)")
	}

	sessionEncKey, err := deriveKey([]byte(sessionSecret), cfg.OAuth.HKDFSalt, "session-cookie-encryption")
	if err != nil {
		return nil, "", fmt.Errorf("deriving session encryption key: %w", err)
	}

	keyPairs := [][]byte{[]byte(sessionSecret), sessionEncKey}
	if prev := cfg.OAuth.SessionSecretPrevious; prev != "" {
		oldEncKey, encErr := deriveKey([]byte(prev), cfg.OAuth.HKDFSalt, "session-cookie-encryption")
		if encErr != nil {
			return nil, "", fmt.Errorf("deriving previous session encryption key: %w", encErr)
		}
		keyPairs = append(keyPairs, []byte(prev), oldEncKey)
		logger.Info("session key rotation enabled (previous key configured)")
	}

	store := sessions.NewCookieStore(keyPairs...)
	store.Options = &sessions.Options{
		Path:     "/",
		HttpOnly: true,
		Secure:   strings.HasPrefix(cfg.App.URL, "https://"),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(cfg.OAuth.SessionMaxAge.Seconds()),
	}

	return store, sessionSecret, nil
}

// riverSetup holds the result of buildRiverClient, including references to
// document workers that need post-creation wiring (circular dependency).
type riverSetup struct {
	Client        *queue.RiverClient
	ExtractWorker *queue.DocumentExtractWorker
}

// buildRiverClient creates the River client with all workers registered.
// When insertOnly is true, queues and periodic jobs are omitted.
// Returns the client and document worker references for pipeline wiring.
func buildRiverClient(f *Foundation, eventBus queue.EventPublisher, insertOnly bool) (*riverSetup, error) {
	cfg := f.Config

	schedulerDeps := queue.SchedulerDeps{
		Services:          f.ExternalServiceRepo,
		HealthChecker:     f.ExternalServiceRepo,
		ZimRepo:           f.ZimArchiveRepo,
		GitRepo:           f.GitTemplateRepo,
		OAuthRepo:         f.OAuthRepo,
		DocRepo:           f.DocumentRepo,
		Metrics:           f.Metrics,
		GitTempDir:        f.GitTempDir,
		StoragePath:       f.StoragePath,
		Logger:            f.Logger,
		GitMaxFileSize:    cfg.Git.MaxFileSize,
		GitMaxTotalSize:   cfg.Git.MaxTotalSize,
		SSRFDialerTimeout: cfg.App.SSRFDialerTimeout,
		KiwixConfig: kiwix.ClientConfig{
			HTTPTimeout:        cfg.Kiwix.HTTPTimeout,
			HealthCheckTimeout: cfg.Kiwix.HealthCheckTimeout,
			CacheTTL:           cfg.Kiwix.CacheTTL,
			SSRFDialerTimeout:  cfg.App.SSRFDialerTimeout,
		},
	}

	// Document workers created with nil deps — wired after pipeline creation.
	extractWorker := &queue.DocumentExtractWorker{}

	workers := river.NewWorkers()
	river.AddWorker(workers, extractWorker)

	// Scheduler workers.
	river.AddWorker(workers, &queue.SyncKiwixWorker{Deps: schedulerDeps})
	river.AddWorker(workers, &queue.SyncGitTemplatesWorker{Deps: schedulerDeps})
	river.AddWorker(workers, &queue.CleanupOAuthTokensWorker{Deps: schedulerDeps})
	river.AddWorker(workers, &queue.CleanupOrphanedFilesWorker{Deps: schedulerDeps})
	river.AddWorker(workers, &queue.PurgeSoftDeletedWorker{Deps: schedulerDeps})
	river.AddWorker(workers, &queue.HealthCheckServicesWorker{Deps: schedulerDeps})

	// Periodic jobs (only when processing jobs).
	var periodicJobs []*river.PeriodicJob
	if !insertOnly && cfg.Scheduler.Enabled {
		periodicJobs = queue.BuildPeriodicJobs(cfg.Scheduler, f.Logger)
		f.Logger.Info("periodic jobs configured",
			"count", len(periodicJobs),
			"kiwix_schedule", cfg.Scheduler.KiwixSchedule,
			"git_schedule", cfg.Scheduler.GitSchedule,
		)
	}

	queueWorkers := map[string]int{
		"high":    cfg.Queue.HighWorkers,
		"default": cfg.Queue.DefaultWorkers,
		"low":     cfg.Queue.LowWorkers,
	}

	riverClient, err := queue.NewRiverClient(queue.RiverConfig{
		Pool:         f.PgxPool,
		EventBus:     eventBus,
		Logger:       f.Logger,
		Metrics:      f.Metrics,
		Workers:      workers,
		PeriodicJobs: periodicJobs,
		InsertOnly:   insertOnly,
		QueueWorkers: queueWorkers,
	})
	if err != nil {
		return nil, fmt.Errorf("creating river client: %w", err)
	}

	return &riverSetup{
		Client:        riverClient,
		ExtractWorker: extractWorker,
	}, nil
}

// riverMode returns a human-readable label for logging.
func riverMode(withWorker bool) string {
	if withWorker {
		return "combined"
	}
	return "insert-only"
}
