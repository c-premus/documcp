package oauthhandler

import (
	"context"
	"database/sql"
	"encoding/base64"
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

// newFormRequest constructs a token-endpoint POST carrying a
// application/x-www-form-urlencoded body (the only content type the endpoint
// accepts as of v0.21.0 — security.md M4).
func newFormRequest(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func TestHandler_Token(t *testing.T) {
	t.Parallel()

	t.Run("returns error when grant_type is missing", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		req := newFormRequest("client_id=a1b2c3d4-e5f6-7890-abcd-ef1234567890")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
		assert.Contains(t, result["error_description"], "grant type")
	})

	t.Run("returns error when client_id is missing", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		req := newFormRequest("grant_type=authorization_code")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
		assert.Contains(t, result["error_description"], "client id")
	})

	t.Run("returns unsupported_grant_type for unknown grant type", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		req := newFormRequest("grant_type=implicit&client_id=a1b2c3d4-e5f6-7890-abcd-ef1234567890")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "unsupported_grant_type", result["error"])
		assert.Contains(t, result["error_description"], "implicit")
	})

	t.Run("rejects application/json with 415 (security.md M4)", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		req := httptest.NewRequest(http.MethodPost, "/oauth/token",
			strings.NewReader(`{"grant_type":"authorization_code","client_id":"cid"}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		assert.Equal(t, http.StatusUnsupportedMediaType, rr.Code)
		assert.Equal(t, "application/x-www-form-urlencoded", rr.Header().Get("Accept"))
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
	})

	t.Run("rejects missing content-type with 415", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		// No Content-Type header at all.
		req := httptest.NewRequest(http.MethodPost, "/oauth/token",
			strings.NewReader("grant_type=authorization_code&client_id=cid"))
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		assert.Equal(t, http.StatusUnsupportedMediaType, rr.Code)
	})

	t.Run("accepts form-urlencoded with charset parameter", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		req := httptest.NewRequest(http.MethodPost, "/oauth/token",
			strings.NewReader("grant_type=authorization_code&client_id=cid"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		// Dispatched — parse errors from inner handlers become 400 invalid_request.
		assert.NotEqual(t, http.StatusUnsupportedMediaType, rr.Code)
	})

	t.Run("accepts HTTP Basic client credentials (security.md M3)", func(t *testing.T) {
		t.Parallel()
		var seenClientID string
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, clientID string) (*model.OAuthClient, error) {
				seenClientID = clientID
				return nil, errors.New("forced not-found to exit early")
			},
		}
		h, _ := newHandlerWithRepo(repo)

		req := newFormRequest("grant_type=authorization_code&code=1|abcdef&redirect_uri=https://example.com/cb")
		// Basic base64("cid:sekret")
		basic := base64.StdEncoding.EncodeToString([]byte("cid:sekret"))
		req.Header.Set("Authorization", "Basic "+basic)
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		assert.Equal(t, "cid", seenClientID,
			"Basic-auth client_id must be parsed and forwarded to the service")
	})

	t.Run("rejects dual-auth: Basic + body credentials (RFC 6749 §2.3.1)", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		req := newFormRequest(
			"grant_type=authorization_code&code=1|abcdef&redirect_uri=https://example.com/cb" +
				"&client_id=cid&client_secret=body_secret")
		basic := base64.StdEncoding.EncodeToString([]byte("cid:basic_secret"))
		req.Header.Set("Authorization", "Basic "+basic)
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
	})

	t.Run("parses form-urlencoded body", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		req := newFormRequest("grant_type=authorization_code&client_id=a1b2c3d4-e5f6-7890-abcd-ef1234567890")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		// Should dispatch to authorization_code handler which requires 'code' field.
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
		assert.Contains(t, result["error_description"], "code")
	})

	t.Run("dispatches to authorization_code grant type", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		req := newFormRequest(
			"grant_type=authorization_code&client_id=cid&code=sometoken&redirect_uri=https://example.com/callback")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_grant", result["error"])
	})

	t.Run("dispatches to refresh_token grant type", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		req := newFormRequest("grant_type=refresh_token&client_id=cid")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
		assert.Contains(t, result["error_description"], "refresh token")
	})

	t.Run("dispatches to device_code grant type", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		req := newFormRequest(
			"grant_type=urn:ietf:params:oauth:grant-type:device_code&client_id=cid")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
		assert.Contains(t, result["error_description"], "device code")
	})

	t.Run("OAuth error response format is correct", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		req := newFormRequest("")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
		result := decodeOAuthJSON(t, rr.Body)
		_, hasError := result["error"]
		_, hasDesc := result["error_description"]
		assert.True(t, hasError, "response must contain 'error' field")
		assert.True(t, hasDesc, "response must contain 'error_description' field")
	})
}

func TestHandler_Token_AuthorizationCode(t *testing.T) {
	t.Parallel()

	t.Run("returns error when code is missing", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		req := newFormRequest(
			"grant_type=authorization_code&client_id=cid&redirect_uri=https://example.com/cb")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
		assert.Contains(t, result["error_description"], "code")
	})

	t.Run("returns error when redirect_uri is missing", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		req := newFormRequest("grant_type=authorization_code&client_id=cid&code=1|abc")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
		assert.Contains(t, result["error_description"], "redirect uri")
	})

	t.Run("returns invalid_grant when service fails", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return nil, errors.New("db error")
			},
		}
		h, _ := newHandlerWithRepo(repo)

		req := newFormRequest(
			"grant_type=authorization_code&client_id=cid&code=1|abcdef&redirect_uri=https://example.com/cb")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_grant", result["error"])
	})
}

func TestHandler_Token_RefreshToken(t *testing.T) {
	t.Parallel()

	t.Run("returns error when refresh_token is missing", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		req := newFormRequest("grant_type=refresh_token&client_id=cid")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
		assert.Contains(t, result["error_description"], "refresh token")
	})

	t.Run("returns invalid_grant when service fails", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return nil, errors.New("db error")
			},
		}
		h, _ := newHandlerWithRepo(repo)

		req := newFormRequest(
			"grant_type=refresh_token&client_id=cid&refresh_token=1|tokenvalue")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_grant", result["error"])
	})
}

func TestHandler_Token_DeviceCode(t *testing.T) {
	t.Parallel()

	t.Run("returns error when device_code is missing", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		req := newFormRequest(
			"grant_type=urn:ietf:params:oauth:grant-type:device_code&client_id=cid")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
		assert.Contains(t, result["error_description"], "device code")
	})

	t.Run("returns typed DeviceCodeError as OAuth error", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return &model.OAuthClient{
					ID:                      1,
					ClientID:                "cid",
					TokenEndpointAuthMethod: "none",
					GrantTypes:              `["urn:ietf:params:oauth:grant-type:device_code"]`,
				}, nil
			},
			FindDeviceCodeByDeviceCodeFunc: func(_ context.Context, _ string) (*model.OAuthDeviceCode, error) {
				return &model.OAuthDeviceCode{
					ID:        1,
					ClientID:  1,
					Status:    model.DeviceCodeStatusPending,
					ExpiresAt: time.Now().Add(15 * time.Minute),
				}, nil
			},
		}
		h, _ := newHandlerWithRepo(repo)

		req := newFormRequest(
			"grant_type=urn:ietf:params:oauth:grant-type:device_code&client_id=cid" +
				"&device_code=1|abcdefghijklmnopqrstuvwxyz012345678901234567890123456789abcdefgh")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "authorization_pending", result["error"])
	})

	t.Run("returns invalid_client for generic service failure", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return nil, errors.New("db error")
			},
		}
		h, _ := newHandlerWithRepo(repo)

		req := newFormRequest(
			"grant_type=urn:ietf:params:oauth:grant-type:device_code&client_id=cid" +
				"&device_code=1|abcdefghijklmnop")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_client", result["error"])
	})

	t.Run("returns expired_token DeviceCodeError", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return &model.OAuthClient{
					ID:                      1,
					ClientID:                "cid",
					TokenEndpointAuthMethod: "none",
					GrantTypes:              `["urn:ietf:params:oauth:grant-type:device_code"]`,
				}, nil
			},
			FindDeviceCodeByDeviceCodeFunc: func(_ context.Context, _ string) (*model.OAuthDeviceCode, error) {
				return &model.OAuthDeviceCode{
					ID:        1,
					ClientID:  1,
					Status:    model.DeviceCodeStatusPending,
					ExpiresAt: time.Now().Add(-1 * time.Minute),
				}, nil
			},
		}
		h, _ := newHandlerWithRepo(repo)

		req := newFormRequest(
			"grant_type=urn:ietf:params:oauth:grant-type:device_code&client_id=cid" +
				"&device_code=1|abcdefghijklmnopqrstuvwxyz012345678901234567890123456789abcdefgh")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "expired_token", result["error"])
	})

	t.Run("returns access_denied when user denied device code", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return &model.OAuthClient{
					ID:                      1,
					ClientID:                "cid",
					TokenEndpointAuthMethod: "none",
					GrantTypes:              `["urn:ietf:params:oauth:grant-type:device_code"]`,
				}, nil
			},
			FindDeviceCodeByDeviceCodeFunc: func(_ context.Context, _ string) (*model.OAuthDeviceCode, error) {
				return &model.OAuthDeviceCode{
					ID:        1,
					ClientID:  1,
					Status:    model.DeviceCodeStatusDenied,
					ExpiresAt: time.Now().Add(15 * time.Minute),
				}, nil
			},
		}
		h, _ := newHandlerWithRepo(repo)

		req := newFormRequest(
			"grant_type=urn:ietf:params:oauth:grant-type:device_code&client_id=cid" +
				"&device_code=1|abcdefghijklmnopqrstuvwxyz012345678901234567890123456789abcdefgh")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "access_denied", result["error"])
	})

	t.Run("successful device code exchange returns tokens", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return &model.OAuthClient{
					ID:                      1,
					ClientID:                "cid",
					TokenEndpointAuthMethod: "none",
					GrantTypes:              `["urn:ietf:params:oauth:grant-type:device_code"]`,
				}, nil
			},
			FindDeviceCodeByDeviceCodeFunc: func(_ context.Context, _ string) (*model.OAuthDeviceCode, error) {
				return &model.OAuthDeviceCode{
					ID:        1,
					ClientID:  1,
					UserID:    sql.NullInt64{Int64: 42, Valid: true},
					Status:    model.DeviceCodeStatusAuthorized,
					Scope:     sql.NullString{String: "mcp:access", Valid: true},
					ExpiresAt: time.Now().Add(15 * time.Minute),
				}, nil
			},
			ExchangeDeviceCodeStatusFunc: func(_ context.Context, _ int64) error {
				return nil
			},
			CreateAccessTokenFunc: func(_ context.Context, token *model.OAuthAccessToken) error {
				token.ID = 100
				return nil
			},
			CreateRefreshTokenFunc: func(_ context.Context, token *model.OAuthRefreshToken) error {
				token.ID = 200
				return nil
			},
		}
		h, _ := newHandlerWithRepo(repo)

		req := newFormRequest(
			"grant_type=urn:ietf:params:oauth:grant-type:device_code&client_id=cid" +
				"&device_code=1|abcdefghijklmnopqrstuvwxyz012345678901234567890123456789abcdefgh")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.NotEmpty(t, result["access_token"])
		assert.Equal(t, "Bearer", result["token_type"])
		assert.NotEmpty(t, result["refresh_token"])
		require.NotNil(t, result["expires_in"])
	})

	t.Run("returns server_error when ExchangeDeviceCode returns non-DeviceCodeError", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return &model.OAuthClient{
					ID:                      1,
					ClientID:                "cid",
					TokenEndpointAuthMethod: "none",
					GrantTypes:              `["urn:ietf:params:oauth:grant-type:device_code"]`,
				}, nil
			},
			FindDeviceCodeByDeviceCodeFunc: func(_ context.Context, _ string) (*model.OAuthDeviceCode, error) {
				return &model.OAuthDeviceCode{
					ID:        1,
					ClientID:  1,
					UserID:    sql.NullInt64{Int64: 42, Valid: true},
					Status:    model.DeviceCodeStatusAuthorized,
					ExpiresAt: time.Now().Add(15 * time.Minute),
				}, nil
			},
			ExchangeDeviceCodeStatusFunc: func(_ context.Context, _ int64) error {
				return errors.New("disk full")
			},
		}
		h, _ := newHandlerWithRepo(repo)

		req := newFormRequest(
			"grant_type=urn:ietf:params:oauth:grant-type:device_code&client_id=cid" +
				"&device_code=1|abcdefghijklmnopqrstuvwxyz012345678901234567890123456789abcdefgh")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "server_error", result["error"])
	})
}
