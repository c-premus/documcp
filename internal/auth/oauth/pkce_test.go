package oauth

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// VerifyPKCE additional edge-case tests
//
// Core cases (RFC 7636, valid/wrong/empty verifier, padding) are in
// token_test.go. This file adds boundary and encoding edge cases.
// ---------------------------------------------------------------------------

func TestVerifyPKCE_BothEmpty(t *testing.T) {
	t.Parallel()

	// SHA256("") produces a known non-empty hash, so empty challenge != computed
	if VerifyPKCE("", "") {
		t.Error("VerifyPKCE returned true when both challenge and verifier are empty")
	}
}

func TestVerifyPKCE_StandardBase64NonURLSafe(t *testing.T) {
	t.Parallel()

	// Standard base64 uses '+' and '/' instead of '-' and '_'.
	verifier := "verifier-with-special-base64-chars"
	h := sha256.Sum256([]byte(verifier))
	standard := base64.RawStdEncoding.EncodeToString(h[:])
	urlSafe := base64.RawURLEncoding.EncodeToString(h[:])

	if standard == urlSafe {
		t.Skip("no difference between standard and URL-safe encoding for this input")
	}

	if VerifyPKCE(standard, verifier) {
		t.Error("VerifyPKCE accepted standard base64 challenge instead of URL-safe")
	}
}

func TestVerifyPKCE_AlteredChallenge(t *testing.T) {
	t.Parallel()

	s256Challenge := func(v string) string {
		h := sha256.Sum256([]byte(v))
		return base64.RawURLEncoding.EncodeToString(h[:])
	}

	verifier := "timing-attack-test-verifier"
	challenge := s256Challenge(verifier)

	// Flip one character in the challenge
	modified := []byte(challenge)
	modified[0] ^= 1
	alteredChallenge := string(modified)

	if VerifyPKCE(alteredChallenge, verifier) {
		t.Error("VerifyPKCE returned true for single-character-altered challenge")
	}
}

func TestVerifyPKCE_MaxLengthVerifier(t *testing.T) {
	t.Parallel()

	// RFC 7636 allows verifiers up to 128 characters
	verifier := strings.Repeat("A", 128)
	h := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(h[:])

	if !VerifyPKCE(challenge, verifier) {
		t.Error("VerifyPKCE returned false for max-length (128 char) verifier")
	}
}

func TestVerifyPKCE_MinLengthVerifier(t *testing.T) {
	t.Parallel()

	// RFC 7636 allows verifiers of at least 43 characters
	verifier := strings.Repeat("x", 43)
	h := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(h[:])

	if !VerifyPKCE(challenge, verifier) {
		t.Error("VerifyPKCE returned false for min-length (43 char) verifier")
	}
}

func TestVerifyPKCE_URLSafeCharacters(t *testing.T) {
	t.Parallel()

	// RFC 7636 verifiers use unreserved characters: [A-Z] / [a-z] / [0-9] / "-" / "." / "_" / "~"
	verifier := "ABCDEFghijklmnop0123456789-._~ABCDEFghijklm"
	h := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(h[:])

	if !VerifyPKCE(challenge, verifier) {
		t.Error("VerifyPKCE returned false for verifier with URL-safe characters")
	}
}

func TestVerifyPKCE_UnicodeVerifier(t *testing.T) {
	t.Parallel()

	// While RFC 7636 restricts verifiers to ASCII unreserved characters,
	// the function should still handle arbitrary byte sequences consistently.
	verifier := "unicode-verifier-\u00e9\u00e8\u00ea"
	h := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(h[:])

	if !VerifyPKCE(challenge, verifier) {
		t.Error("VerifyPKCE returned false for UTF-8 verifier")
	}
}
