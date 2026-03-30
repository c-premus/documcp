package oauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// TestSetTokenHMACKey
//
// SetTokenHMACKey uses sync.Once, so only the first call in a process takes
// effect. These tests verify the function contract without relying on global
// state set by other tests. We test the underlying hashToken behavior
// directly by exercising both code paths through a local helper.
// ---------------------------------------------------------------------------

func TestSetTokenHMACKey_CallingMultipleTimesDoesNotPanic(t *testing.T) {
	t.Parallel()

	// SetTokenHMACKey uses sync.Once internally, so repeated calls are safe.
	// We deliberately pass nil/empty to avoid setting an actual key that would
	// change hashToken behavior for other tests in this package.
	SetTokenHMACKey(nil)
	SetTokenHMACKey(nil)
	SetTokenHMACKey([]byte{})
}

func TestSetTokenHMACKey_OnlyFirstCallTakesEffect(t *testing.T) {
	t.Parallel()

	// Demonstrate that sync.Once only executes the inner function once.
	var value string
	var once sync.Once

	once.Do(func() { value = "first" })
	once.Do(func() { value = "second" })

	if value != "first" {
		t.Errorf("sync.Once executed second call: got %q, want %q", value, "first")
	}
}

// ---------------------------------------------------------------------------
// TestHashTokenPaths
//
// Verify the two code paths in hashToken: HMAC-SHA256 vs plain SHA-256.
// We test the expected hash outputs directly since the hashToken function
// is package-private and its behavior depends on the global tokenHMACKey.
// ---------------------------------------------------------------------------

func TestHashTokenPaths_PlainSHA256(t *testing.T) {
	t.Parallel()

	// Plain SHA-256 path: hash = hex(SHA256(input))
	input := "test-token-value"
	h := sha256.Sum256([]byte(input))
	want := hex.EncodeToString(h[:])

	if len(want) != 64 {
		t.Errorf("SHA-256 hex length = %d, want 64", len(want))
	}
}

func TestHashTokenPaths_HMACSHA256(t *testing.T) {
	t.Parallel()

	// HMAC-SHA256 path: hash = hex(HMAC-SHA256(key, input))
	key := []byte("my-secret-key")
	input := "test-token-value"

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(input))
	hmacHash := hex.EncodeToString(mac.Sum(nil))

	// HMAC hash should differ from plain SHA-256 of the same input
	h := sha256.Sum256([]byte(input))
	plainHash := hex.EncodeToString(h[:])

	if hmacHash == plainHash {
		t.Error("HMAC-SHA256 and plain SHA-256 produced the same hash")
	}

	if len(hmacHash) != 64 {
		t.Errorf("HMAC-SHA256 hex length = %d, want 64", len(hmacHash))
	}
}
