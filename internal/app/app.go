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
	"time"

	"golang.org/x/crypto/hkdf"

	"github.com/gorilla/sessions"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jmoiron/sqlx"
	"github.com/riverqueue/river"

	"git.999.haus/chris/DocuMCP-go/internal/auth/oauth"
	"git.999.haus/chris/DocuMCP-go/internal/crypto"
	"git.999.haus/chris/DocuMCP-go/internal/auth/oidc"
	"git.999.haus/chris/DocuMCP-go/internal/client/confluence"
	"git.999.haus/chris/DocuMCP-go/internal/client/kiwix"
	"git.999.haus/chris/DocuMCP-go/internal/config"
	"git.999.haus/chris/DocuMCP-go/internal/database"
	"git.999.haus/chris/DocuMCP-go/internal/extractor"
	docxext "git.999.haus/chris/DocuMCP-go/internal/extractor/docx"
	htmlext "git.999.haus/chris/DocuMCP-go/internal/extractor/html"
	markdownext "git.999.haus/chris/DocuMCP-go/internal/extractor/markdown"
	pdfext "git.999.haus/chris/DocuMCP-go/internal/extractor/pdf"
	xlsxext "git.999.haus/chris/DocuMCP-go/internal/extractor/xlsx"
	apihandler "git.999.haus/chris/DocuMCP-go/internal/handler/api"
	mcphandler "git.999.haus/chris/DocuMCP-go/internal/handler/mcp"
	oauthhandler "git.999.haus/chris/DocuMCP-go/internal/handler/oauth"
	frontend "git.999.haus/chris/DocuMCP-go/web/frontend"

	"git.999.haus/chris/DocuMCP-go/internal/observability"
	"git.999.haus/chris/DocuMCP-go/internal/queue"
	"git.999.haus/chris/DocuMCP-go/internal/repository"
	"git.999.haus/chris/DocuMCP-go/internal/search"
	"git.999.haus/chris/DocuMCP-go/internal/server"
	"git.999.haus/chris/DocuMCP-go/internal/service"
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
	pgxPool, err := database.NewPgxPool(context.Background(), cfg.DatabaseDSN(), 10)
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
	sessionStore := sessions.NewCookieStore([]byte(sessionSecret))
	sessionStore.Options = &sessions.Options{
		Path:     "/",
		HttpOnly: true,
		Secure:   cfg.App.Env == "production",
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400 * 30, // 30 days
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
	hmacKey, err := deriveKey([]byte(sessionSecret), "oauth-token-hmac", 32)
	if err != nil {
		return nil, fmt.Errorf("deriving HMAC key: %w", err)
	}
	oauth.SetTokenHMACKey(hmacKey)

	// --- Repositories ---
	documentRepo := repository.NewDocumentRepository(db, logger)
	externalServiceRepo := repository.NewExternalServiceRepository(db, logger)
	zimArchiveRepo := repository.NewZimArchiveRepository(db, logger)
	confluenceSpaceRepo := repository.NewConfluenceSpaceRepository(db, logger)
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
	// Look up configured external services from the database and create clients.
	var kiwixClient *kiwix.Client
	var confluenceClient *confluence.Client

	kiwixServices, err := externalServiceRepo.FindEnabledByType(context.Background(), "kiwix")
	if err != nil {
		logger.Warn("failed to look up kiwix services", "error", err)
	} else if len(kiwixServices) > 0 {
		svc := kiwixServices[0]
		var kiwixErr error
		kiwixClient, kiwixErr = kiwix.NewClient(svc.BaseURL, logger)
		if kiwixErr != nil {
			logger.Warn("kiwix client URL rejected", "base_url", svc.BaseURL, "error", kiwixErr)
		} else {
			logger.Info("Kiwix client configured", "base_url", svc.BaseURL)
		}
	}

	confluenceServices, err := externalServiceRepo.FindEnabledByType(context.Background(), "confluence")
	if err != nil {
		logger.Warn("failed to look up confluence services", "error", err)
	} else if len(confluenceServices) > 0 {
		svc := confluenceServices[0]
		// API key is stored in the api_key column (format: "email:token" or just token)
		email := ""
		apiToken := svc.APIKey.String
		if parts := strings.SplitN(apiToken, ":", 2); len(parts) == 2 {
			email = parts[0]
			apiToken = parts[1]
		}
		var confErr error
		confluenceClient, confErr = confluence.NewClient(svc.BaseURL, email, apiToken, logger)
		if confErr != nil {
			logger.Warn("confluence client URL rejected", "base_url", svc.BaseURL, "error", confErr)
		} else {
			logger.Info("Confluence client configured", "base_url", svc.BaseURL)
		}
	}

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
	if err = os.MkdirAll(storagePath, 0o755); err != nil {
		return nil, fmt.Errorf("creating document storage path: %w", err)
	}

	gitTempDir := filepath.Join(cfg.Storage.BasePath, "git")
	if err = os.MkdirAll(gitTempDir, 0o755); err != nil {
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
	eventBus := queue.NewEventBus()

	// --- River Workers ---
	schedulerDeps := queue.SchedulerDeps{
		Services:      externalServiceRepo,
		HealthChecker: externalServiceRepo,
		ZimRepo:       zimArchiveRepo,
		ConfRepo:      confluenceSpaceRepo,
		GitRepo:       gitTemplateRepo,
		OAuthRepo:     oauthRepo,
		DocRepo:       documentRepo,
		Indexer:       searchIndexer,
		GitTempDir:    gitTempDir,
		StoragePath:   storagePath,
		Logger:        logger,
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
	river.AddWorker(workers, &queue.SyncConfluenceWorker{Deps: schedulerDeps})
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
			"confluence_schedule", cfg.Scheduler.ConfluenceSchedule,
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
	externalServiceSvc := service.NewExternalServiceService(externalServiceRepo, logger)

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
	documentH := apihandler.NewDocumentHandler(documentPipeline, documentRepo, logger)
	var searchH *apihandler.SearchHandler
	if searcher != nil {
		searchH = apihandler.NewSearchHandler(searcher, searchQueryRepo, documentRepo, logger)
	}
	zimH := apihandler.NewZimHandler(zimArchiveRepo, kiwixClient, logger)
	confluenceH := apihandler.NewConfluenceHandler(confluenceSpaceRepo, confluenceClient, logger)
	gitTemplateH := apihandler.NewGitTemplateHandler(gitTemplateRepo, logger)
	externalServiceH := apihandler.NewExternalServiceHandler(externalServiceSvc, externalServiceRepo, logger)
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
		confluenceSpaceRepo,
		gitTemplateRepo,
		riverClient,
		logger,
	)

	// --- SSE & Queue Handlers ---
	sseH := apihandler.NewSSEHandler(eventBus)
	queueH := apihandler.NewQueueHandler(riverClient, logger)

	// --- MCP Handler ---
	mcpH := mcphandler.New(mcphandler.Config{
		ServerName:          cfg.DocuMCP.ServerName,
		ServerVersion:       cfg.DocuMCP.ServerVersion,
		Logger:              logger,
		DocumentService:     documentService,
		DocumentRepo:        documentRepo,
		SearchQueryRepo:     searchQueryRepo,
		ExternalServiceRepo: externalServiceRepo,
		ZimArchiveRepo:      zimArchiveRepo,
		ConfluenceSpaceRepo: confluenceSpaceRepo,
		GitTemplateRepo:     gitTemplateRepo,
		KiwixClient:         kiwixClient,
		ConfluenceClient:    confluenceClient,
		Searcher:            searcher,
		ZimEnabled:          true,
		ConfluenceEnabled:   true,
		GitTemplatesEnabled: true,
	})

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

	csrfKey, err := deriveKey([]byte(sessionSecret), "csrf-token-key", 32)
	if err != nil {
		return nil, fmt.Errorf("deriving CSRF key: %w", err)
	}

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
		ConfluenceHandler:      confluenceH,
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
		CSRFKey:                csrfKey,
		IsSecure:               cfg.App.Env == "production",
		AppURL:                 cfg.App.URL,
		SearchClient:           searchClient,
		DB:                     db.DB,
		InternalAPIToken:       cfg.App.InternalAPIToken,
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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := a.Server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	a.Logger.Info("server stopped gracefully")
	return nil
}

// Close releases all resources held by the application.
func (a *App) Close() error {
	if a.RiverClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := a.RiverClient.Stop(ctx); err != nil {
			a.Logger.Error("stopping river client", "error", err)
		}
	}
	if a.tracerShutdown != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
func deriveKey(secret []byte, info string, length int) ([]byte, error) {
	r := hkdf.New(sha256.New, secret, nil, []byte(info))
	key := make([]byte, length)
	if _, err := io.ReadFull(r, key); err != nil {
		return nil, fmt.Errorf("deriving key for %s: %w", info, err)
	}
	return key, nil
}
