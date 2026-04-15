package authmiddleware

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/sessions"

	"github.com/c-premus/documcp/internal/auth/oauth"
	"github.com/c-premus/documcp/internal/config"
	"github.com/c-premus/documcp/internal/model"
)

// ---------------------------------------------------------------------------
// Mock OAuth repository
// ---------------------------------------------------------------------------

type mockOAuthRepo struct {
	// Clients
	createClientFn         func(ctx context.Context, client *model.OAuthClient) error
	findClientByClientIDFn func(ctx context.Context, clientID string) (*model.OAuthClient, error)
	findClientByIDFn       func(ctx context.Context, id int64) (*model.OAuthClient, error)
	touchClientLastUsedFn  func(ctx context.Context, clientID int64) error
	updateClientScopeFn    func(ctx context.Context, clientID int64, scope string) error
	// Auth Codes
	createAuthorizationCodeFn     func(ctx context.Context, code *model.OAuthAuthorizationCode) error
	findAuthorizationCodeByCodeFn func(ctx context.Context, codeHash string) (*model.OAuthAuthorizationCode, error)
	revokeAuthorizationCodeFn     func(ctx context.Context, id int64) error
	// Access Tokens
	createAccessTokenFn      func(ctx context.Context, token *model.OAuthAccessToken) error
	findAccessTokenByIDFn    func(ctx context.Context, id int64) (*model.OAuthAccessToken, error)
	findAccessTokenByTokenFn func(ctx context.Context, tokenHash string) (*model.OAuthAccessToken, error)
	revokeAccessTokenFn      func(ctx context.Context, id int64) error
	// Refresh Tokens
	createRefreshTokenFn                func(ctx context.Context, token *model.OAuthRefreshToken) error
	findRefreshTokenByTokenFn           func(ctx context.Context, tokenHash string) (*model.OAuthRefreshToken, error)
	revokeRefreshTokenFn                func(ctx context.Context, id int64) error
	revokeRefreshTokenByAccessTokenIDFn func(ctx context.Context, accessTokenID int64) error
	// Device Codes
	createDeviceCodeFn           func(ctx context.Context, dc *model.OAuthDeviceCode) error
	findDeviceCodeByDeviceCodeFn func(ctx context.Context, deviceCodeHash string) (*model.OAuthDeviceCode, error)
	findDeviceCodeByUserCodeFn   func(ctx context.Context, userCode string) (*model.OAuthDeviceCode, error)
	updateDeviceCodeStatusFn     func(ctx context.Context, id int64, status model.DeviceCodeStatus, userID *int64) error
	updateDeviceCodeLastPolledFn func(ctx context.Context, id int64, interval int) error
	// Scope Grants
	upsertScopeGrantFn      func(ctx context.Context, grant *model.OAuthClientScopeGrant) error
	findActiveScopeGrantsFn func(ctx context.Context, clientID int64) ([]model.OAuthClientScopeGrant, error)
	// Users
	findUserByIDFn func(ctx context.Context, id int64) (*model.User, error)
}

func (m *mockOAuthRepo) CreateClient(ctx context.Context, client *model.OAuthClient) error {
	if m.createClientFn != nil {
		return m.createClientFn(ctx, client)
	}
	return nil
}

func (m *mockOAuthRepo) FindClientByClientID(ctx context.Context, clientID string) (*model.OAuthClient, error) {
	if m.findClientByClientIDFn != nil {
		return m.findClientByClientIDFn(ctx, clientID)
	}
	return nil, sql.ErrNoRows
}

func (m *mockOAuthRepo) FindClientByID(ctx context.Context, id int64) (*model.OAuthClient, error) {
	if m.findClientByIDFn != nil {
		return m.findClientByIDFn(ctx, id)
	}
	return nil, sql.ErrNoRows
}

func (m *mockOAuthRepo) TouchClientLastUsed(ctx context.Context, clientID int64) error {
	if m.touchClientLastUsedFn != nil {
		return m.touchClientLastUsedFn(ctx, clientID)
	}
	return nil
}

func (m *mockOAuthRepo) UpdateClientScope(ctx context.Context, clientID int64, scope string) error {
	if m.updateClientScopeFn != nil {
		return m.updateClientScopeFn(ctx, clientID, scope)
	}
	return nil
}

func (m *mockOAuthRepo) CreateAuthorizationCode(ctx context.Context, code *model.OAuthAuthorizationCode) error {
	if m.createAuthorizationCodeFn != nil {
		return m.createAuthorizationCodeFn(ctx, code)
	}
	return nil
}

func (m *mockOAuthRepo) FindAuthorizationCodeByCode(ctx context.Context, codeHash string) (*model.OAuthAuthorizationCode, error) {
	if m.findAuthorizationCodeByCodeFn != nil {
		return m.findAuthorizationCodeByCodeFn(ctx, codeHash)
	}
	return nil, sql.ErrNoRows
}

func (m *mockOAuthRepo) RevokeAuthorizationCode(ctx context.Context, id int64) error {
	if m.revokeAuthorizationCodeFn != nil {
		return m.revokeAuthorizationCodeFn(ctx, id)
	}
	return nil
}

func (m *mockOAuthRepo) CreateAccessToken(ctx context.Context, token *model.OAuthAccessToken) error {
	if m.createAccessTokenFn != nil {
		return m.createAccessTokenFn(ctx, token)
	}
	return nil
}

func (m *mockOAuthRepo) FindAccessTokenByID(ctx context.Context, id int64) (*model.OAuthAccessToken, error) {
	if m.findAccessTokenByIDFn != nil {
		return m.findAccessTokenByIDFn(ctx, id)
	}
	return nil, sql.ErrNoRows
}

func (m *mockOAuthRepo) FindAccessTokenByToken(ctx context.Context, tokenHash string) (*model.OAuthAccessToken, error) {
	if m.findAccessTokenByTokenFn != nil {
		return m.findAccessTokenByTokenFn(ctx, tokenHash)
	}
	return nil, sql.ErrNoRows
}

func (m *mockOAuthRepo) RevokeAccessToken(ctx context.Context, id int64) error {
	if m.revokeAccessTokenFn != nil {
		return m.revokeAccessTokenFn(ctx, id)
	}
	return nil
}

func (m *mockOAuthRepo) RevokeTokenPair(_ context.Context, _, _ int64) error {
	return nil
}

func (m *mockOAuthRepo) FindAuthorizationCodeByCodeIncludingRevoked(_ context.Context, _ string) (*model.OAuthAuthorizationCode, error) {
	return nil, sql.ErrNoRows
}

func (m *mockOAuthRepo) RevokeTokenFamilyByAuthorizationCodeID(_ context.Context, _ int64) (int64, error) {
	return 0, nil
}

func (m *mockOAuthRepo) FindRefreshTokenByTokenIgnoringRevocation(_ context.Context, _ string) (*model.OAuthRefreshToken, error) {
	return nil, sql.ErrNoRows
}

func (m *mockOAuthRepo) CreateRefreshToken(ctx context.Context, token *model.OAuthRefreshToken) error {
	if m.createRefreshTokenFn != nil {
		return m.createRefreshTokenFn(ctx, token)
	}
	return nil
}

func (m *mockOAuthRepo) FindRefreshTokenByToken(ctx context.Context, tokenHash string) (*model.OAuthRefreshToken, error) {
	if m.findRefreshTokenByTokenFn != nil {
		return m.findRefreshTokenByTokenFn(ctx, tokenHash)
	}
	return nil, sql.ErrNoRows
}

func (m *mockOAuthRepo) RevokeRefreshToken(ctx context.Context, id int64) error {
	if m.revokeRefreshTokenFn != nil {
		return m.revokeRefreshTokenFn(ctx, id)
	}
	return nil
}

func (m *mockOAuthRepo) RevokeRefreshTokenByAccessTokenID(ctx context.Context, accessTokenID int64) error {
	if m.revokeRefreshTokenByAccessTokenIDFn != nil {
		return m.revokeRefreshTokenByAccessTokenIDFn(ctx, accessTokenID)
	}
	return nil
}

func (m *mockOAuthRepo) CreateDeviceCode(ctx context.Context, dc *model.OAuthDeviceCode) error {
	if m.createDeviceCodeFn != nil {
		return m.createDeviceCodeFn(ctx, dc)
	}
	return nil
}

func (m *mockOAuthRepo) FindDeviceCodeByDeviceCode(ctx context.Context, deviceCodeHash string) (*model.OAuthDeviceCode, error) {
	if m.findDeviceCodeByDeviceCodeFn != nil {
		return m.findDeviceCodeByDeviceCodeFn(ctx, deviceCodeHash)
	}
	return nil, sql.ErrNoRows
}

func (m *mockOAuthRepo) FindDeviceCodeByUserCode(ctx context.Context, userCode string) (*model.OAuthDeviceCode, error) {
	if m.findDeviceCodeByUserCodeFn != nil {
		return m.findDeviceCodeByUserCodeFn(ctx, userCode)
	}
	return nil, sql.ErrNoRows
}

func (m *mockOAuthRepo) UpdateDeviceCodeStatus(ctx context.Context, id int64, status model.DeviceCodeStatus, userID *int64) error {
	if m.updateDeviceCodeStatusFn != nil {
		return m.updateDeviceCodeStatusFn(ctx, id, status, userID)
	}
	return nil
}

func (m *mockOAuthRepo) UpdateDeviceCodeStatusAndScope(_ context.Context, _ int64, _ model.DeviceCodeStatus, _ *int64, _ string) error {
	return nil
}

func (m *mockOAuthRepo) ExchangeDeviceCodeStatus(_ context.Context, _ int64) error {
	return nil
}

func (m *mockOAuthRepo) UpdateDeviceCodeLastPolled(ctx context.Context, id int64, interval int) error {
	if m.updateDeviceCodeLastPolledFn != nil {
		return m.updateDeviceCodeLastPolledFn(ctx, id, interval)
	}
	return nil
}

func (m *mockOAuthRepo) UpsertScopeGrant(ctx context.Context, grant *model.OAuthClientScopeGrant) error {
	if m.upsertScopeGrantFn != nil {
		return m.upsertScopeGrantFn(ctx, grant)
	}
	return nil
}

func (m *mockOAuthRepo) FindActiveScopeGrants(ctx context.Context, clientID int64) ([]model.OAuthClientScopeGrant, error) {
	if m.findActiveScopeGrantsFn != nil {
		return m.findActiveScopeGrantsFn(ctx, clientID)
	}
	return nil, nil
}

func (m *mockOAuthRepo) FindUserByID(ctx context.Context, id int64) (*model.User, error) {
	if m.findUserByIDFn != nil {
		return m.findUserByIDFn(ctx, id)
	}
	return nil, sql.ErrNoRows
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newTestOAuthService(repo *mockOAuthRepo) *oauth.Service {
	cfg := config.OAuthConfig{
		AuthCodeLifetime:     10 * time.Minute,
		AccessTokenLifetime:  1 * time.Hour,
		RefreshTokenLifetime: 30 * 24 * time.Hour,
		DeviceCodeLifetime:   10 * time.Minute,
		DeviceCodeInterval:   5 * time.Second,
	}
	return oauth.NewService(repo, cfg, "https://example.com", nil)
}

// okHandler is a simple handler that writes 200 OK. Used as the "next"
// handler in middleware chains.
func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})
}

func decodeJSONBody(t *testing.T, body io.Reader) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.NewDecoder(body).Decode(&result); err != nil {
		t.Fatalf("decoding JSON response: %v", err)
	}
	return result
}

// ---------------------------------------------------------------------------
// BearerToken middleware tests
// ---------------------------------------------------------------------------

func TestBearerToken(t *testing.T) {
	t.Parallel()

	t.Run("rejects request with no Authorization header", func(t *testing.T) {
		t.Parallel()

		svc := newTestOAuthService(&mockOAuthRepo{})
		middleware := BearerToken(svc, slog.Default())
		handler := middleware(okHandler())

		req := httptest.NewRequest(http.MethodGet, "/api/resource", http.NoBody)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
		}
		body := decodeJSONBody(t, rr.Body)
		if body["message"] != "Bearer token required" {
			t.Errorf("message = %v, want 'Bearer token required'", body["message"])
		}
	})

	t.Run("rejects request with non-Bearer auth scheme", func(t *testing.T) {
		t.Parallel()

		svc := newTestOAuthService(&mockOAuthRepo{})
		middleware := BearerToken(svc, slog.Default())
		handler := middleware(okHandler())

		req := httptest.NewRequest(http.MethodGet, "/api/resource", http.NoBody)
		req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
		}
		body := decodeJSONBody(t, rr.Body)
		if body["message"] != "Bearer token required" {
			t.Errorf("message = %v, want 'Bearer token required'", body["message"])
		}
	})

	t.Run("rejects request with empty Bearer value", func(t *testing.T) {
		t.Parallel()

		svc := newTestOAuthService(&mockOAuthRepo{})
		middleware := BearerToken(svc, slog.Default())
		handler := middleware(okHandler())

		req := httptest.NewRequest(http.MethodGet, "/api/resource", http.NoBody)
		req.Header.Set("Authorization", "Bearer ")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		// Empty token after "Bearer " will fail token validation
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
		}
	})

	t.Run("rejects invalid token format", func(t *testing.T) {
		t.Parallel()

		svc := newTestOAuthService(&mockOAuthRepo{})
		middleware := BearerToken(svc, slog.Default())
		handler := middleware(okHandler())

		req := httptest.NewRequest(http.MethodGet, "/api/resource", http.NoBody)
		req.Header.Set("Authorization", "Bearer invalid-no-pipe-separator")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
		}
	})

	t.Run("rejects token not found in repository", func(t *testing.T) {
		t.Parallel()

		repo := &mockOAuthRepo{
			findAccessTokenByTokenFn: func(_ context.Context, _ string) (*model.OAuthAccessToken, error) {
				return nil, errors.New("not found")
			},
		}
		svc := newTestOAuthService(repo)
		middleware := BearerToken(svc, slog.Default())
		handler := middleware(okHandler())

		req := httptest.NewRequest(http.MethodGet, "/api/resource", http.NoBody)
		req.Header.Set("Authorization", "Bearer 1|"+strings.Repeat("a", 64))
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
		}
	})

	t.Run("accepts valid token and sets access token in context", func(t *testing.T) {
		t.Parallel()

		// Generate a real token pair to get a consistent hash
		tokenPair, err := oauth.GenerateToken()
		if err != nil {
			t.Fatalf("GenerateToken: %v", err)
		}
		tokenPair.SetID(42)

		accessToken := &model.OAuthAccessToken{
			ID:        42,
			Token:     tokenPair.Hash,
			ClientID:  1,
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}

		repo := &mockOAuthRepo{
			findAccessTokenByTokenFn: func(_ context.Context, hash string) (*model.OAuthAccessToken, error) {
				if hash == tokenPair.Hash {
					return accessToken, nil
				}
				return nil, errors.New("not found")
			},
		}
		svc := newTestOAuthService(repo)
		middleware := BearerToken(svc, slog.Default())

		var capturedToken *model.OAuthAccessToken
		inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			tok, ok := r.Context().Value(AccessTokenContextKey).(*model.OAuthAccessToken)
			if ok {
				capturedToken = tok
			}
		})
		handler := middleware(inner)

		req := httptest.NewRequest(http.MethodGet, "/api/resource", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+tokenPair.Plaintext)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		if capturedToken == nil {
			t.Fatal("access token not set in context")
		}
		if capturedToken.ID != 42 {
			t.Errorf("access token ID = %d, want 42", capturedToken.ID)
		}
	})

	t.Run("loads user into context when token has user ID", func(t *testing.T) {
		t.Parallel()

		tokenPair, err := oauth.GenerateToken()
		if err != nil {
			t.Fatalf("GenerateToken: %v", err)
		}
		tokenPair.SetID(10)

		accessToken := &model.OAuthAccessToken{
			ID:        10,
			Token:     tokenPair.Hash,
			ClientID:  1,
			UserID:    sql.NullInt64{Int64: 99, Valid: true},
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}

		user := &model.User{
			ID:      99,
			Name:    "Test User",
			Email:   "test@example.com",
			IsAdmin: false,
		}

		repo := &mockOAuthRepo{
			findAccessTokenByTokenFn: func(_ context.Context, hash string) (*model.OAuthAccessToken, error) {
				if hash == tokenPair.Hash {
					return accessToken, nil
				}
				return nil, errors.New("not found")
			},
			findUserByIDFn: func(_ context.Context, id int64) (*model.User, error) {
				if id == 99 {
					return user, nil
				}
				return nil, sql.ErrNoRows
			},
		}
		svc := newTestOAuthService(repo)
		middleware := BearerToken(svc, slog.Default())

		var capturedUser *model.User
		inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			u, ok := UserFromContext(r.Context())
			if ok {
				capturedUser = u
			}
		})
		handler := middleware(inner)

		req := httptest.NewRequest(http.MethodGet, "/api/resource", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+tokenPair.Plaintext)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		if capturedUser == nil {
			t.Fatal("user not set in context")
		}
		if capturedUser.ID != 99 {
			t.Errorf("user ID = %d, want 99", capturedUser.ID)
		}
		if capturedUser.Name != "Test User" {
			t.Errorf("user Name = %q, want 'Test User'", capturedUser.Name)
		}
	})

	t.Run("response content type is JSON on error", func(t *testing.T) {
		t.Parallel()

		svc := newTestOAuthService(&mockOAuthRepo{})
		middleware := BearerToken(svc, slog.Default())
		handler := middleware(okHandler())

		req := httptest.NewRequest(http.MethodGet, "/api/resource", http.NoBody)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
	})
}

// ---------------------------------------------------------------------------
// SessionAuth middleware tests
// ---------------------------------------------------------------------------

func TestSessionAuth(t *testing.T) {
	t.Parallel()

	t.Run("redirects when no session exists", func(t *testing.T) {
		t.Parallel()

		store := sessions.NewCookieStore([]byte("test-secret-key-for-session"))
		svc := newTestOAuthService(&mockOAuthRepo{})
		middleware := SessionAuth(store, svc, slog.Default())
		handler := middleware(okHandler())

		req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", http.NoBody)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusFound {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusFound)
		}
		location := rr.Header().Get("Location")
		if !strings.Contains(location, "/auth/login") {
			t.Errorf("Location = %q, want it to contain /auth/login", location)
		}
	})

	t.Run("redirects when session has no user_id", func(t *testing.T) {
		t.Parallel()

		store := sessions.NewCookieStore([]byte("test-secret-key-for-session"))
		svc := newTestOAuthService(&mockOAuthRepo{})
		middleware := SessionAuth(store, svc, slog.Default())
		handler := middleware(okHandler())

		// Create a request with a session that has no user_id
		req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", http.NoBody)
		rr := httptest.NewRecorder()

		// Set a session cookie with no user_id
		session, _ := store.Get(req, "documcp_session")
		session.Values["some_other_key"] = "value"
		_ = session.Save(req, rr)

		// Copy cookies from response to new request
		req2 := httptest.NewRequest(http.MethodGet, "/admin/dashboard", http.NoBody)
		for _, cookie := range rr.Result().Cookies() {
			req2.AddCookie(cookie)
		}
		rr2 := httptest.NewRecorder()

		handler.ServeHTTP(rr2, req2)

		if rr2.Code != http.StatusFound {
			t.Errorf("status = %d, want %d", rr2.Code, http.StatusFound)
		}
	})

	t.Run("redirect URL includes the original request URI", func(t *testing.T) {
		t.Parallel()

		store := sessions.NewCookieStore([]byte("test-secret-key-for-session"))
		svc := newTestOAuthService(&mockOAuthRepo{})
		middleware := SessionAuth(store, svc, slog.Default())
		handler := middleware(okHandler())

		req := httptest.NewRequest(http.MethodGet, "/admin/settings?tab=oauth", http.NoBody)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		location := rr.Header().Get("Location")
		// The redirect value is now URL-encoded to prevent open redirect.
		if !strings.Contains(location, "redirect=%2Fadmin%2Fsettings") {
			t.Errorf("Location = %q, want it to contain URL-encoded redirect parameter", location)
		}
	})

	t.Run("passes through and sets user when session is valid", func(t *testing.T) {
		t.Parallel()

		store := sessions.NewCookieStore([]byte("test-secret-key-for-session"))
		user := &model.User{ID: 5, Name: "Admin", Email: "admin@example.com", IsAdmin: true}
		repo := &mockOAuthRepo{
			findUserByIDFn: func(_ context.Context, id int64) (*model.User, error) {
				if id == 5 {
					return user, nil
				}
				return nil, sql.ErrNoRows
			},
		}
		svc := newTestOAuthService(repo)
		middleware := SessionAuth(store, svc, slog.Default())

		var capturedUser *model.User
		inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			u, ok := UserFromContext(r.Context())
			if ok {
				capturedUser = u
			}
		})
		handler := middleware(inner)

		// Create a valid session
		req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", http.NoBody)
		rr := httptest.NewRecorder()
		session, _ := store.Get(req, "documcp_session")
		session.Values["user_id"] = int64(5)
		_ = session.Save(req, rr)

		// Build request with session cookie
		req2 := httptest.NewRequest(http.MethodGet, "/admin/dashboard", http.NoBody)
		for _, cookie := range rr.Result().Cookies() {
			req2.AddCookie(cookie)
		}
		rr2 := httptest.NewRecorder()

		handler.ServeHTTP(rr2, req2)

		if rr2.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr2.Code, http.StatusOK)
		}
		if capturedUser == nil {
			t.Fatal("user not set in context")
		}
		if capturedUser.ID != 5 {
			t.Errorf("user ID = %d, want 5", capturedUser.ID)
		}
	})

	t.Run("redirects and clears session when user not found", func(t *testing.T) {
		t.Parallel()

		store := sessions.NewCookieStore([]byte("test-secret-key-for-session"))
		repo := &mockOAuthRepo{
			findUserByIDFn: func(_ context.Context, _ int64) (*model.User, error) {
				return nil, errors.New("user deleted")
			},
		}
		svc := newTestOAuthService(repo)
		middleware := SessionAuth(store, svc, slog.Default())
		handler := middleware(okHandler())

		// Create a valid session with a user_id
		req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", http.NoBody)
		rr := httptest.NewRecorder()
		session, _ := store.Get(req, "documcp_session")
		session.Values["user_id"] = int64(999)
		_ = session.Save(req, rr)

		// Build request with session cookie
		req2 := httptest.NewRequest(http.MethodGet, "/admin/dashboard", http.NoBody)
		for _, cookie := range rr.Result().Cookies() {
			req2.AddCookie(cookie)
		}
		rr2 := httptest.NewRecorder()

		handler.ServeHTTP(rr2, req2)

		if rr2.Code != http.StatusFound {
			t.Errorf("status = %d, want %d", rr2.Code, http.StatusFound)
		}
	})
}

// ---------------------------------------------------------------------------
// RequireAdmin middleware tests
// ---------------------------------------------------------------------------

func TestRequireAdmin(t *testing.T) {
	t.Parallel()

	t.Run("allows admin user through", func(t *testing.T) {
		t.Parallel()

		user := &model.User{ID: 1, Name: "Admin", IsAdmin: true}
		ctx := context.WithValue(context.Background(), UserContextKey, user)

		req := httptest.NewRequest(http.MethodGet, "/admin/resource", http.NoBody)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		handler := RequireAdmin(okHandler())
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})

	t.Run("rejects non-admin user with consistent JSON error format", func(t *testing.T) {
		t.Parallel()

		user := &model.User{ID: 2, Name: "Regular User", IsAdmin: false}
		ctx := context.WithValue(context.Background(), UserContextKey, user)

		req := httptest.NewRequest(http.MethodGet, "/admin/resource", http.NoBody)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		handler := RequireAdmin(okHandler())
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusForbidden)
		}

		if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding JSON: %v", err)
		}
		if body["error"] != "Forbidden" {
			t.Errorf("error = %v, want 'Forbidden'", body["error"])
		}
		if body["message"] != "Admin privileges required." {
			t.Errorf("message = %v, want 'Admin privileges required.'", body["message"])
		}
	})

	t.Run("rejects request with no user in context", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/admin/resource", http.NoBody)
		rr := httptest.NewRecorder()

		handler := RequireAdmin(okHandler())
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusForbidden)
		}
	})

	t.Run("rejects request with wrong type value in context", func(t *testing.T) {
		t.Parallel()

		// If a non-*model.User value is stored under UserContextKey, the type
		// assertion fails and ok is false, so RequireAdmin should reject.
		ctx := context.WithValue(context.Background(), UserContextKey, "not a user")

		req := httptest.NewRequest(http.MethodGet, "/admin/resource", http.NoBody)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		handler := RequireAdmin(okHandler())
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusForbidden)
		}
	})
}

// ---------------------------------------------------------------------------
// RequireScope middleware tests
// ---------------------------------------------------------------------------

func TestRequireScope(t *testing.T) {
	t.Parallel()

	t.Run("allows token with matching scope", func(t *testing.T) {
		t.Parallel()

		token := &model.OAuthAccessToken{
			ID:    1,
			Scope: sql.NullString{String: "mcp:access mcp:read", Valid: true},
		}
		ctx := context.WithValue(context.Background(), AccessTokenContextKey, token)

		req := httptest.NewRequest(http.MethodGet, "/documcp", http.NoBody)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		handler := RequireScope("mcp:access", slog.Default())(okHandler())
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})

	t.Run("rejects token with missing scope", func(t *testing.T) {
		t.Parallel()

		token := &model.OAuthAccessToken{
			ID:    2,
			Scope: sql.NullString{String: "mcp:read", Valid: true},
		}
		ctx := context.WithValue(context.Background(), AccessTokenContextKey, token)

		req := httptest.NewRequest(http.MethodGet, "/documcp", http.NoBody)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		handler := RequireScope("mcp:access", slog.Default())(okHandler())
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusForbidden)
		}
		if wwwAuth := rr.Header().Get("WWW-Authenticate"); !strings.Contains(wwwAuth, "insufficient_scope") {
			t.Errorf("WWW-Authenticate = %q, want it to contain insufficient_scope", wwwAuth)
		}
	})

	t.Run("rejects token with null scope", func(t *testing.T) {
		t.Parallel()

		token := &model.OAuthAccessToken{
			ID:    3,
			Scope: sql.NullString{Valid: false},
		}
		ctx := context.WithValue(context.Background(), AccessTokenContextKey, token)

		req := httptest.NewRequest(http.MethodGet, "/documcp", http.NoBody)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		handler := RequireScope("mcp:access", slog.Default())(okHandler())
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusForbidden)
		}
		if wwwAuth := rr.Header().Get("WWW-Authenticate"); !strings.Contains(wwwAuth, "insufficient_scope") {
			t.Errorf("WWW-Authenticate = %q, want it to contain insufficient_scope", wwwAuth)
		}
	})

	t.Run("rejects token with empty scope string", func(t *testing.T) {
		t.Parallel()

		token := &model.OAuthAccessToken{
			ID:    4,
			Scope: sql.NullString{String: "", Valid: true},
		}
		ctx := context.WithValue(context.Background(), AccessTokenContextKey, token)

		req := httptest.NewRequest(http.MethodGet, "/documcp", http.NoBody)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		handler := RequireScope("mcp:access", slog.Default())(okHandler())
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusForbidden)
		}
		if wwwAuth := rr.Header().Get("WWW-Authenticate"); !strings.Contains(wwwAuth, "insufficient_scope") {
			t.Errorf("WWW-Authenticate = %q, want it to contain insufficient_scope", wwwAuth)
		}
	})

	t.Run("rejects request with no token in context", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/documcp", http.NoBody)
		rr := httptest.NewRecorder()

		handler := RequireScope("mcp:access", slog.Default())(okHandler())
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
		}
	})
}

// ---------------------------------------------------------------------------
// UserFromContext tests
// ---------------------------------------------------------------------------

func TestUserFromContext(t *testing.T) {
	t.Parallel()

	t.Run("returns user when present", func(t *testing.T) {
		t.Parallel()

		user := &model.User{ID: 42, Name: "Alice"}
		ctx := context.WithValue(context.Background(), UserContextKey, user)

		got, ok := UserFromContext(ctx)
		if !ok {
			t.Fatal("UserFromContext returned false")
		}
		if got.ID != 42 {
			t.Errorf("user ID = %d, want 42", got.ID)
		}
	})

	t.Run("returns false when user not in context", func(t *testing.T) {
		t.Parallel()

		_, ok := UserFromContext(context.Background())
		if ok {
			t.Error("UserFromContext returned true for empty context")
		}
	})

	t.Run("returns false for wrong type in context", func(t *testing.T) {
		t.Parallel()

		ctx := context.WithValue(context.Background(), UserContextKey, "not a user")
		_, ok := UserFromContext(ctx)
		if ok {
			t.Error("UserFromContext returned true for wrong type")
		}
	})
}

// ---------------------------------------------------------------------------
// BearerOrSession middleware tests
// ---------------------------------------------------------------------------

func TestBearerOrSession(t *testing.T) {
	t.Parallel()

	// sessionCookie is a helper that creates a signed session cookie with the
	// given key/value pairs set in the "documcp_session" session. It returns
	// the cookies that should be added to subsequent requests.
	sessionCookie := func(t *testing.T, store sessions.Store, vals map[string]any) []*http.Cookie {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		rr := httptest.NewRecorder()
		session, _ := store.Get(req, "documcp_session")
		for k, v := range vals {
			session.Values[k] = v
		}
		if err := session.Save(req, rr); err != nil {
			t.Fatalf("saving session: %v", err)
		}
		return rr.Result().Cookies()
	}

	t.Run("rejects when no Authorization header and no session cookie", func(t *testing.T) {
		t.Parallel()

		store := sessions.NewCookieStore([]byte("test-secret"))
		svc := newTestOAuthService(&mockOAuthRepo{})
		middleware := BearerOrSession(svc, store, slog.Default())
		handler := middleware(okHandler())

		req := httptest.NewRequest(http.MethodGet, "/api/resource", http.NoBody)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
		}
		body := decodeJSONBody(t, rr.Body)
		if body["message"] != "Authentication required" {
			t.Errorf("message = %v, want 'Authentication required'", body["message"])
		}
	})

	t.Run("rejects when session has invalid user_id type", func(t *testing.T) {
		t.Parallel()

		store := sessions.NewCookieStore([]byte("test-secret"))
		svc := newTestOAuthService(&mockOAuthRepo{})
		middleware := BearerOrSession(svc, store, slog.Default())
		handler := middleware(okHandler())

		// user_id is a string instead of int64 — type assertion will fail
		cookies := sessionCookie(t, store, map[string]any{"user_id": "not-an-int"})
		req := httptest.NewRequest(http.MethodGet, "/api/resource", http.NoBody)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
		}
	})

	t.Run("rejects when session has user_id zero", func(t *testing.T) {
		t.Parallel()

		store := sessions.NewCookieStore([]byte("test-secret"))
		svc := newTestOAuthService(&mockOAuthRepo{})
		middleware := BearerOrSession(svc, store, slog.Default())
		handler := middleware(okHandler())

		cookies := sessionCookie(t, store, map[string]any{"user_id": int64(0)})
		req := httptest.NewRequest(http.MethodGet, "/api/resource", http.NoBody)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
		}
	})

	t.Run("passes through with user in context when session is valid", func(t *testing.T) {
		t.Parallel()

		store := sessions.NewCookieStore([]byte("test-secret"))
		user := &model.User{ID: 7, Name: "Session User", Email: "session@example.com"}
		repo := &mockOAuthRepo{
			findUserByIDFn: func(_ context.Context, id int64) (*model.User, error) {
				if id == 7 {
					return user, nil
				}
				return nil, sql.ErrNoRows
			},
		}
		svc := newTestOAuthService(repo)
		middleware := BearerOrSession(svc, store, slog.Default())

		var capturedUser *model.User
		inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			u, ok := UserFromContext(r.Context())
			if ok {
				capturedUser = u
			}
		})
		handler := middleware(inner)

		cookies := sessionCookie(t, store, map[string]any{"user_id": int64(7)})
		req := httptest.NewRequest(http.MethodGet, "/api/resource", http.NoBody)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		if capturedUser == nil {
			t.Fatal("user not set in context")
		}
		if capturedUser.ID != 7 {
			t.Errorf("user ID = %d, want 7", capturedUser.ID)
		}
		if capturedUser.Name != "Session User" {
			t.Errorf("user Name = %q, want 'Session User'", capturedUser.Name)
		}
	})

	t.Run("rejects when session is valid but FindUserByID fails", func(t *testing.T) {
		t.Parallel()

		store := sessions.NewCookieStore([]byte("test-secret"))
		repo := &mockOAuthRepo{
			findUserByIDFn: func(_ context.Context, _ int64) (*model.User, error) {
				return nil, errors.New("user deleted")
			},
		}
		svc := newTestOAuthService(repo)
		middleware := BearerOrSession(svc, store, slog.Default())
		handler := middleware(okHandler())

		cookies := sessionCookie(t, store, map[string]any{"user_id": int64(42)})
		req := httptest.NewRequest(http.MethodGet, "/api/resource", http.NoBody)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
		}
		body := decodeJSONBody(t, rr.Body)
		if body["message"] != "Authentication required" {
			t.Errorf("message = %v, want 'Authentication required'", body["message"])
		}
	})

	t.Run("rejects Authorization header without Bearer prefix", func(t *testing.T) {
		t.Parallel()

		store := sessions.NewCookieStore([]byte("test-secret"))
		svc := newTestOAuthService(&mockOAuthRepo{})
		middleware := BearerOrSession(svc, store, slog.Default())
		handler := middleware(okHandler())

		req := httptest.NewRequest(http.MethodGet, "/api/resource", http.NoBody)
		req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
		}
		body := decodeJSONBody(t, rr.Body)
		if body["message"] != "Bearer token required" {
			t.Errorf("message = %v, want 'Bearer token required'", body["message"])
		}
		if wwwAuth := rr.Header().Get("WWW-Authenticate"); wwwAuth != "Bearer" {
			t.Errorf("WWW-Authenticate = %q, want 'Bearer'", wwwAuth)
		}
	})

	t.Run("accepts valid bearer token and sets access token in context", func(t *testing.T) {
		t.Parallel()

		store := sessions.NewCookieStore([]byte("test-secret"))

		tokenPair, err := oauth.GenerateToken()
		if err != nil {
			t.Fatalf("GenerateToken: %v", err)
		}
		tokenPair.SetID(50)

		accessToken := &model.OAuthAccessToken{
			ID:        50,
			Token:     tokenPair.Hash,
			ClientID:  1,
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}

		repo := &mockOAuthRepo{
			findAccessTokenByTokenFn: func(_ context.Context, hash string) (*model.OAuthAccessToken, error) {
				if hash == tokenPair.Hash {
					return accessToken, nil
				}
				return nil, errors.New("not found")
			},
		}
		svc := newTestOAuthService(repo)
		middleware := BearerOrSession(svc, store, slog.Default())

		var capturedToken *model.OAuthAccessToken
		inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			tok, ok := r.Context().Value(AccessTokenContextKey).(*model.OAuthAccessToken)
			if ok {
				capturedToken = tok
			}
		})
		handler := middleware(inner)

		req := httptest.NewRequest(http.MethodGet, "/api/resource", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+tokenPair.Plaintext)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		if capturedToken == nil {
			t.Fatal("access token not set in context")
		}
		if capturedToken.ID != 50 {
			t.Errorf("access token ID = %d, want 50", capturedToken.ID)
		}
	})

	t.Run("loads user into context when bearer token has user_id", func(t *testing.T) {
		t.Parallel()

		store := sessions.NewCookieStore([]byte("test-secret"))

		tokenPair, err := oauth.GenerateToken()
		if err != nil {
			t.Fatalf("GenerateToken: %v", err)
		}
		tokenPair.SetID(60)

		accessToken := &model.OAuthAccessToken{
			ID:        60,
			Token:     tokenPair.Hash,
			ClientID:  1,
			UserID:    sql.NullInt64{Int64: 88, Valid: true},
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}

		user := &model.User{ID: 88, Name: "Bearer User", Email: "bearer@example.com"}

		repo := &mockOAuthRepo{
			findAccessTokenByTokenFn: func(_ context.Context, hash string) (*model.OAuthAccessToken, error) {
				if hash == tokenPair.Hash {
					return accessToken, nil
				}
				return nil, errors.New("not found")
			},
			findUserByIDFn: func(_ context.Context, id int64) (*model.User, error) {
				if id == 88 {
					return user, nil
				}
				return nil, sql.ErrNoRows
			},
		}
		svc := newTestOAuthService(repo)
		middleware := BearerOrSession(svc, store, slog.Default())

		var capturedUser *model.User
		var capturedToken *model.OAuthAccessToken
		inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			if u, ok := UserFromContext(r.Context()); ok {
				capturedUser = u
			}
			if tok, ok := r.Context().Value(AccessTokenContextKey).(*model.OAuthAccessToken); ok {
				capturedToken = tok
			}
		})
		handler := middleware(inner)

		req := httptest.NewRequest(http.MethodGet, "/api/resource", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+tokenPair.Plaintext)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		if capturedToken == nil {
			t.Fatal("access token not set in context")
		}
		if capturedToken.ID != 60 {
			t.Errorf("access token ID = %d, want 60", capturedToken.ID)
		}
		if capturedUser == nil {
			t.Fatal("user not set in context")
		}
		if capturedUser.ID != 88 {
			t.Errorf("user ID = %d, want 88", capturedUser.ID)
		}
	})

	t.Run("still succeeds when bearer token has user_id but FindUserByID fails", func(t *testing.T) {
		t.Parallel()

		store := sessions.NewCookieStore([]byte("test-secret"))

		tokenPair, err := oauth.GenerateToken()
		if err != nil {
			t.Fatalf("GenerateToken: %v", err)
		}
		tokenPair.SetID(70)

		accessToken := &model.OAuthAccessToken{
			ID:        70,
			Token:     tokenPair.Hash,
			ClientID:  1,
			UserID:    sql.NullInt64{Int64: 999, Valid: true},
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}

		repo := &mockOAuthRepo{
			findAccessTokenByTokenFn: func(_ context.Context, hash string) (*model.OAuthAccessToken, error) {
				if hash == tokenPair.Hash {
					return accessToken, nil
				}
				return nil, errors.New("not found")
			},
			findUserByIDFn: func(_ context.Context, _ int64) (*model.User, error) {
				return nil, errors.New("user not found")
			},
		}
		svc := newTestOAuthService(repo)
		middleware := BearerOrSession(svc, store, slog.Default())

		var capturedUser *model.User
		var capturedToken *model.OAuthAccessToken
		inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			if u, ok := UserFromContext(r.Context()); ok {
				capturedUser = u
			}
			if tok, ok := r.Context().Value(AccessTokenContextKey).(*model.OAuthAccessToken); ok {
				capturedToken = tok
			}
		})
		handler := middleware(inner)

		req := httptest.NewRequest(http.MethodGet, "/api/resource", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+tokenPair.Plaintext)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		// Should still succeed — user lookup failure only warns, doesn't reject
		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		if capturedToken == nil {
			t.Fatal("access token not set in context")
		}
		if capturedToken.ID != 70 {
			t.Errorf("access token ID = %d, want 70", capturedToken.ID)
		}
		// User should NOT be in context since lookup failed
		if capturedUser != nil {
			t.Error("user should not be in context when FindUserByID fails")
		}
	})

	t.Run("rejects invalid bearer token", func(t *testing.T) {
		t.Parallel()

		store := sessions.NewCookieStore([]byte("test-secret"))
		repo := &mockOAuthRepo{
			findAccessTokenByTokenFn: func(_ context.Context, _ string) (*model.OAuthAccessToken, error) {
				return nil, errors.New("not found")
			},
		}
		svc := newTestOAuthService(repo)
		middleware := BearerOrSession(svc, store, slog.Default())
		handler := middleware(okHandler())

		req := httptest.NewRequest(http.MethodGet, "/api/resource", http.NoBody)
		req.Header.Set("Authorization", "Bearer 1|"+strings.Repeat("x", 64))
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
		}
		body := decodeJSONBody(t, rr.Body)
		if body["message"] != "Invalid or expired token" {
			t.Errorf("message = %v, want 'Invalid or expired token'", body["message"])
		}
		if wwwAuth := rr.Header().Get("WWW-Authenticate"); !strings.Contains(wwwAuth, "invalid_token") {
			t.Errorf("WWW-Authenticate = %q, want it to contain 'invalid_token'", wwwAuth)
		}
	})
}

// ---------------------------------------------------------------------------
// AccessTokenContextKey tests
// ---------------------------------------------------------------------------

func TestAccessTokenContextKey(t *testing.T) {
	t.Parallel()

	t.Run("returns token when present", func(t *testing.T) {
		t.Parallel()

		token := &model.OAuthAccessToken{ID: 7, Token: "hash"}
		ctx := context.WithValue(context.Background(), AccessTokenContextKey, token)

		got, ok := ctx.Value(AccessTokenContextKey).(*model.OAuthAccessToken)
		if !ok {
			t.Fatal("access token not found in context")
		}
		if got.ID != 7 {
			t.Errorf("token ID = %d, want 7", got.ID)
		}
	})

	t.Run("returns false when token not in context", func(t *testing.T) {
		t.Parallel()

		val := context.Background().Value(AccessTokenContextKey)
		if val != nil {
			t.Error("expected nil for empty context")
		}
	})
}
