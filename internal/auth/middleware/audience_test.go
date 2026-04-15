package authmiddleware

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/c-premus/documcp/internal/auth/oauth"
	"github.com/c-premus/documcp/internal/model"
)

func newAudienceTokenAndRepo(t *testing.T, resource sql.NullString) (string, *mockOAuthRepo) {
	t.Helper()
	pair, err := oauth.GenerateToken()
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
		mw := BearerTokenWithAudience(svc, slog.Default(), expected)
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
		mw := BearerTokenWithAudience(svc, slog.Default(), expected)
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
		mw := BearerTokenWithAudience(svc, slog.Default(), expected)
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
		mw := BearerTokenWithAudience(svc, slog.Default(), expected)
		handler := mw(okHandler())

		req := httptest.NewRequest(http.MethodGet, "/documcp", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+plain)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rr.Code)
		}
	})
}
