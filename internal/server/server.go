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
	TLS               TLSConfig
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
	tlsCfg         TLSConfig
	redirectAddr   string       // host:port for HTTP→HTTPS redirect listener
	redirectServer *http.Server // HTTP→HTTPS redirect (nil when TLS disabled)
}

// New creates a new HTTP server with the given configuration and logger.
func New(cfg Config, logger *slog.Logger) *Server {
	r := chi.NewRouter()

	// When TLS is enabled, the main server listens on the TLS port and a
	// separate redirect server handles the plain HTTP port. When TLS is
	// disabled, the main server listens on the plain HTTP port directly.
	listenPort := cfg.Port
	if cfg.TLS.Enabled && cfg.TLS.Port > 0 {
		listenPort = cfg.TLS.Port
	}

	s := &Server{
		router: r,
		httpServer: &http.Server{
			Addr:              fmt.Sprintf("%s:%d", cfg.Host, listenPort),
			Handler:           r,
			ReadTimeout:       cfg.ReadTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
			ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		},
		logger:         logger,
		trustedProxies: cfg.TrustedProxies,
		tlsCfg:         cfg.TLS,
		redirectAddr:   fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
	}

	return s
}

// Start starts the HTTP server. When TLS is enabled, it configures a hardened
// tls.Config and starts an HTTPS listener. If no certificate files are
// provided, an ephemeral self-signed certificate is generated (development
// only). A plaintext HTTP listener on port 80 redirects all traffic to HTTPS.
//
// When TLS is disabled, it starts a plaintext HTTP listener (reverse-proxy
// mode). The method blocks until the server stops; http.ErrServerClosed is not
// returned when the server is shut down gracefully via Shutdown.
func (s *Server) Start() error {
	if !s.tlsCfg.Enabled {
		s.logger.Info("starting HTTP server", "addr", s.httpServer.Addr)
		err := s.httpServer.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("starting HTTP server: %w", err)
		}
		return nil
	}

	tlsConfig, err := buildTLSConfig(s.tlsCfg)
	if err != nil {
		return fmt.Errorf("configuring TLS: %w", err)
	}
	s.httpServer.TLSConfig = tlsConfig

	if s.tlsCfg.CertFile == "" {
		s.logger.Warn("TLS enabled with self-signed certificate (development only)")
	} else {
		s.logger.Info("TLS enabled with provided certificate",
			"cert", s.tlsCfg.CertFile, "key", s.tlsCfg.KeyFile)
	}

	// Start HTTP→HTTPS redirect listener on port 80.
	s.startRedirectServer()

	s.logger.Info("starting HTTPS server", "addr", s.httpServer.Addr)

	// Certs are already loaded in TLSConfig.Certificates — pass empty paths
	// to avoid double-loading from disk.
	err = s.httpServer.ListenAndServeTLS("", "")
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("starting HTTPS server: %w", err)
	}

	return nil
}

// startRedirectServer launches a background HTTP listener on the plain HTTP
// port (SERVER_PORT) that redirects all requests to the HTTPS address.
// The redirect target uses the server's configured HTTPS address rather than
// reflecting the request Host header to prevent host header injection.
func (s *Server) startRedirectServer() {
	// Use the configured HTTPS listen address as the redirect target.
	// This prevents an attacker from injecting a crafted Host header
	// to redirect users to a malicious domain.
	httpsHost := s.httpServer.Addr
	s.redirectServer = &http.Server{
		Addr:              s.redirectAddr,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			target := "https://" + httpsHost + r.URL.RequestURI()
			//nolint:gosec // G710: host portion is the server-configured TLS address; RequestURI() returns path+query only, never scheme or host
			http.Redirect(w, r, target, http.StatusMovedPermanently)
		}),
	}

	go func() {
		s.logger.Info("starting HTTP→HTTPS redirect server", "addr", s.redirectServer.Addr)
		if err := s.redirectServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("HTTP redirect server error", "error", err)
		}
	}()
}

// Shutdown gracefully stops the server using the provided context for the
// deadline. In-flight requests are given time to complete; idle connections
// are closed immediately. When TLS is enabled, the HTTP→HTTPS redirect
// server is also shut down.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down HTTP server")

	if s.redirectServer != nil {
		if err := s.redirectServer.Shutdown(ctx); err != nil {
			s.logger.Error("shutting down redirect server", "error", err)
		}
	}

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
