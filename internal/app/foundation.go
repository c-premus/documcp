// Package app implements the Foundation service locator and the HTTP/worker
// app lifecycles that consume it. Foundation deliberately leans on the
// service-locator pattern: the application needs ~20 interdependent
// singletons (pgxpool, Redis clients, repositories, extractors, tracing,
// Sentry) that all must exist together. Each field is constructed by a
// phase factory (newDatabasePool, newRedisClients, newRepositoryBundle,
// newStoragePaths, newObservability) so the init code reads as a sequence
// of narrow steps instead of one 275-line prelude.
package app

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
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
	epubext "github.com/c-premus/documcp/internal/extractor/epub"
	htmlext "github.com/c-premus/documcp/internal/extractor/html"
	markdownext "github.com/c-premus/documcp/internal/extractor/markdown"
	pdfext "github.com/c-premus/documcp/internal/extractor/pdf"
	xlsxext "github.com/c-premus/documcp/internal/extractor/xlsx"
	"github.com/c-premus/documcp/internal/observability"
	"github.com/c-premus/documcp/internal/queue"
	"github.com/c-premus/documcp/internal/repository"
	"github.com/c-premus/documcp/internal/search"
	"github.com/c-premus/documcp/internal/storage"
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

	// ControlBus is a cross-replica pub/sub channel for control messages
	// (cache invalidation, etc.). Uses its own Redis channels, independent
	// of the job-event bus so a flooded SSE subscriber pool can't crowd
	// out invalidation broadcasts.
	ControlBus queue.ControlBus

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

	// Storage
	//
	// BlobStore is the document blob backend — filesystem or S3-compatible.
	// Uploads, reads, deletes, and orphan cleanup all go through this
	// interface. Callers treat file paths stored in the database as opaque
	// keys into the store.
	BlobStore storage.Blob

	// StoragePath is the filesystem root for the local document store. It
	// remains populated when Driver=local for callers that have not yet been
	// migrated to the BlobStore interface. New code should use BlobStore.
	StoragePath string

	// WorkerTempDir is a node-local scratch directory used by workers to
	// stage blobs before handing them to extractors that require a seekable
	// file path (PDF, DOCX, XLSX). Lives under BasePath so operators can
	// size it with a volume or tmpfs.
	WorkerTempDir string

	// GitTempDir is the base directory for git template clones. Ephemeral
	// per-replica; contents are regenerated on each sync.
	GitTempDir string

	tracerShutdown func(context.Context) error
	sentryFlush    func()
}

// NewFoundation constructs every shared singleton by calling narrow phase
// factories in sequence. On any error before initOK flips true, the deferred
// cleanup chain closes each resource already opened in reverse order.
func NewFoundation(cfg *config.Config) (*Foundation, error) {
	logger := newLogger(cfg.App.Env, cfg.App.Debug, os.Stdout)

	pgxPool, err := newDatabasePool(cfg, logger)
	if err != nil {
		return nil, err
	}

	var initOK bool
	defer func() {
		if !initOK {
			pgxPool.Close()
		}
	}()

	redisClient, bareRedisClient, err := newRedisClients(cfg, logger)
	if err != nil {
		return nil, err
	}
	defer func() {
		if !initOK {
			_ = bareRedisClient.Close()
			_ = redisClient.Close()
		}
	}()

	if migErr := runMigrations(pgxPool); migErr != nil {
		return nil, migErr
	}
	logger.Info("database and river migrations applied")

	encryptor, err := newEncryptor(cfg, logger)
	if err != nil {
		return nil, err
	}

	repos := newRepositoryBundle(pgxPool, logger, encryptor)
	searcher := search.NewSearcher(pgxPool, logger)

	kiwixFactory := kiwix.NewClientFactory(repos.ExternalService, kiwix.ClientConfig{
		HTTPTimeout:        cfg.Kiwix.HTTPTimeout,
		HealthCheckTimeout: cfg.Kiwix.HealthCheckTimeout,
		CacheTTL:           cfg.Kiwix.CacheTTL,
		SSRFDialerTimeout:  cfg.App.SSRFDialerTimeout,
	}, logger)

	extractorRegistry := newExtractorRegistry(cfg)

	storagePath, workerTempDir, gitTempDir, err := newStoragePaths(cfg)
	if err != nil {
		return nil, err
	}

	blobStore, err := openBlobStore(context.Background(), cfg.Storage, storagePath)
	if err != nil {
		return nil, fmt.Errorf("opening blob store: %w", err)
	}
	defer func() {
		if !initOK {
			_ = blobStore.Close()
		}
	}()
	logger.Info("blob store opened", "driver", cfg.Storage.Driver)

	// Control bus for cross-replica control messages (cache invalidation).
	// Lives in its own Redis channels so it doesn't contend with the SSE
	// subscriber cap on the main event bus.
	controlBus := queue.NewRedisControlBus(redisClient, logger)
	defer func() {
		if !initOK {
			_ = controlBus.Close()
		}
	}()

	obs, err := newObservability(context.Background(), cfg, pgxPool, redisClient, searcher, logger)
	if err != nil {
		return nil, err
	}
	// Observability may wrap the logger with the OTEL trace-correlating
	// handler, so replace from here onward.
	logger = obs.Logger

	initOK = true
	return &Foundation{
		Config:              cfg,
		Logger:              logger,
		PgxPool:             pgxPool,
		RedisClient:         redisClient,
		BareRedisClient:     bareRedisClient,
		Metrics:             obs.Metrics,
		DocumentRepo:        repos.Document,
		ExternalServiceRepo: repos.ExternalService,
		ZimArchiveRepo:      repos.ZimArchive,
		GitTemplateRepo:     repos.GitTemplate,
		SearchQueryRepo:     repos.SearchQuery,
		OAuthRepo:           repos.OAuth,
		Searcher:            searcher,
		KiwixFactory:        kiwixFactory,
		ExtractorRegistry:   extractorRegistry,
		Encryptor:           encryptor,
		BlobStore:           blobStore,
		StoragePath:         storagePath,
		WorkerTempDir:       workerTempDir,
		GitTempDir:          gitTempDir,
		ControlBus:          controlBus,
		tracerShutdown:      obs.TracerShutdown,
		sentryFlush:         obs.SentryFlush,
	}, nil
}

// newDatabasePool initializes the pgxpool used by repositories, River, and
// goose migrations.
func newDatabasePool(cfg *config.Config, logger *slog.Logger) (*pgxpool.Pool, error) {
	pool, err := database.NewPgxPool(
		context.Background(),
		cfg.DatabaseDSN(),
		cfg.Database.MaxOpenConns,
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
	return pool, nil
}

// newRedisClients creates the instrumented main client (EventBus, app
// queries) and the uninstrumented bare client (rate limit + readiness).
// Both share the same TLS config when REDIS_TLS_ENABLED=true. The bare
// client uses MaxRetries=-1 and short timeouts so readiness checks fail
// fast and rate-limit TxPipelines can't cause partial-response noise.
func newRedisClients(cfg *config.Config, logger *slog.Logger) (mainClient, bareClient *redis.Client, err error) {
	// Bridge go-redis internal logger to slog so pool warnings appear
	// in structured logs instead of raw stderr.
	redis.SetLogger(&redisSlogLogger{logger: logger})

	redisTLS, err := buildRedisTLS(cfg, logger)
	if err != nil {
		return nil, nil, err
	}

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

		TLSConfig: redisTLS,
	}
	mainClient = redis.NewClient(redisOpts)
	if err = mainClient.Ping(context.Background()).Err(); err != nil {
		return nil, nil, fmt.Errorf("connecting to redis: %w", err)
	}
	if err = redisotel.InstrumentTracing(mainClient); err != nil {
		_ = mainClient.Close()
		return nil, nil, fmt.Errorf("instrumenting redis tracing: %w", err)
	}

	// Dedicated rate-limit client — isolates httprate-redis TxPipeline
	// (MULTI/INCR/EXPIRE/EXEC) from the main pool. MaxRetries -1 avoids
	// retry-induced partial responses. DisableIdentity skips CLIENT SETINFO.
	// No redisotel hook — counter increments are high-frequency, low-value to trace.
	// NOTE: Redis ACL must include +@transaction for MULTI/EXEC to succeed.
	bareClient = redis.NewClient(&redis.Options{
		Addr:                  cfg.Redis.Addr,
		Username:              cfg.Redis.Username,
		Password:              cfg.Redis.Password,
		DB:                    cfg.Redis.DB,
		Protocol:              2, // RESP2: match main client; avoid RESP3 push notifications
		DisableIdentity:       true,
		PoolSize:              3,
		MinIdleConns:          1,
		MaxRetries:            -1,
		ReadTimeout:           500 * time.Millisecond,
		WriteTimeout:          500 * time.Millisecond,
		ContextTimeoutEnabled: true,
		TLSConfig:             redisTLS,
	})
	if err = bareClient.Ping(context.Background()).Err(); err != nil {
		_ = mainClient.Close()
		return nil, nil, fmt.Errorf("connecting to redis (rate limit): %w", err)
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
	return mainClient, bareClient, nil
}

// buildRedisTLS returns the tls.Config for Redis connections. Returns nil
// when TLS is disabled. When a CA file is set, it is loaded into a private
// cert pool; otherwise the system pool is used.
func buildRedisTLS(cfg *config.Config, logger *slog.Logger) (*tls.Config, error) {
	if !cfg.Redis.TLSEnabled {
		return nil, nil
	}
	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if cfg.Redis.TLSCAFile != "" {
		caCert, err := os.ReadFile(cfg.Redis.TLSCAFile)
		if err != nil {
			return nil, fmt.Errorf("reading redis TLS CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caCert) {
			return nil, errors.New("redis TLS CA file contains no valid certificates")
		}
		tlsCfg.RootCAs = pool
	}
	logger.Info("redis TLS enabled", "ca_file", cfg.Redis.TLSCAFile)
	return tlsCfg, nil
}

// newEncryptor returns an AES-256-GCM encryptor when EncryptionKeyBytes is
// set; otherwise nil with a warning log. Repositories that encrypt fields
// (git_token, service credentials) must check for nil before use.
func newEncryptor(cfg *config.Config, logger *slog.Logger) (*crypto.Encryptor, error) {
	if len(cfg.App.EncryptionKeyBytes) == 0 {
		logger.Warn("ENCRYPTION_KEY not set, secrets will be stored in plaintext")
		return nil, nil
	}
	enc, err := crypto.NewEncryptor(cfg.App.EncryptionKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("initializing encryptor: %w", err)
	}
	logger.Info("encryption at rest enabled")
	return enc, nil
}

// repositoryBundle groups the six repositories constructed from a shared
// pool. Kept unexported — external callers read individual fields off
// Foundation.
type repositoryBundle struct {
	Document        *repository.DocumentRepository
	ExternalService *repository.ExternalServiceRepository
	ZimArchive      *repository.ZimArchiveRepository
	GitTemplate     *repository.GitTemplateRepository
	SearchQuery     *repository.SearchQueryRepository
	OAuth           *repository.OAuthRepository
}

func newRepositoryBundle(pool *pgxpool.Pool, logger *slog.Logger, encryptor *crypto.Encryptor) repositoryBundle {
	return repositoryBundle{
		Document:        repository.NewDocumentRepository(pool, logger),
		ExternalService: repository.NewExternalServiceRepository(pool, logger, encryptor),
		ZimArchive:      repository.NewZimArchiveRepository(pool, logger),
		GitTemplate:     repository.NewGitTemplateRepository(pool, logger, encryptor),
		SearchQuery:     repository.NewSearchQueryRepository(pool, logger),
		OAuth:           repository.NewOAuthRepository(pool, logger),
	}
}

// newExtractorRegistry wires every content extractor with the operator-
// configured limits. First MIME match wins at registry lookup time.
func newExtractorRegistry(cfg *config.Config) *extractor.Registry {
	return extractor.NewRegistry(
		pdfext.NewWithLimits(cfg.Storage.MaxExtractedText),
		docxext.NewWithLimits(cfg.Storage.MaxZIPFiles, cfg.Storage.MaxExtractedText),
		xlsxext.NewWithLimits(cfg.Storage.MaxSheets, cfg.Storage.MaxExtractedText),
		epubext.NewWithLimits(cfg.Storage.MaxZIPFiles, cfg.Storage.MaxExtractedText),
		htmlext.New(),
		markdownext.New(),
	)
}

// newStoragePaths creates the three scratch directories under BasePath and
// returns their absolute locations. Worker-tmp and git-tmp live alongside
// the blob root so operators can size the whole scratch area with a single
// volume or tmpfs.
func newStoragePaths(cfg *config.Config) (storagePath, workerTempDir, gitTempDir string, err error) {
	storagePath = filepath.Join(cfg.Storage.BasePath, cfg.Storage.DocumentPath)
	if err = os.MkdirAll(storagePath, 0o750); err != nil {
		return "", "", "", fmt.Errorf("creating document storage path: %w", err)
	}
	workerTempDir = filepath.Join(cfg.Storage.BasePath, "worker-tmp")
	if err = os.MkdirAll(workerTempDir, 0o750); err != nil {
		return "", "", "", fmt.Errorf("creating worker temp path: %w", err)
	}
	gitTempDir = filepath.Join(cfg.Storage.BasePath, "git")
	if err = os.MkdirAll(gitTempDir, 0o750); err != nil {
		return "", "", "", fmt.Errorf("creating git temp path: %w", err)
	}
	return storagePath, workerTempDir, gitTempDir, nil
}

// observabilityBundle groups the values newObservability returns. Logger
// may be a wrapped version of the input logger when OTEL is enabled.
type observabilityBundle struct {
	Metrics        *observability.Metrics
	TracerShutdown func(context.Context) error
	SentryFlush    func()
	Logger         *slog.Logger
}

// newObservability registers Prometheus metrics, initializes the OTEL
// tracer, and opens the Sentry client. Wraps the logger with a trace-
// correlating handler when OTEL is enabled so span IDs reach log lines.
func newObservability(
	ctx context.Context,
	cfg *config.Config,
	pool *pgxpool.Pool,
	redisClient *redis.Client,
	searcher *search.Searcher,
	logger *slog.Logger,
) (observabilityBundle, error) {
	metrics := observability.NewMetrics()
	observability.RegisterDBMetrics(pool)
	observability.RegisterRedisMetrics(redisClient)
	observability.RegisterDocumentCount(pool)
	searcher.SetMetrics(metrics)
	logger.Info("Prometheus metrics registered")

	tracerShutdown, err := observability.InitTracer(ctx, cfg.OTEL)
	if err != nil {
		return observabilityBundle{}, fmt.Errorf("initializing tracer: %w", err)
	}
	if cfg.OTEL.Enabled {
		logger = slog.New(observability.NewTracedHandler(logger.Handler()))
		logger.Info("OpenTelemetry tracing enabled", "endpoint", cfg.OTEL.Endpoint)
	}

	sentryFlush, err := observability.InitSentry(cfg.Sentry, cfg.App.Env, cfg.DocuMCP.ServerVersion)
	if err != nil {
		return observabilityBundle{}, fmt.Errorf("initializing sentry: %w", err)
	}
	if cfg.Sentry.DSN != "" {
		logger.Info("Sentry error tracking enabled")
	}

	return observabilityBundle{
		Metrics:        metrics,
		TracerShutdown: tracerShutdown,
		SentryFlush:    sentryFlush,
		Logger:         logger,
	}, nil
}

// openBlobStore constructs the configured blob backend. When Driver is
// "local", "fs", or empty, it returns a filesystem-rooted FSBlob at
// storagePath. When Driver is "s3", it opens the configured S3-compatible
// bucket. Unknown drivers are an error (config validation should catch
// those before we get here).
func openBlobStore(ctx context.Context, cfg config.StorageConfig, storagePath string) (storage.Blob, error) {
	switch cfg.Driver {
	case "", "local", "fs":
		return storage.NewFSBlob(storagePath)
	case "s3":
		return storage.NewS3Blob(ctx, storage.S3Config{
			Endpoint:        cfg.S3Endpoint,
			Bucket:          cfg.S3Bucket,
			Region:          cfg.S3Region,
			AccessKeyID:     cfg.S3AccessKeyID,
			SecretAccessKey: cfg.S3SecretAccessKey,
			UsePathStyle:    cfg.S3UsePathStyle,
			ForceSSL:        cfg.S3ForceSSL,
		})
	default:
		return nil, fmt.Errorf("unknown storage driver %q", cfg.Driver)
	}
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
	if f.ControlBus != nil {
		if err := f.ControlBus.Close(); err != nil {
			f.Logger.Error("closing control bus", "error", err)
		}
	}
	if f.BlobStore != nil {
		if err := f.BlobStore.Close(); err != nil {
			f.Logger.Error("closing blob store", "error", err)
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
		cfg.Database.MaxOpenConns,
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
