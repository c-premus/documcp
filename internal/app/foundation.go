package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"

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
	Config      *config.Config
	Logger      *slog.Logger
	PgxPool     *pgxpool.Pool
	RedisClient *redis.Client
	Metrics     *observability.Metrics

	// BareRedisClient is an uninstrumented Redis client (no redisotel).
	// Used for rate limiting (httprate-redis TxPipeline) and readiness
	// health checks — both are high-frequency, low-value to trace.
	// MaxRetries -1 prevents retry-induced partial responses.
	BareRedisClient *redis.Client

	// Repositories
	DocumentRepo        *repository.DocumentRepository
	ExternalServiceRepo *repository.ExternalServiceRepository
	ZimArchiveRepo      *repository.ZimArchiveRepository
	GitTemplateRepo     *repository.GitTemplateRepository
	SearchQueryRepo     *repository.SearchQueryRepository
	OAuthRepo           *repository.OAuthRepository

	// Search
	Searcher *search.Searcher

	// External clients
	KiwixFactory      *kiwix.ClientFactory
	ExtractorRegistry *extractor.Registry

	// Encryption
	Encryptor *crypto.Encryptor

	// Storage paths
	StoragePath string
	GitTempDir  string

	tracerShutdown func(context.Context) error
	sentryFlush    func()
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
		"max_conns", cfg.Database.MaxOpenConns,
		"min_conns", cfg.Database.PgxMinConns,
	)

	// --- Redis ---
	// Bridge go-redis internal logger to slog so pool warnings appear
	// in structured logs instead of raw stderr.
	redis.SetLogger(&redisSlogLogger{logger: logger})

	redisOpts := &redis.Options{
		Addr:     cfg.Redis.Addr,
		Username: cfg.Redis.Username,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
		Protocol: 2, // RESP2: go-redis v9.18 defaults to RESP3; pin to avoid push notification overhead

		// DisableIdentity skips CLIENT SETINFO on new connections — avoids
		// unnecessary round-trips and prevents stale buffer data on high-latency
		// Docker bridge networks.
		DisableIdentity: true,

		DialTimeout:  cfg.Redis.DialTimeout,
		ReadTimeout:  cfg.Redis.ReadTimeout,
		WriteTimeout: cfg.Redis.WriteTimeout,

		ContextTimeoutEnabled: true,

		MaxRetries: cfg.Redis.MaxRetries,

		// Pool
		PoolSize:        cfg.Redis.PoolSize,
		MinIdleConns:    cfg.Redis.MinIdleConns,
		MaxActiveConns:  cfg.Redis.MaxActiveConns,
		ConnMaxIdleTime: cfg.Redis.ConnMaxIdleTime,
	}
	redisClient := redis.NewClient(redisOpts)
	if err = redisClient.Ping(context.Background()).Err(); err != nil {
		pgxPool.Close()
		return nil, fmt.Errorf("connecting to redis: %w", err)
	}

	// Instrument Redis with OpenTelemetry tracing.
	if err = redisotel.InstrumentTracing(redisClient); err != nil {
		_ = redisClient.Close()
		pgxPool.Close()
		return nil, fmt.Errorf("instrumenting redis tracing: %w", err)
	}

	// Dedicated rate-limit client — isolates httprate-redis TxPipeline
	// (MULTI/INCR/EXPIRE/EXEC) from the main pool. MaxRetries -1 avoids
	// retry-induced partial responses. DisableIdentity skips CLIENT SETINFO.
	// No redisotel hook — counter increments are high-frequency, low-value to trace.
	// NOTE: Redis ACL must include +@transaction for MULTI/EXEC to succeed.
	bareRedisClient := redis.NewClient(&redis.Options{
		Addr:            cfg.Redis.Addr,
		Username:        cfg.Redis.Username,
		Password:        cfg.Redis.Password,
		DB:              cfg.Redis.DB,
		Protocol:        2, // RESP2: match main client; avoid RESP3 push notifications
		DisableIdentity: true,
		PoolSize:        3,
		MinIdleConns:    1,
		MaxRetries:      -1,
		ReadTimeout:     500 * time.Millisecond,
		WriteTimeout:    500 * time.Millisecond,
		ContextTimeoutEnabled: true,
	})
	if err = bareRedisClient.Ping(context.Background()).Err(); err != nil {
		_ = redisClient.Close()
		pgxPool.Close()
		return nil, fmt.Errorf("connecting to redis (rate limit): %w", err)
	}

	logger.Info("redis connected",
		"addr", cfg.Redis.Addr,
		"pool_size", redisOpts.PoolSize,
		"max_retries", redisOpts.MaxRetries,
		"read_timeout", redisOpts.ReadTimeout,
		"write_timeout", redisOpts.WriteTimeout,
		"context_timeout_enabled", true,
		"rate_limit_pool_size", 3,
	)

	// After this point pgxPool, redisClient, and bareRedisClient are live.
	// Use a deferred cleanup to close all on any initialization error.
	var initOK bool
	defer func() {
		if !initOK {
			_ = bareRedisClient.Close()
			_ = redisClient.Close()
			pgxPool.Close()
		}
	}()

	// Run database and River schema migrations.
	if migErr := runMigrations(pgxPool); migErr != nil {
		return nil, migErr
	}
	logger.Info("database and river migrations applied")

	// --- Encryption ---
	var encryptor *crypto.Encryptor
	if len(cfg.App.EncryptionKeyBytes) > 0 {
		var encErr error
		encryptor, encErr = crypto.NewEncryptor(cfg.App.EncryptionKeyBytes)
		if encErr != nil {
			return nil, fmt.Errorf("initializing encryptor: %w", encErr)
		}
		logger.Info("encryption at rest enabled")
	} else {
		logger.Warn("ENCRYPTION_KEY not set, secrets will be stored in plaintext")
	}

	// --- Repositories ---
	documentRepo := repository.NewDocumentRepository(pgxPool, logger)
	externalServiceRepo := repository.NewExternalServiceRepository(pgxPool, logger, encryptor)
	zimArchiveRepo := repository.NewZimArchiveRepository(pgxPool, logger)
	gitTemplateRepo := repository.NewGitTemplateRepository(pgxPool, logger, encryptor)
	searchQueryRepo := repository.NewSearchQueryRepository(pgxPool, logger)
	oauthRepo := repository.NewOAuthRepository(pgxPool, logger)

	// --- Search ---
	searcher := search.NewSearcher(pgxPool, logger)

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
		return nil, fmt.Errorf("creating document storage path: %w", err)
	}

	gitTempDir := filepath.Join(cfg.Storage.BasePath, "git")
	if err = os.MkdirAll(gitTempDir, 0o750); err != nil {
		return nil, fmt.Errorf("creating git temp path: %w", err)
	}

	// --- Observability ---
	metrics := observability.NewMetrics()
	observability.RegisterDBMetrics(pgxPool)
	observability.RegisterRedisMetrics(redisClient)
	observability.RegisterDocumentCount(pgxPool)
	searcher.SetMetrics(metrics)
	logger.Info("Prometheus metrics registered")

	tracerShutdown, err := observability.InitTracer(context.Background(), cfg.OTEL)
	if err != nil {
		return nil, fmt.Errorf("initializing tracer: %w", err)
	}
	if cfg.OTEL.Enabled {
		logger = slog.New(observability.NewTracedHandler(logger.Handler()))
		logger.Info("OpenTelemetry tracing enabled", "endpoint", cfg.OTEL.Endpoint)
	}

	sentryFlush, err := observability.InitSentry(cfg.Sentry, cfg.App.Env, cfg.DocuMCP.ServerVersion)
	if err != nil {
		return nil, fmt.Errorf("initializing sentry: %w", err)
	}
	if cfg.Sentry.DSN != "" {
		logger.Info("Sentry error tracking enabled")
	}

	initOK = true
	return &Foundation{
		Config:              cfg,
		Logger:              logger,
		PgxPool:             pgxPool,
		RedisClient:          redisClient,
		BareRedisClient: bareRedisClient,
		Metrics:              metrics,
		DocumentRepo:        documentRepo,
		ExternalServiceRepo: externalServiceRepo,
		ZimArchiveRepo:      zimArchiveRepo,
		GitTemplateRepo:     gitTemplateRepo,
		SearchQueryRepo:     searchQueryRepo,
		OAuthRepo:           oauthRepo,
		Searcher:            searcher,
		KiwixFactory:        kiwixFactory,
		ExtractorRegistry:   extractorRegistry,
		Encryptor:           encryptor,
		StoragePath:         storagePath,
		GitTempDir:          gitTempDir,
		tracerShutdown:      tracerShutdown,
		sentryFlush:         sentryFlush,
	}, nil
}

// Close releases all resources held by the Foundation.
func (f *Foundation) Close() {
	if f.sentryFlush != nil {
		f.sentryFlush()
	}
	if f.tracerShutdown != nil {
		ctx, cancel := context.WithTimeout(context.Background(), f.Config.App.TracerStopTimeout)
		defer cancel()
		if err := f.tracerShutdown(ctx); err != nil {
			f.Logger.Error("flushing tracer spans", "error", err)
		}
	}
	if f.BareRedisClient != nil {
		if err := f.BareRedisClient.Close(); err != nil {
			f.Logger.Error("closing rate limit redis client", "error", err)
		}
	}
	if f.RedisClient != nil {
		if err := f.RedisClient.Close(); err != nil {
			f.Logger.Error("closing redis client", "error", err)
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
