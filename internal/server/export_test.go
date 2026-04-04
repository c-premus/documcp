package server

import (
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"time"
)

// Exported wrappers for unexported functions, used by the server_test package.

// HardenedTLSConfig exposes hardenedTLSConfig for testing.
func HardenedTLSConfig() *tls.Config { return hardenedTLSConfig() }

// GenerateSelfSignedCert exposes generateSelfSignedCert for testing.
func GenerateSelfSignedCert() (tls.Certificate, error) { return generateSelfSignedCert() }

// BuildTLSConfig exposes buildTLSConfig for testing.
func BuildTLSConfig(cfg TLSConfig) (*tls.Config, error) { return buildTLSConfig(cfg) }

// OverrideSelfSignedValidity sets selfSignedValidity and returns a cleanup func
// that restores the original value.
func OverrideSelfSignedValidity(d time.Duration) func() {
	orig := selfSignedValidity
	selfSignedValidity = d
	return func() { selfSignedValidity = orig }
}

// ServeOnListener starts the server's HTTP handler on the given listener in a
// background goroutine. This is used by TLS tests that need to wrap a listener
// with tls.NewListener before serving.
func (s *Server) ServeOnListener(ln net.Listener) error {
	s.httpServer.Addr = ln.Addr().String()
	err := s.httpServer.Serve(ln)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
