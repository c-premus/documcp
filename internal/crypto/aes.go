// Package crypto provides symmetric encryption for secrets stored at rest.
package crypto //nolint:revive // package name matches directory convention; internal package shadowing is acceptable

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ErrInvalidCiphertext is returned when decryption input is malformed.
var ErrInvalidCiphertext = errors.New("invalid ciphertext")

// Key is an AES-256 key with a derived single-hex-char version identifier.
// Encrypt emits a `v<Version>$<base64>` prefix so Decrypt can route ciphertext
// back to the key that produced it — enabling key rotation without a flag day.
type Key struct {
	Version byte
	Key     []byte // 32 bytes for AES-256
}

// Encryptor encrypts with a primary key and decrypts by matching the version
// prefix on the stored ciphertext. Retired keys accepted on decryption keep
// pre-rotation rows readable until a rekey CLI re-encrypts them under the
// new primary.
//
// Legacy ciphertext (produced before versioned prefixes existed) has no prefix;
// Decrypt falls back to trying every configured key in order.
//
// A nil Encryptor is safe to use and acts as a no-op passthrough.
type Encryptor struct {
	keys []Key // primary first; retired keys follow
}

// VersionedCiphertextPrefix is the byte string that precedes a versioned
// ciphertext's base64 payload. Useful for callers that need to recognize
// versioned values without decrypting.
const VersionedCiphertextPrefix = "v"

const ciphertextPrefixSep = "$"

// NewEncryptor creates an Encryptor from a primary 32-byte key and zero or
// more retired 32-byte keys. The primary is used for all new encryption; the
// retired keys are only consulted on decrypt.
//
// Returns nil (and nil error) when primary is empty, disabling encryption.
// Returns an error when any key is the wrong size or when two keys derive to
// the same version byte.
func NewEncryptor(primary []byte, previous ...[]byte) (*Encryptor, error) {
	if len(primary) == 0 {
		return nil, nil
	}
	if len(primary) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes, got %d", len(primary))
	}

	keys := make([]Key, 0, 1+len(previous))
	keys = append(keys, Key{Version: versionByte(primary), Key: primary})

	for i, prev := range previous {
		if len(prev) == 0 {
			continue
		}
		if len(prev) != 32 {
			return nil, fmt.Errorf("previous encryption key %d must be 32 bytes, got %d", i, len(prev))
		}
		v := versionByte(prev)
		for _, existing := range keys {
			if existing.Version == v {
				return nil, fmt.Errorf("previous encryption key %d derives to the same version %q as an earlier key — regenerate one", i, v)
			}
		}
		keys = append(keys, Key{Version: v, Key: prev})
	}

	return &Encryptor{keys: keys}, nil
}

// PrimaryVersion returns the version byte of the key used for new encryption.
// Returns 0 when e is nil.
func (e *Encryptor) PrimaryVersion() byte {
	if e == nil || len(e.keys) == 0 {
		return 0
	}
	return e.keys[0].Version
}

// NeedsRekey returns true when encoded is encrypted but not under the primary
// key — i.e., it was produced by a retired key or has no version prefix at
// all. The rekey CLI uses this to decide whether to re-encrypt a row.
// Empty input and nil Encryptor both return false (nothing to rekey).
func (e *Encryptor) NeedsRekey(encoded string) bool {
	if e == nil || encoded == "" {
		return false
	}
	v, ok := parseVersion(encoded)
	if !ok {
		// Legacy (no prefix) — always needs rekey.
		return true
	}
	return v != e.keys[0].Version
}

// Encrypt encrypts plaintext under the primary key and returns the versioned
// ciphertext `v<version>$<base64>`.
// A nil Encryptor returns the plaintext unchanged.
func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	if e == nil || plaintext == "" {
		return plaintext, nil
	}

	primary := e.keys[0]
	sealed, err := seal(primary.Key, []byte(plaintext))
	if err != nil {
		return "", err
	}
	return VersionedCiphertextPrefix + string(primary.Version) + ciphertextPrefixSep + base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt decodes a ciphertext and returns the plaintext. When the ciphertext
// carries a version prefix, only the matching key is tried; otherwise every
// configured key is tried in order (primary first) to cover legacy values
// written before versioned prefixes existed.
// A nil Encryptor returns the input unchanged.
func (e *Encryptor) Decrypt(encoded string) (string, error) {
	if e == nil || encoded == "" {
		return encoded, nil
	}

	if v, ok := parseVersion(encoded); ok {
		rest := encoded[len(VersionedCiphertextPrefix)+1+len(ciphertextPrefixSep):]
		raw, err := base64.StdEncoding.DecodeString(rest)
		if err != nil {
			return "", fmt.Errorf("decoding ciphertext: %w", err)
		}
		key, ok := e.keyByVersion(v)
		if !ok {
			return "", fmt.Errorf("%w: no configured key for version %q", ErrInvalidCiphertext, v)
		}
		plaintext, err := open(key, raw)
		if err != nil {
			return "", fmt.Errorf("decrypting under version %q: %w", v, err)
		}
		return string(plaintext), nil
	}

	// Legacy: no prefix. Try every key in order.
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decoding ciphertext: %w", err)
	}
	for _, k := range e.keys {
		if plaintext, err := open(k.Key, raw); err == nil {
			return string(plaintext), nil
		}
	}
	return "", fmt.Errorf("%w: no configured key decrypts legacy ciphertext", ErrInvalidCiphertext)
}

// keyByVersion returns the key matching the given version byte.
func (e *Encryptor) keyByVersion(v byte) ([]byte, bool) {
	for _, k := range e.keys {
		if k.Version == v {
			return k.Key, true
		}
	}
	return nil, false
}

// seal performs AES-256-GCM encryption, returning nonce || ct || tag.
func seal(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// open performs AES-256-GCM decryption on nonce || ct || tag.
func open(key, raw []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return nil, ErrInvalidCiphertext
	}
	nonce, ct := raw[:nonceSize], raw[nonceSize:]
	return gcm.Open(nil, nonce, ct, nil)
}

// versionByte returns a stable single hex character derived from SHA-256(key).
// Matches the convention used for HMAC key rotation so the two codepaths share
// a mental model: every distinct key maps to a distinct version byte with
// probability 15/16 per added key.
func versionByte(key []byte) byte {
	sum := sha256.Sum256(key)
	const hexAlphabet = "0123456789abcdef"
	return hexAlphabet[sum[0]>>4]
}

// parseVersion extracts the version byte from a `v<byte>$...` prefix. Returns
// the zero byte and false when the input has no prefix.
func parseVersion(encoded string) (byte, bool) {
	if !strings.HasPrefix(encoded, VersionedCiphertextPrefix) {
		return 0, false
	}
	if len(encoded) < len(VersionedCiphertextPrefix)+1+len(ciphertextPrefixSep) {
		return 0, false
	}
	sepIdx := len(VersionedCiphertextPrefix) + 1
	if encoded[sepIdx:sepIdx+len(ciphertextPrefixSep)] != ciphertextPrefixSep {
		return 0, false
	}
	return encoded[len(VersionedCiphertextPrefix)], true
}
