package authmiddleware

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/sessions"

	"github.com/c-premus/documcp/internal/model"
)

func newAudienceTokenAndRepo(t *testing.T, resource sql.NullString) (string, *mockOAuthRepo) {
	t.Helper()
	pair, err := newTestOAuthService(nil).GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	pair.SetID(7)

	tok := &model.OAuthAccessToken{
		ID:        7,
		Token:     pair.Hash,
		ClientID:  1,
		Resource:  resource,
		ExpiresAt: time.Now().Add(time.Hour),
	}
	repo := &mockOAuthRepo{
		findAccessTokenByTokenFn: func(_ context.Context, hash string) (*model.OAuthAccessToken, error) {
			if hash == pair.Hash {
				return tok, nil
			}
			return nil, errors.New("not found")
		},
	}
	return pair.Plaintext, repo
}

func TestBearerTokenWithAudience(t *testing.T) {
	t.Parallel()

	const expected = "https://documcp.example.com/documcp"

	t.Run("matching resource passes", func(t *testing.T) {
		t.Parallel()
		plain, repo := newAudienceTokenAndRepo(t, sql.NullString{String: expected, Valid: true})
		svc := newTestOAuthService(repo)
		mw := BearerTokenWithAudience(svc, slog.Default(), expected, false)
		handler := mw(okHandler())

		req := httptest.NewRequest(http.MethodGet, "/documcp", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+plain)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("mismatched resource rejects with 401", func(t *testing.T) {
		t.Parallel()
		plain, repo := newAudienceTokenAndRepo(t, sql.NullString{String: "https://documcp.example.com", Valid: true})
		svc := newTestOAuthService(repo)
		mw := BearerTokenWithAudience(svc, slog.Default(), expected, false)
		handler := mw(okHandler())

		req := httptest.NewRequest(http.MethodGet, "/documcp", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+plain)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rr.Code)
		}
		if got := rr.Header().Get("WWW-Authenticate"); got != `Bearer error="invalid_token", error_description="audience mismatch"` {
			t.Errorf("WWW-Authenticate = %q", got)
		}
	})

	t.Run("NULL resource (legacy token) rejects with 401", func(t *testing.T) {
		t.Parallel()
		plain, repo := newAudienceTokenAndRepo(t, sql.NullString{Valid: false})
		svc := newTestOAuthService(repo)
		mw := BearerTokenWithAudience(svc, slog.Default(), expected, false)
		handler := mw(okHandler())

		req := httptest.NewRequest(http.MethodGet, "/documcp", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+plain)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rr.Code)
		}
	})

	t.Run("empty-string resource rejects with 401", func(t *testing.T) {
		t.Parallel()
		plain, repo := newAudienceTokenAndRepo(t, sql.NullString{String: "", Valid: true})
		svc := newTestOAuthService(repo)
		mw := BearerTokenWithAudience(svc, slog.Default(), expected, false)
		handler := mw(okHandler())

		req := httptest.NewRequest(http.MethodGet, "/documcp", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+plain)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rr.Code)
		}
	})

	// OAUTH_ACCEPT_EMPTY_RESOURCE shim coverage (issue #164).

	t.Run("empty-string resource passes when AcceptEmptyResource=true", func(t *testing.T) {
		t.Parallel()
		plain, repo := newAudienceTokenAndRepo(t, sql.NullString{String: "", Valid: true})
		svc := newTestOAuthService(repo)
		mw := BearerTokenWithAudience(svc, slog.Default(), expected, true)
		handler := mw(okHandler())

		req := httptest.NewRequest(http.MethodGet, "/documcp", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+plain)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("NULL resource passes when AcceptEmptyResource=true", func(t *testing.T) {
		t.Parallel()
		plain, repo := newAudienceTokenAndRepo(t, sql.NullString{Valid: false})
		svc := newTestOAuthService(repo)
		mw := BearerTokenWithAudience(svc, slog.Default(), expected, true)
		handler := mw(okHandler())

		req := httptest.NewRequest(http.MethodGet, "/documcp", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+plain)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("non-empty mismatch still rejects when AcceptEmptyResource=true", func(t *testing.T) {
		t.Parallel()
		plain, repo := newAudienceTokenAndRepo(t, sql.NullString{String: "https://other.example.com/", Valid: true})
		svc := newTestOAuthService(repo)
		mw := BearerTokenWithAudience(svc, slog.Default(), expected, true)
		handler := mw(okHandler())

		req := httptest.NewRequest(http.MethodGet, "/documcp", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+plain)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rr.Code)
		}
		if got := rr.Header().Get("WWW-Authenticate"); got != `Bearer error="invalid_token", error_description="audience mismatch"` {
			t.Errorf("WWW-Authenticate = %q", got)
		}
	})

	t.Run("accepted empty resource emits WARN with client_id", func(t *testing.T) {
		t.Parallel()
		plain, repo := newAudienceTokenAndRepo(t, sql.NullString{String: "", Valid: true})
		svc := newTestOAuthService(repo)

		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
		mw := BearerTokenWithAudience(svc, logger, expected, true)
		handler := mw(okHandler())

		req := httptest.NewRequest(http.MethodGet, "/documcp", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+plain)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
		}
		log := buf.String()
		if !strings.Contains(log, "OAUTH_ACCEPT_EMPTY_RESOURCE=true") {
			t.Errorf("WARN log missing accept-shim message; got: %s", log)
		}
		if !strings.Contains(log, "client_id=1") {
			t.Errorf("WARN log missing client_id=1; got: %s", log)
		}
		if !strings.Contains(log, "level=WARN") {
			t.Errorf("expected WARN-level log; got: %s", log)
		}
	})
}

// TestBearerOrSessionWithAudience_AcceptEmpty locks the contract that the
// shim applies to the REST /api surface as well — both audience-checking
// middlewares share the same checkAudience helper, but a regression test
// guards against an accidental divergence.
func TestBearerOrSessionWithAudience_AcceptEmpty(t *testing.T) {
	t.Parallel()

	const expected = "https://documcp.example.com"

	t.Run("empty resource passes when AcceptEmptyResource=true", func(t *testing.T) {
		t.Parallel()
		plain, repo := newAudienceTokenAndRepo(t, sql.NullString{String: "", Valid: true})
		svc := newTestOAuthService(repo)
		mw := BearerOrSessionWithAudience(svc, sessions.NewCookieStore([]byte("test-secret-key-for-audience")), slog.Default(), expected, 0, true)
		handler := mw(okHandler())

		req := httptest.NewRequest(http.MethodGet, "/api/documents", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+plain)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("non-empty mismatch still rejects when AcceptEmptyResource=true", func(t *testing.T) {
		t.Parallel()
		plain, repo := newAudienceTokenAndRepo(t, sql.NullString{String: "https://other.example.com/", Valid: true})
		svc := newTestOAuthService(repo)
		mw := BearerOrSessionWithAudience(svc, sessions.NewCookieStore([]byte("test-secret-key-for-audience")), slog.Default(), expected, 0, true)
		handler := mw(okHandler())

		req := httptest.NewRequest(http.MethodGet, "/api/documents", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+plain)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rr.Code)
		}
	})

	t.Run("empty resource rejects when AcceptEmptyResource=false", func(t *testing.T) {
		t.Parallel()
		plain, repo := newAudienceTokenAndRepo(t, sql.NullString{String: "", Valid: true})
		svc := newTestOAuthService(repo)
		mw := BearerOrSessionWithAudience(svc, sessions.NewCookieStore([]byte("test-secret-key-for-audience")), slog.Default(), expected, 0, false)
		handler := mw(okHandler())

		req := httptest.NewRequest(http.MethodGet, "/api/documents", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+plain)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rr.Code)
		}
	})
}
