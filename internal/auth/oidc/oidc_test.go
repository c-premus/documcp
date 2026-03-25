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
	"testing"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4"
	josejwt "github.com/go-jose/go-jose/v4/jwt"
	"github.com/gorilla/sessions"

	"github.com/c-premus/documcp/internal/config"
	"github.com/c-premus/documcp/internal/model"
)

// mockUserRepo implements UserRepo for testing.
type mockUserRepo struct {
	findBySubFn   func(ctx context.Context, sub string) (*model.User, error)
	findByEmailFn func(ctx context.Context, email string) (*model.User, error)
	createFn      func(ctx context.Context, user *model.User) error
	updateFn      func(ctx context.Context, user *model.User) error
}

func (m *mockUserRepo) FindUserByOIDCSub(ctx context.Context, sub string) (*model.User, error) {
	if m.findBySubFn != nil {
		return m.findBySubFn(ctx, sub)
	}
	return nil, sql.ErrNoRows
}

func (m *mockUserRepo) FindUserByEmail(ctx context.Context, email string) (*model.User, error) {
	if m.findByEmailFn != nil {
		return m.findByEmailFn(ctx, email)
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
func setupMockOIDCProvider(t *testing.T) (*httptest.Server, *rsa.PrivateKey) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}

	mux := http.NewServeMux()

	// We need a reference to the server URL before creating it (for issuer), so use a pointer.
	var serverURL string

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
	return server, privateKey
}

// newTestHandler creates a Handler backed by the mock OIDC provider.
func newTestHandler(t *testing.T, repo UserRepo) (*Handler, *httptest.Server) {
	t.Helper()

	mockServer, _ := setupMockOIDCProvider(t)

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
	})
	if err != nil {
		t.Fatalf("creating OIDC handler: %v", err)
	}
	if h == nil {
		t.Fatal("OIDC handler is nil")
	}
	return h, mockServer
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
		h, server := newTestHandler(t, &mockUserRepo{})
		defer server.Close()
		if h == nil {
			t.Fatal("expected non-nil handler")
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
	h, server := newTestHandler(t, repo)
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
		h, server := newTestHandler(t, repo)
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
		h, server := newTestHandler(t, repo)
		defer server.Close()

		// First do a login to set the state in session.
		loginReq := httptest.NewRequest(http.MethodGet, "/auth/login", http.NoBody)
		loginW := httptest.NewRecorder()
		h.Login(loginW, loginReq)

		// Use the session cookie from login but with wrong state.
		req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=VALID&state=WRONG", http.NoBody)
		for _, c := range loginW.Result().Cookies() {
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
		h, server := newTestHandler(t, repo)
		defer server.Close()

		// Step 1: Login to get state.
		loginReq := httptest.NewRequest(http.MethodGet, "/auth/login", http.NoBody)
		loginW := httptest.NewRecorder()
		h.Login(loginW, loginReq)

		// Extract state from redirect URL.
		location := loginW.Header().Get("Location")
		parsed, _ := parseRedirectURL(location)
		state := parsed.Query().Get("state")
		if state == "" {
			t.Fatal("expected state in redirect URL")
		}

		// Step 2: Callback with the correct state and a valid code.
		callbackURL := "/auth/callback?code=test-auth-code&state=" + state
		req := httptest.NewRequest(http.MethodGet, callbackURL, http.NoBody)
		for _, c := range loginW.Result().Cookies() {
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
		h, server := newTestHandler(t, repo)
		defer server.Close()

		// Login to get state.
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

		if w.Code != http.StatusFound {
			t.Fatalf("expected 302, got %d; body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("links existing user by email", func(t *testing.T) {
		var updatedUser *model.User
		existingUser := &model.User{
			ID:    2,
			Name:  "Existing",
			Email: "test@example.com",
		}
		repo := &mockUserRepo{
			findByEmailFn: func(ctx context.Context, email string) (*model.User, error) {
				if email == "test@example.com" {
					return existingUser, nil
				}
				return nil, sql.ErrNoRows
			},
			updateFn: func(ctx context.Context, user *model.User) error {
				updatedUser = user
				return nil
			},
		}
		h, server := newTestHandler(t, repo)
		defer server.Close()

		// Login to get state.
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

		if w.Code != http.StatusFound {
			t.Fatalf("expected 302, got %d; body: %s", w.Code, w.Body.String())
		}

		if updatedUser == nil {
			t.Fatal("expected UpdateUser to be called to link OIDC identity")
		}
		if !updatedUser.OIDCSub.Valid || updatedUser.OIDCSub.String != "test-sub-123" {
			t.Errorf("expected OIDC sub to be linked, got %v", updatedUser.OIDCSub)
		}
	})

	t.Run("follows safe redirect from login", func(t *testing.T) {
		repo := &mockUserRepo{
			createFn: func(ctx context.Context, user *model.User) error {
				user.ID = 99
				return nil
			},
		}
		h, server := newTestHandler(t, repo)
		defer server.Close()

		// Login with redirect param.
		loginReq := httptest.NewRequest(http.MethodGet, "/auth/login?redirect=/admin/documents", http.NoBody)
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

		if w.Code != http.StatusFound {
			t.Fatalf("expected 302, got %d; body: %s", w.Code, w.Body.String())
		}
		redirectTo := w.Header().Get("Location")
		if redirectTo != "/admin/documents" {
			t.Errorf("expected redirect to /admin/documents, got %q", redirectTo)
		}
	})

	t.Run("rejects callback when email missing from claims", func(t *testing.T) {
		// To test missing email, we need a token endpoint that returns a token without email claim.
		// We'll create a custom mock provider for this case.
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
			signer, _ := jose.NewSigner(
				jose.SigningKey{Algorithm: jose.RS256, Key: privateKey},
				(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "test-key"),
			)
			now := time.Now()
			claims := josejwt.Claims{
				Issuer:    serverURL,
				Subject:   "test-sub-no-email",
				Audience:  josejwt.Audience{"test-client-id"},
				IssuedAt:  josejwt.NewNumericDate(now),
				Expiry:    josejwt.NewNumericDate(now.Add(1 * time.Hour)),
				NotBefore: josejwt.NewNumericDate(now.Add(-1 * time.Minute)),
			}
			// No email claim!
			extraClaims := map[string]any{
				"name": "No Email User",
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

		// Login to get state.
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

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for missing email, got %d; body: %s", w.Code, w.Body.String())
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
		h, server := newTestHandler(t, repo)
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

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d; body: %s", w.Code, w.Body.String())
		}
	})
}

func TestLogout(t *testing.T) {
	t.Run("clears session and redirects to root", func(t *testing.T) {
		repo := &mockUserRepo{}
		h, server := newTestHandler(t, repo)
		defer server.Close()

		req := httptest.NewRequest(http.MethodPost, "/auth/logout", http.NoBody)
		w := httptest.NewRecorder()

		h.Logout(w, req)

		if w.Code != http.StatusFound {
			t.Fatalf("expected 302, got %d", w.Code)
		}
		location := w.Header().Get("Location")
		if location != "/" {
			t.Errorf("expected redirect to /, got %q", location)
		}

		// Verify session cookie has MaxAge=-1 (expired).
		for _, c := range w.Result().Cookies() {
			if c.Name == sessionName {
				if c.MaxAge >= 0 {
					t.Errorf("expected MaxAge < 0 (session invalidated), got %d", c.MaxAge)
				}
			}
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
		h, server := newTestHandler(t, repo)
		defer server.Close()

		user, err := h.findOrCreateUser(context.Background(), "sub-1", "new@example.com", "New Name", nil)
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

	t.Run("finds by email and links OIDC identity", func(t *testing.T) {
		var updatedUser *model.User
		existingUser := &model.User{
			ID:    2,
			Email: "user@example.com",
		}
		repo := &mockUserRepo{
			findByEmailFn: func(ctx context.Context, email string) (*model.User, error) {
				return existingUser, nil
			},
			updateFn: func(ctx context.Context, user *model.User) error {
				updatedUser = user
				return nil
			},
		}
		h, server := newTestHandler(t, repo)
		defer server.Close()

		user, err := h.findOrCreateUser(context.Background(), "new-sub", "user@example.com", "User", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.ID != 2 {
			t.Errorf("expected existing user ID 2, got %d", user.ID)
		}
		if updatedUser == nil || !updatedUser.OIDCSub.Valid || updatedUser.OIDCSub.String != "new-sub" {
			t.Error("expected OIDC sub to be linked")
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
		h, server := newTestHandler(t, repo)
		defer server.Close()

		user, err := h.findOrCreateUser(context.Background(), "brand-new-sub", "new@example.com", "New User", nil)
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
		h, server := newTestHandler(t, repo)
		defer server.Close()

		_, err := h.findOrCreateUser(context.Background(), "sub", "email@example.com", "Name", nil)
		if err == nil {
			t.Fatal("expected error on sub lookup failure")
		}
	})

	t.Run("returns error on email lookup failure", func(t *testing.T) {
		repo := &mockUserRepo{
			findByEmailFn: func(ctx context.Context, email string) (*model.User, error) {
				return nil, errors.New("database error")
			},
		}
		h, server := newTestHandler(t, repo)
		defer server.Close()

		_, err := h.findOrCreateUser(context.Background(), "sub", "email@example.com", "Name", nil)
		if err == nil {
			t.Fatal("expected error on email lookup failure")
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
		h, server := newTestHandler(t, repo)
		defer server.Close()
		h.adminGroups = []string{"documcp-admins"}

		_, err := h.findOrCreateUser(context.Background(), "sub-new", "admin@example.com", "Admin", []string{"users", "documcp-admins"})
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
		h, server := newTestHandler(t, repo)
		defer server.Close()
		h.adminGroups = []string{"documcp-admins"}

		_, err := h.findOrCreateUser(context.Background(), "sub-new", "user@example.com", "User", []string{"users"})
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
		h, server := newTestHandler(t, repo)
		defer server.Close()
		// adminGroups is nil (default from newTestHandler)

		_, err := h.findOrCreateUser(context.Background(), "sub-new", "user@example.com", "User", []string{"documcp-admins"})
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
		h, server := newTestHandler(t, repo)
		defer server.Close()
		h.adminGroups = []string{"admins"}

		user, err := h.findOrCreateUser(context.Background(), "sub-1", "user@example.com", "User", []string{"admins"})
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
		h, server := newTestHandler(t, repo)
		defer server.Close()
		h.adminGroups = []string{"admins"}

		user, err := h.findOrCreateUser(context.Background(), "sub-1", "admin@example.com", "Admin", []string{"users"})
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
		h, server := newTestHandler(t, repo)
		defer server.Close()
		// adminGroups is nil (feature disabled)

		user, err := h.findOrCreateUser(context.Background(), "sub-1", "admin@example.com", "Admin", []string{"users"})
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

	t.Run("email-linked user gets admin synced", func(t *testing.T) {
		var updatedUser *model.User
		existingUser := &model.User{ID: 2, Email: "user@example.com", IsAdmin: false}
		repo := &mockUserRepo{
			findByEmailFn: func(ctx context.Context, email string) (*model.User, error) {
				return existingUser, nil
			},
			updateFn: func(ctx context.Context, user *model.User) error {
				updatedUser = user
				return nil
			},
		}
		h, server := newTestHandler(t, repo)
		defer server.Close()
		h.adminGroups = []string{"admins"}

		user, err := h.findOrCreateUser(context.Background(), "new-sub", "user@example.com", "User", []string{"admins"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !user.IsAdmin {
			t.Error("expected email-linked user to get admin from groups")
		}
		if updatedUser == nil || !updatedUser.IsAdmin {
			t.Error("expected UpdateUser with IsAdmin=true")
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
