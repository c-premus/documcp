package oauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"testing"

	"github.com/c-premus/documcp/internal/config"
)

// newHashOnlyService constructs a minimal *Service suitable for exercising
// hashToken / GenerateToken / ParseToken in isolation.
func newHashOnlyService(key []byte) *Service {
	return NewService(nil, config.OAuthConfig{}, "https://app.example.com", slog.Default(), key)
}

// tokenTestSvc is a shared hash-only service for token round-trip tests that
// do not care about HMAC key behavior. Its methods only read hmacKey after
// construction, so concurrent use from t.Parallel tests is safe.
var tokenTestSvc = newHashOnlyService(nil)

// ---------------------------------------------------------------------------
// TestService_HMACKey
// ---------------------------------------------------------------------------

func TestService_HashToken_SwitchesOnKey(t *testing.T) {
	t.Parallel()

	const input = "test-token-value"

	t.Run("nil key falls back to plain SHA-256", func(t *testing.T) {
		t.Parallel()
		svc := newHashOnlyService(nil)
		got := svc.hashToken(input)

		h := sha256.Sum256([]byte(input))
		want := hex.EncodeToString(h[:])
		if got != want {
			t.Errorf("hashToken without key: got %s, want %s", got[:16], want[:16])
		}
	})

	t.Run("non-nil key produces HMAC-SHA256", func(t *testing.T) {
		t.Parallel()
		key := []byte("my-secret-key")
		svc := newHashOnlyService(key)
		got := svc.hashToken(input)

		mac := hmac.New(sha256.New, key)
		mac.Write([]byte(input))
		want := hex.EncodeToString(mac.Sum(nil))
		if got != want {
			t.Errorf("hashToken with key: got %s, want %s", got[:16], want[:16])
		}

		h := sha256.Sum256([]byte(input))
		plainHash := hex.EncodeToString(h[:])
		if got == plainHash {
			t.Error("HMAC-SHA256 and plain SHA-256 produced the same hash")
		}
	})
}

func TestService_HashToken_KeyIsolation(t *testing.T) {
	t.Parallel()

	const input = "test-token-value"
	svc1 := newHashOnlyService([]byte("first-key"))
	svc2 := newHashOnlyService([]byte("second-key"))

	if svc1.hashToken(input) == svc2.hashToken(input) {
		t.Error("different HMAC keys produced the same hash")
	}
}
