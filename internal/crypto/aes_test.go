package crypto //nolint:revive // package name matches directory convention; internal package shadowing is acceptable

import (
	"crypto/rand"
	"testing"
)

func TestNewEncryptor(t *testing.T) {
	t.Run("nil for empty key", func(t *testing.T) {
		enc, err := NewEncryptor(nil)
		if err != nil {
			t.Fatal(err)
		}
		if enc != nil {
			t.Fatal("expected nil encryptor")
		}
	})

	t.Run("rejects wrong key length", func(t *testing.T) {
		_, err := NewEncryptor([]byte("tooshort"))
		if err == nil {
			t.Fatal("expected error for short key")
		}
	})

	t.Run("accepts 32-byte key", func(t *testing.T) {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			t.Fatal(err)
		}
		enc, err := NewEncryptor(key)
		if err != nil {
			t.Fatal(err)
		}
		if enc == nil {
			t.Fatal("expected non-nil encryptor")
		}
	})
}

func TestEncryptDecrypt(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	enc, err := NewEncryptor(key)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("round trip", func(t *testing.T) {
		plaintext := "super-secret-token-123"
		ciphertext, err := enc.Encrypt(plaintext)
		if err != nil {
			t.Fatal(err)
		}
		if ciphertext == plaintext {
			t.Fatal("ciphertext should differ from plaintext")
		}

		decrypted, err := enc.Decrypt(ciphertext)
		if err != nil {
			t.Fatal(err)
		}
		if decrypted != plaintext {
			t.Fatalf("got %q, want %q", decrypted, plaintext)
		}
	})

	t.Run("empty string passthrough", func(t *testing.T) {
		result, err := enc.Encrypt("")
		if err != nil {
			t.Fatal(err)
		}
		if result != "" {
			t.Fatalf("expected empty string, got %q", result)
		}
	})

	t.Run("different ciphertexts for same plaintext", func(t *testing.T) {
		c1, _ := enc.Encrypt("same")
		c2, _ := enc.Encrypt("same")
		if c1 == c2 {
			t.Fatal("ciphertexts should differ due to random nonce")
		}
	})
}

func TestNilEncryptor(t *testing.T) {
	var enc *Encryptor

	t.Run("encrypt passthrough", func(t *testing.T) {
		result, err := enc.Encrypt("hello")
		if err != nil {
			t.Fatal(err)
		}
		if result != "hello" {
			t.Fatalf("got %q, want %q", result, "hello")
		}
	})

	t.Run("decrypt passthrough", func(t *testing.T) {
		result, err := enc.Decrypt("hello")
		if err != nil {
			t.Fatal(err)
		}
		if result != "hello" {
			t.Fatalf("got %q, want %q", result, "hello")
		}
	})
}

func TestDecryptInvalidInput(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	enc, _ := NewEncryptor(key)

	t.Run("invalid base64", func(t *testing.T) {
		_, err := enc.Decrypt("not-base64!!!")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("too short ciphertext", func(t *testing.T) {
		_, err := enc.Decrypt("AQID") // 3 bytes
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("tampered ciphertext", func(t *testing.T) {
		ciphertext, err := enc.Encrypt("secret")
		if err != nil {
			t.Fatal(err)
		}
		// Flip a character in the middle of the ciphertext.
		tampered := []byte(ciphertext)
		tampered[len(tampered)/2] ^= 0xFF
		_, err = enc.Decrypt(string(tampered))
		if err == nil {
			t.Fatal("expected error for tampered ciphertext")
		}
	})

	t.Run("wrong key cannot decrypt", func(t *testing.T) {
		ciphertext, err := enc.Encrypt("secret")
		if err != nil {
			t.Fatal(err)
		}

		otherKey := make([]byte, 32)
		if _, err = rand.Read(otherKey); err != nil {
			t.Fatal(err)
		}
		otherEnc, err := NewEncryptor(otherKey)
		if err != nil {
			t.Fatal(err)
		}

		_, err = otherEnc.Decrypt(ciphertext)
		if err == nil {
			t.Fatal("expected error when decrypting with wrong key")
		}
	})

	t.Run("empty string decrypt passthrough", func(t *testing.T) {
		result, err := enc.Decrypt("")
		if err != nil {
			t.Fatal(err)
		}
		if result != "" {
			t.Fatalf("expected empty string, got %q", result)
		}
	})
}
