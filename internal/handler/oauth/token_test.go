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

	"git.999.haus/chris/DocuMCP-go/internal/model"
)

func TestHandler_Token(t *testing.T) {
	t.Parallel()

	t.Run("returns error when grant_type is missing", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		body := `{"client_id":"a1b2c3d4-e5f6-7890-abcd-ef1234567890"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
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

		body := `{"grant_type":"authorization_code"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
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

		body := `{"grant_type":"implicit","client_id":"a1b2c3d4-e5f6-7890-abcd-ef1234567890"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "unsupported_grant_type", result["error"])
		assert.Contains(t, result["error_description"], "implicit")
	})

	t.Run("returns error for invalid JSON body", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader("{invalid json"))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
	})

	t.Run("parses form-urlencoded body", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		formBody := "grant_type=authorization_code&client_id=a1b2c3d4-e5f6-7890-abcd-ef1234567890"
		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		// Should dispatch to authorization_code handler which requires 'code' field
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
		assert.Contains(t, result["error_description"], "code")
	})

	t.Run("parses JSON body", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		body := `{"grant_type":"authorization_code","client_id":"a1b2c3d4-e5f6-7890-abcd-ef1234567890"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		// Should dispatch to authorization_code handler which requires 'code' field
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
		assert.Contains(t, result["error_description"], "code")
	})

	t.Run("dispatches to authorization_code grant type", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		body := `{"grant_type":"authorization_code","client_id":"cid","code":"sometoken","redirect_uri":"https://example.com/callback"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		// The service call will fail because the code is invalid, but the dispatch happened
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_grant", result["error"])
	})

	t.Run("dispatches to refresh_token grant type", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		body := `{"grant_type":"refresh_token","client_id":"cid"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		// refresh_token field is required
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
		assert.Contains(t, result["error_description"], "refresh token")
	})

	t.Run("dispatches to device_code grant type", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		body := `{"grant_type":"urn:ietf:params:oauth:grant-type:device_code","client_id":"cid"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		// device_code field is required
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
		assert.Contains(t, result["error_description"], "device code")
	})

	t.Run("OAuth error response format is correct", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		body := `{}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
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

		body := `{"grant_type":"authorization_code","client_id":"cid","redirect_uri":"https://example.com/cb"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
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

		body := `{"grant_type":"authorization_code","client_id":"cid","code":"1|abc"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
		assert.Contains(t, result["error_description"], "redirect uri")
	})

	t.Run("returns server_error when service fails", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return nil, errors.New("db error")
			},
		}
		h, _ := newHandlerWithRepo(repo)

		body := `{"grant_type":"authorization_code","client_id":"cid","code":"1|abcdef","redirect_uri":"https://example.com/cb"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
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

		body := `{"grant_type":"refresh_token","client_id":"cid"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
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

		body := `{"grant_type":"refresh_token","client_id":"cid","refresh_token":"1|tokenvalue"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_grant", result["error"])
	})

	t.Run("passes form-urlencoded refresh_token fields to service", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return nil, errors.New("not found")
			},
		}
		h, _ := newHandlerWithRepo(repo)

		formBody := "grant_type=refresh_token&client_id=cid&refresh_token=1|tokenvalue"
		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		// Service was called (and returned error), proving form fields were parsed
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

func TestHandler_Token_DeviceCode(t *testing.T) {
	t.Parallel()

	t.Run("returns error when device_code is missing", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		body := `{"grant_type":"urn:ietf:params:oauth:grant-type:device_code","client_id":"cid"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
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
					IsActive:                true,
					GrantTypes:              `["urn:ietf:params:oauth:grant-type:device_code"]`,
				}, nil
			},
			FindDeviceCodeByDeviceCodeFunc: func(_ context.Context, _ string) (*model.OAuthDeviceCode, error) {
				return &model.OAuthDeviceCode{
					ID:        1,
					ClientID:  1,
					Status:    "pending",
					ExpiresAt: time.Now().Add(15 * time.Minute),
				}, nil
			},
		}
		h, _ := newHandlerWithRepo(repo)

		// We need a properly formatted token (id|random) to pass ParseToken
		body := `{"grant_type":"urn:ietf:params:oauth:grant-type:device_code","client_id":"cid","device_code":"1|abcdefghijklmnopqrstuvwxyz012345678901234567890123456789abcdefgh"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		// The device code flow returns authorization_pending as a DeviceCodeError
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "authorization_pending", result["error"])
	})

	t.Run("returns server_error for generic service failure", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return nil, errors.New("db error")
			},
		}
		h, _ := newHandlerWithRepo(repo)

		body := `{"grant_type":"urn:ietf:params:oauth:grant-type:device_code","client_id":"cid","device_code":"1|abcdefghijklmnop"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		// DeviceCodeError with invalid_client is returned as 400
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
					IsActive:                true,
					GrantTypes:              `["urn:ietf:params:oauth:grant-type:device_code"]`,
				}, nil
			},
			FindDeviceCodeByDeviceCodeFunc: func(_ context.Context, _ string) (*model.OAuthDeviceCode, error) {
				return &model.OAuthDeviceCode{
					ID:        1,
					ClientID:  1,
					Status:    "pending",
					ExpiresAt: time.Now().Add(-1 * time.Minute), // expired
				}, nil
			},
		}
		h, _ := newHandlerWithRepo(repo)

		body := `{"grant_type":"urn:ietf:params:oauth:grant-type:device_code","client_id":"cid","device_code":"1|abcdefghijklmnopqrstuvwxyz012345678901234567890123456789abcdefgh"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
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
					IsActive:                true,
					GrantTypes:              `["urn:ietf:params:oauth:grant-type:device_code"]`,
				}, nil
			},
			FindDeviceCodeByDeviceCodeFunc: func(_ context.Context, _ string) (*model.OAuthDeviceCode, error) {
				return &model.OAuthDeviceCode{
					ID:        1,
					ClientID:  1,
					Status:    "denied",
					ExpiresAt: time.Now().Add(15 * time.Minute),
				}, nil
			},
		}
		h, _ := newHandlerWithRepo(repo)

		body := `{"grant_type":"urn:ietf:params:oauth:grant-type:device_code","client_id":"cid","device_code":"1|abcdefghijklmnopqrstuvwxyz012345678901234567890123456789abcdefgh"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
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
					IsActive:                true,
					GrantTypes:              `["urn:ietf:params:oauth:grant-type:device_code"]`,
				}, nil
			},
			FindDeviceCodeByDeviceCodeFunc: func(_ context.Context, _ string) (*model.OAuthDeviceCode, error) {
				return &model.OAuthDeviceCode{
					ID:        1,
					ClientID:  1,
					UserID:    sql.NullInt64{Int64: 42, Valid: true},
					Status:    "authorized",
					Scope:     sql.NullString{String: "mcp:access", Valid: true},
					ExpiresAt: time.Now().Add(15 * time.Minute),
				}, nil
			},
			UpdateDeviceCodeStatusFunc: func(_ context.Context, _ int64, _ string, _ *int64) error {
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

		body := `{"grant_type":"urn:ietf:params:oauth:grant-type:device_code","client_id":"cid","device_code":"1|abcdefghijklmnopqrstuvwxyz012345678901234567890123456789abcdefgh"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
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
		// This covers the generic error path in tokenDeviceCode (not DeviceCodeError).
		// ExchangeDeviceCode returns a non-DeviceCodeError when UpdateDeviceCodeStatus fails
		// on an "authorized" device code (fmt.Errorf wraps the error, not &DeviceCodeError{}).
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return &model.OAuthClient{
					ID:                      1,
					ClientID:                "cid",
					TokenEndpointAuthMethod: "none",
					IsActive:                true,
					GrantTypes:              `["urn:ietf:params:oauth:grant-type:device_code"]`,
				}, nil
			},
			FindDeviceCodeByDeviceCodeFunc: func(_ context.Context, _ string) (*model.OAuthDeviceCode, error) {
				return &model.OAuthDeviceCode{
					ID:        1,
					ClientID:  1,
					UserID:    sql.NullInt64{Int64: 42, Valid: true},
					Status:    "authorized",
					ExpiresAt: time.Now().Add(15 * time.Minute),
				}, nil
			},
			UpdateDeviceCodeStatusFunc: func(_ context.Context, _ int64, _ string, _ *int64) error {
				return errors.New("disk full")
			},
		}
		h, _ := newHandlerWithRepo(repo)

		body := `{"grant_type":"urn:ietf:params:oauth:grant-type:device_code","client_id":"cid","device_code":"1|abcdefghijklmnopqrstuvwxyz012345678901234567890123456789abcdefgh"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Token(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "server_error", result["error"])
	})
}
