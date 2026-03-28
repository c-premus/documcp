package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgxpool"

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
	"github.com/c-premus/documcp/internal/observability"
	"github.com/c-premus/documcp/internal/repository"
	"github.com/c-premus/documcp/internal/search"
)

// Foundation holds shared dependencies used by both ServerApp and WorkerApp.
type Foundation struct {
	Config  *config.Config
	Logger  *slog.Logger
	PgxPool *pgxpool.Pool
	Metrics *observability.Metrics

	// Repositories
	DocumentRepo        *repository.DocumentRepository
	ExternalServiceRepo *repository.ExternalServiceRepository
	ZimArchiveRepo      *repository.ZimArchiveRepository
	GitTemplateRepo     *repository.GitTemplateRepository
	SearchQueryRepo     *repository.SearchQueryRepository
	OAuthRepo           *repository.OAuthRepository

	// Search (nil when Meilisearch disabled)
	SearchClient  *search.Client
	SearchIndexer *search.Indexer
	Searcher      *search.Searcher

	// External clients
	KiwixFactory      *kiwix.ClientFactory
	ExtractorRegistry *extractor.Registry

	// Encryption
	Encryptor *crypto.Encryptor

	// Storage paths
	StoragePath string
	GitTempDir  string

	tracerShutdown func(context.Context) error
}

// NewFoundation initializes all shared dependencies: database, repositories,
// search, extractors, storage, observability, and tracing.
func NewFoundation(cfg *config.Config) (*Foundation, error) {
	logger := newLogger(cfg.App.Env, cfg.App.Debug, os.Stdout)

	pgxPool, err := database.NewPgxPool(
		context.Background(),
		cfg.DatabaseDSN(),
		int32(min(cfg.Database.MaxOpenConns, 1<<31-1)), //nolint:gosec // config value is bounded by validation
		cfg.Database.PgxMinConns,
		cfg.Database.PgxMaxConnLifetime,
		cfg.Database.PgxMaxConnIdleTime,
	)
	if err != nil {
		return nil, fmt.Errorf("initializing database pool: %w", err)
	}

	logger.Info("database connected",
		"host", cfg.Database.Host,
		"database", cfg.Database.Database,
	)

	// Run database and River schema migrations.
	if err = runMigrations(pgxPool); err != nil {
		pgxPool.Close()
		return nil, err
	}
	logger.Info("database and river migrations applied")

	// --- Encryption ---
	var encryptor *crypto.Encryptor
	if cfg.App.EncryptionKey != "" {
		var encErr error
		encryptor, encErr = crypto.NewEncryptor([]byte(cfg.App.EncryptionKey))
		if encErr != nil {
			pgxPool.Close()
			return nil, fmt.Errorf("initializing encryptor: %w", encErr)
		}
		logger.Info("encryption at rest enabled")
	} else {
		logger.Warn("ENCRYPTION_KEY not set, git tokens will be stored in plaintext")
	}

	// --- Repositories ---
	documentRepo := repository.NewDocumentRepository(pgxPool, logger)
	externalServiceRepo := repository.NewExternalServiceRepository(pgxPool, logger)
	zimArchiveRepo := repository.NewZimArchiveRepository(pgxPool, logger)
	gitTemplateRepo := repository.NewGitTemplateRepository(pgxPool, logger, encryptor)
	searchQueryRepo := repository.NewSearchQueryRepository(pgxPool, logger)
	oauthRepo := repository.NewOAuthRepository(pgxPool, logger)

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
		} else {
			logger.Warn("Meilisearch not reachable, search features disabled", "host", cfg.Meilisearch.Host)
		}
	}

	// --- External Service Clients ---
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
		pgxPool.Close()
		return nil, fmt.Errorf("creating document storage path: %w", err)
	}

	gitTempDir := filepath.Join(cfg.Storage.BasePath, "git")
	if err = os.MkdirAll(gitTempDir, 0o750); err != nil {
		pgxPool.Close()
		return nil, fmt.Errorf("creating git temp path: %w", err)
	}

	// --- Observability ---
	metrics := observability.NewMetrics()
	observability.RegisterDBMetrics(pgxPool)
	if searcher != nil {
		searcher.SetMetrics(metrics)
	}
	logger.Info("Prometheus metrics registered")

	tracerShutdown, err := observability.InitTracer(context.Background(), cfg.OTEL)
	if err != nil {
		pgxPool.Close()
		return nil, fmt.Errorf("initializing tracer: %w", err)
	}
	if cfg.OTEL.Enabled {
		logger = slog.New(observability.NewTracedHandler(logger.Handler()))
		logger.Info("OpenTelemetry tracing enabled", "endpoint", cfg.OTEL.Endpoint)
	}

	return &Foundation{
		Config:              cfg,
		Logger:              logger,
		PgxPool:             pgxPool,
		Metrics:             metrics,
		DocumentRepo:        documentRepo,
		ExternalServiceRepo: externalServiceRepo,
		ZimArchiveRepo:      zimArchiveRepo,
		GitTemplateRepo:     gitTemplateRepo,
		SearchQueryRepo:     searchQueryRepo,
		OAuthRepo:           oauthRepo,
		SearchClient:        searchClient,
		SearchIndexer:       searchIndexer,
		Searcher:            searcher,
		KiwixFactory:        kiwixFactory,
		ExtractorRegistry:   extractorRegistry,
		Encryptor:           encryptor,
		StoragePath:         storagePath,
		GitTempDir:          gitTempDir,
		tracerShutdown:      tracerShutdown,
	}, nil
}

// Close releases all resources held by the Foundation.
func (f *Foundation) Close() {
	if f.tracerShutdown != nil {
		ctx, cancel := context.WithTimeout(context.Background(), f.Config.App.TracerStopTimeout)
		defer cancel()
		if err := f.tracerShutdown(ctx); err != nil {
			f.Logger.Error("flushing tracer spans", "error", err)
		}
	}
	if f.PgxPool != nil {
		f.PgxPool.Close()
	}
}

// runMigrations runs goose and River schema migrations.
func runMigrations(pool *pgxpool.Pool) error {
	sqlDB := database.SQLDBFromPool(pool)
	if err := database.RunMigrations(sqlDB, "migrations"); err != nil {
		_ = sqlDB.Close()
		return fmt.Errorf("running migrations: %w", err)
	}
	_ = sqlDB.Close()

	if err := database.RunRiverMigrations(context.Background(), pool); err != nil {
		return fmt.Errorf("running river migrations: %w", err)
	}

	return nil
}

// RunMigrationsOnly initializes just the database pool and runs migrations,
// then closes everything. Used by the migrate subcommand.
func RunMigrationsOnly(cfg *config.Config) error {
	logger := newLogger(cfg.App.Env, cfg.App.Debug, os.Stdout)

	pgxPool, err := database.NewPgxPool(
		context.Background(),
		cfg.DatabaseDSN(),
		int32(min(cfg.Database.MaxOpenConns, 1<<31-1)), //nolint:gosec // config value is bounded by validation
		cfg.Database.PgxMinConns,
		cfg.Database.PgxMaxConnLifetime,
		cfg.Database.PgxMaxConnIdleTime,
	)
	if err != nil {
		return fmt.Errorf("initializing database pool: %w", err)
	}
	defer pgxPool.Close()

	logger.Info("database connected",
		"host", cfg.Database.Host,
		"database", cfg.Database.Database,
	)

	if err := runMigrations(pgxPool); err != nil {
		return err
	}

	logger.Info("all migrations applied successfully")
	return nil
}
