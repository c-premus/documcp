package oauthhandler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.999.haus/chris/DocuMCP-go/internal/model"
)

func TestHandler_Revoke(t *testing.T) {
	t.Parallel()

	t.Run("returns error when token is missing (JSON)", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		body := `{"client_id":"cid"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Revoke(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
		assert.Contains(t, result["error_description"], "token")
	})

	t.Run("returns error when token is missing (form)", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		formBody := "client_id=cid"
		req := httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.Revoke(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
		assert.Contains(t, result["error_description"], "token")
	})

	t.Run("returns error when client_id is missing", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		body := `{"token":"some-token-value"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Revoke(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
		assert.Contains(t, result["error_description"], "client id")
	})

	t.Run("returns error for invalid token_type_hint", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		body := `{"token":"some-token","client_id":"cid","token_type_hint":"invalid_type"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Revoke(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
		assert.Contains(t, result["error_description"], "token type hint")
	})

	t.Run("accepts access_token hint", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return &model.OAuthClient{
					ID:                      1,
					ClientID:                "cid",
					TokenEndpointAuthMethod: "none",
				}, nil
			},
			FindAccessTokenByTokenFunc: func(_ context.Context, _ string) (*model.OAuthAccessToken, error) {
				return nil, errors.New("not found")
			},
		}
		h, _ := newHandlerWithRepo(repo)

		body := `{"token":"1|tokenvalue","client_id":"cid","token_type_hint":"access_token"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Revoke(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("accepts refresh_token hint", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return &model.OAuthClient{
					ID:                      1,
					ClientID:                "cid",
					TokenEndpointAuthMethod: "none",
				}, nil
			},
			FindRefreshTokenByTokenFunc: func(_ context.Context, _ string) (*model.OAuthRefreshToken, error) {
				return nil, errors.New("not found")
			},
		}
		h, _ := newHandlerWithRepo(repo)

		body := `{"token":"1|tokenvalue","client_id":"cid","token_type_hint":"refresh_token"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Revoke(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("RFC 7009 compliance: always returns 200 for valid request", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return &model.OAuthClient{
					ID:                      1,
					ClientID:                "cid",
					TokenEndpointAuthMethod: "none",
				}, nil
			},
			FindAccessTokenByTokenFunc: func(_ context.Context, _ string) (*model.OAuthAccessToken, error) {
				return nil, errors.New("not found")
			},
			FindRefreshTokenByTokenFunc: func(_ context.Context, _ string) (*model.OAuthRefreshToken, error) {
				return nil, errors.New("not found")
			},
		}
		h, _ := newHandlerWithRepo(repo)

		// Token that does not exist should still return 200
		body := `{"token":"1|nonexistenttoken","client_id":"cid"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Revoke(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	})

	t.Run("RFC 7009 compliance: response body is empty array", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return &model.OAuthClient{
					ID:                      1,
					ClientID:                "cid",
					TokenEndpointAuthMethod: "none",
				}, nil
			},
			FindAccessTokenByTokenFunc: func(_ context.Context, _ string) (*model.OAuthAccessToken, error) {
				return nil, errors.New("not found")
			},
			FindRefreshTokenByTokenFunc: func(_ context.Context, _ string) (*model.OAuthRefreshToken, error) {
				return nil, errors.New("not found")
			},
		}
		h, _ := newHandlerWithRepo(repo)

		body := `{"token":"1|sometoken","client_id":"cid"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Revoke(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		// Response body should be a JSON empty array
		assert.Contains(t, strings.TrimSpace(rr.Body.String()), "[]")
	})

	t.Run("parses form-urlencoded body correctly", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return &model.OAuthClient{
					ID:                      1,
					ClientID:                "cid",
					TokenEndpointAuthMethod: "none",
				}, nil
			},
			FindAccessTokenByTokenFunc: func(_ context.Context, _ string) (*model.OAuthAccessToken, error) {
				return nil, errors.New("not found")
			},
		}
		h, _ := newHandlerWithRepo(repo)

		formBody := "token=1%7Csometoken&client_id=cid&token_type_hint=access_token"
		req := httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.Revoke(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("returns error for invalid JSON body", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		req := httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader("{bad json"))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Revoke(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
	})

	t.Run("returns server_error when service fails", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return nil, errors.New("db error")
			},
		}
		h, _ := newHandlerWithRepo(repo)

		body := `{"token":"1|sometoken","client_id":"cid"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Revoke(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "server_error", result["error"])
	})

	t.Run("no hint tries both access and refresh token revocation", func(t *testing.T) {
		t.Parallel()
		var accessTokenLookedUp, refreshTokenLookedUp bool
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return &model.OAuthClient{
					ID:                      1,
					ClientID:                "cid",
					TokenEndpointAuthMethod: "none",
				}, nil
			},
			FindAccessTokenByTokenFunc: func(_ context.Context, _ string) (*model.OAuthAccessToken, error) {
				accessTokenLookedUp = true
				return nil, errors.New("not found")
			},
			FindRefreshTokenByTokenFunc: func(_ context.Context, _ string) (*model.OAuthRefreshToken, error) {
				refreshTokenLookedUp = true
				return nil, errors.New("not found")
			},
		}
		h, _ := newHandlerWithRepo(repo)

		body := `{"token":"1|sometoken","client_id":"cid"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Revoke(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.True(t, accessTokenLookedUp, "should attempt access token lookup")
		assert.True(t, refreshTokenLookedUp, "should attempt refresh token lookup")
	})
}
