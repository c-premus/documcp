package server_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/c-premus/documcp/internal/server"
)

func TestHardenedTLSConfig(t *testing.T) {
	t.Parallel()

	cfg := server.HardenedTLSConfig()

	t.Run("MinVersion is TLS 1.2", func(t *testing.T) {
		t.Parallel()
		if cfg.MinVersion != tls.VersionTLS12 {
			t.Errorf("MinVersion = %d, want %d (TLS 1.2)", cfg.MinVersion, tls.VersionTLS12)
		}
	})

	t.Run("CurvePreferences contains X25519 and P-256", func(t *testing.T) {
		t.Parallel()
		want := map[tls.CurveID]bool{
			tls.X25519:    false,
			tls.CurveP256: false,
		}
		for _, c := range cfg.CurvePreferences {
			if _, ok := want[c]; ok {
				want[c] = true
			}
		}
		for curve, found := range want {
			if !found {
				t.Errorf("CurvePreferences missing curve %d", curve)
			}
		}
	})

	t.Run("X25519 is preferred over P-256", func(t *testing.T) {
		t.Parallel()
		if len(cfg.CurvePreferences) < 2 {
			t.Fatal("expected at least 2 curve preferences")
		}
		if cfg.CurvePreferences[0] != tls.X25519 {
			t.Errorf("first curve = %d, want X25519 (%d)", cfg.CurvePreferences[0], tls.X25519)
		}
	})

	t.Run("CipherSuites are AEAD only (no CBC)", func(t *testing.T) {
		t.Parallel()
		cbcSuites := map[uint16]string{
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA:    "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA",
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA:    "TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA",
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA:      "TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA",
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA:      "TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA",
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256:   "TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256",
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256: "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256",
		}
		for _, suite := range cfg.CipherSuites {
			if name, isCBC := cbcSuites[suite]; isCBC {
				t.Errorf("CipherSuites contains CBC suite: %s", name)
			}
		}
		if len(cfg.CipherSuites) == 0 {
			t.Error("CipherSuites is empty")
		}
	})

	t.Run("CipherSuites contains expected AEAD suites", func(t *testing.T) {
		t.Parallel()
		expected := []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		}
		suiteSet := make(map[uint16]bool, len(cfg.CipherSuites))
		for _, s := range cfg.CipherSuites {
			suiteSet[s] = true
		}
		for _, e := range expected {
			if !suiteSet[e] {
				t.Errorf("CipherSuites missing expected suite 0x%04x", e)
			}
		}
	})
}

func TestGenerateSelfSignedCert(t *testing.T) {
	t.Parallel()

	cert, err := server.GenerateSelfSignedCert()
	if err != nil {
		t.Fatalf("GenerateSelfSignedCert() error = %v", err)
	}

	if len(cert.Certificate) == 0 {
		t.Fatal("certificate chain is empty")
	}

	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("parsing leaf certificate: %v", err)
	}

	t.Run("SANs include localhost", func(t *testing.T) {
		t.Parallel()
		if !slices.Contains(leaf.DNSNames, "localhost") {
			t.Errorf("DNSNames = %v, want to contain 'localhost'", leaf.DNSNames)
		}
	})

	t.Run("SANs include 127.0.0.1", func(t *testing.T) {
		t.Parallel()
		if !containsIP(leaf.IPAddresses, net.IPv4(127, 0, 0, 1)) {
			t.Errorf("IPAddresses = %v, want to contain 127.0.0.1", leaf.IPAddresses)
		}
	})

	t.Run("SANs include IPv6 loopback", func(t *testing.T) {
		t.Parallel()
		if !containsIP(leaf.IPAddresses, net.IPv6loopback) {
			t.Errorf("IPAddresses = %v, want to contain ::1", leaf.IPAddresses)
		}
	})

	t.Run("KeyUsage includes DigitalSignature", func(t *testing.T) {
		t.Parallel()
		if leaf.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
			t.Errorf("KeyUsage = %d, want DigitalSignature bit set", leaf.KeyUsage)
		}
	})

	t.Run("ExtKeyUsage includes ServerAuth", func(t *testing.T) {
		t.Parallel()
		if !slices.Contains(leaf.ExtKeyUsage, x509.ExtKeyUsageServerAuth) {
			t.Errorf("ExtKeyUsage = %v, want ServerAuth", leaf.ExtKeyUsage)
		}
	})

	t.Run("Organization is DocuMCP Self-Signed", func(t *testing.T) {
		t.Parallel()
		want := "DocuMCP Self-Signed"
		if !slices.Contains(leaf.Subject.Organization, want) {
			t.Errorf("Subject.Organization = %v, want %q", leaf.Subject.Organization, want)
		}
	})

	t.Run("certificate is valid now", func(t *testing.T) {
		t.Parallel()
		now := time.Now()
		if now.Before(leaf.NotBefore) {
			t.Errorf("NotBefore = %v is in the future", leaf.NotBefore)
		}
		if now.After(leaf.NotAfter) {
			t.Errorf("NotAfter = %v is in the past", leaf.NotAfter)
		}
	})
}

func TestBuildTLSConfig_SelfSigned(t *testing.T) {
	t.Parallel()

	cfg := server.TLSConfig{
		Enabled:  true,
		CertFile: "",
		KeyFile:  "",
	}

	tlsCfg, err := server.BuildTLSConfig(cfg)
	if err != nil {
		t.Fatalf("BuildTLSConfig() error = %v", err)
	}
	if tlsCfg == nil {
		t.Fatal("BuildTLSConfig() returned nil config")
	}
	if got := len(tlsCfg.Certificates); got != 1 {
		t.Errorf("len(Certificates) = %d, want 1", got)
	}
	if tlsCfg.MinVersion != tls.VersionTLS12 {
		t.Errorf("MinVersion = %d, want %d (TLS 1.2)", tlsCfg.MinVersion, tls.VersionTLS12)
	}
}

func TestBuildTLSConfig_FromFiles(t *testing.T) {
	t.Parallel()

	certPEM, keyPEM := generateTestCertPEM(t)

	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")

	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		t.Fatalf("writing cert file: %v", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("writing key file: %v", err)
	}

	cfg := server.TLSConfig{
		Enabled:  true,
		CertFile: certPath,
		KeyFile:  keyPath,
	}

	tlsCfg, err := server.BuildTLSConfig(cfg)
	if err != nil {
		t.Fatalf("BuildTLSConfig() error = %v", err)
	}
	if got := len(tlsCfg.Certificates); got != 1 {
		t.Errorf("len(Certificates) = %d, want 1", got)
	}
}

func TestBuildTLSConfig_InvalidFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		certFile string
		keyFile  string
	}{
		{
			name:     "both paths non-existent",
			certFile: "/nonexistent/cert.pem",
			keyFile:  "/nonexistent/key.pem",
		},
		{
			name:     "cert exists but key does not",
			certFile: createTempFile(t, "cert.pem", []byte("not a real cert")),
			keyFile:  "/nonexistent/key.pem",
		},
		{
			name:     "both exist but contain invalid PEM",
			certFile: createTempFile(t, "bad-cert.pem", []byte("garbage")),
			keyFile:  createTempFile(t, "bad-key.pem", []byte("garbage")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := server.TLSConfig{
				Enabled:  true,
				CertFile: tt.certFile,
				KeyFile:  tt.keyFile,
			}

			_, err := server.BuildTLSConfig(cfg)
			if err == nil {
				t.Error("BuildTLSConfig() expected error for invalid files, got nil")
			}
		})
	}
}

func TestStartTLS_SelfSigned(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	cfg := server.DefaultConfig()
	cfg.TLS = server.TLSConfig{Enabled: true}

	srv := server.New(cfg, logger)

	// Register a simple health endpoint.
	srv.Router().Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "ok")
	})

	// Build a TLS config with a self-signed cert.
	tlsCfg, err := server.BuildTLSConfig(server.TLSConfig{Enabled: true})
	if err != nil {
		t.Fatalf("BuildTLSConfig() error = %v", err)
	}

	// Listen on a random port, wrap with TLS.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen error = %v", err)
	}
	tlsLn := tls.NewListener(ln, tlsCfg)

	// Serve in background via the exported test helper.
	go func() {
		_ = srv.ServeOnListener(tlsLn)
	}()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})

	// HTTPS client that trusts any certificate (self-signed).
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec // test-only
			},
		},
		Timeout: 3 * time.Second,
	}

	addr := tlsLn.Addr().String()
	resp, err := client.Get(fmt.Sprintf("https://%s/healthz", addr))
	if err != nil {
		t.Fatalf("HTTPS GET error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	if got := string(body); got != "ok" {
		t.Errorf("body = %q, want %q", got, "ok")
	}

	if resp.TLS == nil {
		t.Error("response TLS state is nil, expected a TLS connection")
	}
}

func TestSelfSignedValidity_Override(t *testing.T) {
	// Not parallel: mutates package-level selfSignedValidity.
	customValidity := 48 * time.Hour
	restore := server.OverrideSelfSignedValidity(customValidity)
	defer restore()

	cert, err := server.GenerateSelfSignedCert()
	if err != nil {
		t.Fatalf("GenerateSelfSignedCert() error = %v", err)
	}

	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("parsing certificate: %v", err)
	}

	actualValidity := leaf.NotAfter.Sub(leaf.NotBefore)
	tolerance := time.Minute
	diff := actualValidity - customValidity
	if diff < -tolerance || diff > tolerance {
		t.Errorf("validity = %v, want ~%v (tolerance %v)", actualValidity, customValidity, tolerance)
	}
}

// --- helpers ---

// generateTestCertPEM generates a self-signed certificate via the server
// package and returns PEM-encoded cert and key bytes for writing to disk.
func generateTestCertPEM(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()

	cert, err := server.GenerateSelfSignedCert()
	if err != nil {
		t.Fatalf("GenerateSelfSignedCert() error = %v", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Certificate[0],
	})

	// The PrivateKey is *ecdsa.PrivateKey; marshal it to PEM.
	ecKey, ok := cert.PrivateKey.(*ecdsa.PrivateKey)
	if !ok {
		t.Fatal("expected *ecdsa.PrivateKey")
	}
	keyDER, err := x509.MarshalECPrivateKey(ecKey)
	if err != nil {
		t.Fatalf("marshaling EC private key: %v", err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyDER,
	})

	return certPEM, keyPEM
}

// createTempFile writes content to a temp file and returns its path.
func createTempFile(t *testing.T, name string, content []byte) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	return path
}

// containsIP reports whether ips contains an IP equal to target.
func containsIP(ips []net.IP, target net.IP) bool {
	for _, ip := range ips {
		if ip.Equal(target) {
			return true
		}
	}
	return false
}
