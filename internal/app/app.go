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
	"syscall"
	"time"

	"github.com/gorilla/sessions"
	"github.com/jmoiron/sqlx"

	"git.999.haus/chris/DocuMCP-go/internal/auth/oauth"
	"git.999.haus/chris/DocuMCP-go/internal/auth/oidc"
	"git.999.haus/chris/DocuMCP-go/internal/config"
	"git.999.haus/chris/DocuMCP-go/internal/database"
	mcphandler "git.999.haus/chris/DocuMCP-go/internal/handler/mcp"
	oauthhandler "git.999.haus/chris/DocuMCP-go/internal/handler/oauth"
	"git.999.haus/chris/DocuMCP-go/internal/repository"
	"git.999.haus/chris/DocuMCP-go/internal/server"
	"git.999.haus/chris/DocuMCP-go/internal/service"
)

// App holds all application dependencies wired together.
type App struct {
	Config *config.Config
	DB     *sqlx.DB
	Logger *slog.Logger
	Server *server.Server
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

	// --- Services ---
	documentService := service.NewDocumentService(documentRepo, logger)
	oauthService := oauth.NewService(oauthRepo, cfg.OAuth, cfg.App.URL, logger)

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
		ZimEnabled:          true,
		ConfluenceEnabled:   true,
		GitTemplatesEnabled: true,
	})

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
		Version:      cfg.DocuMCP.ServerVersion,
		MCPHandler:   mcpH,
		OAuthHandler: oauthH,
		OIDCHandler:  oidcH,
		OAuthService: oauthService,
		SessionStore: sessionStore,
	})

	logger.Info("MCP server configured",
		"name", cfg.DocuMCP.ServerName,
		"version", cfg.DocuMCP.ServerVersion,
	)

	return &App{
		Config: cfg,
		DB:     db,
		Logger: logger,
		Server: srv,
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
