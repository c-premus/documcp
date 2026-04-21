package oauth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/c-premus/documcp/internal/config"
	"github.com/c-premus/documcp/internal/model"
)

// newHashOnlyService constructs a minimal *Service with a single HMAC key,
// used for exercising hashToken / GenerateToken / ParseToken in isolation.
func newHashOnlyService(t *testing.T, key []byte) *Service {
	t.Helper()
	if len(key) == 0 {
		t.Fatalf("newHashOnlyService requires a non-empty key — the SHA-256 fallback was removed (security L4)")
	}
	svc, err := NewService(nil, config.OAuthConfig{}, "https://app.example.com",
		slog.Default(), []HMACKey{{Version: '1', Key: key}})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc
}

// newRotationService constructs a *Service with two keys: primary at version
// '1' and a retired key at version '2'. Used to exercise verify paths during
// rotation.
func newRotationService(t *testing.T, primary, retired []byte) *Service {
	t.Helper()
	svc, err := NewService(nil, config.OAuthConfig{}, "https://app.example.com",
		slog.Default(), []HMACKey{
			{Version: '1', Key: primary},
			{Version: '2', Key: retired},
		})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc
}

// tokenTestSvc is a shared service for token round-trip tests that don't
// care about key rotation. Its methods only read hmacKeys after construction,
// so concurrent use from t.Parallel tests is safe.
var tokenTestSvc = mustTokenTestSvc()

func mustTokenTestSvc() *Service {
	svc, err := NewService(nil, config.OAuthConfig{}, "https://app.example.com",
		slog.Default(), []HMACKey{{Version: '1', Key: []byte("shared-token-test-key")}})
	if err != nil {
		panic(err)
	}
	return svc
}

// ---------------------------------------------------------------------------
// HMAC construction and version prefix
// ---------------------------------------------------------------------------

func TestNewService_RejectsEmptyKeys(t *testing.T) {
	t.Parallel()

	if _, err := NewService(nil, config.OAuthConfig{}, "http://x", slog.Default(), nil); err == nil {
		t.Fatal("expected error for nil hmacKeys (security L4)")
	}
	if _, err := NewService(nil, config.OAuthConfig{}, "http://x", slog.Default(), []HMACKey{}); err == nil {
		t.Fatal("expected error for empty hmacKeys (security L4)")
	}
	if _, err := NewService(nil, config.OAuthConfig{}, "http://x", slog.Default(),
		[]HMACKey{{Version: '1', Key: nil}}); err == nil {
		t.Fatal("expected error for empty key bytes")
	}
}

func TestService_HashToken_VersionPrefix(t *testing.T) {
	t.Parallel()

	const input = "test-token-value"
	key := []byte("my-secret-key")
	svc := newHashOnlyService(t, key)
	got := svc.hashToken(input)

	if !strings.HasPrefix(got, "v1$") {
		t.Errorf("hashToken result %q missing v1$ prefix (security M2)", got)
	}

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(input))
	want := "v1$" + hex.EncodeToString(mac.Sum(nil))
	if got != want {
		t.Errorf("hashToken: got %s, want %s", got[:min(24, len(got))], want[:min(24, len(want))])
	}
}

func TestService_HashToken_KeyIsolation(t *testing.T) {
	t.Parallel()

	const input = "test-token-value"
	svc1 := newHashOnlyService(t, []byte("first-key"))
	svc2 := newHashOnlyService(t, []byte("second-key"))

	if svc1.hashToken(input) == svc2.hashToken(input) {
		t.Error("different HMAC keys produced the same hash")
	}
}

func TestService_ParseTokenCandidateHashes_CoversEveryKey(t *testing.T) {
	t.Parallel()

	svc := newRotationService(t,
		[]byte("primary-key-current"),
		[]byte("retired-key-previous"),
	)

	id, hashes, err := svc.parseTokenCandidateHashes("42|" + strings.Repeat("a", 64))
	if err != nil {
		t.Fatalf("parseTokenCandidateHashes: %v", err)
	}
	if id != 42 {
		t.Errorf("id = %d, want 42", id)
	}
	if len(hashes) != 2 {
		t.Fatalf("len(hashes) = %d, want 2 (one per configured key)", len(hashes))
	}
	if !strings.HasPrefix(hashes[0], "v1$") {
		t.Errorf("hashes[0] should carry v1$ prefix, got %q", hashes[0][:min(24, len(hashes[0]))])
	}
	if !strings.HasPrefix(hashes[1], "v2$") {
		t.Errorf("hashes[1] should carry v2$ prefix, got %q", hashes[1][:min(24, len(hashes[1]))])
	}
	if hashes[0] == hashes[1] {
		t.Error("candidate hashes must differ between key versions")
	}
}

// TestService_ValidateAccessToken_AcceptsRetiredKey proves that a token
// hashed under a key that was primary at write-time still authenticates
// after rotation — the verify path tries every configured key by its stable
// per-key version identifier. This is the load-bearing behavior that the
// audit M2 closure enables: rotating OAUTH_SESSION_SECRET no longer silently
// invalidates live tokens.
//
// Each HMAC key carries a stable Version byte that identifies the key, not
// its role. When a key moves from primary to retired, its Version stays the
// same, so the stored prefix still points at the correct key.
func TestService_ValidateAccessToken_AcceptsRetiredKey(t *testing.T) {
	t.Parallel()

	oldKey := []byte("key-before-rotation")
	newKey := []byte("key-after-rotation")
	const oldVersion byte = 'a' // stable identifier for oldKey
	const newVersion byte = 'b' // stable identifier for newKey

	// Step 1: under the old regime, a single-key service mints a token.
	oldSvc, err := NewService(nil, config.OAuthConfig{}, "https://app.example.com",
		slog.Default(), []HMACKey{{Version: oldVersion, Key: oldKey}})
	if err != nil {
		t.Fatalf("NewService(old): %v", err)
	}
	tp, err := oldSvc.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	const tokenID = int64(7)
	tp.SetID(tokenID)
	storedHash := tp.Hash // what the DB holds; prefixed `va$`

	// Step 2: operator rotates — new primary is `newKey` under its stable
	// version 'b'. The retired `oldKey` keeps its stable version 'a'. Repo
	// behaves like the real DB: returns the row only when the candidate
	// hash matches.
	repo := &mockOAuthRepo{
		FindAccessTokenByTokenFunc: func(_ context.Context, tokenHash string) (*model.OAuthAccessToken, error) {
			if tokenHash != storedHash {
				return nil, sql.ErrNoRows
			}
			return &model.OAuthAccessToken{
				ID:        tokenID,
				Token:     storedHash,
				ClientID:  1,
				ExpiresAt: time.Now().Add(time.Hour),
			}, nil
		},
	}
	newSvc, err := NewService(repo, config.OAuthConfig{}, "https://app.example.com",
		slog.Default(), []HMACKey{
			{Version: newVersion, Key: newKey},
			{Version: oldVersion, Key: oldKey},
		})
	if err != nil {
		t.Fatalf("NewService(rotated): %v", err)
	}

	got, err := newSvc.ValidateAccessToken(context.Background(), tp.Plaintext)
	if err != nil {
		t.Fatalf("ValidateAccessToken after rotation: %v", err)
	}
	if got.ID != tokenID {
		t.Errorf("got.ID = %d, want %d", got.ID, tokenID)
	}

	// Fresh tokens minted after rotation use the new primary, prefix 'vb$'.
	fresh, err := newSvc.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken post-rotation: %v", err)
	}
	if !strings.HasPrefix(fresh.Hash, "vb$") {
		t.Errorf("post-rotation fresh hash %q should start with vb$ (new primary version)", fresh.Hash)
	}
}

// TestService_ValidateAccessToken_RejectsUnknownKey proves tokens hashed
// under a key that was never configured on this server are rejected —
// candidate lookup must not become a confused-deputy accept-anything.
func TestService_ValidateAccessToken_RejectsUnknownKey(t *testing.T) {
	t.Parallel()

	strangerSvc, err := NewService(nil, config.OAuthConfig{}, "https://app.example.com",
		slog.Default(), []HMACKey{{Version: '1', Key: []byte("some-other-key")}})
	if err != nil {
		t.Fatalf("NewService(stranger): %v", err)
	}
	tp, err := strangerSvc.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	tp.SetID(9)

	repo := &mockOAuthRepo{
		FindAccessTokenByTokenFunc: func(_ context.Context, _ string) (*model.OAuthAccessToken, error) {
			return nil, sql.ErrNoRows
		},
	}
	localSvc, err := NewService(repo, config.OAuthConfig{}, "https://app.example.com",
		slog.Default(), []HMACKey{{Version: '1', Key: []byte("our-real-key")}})
	if err != nil {
		t.Fatalf("NewService(local): %v", err)
	}

	if _, err := localSvc.ValidateAccessToken(context.Background(), tp.Plaintext); err == nil {
		t.Fatal("expected error for token hashed under unknown key")
	}
}
