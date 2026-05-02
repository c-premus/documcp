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

	"github.com/c-premus/documcp/internal/observability"
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
	eventBus, err := queue.NewRedisEventBus(context.Background(), f.RedisClient, logger)
	if err != nil {
		return nil, fmt.Errorf("redis event bus: %w", err)
	}
	var eventBusOK bool
	defer func() {
		if !eventBusOK {
			eventBus.Close()
		}
	}()

	// --- River Workers + Client (full worker mode) ---
	// Workers operate on the OAuth repository directly (cleanup, reconciliation).
	// No queue worker currently parses or generates bearer tokens, so no
	// oauth.Service (and no HMAC key) is required in worker-only mode.
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
		f.BlobStore,
		f.WorkerTempDir,
		f.Config.Storage.MaxUploadSize,
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

	eventBusOK = true
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

// newHealthServer creates a minimal HTTP server with three surfaces:
//
//   - Liveness probes at /healthz and /health (alias). Always 200 if the
//     process is running.
//   - Readiness probes at /readyz and /health/ready (alias). 200 when both
//     the database and Redis respond to Ping, 503 otherwise. Pings go through
//     the uninstrumented BarePgxPool + BareRedisClient so the probe path
//     emits no otelpgx/redisotel spans on every k8s poll.
//   - Prometheus metrics at /metrics, gated by INTERNAL_API_TOKEN when set
//     and exposed unauthenticated with a one-time WARN log when unset
//     (mirrors serve-mode behavior in internal/server/routes.go).
//
// The path aliases let `documcp health --port <healthPort>` work in worker
// mode without flag changes — the binary probes /health/ready, which now
// resolves to the same handler as /readyz.
func newHealthServer(port int, f *Foundation) *http.Server {
	mux := http.NewServeMux()

	// Liveness — process up. No I/O; always succeeds.
	livenessHandler := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "ok")
	}
	mux.HandleFunc("GET /healthz", livenessHandler)
	mux.HandleFunc("GET /health", livenessHandler)

	// Readiness — DB + Redis reachable through the uninstrumented clients.
	readinessHandler := func(w http.ResponseWriter, r *http.Request) {
		pingCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := f.BarePgxPool.Ping(pingCtx); err != nil {
			http.Error(w, "database not ready", http.StatusServiceUnavailable)
			return
		}
		if f.BareRedisClient != nil {
			if err := f.BareRedisClient.Ping(pingCtx).Err(); err != nil {
				http.Error(w, "redis not ready", http.StatusServiceUnavailable)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "ready")
	}
	mux.HandleFunc("GET /readyz", readinessHandler)
	mux.HandleFunc("GET /health/ready", readinessHandler)

	// Prometheus metrics — same auth contract as serve-mode.
	if f.Metrics != nil {
		metricsHandler := observability.MetricsHandler()
		if token := f.Config.App.InternalAPIToken; token != "" {
			metricsHandler = observability.InternalTokenAuth(token)(metricsHandler)
		} else {
			f.Logger.Warn("metrics endpoint exposed without authentication (INTERNAL_API_TOKEN not set)")
		}
		mux.Handle("GET /metrics", metricsHandler)
		f.Logger.Info("Prometheus metrics endpoint registered", "path", "/metrics")
	}

	return &http.Server{
		Addr:              net.JoinHostPort("0.0.0.0", strconv.Itoa(port)),
		Handler:           mux,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
		ErrorLog:          slog.NewLogLogger(f.Logger.Handler(), slog.LevelError),
	}
}
