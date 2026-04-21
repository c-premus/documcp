package app

import (
	"strings"
	"testing"

	"github.com/c-premus/documcp/internal/config"
)

// cfgWithSalt returns a minimal *config.Config carrying a valid HKDF salt so
// deriveKey does not reject inputs on length grounds.
func cfgWithSalt(previous string) *config.Config {
	cfg := &config.Config{}
	cfg.OAuth.HKDFSalt = "test-hkdf-salt-is-at-least-16-bytes-ok"
	cfg.OAuth.SessionSecretPrevious = previous
	return cfg
}

func TestBuildHMACKeys_PrimaryOnly(t *testing.T) {
	t.Parallel()

	keys, err := buildHMACKeys(cfgWithSalt(""), "some-session-secret-value")
	if err != nil {
		t.Fatalf("buildHMACKeys: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("len(keys) = %d, want 1 when no previous secret configured", len(keys))
	}
	if keys[0].Version == 0 {
		t.Error("primary key Version byte is zero")
	}
	if len(keys[0].Key) == 0 {
		t.Error("primary key bytes are empty")
	}
}

func TestBuildHMACKeys_WithPrevious(t *testing.T) {
	t.Parallel()

	keys, err := buildHMACKeys(cfgWithSalt("prior-secret-value-xyz"), "new-secret-value-abc")
	if err != nil {
		t.Fatalf("buildHMACKeys: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("len(keys) = %d, want 2 when previous secret configured", len(keys))
	}
	if keys[0].Version == keys[1].Version {
		t.Errorf("primary and previous keys share version %q — rotation would fail", keys[0].Version)
	}
}

func TestBuildHMACKeys_CollidingVersionsRejected(t *testing.T) {
	t.Parallel()

	// Two secrets that happen to derive the same version byte. Search for a
	// colliding pair by brute force; bounded to a few thousand tries, which
	// is cheap with a 16-value version space.
	primary := "deterministic-primary"
	target := hmacVersionByte(primary)
	var colliding string
	for i := range 10_000 {
		candidate := "candidate-" + padIndex(i)
		if candidate == primary {
			continue
		}
		if hmacVersionByte(candidate) == target {
			colliding = candidate
			break
		}
	}
	if colliding == "" {
		t.Skip("could not find a colliding version within the search bound")
	}

	_, err := buildHMACKeys(cfgWithSalt(colliding), primary)
	if err == nil {
		t.Fatal("expected error for colliding version bytes")
	}
	if !strings.Contains(err.Error(), "derive to the same HMAC key version") {
		t.Errorf("error = %v, want contains 'derive to the same HMAC key version'", err)
	}
}

func padIndex(i int) string {
	// zero-pad into a short stable string so candidates sort deterministically.
	const zeros = "000000"
	s := intToString(i)
	if len(s) >= len(zeros) {
		return s
	}
	return zeros[:len(zeros)-len(s)] + s
}

func intToString(i int) string {
	if i == 0 {
		return "0"
	}
	digits := ""
	for i > 0 {
		digits = string(rune('0'+i%10)) + digits
		i /= 10
	}
	return digits
}
