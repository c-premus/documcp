// Package app wires together all application dependencies and manages lifecycle.
package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/crypto/hkdf"

	"github.com/gorilla/sessions"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jmoiron/sqlx"
	"github.com/riverqueue/river"

	"github.com/c-premus/documcp/internal/auth/oauth"
	"github.com/c-premus/documcp/internal/auth/oidc"
	"github.com/c-premus/documcp/internal/client/kiwix"
	"github.com/c-premus/documcp/internal/config"
	"github.com/c-premus/documcp/internal/crypto"
	"github.com/c-premus/documcp/internal/database"
	"github.com/c-premus/documcp/internal/extractor"
	docxext "github.com/c-premus/documcp/internal/extractor/docx"
	htmlext "github.com/c-premus/documcp/internal/extractor/html"
	markdownext "github.com/c-premus/documcp/internal/extractor/markdown"
	pdfext "github.com/c-premus/documcp/internal/extractor/pdf"
	xlsxext "github.com/c-premus/documcp/internal/extractor/xlsx"
	apihandler "github.com/c-premus/documcp/internal/handler/api"
	mcphandler "github.com/c-premus/documcp/internal/handler/mcp"
	oauthhandler "github.com/c-premus/documcp/internal/handler/oauth"
	frontend "github.com/c-premus/documcp/web/frontend"

	"github.com/c-premus/documcp/internal/observability"
	"github.com/c-premus/documcp/internal/queue"
	"github.com/c-premus/documcp/internal/repository"
	"github.com/c-premus/documcp/internal/search"
	"github.com/c-premus/documcp/internal/server"
	"github.com/c-premus/documcp/internal/service"
)

// App holds all application dependencies wired together.
type App struct {
	Config          *config.Config
	DB              *sqlx.DB
	Logger          *slog.Logger
	Metrics         *observability.Metrics
	Server          *server.Server
	PgxPool         *pgxpool.Pool
	RiverClient     *queue.RiverClient
	EventBus        *queue.EventBus
	docStatusFinder queue.DocumentStatusFinder
	tracerShutdown  func(context.Context) error
}

// New creates a new App, wiring all dependencies together.
func New(cfg *config.Config) (*App, error) {
	logger := newLogger(cfg.App.Env, cfg.App.Debug, os.Stdout)

	db, err := database.New(
		cfg.DatabaseDSN(),
		cfg.Database.MaxOpenConns,
		cfg.Database.MaxIdleConns,
		cfg.Database.MaxLifetime,
	)
	if err != nil {
		return nil, fmt.Errorf("initializing database: %w", err)
	}

	logger.Info("database connected",
		"host", cfg.Database.Host,
		"database", cfg.Database.Database,
	)

	if err = database.RunMigrations(db.DB, "migrations"); err != nil {
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	logger.Info("database migrations applied")

	// --- pgxpool for River ---
	pgxPool, err := database.NewPgxPool(
		context.Background(),
		cfg.DatabaseDSN(),
		10,
		cfg.Database.PgxMinConns,
		cfg.Database.PgxMaxConnLifetime,
		cfg.Database.PgxMaxConnIdleTime,
	)
	if err != nil {
		return nil, fmt.Errorf("creating pgxpool for river: %w", err)
	}

	logger.Info("pgxpool connected for river queue")

	if err = database.RunRiverMigrations(context.Background(), pgxPool); err != nil {
		pgxPool.Close()
		return nil, fmt.Errorf("running river migrations: %w", err)
	}

	logger.Info("river schema migrations applied")

	// --- Session Store ---
	sessionSecret := cfg.OAuth.SessionSecret
	if sessionSecret == "" {
		// Generate a random secret for development
		b := make([]byte, 32)
		if _, err = rand.Read(b); err != nil {
			return nil, fmt.Errorf("generating session secret: %w", err)
		}
		sessionSecret = hex.EncodeToString(b)
		logger.Warn("no OAUTH_SESSION_SECRET configured, using random secret (sessions will not survive restarts)")
	}
	// Derive a separate encryption key for session cookies (AES-256).
	// This ensures session data (user_id, is_admin, email) is not visible in base64.
	sessionEncKey, err := deriveKey([]byte(sessionSecret), cfg.OAuth.HKDFSalt, "session-cookie-encryption", 32)
	if err != nil {
		return nil, fmt.Errorf("deriving session encryption key: %w", err)
	}

	// Key rotation: gorilla CookieStore accepts alternating (auth, enc) key pairs.
	// New keys are tried first for signing; old keys are tried as fallback for verification.
	keyPairs := [][]byte{[]byte(sessionSecret), sessionEncKey}
	if prev := cfg.OAuth.SessionSecretPrevious; prev != "" {
		oldEncKey, encErr := deriveKey([]byte(prev), cfg.OAuth.HKDFSalt, "session-cookie-encryption", 32)
		if encErr != nil {
			return nil, fmt.Errorf("deriving previous session encryption key: %w", encErr)
		}
		keyPairs = append(keyPairs, []byte(prev), oldEncKey)
		logger.Info("session key rotation enabled (previous key configured)")
	}
	sessionStore := sessions.NewCookieStore(keyPairs...)
	sessionStore.Options = &sessions.Options{
		Path:     "/",
		HttpOnly: true,
		Secure:   strings.HasPrefix(cfg.App.URL, "https://"),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(cfg.OAuth.SessionMaxAge.Seconds()),
	}

	// --- Encryption ---
	var encryptor *crypto.Encryptor
	if cfg.App.EncryptionKey != "" {
		var encErr error
		encryptor, encErr = crypto.NewEncryptor([]byte(cfg.App.EncryptionKey))
		if encErr != nil {
			return nil, fmt.Errorf("initializing encryptor: %w", encErr)
		}
		logger.Info("encryption at rest enabled")
	} else {
		logger.Warn("ENCRYPTION_KEY not set, git tokens will be stored in plaintext")
	}

	// --- Token HMAC key ---
	hmacKey, err := deriveKey([]byte(sessionSecret), cfg.OAuth.HKDFSalt, "oauth-token-hmac", 32)
	if err != nil {
		return nil, fmt.Errorf("deriving HMAC key: %w", err)
	}
	oauth.SetTokenHMACKey(hmacKey)

	// --- Repositories ---
	documentRepo := repository.NewDocumentRepository(db, logger)
	externalServiceRepo := repository.NewExternalServiceRepository(db, logger)
	zimArchiveRepo := repository.NewZimArchiveRepository(db, logger)
	gitTemplateRepo := repository.NewGitTemplateRepository(db, logger, encryptor)
	searchQueryRepo := repository.NewSearchQueryRepository(db, logger)
	oauthRepo := repository.NewOAuthRepository(db, logger)

	// --- Meilisearch ---
	var searchClient *search.Client
	var searchIndexer *search.Indexer
	var searcher *search.Searcher

	if cfg.Meilisearch.Host != "" {
		searchClient = search.NewClient(cfg.Meilisearch.Host, cfg.Meilisearch.Key, logger)
		if searchClient.Healthy() {
			if err = searchClient.ConfigureIndexes(context.Background()); err != nil {
				logger.Warn("failed to configure Meilisearch indexes", "error", err)
			} else {
				logger.Info("Meilisearch connected and indexes configured", "host", cfg.Meilisearch.Host)
			}
			searchIndexer = search.NewIndexer(searchClient, logger)
			searcher = search.NewSearcher(searchClient, logger)
			// Metrics are wired after NewMetrics() below.
		} else {
			logger.Warn("Meilisearch not reachable, search features disabled", "host", cfg.Meilisearch.Host)
		}
	}

	// --- External Service Clients ---
	// Kiwix client is lazy-initialized on first use from the database.
	kiwixFactory := kiwix.NewClientFactory(externalServiceRepo, kiwix.ClientConfig{
		HTTPTimeout:        cfg.Kiwix.HTTPTimeout,
		HealthCheckTimeout: cfg.Kiwix.HealthCheckTimeout,
		CacheTTL:           cfg.Kiwix.CacheTTL,
		SSRFDialerTimeout:  cfg.App.SSRFDialerTimeout,
	}, logger)

	// --- Content Extractors ---
	extractorRegistry := extractor.NewRegistry(
		pdfext.New(),
		docxext.New(),
		xlsxext.New(),
		htmlext.New(),
		markdownext.New(),
	)

	// --- Storage ---
	storagePath := filepath.Join(cfg.Storage.BasePath, cfg.Storage.DocumentPath)
	if err = os.MkdirAll(storagePath, 0o750); err != nil {
		return nil, fmt.Errorf("creating document storage path: %w", err)
	}

	gitTempDir := filepath.Join(cfg.Storage.BasePath, "git")
	if err = os.MkdirAll(gitTempDir, 0o750); err != nil {
		return nil, fmt.Errorf("creating git temp path: %w", err)
	}

	// --- Observability (Metrics) ---
	metrics := observability.NewMetrics()
	observability.RegisterDBMetrics(db.DB)
	if searcher != nil {
		searcher.SetMetrics(metrics)
	}
	logger.Info("Prometheus metrics registered")

	// --- EventBus ---
	eventBus := queue.NewEventBus(logger)

	// --- River Workers ---
	schedulerDeps := queue.SchedulerDeps{
		Services:          externalServiceRepo,
		HealthChecker:     externalServiceRepo,
		ZimRepo:           zimArchiveRepo,
		GitRepo:           gitTemplateRepo,
		OAuthRepo:         oauthRepo,
		DocRepo:           documentRepo,
		Indexer:           searchIndexer,
		GitTempDir:        gitTempDir,
		StoragePath:       storagePath,
		Logger:            logger,
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

	// Create document workers with nil dependencies (wired after pipeline creation).
	extractWorker := &queue.DocumentExtractWorker{}
	indexWorker := &queue.DocumentIndexWorker{}
	reindexWorker := &queue.ReindexAllWorker{}

	workers := river.NewWorkers()
	river.AddWorker(workers, extractWorker)
	river.AddWorker(workers, indexWorker)
	river.AddWorker(workers, reindexWorker)

	// Register scheduler migration workers.
	river.AddWorker(workers, &queue.SyncKiwixWorker{Deps: schedulerDeps})
	river.AddWorker(workers, &queue.SyncGitTemplatesWorker{Deps: schedulerDeps})
	river.AddWorker(workers, &queue.CleanupOAuthTokensWorker{Deps: schedulerDeps})
	river.AddWorker(workers, &queue.CleanupOrphanedFilesWorker{Deps: schedulerDeps})
	river.AddWorker(workers, &queue.VerifySearchIndexWorker{Deps: schedulerDeps})
	river.AddWorker(workers, &queue.PurgeSoftDeletedWorker{Deps: schedulerDeps})
	river.AddWorker(workers, &queue.CleanupDisabledZimWorker{Deps: schedulerDeps})
	river.AddWorker(workers, &queue.HealthCheckServicesWorker{Deps: schedulerDeps})

	// Build periodic jobs from scheduler config (only if scheduler is enabled).
	var periodicJobs []*river.PeriodicJob
	if cfg.Scheduler.Enabled {
		periodicJobs = queue.BuildPeriodicJobs(cfg.Scheduler, logger)
		logger.Info("periodic jobs configured",
			"count", len(periodicJobs),
			"kiwix_schedule", cfg.Scheduler.KiwixSchedule,
			"git_schedule", cfg.Scheduler.GitSchedule,
		)
	}

	// Create River client.
	riverClient, err := queue.NewRiverClient(queue.RiverConfig{
		Pool:         pgxPool,
		EventBus:     eventBus,
		Logger:       logger,
		Metrics:      metrics,
		Workers:      workers,
		PeriodicJobs: periodicJobs,
	})
	if err != nil {
		pgxPool.Close()
		return nil, fmt.Errorf("creating river client: %w", err)
	}

	logger.Info("river queue client configured")

	// --- Services ---
	documentService := service.NewDocumentService(documentRepo, logger)
	documentPipeline := service.NewDocumentPipeline(
		documentService,
		extractorRegistry,
		searchIndexer,
		riverClient,
		storagePath,
	)

	// Wire pipeline and metrics into document workers (resolves circular dependency).
	extractWorker.Pipeline = documentPipeline
	extractWorker.Metrics = metrics
	indexWorker.Indexer = documentPipeline
	indexWorker.Metrics = metrics
	reindexWorker.Indexer = documentPipeline
	reindexWorker.Lister = documentRepo
	reindexWorker.Logger = logger

	oauthService := oauth.NewService(oauthRepo, cfg.OAuth, cfg.App.URL, logger)
	var extSvcIndexCleaner service.ExternalServiceIndexCleaner
	if searchIndexer != nil {
		extSvcIndexCleaner = searchIndexer
	}
	externalServiceSvc := service.NewExternalServiceService(
		externalServiceRepo,
		zimArchiveRepo,
		extSvcIndexCleaner,
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
		Repo:         oauthRepo,
		Logger:       logger,
	})
	if err != nil {
		logger.Warn("OIDC provider discovery failed, OIDC login disabled", "error", err)
	} else if oidcH != nil {
		logger.Info("OIDC provider configured", "provider_url", cfg.OIDC.ProviderURL)
	}

	// --- API Handlers ---
	var docIndexer apihandler.DocumentIndexer
	if searchIndexer != nil {
		docIndexer = searchIndexer
	}
	documentH := apihandler.NewDocumentHandler(documentPipeline, documentRepo, docIndexer, logger)
	var searchH *apihandler.SearchHandler
	if searcher != nil {
		searchH = apihandler.NewSearchHandler(searcher, searchQueryRepo, documentRepo, logger)
	}
	zimH := apihandler.NewZimHandler(zimArchiveRepo, &apihandler.KiwixFactoryAdapter{Factory: kiwixFactory}, logger)
	gitTemplateH := apihandler.NewGitTemplateHandler(gitTemplateRepo, riverClient, logger)
	externalServiceH := apihandler.NewExternalServiceHandler(externalServiceSvc, externalServiceRepo, riverClient, kiwixFactory, logger)
	userH := apihandler.NewUserHandler(oauthRepo, logger)
	oauthClientH := apihandler.NewOAuthClientHandler(oauthRepo, logger)

	// --- Auth & SPA Handlers ---
	authH := apihandler.NewAuthHandler(sessionStore, oauthRepo, logger)
	spaHandler := frontend.Handler()

	// --- Dashboard Handler ---
	dashboardH := apihandler.NewDashboardHandler(
		documentRepo,
		oauthRepo,
		oauthRepo,
		externalServiceRepo,
		zimArchiveRepo,
		gitTemplateRepo,
		riverClient,
		logger,
	)

	// --- SSE & Queue Handlers ---
	sseH := apihandler.NewSSEHandler(eventBus, cfg.Server.SSEHeartbeatInterval)
	queueH := apihandler.NewQueueHandler(riverClient, logger)

	// --- MCP Handler ---
	// Assign interface fields conditionally to avoid the Go nil-interface trap:
	// assigning a nil *T to an interface yields a non-nil interface (has type, nil value),
	// which bypasses nil checks inside the handler and causes a nil pointer dereference.
	mcpCfg := mcphandler.Config{
		ServerName:          cfg.DocuMCP.ServerName,
		ServerVersion:       cfg.DocuMCP.ServerVersion,
		Logger:              logger,
		DocumentService:     documentService,
		DocumentRepo:        documentRepo,
		SearchQueryRepo:     searchQueryRepo,
		ExternalServiceRepo: externalServiceRepo,
		ZimArchiveRepo:      zimArchiveRepo,
		GitTemplateRepo:     gitTemplateRepo,
		ZimEnabled:          true,
		GitTemplatesEnabled: true,

		FederatedSearchTimeout:   cfg.Kiwix.FederatedSearchTimeout,
		FederatedMaxArchives:     cfg.Kiwix.FederatedMaxArchives,
		FederatedPerArchiveLimit: cfg.Kiwix.FederatedPerArchiveLimit,
	}
	mcpCfg.KiwixFactory = kiwixFactory
	if searcher != nil {
		mcpCfg.Searcher = searcher
	}
	mcpH := mcphandler.New(mcpCfg)

	// --- Observability ---
	tracerShutdown, err := observability.InitTracer(context.Background(), cfg.OTEL)
	if err != nil {
		return nil, fmt.Errorf("initializing tracer: %w", err)
	}
	if cfg.OTEL.Enabled {
		// Wrap the slog handler to inject trace_id and span_id into log entries.
		logger = slog.New(observability.NewTracedHandler(logger.Handler()))
		logger.Info("OpenTelemetry tracing enabled", "endpoint", cfg.OTEL.Endpoint)
	}

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
		DashboardHandler:       dashboardH,
		SSEHandler:             sseH,
		QueueHandler:           queueH,
		Metrics:                metrics,
		OTELEnabled:            cfg.OTEL.Enabled,
		IsSecure:               cfg.App.Env == "production",
		SearchClient:           searchClient,
		DB:                     db.DB,
		InternalAPIToken:       cfg.App.InternalAPIToken,
		MaxBodySize:            cfg.Server.MaxBodySize,
		RequestTimeout:         cfg.Server.RequestTimeout,
		HSTSMaxAge:             cfg.Server.HSTSMaxAge,
	})

	logger.Info("MCP server configured",
		"name", cfg.DocuMCP.ServerName,
		"version", cfg.DocuMCP.ServerVersion,
	)

	return &App{
		Config:          cfg,
		DB:              db,
		Logger:          logger,
		Metrics:         metrics,
		Server:          srv,
		PgxPool:         pgxPool,
		RiverClient:     riverClient,
		EventBus:        eventBus,
		docStatusFinder: &docStatusAdapter{repo: documentRepo},
		tracerShutdown:  tracerShutdown,
	}, nil
}

// Start runs the HTTP server and blocks until a shutdown signal is received.
// It handles SIGINT and SIGTERM for graceful shutdown.
func (a *App) Start(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start River queue client for job processing.
	if a.RiverClient != nil {
		if err := a.RiverClient.Start(ctx); err != nil {
			return fmt.Errorf("starting river client: %w", err)
		}
		a.Logger.Info("river queue started")

		// Forward River job completion/snooze events to the EventBus for SSE.
		a.RiverClient.StartEventForwarding()

		// Re-dispatch jobs for documents stuck in intermediate states.
		queue.RecoverStuckDocuments(ctx, a.RiverClient, a.docStatusFinder, a.Logger)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- a.Server.Start()
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		a.Logger.Info("shutdown signal received")
	}

	// Close the EventBus first so all admin SSE connections exit immediately
	// (EventBus.Close() triggers ok=false on all subscriber channels).
	if a.EventBus != nil {
		a.EventBus.Close()
	}

	// Stage 1: graceful shutdown — give in-flight requests time to finish.
	// MCP stateful SSE streams (GET /documcp) will not go idle on their own,
	// so we don't wait long for them here.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.Config.Server.ShutdownTimeout)
	defer cancel()

	if err := a.Server.Shutdown(shutdownCtx); err != nil {
		// Stage 2: force-close any remaining connections (lingering MCP SSE sessions,
		// idle keep-alive connections, etc.).
		a.Logger.Warn("graceful shutdown timed out, forcing connection close")
		if closeErr := a.Server.Close(); closeErr != nil {
			a.Logger.Error("force-closing server", "error", closeErr)
		}
	}

	a.Logger.Info("server stopped")
	return nil
}

// Close releases all resources held by the application.
func (a *App) Close() error {
	if a.RiverClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), a.Config.App.QueueStopTimeout)
		defer cancel()
		if err := a.RiverClient.Stop(ctx); err != nil {
			a.Logger.Error("stopping river client", "error", err)
		}
	}
	if a.tracerShutdown != nil {
		ctx, cancel := context.WithTimeout(context.Background(), a.Config.App.TracerStopTimeout)
		defer cancel()
		if err := a.tracerShutdown(ctx); err != nil {
			a.Logger.Error("flushing tracer spans", "error", err)
		}
	}
	if a.PgxPool != nil {
		a.PgxPool.Close()
	}
	if a.DB != nil {
		if err := a.DB.Close(); err != nil {
			return fmt.Errorf("closing database: %w", err)
		}
	}
	return nil
}

// newLogger creates a structured logger appropriate for the environment.
func newLogger(env string, debug bool, w io.Writer) *slog.Logger {
	var level slog.Level
	if debug {
		level = slog.LevelDebug
	} else {
		level = slog.LevelInfo
	}

	var handler slog.Handler
	if env == "production" || env == "staging" {
		handler = slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})
	} else {
		handler = slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})
	}

	return slog.New(handler)
}

// docStatusAdapter adapts DocumentRepository.FindByStatus to queue.DocumentStatusFinder.
type docStatusAdapter struct {
	repo *repository.DocumentRepository
}

// FindByStatus returns jobs whose associated documents have the given processing status.
func (a *docStatusAdapter) FindByStatus(ctx context.Context, status string) ([]queue.StuckDocument, error) {
	docs, err := a.repo.FindByStatus(ctx, status, 1000)
	if err != nil {
		return nil, err
	}
	result := make([]queue.StuckDocument, len(docs))
	for i := range docs {
		result[i] = queue.StuckDocument{ID: docs[i].ID, UUID: docs[i].UUID}
	}
	return result, nil
}

// deriveKey uses HKDF-SHA256 to derive a subkey from a master secret.
// This ensures different keys for different purposes (e.g. CSRF vs sessions).
func deriveKey(secret []byte, salt, info string, length int) ([]byte, error) {
	r := hkdf.New(sha256.New, secret, []byte(salt), []byte(info))
	key := make([]byte, length)
	if _, err := io.ReadFull(r, key); err != nil {
		return nil, fmt.Errorf("deriving key for %s: %w", info, err)
	}
	return key, nil
}
