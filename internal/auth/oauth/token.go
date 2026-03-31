// Package oauth implements the OAuth 2.1 authorization server.
package oauth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

// tokenHMACKey holds the server-side key for HMAC-SHA256 token hashing.
// When set (via SetTokenHMACKey), token hashes are keyed, preventing offline
// brute-force attacks if the database is compromised.
// When nil, falls back to plain SHA-256 (still safe for high-entropy tokens).
var (
	tokenHMACKey []byte      //nolint:gochecknoglobals // module-level singleton initialized once at startup
	hmacKeyMu    sync.RWMutex //nolint:gochecknoglobals // guards read/write access to tokenHMACKey
)

// SetTokenHMACKey configures the HMAC key used for token hashing.
// Must be called once at startup before any token operations.
// Safe to call concurrently — subsequent calls after the first non-nil key are ignored.
func SetTokenHMACKey(key []byte) {
	hmacKeyMu.Lock()
	defer hmacKeyMu.Unlock()
	if tokenHMACKey == nil {
		tokenHMACKey = key
	}
}

// resetTokenHMACKey clears the HMAC key for testing. Must not be used in production.
func resetTokenHMACKey() {
	hmacKeyMu.Lock()
	defer hmacKeyMu.Unlock()
	tokenHMACKey = nil
}

// TokenPair holds a plaintext token and its database-storable hash.
// The plaintext is returned to the client exactly once; the hash is persisted.
type TokenPair struct {
	ID        int64
	Plaintext string // "{id}|{64_char_random}" — returned to client
	Hash      string // HMAC-SHA256 (or SHA-256 fallback) hex of the random portion — stored in DB
}

// GenerateToken creates a new token in the format "{id}|{64_char_random}".
// The id is set after the database INSERT (via RETURNING), so initially it is 0.
// Call SetID after persisting to finalize the plaintext.
func GenerateToken() (*TokenPair, error) {
	random, err := randomString(64)
	if err != nil {
		return nil, fmt.Errorf("generating random token: %w", err)
	}
	hash := hashToken(random)
	return &TokenPair{
		Plaintext: random, // id prefix added by SetID
		Hash:      hash,
	}, nil
}

// SetID finalizes the token plaintext with the database-assigned ID.
func (t *TokenPair) SetID(id int64) {
	t.ID = id
	t.Plaintext = fmt.Sprintf("%d|%s", id, t.Plaintext)
}

// ParseToken splits a plaintext token "{id}|{random}" into its components
// and returns the database ID and the keyed hash of the random portion.
func ParseToken(plaintext string) (id int64, hash string, err error) {
	parts := strings.SplitN(plaintext, "|", 2)
	if len(parts) != 2 {
		return 0, "", errors.New("invalid token format")
	}
	id, err = strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("invalid token id: %w", err)
	}
	hash = hashToken(parts[1])
	return id, hash, nil
}

// HashSecret hashes a client secret using bcrypt.
func HashSecret(secret string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hashing secret: %w", err)
	}
	return string(hash), nil
}

// VerifySecret checks a plaintext secret against a bcrypt hash.
func VerifySecret(hash, secret string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(secret)) == nil
}

// GenerateClientSecret creates a random 64-character client secret.
func GenerateClientSecret() (plaintext, hashed string, err error) {
	plaintext, err = randomString(64)
	if err != nil {
		return "", "", fmt.Errorf("generating client secret: %w", err)
	}
	hashed, err = HashSecret(plaintext)
	if err != nil {
		return "", "", err
	}
	return plaintext, hashed, nil
}

// hashToken returns a hex-encoded hash of s.
// Uses HMAC-SHA256 when a key is configured, otherwise plain SHA-256.
func hashToken(s string) string {
	hmacKeyMu.RLock()
	key := tokenHMACKey
	hmacKeyMu.RUnlock()

	if len(key) > 0 {
		mac := hmac.New(sha256.New, key)
		mac.Write([]byte(s))
		return hex.EncodeToString(mac.Sum(nil))
	}
	slog.Warn("token HMAC key not configured, falling back to plain SHA-256")
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// randomString generates a cryptographically random string of the given length
// using the unreserved URI character set with rejection sampling to avoid
// modulo bias.
func randomString(length int) (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	charsetLen := big.NewInt(int64(len(charset)))
	b := make([]byte, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", fmt.Errorf("generating random byte: %w", err)
		}
		b[i] = charset[n.Int64()]
	}
	return string(b), nil
}
