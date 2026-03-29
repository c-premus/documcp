package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// Config holds HTTP server configuration.
type Config struct {
	Host              string
	Port              int
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	TrustedProxies    []*net.IPNet
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Host:              "0.0.0.0",
		Port:              8080,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

// Server wraps an HTTP server with chi routing and structured logging.
type Server struct {
	router         chi.Router
	httpServer     *http.Server
	logger         *slog.Logger
	trustedProxies []*net.IPNet
}

// New creates a new HTTP server with the given configuration and logger.
func New(cfg Config, logger *slog.Logger) *Server {
	r := chi.NewRouter()

	s := &Server{
		router: r,
		httpServer: &http.Server{
			Addr:              fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
			Handler:           r,
			ReadTimeout:       cfg.ReadTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
			ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		},
		logger:         logger,
		trustedProxies: cfg.TrustedProxies,
	}

	return s
}

// Start starts the HTTP server. It blocks until the server stops or returns an
// Error http.ErrServerClosed is not returned when the server is shut down
// gracefully via Shutdown.
func (s *Server) Start() error {
	s.logger.Info("starting HTTP server", "addr", s.httpServer.Addr)

	err := s.httpServer.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("starting HTTP server: %w", err)
	}

	return nil
}

// Shutdown gracefully stops the server using the provided context for the
// deadline. In-flight requests are given time to complete; idle connections
// are closed immediately.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down HTTP server")

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutting down HTTP server: %w", err)
	}

	return nil
}

// Close immediately closes all active connections without waiting for them to
// finish. Use after Shutdown times out to force-close lingering long-lived
// connections (e.g. MCP SSE streams, admin event streams).
func (s *Server) Close() error {
	return s.httpServer.Close()
}

// Router returns the chi router so that external packages can register routes.
func (s *Server) Router() chi.Router {
	return s.router
}

// Addr returns the address the server is configured to listen on. This is
// useful for tests that need to know the address after calling Start.
func (s *Server) Addr() string {
	return s.httpServer.Addr
}

// ListenAndServeOnAvailablePort starts the server on a random available port.
// It is intended for use in tests. The actual listener address can be retrieved
// from the returned net.Listener.
func (s *Server) ListenAndServeOnAvailablePort() (net.Listener, error) {
	ln, err := new(net.ListenConfig).Listen(context.Background(), "tcp", ":0")
	if err != nil {
		return nil, fmt.Errorf("listening on available port: %w", err)
	}

	s.httpServer.Addr = ln.Addr().String()
	s.logger.Info("starting HTTP server", "addr", s.httpServer.Addr)

	go func() {
		if serveErr := s.httpServer.Serve(ln); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			s.logger.Error("HTTP server error", "error", serveErr)
		}
	}()

	return ln, nil
}
