package oauthhandler

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c-premus/documcp/internal/model"
)

func TestHandler_Authorize(t *testing.T) {
	t.Parallel()

	t.Run("returns error when response_type is missing", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?client_id=cid&redirect_uri=https://example.com/cb&state=abcdefgh", http.NoBody)
		rr := httptest.NewRecorder()

		h.Authorize(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
		assert.Contains(t, result["error_description"], "response type")
	})

	t.Run("returns error when response_type is not code", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?response_type=token&client_id=cid&redirect_uri=https://example.com/cb&state=abcdefgh", http.NoBody)
		rr := httptest.NewRecorder()

		h.Authorize(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
		assert.Contains(t, result["error_description"], "response type")
	})

	t.Run("returns error when client_id is missing", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?response_type=code&redirect_uri=https://example.com/cb&state=abcdefgh", http.NoBody)
		rr := httptest.NewRecorder()

		h.Authorize(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
		assert.Contains(t, result["error_description"], "client id")
	})

	t.Run("returns error when redirect_uri is missing", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?response_type=code&client_id=cid&state=abcdefgh", http.NoBody)
		rr := httptest.NewRecorder()

		h.Authorize(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
		assert.Contains(t, result["error_description"], "redirect uri")
	})

	t.Run("returns error when state is missing", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?response_type=code&client_id=cid&redirect_uri=https://example.com/cb", http.NoBody)
		rr := httptest.NewRecorder()

		h.Authorize(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
		assert.Contains(t, result["error_description"], "state")
	})

	t.Run("returns error when state is too short", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?response_type=code&client_id=cid&redirect_uri=https://example.com/cb&state=short", http.NoBody)
		rr := httptest.NewRecorder()

		h.Authorize(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
		assert.Contains(t, result["error_description"], "state")
	})

	t.Run("returns error when code_challenge_method is not S256", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?response_type=code&client_id=cid&redirect_uri=https://example.com/cb&state=abcdefgh&code_challenge_method=plain", http.NoBody)
		rr := httptest.NewRecorder()

		h.Authorize(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
		assert.Contains(t, result["error_description"], "code challenge method")
	})

	t.Run("redirects to login when user is not authenticated", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?response_type=code&client_id=cid&redirect_uri=https://example.com/cb&state=abcdefgh", http.NoBody)
		rr := httptest.NewRecorder()

		h.Authorize(rr, req)

		assert.Equal(t, http.StatusFound, rr.Code)
		assert.Contains(t, rr.Header().Get("Location"), "/auth/login")
	})

	t.Run("returns invalid_client when client not found", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return nil, sql.ErrNoRows
			},
		}
		h, store := newHandlerWithRepo(repo)
		store.session.Values["user_id"] = int64(42)

		req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?response_type=code&client_id=cid&redirect_uri=https://example.com/cb&state=abcdefgh", http.NoBody)
		rr := httptest.NewRecorder()

		h.Authorize(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_client", result["error"])
	})

	t.Run("returns error for invalid redirect_uri", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return &model.OAuthClient{
					ID:                      1,
					ClientID:                "cid",
					ClientName:              "Test App",
					RedirectURIs:            `["https://example.com/callback"]`,
					TokenEndpointAuthMethod: "none",
					IsActive:                true,
				}, nil
			},
		}
		h, store := newHandlerWithRepo(repo)
		store.session.Values["user_id"] = int64(42)

		// Use a redirect_uri that doesn't match the registered one
		req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?response_type=code&client_id=cid&redirect_uri=https://evil.com/steal&state=abcdefgh", http.NoBody)
		rr := httptest.NewRecorder()

		h.Authorize(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
		assert.Contains(t, result["error_description"], "redirect_uri")
	})

	t.Run("requires PKCE for public clients (token_endpoint_auth_method=none)", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return &model.OAuthClient{
					ID:                      1,
					ClientID:                "cid",
					ClientName:              "Public App",
					RedirectURIs:            `["https://example.com/cb"]`,
					TokenEndpointAuthMethod: "none",
					IsActive:                true,
				}, nil
			},
		}
		h, store := newHandlerWithRepo(repo)
		store.session.Values["user_id"] = int64(42)

		// No code_challenge for public client
		req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?response_type=code&client_id=cid&redirect_uri=https://example.com/cb&state=abcdefgh", http.NoBody)
		rr := httptest.NewRecorder()

		h.Authorize(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
		assert.Contains(t, result["error_description"], "PKCE")
	})

	t.Run("requires code_challenge_method for public clients when code_challenge present", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return &model.OAuthClient{
					ID:                      1,
					ClientID:                "cid",
					ClientName:              "Public App",
					RedirectURIs:            `["https://example.com/cb"]`,
					TokenEndpointAuthMethod: "none",
					IsActive:                true,
				}, nil
			},
		}
		h, store := newHandlerWithRepo(repo)
		store.session.Values["user_id"] = int64(42)

		req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?response_type=code&client_id=cid&redirect_uri=https://example.com/cb&state=abcdefgh&code_challenge=dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk", http.NoBody)
		rr := httptest.NewRecorder()

		h.Authorize(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Contains(t, result["error_description"], "code_challenge_method")
	})

	t.Run("happy path renders consent screen for authenticated user", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return &model.OAuthClient{
					ID:                      1,
					ClientID:                "cid",
					ClientName:              "My MCP App",
					RedirectURIs:            `["https://example.com/cb"]`,
					TokenEndpointAuthMethod: "client_secret_post",
					IsActive:                true,
				}, nil
			},
		}
		h, store := newHandlerWithRepo(repo)
		store.session.Values["user_id"] = int64(42)

		req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?response_type=code&client_id=cid&redirect_uri=https://example.com/cb&state=abcdefgh&scope=mcp:access", http.NoBody)
		rr := httptest.NewRecorder()

		h.Authorize(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Header().Get("Content-Type"), "text/html")
		body := rr.Body.String()
		assert.Contains(t, body, "My MCP App")
		assert.Contains(t, body, "mcp:access")
		assert.Contains(t, body, "nonce")
	})

	t.Run("happy path with PKCE for public client", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return &model.OAuthClient{
					ID:                      1,
					ClientID:                "cid",
					ClientName:              "Public App",
					RedirectURIs:            `["https://example.com/cb"]`,
					TokenEndpointAuthMethod: "none",
					IsActive:                true,
				}, nil
			},
		}
		h, store := newHandlerWithRepo(repo)
		store.session.Values["user_id"] = int64(42)

		req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?response_type=code&client_id=cid&redirect_uri=https://example.com/cb&state=abcdefgh&code_challenge=E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM&code_challenge_method=S256", http.NoBody)
		rr := httptest.NewRecorder()

		h.Authorize(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Header().Get("Content-Type"), "text/html")
		assert.Contains(t, rr.Body.String(), "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM")
	})

	t.Run("stores pending request in session", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return &model.OAuthClient{
					ID:                      1,
					ClientID:                "cid",
					ClientName:              "Test App",
					RedirectURIs:            `["https://example.com/cb"]`,
					TokenEndpointAuthMethod: "client_secret_post",
					IsActive:                true,
				}, nil
			},
		}
		h, store := newHandlerWithRepo(repo)
		store.session.Values["user_id"] = int64(42)

		req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?response_type=code&client_id=cid&redirect_uri=https://example.com/cb&state=abcdefgh&scope=mcp:read", http.NoBody)
		rr := httptest.NewRecorder()

		h.Authorize(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)

		pendingRaw, exists := store.session.Values["oauth_pending_request"]
		require.True(t, exists, "session should contain pending request")

		pending, ok := pendingRaw.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "cid", pending["client_id"])
		assert.Equal(t, "abcdefgh", pending["state"])
		assert.Equal(t, "https://example.com/cb", pending["redirect_uri"])
		assert.Equal(t, "mcp:read", pending["scope"])
		assert.NotEmpty(t, pending["nonce"])
		assert.NotNil(t, pending["timestamp"])
	})
}

func TestHandler_AuthorizeApprove(t *testing.T) {
	t.Parallel()

	t.Run("returns 401 when not authenticated", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		formBody := "client_id=cid&redirect_uri=https://example.com/cb&state=abcdefgh&nonce=some-nonce"
		req := httptest.NewRequest(http.MethodPost, "/oauth/authorize/approve", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.AuthorizeApprove(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("returns error when no pending OAuth request in session", func(t *testing.T) {
		t.Parallel()
		h, store := newHandlerWithRepo(&mockOAuthRepo{})
		store.session.Values["user_id"] = int64(42)

		formBody := "client_id=cid&redirect_uri=https://example.com/cb&state=abcdefgh&nonce=some-nonce"
		req := httptest.NewRequest(http.MethodPost, "/oauth/authorize/approve", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.AuthorizeApprove(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "No pending OAuth request")
	})

	t.Run("returns error on nonce mismatch", func(t *testing.T) {
		t.Parallel()
		h, store := newHandlerWithRepo(&mockOAuthRepo{})
		store.session.Values["user_id"] = int64(42)
		store.session.Values["oauth_pending_request"] = map[string]any{
			"nonce":     "correct-nonce",
			"client_id": "cid",
			"state":     "abcdefgh",
			"timestamp": time.Now().Unix(),
		}

		formBody := "client_id=cid&redirect_uri=https://example.com/cb&state=abcdefgh&nonce=wrong-nonce"
		req := httptest.NewRequest(http.MethodPost, "/oauth/authorize/approve", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.AuthorizeApprove(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invalid authorization request")
	})

	t.Run("returns error on empty nonce", func(t *testing.T) {
		t.Parallel()
		h, store := newHandlerWithRepo(&mockOAuthRepo{})
		store.session.Values["user_id"] = int64(42)
		store.session.Values["oauth_pending_request"] = map[string]any{
			"nonce":     "correct-nonce",
			"client_id": "cid",
			"state":     "abcdefgh",
			"timestamp": time.Now().Unix(),
		}

		formBody := "client_id=cid&redirect_uri=https://example.com/cb&state=abcdefgh&nonce="
		req := httptest.NewRequest(http.MethodPost, "/oauth/authorize/approve", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.AuthorizeApprove(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invalid authorization request")
	})

	t.Run("returns error on client_id mismatch", func(t *testing.T) {
		t.Parallel()
		h, store := newHandlerWithRepo(&mockOAuthRepo{})
		store.session.Values["user_id"] = int64(42)
		store.session.Values["oauth_pending_request"] = map[string]any{
			"nonce":     "correct-nonce",
			"client_id": "original-client",
			"state":     "abcdefgh",
			"timestamp": time.Now().Unix(),
		}

		formBody := "client_id=different-client&redirect_uri=https://example.com/cb&state=abcdefgh&nonce=correct-nonce"
		req := httptest.NewRequest(http.MethodPost, "/oauth/authorize/approve", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.AuthorizeApprove(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "mismatch")
	})

	t.Run("returns error on state mismatch", func(t *testing.T) {
		t.Parallel()
		h, store := newHandlerWithRepo(&mockOAuthRepo{})
		store.session.Values["user_id"] = int64(42)
		store.session.Values["oauth_pending_request"] = map[string]any{
			"nonce":     "correct-nonce",
			"client_id": "cid",
			"state":     "original-state-value",
			"timestamp": time.Now().Unix(),
		}

		formBody := "client_id=cid&redirect_uri=https://example.com/cb&state=different-state&nonce=correct-nonce"
		req := httptest.NewRequest(http.MethodPost, "/oauth/authorize/approve", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.AuthorizeApprove(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "state mismatch")
	})

	t.Run("returns error when request is expired", func(t *testing.T) {
		t.Parallel()
		h, store := newHandlerWithRepo(&mockOAuthRepo{})
		store.session.Values["user_id"] = int64(42)
		store.session.Values["oauth_pending_request"] = map[string]any{
			"nonce":     "correct-nonce",
			"client_id": "cid",
			"state":     "abcdefgh",
			"timestamp": time.Now().Add(-15 * time.Minute).Unix(), // expired (>10 min)
		}

		formBody := "client_id=cid&redirect_uri=https://example.com/cb&state=abcdefgh&nonce=correct-nonce"
		req := httptest.NewRequest(http.MethodPost, "/oauth/authorize/approve", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.AuthorizeApprove(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "expired")
	})

	t.Run("happy path redirects with code and state", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return &model.OAuthClient{
					ID:                      1,
					ClientID:                "cid",
					ClientName:              "Test App",
					TokenEndpointAuthMethod: "none",
					IsActive:                true,
				}, nil
			},
			FindClientByIDFunc: func(_ context.Context, id int64) (*model.OAuthClient, error) {
				return &model.OAuthClient{ID: id}, nil
			},
			CreateAuthorizationCodeFunc: func(_ context.Context, code *model.OAuthAuthorizationCode) error {
				code.ID = 99
				return nil
			},
		}
		h, store := newHandlerWithRepo(repo)
		store.session.Values["user_id"] = int64(42)
		store.session.Values["oauth_pending_request"] = map[string]any{
			"nonce":                 "test-nonce-value",
			"client_id":             "cid",
			"state":                 "my_state_",
			"redirect_uri":          "https://example.com/cb",
			"code_challenge":        "",
			"code_challenge_method": "",
			"scope":                 "mcp:access",
			"timestamp":             time.Now().Unix(),
		}

		formBody := "client_id=cid&redirect_uri=https://example.com/cb&state=my_state_&scope=mcp:access&nonce=test-nonce-value"
		req := httptest.NewRequest(http.MethodPost, "/oauth/authorize/approve", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.AuthorizeApprove(rr, req)

		require.Equal(t, http.StatusFound, rr.Code)
		location := rr.Header().Get("Location")
		assert.Contains(t, location, "https://example.com/cb")
		assert.Contains(t, location, "code=")
		assert.Contains(t, location, "state=my_state_")
	})

	t.Run("happy path clears pending request from session", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return &model.OAuthClient{
					ID:                      1,
					ClientID:                "cid",
					ClientName:              "Test App",
					TokenEndpointAuthMethod: "none",
					IsActive:                true,
				}, nil
			},
			CreateAuthorizationCodeFunc: func(_ context.Context, code *model.OAuthAuthorizationCode) error {
				code.ID = 99
				return nil
			},
		}
		h, store := newHandlerWithRepo(repo)
		store.session.Values["user_id"] = int64(42)
		store.session.Values["oauth_pending_request"] = map[string]any{
			"nonce":                 "test-nonce-value",
			"client_id":             "cid",
			"state":                 "my_state_",
			"redirect_uri":          "https://example.com/cb",
			"code_challenge":        "",
			"code_challenge_method": "",
			"scope":                 "",
			"timestamp":             time.Now().Unix(),
		}

		formBody := "client_id=cid&redirect_uri=https://example.com/cb&state=my_state_&nonce=test-nonce-value"
		req := httptest.NewRequest(http.MethodPost, "/oauth/authorize/approve", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.AuthorizeApprove(rr, req)

		require.Equal(t, http.StatusFound, rr.Code)
		_, exists := store.session.Values["oauth_pending_request"]
		assert.False(t, exists, "pending request should be cleared from session")
	})

	t.Run("returns 500 when finding client fails", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return nil, errors.New("db error")
			},
		}
		h, store := newHandlerWithRepo(repo)
		store.session.Values["user_id"] = int64(42)
		store.session.Values["oauth_pending_request"] = map[string]any{
			"nonce":                 "test-nonce",
			"client_id":             "cid",
			"state":                 "my_state_",
			"redirect_uri":          "https://example.com/cb",
			"code_challenge":        "",
			"code_challenge_method": "",
			"scope":                 "",
			"timestamp":             time.Now().Unix(),
		}

		formBody := "client_id=cid&redirect_uri=https://example.com/cb&state=my_state_&nonce=test-nonce"
		req := httptest.NewRequest(http.MethodPost, "/oauth/authorize/approve", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.AuthorizeApprove(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("returns 500 when generating auth code fails", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return &model.OAuthClient{
					ID:                      1,
					ClientID:                "cid",
					ClientName:              "Test App",
					TokenEndpointAuthMethod: "none",
					IsActive:                true,
				}, nil
			},
			CreateAuthorizationCodeFunc: func(_ context.Context, _ *model.OAuthAuthorizationCode) error {
				return errors.New("db error")
			},
		}
		h, store := newHandlerWithRepo(repo)
		store.session.Values["user_id"] = int64(42)
		store.session.Values["oauth_pending_request"] = map[string]any{
			"nonce":                 "test-nonce",
			"client_id":             "cid",
			"state":                 "my_state_",
			"redirect_uri":          "https://example.com/cb",
			"code_challenge":        "",
			"code_challenge_method": "",
			"scope":                 "",
			"timestamp":             time.Now().Unix(),
		}

		formBody := "client_id=cid&redirect_uri=https://example.com/cb&state=my_state_&nonce=test-nonce"
		req := httptest.NewRequest(http.MethodPost, "/oauth/authorize/approve", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.AuthorizeApprove(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("parses JSON body correctly", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return &model.OAuthClient{
					ID:                      1,
					ClientID:                "cid",
					ClientName:              "Test App",
					TokenEndpointAuthMethod: "none",
					IsActive:                true,
				}, nil
			},
			FindClientByIDFunc: func(_ context.Context, id int64) (*model.OAuthClient, error) {
				return &model.OAuthClient{ID: id}, nil
			},
			CreateAuthorizationCodeFunc: func(_ context.Context, code *model.OAuthAuthorizationCode) error {
				code.ID = 99
				return nil
			},
		}
		h, store := newHandlerWithRepo(repo)
		store.session.Values["user_id"] = int64(42)
		store.session.Values["oauth_pending_request"] = map[string]any{
			"nonce":                 "json-nonce",
			"client_id":             "cid",
			"state":                 "my_state_",
			"redirect_uri":          "https://example.com/cb",
			"code_challenge":        "",
			"code_challenge_method": "",
			"scope":                 "mcp:access",
			"timestamp":             time.Now().Unix(),
		}

		body := `{"client_id":"cid","redirect_uri":"https://example.com/cb","state":"my_state_","scope":"mcp:access","nonce":"json-nonce"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/authorize/approve", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.AuthorizeApprove(rr, req)

		require.Equal(t, http.StatusFound, rr.Code)
		location := rr.Header().Get("Location")
		assert.Contains(t, location, "code=")
	})

	t.Run("omits state from redirect when state is empty", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return &model.OAuthClient{
					ID:                      1,
					ClientID:                "cid",
					ClientName:              "Test App",
					TokenEndpointAuthMethod: "none",
					IsActive:                true,
				}, nil
			},
			CreateAuthorizationCodeFunc: func(_ context.Context, code *model.OAuthAuthorizationCode) error {
				code.ID = 99
				return nil
			},
		}
		h, store := newHandlerWithRepo(repo)
		store.session.Values["user_id"] = int64(42)
		store.session.Values["oauth_pending_request"] = map[string]any{
			"nonce":                 "test-nonce",
			"client_id":             "cid",
			"state":                 "",
			"redirect_uri":          "https://example.com/cb",
			"code_challenge":        "",
			"code_challenge_method": "",
			"scope":                 "",
			"timestamp":             time.Now().Unix(),
		}

		formBody := "client_id=cid&redirect_uri=https://example.com/cb&state=&nonce=test-nonce"
		req := httptest.NewRequest(http.MethodPost, "/oauth/authorize/approve", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.AuthorizeApprove(rr, req)

		require.Equal(t, http.StatusFound, rr.Code)
		location := rr.Header().Get("Location")
		assert.NotContains(t, location, "state=")
	})
}
