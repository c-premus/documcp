package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"
)

// TLSConfig holds TLS settings for the HTTP server. When Enabled is true and
// CertFile/KeyFile are empty, the server generates an ephemeral self-signed
// certificate (suitable for development, not production).
type TLSConfig struct {
	Enabled  bool
	CertFile string
	KeyFile  string
	Port     int // HTTPS listen port (default 8443)
}

// selfSignedValidity is the duration a generated self-signed certificate is
// valid for. Var (not const) so tests can override.
var selfSignedValidity = 365 * 24 * time.Hour

// hardenedTLSConfig returns a *tls.Config with modern security defaults:
//   - MinVersion TLS 1.2 (TLS 1.3 negotiated when both sides support it)
//   - Explicit cipher suite preference for TLS 1.2 (TLS 1.3 suites are not configurable)
//   - Strong curve preferences (X25519 first, then P-256)
func hardenedTLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
		},
		CipherSuites: []uint16{
			// TLS 1.3 cipher suites are automatic and not listed here.
			// TLS 1.2 AEAD ciphers only (no CBC):
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		},
	}
}

// generateSelfSignedCert creates an ephemeral ECDSA P-256 self-signed
// certificate valid for localhost, 127.0.0.1, and ::1. It returns a
// tls.Certificate ready for use in a tls.Config.
func generateSelfSignedCert() (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generating ECDSA key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generating serial number: %w", err)
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      pkix.Name{Organization: []string{"DocuMCP Self-Signed"}},
		NotBefore:    now,
		NotAfter:     now.Add(selfSignedValidity),

		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},

		DNSNames:    []string{"localhost"},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},

		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("creating certificate: %w", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("marshaling private key: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return tls.X509KeyPair(certPEM, keyPEM)
}

// buildTLSConfig returns a *tls.Config for the given TLSConfig. When CertFile
// and KeyFile are provided, it loads them from disk. Otherwise it generates an
// ephemeral self-signed certificate.
func buildTLSConfig(cfg TLSConfig) (*tls.Config, error) {
	tlsCfg := hardenedTLSConfig()

	if cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("loading TLS certificate: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
		return tlsCfg, nil
	}

	cert, err := generateSelfSignedCert()
	if err != nil {
		return nil, fmt.Errorf("generating self-signed certificate: %w", err)
	}
	tlsCfg.Certificates = []tls.Certificate{cert}
	return tlsCfg, nil
}
