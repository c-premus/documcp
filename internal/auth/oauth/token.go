// Package oauth implements the OAuth 2.1 authorization server.
package oauth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// HMACKey binds an HMAC signing key to a single-byte version identifier.
// Stored token hashes are prefixed `v<Version>$<hex>` so rotation can happen
// without silently invalidating every active token hash (security M2). Version
// is a printable ASCII byte chosen by operators; the ServerApp wires '1' for
// the key derived from OAUTH_SESSION_SECRET and '2' for OAUTH_SESSION_SECRET_PREVIOUS.
type HMACKey struct {
	Version byte
	Key     []byte
}

// hashPrefix returns the `v<byte>$` prefix used on stored hashes.
func (k HMACKey) hashPrefix() string {
	return "v" + string(k.Version) + "$"
}

// TokenPair holds a plaintext token and its database-storable hash.
// The plaintext is returned to the client exactly once; the hash is persisted.
type TokenPair struct {
	ID        int64
	Plaintext string // "{id}|{64_char_random}" — returned to client
	Hash      string // "v<key-version>$<hex>" HMAC-SHA256 of the random portion — stored in DB
}

// SetID finalizes the token plaintext with the database-assigned ID.
func (t *TokenPair) SetID(id int64) {
	t.ID = id
	t.Plaintext = fmt.Sprintf("%d|%s", id, t.Plaintext)
}

// GenerateToken creates a new token in the format "{id}|{64_char_random}".
// The id is set after the database INSERT (via RETURNING), so initially it is 0.
// Call SetID after persisting to finalize the plaintext.
func (s *Service) GenerateToken() (*TokenPair, error) {
	random, err := randomString(64)
	if err != nil {
		return nil, fmt.Errorf("generating random token: %w", err)
	}
	hash := s.hashToken(random)
	return &TokenPair{
		Plaintext: random, // id prefix added by SetID
		Hash:      hash,
	}, nil
}

// ParseToken splits a plaintext token "{id}|{random}" into its components
// and returns the database ID and the keyed hash computed with the CURRENT
// (primary) HMAC key. Use parseTokenCandidateHashes on verify paths that
// need to accept hashes minted under a previously-current key.
func (s *Service) ParseToken(plaintext string) (id int64, hash string, err error) {
	id, _, err = s.parseTokenID(plaintext)
	if err != nil {
		return 0, "", err
	}
	random, randomErr := s.parseTokenRandom(plaintext)
	if randomErr != nil {
		return 0, "", randomErr
	}
	return id, s.hashToken(random), nil
}

// parseTokenID extracts the leading int64 id and the random remainder without
// computing any hash.
func (s *Service) parseTokenID(plaintext string) (id int64, random string, err error) {
	parts := strings.SplitN(plaintext, "|", 2)
	if len(parts) != 2 {
		return 0, "", errors.New("invalid token format")
	}
	id, err = strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("invalid token id: %w", err)
	}
	return id, parts[1], nil
}

// parseTokenRandom returns only the random portion after the "|" separator.
func (s *Service) parseTokenRandom(plaintext string) (string, error) {
	_, random, err := s.parseTokenID(plaintext)
	return random, err
}

// parseTokenCandidateHashes returns the ID plus one candidate hash per
// configured HMAC key — hashes[0] is the primary-key hash, subsequent entries
// cover retired keys. Verify paths try them in order so a token minted under
// the previous key still authenticates during a rotation window.
func (s *Service) parseTokenCandidateHashes(plaintext string) (id int64, hashes []string, err error) {
	id, random, err := s.parseTokenID(plaintext)
	if err != nil {
		return 0, nil, err
	}
	hashes = make([]string, 0, len(s.hmacKeys))
	for i := range s.hmacKeys {
		hashes = append(hashes, s.hashWithKey(random, s.hmacKeys[i]))
	}
	return id, hashes, nil
}

// parseTokenOrZero attempts to parse a token string into its ID and hash.
// Returns false if the token is malformed.
func (s *Service) parseTokenOrZero(plaintext string) (id int64, hash string, ok bool) {
	id, hash, err := s.ParseToken(plaintext)
	return id, hash, err == nil
}

// hashToken returns the hex-encoded HMAC-SHA256 of plaintext under the
// current primary key, prefixed with `v<version>$` so the stored hash records
// which key version produced it.
//
// Service construction rejects an empty hmacKeys slice so this method always
// has a key to use — no silent SHA-256 fallback (security L4).
func (s *Service) hashToken(plaintext string) string {
	return s.hashWithKey(plaintext, s.hmacKeys[0])
}

// hashWithKey computes the versioned HMAC hash for an explicit key. Used on
// verify paths to try multiple keys during rotation.
func (s *Service) hashWithKey(plaintext string, k HMACKey) string {
	mac := hmac.New(sha256.New, k.Key)
	mac.Write([]byte(plaintext))
	return k.hashPrefix() + hex.EncodeToString(mac.Sum(nil))
}

// HashSecret hashes a client secret using bcrypt.
// Bcrypt silently truncates input at 72 bytes — reject longer secrets to
// prevent two distinct secrets from hashing identically.
func HashSecret(secret string) (string, error) {
	if len(secret) > 72 {
		return "", fmt.Errorf("secret length %d exceeds bcrypt maximum of 72 bytes", len(secret))
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hashing secret: %w", err)
	}
	return string(hash), nil
}

// VerifySecret checks a plaintext secret against a bcrypt hash.
// Returns false for secrets exceeding bcrypt's 72-byte limit.
func VerifySecret(hash, secret string) bool {
	if len(secret) > 72 {
		return false
	}
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
