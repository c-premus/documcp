package oauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// ---------------------------------------------------------------------------
// TestSetTokenHMACKey
// ---------------------------------------------------------------------------

func TestSetTokenHMACKey_CallingMultipleTimesDoesNotPanic(t *testing.T) {
	t.Cleanup(resetTokenHMACKey)

	// Repeated calls are safe — only the first non-nil key takes effect.
	SetTokenHMACKey([]byte("first"))
	SetTokenHMACKey([]byte("second"))
	SetTokenHMACKey(nil)
}

func TestSetTokenHMACKey_OnlyFirstCallTakesEffect(t *testing.T) {
	t.Cleanup(resetTokenHMACKey)

	key1 := []byte("first-key")
	key2 := []byte("second-key")

	SetTokenHMACKey(key1)
	SetTokenHMACKey(key2) // should be ignored

	input := "test-token"
	got := hashToken(input)

	// Expect HMAC with key1, not key2
	mac := hmac.New(sha256.New, key1)
	mac.Write([]byte(input))
	want := hex.EncodeToString(mac.Sum(nil))

	if got != want {
		t.Errorf("hashToken used wrong key: got %s, want %s", got[:16], want[:16])
	}
}

// ---------------------------------------------------------------------------
// TestHashTokenPaths
// ---------------------------------------------------------------------------

func TestHashTokenPaths_PlainSHA256(t *testing.T) {
	t.Cleanup(resetTokenHMACKey)
	resetTokenHMACKey() // ensure no key is set

	input := "test-token-value"
	got := hashToken(input)

	h := sha256.Sum256([]byte(input))
	want := hex.EncodeToString(h[:])

	if got != want {
		t.Errorf("hashToken without key: got %s, want %s", got[:16], want[:16])
	}
}

func TestHashTokenPaths_HMACSHA256(t *testing.T) {
	t.Cleanup(resetTokenHMACKey)

	key := []byte("my-secret-key")
	SetTokenHMACKey(key)

	input := "test-token-value"
	got := hashToken(input)

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(input))
	want := hex.EncodeToString(mac.Sum(nil))

	if got != want {
		t.Errorf("hashToken with key: got %s, want %s", got[:16], want[:16])
	}

	// HMAC hash should differ from plain SHA-256
	h := sha256.Sum256([]byte(input))
	plainHash := hex.EncodeToString(h[:])
	if got == plainHash {
		t.Error("HMAC-SHA256 and plain SHA-256 produced the same hash")
	}
}
