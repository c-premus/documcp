// Package app wires together all application dependencies and manages lifecycle.
package app

import (
	"context"
	"crypto/rand"
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

	"github.com/gorilla/sessions"
	"github.com/jmoiron/sqlx"

	"git.999.haus/chris/DocuMCP-go/internal/auth/oauth"
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
	adminhandler "git.999.haus/chris/DocuMCP-go/internal/handler/admin"
	apihandler "git.999.haus/chris/DocuMCP-go/internal/handler/api"
	mcphandler "git.999.haus/chris/DocuMCP-go/internal/handler/mcp"
	oauthhandler "git.999.haus/chris/DocuMCP-go/internal/handler/oauth"
	"git.999.haus/chris/DocuMCP-go/internal/observability"
	"git.999.haus/chris/DocuMCP-go/internal/queue"
	"git.999.haus/chris/DocuMCP-go/internal/repository"
	"git.999.haus/chris/DocuMCP-go/internal/search"
	"git.999.haus/chris/DocuMCP-go/internal/server"
	"git.999.haus/chris/DocuMCP-go/internal/service"
)

// App holds all application dependencies wired together.
type App struct {
	Config        *config.Config
	DB            *sqlx.DB
	Logger        *slog.Logger
	Metrics       *observability.Metrics
	Server        *server.Server
	WorkerPool    *queue.Pool
	tracerShutdown func(context.Context) error
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

	if err := database.RunMigrations(db.DB, "migrations"); err != nil {
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	logger.Info("database migrations applied")

	// --- Session Store ---
	sessionSecret := cfg.OAuth.SessionSecret
	if sessionSecret == "" {
		// Generate a random secret for development
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
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

	// --- Repositories ---
	documentRepo := repository.NewDocumentRepository(db, logger)
	externalServiceRepo := repository.NewExternalServiceRepository(db, logger)
	zimArchiveRepo := repository.NewZimArchiveRepository(db, logger)
	confluenceSpaceRepo := repository.NewConfluenceSpaceRepository(db, logger)
	gitTemplateRepo := repository.NewGitTemplateRepository(db, logger)
	searchQueryRepo := repository.NewSearchQueryRepository(db, logger)
	oauthRepo := repository.NewOAuthRepository(db, logger)

	// --- Meilisearch ---
	var searchClient *search.Client
	var searchIndexer *search.Indexer
	var searcher *search.Searcher

	if cfg.Meilisearch.Host != "" {
		searchClient = search.NewClient(cfg.Meilisearch.Host, cfg.Meilisearch.Key, logger)
		if searchClient.Healthy() {
			if err := searchClient.ConfigureIndexes(context.Background()); err != nil {
				logger.Warn("failed to configure Meilisearch indexes", "error", err)
			} else {
				logger.Info("Meilisearch connected and indexes configured", "host", cfg.Meilisearch.Host)
			}
			searchIndexer = search.NewIndexer(searchClient, logger)
			searcher = search.NewSearcher(searchClient, logger)
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
		kiwixClient = kiwix.NewClient(svc.BaseURL, logger)
		logger.Info("Kiwix client configured", "base_url", svc.BaseURL)
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
		confluenceClient = confluence.NewClient(svc.BaseURL, email, apiToken, logger)
		logger.Info("Confluence client configured", "base_url", svc.BaseURL)
	}

	// --- Content Extractors ---
	extractorRegistry := extractor.NewRegistry(
		pdfext.New(),
		docxext.New(),
		xlsxext.New(),
		htmlext.New(),
		markdownext.New(),
	)

	// --- Worker Pool ---
	workerPool := queue.NewPool(3, 100, logger)

	// --- Storage ---
	storagePath := filepath.Join(cfg.Storage.BasePath, cfg.Storage.DocumentPath)
	if err := os.MkdirAll(storagePath, 0o755); err != nil {
		return nil, fmt.Errorf("creating document storage path: %w", err)
	}

	// --- Services ---
	documentService := service.NewDocumentService(documentRepo, logger)
	documentPipeline := service.NewDocumentPipeline(
		documentService,
		extractorRegistry,
		searchIndexer,
		workerPool,
		storagePath,
	)
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
		searchH = apihandler.NewSearchHandler(searcher, logger)
	}
	zimH := apihandler.NewZimHandler(zimArchiveRepo, kiwixClient, logger)
	confluenceH := apihandler.NewConfluenceHandler(confluenceSpaceRepo, confluenceClient, logger)
	gitTemplateH := apihandler.NewGitTemplateHandler(gitTemplateRepo, logger)
	externalServiceH := apihandler.NewExternalServiceHandler(externalServiceSvc, logger)
	userH := apihandler.NewUserHandler(oauthRepo, logger)
	oauthClientH := apihandler.NewOAuthClientHandler(oauthRepo, logger)

	// --- Admin Handler ---
	adminH := adminhandler.NewHandler(
		documentRepo,
		oauthRepo,
		externalServiceRepo,
		zimArchiveRepo,
		confluenceSpaceRepo,
		gitTemplateRepo,
		documentPipeline,
		externalServiceSvc,
		logger,
	)

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

	metrics := observability.NewMetrics()
	observability.RegisterDBMetrics(db.DB)
	logger.Info("Prometheus metrics registered")

	// --- HTTP Server ---
	srv := server.New(server.Config{
		Host:           cfg.Server.Host,
		Port:           cfg.Server.Port,
		ReadTimeout:    cfg.Server.ReadTimeout,
		WriteTimeout:   cfg.Server.WriteTimeout,
		IdleTimeout:    cfg.Server.IdleTimeout,
		TrustedProxies: cfg.Server.TrustedProxies,
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
		ConfluenceHandler:      confluenceH,
		GitTemplateHandler:     gitTemplateH,
		ExternalServiceHandler: externalServiceH,
		UserHandler:            userH,
		OAuthClientHandler:     oauthClientH,
		AdminHandler:           adminH,
		Metrics:                metrics,
		OTELEnabled:            cfg.OTEL.Enabled,
		CSRFKey:                []byte(sessionSecret)[:32],
		IsSecure:               cfg.App.Env == "production",
		DB:                     db.DB,
	})

	logger.Info("MCP server configured",
		"name", cfg.DocuMCP.ServerName,
		"version", cfg.DocuMCP.ServerVersion,
	)

	return &App{
		Config:         cfg,
		DB:             db,
		Logger:         logger,
		Metrics:        metrics,
		Server:         srv,
		WorkerPool:     workerPool,
		tracerShutdown: tracerShutdown,
	}, nil
}

// Start runs the HTTP server and blocks until a shutdown signal is received.
// It handles SIGINT and SIGTERM for graceful shutdown.
func (a *App) Start(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

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
	if a.WorkerPool != nil {
		a.WorkerPool.Shutdown()
	}
	if a.tracerShutdown != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.tracerShutdown(ctx); err != nil {
			a.Logger.Error("flushing tracer spans", "error", err)
		}
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
