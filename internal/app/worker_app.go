package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/c-premus/documcp/internal/auth/oauth"
	"github.com/c-premus/documcp/internal/queue"
	"github.com/c-premus/documcp/internal/service"
)

// WorkerApp manages the River queue worker lifecycle. It runs job workers,
// periodic jobs, and a minimal health HTTP endpoint for K8s probes.
type WorkerApp struct {
	Foundation   *Foundation
	RiverClient  *queue.RiverClient
	EventBus     queue.EventSubscriber
	HealthServer *http.Server
}

// NewWorkerApp creates a WorkerApp with River workers and a health endpoint.
func NewWorkerApp(f *Foundation) (*WorkerApp, error) {
	logger := f.Logger

	// --- EventBus (Redis-backed for cross-instance event delivery) ---
	eventBus := queue.NewRedisEventBus(context.Background(), f.RedisClient, logger)

	// Token HMAC key — needed by workers that process token-related jobs.
	sessionSecret := f.Config.OAuth.SessionSecret
	if sessionSecret != "" {
		hmacKey, err := deriveKey([]byte(sessionSecret), f.Config.OAuth.HKDFSalt, "oauth-token-hmac")
		if err != nil {
			return nil, fmt.Errorf("deriving token HMAC key: %w", err)
		}
		oauth.SetTokenHMACKey(hmacKey)
	}

	// --- River Workers + Client (full worker mode) ---
	rs, err := buildRiverClient(f, eventBus, false)
	if err != nil {
		return nil, err
	}
	riverClient := rs.Client

	logger.Info("river queue client configured", "mode", "worker")

	// --- Document Pipeline (needed by workers) ---
	documentService := service.NewDocumentService(f.DocumentRepo, logger)
	documentPipeline := service.NewDocumentPipeline(
		documentService,
		f.ExtractorRegistry,
		riverClient,
		f.StoragePath,
	)

	// Wire pipeline into document workers (resolves circular dependency).
	rs.ExtractWorker.Pipeline = documentPipeline
	rs.ExtractWorker.Metrics = f.Metrics

	// --- Health Endpoint ---
	healthPort := f.Config.Queue.HealthPort
	healthServer := newHealthServer(healthPort, f)

	logger.Info("worker health endpoint configured",
		"port", healthPort,
	)

	return &WorkerApp{
		Foundation:   f,
		RiverClient:  riverClient,
		EventBus:     eventBus,
		HealthServer: healthServer,
	}, nil
}

// Start begins processing jobs and blocks until a shutdown signal is received.
func (w *WorkerApp) Start(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start River client for job processing.
	if err := w.RiverClient.Start(ctx); err != nil {
		return fmt.Errorf("starting river client: %w", err)
	}
	w.Foundation.Logger.Info("river queue started (worker mode)")

	// Forward events for metrics (no SSE subscribers in worker mode).
	w.RiverClient.StartEventForwarding()

	// Re-dispatch jobs for documents stuck in intermediate states.
	queue.RecoverStuckDocuments(ctx, w.RiverClient, newDocStatusAdapter(w.Foundation.DocumentRepo), w.Foundation.Logger)

	// Start health endpoint in background.
	healthErrCh := make(chan error, 1)
	go func() {
		if err := w.HealthServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			healthErrCh <- err
		}
	}()

	w.Foundation.Logger.Info("worker ready",
		"health_addr", w.HealthServer.Addr,
	)

	// Block until shutdown signal or health server error.
	select {
	case err := <-healthErrCh:
		return fmt.Errorf("health server error: %w", err)
	case <-ctx.Done():
		w.Foundation.Logger.Info("shutdown signal received")
	}

	return nil
}

// Close releases WorkerApp-specific resources.
func (w *WorkerApp) Close() error {
	// Shut down health server.
	if w.HealthServer != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := w.HealthServer.Shutdown(shutdownCtx); err != nil {
			w.Foundation.Logger.Error("shutting down health server", "error", err)
		}
	}

	// Close EventBus.
	if w.EventBus != nil {
		w.EventBus.Close()
	}

	// Stop River client.
	if w.RiverClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), w.Foundation.Config.App.QueueStopTimeout)
		defer cancel()
		if err := w.RiverClient.Stop(ctx); err != nil {
			w.Foundation.Logger.Error("stopping river client", "error", err)
		}
	}

	return nil
}

// newHealthServer creates a minimal HTTP server for K8s liveness and readiness probes.
func newHealthServer(port int, f *Foundation) *http.Server {
	mux := http.NewServeMux()

	// Liveness probe — always returns 200 if the process is running.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "ok")
	})

	// Readiness probe — verifies database connectivity.
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := f.PgxPool.Ping(r.Context()); err != nil {
			http.Error(w, "database not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "ready")
	})

	return &http.Server{
		Addr:              net.JoinHostPort("0.0.0.0", strconv.Itoa(port)),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ErrorLog:          slog.NewLogLogger(f.Logger.Handler(), slog.LevelError),
	}
}
