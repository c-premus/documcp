package security_test

import (
	"testing"

	"github.com/c-premus/documcp/internal/security"
)

func BenchmarkValidateExternalURL_ValidHTTPS(b *testing.B) {
	for b.Loop() {
		_ = security.ValidateExternalURL("https://example.com")
	}
}

func BenchmarkValidateExternalURL_BlockedPrivateIP(b *testing.B) {
	for b.Loop() {
		_ = security.ValidateExternalURL("http://10.0.0.1")
	}
}

func BenchmarkValidateExternalURL_BlockedLocalhost(b *testing.B) {
	for b.Loop() {
		_ = security.ValidateExternalURL("http://localhost")
	}
}

func BenchmarkValidateExternalURL_InvalidScheme(b *testing.B) {
	for b.Loop() {
		_ = security.ValidateExternalURL("ftp://example.com")
	}
}
