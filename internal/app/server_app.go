package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
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

	"riverqueue.com/riverui"

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
	Foundation     *Foundation
	Server         *server.Server
	RiverClient    *queue.RiverClient
	RiverUIHandler *riverui.Handler
	EventBus       queue.EventSubscriber
	MCPHandler     *mcphandler.Handler
	WithWorker     bool
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

	// --- Token HMAC keys ---
	// Primary key derived from OAUTH_SESSION_SECRET, written as version '1'.
	// Previous key (if OAUTH_SESSION_SECRET_PREVIOUS is set) is version '2':
	// accepted on verify so rotation does not silently invalidate live tokens.
	hmacKeys, err := buildHMACKeys(cfg, sessionSecret)
	if err != nil {
		return nil, fmt.Errorf("building token HMAC keys: %w", err)
	}

	// --- EventBus (Redis-backed for cross-instance SSE delivery) ---
	// Uses the Foundation context so the pub/sub consumer goroutine exits
	// when Foundation.Close cancels, alongside eventBus.Close (architecture A3).
	eventBus, err := queue.NewRedisEventBus(f.Ctx(), f.RedisClient, logger)
	if err != nil {
		return nil, fmt.Errorf("redis event bus: %w", err)
	}
	var eventBusOK bool
	defer func() {
		if !eventBusOK {
			eventBus.Close()
		}
	}()

	// --- Control Bus Subscriptions ---
	// Remote replicas publish on this topic after admin-UI edits to
	// external services; we clear our local kiwix factory cache so the
	// next request re-reads from Postgres. Foundation.Ctx is canceled at
	// shutdown so the subscriber goroutine has a second exit path on top of
	// Foundation.ControlBus.Close (architecture A3).
	if f.ControlBus != nil {
		subErr := f.ControlBus.Subscribe(f.Ctx(), apihandler.KiwixCacheInvalidateTopic, func(_ []byte) {
			f.KiwixFactory.Invalidate()
			logger.Info("kiwix cache invalidated by control bus")
		})
		if subErr != nil {
			return nil, fmt.Errorf("subscribing to kiwix cache invalidation: %w", subErr)
		}
	}

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
		f.BlobStore,
		f.WorkerTempDir,
		f.Config.Storage.MaxUploadSize,
	)

	// Wire pipeline into document workers (resolves circular dependency).
	// Workers are registered even in insert-only mode for job kind validation.
	rs.ExtractWorker.Pipeline = documentPipeline
	rs.ExtractWorker.Metrics = f.Metrics

	oauthService, err := oauth.NewService(f.OAuthRepo, cfg.OAuth, cfg.App.URL, logger, hmacKeys)
	if err != nil {
		return nil, fmt.Errorf("creating oauth service: %w", err)
	}
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
		TokenRevoker: oauthService,
		Logger:       logger,
		AppURL:       cfg.App.URL,
	})
	if err != nil {
		logger.Warn("OIDC provider discovery failed, OIDC login disabled", "error", err)
	} else if oidcH != nil {
		logger.Info("OIDC provider configured", "provider_url", cfg.OIDC.ProviderURL)
	}

	// --- API Handlers ---
	documentH := apihandler.NewDocumentHandler(documentPipeline, f.BlobStore, f.WorkerTempDir, logger)
	searchH := apihandler.NewSearchHandler(f.Searcher, f.SearchQueryRepo, f.DocumentRepo, logger)
	zimH := apihandler.NewZimHandler(f.ZimArchiveRepo, &apihandler.KiwixFactoryAdapter{Factory: f.KiwixFactory}, logger)
	gitTemplateSvc := service.NewGitTemplateService(f.GitTemplateRepo, riverClient, f.Encryptor, logger)
	gitTemplateH := apihandler.NewGitTemplateHandler(gitTemplateSvc, f.GitTemplateRepo, logger)
	externalServiceH := apihandler.NewExternalServiceHandler(externalServiceSvc, f.ExternalServiceRepo, riverClient, f.KiwixFactory, f.ControlBus, logger)
	userH := apihandler.NewUserHandler(f.OAuthRepo, logger)
	oauthClientSvc := service.NewOAuthClientService(f.OAuthRepo, logger)
	oauthClientH := apihandler.NewOAuthClientHandler(f.OAuthRepo, oauthClientSvc, logger)

	// --- Auth & SPA Handlers ---
	authH := apihandler.NewAuthHandler(logger)
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

	// --- River UI Handler ---
	var riverUIHandler *riverui.Handler
	riverUIH, riverUIErr := queue.NewRiverUIHandler(riverClient.Client(), logger, "/admin/river")
	if riverUIErr != nil {
		logger.Warn("river UI handler creation failed, river UI disabled", "error", riverUIErr)
	} else {
		riverUIHandler = riverUIH
		logger.Info("river UI handler configured", "prefix", "/admin/river")
	}

	// --- MCP Handler ---
	mcpCfg := mcphandler.Config{
		ServerName:          cfg.DocuMCP.ServerName,
		ServerVersion:       cfg.DocuMCP.ServerVersion,
		AppURL:              cfg.App.URL,
		Logger:              logger,
		DocumentService:     documentService,
		DocumentRepo:        f.DocumentRepo,
		ExternalServiceRepo: f.ExternalServiceRepo,
		ZimArchiveRepo:      f.ZimArchiveRepo,
		GitTemplateRepo:     f.GitTemplateRepo,
		GitTemplateService:  gitTemplateSvc,
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
		TLS: server.TLSConfig{
			Enabled:  cfg.Server.TLSEnabled,
			Port:     cfg.Server.TLSPort,
			CertFile: cfg.Server.TLSCertFile,
			KeyFile:  cfg.Server.TLSKeyFile,
		},
	}, logger)

	srv.RegisterRoutes(server.Deps{
		Version: cfg.DocuMCP.ServerVersion,
		Handlers: server.Handlers{
			MCPHandler:             mcpH,
			DocumentHandler:        documentH,
			SearchHandler:          searchH,
			ZimHandler:             zimH,
			GitTemplateHandler:     gitTemplateH,
			ExternalServiceHandler: externalServiceH,
			UserHandler:            userH,
			OAuthClientHandler:     oauthClientH,
			SSEHandler:             sseH,
			QueueHandler:           queueH,
			DashboardHandler:       dashboardH,
			RiverUIHandler:         riverUIHandler,
			AuthHandler:            authH,
			SPAHandler:             spaHandler,
			RootAssetHandler:       rootAssetHandler,
		},
		Auth: server.Auth{
			OAuthHandler:          oauthH,
			OIDCHandler:           oidcH,
			OAuthService:          oauthService,
			SessionStore:          sessionStore,
			MCPResource:           cfg.App.URL + cfg.DocuMCP.Endpoint,
			APIResource:           cfg.App.URL,
			SessionAbsoluteMaxAge: cfg.OAuth.SessionAbsoluteMaxAge,
		},
		Tuning: server.Tuning{
			MaxBodySize:      cfg.Server.MaxBodySize,
			RequestTimeout:   cfg.Server.RequestTimeout,
			HSTSMaxAge:       cfg.Server.HSTSMaxAge,
			InternalAPIToken: cfg.App.InternalAPIToken,
		},
		Metrics:         f.Metrics,
		OTELEnabled:     cfg.OTEL.Enabled,
		BareRedisClient: f.BareRedisClient,
		RedisClient:     f.RedisClient,
		DB:              &server.PgxPoolPinger{Pool: f.BarePgxPool},
	})

	logger.Info("HTTP server configured",
		"name", cfg.DocuMCP.ServerName,
		"version", cfg.DocuMCP.ServerVersion,
		"mode", riverMode(withWorker),
	)

	eventBusOK = true
	return &ServerApp{
		Foundation:     f,
		Server:         srv,
		RiverClient:    riverClient,
		RiverUIHandler: riverUIHandler,
		EventBus:       eventBus,
		MCPHandler:     mcpH,
		WithWorker:     withWorker,
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

	// Start River UI background services (auto-stops when ctx is canceled).
	if s.RiverUIHandler != nil {
		if err := s.RiverUIHandler.Start(ctx); err != nil {
			s.Foundation.Logger.Warn("river UI handler start failed", "error", err)
		}
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

// buildHMACKeys derives the OAuth token HMAC keys from OAUTH_SESSION_SECRET
// (primary) and optionally OAUTH_SESSION_SECRET_PREVIOUS, returning them
// newest-first. Each key's Version byte is the first hex character of
// SHA-256(secret), giving a stable per-secret identifier so hashes stored
// before a rotation still verify under the retired key (security M2).
//
// A 1-in-16 collision on the derived version is caught at boot — distinct
// secrets must derive distinct versions so the stored prefix identifies
// exactly one key. Operators resolve the collision by regenerating one of
// the secrets.
//
// Returns an error when the primary key would be empty — production boots
// fail-fast rather than falling back to unkeyed SHA-256 (security L4).
func buildHMACKeys(cfg *config.Config, sessionSecret string) ([]oauth.HMACKey, error) {
	primary, err := deriveKey([]byte(sessionSecret), cfg.OAuth.HKDFSalt, "oauth-token-hmac")
	if err != nil {
		return nil, fmt.Errorf("deriving primary HMAC key: %w", err)
	}
	keys := []oauth.HMACKey{{Version: hmacVersionByte(sessionSecret), Key: primary}}

	if prev := cfg.OAuth.SessionSecretPrevious; prev != "" {
		previous, prevErr := deriveKey([]byte(prev), cfg.OAuth.HKDFSalt, "oauth-token-hmac")
		if prevErr != nil {
			return nil, fmt.Errorf("deriving previous HMAC key: %w", prevErr)
		}
		prevVersion := hmacVersionByte(prev)
		if prevVersion == keys[0].Version {
			return nil, fmt.Errorf("OAUTH_SESSION_SECRET and OAUTH_SESSION_SECRET_PREVIOUS derive to the same HMAC key version %q — regenerate one secret", prevVersion)
		}
		keys = append(keys, oauth.HMACKey{Version: prevVersion, Key: previous})
	}
	return keys, nil
}

// hmacVersionByte returns a stable single-byte identifier for an HMAC key
// derived from secret. Uses the first hex character of SHA-256(secret) so
// every distinct secret maps to a distinct version with high probability
// (15/16 per added secret). Rotating the primary secret keeps the retired
// key's version byte stable, so stored hashes under that key continue to
// verify after rotation.
func hmacVersionByte(secret string) byte {
	sum := sha256.Sum256([]byte(secret))
	const hexAlphabet = "0123456789abcdef"
	return hexAlphabet[sum[0]>>4]
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
		Secure:   strings.HasPrefix(cfg.App.URL, "https://") || cfg.Server.TLSEnabled,
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
		Blob:              f.BlobStore,
		GitTempDir:        f.GitTempDir,
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
