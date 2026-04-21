package oauth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// TestGenerateToken
// ---------------------------------------------------------------------------

func TestGenerateToken(t *testing.T) {
	t.Parallel()

	t.Run("returns non-nil token pair", func(t *testing.T) {
		t.Parallel()
		tp, err := tokenTestSvc.GenerateToken()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tp == nil {
			t.Fatal("expected non-nil TokenPair")
		}
	})

	t.Run("plaintext is 64 characters", func(t *testing.T) {
		t.Parallel()
		tp, err := tokenTestSvc.GenerateToken()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := len(tp.Plaintext); got != 64 {
			t.Errorf("plaintext length = %d, want 64", got)
		}
	})

	t.Run("plaintext contains only alphanumeric characters", func(t *testing.T) {
		t.Parallel()
		const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
		tp, err := tokenTestSvc.GenerateToken()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for i, c := range tp.Plaintext {
			if !strings.ContainsRune(charset, c) {
				t.Errorf("plaintext[%d] = %q, not in charset", i, string(c))
			}
		}
	})

	t.Run("hash is SHA-256 hex of plaintext", func(t *testing.T) {
		t.Parallel()
		tp, err := tokenTestSvc.GenerateToken()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := sha256Hex(tp.Plaintext)
		if tp.Hash != want {
			t.Errorf("hash = %q, want SHA-256(%q) = %q", tp.Hash, tp.Plaintext, want)
		}
	})

	t.Run("hash is 64-character hex string", func(t *testing.T) {
		t.Parallel()
		tp, err := tokenTestSvc.GenerateToken()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := len(tp.Hash); got != 64 {
			t.Errorf("hash length = %d, want 64", got)
		}
	})

	t.Run("ID defaults to zero", func(t *testing.T) {
		t.Parallel()
		tp, err := tokenTestSvc.GenerateToken()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tp.ID != 0 {
			t.Errorf("ID = %d, want 0", tp.ID)
		}
	})

	t.Run("successive calls produce unique tokens", func(t *testing.T) {
		t.Parallel()
		tp1, err := tokenTestSvc.GenerateToken()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		tp2, err := tokenTestSvc.GenerateToken()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tp1.Plaintext == tp2.Plaintext {
			t.Error("two generated tokens have identical plaintext")
		}
		if tp1.Hash == tp2.Hash {
			t.Error("two generated tokens have identical hash")
		}
	})
}

// ---------------------------------------------------------------------------
// TestTokenPairSetID
// ---------------------------------------------------------------------------

func TestTokenPairSetID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   int64
	}{
		{name: "positive id", id: 42},
		{name: "large id", id: 9999999},
		{name: "id of 1", id: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tp, err := tokenTestSvc.GenerateToken()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			originalPlaintext := tp.Plaintext
			tp.SetID(tt.id)

			if tp.ID != tt.id {
				t.Errorf("ID = %d, want %d", tp.ID, tt.id)
			}

			// Check the plaintext starts with "id|"
			prefix := strings.SplitN(tp.Plaintext, "|", 2)
			if len(prefix) != 2 {
				t.Fatalf("plaintext %q does not contain pipe separator", tp.Plaintext)
			}
			if prefix[1] != originalPlaintext {
				t.Errorf("random portion after SetID = %q, want %q", prefix[1], originalPlaintext)
			}
		})
	}

	t.Run("hash is not modified by SetID", func(t *testing.T) {
		t.Parallel()
		tp, err := tokenTestSvc.GenerateToken()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		originalHash := tp.Hash
		tp.SetID(99)
		if tp.Hash != originalHash {
			t.Errorf("hash changed after SetID: got %q, want %q", tp.Hash, originalHash)
		}
	})
}

// ---------------------------------------------------------------------------
// TestParseToken
// ---------------------------------------------------------------------------

func TestParseToken(t *testing.T) {
	t.Parallel()

	t.Run("valid token", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name      string
			plaintext string
			wantID    int64
			wantHash  string
		}{
			{
				name:      "simple id and random",
				plaintext: "42|abcdef",
				wantID:    42,
				wantHash:  sha256Hex("abcdef"),
			},
			{
				name:      "id of 1",
				plaintext: "1|xyz123",
				wantID:    1,
				wantHash:  sha256Hex("xyz123"),
			},
			{
				name:      "large id",
				plaintext: "9999999|secret",
				wantID:    9999999,
				wantHash:  sha256Hex("secret"),
			},
			{
				name:      "random portion contains pipe",
				plaintext: "7|part1|part2",
				wantID:    7,
				wantHash:  sha256Hex("part1|part2"),
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				id, hash, err := tokenTestSvc.ParseToken(tt.plaintext)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if id != tt.wantID {
					t.Errorf("id = %d, want %d", id, tt.wantID)
				}
				if hash != tt.wantHash {
					t.Errorf("hash = %q, want %q", hash, tt.wantHash)
				}
			})
		}
	})

	t.Run("invalid format", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name      string
			plaintext string
			wantErr   string
		}{
			{
				name:      "no pipe separator",
				plaintext: "nopipe",
				wantErr:   "invalid token format",
			},
			{
				name:      "empty string",
				plaintext: "",
				wantErr:   "invalid token format",
			},
			{
				name:      "non-numeric id",
				plaintext: "abc|randompart",
				wantErr:   "invalid token id",
			},
			{
				name:      "float id",
				plaintext: "3.14|randompart",
				wantErr:   "invalid token id",
			},
			{
				name:      "negative overflow id",
				plaintext: "99999999999999999999|randompart",
				wantErr:   "invalid token id",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				_, _, err := tokenTestSvc.ParseToken(tt.plaintext)
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error = %q, want it to contain %q", err.Error(), tt.wantErr)
				}
			})
		}
	})
}

// ---------------------------------------------------------------------------
// TestTokenRoundTrip
// ---------------------------------------------------------------------------

func TestTokenRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   int64
	}{
		{name: "id 1", id: 1},
		{name: "id 42", id: 42},
		{name: "large id", id: 1_000_000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			tp, err := tokenTestSvc.GenerateToken()
			if err != nil {
				t.Fatalf("GenerateToken: %v", err)
			}
			originalHash := tp.Hash

			// Act
			tp.SetID(tt.id)
			parsedID, parsedHash, err := tokenTestSvc.ParseToken(tp.Plaintext)

			// Assert
			if err != nil {
				t.Fatalf("ParseToken: %v", err)
			}
			if parsedID != tt.id {
				t.Errorf("parsed ID = %d, want %d", parsedID, tt.id)
			}
			if parsedHash != originalHash {
				t.Errorf("parsed hash = %q, want %q (original hash)", parsedHash, originalHash)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestHashSecret
// ---------------------------------------------------------------------------

func TestHashSecret(t *testing.T) {
	t.Parallel()

	t.Run("produces valid bcrypt hash", func(t *testing.T) {
		t.Parallel()
		hash, err := HashSecret("my-secret")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// bcrypt hashes start with $2a$ or $2b$
		if !strings.HasPrefix(hash, "$2a$") && !strings.HasPrefix(hash, "$2b$") {
			t.Errorf("hash = %q, does not look like bcrypt", hash)
		}
	})

	t.Run("different calls produce different hashes for same input", func(t *testing.T) {
		t.Parallel()
		h1, err := HashSecret("same-secret")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		h2, err := HashSecret("same-secret")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if h1 == h2 {
			t.Error("two bcrypt hashes of the same secret should differ (different salts)")
		}
	})

	t.Run("empty secret is hashed without error", func(t *testing.T) {
		t.Parallel()
		hash, err := HashSecret("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hash == "" {
			t.Error("expected non-empty hash")
		}
	})
}

// ---------------------------------------------------------------------------
// TestVerifySecret
// ---------------------------------------------------------------------------

func TestVerifySecret(t *testing.T) {
	t.Parallel()

	t.Run("correct secret verifies", func(t *testing.T) {
		t.Parallel()
		secret := "correct-horse-battery-staple"
		hash, err := HashSecret(secret)
		if err != nil {
			t.Fatalf("HashSecret: %v", err)
		}
		if !VerifySecret(hash, secret) {
			t.Error("VerifySecret returned false for correct secret")
		}
	})

	t.Run("wrong secret does not verify", func(t *testing.T) {
		t.Parallel()
		hash, err := HashSecret("right-secret")
		if err != nil {
			t.Fatalf("HashSecret: %v", err)
		}
		if VerifySecret(hash, "wrong-secret") {
			t.Error("VerifySecret returned true for wrong secret")
		}
	})

	t.Run("empty secret against non-empty hash fails", func(t *testing.T) {
		t.Parallel()
		hash, err := HashSecret("notempty")
		if err != nil {
			t.Fatalf("HashSecret: %v", err)
		}
		if VerifySecret(hash, "") {
			t.Error("VerifySecret returned true for empty secret")
		}
	})

	t.Run("invalid hash returns false", func(t *testing.T) {
		t.Parallel()
		if VerifySecret("not-a-bcrypt-hash", "anything") {
			t.Error("VerifySecret returned true for invalid hash")
		}
	})
}

// ---------------------------------------------------------------------------
// TestGenerateClientSecret
// ---------------------------------------------------------------------------

func TestGenerateClientSecret(t *testing.T) {
	t.Parallel()

	t.Run("returns plaintext and verifiable hash", func(t *testing.T) {
		t.Parallel()
		plaintext, hashed, err := GenerateClientSecret()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if plaintext == "" {
			t.Error("plaintext is empty")
		}
		if hashed == "" {
			t.Error("hashed is empty")
		}
		if !VerifySecret(hashed, plaintext) {
			t.Error("hashed does not verify against plaintext")
		}
	})

	t.Run("plaintext is 64 characters", func(t *testing.T) {
		t.Parallel()
		plaintext, _, err := GenerateClientSecret()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := len(plaintext); got != 64 {
			t.Errorf("plaintext length = %d, want 64", got)
		}
	})

	t.Run("hashed is a bcrypt hash", func(t *testing.T) {
		t.Parallel()
		_, hashed, err := GenerateClientSecret()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasPrefix(hashed, "$2a$") && !strings.HasPrefix(hashed, "$2b$") {
			t.Errorf("hashed = %q, does not look like bcrypt", hashed)
		}
	})

	t.Run("successive calls produce unique secrets", func(t *testing.T) {
		t.Parallel()
		p1, _, err := GenerateClientSecret()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		p2, _, err := GenerateClientSecret()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p1 == p2 {
			t.Error("two generated secrets have identical plaintext")
		}
	})
}

// ---------------------------------------------------------------------------
// TestVerifyPKCE
// ---------------------------------------------------------------------------

func TestVerifyPKCE(t *testing.T) {
	t.Parallel()

	// Helper: compute the S256 challenge from a verifier.
	s256Challenge := func(verifier string) string {
		h := sha256.Sum256([]byte(verifier))
		return base64.RawURLEncoding.EncodeToString(h[:])
	}

	t.Run("valid verifier matches its challenge", func(t *testing.T) {
		t.Parallel()
		verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
		challenge := s256Challenge(verifier)
		if !VerifyPKCE(challenge, verifier) {
			t.Error("VerifyPKCE returned false for valid pair")
		}
	})

	t.Run("known RFC 7636 Appendix B test vector", func(t *testing.T) {
		t.Parallel()
		// RFC 7636 Appendix B:
		// verifier  = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
		// challenge = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
		verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
		challenge := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
		if !VerifyPKCE(challenge, verifier) {
			t.Error("VerifyPKCE returned false for RFC 7636 test vector")
		}
	})

	t.Run("wrong verifier does not match", func(t *testing.T) {
		t.Parallel()
		verifier := "correct-verifier-string"
		challenge := s256Challenge(verifier)
		if VerifyPKCE(challenge, "wrong-verifier-string") {
			t.Error("VerifyPKCE returned true for wrong verifier")
		}
	})

	t.Run("empty verifier does not match non-empty challenge", func(t *testing.T) {
		t.Parallel()
		challenge := s256Challenge("some-verifier")
		if VerifyPKCE(challenge, "") {
			t.Error("VerifyPKCE returned true for empty verifier")
		}
	})

	t.Run("empty challenge does not match", func(t *testing.T) {
		t.Parallel()
		if VerifyPKCE("", "some-verifier") {
			t.Error("VerifyPKCE returned true for empty challenge")
		}
	})

	t.Run("challenge with standard base64 padding does not match", func(t *testing.T) {
		t.Parallel()
		// S256 uses RawURLEncoding (no padding). A padded variant must not match.
		verifier := "test-verifier-for-padding"
		h := sha256.Sum256([]byte(verifier))
		paddedChallenge := base64.URLEncoding.EncodeToString(h[:]) // with '=' padding
		rawChallenge := base64.RawURLEncoding.EncodeToString(h[:]) // without padding
		if paddedChallenge == rawChallenge {
			t.Skip("no padding difference for this input")
		}
		if VerifyPKCE(paddedChallenge, verifier) {
			t.Error("VerifyPKCE accepted padded base64 challenge")
		}
	})
}

// ---------------------------------------------------------------------------
// TestMatchRedirectURI
// ---------------------------------------------------------------------------

func TestMatchRedirectURI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		requestURI     string
		registeredURIs []string
		want           bool
	}{
		// Exact matches
		{
			name:           "exact match",
			requestURI:     "https://example.com/callback",
			registeredURIs: []string{"https://example.com/callback"},
			want:           true,
		},
		{
			name:           "exact match among multiple registered",
			requestURI:     "https://example.com/cb2",
			registeredURIs: []string{"https://example.com/cb1", "https://example.com/cb2"},
			want:           true,
		},
		{
			name:           "no match",
			requestURI:     "https://example.com/other",
			registeredURIs: []string{"https://example.com/callback"},
			want:           false,
		},

		// Scheme mismatch
		{
			name:           "scheme mismatch http vs https",
			requestURI:     "http://example.com/callback",
			registeredURIs: []string{"https://example.com/callback"},
			want:           false,
		},

		// Path mismatch
		{
			name:           "path mismatch",
			requestURI:     "https://example.com/other",
			registeredURIs: []string{"https://example.com/callback"},
			want:           false,
		},

		// Numeric loopback port flexibility (RFC 8252 §7.3).
		// The literal "localhost" is DNS-hijackable — we only accept
		// numeric loopback (127.0.0.1 / ::1) for the any-port exemption.
		{
			name:           "127.0.0.1 different port allowed",
			requestURI:     "http://127.0.0.1:3000/callback",
			registeredURIs: []string{"http://127.0.0.1:8080/callback"},
			want:           true,
		},
		{
			name:           "IPv6 loopback different port allowed",
			requestURI:     "http://[::1]:3000/callback",
			registeredURIs: []string{"http://[::1]:8080/callback"},
			want:           true,
		},
		{
			name:           "localhost literal rejected (DNS-hijack risk)",
			requestURI:     "http://localhost:9999/callback",
			registeredURIs: []string{"http://localhost:8080/callback"},
			want:           false,
		},

		// Non-loopback different port rejected (exact match required)
		{
			name:           "non-loopback different port rejected",
			requestURI:     "https://example.com:9999/callback",
			registeredURIs: []string{"https://example.com:8080/callback"},
			want:           false,
		},

		// Empty registered URIs
		{
			name:           "empty registered URIs returns false",
			requestURI:     "https://example.com/callback",
			registeredURIs: []string{},
			want:           false,
		},
		{
			name:           "nil registered URIs returns false",
			requestURI:     "https://example.com/callback",
			registeredURIs: nil,
			want:           false,
		},

		// Malformed request URI
		{
			name:           "malformed request URI returns false",
			requestURI:     "://bad-uri",
			registeredURIs: []string{"https://example.com/callback"},
			want:           false,
		},

		// Cross-loopback no longer bridges "localhost" and 127.0.0.1 —
		// the literal is rejected per RFC 8252 §7.3.
		{
			name:           "localhost request does NOT match 127.0.0.1 registered",
			requestURI:     "http://localhost:5000/callback",
			registeredURIs: []string{"http://127.0.0.1:8080/callback"},
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := MatchRedirectURI(tt.requestURI, tt.registeredURIs)
			if got != tt.want {
				t.Errorf("MatchRedirectURI(%q, %v) = %v, want %v",
					tt.requestURI, tt.registeredURIs, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestGenerateUserCode
// ---------------------------------------------------------------------------

func TestGenerateUserCode(t *testing.T) {
	t.Parallel()

	t.Run("format is XXXX-XXXX", func(t *testing.T) {
		t.Parallel()
		code, err := GenerateUserCode()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(code) != 9 {
			t.Errorf("code length = %d, want 9 (XXXX-XXXX)", len(code))
		}
		if code[4] != '-' {
			t.Errorf("code[4] = %q, want '-'", string(code[4]))
		}
	})

	t.Run("uses only base-20 charset characters", func(t *testing.T) {
		t.Parallel()
		// Generate several codes to increase confidence.
		for i := range 50 {
			code, err := GenerateUserCode()
			if err != nil {
				t.Fatalf("unexpected error on iteration %d: %v", i, err)
			}
			stripped := strings.ReplaceAll(code, "-", "")
			for j, c := range stripped {
				if !strings.ContainsRune(deviceCodeCharset, c) {
					t.Errorf("code %q char[%d] = %q, not in charset %q", code, j, string(c), deviceCodeCharset)
				}
			}
		}
	})

	t.Run("contains no vowels", func(t *testing.T) {
		t.Parallel()
		vowels := "AEIOU"
		for i := range 50 {
			code, err := GenerateUserCode()
			if err != nil {
				t.Fatalf("unexpected error on iteration %d: %v", i, err)
			}
			for _, c := range code {
				if strings.ContainsRune(vowels, c) {
					t.Errorf("code %q contains vowel %q", code, string(c))
				}
			}
		}
	})

	t.Run("successive calls produce different codes", func(t *testing.T) {
		t.Parallel()
		// With 20^8 possibilities, collision in two calls is negligible.
		c1, err := GenerateUserCode()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		c2, err := GenerateUserCode()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c1 == c2 {
			t.Error("two generated user codes are identical")
		}
	})
}

// ---------------------------------------------------------------------------
// TestNormalizeUserCode
// ---------------------------------------------------------------------------

func TestNormalizeUserCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "removes dash and uppercases",
			input: "bcdf-ghjk",
			want:  "BCDFGHJK",
		},
		{
			name:  "already normalized",
			input: "BCDFGHJK",
			want:  "BCDFGHJK",
		},
		{
			name:  "lowercase without dash",
			input: "bcdfghjk",
			want:  "BCDFGHJK",
		},
		{
			name:  "multiple dashes removed",
			input: "B-C-D-F",
			want:  "BCDF",
		},
		{
			name:  "mixed case with dash",
			input: "BcDf-GhJk",
			want:  "BCDFGHJK",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := NormalizeUserCode(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeUserCode(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}

	t.Run("round-trip with GenerateUserCode", func(t *testing.T) {
		t.Parallel()
		code, err := GenerateUserCode()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		normalized := NormalizeUserCode(code)
		// After normalization: no dashes, all uppercase, 8 chars
		if len(normalized) != 8 {
			t.Errorf("normalized length = %d, want 8", len(normalized))
		}
		if strings.Contains(normalized, "-") {
			t.Error("normalized code still contains dash")
		}
		if normalized != strings.ToUpper(normalized) {
			t.Error("normalized code is not all uppercase")
		}
	})
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
