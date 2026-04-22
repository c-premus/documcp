package crypto //nolint:revive // package name matches directory convention; internal package shadowing is acceptable

import (
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func base64EncodeForTest(t *testing.T, b []byte) string {
	t.Helper()
	return base64.StdEncoding.EncodeToString(b)
}

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

// ---------------------------------------------------------------------------
// Key rotation tests
// ---------------------------------------------------------------------------

func mustKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	return key
}

// mustKeyWithDistinctVersion generates a 32-byte key whose derived version
// byte differs from every key passed in. Needed because random 32-byte keys
// collide on the one-hex-char version byte about 1/16 of the time, which
// makes tests that configure two keys flake without retry.
func mustKeyWithDistinctVersion(t *testing.T, others ...[]byte) []byte {
	t.Helper()
	for range 64 {
		candidate := mustKey(t)
		cv := versionByte(candidate)
		collision := false
		for _, other := range others {
			if versionByte(other) == cv {
				collision = true
				break
			}
		}
		if !collision {
			return candidate
		}
	}
	t.Fatal("could not generate key with distinct version byte after 64 tries")
	return nil
}

func TestEncrypt_EmitsVersionedPrefix(t *testing.T) {
	key := mustKey(t)
	enc, err := NewEncryptor(key)
	if err != nil {
		t.Fatal(err)
	}

	ciphertext, err := enc.Encrypt("hello")
	if err != nil {
		t.Fatal(err)
	}

	// Expect `v<byte>$` prefix.
	if len(ciphertext) < 3 || ciphertext[0] != 'v' || ciphertext[2] != '$' {
		t.Fatalf("ciphertext missing versioned prefix: %q", ciphertext)
	}
	if ciphertext[1] != enc.PrimaryVersion() {
		t.Fatalf("ciphertext version byte %q does not match primary %q", ciphertext[1], enc.PrimaryVersion())
	}
}

func TestDecrypt_UsesRetiredKeyForPreviousCiphertext(t *testing.T) {
	primaryKey := mustKey(t)
	previousKey := mustKeyWithDistinctVersion(t, primaryKey)

	// Ciphertext originally produced by the (soon-to-be) retired key.
	oldEnc, err := NewEncryptor(previousKey)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := oldEnc.Encrypt("rotate-me")
	if err != nil {
		t.Fatal(err)
	}

	// Rotate: primary is now the new key, previous keeps the retired one.
	rotated, err := NewEncryptor(primaryKey, previousKey)
	if err != nil {
		t.Fatal(err)
	}

	got, err := rotated.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("decrypt failed after rotation: %v", err)
	}
	if got != "rotate-me" {
		t.Fatalf("got %q, want %q", got, "rotate-me")
	}
}

func TestNewEncryptor_RejectsCollidingVersions(t *testing.T) {
	// Two identical key values collide by construction. The constructor must
	// reject this so the stored prefix identifies exactly one key.
	key := mustKey(t)
	_, err := NewEncryptor(key, key)
	if err == nil {
		t.Fatal("expected error for identical primary and previous keys")
	}
}

func TestDecrypt_LegacyUnprefixedFallback(t *testing.T) {
	// Simulate a legacy ciphertext value: base64 of seal() with no prefix,
	// representing values written before versioned prefixes existed.
	key := mustKey(t)
	sealed, err := seal(key, []byte("legacy-value"))
	if err != nil {
		t.Fatal(err)
	}
	legacy := base64EncodeForTest(t, sealed)

	// An Encryptor built with the same key as its primary must still decrypt
	// the un-prefixed ciphertext.
	enc, err := NewEncryptor(key)
	if err != nil {
		t.Fatal(err)
	}
	got, err := enc.Decrypt(legacy)
	if err != nil {
		t.Fatalf("legacy decrypt failed: %v", err)
	}
	if got != "legacy-value" {
		t.Fatalf("got %q, want %q", got, "legacy-value")
	}
}

func TestDecrypt_LegacyFallsBackThroughPreviousKey(t *testing.T) {
	// Legacy ciphertext was written under what is now the retired key.
	previousKey := mustKey(t)
	primaryKey := mustKeyWithDistinctVersion(t, previousKey)
	sealed, err := seal(previousKey, []byte("legacy-retired"))
	if err != nil {
		t.Fatal(err)
	}
	legacy := base64EncodeForTest(t, sealed)

	rotated, err := NewEncryptor(primaryKey, previousKey)
	if err != nil {
		t.Fatal(err)
	}
	got, err := rotated.Decrypt(legacy)
	if err != nil {
		t.Fatalf("legacy retired decrypt failed: %v", err)
	}
	if got != "legacy-retired" {
		t.Fatalf("got %q, want %q", got, "legacy-retired")
	}
}

func TestNeedsRekey(t *testing.T) {
	primaryKey := mustKey(t)
	previousKey := mustKeyWithDistinctVersion(t, primaryKey)

	rotated, err := NewEncryptor(primaryKey, previousKey)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("empty is no-op", func(t *testing.T) {
		if rotated.NeedsRekey("") {
			t.Fatal("empty string should not need rekey")
		}
	})

	t.Run("nil encryptor is no-op", func(t *testing.T) {
		var e *Encryptor
		if e.NeedsRekey("anything") {
			t.Fatal("nil encryptor should never report needs-rekey")
		}
	})

	t.Run("primary-prefixed is up-to-date", func(t *testing.T) {
		ct, err := rotated.Encrypt("value")
		if err != nil {
			t.Fatal(err)
		}
		if rotated.NeedsRekey(ct) {
			t.Fatal("primary-prefixed ciphertext should not need rekey")
		}
	})

	t.Run("previous-prefixed needs rekey", func(t *testing.T) {
		// Ciphertext originally under the retired key, now carries its prefix.
		oldEnc, err := NewEncryptor(previousKey)
		if err != nil {
			t.Fatal(err)
		}
		ct, err := oldEnc.Encrypt("value")
		if err != nil {
			t.Fatal(err)
		}
		if !rotated.NeedsRekey(ct) {
			t.Fatal("previous-key ciphertext should need rekey")
		}
	})

	t.Run("legacy unprefixed always needs rekey", func(t *testing.T) {
		sealed, err := seal(primaryKey, []byte("legacy"))
		if err != nil {
			t.Fatal(err)
		}
		legacy := base64EncodeForTest(t, sealed)
		if !rotated.NeedsRekey(legacy) {
			t.Fatal("legacy ciphertext should always need rekey")
		}
	})
}

func TestDecrypt_UnknownVersionErrors(t *testing.T) {
	primaryKey := mustKey(t)
	enc, err := NewEncryptor(primaryKey)
	if err != nil {
		t.Fatal(err)
	}

	// Forge a ciphertext claiming a version that isn't configured.
	forged := "vz$" + base64EncodeForTest(t, []byte("doesntmatter"))
	_, err = enc.Decrypt(forged)
	if err == nil {
		t.Fatal("expected error for unknown version byte")
	}
}

