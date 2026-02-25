// Package oauth implements the OAuth 2.1 authorization server.
package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// TokenPair holds a plaintext token and its database-storable hash.
// The plaintext is returned to the client exactly once; the hash is persisted.
type TokenPair struct {
	ID        int64
	Plaintext string // "{id}|{64_char_random}" — returned to client
	Hash      string // SHA-256 hex of the 64-char random portion — stored in DB
}

// GenerateToken creates a new token in the format "{id}|{64_char_random}".
// The id is set after the database INSERT (via RETURNING), so initially it is 0.
// Call SetID after persisting to finalize the plaintext.
func GenerateToken() (*TokenPair, error) {
	random, err := randomString(64)
	if err != nil {
		return nil, fmt.Errorf("generating random token: %w", err)
	}
	hash := hashSHA256(random)
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
// and returns the database ID and the SHA-256 hash of the random portion.
func ParseToken(plaintext string) (id int64, hash string, err error) {
	parts := strings.SplitN(plaintext, "|", 2)
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("invalid token format")
	}
	id, err = strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("invalid token id: %w", err)
	}
	hash = hashSHA256(parts[1])
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
func GenerateClientSecret() (plaintext string, hashed string, err error) {
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

// hashSHA256 returns the hex-encoded SHA-256 hash of s.
func hashSHA256(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// randomString generates a cryptographically random string of the given length
// using the unreserved URI character set.
func randomString(length int) (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b), nil
}
