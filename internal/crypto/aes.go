// Package crypto provides symmetric encryption for secrets stored at rest.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// ErrInvalidCiphertext is returned when decryption input is malformed.
var ErrInvalidCiphertext = errors.New("invalid ciphertext")

// Encryptor encrypts and decrypts strings using AES-256-GCM.
// A nil Encryptor is safe to use and acts as a no-op passthrough.
type Encryptor struct {
	key []byte // 32 bytes for AES-256
}

// NewEncryptor creates an Encryptor with the given 32-byte key.
// Returns nil if key is empty (disabling encryption).
func NewEncryptor(key []byte) (*Encryptor, error) {
	if len(key) == 0 {
		return nil, nil
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes, got %d", len(key))
	}
	return &Encryptor{key: key}, nil
}

// Encrypt encrypts plaintext and returns a base64-encoded ciphertext.
// A nil Encryptor returns the plaintext unchanged.
func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	if e == nil || plaintext == "" {
		return plaintext, nil
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("creating GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decodes a base64-encoded ciphertext and returns the plaintext.
// A nil Encryptor returns the input unchanged.
func (e *Encryptor) Decrypt(encoded string) (string, error) {
	if e == nil || encoded == "" {
		return encoded, nil
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decoding ciphertext: %w", err)
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("creating GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", ErrInvalidCiphertext
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypting: %w", err)
	}

	return string(plaintext), nil
}
