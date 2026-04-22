package oidc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"strings"
	"testing"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4"
	josejwt "github.com/go-jose/go-jose/v4/jwt"
	"github.com/gorilla/sessions"

	"github.com/c-premus/documcp/internal/config"
	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/security"
)

func init() {
	// httptest servers bind to 127.0.0.1, which SafeTransport correctly rejects
	// as loopback at both validation and dial time. Disable both guards for the
	// test binary. Tests that need to exercise the real validator (e.g.
	// TestNew_RejectsUnsafeProviderURL) save and restore these vars themselves.
	validateOIDCURL = func(string) error { return nil }
	oidcBaseTransport = func() http.RoundTripper { return http.DefaultTransport }
}

// mockUserRepo implements UserRepo for testing.
type mockUserRepo struct {
	findBySubFn func(ctx context.Context, sub string) (*model.User, error)
	createFn    func(ctx context.Context, user *model.User) error
	updateFn    func(ctx context.Context, user *model.User) error
}

func (m *mockUserRepo) FindUserByOIDCSub(ctx context.Context, sub string) (*model.User, error) {
	if m.findBySubFn != nil {
		return m.findBySubFn(ctx, sub)
	}
	return nil, sql.ErrNoRows
}

func (m *mockUserRepo) CreateUser(ctx context.Context, user *model.User) error {
	if m.createFn != nil {
		return m.createFn(ctx, user)
	}
	return nil
}

func (m *mockUserRepo) UpdateUser(ctx context.Context, user *model.User) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, user)
	}
	return nil
}

// setupMockOIDCProvider creates a test OIDC provider that serves discovery, JWKS, and token endpoints.
// The returned nonce pointer should be set to the expected nonce before calling the token endpoint;
// the mock will include it in the signed ID token claims.
func setupMockOIDCProvider(t *testing.T) (*httptest.Server, *rsa.PrivateKey, *string) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}

	mux := http.NewServeMux()

	// We need a reference to the server URL before creating it (for issuer), so use a pointer.
	var serverURL string
	// Nonce shared between test and mock token endpoint — set before calling callback.
	var expectedNonce string

	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		discovery := map[string]any{
			"issuer":                                serverURL,
			"authorization_endpoint":                serverURL + "/authorize",
			"token_endpoint":                        serverURL + "/token",
			"jwks_uri":                              serverURL + "/certs",
			"end_session_endpoint":                  serverURL + "/end-session",
			"id_token_signing_alg_values_supported": []string{"RS256"},
			"subject_types_supported":               []string{"public"},
			"response_types_supported":              []string{"code"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(discovery)
	})

	mux.HandleFunc("GET /certs", func(w http.ResponseWriter, r *http.Request) {
		jwk := jose.JSONWebKey{
			Key:       &privateKey.PublicKey,
			KeyID:     "test-key",
			Algorithm: string(jose.RS256),
			Use:       "sig",
		}
		jwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{jwk}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	})

	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		// Generate a signed ID token.
		signer, err := jose.NewSigner(
			jose.SigningKey{Algorithm: jose.RS256, Key: privateKey},
			(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "test-key"),
		)
		if err != nil {
			http.Error(w, "signer error", http.StatusInternalServerError)
			return
		}

		now := time.Now()
		claims := josejwt.Claims{
			Issuer:    serverURL,
			Subject:   "test-sub-123",
			Audience:  josejwt.Audience{"test-client-id"},
			IssuedAt:  josejwt.NewNumericDate(now),
			Expiry:    josejwt.NewNumericDate(now.Add(1 * time.Hour)),
			NotBefore: josejwt.NewNumericDate(now.Add(-1 * time.Minute)),
		}
		extraClaims := map[string]any{
			"email": "test@example.com",
			"name":  "Test User",
			"nonce": expectedNonce,
		}

		rawToken, err := josejwt.Signed(signer).Claims(claims).Claims(extraClaims).Serialize()
		if err != nil {
			http.Error(w, "token error", http.StatusInternalServerError)
			return
		}

		tokenResponse := map[string]any{
			"access_token": "test-access-token",
			"token_type":   "Bearer",
			"id_token":     rawToken,
			"expires_in":   3600,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tokenResponse)
	})

	server := httptest.NewServer(mux)
	serverURL = server.URL
	return server, privateKey, &expectedNonce
}

// newTestHandler creates a Handler backed by the mock OIDC provider.
// Returns a nonce pointer — set *nonce before calling Callback so the mock
// token endpoint includes it in the signed ID token.
func newTestHandler(t *testing.T, repo UserRepo) (*Handler, *httptest.Server, *string) {
	t.Helper()

	mockServer, _, noncePtr := setupMockOIDCProvider(t)

	// Use go-oidc insecure issuer context for test servers (HTTP).
	ctx := gooidc.InsecureIssuerURLContext(context.Background(), mockServer.URL)

	store := sessions.NewCookieStore([]byte("test-secret-key-32-bytes-long!!"))
	store.Options = &sessions.Options{
		Path:     "/",
		HttpOnly: true,
		MaxAge:   86400,
	}

	h, err := New(ctx, Config{
		OIDCCfg: config.OIDCConfig{
			ProviderURL:  mockServer.URL,
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			RedirectURL:  "http://localhost:8080/auth/callback",
		},
		SessionStore: store,
		Repo:         repo,
		Logger:       slog.Default(),
		AppURL:       "http://localhost:8080",
	})
	if err != nil {
		t.Fatalf("creating OIDC handler: %v", err)
	}
	if h == nil {
		t.Fatal("OIDC handler is nil")
	}
	return h, mockServer, noncePtr
}

// doLogin calls h.Login and returns state, nonce, and cookies. It also sets
// *noncePtr so the mock token endpoint includes the nonce in the ID token.
func doLogin(t *testing.T, h *Handler, noncePtr *string) (state string, cookies []*http.Cookie) {
	t.Helper()
	loginReq := httptest.NewRequest(http.MethodGet, "/auth/login", http.NoBody)
	loginW := httptest.NewRecorder()
	h.Login(loginW, loginReq)

	location := loginW.Header().Get("Location")
	parsed, err := parseRedirectURL(location)
	if err != nil {
		t.Fatalf("parsing login redirect URL: %v", err)
	}
	state = parsed.Query().Get("state")
	if state == "" {
		t.Fatal("expected state in login redirect URL")
	}
	nonce := parsed.Query().Get("nonce")
	if nonce == "" {
		t.Fatal("expected nonce in login redirect URL")
	}
	*noncePtr = nonce
	return state, loginW.Result().Cookies()
}

func TestNew(t *testing.T) {
	t.Run("returns nil when not configured", func(t *testing.T) {
		h, err := New(context.Background(), Config{
			OIDCCfg: config.OIDCConfig{},
			Logger:  slog.Default(),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if h != nil {
			t.Fatal("expected nil handler when OIDC not configured")
		}
	})

	t.Run("returns nil when client ID missing", func(t *testing.T) {
		h, err := New(context.Background(), Config{
			OIDCCfg: config.OIDCConfig{
				ProviderURL: "https://example.com",
			},
			Logger: slog.Default(),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if h != nil {
			t.Fatal("expected nil handler when client ID missing")
		}
	})

	t.Run("returns error for invalid provider URL", func(t *testing.T) {
		origRetries := discoveryMaxRetries
		origDelay := discoveryBaseDelay
		discoveryMaxRetries = 1
		discoveryBaseDelay = 1 * time.Millisecond
		defer func() {
			discoveryMaxRetries = origRetries
			discoveryBaseDelay = origDelay
		}()

		_, err := New(context.Background(), Config{
			OIDCCfg: config.OIDCConfig{
				ProviderURL: "http://invalid.localhost.test:1",
				ClientID:    "test",
			},
			Logger: slog.Default(),
		})
		if err == nil {
			t.Fatal("expected error for invalid provider URL")
		}
	})

	t.Run("creates handler with valid config", func(t *testing.T) {
		h, server, _ := newTestHandler(t, &mockUserRepo{})
		defer server.Close()
		if h == nil {
			t.Fatal("expected non-nil handler")
		}
	})

	t.Run("retries discovery on transient failure then succeeds", func(t *testing.T) {
		attempts := 0
		var serverURL string
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts <= 2 {
				// Simulate transient failure for first 2 attempts.
				http.Error(w, "service unavailable", http.StatusServiceUnavailable)
				return
			}
			// Third attempt succeeds with valid discovery document.
			discovery := map[string]any{
				"issuer":                                serverURL,
				"authorization_endpoint":                serverURL + "/authorize",
				"token_endpoint":                        serverURL + "/token",
				"jwks_uri":                              serverURL + "/certs",
				"id_token_signing_alg_values_supported": []string{"RS256"},
				"subject_types_supported":               []string{"public"},
				"response_types_supported":              []string{"code"},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(discovery)
		}))
		serverURL = mockServer.URL
		defer mockServer.Close()

		// Override constants for fast test — use package-level vars instead.
		origRetries := discoveryMaxRetries
		origDelay := discoveryBaseDelay
		discoveryMaxRetries = 4
		discoveryBaseDelay = 1 * time.Millisecond
		defer func() {
			discoveryMaxRetries = origRetries
			discoveryBaseDelay = origDelay
		}()

		ctx := gooidc.InsecureIssuerURLContext(context.Background(), mockServer.URL)
		h, err := New(ctx, Config{
			OIDCCfg: config.OIDCConfig{
				ProviderURL:  mockServer.URL,
				ClientID:     "test-client-id",
				ClientSecret: "test-secret",
				RedirectURL:  "http://localhost/auth/callback",
			},
			SessionStore: sessions.NewCookieStore([]byte("test-secret-key-32-bytes-long!!")),
			Repo:         &mockUserRepo{},
			Logger:       slog.Default(),
		})
		if err != nil {
			t.Fatalf("expected success after retry, got: %v", err)
		}
		if h == nil {
			t.Fatal("expected non-nil handler after retry")
		}
		if attempts != 3 {
			t.Errorf("expected 3 discovery attempts, got %d", attempts)
		}
	})

	t.Run("returns error after all retries exhausted", func(t *testing.T) {
		origRetries := discoveryMaxRetries
		origDelay := discoveryBaseDelay
		discoveryMaxRetries = 2
		discoveryBaseDelay = 1 * time.Millisecond
		defer func() {
			discoveryMaxRetries = origRetries
			discoveryBaseDelay = origDelay
		}()

		_, err := New(context.Background(), Config{
			OIDCCfg: config.OIDCConfig{
				ProviderURL: "http://invalid.localhost.test:1",
				ClientID:    "test",
			},
			Logger: slog.Default(),
		})
		if err == nil {
			t.Fatal("expected error after all retries exhausted")
		}
		if !strings.Contains(err.Error(), "after 2 attempts") {
			t.Errorf("error should mention attempt count, got: %v", err)
		}
	})

	t.Run("respects context cancellation during retry", func(t *testing.T) {
		origRetries := discoveryMaxRetries
		origDelay := discoveryBaseDelay
		discoveryMaxRetries = 10
		discoveryBaseDelay = 10 * time.Second // long delay — context should cancel first
		defer func() {
			discoveryMaxRetries = origRetries
			discoveryBaseDelay = origDelay
		}()

		ctx, cancel := context.WithCancel(context.Background())
		// Cancel after a short delay so the first retry's backoff sleep is interrupted.
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		_, err := New(ctx, Config{
			OIDCCfg: config.OIDCConfig{
				ProviderURL: "http://invalid.localhost.test:1",
				ClientID:    "test",
			},
			Logger: slog.Default(),
		})
		if err == nil {
			t.Fatal("expected error on context cancellation")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got: %v", err)
		}
	})
}

func TestIsSafeRedirect(t *testing.T) {
	tests := []struct {
		redirect string
		safe     bool
	}{
		{"/admin", true},
		{"/admin/documents", true},
		{"/app/dashboard", true},
		{"//evil.com", false},
		{"\\\\evil.com", false},
		{"/path//double", false},
		{"https://evil.com", false},
		{"", false},
		{"javascript:alert(1)", false},
		{"relative/path", false},
		{"/valid/path\\evil", false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("redirect=%q", tt.redirect), func(t *testing.T) {
			got := isSafeRedirect(tt.redirect)
			if got != tt.safe {
				t.Errorf("isSafeRedirect(%q) = %v, want %v", tt.redirect, got, tt.safe)
			}
		})
	}
}

func TestLogin(t *testing.T) {
	repo := &mockUserRepo{}
	h, server, _ := newTestHandler(t, repo)
	defer server.Close()

	t.Run("redirects to OIDC provider", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/login", http.NoBody)
		w := httptest.NewRecorder()

		h.Login(w, req)

		if w.Code != http.StatusFound {
			t.Fatalf("expected 302, got %d", w.Code)
		}
		location := w.Header().Get("Location")
		if location == "" {
			t.Fatal("expected Location header")
		}
		// Should redirect to the mock server's authorize endpoint.
		if got := location; got == "" {
			t.Fatal("expected non-empty redirect URL")
		}
	})

	t.Run("preserves safe redirect param", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/login?redirect=/admin/documents", http.NoBody)
		w := httptest.NewRecorder()

		h.Login(w, req)

		if w.Code != http.StatusFound {
			t.Fatalf("expected 302, got %d", w.Code)
		}

		// Check session has the redirect stored.
		cookies := w.Result().Cookies()
		if len(cookies) == 0 {
			t.Fatal("expected session cookie to be set")
		}
	})

	t.Run("ignores unsafe redirect with double slash", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/login?redirect=//evil.com", http.NoBody)
		w := httptest.NewRecorder()

		h.Login(w, req)

		if w.Code != http.StatusFound {
			t.Fatalf("expected 302, got %d", w.Code)
		}
	})

	t.Run("ignores unsafe redirect with backslash", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/login?redirect=\\\\evil.com", http.NoBody)
		w := httptest.NewRecorder()

		h.Login(w, req)

		if w.Code != http.StatusFound {
			t.Fatalf("expected 302, got %d", w.Code)
		}
	})
}

func TestCallback(t *testing.T) {
	t.Run("rejects missing state", func(t *testing.T) {
		repo := &mockUserRepo{}
		h, server, _ := newTestHandler(t, repo)
		defer server.Close()

		req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=VALID", http.NoBody)
		w := httptest.NewRecorder()

		h.Callback(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("rejects invalid state", func(t *testing.T) {
		repo := &mockUserRepo{}
		h, server, noncePtr := newTestHandler(t, repo)
		defer server.Close()

		state, cookies := doLogin(t, h, noncePtr)
		_ = state // intentionally using wrong state
		req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=VALID&state=WRONG", http.NoBody)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		w := httptest.NewRecorder()

		h.Callback(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("exchanges code and creates session for new user", func(t *testing.T) {
		var createdUser *model.User
		repo := &mockUserRepo{
			createFn: func(ctx context.Context, user *model.User) error {
				user.ID = 42
				createdUser = user
				return nil
			},
		}
		h, server, noncePtr := newTestHandler(t, repo)
		defer server.Close()

		state, cookies := doLogin(t, h, noncePtr)
		callbackURL := "/auth/callback?code=test-auth-code&state=" + state
		req := httptest.NewRequest(http.MethodGet, callbackURL, http.NoBody)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		w := httptest.NewRecorder()

		h.Callback(w, req)

		// Should redirect to /admin on success.
		if w.Code != http.StatusFound {
			t.Fatalf("expected 302, got %d; body: %s", w.Code, w.Body.String())
		}
		redirectTo := w.Header().Get("Location")
		if redirectTo != "/admin" {
			t.Errorf("expected redirect to /admin, got %q", redirectTo)
		}

		// Verify user was created.
		if createdUser == nil {
			t.Fatal("expected CreateUser to be called")
		}
		if createdUser.Email != "test@example.com" {
			t.Errorf("expected email test@example.com, got %q", createdUser.Email)
		}
		if createdUser.OIDCSub.String != "test-sub-123" {
			t.Errorf("expected OIDC sub test-sub-123, got %q", createdUser.OIDCSub.String)
		}
		if createdUser.IsAdmin {
			t.Error("expected new user to not be admin")
		}
	})

	t.Run("finds existing user by sub", func(t *testing.T) {
		existingUser := &model.User{
			ID:    1,
			Name:  "Test User",
			Email: "test@example.com",
			OIDCSub: sql.NullString{
				String: "test-sub-123",
				Valid:  true,
			},
			IsAdmin: true,
		}
		repo := &mockUserRepo{
			findBySubFn: func(ctx context.Context, sub string) (*model.User, error) {
				if sub == "test-sub-123" {
					return existingUser, nil
				}
				return nil, sql.ErrNoRows
			},
		}
		h, server, noncePtr := newTestHandler(t, repo)
		defer server.Close()

		state, cookies := doLogin(t, h, noncePtr)
		callbackURL := "/auth/callback?code=test-auth-code&state=" + state
		req := httptest.NewRequest(http.MethodGet, callbackURL, http.NoBody)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		w := httptest.NewRecorder()

		h.Callback(w, req)

		if w.Code != http.StatusFound {
			t.Fatalf("expected 302, got %d; body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("unknown sub creates fresh user even when email matches existing", func(t *testing.T) {
		// v0.21.0 removed the FindUserByEmail fallback. A pre-existing local
		// record with the same email as the OIDC user MUST NOT be linked —
		// that was the admin-takeover vector. The callback path goes straight
		// to CreateUser.
		var updatedCalls int
		var created *model.User
		repo := &mockUserRepo{
			updateFn: func(_ context.Context, _ *model.User) error {
				updatedCalls++
				return nil
			},
			createFn: func(_ context.Context, user *model.User) error {
				user.ID = 42
				created = user
				return nil
			},
		}
		h, server, noncePtr := newTestHandler(t, repo)
		defer server.Close()

		state, cookies := doLogin(t, h, noncePtr)
		callbackURL := "/auth/callback?code=test-auth-code&state=" + state
		req := httptest.NewRequest(http.MethodGet, callbackURL, http.NoBody)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		w := httptest.NewRecorder()

		h.Callback(w, req)

		if w.Code != http.StatusFound {
			t.Fatalf("expected 302, got %d; body: %s", w.Code, w.Body.String())
		}
		if updatedCalls != 0 {
			t.Errorf("expected no UpdateUser call (email linking removed), got %d", updatedCalls)
		}
		if created == nil {
			t.Fatal("expected CreateUser to be called for unknown sub")
		}
		if !created.OIDCSub.Valid || created.OIDCSub.String != "test-sub-123" {
			t.Errorf("expected new user's sub to be test-sub-123, got %v", created.OIDCSub)
		}
	})

	t.Run("follows safe redirect from login", func(t *testing.T) {
		repo := &mockUserRepo{
			createFn: func(ctx context.Context, user *model.User) error {
				user.ID = 99
				return nil
			},
		}
		h, server, noncePtr := newTestHandler(t, repo)
		defer server.Close()

		// Login with redirect param — cannot use doLogin because of custom query.
		loginReq := httptest.NewRequest(http.MethodGet, "/auth/login?redirect=/admin/documents", http.NoBody)
		loginW := httptest.NewRecorder()
		h.Login(loginW, loginReq)

		location := loginW.Header().Get("Location")
		parsed, _ := parseRedirectURL(location)
		state := parsed.Query().Get("state")
		*noncePtr = parsed.Query().Get("nonce")

		callbackURL := "/auth/callback?code=test-auth-code&state=" + state
		req := httptest.NewRequest(http.MethodGet, callbackURL, http.NoBody)
		for _, c := range loginW.Result().Cookies() {
			req.AddCookie(c)
		}
		w := httptest.NewRecorder()

		h.Callback(w, req)

		if w.Code != http.StatusFound {
			t.Fatalf("expected 302, got %d; body: %s", w.Code, w.Body.String())
		}
		redirectTo := w.Header().Get("Location")
		if redirectTo != "/admin/documents" {
			t.Errorf("expected redirect to /admin/documents, got %q", redirectTo)
		}
	})

	t.Run("rejects callback when sub missing from claims", func(t *testing.T) {
		// v0.21.0: identity is `sub`; email is optional and display-only.
		// A token with no sub must be rejected (previously: no email → 400).
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatalf("generating RSA key: %v", err)
		}

		var serverURL string
		var expectedNonce string
		mux := http.NewServeMux()
		mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
			discovery := map[string]any{
				"issuer":                                serverURL,
				"authorization_endpoint":                serverURL + "/authorize",
				"token_endpoint":                        serverURL + "/token",
				"jwks_uri":                              serverURL + "/certs",
				"id_token_signing_alg_values_supported": []string{"RS256"},
				"subject_types_supported":               []string{"public"},
				"response_types_supported":              []string{"code"},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(discovery)
		})
		mux.HandleFunc("GET /certs", func(w http.ResponseWriter, r *http.Request) {
			jwk := jose.JSONWebKey{Key: &privateKey.PublicKey, KeyID: "test-key", Algorithm: string(jose.RS256), Use: "sig"}
			jwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{jwk}}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(jwks)
		})
		mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
			signer, _ := jose.NewSigner(
				jose.SigningKey{Algorithm: jose.RS256, Key: privateKey},
				(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "test-key"),
			)
			now := time.Now()
			// No Subject — this is the scenario under test.
			claims := josejwt.Claims{
				Issuer:    serverURL,
				Audience:  josejwt.Audience{"test-client-id"},
				IssuedAt:  josejwt.NewNumericDate(now),
				Expiry:    josejwt.NewNumericDate(now.Add(1 * time.Hour)),
				NotBefore: josejwt.NewNumericDate(now.Add(-1 * time.Minute)),
			}
			extraClaims := map[string]any{
				"email": "present@example.com",
				"name":  "No Sub User",
				"nonce": expectedNonce,
			}
			rawToken, _ := josejwt.Signed(signer).Claims(claims).Claims(extraClaims).Serialize()
			tokenResponse := map[string]any{
				"access_token": "test-access-token",
				"token_type":   "Bearer",
				"id_token":     rawToken,
				"expires_in":   3600,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(tokenResponse)
		})

		mockServer := httptest.NewServer(mux)
		defer mockServer.Close()
		serverURL = mockServer.URL

		ctx := gooidc.InsecureIssuerURLContext(context.Background(), mockServer.URL)
		store := sessions.NewCookieStore([]byte("test-secret-key-32-bytes-long!!"))
		store.Options = &sessions.Options{Path: "/", HttpOnly: true, MaxAge: 86400}

		h, err := New(ctx, Config{
			OIDCCfg: config.OIDCConfig{
				ProviderURL:  mockServer.URL,
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
				RedirectURL:  "http://localhost:8080/auth/callback",
			},
			SessionStore: store,
			Repo:         &mockUserRepo{},
			Logger:       slog.Default(),
		})
		if err != nil {
			t.Fatalf("creating handler: %v", err)
		}

		loginReq := httptest.NewRequest(http.MethodGet, "/auth/login", http.NoBody)
		loginW := httptest.NewRecorder()
		h.Login(loginW, loginReq)

		location := loginW.Header().Get("Location")
		parsed, _ := parseRedirectURL(location)
		state := parsed.Query().Get("state")
		expectedNonce = parsed.Query().Get("nonce")

		callbackURL := "/auth/callback?code=test-auth-code&state=" + state
		req := httptest.NewRequest(http.MethodGet, callbackURL, http.NoBody)
		for _, c := range loginW.Result().Cookies() {
			req.AddCookie(c)
		}
		w := httptest.NewRecorder()

		h.Callback(w, req)

		// go-oidc's IDTokenVerifier requires the `sub` claim and rejects the
		// token at Verify() time — so we get 401 (authentication failed),
		// not 400 from our own sub check. Either outcome is correct; assert
		// the token was rejected.
		if w.Code != http.StatusBadRequest && w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 400 or 401 for missing sub, got %d; body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("rejects callback when token endpoint returns no id_token", func(t *testing.T) {
		h, server := newCustomTokenHandler(t, &mockUserRepo{}, func(w http.ResponseWriter, r *http.Request, serverURL string) {
			// Return valid access token but no id_token.
			tokenResponse := map[string]any{
				"access_token": "test-access-token",
				"token_type":   "Bearer",
				"expires_in":   3600,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(tokenResponse)
		})
		defer server.Close()

		loginReq := httptest.NewRequest(http.MethodGet, "/auth/login", http.NoBody)
		loginW := httptest.NewRecorder()
		h.Login(loginW, loginReq)

		location := loginW.Header().Get("Location")
		parsed, _ := parseRedirectURL(location)
		state := parsed.Query().Get("state")

		callbackURL := "/auth/callback?code=test-auth-code&state=" + state
		req := httptest.NewRequest(http.MethodGet, callbackURL, http.NoBody)
		for _, c := range loginW.Result().Cookies() {
			req.AddCookie(c)
		}
		w := httptest.NewRecorder()

		h.Callback(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for missing id_token, got %d; body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("rejects callback when token exchange fails", func(t *testing.T) {
		h, server := newCustomTokenHandler(t, &mockUserRepo{}, func(w http.ResponseWriter, r *http.Request, serverURL string) {
			http.Error(w, "token_error", http.StatusBadRequest)
		})
		defer server.Close()

		loginReq := httptest.NewRequest(http.MethodGet, "/auth/login", http.NoBody)
		loginW := httptest.NewRecorder()
		h.Login(loginW, loginReq)

		location := loginW.Header().Get("Location")
		parsed, _ := parseRedirectURL(location)
		state := parsed.Query().Get("state")

		callbackURL := "/auth/callback?code=test-auth-code&state=" + state
		req := httptest.NewRequest(http.MethodGet, callbackURL, http.NoBody)
		for _, c := range loginW.Result().Cookies() {
			req.AddCookie(c)
		}
		w := httptest.NewRecorder()

		h.Callback(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for token exchange failure, got %d; body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("rejects callback when id_token verification fails", func(t *testing.T) {
		h, server := newCustomTokenHandler(t, &mockUserRepo{}, func(w http.ResponseWriter, r *http.Request, serverURL string) {
			// Return an invalid JWT as id_token.
			tokenResponse := map[string]any{
				"access_token": "test-access-token",
				"token_type":   "Bearer",
				"id_token":     "invalid.jwt.token",
				"expires_in":   3600,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(tokenResponse)
		})
		defer server.Close()

		loginReq := httptest.NewRequest(http.MethodGet, "/auth/login", http.NoBody)
		loginW := httptest.NewRecorder()
		h.Login(loginW, loginReq)

		location := loginW.Header().Get("Location")
		parsed, _ := parseRedirectURL(location)
		state := parsed.Query().Get("state")

		callbackURL := "/auth/callback?code=test-auth-code&state=" + state
		req := httptest.NewRequest(http.MethodGet, callbackURL, http.NoBody)
		for _, c := range loginW.Result().Cookies() {
			req.AddCookie(c)
		}
		w := httptest.NewRecorder()

		h.Callback(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for invalid id_token, got %d; body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("returns 500 when user creation fails", func(t *testing.T) {
		repo := &mockUserRepo{
			createFn: func(ctx context.Context, user *model.User) error {
				return errors.New("database error")
			},
		}
		h, server, noncePtr := newTestHandler(t, repo)
		defer server.Close()

		state, cookies := doLogin(t, h, noncePtr)
		callbackURL := "/auth/callback?code=test-auth-code&state=" + state
		req := httptest.NewRequest(http.MethodGet, callbackURL, http.NoBody)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		w := httptest.NewRecorder()

		h.Callback(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d; body: %s", w.Code, w.Body.String())
		}
	})
}

func TestLogout(t *testing.T) {
	t.Run("returns end_session URL with id_token_hint when configured", func(t *testing.T) {
		repo := &mockUserRepo{
			createFn: func(ctx context.Context, user *model.User) error {
				user.ID = 1
				return nil
			},
		}
		h, server, noncePtr := newTestHandler(t, repo)
		defer server.Close()

		// Do a full login+callback to populate session with id_token.
		state, loginCookies := doLogin(t, h, noncePtr)
		callbackReq := httptest.NewRequest(http.MethodGet, "/auth/callback?code=test-code&state="+state, http.NoBody)
		for _, c := range loginCookies {
			callbackReq.AddCookie(c)
		}
		callbackW := httptest.NewRecorder()
		h.Callback(callbackW, callbackReq)
		if callbackW.Code != http.StatusFound {
			t.Fatalf("callback: expected 302, got %d", callbackW.Code)
		}

		// Now call logout with both session cookies from callback.
		// The callback response contains multiple Set-Cookie headers for the
		// same name (one expiring the old session, one setting the new one).
		// Use only the last of each name — browsers replace by name.
		logoutReq := httptest.NewRequest(http.MethodPost, "/auth/logout", http.NoBody)
		callbackCookies := callbackW.Result().Cookies()
		added := make(map[string]bool)
		for i := len(callbackCookies) - 1; i >= 0; i-- {
			name := callbackCookies[i].Name
			if !added[name] && (name == sessionName || name == idTokenSessionName) {
				logoutReq.AddCookie(callbackCookies[i])
				added[name] = true
			}
		}
		logoutW := httptest.NewRecorder()
		h.Logout(logoutW, logoutReq)

		if logoutW.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", logoutW.Code)
		}
		if ct := logoutW.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", ct)
		}

		var body map[string]string
		if err := json.NewDecoder(logoutW.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		redirectURL := body["redirect_url"]
		parsed, err := neturl.Parse(redirectURL)
		if err != nil {
			t.Fatalf("parsing redirect_url: %v", err)
		}

		// Should point to the provider's end-session endpoint.
		if !strings.HasSuffix(parsed.Path, "/end-session") {
			t.Errorf("expected end-session path, got %q", parsed.Path)
		}
		if hint := parsed.Query().Get("id_token_hint"); hint == "" {
			t.Error("expected id_token_hint in redirect URL")
		}
		if postLogout := parsed.Query().Get("post_logout_redirect_uri"); postLogout != "http://localhost:8080" {
			t.Errorf("expected post_logout_redirect_uri=http://localhost:8080, got %q", postLogout)
		}

		// Verify session cookie has MaxAge=-1 (expired).
		for _, c := range logoutW.Result().Cookies() {
			if c.Name == sessionName {
				if c.MaxAge >= 0 {
					t.Errorf("expected MaxAge < 0 (session invalidated), got %d", c.MaxAge)
				}
			}
		}
	})

	t.Run("falls back to root when no end_session_endpoint", func(t *testing.T) {
		repo := &mockUserRepo{}
		h, server, _ := newTestHandler(t, repo)
		defer server.Close()

		// Clear end_session_endpoint to simulate a provider that doesn't support it.
		h.endSessionEndpoint = ""

		req := httptest.NewRequest(http.MethodPost, "/auth/logout", http.NoBody)
		w := httptest.NewRecorder()
		h.Logout(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var body map[string]string
		if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if body["redirect_url"] != "/" {
			t.Errorf("expected redirect_url=/, got %q", body["redirect_url"])
		}
	})

	t.Run("returns end_session URL without id_token_hint for stale session", func(t *testing.T) {
		repo := &mockUserRepo{}
		h, server, _ := newTestHandler(t, repo)
		defer server.Close()

		// No login — session has no id_token.
		req := httptest.NewRequest(http.MethodPost, "/auth/logout", http.NoBody)
		w := httptest.NewRecorder()
		h.Logout(w, req)

		var body map[string]string
		if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		parsed, err := neturl.Parse(body["redirect_url"])
		if err != nil {
			t.Fatalf("parsing redirect_url: %v", err)
		}
		if !strings.HasSuffix(parsed.Path, "/end-session") {
			t.Errorf("expected end-session path, got %q", parsed.Path)
		}
		if hint := parsed.Query().Get("id_token_hint"); hint != "" {
			t.Error("expected no id_token_hint for stale session")
		}
	})
}

// stubTokenRevoker captures RevokeUserTokensSince calls for assertion.
type stubTokenRevoker struct {
	calls  int
	userID int64
	since  time.Time
	count  int64
	err    error
}

func (s *stubTokenRevoker) RevokeUserTokensSince(_ context.Context, userID int64, since time.Time) (int64, error) {
	s.calls++
	s.userID = userID
	s.since = since
	return s.count, s.err
}

func TestCallback_WritesLoginAtAnchor(t *testing.T) {
	repo := &mockUserRepo{
		createFn: func(_ context.Context, user *model.User) error {
			user.ID = 77
			return nil
		},
	}
	h, server, noncePtr := newTestHandler(t, repo)
	defer server.Close()

	state, loginCookies := doLogin(t, h, noncePtr)
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=test-code&state="+state, http.NoBody)
	for _, c := range loginCookies {
		req.AddCookie(c)
	}
	before := time.Now().Unix()
	w := httptest.NewRecorder()
	h.Callback(w, req)
	after := time.Now().Unix()

	if w.Code != http.StatusFound {
		t.Fatalf("callback status = %d, want 302: %s", w.Code, w.Body.String())
	}

	// Reconstruct the session from the Set-Cookie headers and assert login_at
	// is present with a timestamp bracketing the callback execution.
	cookies := w.Result().Cookies()
	verifyReq := httptest.NewRequest(http.MethodGet, "/any", http.NoBody)
	added := make(map[string]bool)
	for i := len(cookies) - 1; i >= 0; i-- {
		if cookies[i].Name == sessionName && !added[sessionName] {
			verifyReq.AddCookie(cookies[i])
			added[sessionName] = true
		}
	}
	session, err := h.store.Get(verifyReq, sessionName)
	if err != nil {
		t.Fatalf("re-reading session: %v", err)
	}
	loginAt, ok := session.Values["login_at"].(int64)
	if !ok {
		t.Fatalf("login_at missing from session (values=%#v)", session.Values)
	}
	if loginAt < before || loginAt > after {
		t.Errorf("login_at = %d, want between %d and %d", loginAt, before, after)
	}
}

func TestLogout_RevokesOAuthTokensWhenOptedIn(t *testing.T) {
	t.Run("revoke_oauth=true calls revoker with login_at", func(t *testing.T) {
		revoker := &stubTokenRevoker{count: 3}
		repo := &mockUserRepo{
			createFn: func(_ context.Context, user *model.User) error {
				user.ID = 42
				return nil
			},
		}
		h, server, noncePtr := newTestHandler(t, repo)
		defer server.Close()
		h.tokenRevoker = revoker

		// Full login+callback so the session carries user_id + login_at.
		state, loginCookies := doLogin(t, h, noncePtr)
		callbackReq := httptest.NewRequest(http.MethodGet, "/auth/callback?code=test-code&state="+state, http.NoBody)
		for _, c := range loginCookies {
			callbackReq.AddCookie(c)
		}
		callbackW := httptest.NewRecorder()
		h.Callback(callbackW, callbackReq)

		logoutReq := httptest.NewRequest(http.MethodPost, "/auth/logout?revoke_oauth=true", http.NoBody)
		// Callback emits one cookie to expire the pre-callback session and another
		// to set the new session. Walk newest-first and keep the last of each
		// name — matches how browsers resolve duplicate Set-Cookie headers.
		cbCookies := callbackW.Result().Cookies()
		added := make(map[string]bool)
		for i := len(cbCookies) - 1; i >= 0; i-- {
			c := cbCookies[i]
			if !added[c.Name] && (c.Name == sessionName || c.Name == idTokenSessionName) {
				logoutReq.AddCookie(c)
				added[c.Name] = true
			}
		}
		h.Logout(httptest.NewRecorder(), logoutReq)

		if revoker.calls != 1 {
			t.Fatalf("revoker.calls = %d, want 1", revoker.calls)
		}
		if revoker.userID != 42 {
			t.Errorf("revoker.userID = %d, want 42", revoker.userID)
		}
		if time.Since(revoker.since) > time.Minute {
			t.Errorf("revoker.since = %v, should be within the last minute", revoker.since)
		}
	})

	t.Run("default logout does not revoke", func(t *testing.T) {
		revoker := &stubTokenRevoker{}
		repo := &mockUserRepo{
			createFn: func(_ context.Context, user *model.User) error {
				user.ID = 99
				return nil
			},
		}
		h, server, noncePtr := newTestHandler(t, repo)
		defer server.Close()
		h.tokenRevoker = revoker

		state, loginCookies := doLogin(t, h, noncePtr)
		callbackReq := httptest.NewRequest(http.MethodGet, "/auth/callback?code=test-code&state="+state, http.NoBody)
		for _, c := range loginCookies {
			callbackReq.AddCookie(c)
		}
		callbackW := httptest.NewRecorder()
		h.Callback(callbackW, callbackReq)

		logoutReq := httptest.NewRequest(http.MethodPost, "/auth/logout", http.NoBody)
		cbCookies := callbackW.Result().Cookies()
		added := make(map[string]bool)
		for i := len(cbCookies) - 1; i >= 0; i-- {
			c := cbCookies[i]
			if !added[c.Name] && (c.Name == sessionName || c.Name == idTokenSessionName) {
				logoutReq.AddCookie(c)
				added[c.Name] = true
			}
		}
		h.Logout(httptest.NewRecorder(), logoutReq)

		if revoker.calls != 0 {
			t.Errorf("revoker.calls = %d, want 0 when flag absent", revoker.calls)
		}
	})

	t.Run("no-op when TokenRevoker nil", func(t *testing.T) {
		repo := &mockUserRepo{
			createFn: func(_ context.Context, user *model.User) error {
				user.ID = 5
				return nil
			},
		}
		h, server, noncePtr := newTestHandler(t, repo)
		defer server.Close()
		// h.tokenRevoker left nil — should not panic.

		state, loginCookies := doLogin(t, h, noncePtr)
		callbackReq := httptest.NewRequest(http.MethodGet, "/auth/callback?code=test-code&state="+state, http.NoBody)
		for _, c := range loginCookies {
			callbackReq.AddCookie(c)
		}
		callbackW := httptest.NewRecorder()
		h.Callback(callbackW, callbackReq)

		logoutReq := httptest.NewRequest(http.MethodPost, "/auth/logout?revoke_oauth=true", http.NoBody)
		for _, c := range callbackW.Result().Cookies() {
			logoutReq.AddCookie(c)
		}
		w := httptest.NewRecorder()
		h.Logout(w, logoutReq)

		if w.Code != http.StatusOK {
			t.Errorf("logout status = %d, want 200", w.Code)
		}
	})

	t.Run("silent when session has no login_at anchor", func(t *testing.T) {
		revoker := &stubTokenRevoker{}
		h, server, _ := newTestHandler(t, &mockUserRepo{})
		defer server.Close()
		h.tokenRevoker = revoker

		// Seed a session with user_id but no login_at.
		seed := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		seedRR := httptest.NewRecorder()
		session, _ := h.store.New(seed, sessionName)
		session.Values["user_id"] = int64(7)
		if err := session.Save(seed, seedRR); err != nil {
			t.Fatalf("seeding session: %v", err)
		}

		logoutReq := httptest.NewRequest(http.MethodPost, "/auth/logout?revoke_oauth=true", http.NoBody)
		for _, c := range seedRR.Result().Cookies() {
			logoutReq.AddCookie(c)
		}
		h.Logout(httptest.NewRecorder(), logoutReq)

		if revoker.calls != 0 {
			t.Errorf("revoker.calls = %d, want 0 when login_at missing", revoker.calls)
		}
	})
}

func TestGenerateState(t *testing.T) {
	t.Run("generates non-empty unique states", func(t *testing.T) {
		s1, err := generateState()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		s2, err := generateState()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s1 == "" || s2 == "" {
			t.Fatal("expected non-empty states")
		}
		if s1 == s2 {
			t.Error("expected unique states")
		}
	})
}

func TestFindOrCreateUser(t *testing.T) {
	t.Run("finds by sub and updates profile", func(t *testing.T) {
		var updated bool
		existingUser := &model.User{
			ID:    1,
			Name:  "Old Name",
			Email: "old@example.com",
			OIDCSub: sql.NullString{
				String: "sub-1",
				Valid:  true,
			},
		}
		repo := &mockUserRepo{
			findBySubFn: func(ctx context.Context, sub string) (*model.User, error) {
				return existingUser, nil
			},
			updateFn: func(ctx context.Context, user *model.User) error {
				updated = true
				return nil
			},
		}
		h, server, _ := newTestHandler(t, repo)
		defer server.Close()

		user, err := h.findOrCreateUser(context.Background(), "sub-1", "new@example.com", true, "New Name", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Email != "new@example.com" {
			t.Errorf("expected email updated, got %q", user.Email)
		}
		if !updated {
			t.Error("expected update to be called for profile change")
		}
	})

	t.Run("unknown sub creates fresh user even when email would match another record", func(t *testing.T) {
		// Regression guard for v0.21.0: FindUserByEmail is gone. The only
		// lookup is by sub. An unknown sub must always hit CreateUser.
		var created *model.User
		repo := &mockUserRepo{
			createFn: func(_ context.Context, user *model.User) error {
				user.ID = 101
				created = user
				return nil
			},
		}
		h, server, _ := newTestHandler(t, repo)
		defer server.Close()

		user, err := h.findOrCreateUser(context.Background(), "new-sub", "user@example.com", true, "User", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.ID != 101 {
			t.Errorf("expected new user ID 101, got %d", user.ID)
		}
		if created == nil || !created.OIDCSub.Valid || created.OIDCSub.String != "new-sub" {
			t.Error("expected new user created with the incoming sub")
		}
	})

	t.Run("creates new user when not found", func(t *testing.T) {
		var created *model.User
		repo := &mockUserRepo{
			createFn: func(ctx context.Context, user *model.User) error {
				user.ID = 100
				created = user
				return nil
			},
		}
		h, server, _ := newTestHandler(t, repo)
		defer server.Close()

		user, err := h.findOrCreateUser(context.Background(), "brand-new-sub", "new@example.com", true, "New User", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.ID != 100 {
			t.Errorf("expected created user ID 100, got %d", user.ID)
		}
		if created == nil {
			t.Fatal("expected CreateUser to be called")
		}
		if created.IsAdmin {
			t.Error("new users should not be admin")
		}
		if !created.OIDCSub.Valid || created.OIDCSub.String != "brand-new-sub" {
			t.Error("expected OIDC sub to be set")
		}
	})

	t.Run("returns error on sub lookup failure", func(t *testing.T) {
		repo := &mockUserRepo{
			findBySubFn: func(ctx context.Context, sub string) (*model.User, error) {
				return nil, errors.New("database error")
			},
		}
		h, server, _ := newTestHandler(t, repo)
		defer server.Close()

		_, err := h.findOrCreateUser(context.Background(), "sub", "email@example.com", true, "Name", nil)
		if err == nil {
			t.Fatal("expected error on sub lookup failure")
		}
	})
}

func TestFindOrCreateUserBootstrapAdmin(t *testing.T) {
	t.Run("promotes first user when verified email matches", func(t *testing.T) {
		var created *model.User
		repo := &mockUserRepo{
			createFn: func(_ context.Context, user *model.User) error {
				user.ID = 1
				created = user
				return nil
			},
		}
		h, server, _ := newTestHandler(t, repo)
		defer server.Close()
		h.bootstrapAdminEmail = "owner@example.com"

		_, err := h.findOrCreateUser(context.Background(), "sub", "Owner@example.com", true, "Owner", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if created == nil || !created.IsAdmin {
			t.Error("expected bootstrap admin to be promoted")
		}
	})

	t.Run("does not promote when email_verified=false", func(t *testing.T) {
		var created *model.User
		repo := &mockUserRepo{
			createFn: func(_ context.Context, user *model.User) error {
				user.ID = 2
				created = user
				return nil
			},
		}
		h, server, _ := newTestHandler(t, repo)
		defer server.Close()
		h.bootstrapAdminEmail = "owner@example.com"

		_, err := h.findOrCreateUser(context.Background(), "sub", "owner@example.com", false, "Owner", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if created == nil || created.IsAdmin {
			t.Error("expected bootstrap promotion to require email_verified=true")
		}
	})

	t.Run("does not promote when email does not match", func(t *testing.T) {
		var created *model.User
		repo := &mockUserRepo{
			createFn: func(_ context.Context, user *model.User) error {
				user.ID = 3
				created = user
				return nil
			},
		}
		h, server, _ := newTestHandler(t, repo)
		defer server.Close()
		h.bootstrapAdminEmail = "owner@example.com"

		_, err := h.findOrCreateUser(context.Background(), "sub", "other@example.com", true, "Other", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if created == nil || created.IsAdmin {
			t.Error("expected no promotion for non-matching email")
		}
	})

	t.Run("returning user is not re-bootstrapped", func(t *testing.T) {
		existing := &model.User{
			ID:      5,
			Email:   "owner@example.com",
			OIDCSub: sql.NullString{String: "sub", Valid: true},
			IsAdmin: false,
		}
		var updated *model.User
		repo := &mockUserRepo{
			findBySubFn: func(_ context.Context, _ string) (*model.User, error) {
				return existing, nil
			},
			updateFn: func(_ context.Context, u *model.User) error {
				updated = u
				return nil
			},
		}
		h, server, _ := newTestHandler(t, repo)
		defer server.Close()
		h.bootstrapAdminEmail = "owner@example.com"

		user, err := h.findOrCreateUser(context.Background(), "sub", "owner@example.com", true, "Owner", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.IsAdmin {
			t.Error("returning user must not be re-bootstrapped to admin")
		}
		if updated != nil && updated.IsAdmin {
			t.Error("UpdateUser must not flip IsAdmin via bootstrap")
		}
	})

	t.Run("groups outrank bootstrap on create", func(t *testing.T) {
		// When both are configured and groups grants admin, adminSource is
		// groups. When groups does not grant (user not in group) the bootstrap
		// path may still fire. This test covers the "groups grants" case.
		var created *model.User
		repo := &mockUserRepo{
			createFn: func(_ context.Context, user *model.User) error {
				user.ID = 7
				created = user
				return nil
			},
		}
		h, server, _ := newTestHandler(t, repo)
		defer server.Close()
		h.adminGroups = []string{"admins"}
		h.bootstrapAdminEmail = "owner@example.com"

		_, err := h.findOrCreateUser(context.Background(), "sub", "owner@example.com", true, "Owner", []string{"admins"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if created == nil || !created.IsAdmin {
			t.Error("expected user to be admin (via groups)")
		}
	})
}

func TestResolveAdmin(t *testing.T) {
	tests := []struct {
		name        string
		adminGroups []string
		groups      []string
		wantAdmin   bool
		wantSync    bool
	}{
		{"empty config no groups", nil, nil, false, false},
		{"empty config user has groups", nil, []string{"admins"}, false, false},
		{"configured user matches", []string{"admins"}, []string{"users", "admins"}, true, true},
		{"configured user no match", []string{"admins"}, []string{"users", "editors"}, false, true},
		{"configured user no groups", []string{"admins"}, nil, false, true},
		{"configured user empty groups", []string{"admins"}, []string{}, false, true},
		{"multiple admin groups one match", []string{"admins", "superusers"}, []string{"superusers"}, true, true},
		{"multiple admin groups no match", []string{"admins", "superusers"}, []string{"viewers"}, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handler{adminGroups: tt.adminGroups}
			gotAdmin, gotSync := h.resolveAdmin(tt.groups)
			if gotAdmin != tt.wantAdmin {
				t.Errorf("resolveAdmin() isAdmin = %v, want %v", gotAdmin, tt.wantAdmin)
			}
			if gotSync != tt.wantSync {
				t.Errorf("resolveAdmin() shouldSync = %v, want %v", gotSync, tt.wantSync)
			}
		})
	}
}

func TestFindOrCreateUserAdminGroups(t *testing.T) {
	t.Run("new user gets admin when group matches", func(t *testing.T) {
		var created *model.User
		repo := &mockUserRepo{
			createFn: func(ctx context.Context, user *model.User) error {
				user.ID = 10
				created = user
				return nil
			},
		}
		h, server, _ := newTestHandler(t, repo)
		defer server.Close()
		h.adminGroups = []string{"documcp-admins"}

		_, err := h.findOrCreateUser(context.Background(), "sub-new", "admin@example.com", true, "Admin", []string{"users", "documcp-admins"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if created == nil {
			t.Fatal("expected CreateUser to be called")
		}
		if !created.IsAdmin {
			t.Error("expected new user to be admin")
		}
	})

	t.Run("new user not admin when group does not match", func(t *testing.T) {
		var created *model.User
		repo := &mockUserRepo{
			createFn: func(ctx context.Context, user *model.User) error {
				user.ID = 11
				created = user
				return nil
			},
		}
		h, server, _ := newTestHandler(t, repo)
		defer server.Close()
		h.adminGroups = []string{"documcp-admins"}

		_, err := h.findOrCreateUser(context.Background(), "sub-new", "user@example.com", true, "User", []string{"users"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if created == nil {
			t.Fatal("expected CreateUser to be called")
		}
		if created.IsAdmin {
			t.Error("expected new user to NOT be admin")
		}
	})

	t.Run("new user not admin when feature disabled", func(t *testing.T) {
		var created *model.User
		repo := &mockUserRepo{
			createFn: func(ctx context.Context, user *model.User) error {
				user.ID = 12
				created = user
				return nil
			},
		}
		h, server, _ := newTestHandler(t, repo)
		defer server.Close()
		// adminGroups is nil (default from newTestHandler)

		_, err := h.findOrCreateUser(context.Background(), "sub-new", "user@example.com", true, "User", []string{"documcp-admins"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if created.IsAdmin {
			t.Error("expected new user to NOT be admin when feature disabled")
		}
	})

	t.Run("returning user promoted to admin", func(t *testing.T) {
		var updated bool
		existingUser := &model.User{ID: 1, Name: "User", Email: "user@example.com", IsAdmin: false,
			OIDCSub: sql.NullString{String: "sub-1", Valid: true}}
		repo := &mockUserRepo{
			findBySubFn: func(ctx context.Context, sub string) (*model.User, error) {
				return existingUser, nil
			},
			updateFn: func(ctx context.Context, user *model.User) error {
				updated = true
				return nil
			},
		}
		h, server, _ := newTestHandler(t, repo)
		defer server.Close()
		h.adminGroups = []string{"admins"}

		user, err := h.findOrCreateUser(context.Background(), "sub-1", "user@example.com", true, "User", []string{"admins"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !user.IsAdmin {
			t.Error("expected user to be promoted to admin")
		}
		if !updated {
			t.Error("expected UpdateUser to be called")
		}
	})

	t.Run("returning user demoted from admin", func(t *testing.T) {
		var updated bool
		existingUser := &model.User{ID: 1, Name: "Admin", Email: "admin@example.com", IsAdmin: true,
			OIDCSub: sql.NullString{String: "sub-1", Valid: true}}
		repo := &mockUserRepo{
			findBySubFn: func(ctx context.Context, sub string) (*model.User, error) {
				return existingUser, nil
			},
			updateFn: func(ctx context.Context, user *model.User) error {
				updated = true
				return nil
			},
		}
		h, server, _ := newTestHandler(t, repo)
		defer server.Close()
		h.adminGroups = []string{"admins"}

		user, err := h.findOrCreateUser(context.Background(), "sub-1", "admin@example.com", true, "Admin", []string{"users"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.IsAdmin {
			t.Error("expected user to be demoted from admin")
		}
		if !updated {
			t.Error("expected UpdateUser to be called for demotion")
		}
	})

	t.Run("feature disabled preserves manual admin", func(t *testing.T) {
		var updated bool
		existingUser := &model.User{ID: 1, Name: "Admin", Email: "admin@example.com", IsAdmin: true,
			OIDCSub: sql.NullString{String: "sub-1", Valid: true}}
		repo := &mockUserRepo{
			findBySubFn: func(ctx context.Context, sub string) (*model.User, error) {
				return existingUser, nil
			},
			updateFn: func(ctx context.Context, user *model.User) error {
				updated = true
				return nil
			},
		}
		h, server, _ := newTestHandler(t, repo)
		defer server.Close()
		// adminGroups is nil (feature disabled)

		user, err := h.findOrCreateUser(context.Background(), "sub-1", "admin@example.com", true, "Admin", []string{"users"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !user.IsAdmin {
			t.Error("expected manual admin to be preserved when feature disabled")
		}
		if updated {
			t.Error("expected no update when nothing changed")
		}
	})
}

// newCustomTokenHandler creates an OIDC handler with a custom token endpoint handler.
func newCustomTokenHandler(t *testing.T, repo UserRepo, tokenHandler func(http.ResponseWriter, *http.Request, string)) (*Handler, *httptest.Server) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}

	var serverURL string
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		discovery := map[string]any{
			"issuer":                                serverURL,
			"authorization_endpoint":                serverURL + "/authorize",
			"token_endpoint":                        serverURL + "/token",
			"jwks_uri":                              serverURL + "/certs",
			"id_token_signing_alg_values_supported": []string{"RS256"},
			"subject_types_supported":               []string{"public"},
			"response_types_supported":              []string{"code"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(discovery)
	})
	mux.HandleFunc("GET /certs", func(w http.ResponseWriter, r *http.Request) {
		jwk := jose.JSONWebKey{Key: &privateKey.PublicKey, KeyID: "test-key", Algorithm: string(jose.RS256), Use: "sig"}
		jwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{jwk}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	})
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		tokenHandler(w, r, serverURL)
	})

	server := httptest.NewServer(mux)
	serverURL = server.URL

	ctx := gooidc.InsecureIssuerURLContext(context.Background(), server.URL)
	store := sessions.NewCookieStore([]byte("test-secret-key-32-bytes-long!!"))
	store.Options = &sessions.Options{Path: "/", HttpOnly: true, MaxAge: 86400}

	h, err := New(ctx, Config{
		OIDCCfg: config.OIDCConfig{
			ProviderURL:  server.URL,
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			RedirectURL:  "http://localhost:8080/auth/callback",
		},
		SessionStore: store,
		Repo:         repo,
		Logger:       slog.Default(),
	})
	if err != nil {
		t.Fatalf("creating OIDC handler: %v", err)
	}
	return h, server
}

// parseRedirectURL is a helper to parse the Location header URL.
func parseRedirectURL(rawURL string) (*neturl.URL, error) {
	return neturl.Parse(rawURL)
}

// TestNew_RejectsUnsafeProviderURL verifies that New() rejects OIDC provider
// URLs that fall in the SSRF block-list (loopback, link-local, unspecified).
// It restores the real validateOIDCURL for the duration of the test — the
// package-level init() in this file otherwise no-ops it so existing tests
// can use httptest servers bound to 127.0.0.1.
func TestNew_RejectsUnsafeProviderURL(t *testing.T) {
	orig := validateOIDCURL
	validateOIDCURL = func(u string) error {
		return security.ValidateExternalURL(u, true)
	}
	t.Cleanup(func() { validateOIDCURL = orig })

	cases := []struct {
		name, url, wantSubstr string
	}{
		{"loopback IPv4", "http://127.0.0.1/", "loopback"},
		{"loopback IPv6", "http://[::1]/", "loopback"},
		{"localhost hostname", "http://localhost/", "localhost"},
		{"link-local IPv4", "http://169.254.169.254/", "link-local"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New(context.Background(), Config{
				OIDCCfg: config.OIDCConfig{
					ProviderURL: tc.url,
					ClientID:    "test-client-id",
				},
				SessionStore: sessions.NewCookieStore([]byte("test-secret-key-32-bytes-long!!")),
				Repo:         &mockUserRepo{},
				Logger:       slog.Default(),
			})
			if err == nil {
				t.Fatalf("expected validation error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantSubstr)
			}
		})
	}
}
