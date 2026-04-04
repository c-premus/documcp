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

func TestHandler_DeviceAuthorization(t *testing.T) {
	t.Parallel()

	t.Run("returns error when client_id is missing (JSON)", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		body := `{"scope":"mcp:access"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/device/code", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.DeviceAuthorization(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
		assert.Contains(t, result["error_description"], "client id")
	})

	t.Run("returns error when client_id is missing (form)", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		formBody := "scope=mcp:access"
		req := httptest.NewRequest(http.MethodPost, "/oauth/device/code", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.DeviceAuthorization(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
	})

	t.Run("returns error for invalid JSON body", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		req := httptest.NewRequest(http.MethodPost, "/oauth/device/code", strings.NewReader("{bad"))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.DeviceAuthorization(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_request", result["error"])
	})

	t.Run("returns invalid_client when client not found", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return nil, sql.ErrNoRows
			},
		}
		h, _ := newHandlerWithRepo(repo)

		body := `{"client_id":"nonexistent-client"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/device/code", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.DeviceAuthorization(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_client", result["error"])
	})

	t.Run("returns invalid_client when client does not support device_code grant", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return &model.OAuthClient{
					ID:                      1,
					ClientID:                "cid",
					TokenEndpointAuthMethod: "none",
					GrantTypes:              `["authorization_code"]`, // no device_code
				}, nil
			},
		}
		h, _ := newHandlerWithRepo(repo)

		body := `{"client_id":"cid"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/device/code", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.DeviceAuthorization(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_client", result["error"])
		assert.Contains(t, result["error_description"], "device_code")
	})

	t.Run("returns server_error for unexpected service errors", func(t *testing.T) {
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
			CreateDeviceCodeFunc: func(_ context.Context, _ *model.OAuthDeviceCode) error {
				return errors.New("database connection lost")
			},
		}
		h, _ := newHandlerWithRepo(repo)

		body := `{"client_id":"cid"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/device/code", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.DeviceAuthorization(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "server_error", result["error"])
	})

	t.Run("happy path returns all required response fields", func(t *testing.T) {
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
			CreateDeviceCodeFunc: func(_ context.Context, dc *model.OAuthDeviceCode) error {
				dc.ID = 42
				return nil
			},
		}
		h, _ := newHandlerWithRepo(repo)

		body := `{"client_id":"cid","scope":"mcp:access"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/device/code", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.DeviceAuthorization(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

		result := decodeOAuthJSON(t, rr.Body)
		assert.NotEmpty(t, result["device_code"], "device_code should be present")
		assert.NotEmpty(t, result["user_code"], "user_code should be present")
		assert.NotEmpty(t, result["verification_uri"], "verification_uri should be present")
		assert.NotEmpty(t, result["verification_uri_complete"], "verification_uri_complete should be present")
		assert.NotNil(t, result["expires_in"], "expires_in should be present")
		assert.NotNil(t, result["interval"], "interval should be present")
	})

	t.Run("verification_uri uses app URL", func(t *testing.T) {
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
			CreateDeviceCodeFunc: func(_ context.Context, dc *model.OAuthDeviceCode) error {
				dc.ID = 42
				return nil
			},
		}
		h, _ := newHandlerWithRepo(repo)

		body := `{"client_id":"cid"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/device/code", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.DeviceAuthorization(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "https://example.com/oauth/device", result["verification_uri"])
		assert.Contains(t, result["verification_uri_complete"].(string), "https://example.com/oauth/device?user_code=")
	})

	t.Run("user_code follows XXXX-XXXX format", func(t *testing.T) {
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
			CreateDeviceCodeFunc: func(_ context.Context, dc *model.OAuthDeviceCode) error {
				dc.ID = 42
				return nil
			},
		}
		h, _ := newHandlerWithRepo(repo)

		body := `{"client_id":"cid"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/device/code", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.DeviceAuthorization(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		userCode := result["user_code"].(string)
		assert.Len(t, userCode, 9, "user_code should be 9 characters (XXXX-XXXX)")
		assert.Equal(t, "-", string(userCode[4]), "user_code should have dash at position 4")
	})

	t.Run("expires_in reflects device code lifetime config", func(t *testing.T) {
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
			CreateDeviceCodeFunc: func(_ context.Context, dc *model.OAuthDeviceCode) error {
				dc.ID = 42
				return nil
			},
		}
		h, _ := newHandlerWithRepo(repo)

		body := `{"client_id":"cid"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/device/code", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.DeviceAuthorization(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		// Default config is 15 minutes = 900 seconds
		assert.InEpsilon(t, float64(900), result["expires_in"], 1e-9)
	})

	t.Run("interval is at least 5 seconds", func(t *testing.T) {
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
			CreateDeviceCodeFunc: func(_ context.Context, dc *model.OAuthDeviceCode) error {
				dc.ID = 42
				return nil
			},
		}
		cfg := defaultOAuthConfig()
		cfg.DeviceCodeInterval = 1 * time.Second // less than 5
		h, _ := newHandlerWithRepoAndConfig(repo, cfg)

		body := `{"client_id":"cid"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/device/code", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.DeviceAuthorization(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		interval := result["interval"].(float64)
		assert.GreaterOrEqual(t, interval, float64(5), "interval should be at least 5 seconds")
	})

	t.Run("parses form-urlencoded request", func(t *testing.T) {
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
			CreateDeviceCodeFunc: func(_ context.Context, dc *model.OAuthDeviceCode) error {
				dc.ID = 42
				return nil
			},
		}
		h, _ := newHandlerWithRepo(repo)

		formBody := "client_id=cid&scope=mcp:access"
		req := httptest.NewRequest(http.MethodPost, "/oauth/device/code", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.DeviceAuthorization(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.NotEmpty(t, result["device_code"])
		assert.NotEmpty(t, result["user_code"])
	})
}

func TestHandler_DeviceVerification(t *testing.T) {
	t.Parallel()

	t.Run("redirects to login when not authenticated", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		req := httptest.NewRequest(http.MethodGet, "/oauth/device", http.NoBody)
		rr := httptest.NewRecorder()

		h.DeviceVerification(rr, req)

		assert.Equal(t, http.StatusFound, rr.Code)
		assert.Contains(t, rr.Header().Get("Location"), "/auth/login")
	})

	t.Run("renders verification page when authenticated", func(t *testing.T) {
		t.Parallel()
		h, store := newHandlerWithRepo(&mockOAuthRepo{})
		store.session.Values["user_id"] = int64(42)

		req := httptest.NewRequest(http.MethodGet, "/oauth/device?user_code=ABCD-EFGH", http.NoBody)
		rr := httptest.NewRecorder()

		h.DeviceVerification(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Header().Get("Content-Type"), "text/html")
		assert.Contains(t, rr.Body.String(), "ABCD-EFGH")
	})
}

func TestHandler_DeviceApprove(t *testing.T) {
	t.Parallel()

	t.Run("returns 401 when not authenticated", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		formBody := "user_code=ABCD-EFGH&approve=approve"
		req := httptest.NewRequest(http.MethodPost, "/oauth/device/approve", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.DeviceApprove(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("returns error when no pending device code in session", func(t *testing.T) {
		t.Parallel()
		h, store := newHandlerWithRepo(&mockOAuthRepo{})
		store.session.Values["user_id"] = int64(42)

		formBody := "user_code=ABCD-EFGH&approve=approve"
		req := httptest.NewRequest(http.MethodPost, "/oauth/device/approve", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.DeviceApprove(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "No pending device authorization")
	})

	t.Run("returns error on user code mismatch", func(t *testing.T) {
		t.Parallel()
		h, store := newHandlerWithRepo(&mockOAuthRepo{})
		store.session.Values["user_id"] = int64(42)
		store.session.Values["device_code_pending"] = map[string]any{
			"user_code": "ABCD-EFGH",
			"timestamp": time.Now().Unix(),
		}

		formBody := "user_code=WXYZ-LMNP&approve=approve"
		req := httptest.NewRequest(http.MethodPost, "/oauth/device/approve", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.DeviceApprove(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "User code mismatch")
	})

	t.Run("returns error when pending request is expired", func(t *testing.T) {
		t.Parallel()
		h, store := newHandlerWithRepo(&mockOAuthRepo{})
		store.session.Values["user_id"] = int64(42)
		store.session.Values["device_code_pending"] = map[string]any{
			"user_code": "ABCD-EFGH",
			"timestamp": time.Now().Add(-15 * time.Minute).Unix(), // expired (>10 min)
		}

		formBody := "user_code=ABCD-EFGH&approve=approve"
		req := httptest.NewRequest(http.MethodPost, "/oauth/device/approve", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.DeviceApprove(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "expired")
	})

	t.Run("shows success page when approved", func(t *testing.T) {
		t.Parallel()
		var authorizedCalled bool
		var capturedScope string
		repo := &mockOAuthRepo{
			FindDeviceCodeByUserCodeFunc: func(_ context.Context, userCode string) (*model.OAuthDeviceCode, error) {
				return &model.OAuthDeviceCode{
					ID:        1,
					UserCode:  userCode,
					Scope:     sql.NullString{String: "mcp:access documents:read", Valid: true},
					Status:    model.DeviceCodeStatusPending,
					ExpiresAt: time.Now().Add(15 * time.Minute),
				}, nil
			},
			FindUserByIDFunc: func(_ context.Context, id int64) (*model.User, error) {
				return &model.User{ID: id, IsAdmin: false}, nil
			},
			UpdateDeviceCodeStatusAndScopeFunc: func(_ context.Context, _ int64, status model.DeviceCodeStatus, _ *int64, scope string) error {
				authorizedCalled = status == model.DeviceCodeStatusAuthorized
				capturedScope = scope
				return nil
			},
		}
		h, store := newHandlerWithRepo(repo)
		store.session.Values["user_id"] = int64(42)
		store.session.Values["device_code_pending"] = map[string]any{
			"user_code": "ABCD-EFGH",
			"timestamp": time.Now().Unix(),
		}

		formBody := "user_code=ABCD-EFGH&approve=approve"
		req := httptest.NewRequest(http.MethodPost, "/oauth/device/approve", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.DeviceApprove(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "Authorization Successful")
		assert.True(t, authorizedCalled)
		assert.Contains(t, capturedScope, "mcp:access")
	})

	t.Run("returns error when session pending value is wrong type", func(t *testing.T) {
		t.Parallel()
		h, store := newHandlerWithRepo(&mockOAuthRepo{})
		store.session.Values["user_id"] = int64(42)
		// pendingRaw is a string, not map[string]any — triggers the type assertion failure branch.
		store.session.Values["device_code_pending"] = "not-a-map"

		formBody := "user_code=ABCD-EFGH&approve=approve"
		req := httptest.NewRequest(http.MethodPost, "/oauth/device/approve", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.DeviceApprove(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "No pending device authorization")
	})

	t.Run("returns error when AuthorizeDeviceCode fails", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindDeviceCodeByUserCodeFunc: func(_ context.Context, _ string) (*model.OAuthDeviceCode, error) {
				return nil, errors.New("device code not found")
			},
		}
		h, store := newHandlerWithRepo(repo)
		store.session.Values["user_id"] = int64(42)
		store.session.Values["device_code_pending"] = map[string]any{
			"user_code": "ABCD-EFGH",
			"timestamp": time.Now().Unix(),
		}

		formBody := "user_code=ABCD-EFGH&approve=approve"
		req := httptest.NewRequest(http.MethodPost, "/oauth/device/approve", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.DeviceApprove(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "error occurred")
	})

	t.Run("shows denied page when user denies", func(t *testing.T) {
		t.Parallel()
		var deniedStatus model.DeviceCodeStatus
		repo := &mockOAuthRepo{
			FindDeviceCodeByUserCodeFunc: func(_ context.Context, userCode string) (*model.OAuthDeviceCode, error) {
				return &model.OAuthDeviceCode{
					ID:        1,
					UserCode:  userCode,
					Status:    model.DeviceCodeStatusPending,
					ExpiresAt: time.Now().Add(15 * time.Minute),
				}, nil
			},
			UpdateDeviceCodeStatusFunc: func(_ context.Context, _ int64, status model.DeviceCodeStatus, _ *int64) error {
				deniedStatus = status
				return nil
			},
		}
		h, store := newHandlerWithRepo(repo)
		store.session.Values["user_id"] = int64(42)
		store.session.Values["device_code_pending"] = map[string]any{
			"user_code": "ABCD-EFGH",
			"timestamp": time.Now().Unix(),
		}

		formBody := "user_code=ABCD-EFGH&approve=deny"
		req := httptest.NewRequest(http.MethodPost, "/oauth/device/approve", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.DeviceApprove(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "Authorization Denied")
		assert.Equal(t, model.DeviceCodeStatusDenied, deniedStatus)
	})
}

func TestHandler_DeviceVerificationSubmit(t *testing.T) {
	t.Parallel()

	t.Run("redirects to login when not authenticated", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		formBody := "user_code=ABCD-EFGH"
		req := httptest.NewRequest(http.MethodPost, "/oauth/device", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.DeviceVerificationSubmit(rr, req)

		assert.Equal(t, http.StatusFound, rr.Code)
		assert.Contains(t, rr.Header().Get("Location"), "/auth/login")
	})

	t.Run("returns error HTML for empty user_code", func(t *testing.T) {
		t.Parallel()
		h, store := newHandlerWithRepo(&mockOAuthRepo{})
		store.session.Values["user_id"] = int64(42)

		formBody := "user_code="
		req := httptest.NewRequest(http.MethodPost, "/oauth/device", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.DeviceVerificationSubmit(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invalid or expired")
	})

	t.Run("returns error HTML for user_code longer than 9 chars", func(t *testing.T) {
		t.Parallel()
		h, store := newHandlerWithRepo(&mockOAuthRepo{})
		store.session.Values["user_id"] = int64(42)

		formBody := "user_code=ABCDEFGHIJ"
		req := httptest.NewRequest(http.MethodPost, "/oauth/device", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.DeviceVerificationSubmit(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invalid or expired")
	})

	t.Run("returns error HTML when device code not found", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindDeviceCodeByUserCodeFunc: func(_ context.Context, _ string) (*model.OAuthDeviceCode, error) {
				return nil, errors.New("not found")
			},
		}
		h, store := newHandlerWithRepo(repo)
		store.session.Values["user_id"] = int64(42)

		formBody := "user_code=ABCD-EFGH"
		req := httptest.NewRequest(http.MethodPost, "/oauth/device", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.DeviceVerificationSubmit(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invalid or expired")
	})

	t.Run("returns error HTML when device code is expired", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindDeviceCodeByUserCodeFunc: func(_ context.Context, userCode string) (*model.OAuthDeviceCode, error) {
				return &model.OAuthDeviceCode{
					ID:        1,
					ClientID:  10,
					UserCode:  userCode,
					Status:    model.DeviceCodeStatusPending,
					ExpiresAt: time.Now().Add(-1 * time.Minute),
				}, nil
			},
		}
		h, store := newHandlerWithRepo(repo)
		store.session.Values["user_id"] = int64(42)

		formBody := "user_code=ABCD-EFGH"
		req := httptest.NewRequest(http.MethodPost, "/oauth/device", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.DeviceVerificationSubmit(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "expired")
	})

	t.Run("returns error HTML when device code is not pending", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindDeviceCodeByUserCodeFunc: func(_ context.Context, userCode string) (*model.OAuthDeviceCode, error) {
				return &model.OAuthDeviceCode{
					ID:        1,
					ClientID:  10,
					UserCode:  userCode,
					Status:    model.DeviceCodeStatusAuthorized,
					ExpiresAt: time.Now().Add(15 * time.Minute),
				}, nil
			},
		}
		h, store := newHandlerWithRepo(repo)
		store.session.Values["user_id"] = int64(42)

		formBody := "user_code=ABCD-EFGH"
		req := httptest.NewRequest(http.MethodPost, "/oauth/device", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.DeviceVerificationSubmit(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "already been used")
	})

	t.Run("returns error HTML when client lookup fails", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindDeviceCodeByUserCodeFunc: func(_ context.Context, userCode string) (*model.OAuthDeviceCode, error) {
				return &model.OAuthDeviceCode{
					ID:        1,
					ClientID:  10,
					UserCode:  userCode,
					Status:    model.DeviceCodeStatusPending,
					ExpiresAt: time.Now().Add(15 * time.Minute),
				}, nil
			},
			FindClientByIDFunc: func(_ context.Context, _ int64) (*model.OAuthClient, error) {
				return nil, errors.New("client not found")
			},
		}
		h, store := newHandlerWithRepo(repo)
		store.session.Values["user_id"] = int64(42)

		formBody := "user_code=ABCD-EFGH"
		req := httptest.NewRequest(http.MethodPost, "/oauth/device", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.DeviceVerificationSubmit(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "error occurred")
	})

	t.Run("shows consent screen for valid pending device code", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindDeviceCodeByUserCodeFunc: func(_ context.Context, userCode string) (*model.OAuthDeviceCode, error) {
				return &model.OAuthDeviceCode{
					ID:        1,
					ClientID:  10,
					UserCode:  userCode,
					Status:    model.DeviceCodeStatusPending,
					ExpiresAt: time.Now().Add(15 * time.Minute),
				}, nil
			},
			FindClientByIDFunc: func(_ context.Context, _ int64) (*model.OAuthClient, error) {
				return &model.OAuthClient{
					ID:         10,
					ClientID:   "test-client",
					ClientName: "MyApp",
				}, nil
			},
		}
		h, store := newHandlerWithRepo(repo)
		store.session.Values["user_id"] = int64(42)

		formBody := "user_code=ABCD-EFGH"
		req := httptest.NewRequest(http.MethodPost, "/oauth/device", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.DeviceVerificationSubmit(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Header().Get("Content-Type"), "text/html")
		assert.Contains(t, rr.Body.String(), "MyApp")
	})

	t.Run("consent screen includes scope when present", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindDeviceCodeByUserCodeFunc: func(_ context.Context, userCode string) (*model.OAuthDeviceCode, error) {
				return &model.OAuthDeviceCode{
					ID:        1,
					ClientID:  10,
					UserCode:  userCode,
					Status:    model.DeviceCodeStatusPending,
					ExpiresAt: time.Now().Add(15 * time.Minute),
					Scope:     sql.NullString{String: "mcp:access", Valid: true},
				}, nil
			},
			FindClientByIDFunc: func(_ context.Context, _ int64) (*model.OAuthClient, error) {
				return &model.OAuthClient{
					ID:         10,
					ClientID:   "test-client",
					ClientName: "MyApp",
				}, nil
			},
			FindUserByIDFunc: func(_ context.Context, id int64) (*model.User, error) {
				return &model.User{ID: id, IsAdmin: false}, nil
			},
		}
		h, store := newHandlerWithRepo(repo)
		store.session.Values["user_id"] = int64(42)

		formBody := "user_code=ABCD-EFGH"
		req := httptest.NewRequest(http.MethodPost, "/oauth/device", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.DeviceVerificationSubmit(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "mcp:access")
	})

	t.Run("consent screen works when scope is null", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindDeviceCodeByUserCodeFunc: func(_ context.Context, userCode string) (*model.OAuthDeviceCode, error) {
				return &model.OAuthDeviceCode{
					ID:        1,
					ClientID:  10,
					UserCode:  userCode,
					Status:    model.DeviceCodeStatusPending,
					ExpiresAt: time.Now().Add(15 * time.Minute),
					Scope:     sql.NullString{Valid: false},
				}, nil
			},
			FindClientByIDFunc: func(_ context.Context, _ int64) (*model.OAuthClient, error) {
				return &model.OAuthClient{
					ID:         10,
					ClientID:   "test-client",
					ClientName: "MyApp",
				}, nil
			},
		}
		h, store := newHandlerWithRepo(repo)
		store.session.Values["user_id"] = int64(42)

		formBody := "user_code=ABCD-EFGH"
		req := httptest.NewRequest(http.MethodPost, "/oauth/device", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.DeviceVerificationSubmit(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Header().Get("Content-Type"), "text/html")
	})
}
