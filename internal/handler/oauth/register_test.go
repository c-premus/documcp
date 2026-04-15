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

	authscope "github.com/c-premus/documcp/internal/auth/scope"
	"github.com/c-premus/documcp/internal/model"
)

func TestHandler_Register(t *testing.T) {
	t.Parallel()

	t.Run("returns 404 when registration is disabled", func(t *testing.T) {
		t.Parallel()
		cfg := defaultOAuthConfig()
		cfg.RegistrationEnabled = false
		h, _ := newHandlerWithRepoAndConfig(&mockOAuthRepo{}, cfg)

		body := `{"client_name":"test","redirect_uris":["https://example.com/cb"]}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Register(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("returns 401 when auth required and no session", func(t *testing.T) {
		t.Parallel()
		cfg := defaultOAuthConfig()
		cfg.RegistrationRequireAuth = true
		h, _ := newHandlerWithRepoAndConfig(&mockOAuthRepo{}, cfg)

		body := `{"client_name":"test","redirect_uris":["https://example.com/cb"]}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Register(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("returns 401 when auth required and user not found", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindUserByIDFunc: func(_ context.Context, _ int64) (*model.User, error) {
				return nil, errors.New("not found")
			},
		}
		cfg := defaultOAuthConfig()
		cfg.RegistrationRequireAuth = true
		h, store := newHandlerWithRepoAndConfig(repo, cfg)
		store.session.Values["user_id"] = int64(42)

		body := `{"client_name":"test","redirect_uris":["https://example.com/cb"]}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Register(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("returns 403 when auth required and user is not admin", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindUserByIDFunc: func(_ context.Context, _ int64) (*model.User, error) {
				return &model.User{ID: 42, IsAdmin: false}, nil
			},
		}
		cfg := defaultOAuthConfig()
		cfg.RegistrationRequireAuth = true
		h, store := newHandlerWithRepoAndConfig(repo, cfg)
		store.session.Values["user_id"] = int64(42)

		body := `{"client_name":"test","redirect_uris":["https://example.com/cb"]}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Register(rr, req)

		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("allows registration when user is admin", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindUserByIDFunc: func(_ context.Context, _ int64) (*model.User, error) {
				return &model.User{ID: 42, IsAdmin: true}, nil
			},
			CreateClientFunc: func(_ context.Context, c *model.OAuthClient) error {
				c.ID = 1
				return nil
			},
		}
		cfg := defaultOAuthConfig()
		cfg.RegistrationRequireAuth = true
		h, store := newHandlerWithRepoAndConfig(repo, cfg)
		store.session.Values["user_id"] = int64(42)

		body := `{"client_name":"My App","redirect_uris":["https://example.com/cb"]}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Register(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
	})

	t.Run("returns error for invalid JSON body", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader("{bad"))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Register(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_client_metadata", result["error"])
	})

	t.Run("returns error when client_name is missing", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		body := `{"redirect_uris":["https://example.com/cb"]}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Register(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_client_metadata", result["error"])
		assert.Contains(t, result["error_description"], "client name")
	})

	t.Run("returns error when client_name exceeds 255 characters", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		longName := strings.Repeat("a", 256)
		body := `{"client_name":"` + longName + `","redirect_uris":["https://example.com/cb"]}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Register(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_client_metadata", result["error"])
		assert.Contains(t, result["error_description"], "255")
	})

	t.Run("returns error when redirect_uris is empty", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		body := `{"client_name":"My App","redirect_uris":[]}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Register(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_client_metadata", result["error"])
		assert.Contains(t, result["error_description"], "redirect uris")
	})

	t.Run("returns error when redirect_uri is invalid URL", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		body := `{"client_name":"My App","redirect_uris":["not a url"]}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Register(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_client_metadata", result["error"])
		assert.Contains(t, result["error_description"], "redirect_uris")
	})

	t.Run("returns error for invalid grant type", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		body := `{"client_name":"My App","redirect_uris":["https://example.com/cb"],"grant_types":["implicit"]}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Register(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_client_metadata", result["error"])
		assert.Contains(t, result["error_description"], "grant_types")
	})

	t.Run("returns error for invalid response type", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		body := `{"client_name":"My App","redirect_uris":["https://example.com/cb"],"response_types":["token"]}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Register(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_client_metadata", result["error"])
		assert.Contains(t, result["error_description"], "response_types")
	})

	t.Run("returns error for invalid token_endpoint_auth_method", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindUserByIDFunc: func(_ context.Context, _ int64) (*model.User, error) {
				return &model.User{ID: 42, IsAdmin: true}, nil
			},
		}
		cfg := defaultOAuthConfig()
		cfg.RegistrationRequireAuth = true
		h, store := newHandlerWithRepoAndConfig(repo, cfg)
		store.session.Values["user_id"] = int64(42)

		body := `{"client_name":"My App","redirect_uris":["https://example.com/cb"],"token_endpoint_auth_method":"private_key_jwt"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Register(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_client_metadata", result["error"])
		assert.Contains(t, result["error_description"], "token endpoint auth method")
	})

	t.Run("returns error when software_id exceeds 255 characters", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		longID := strings.Repeat("x", 256)
		body := `{"client_name":"My App","redirect_uris":["https://example.com/cb"],"software_id":"` + longID + `"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Register(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_client_metadata", result["error"])
		assert.Contains(t, result["error_description"], "software id")
	})

	t.Run("returns error when software_version exceeds 100 characters", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepo(&mockOAuthRepo{})

		longVersion := strings.Repeat("v", 101)
		body := `{"client_name":"My App","redirect_uris":["https://example.com/cb"],"software_version":"` + longVersion + `"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Register(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_client_metadata", result["error"])
		assert.Contains(t, result["error_description"], "software version")
	})

	t.Run("happy path returns 201 with expected fields", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			CreateClientFunc: func(_ context.Context, c *model.OAuthClient) error {
				c.ID = 1
				return nil
			},
		}
		h, _ := newHandlerWithRepo(repo)

		body := `{"client_name":"My MCP Client","redirect_uris":["https://example.com/callback"]}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Register(rr, req)

		require.Equal(t, http.StatusCreated, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

		result := decodeOAuthJSON(t, rr.Body)
		assert.NotEmpty(t, result["client_id"])
		assert.Equal(t, "My MCP Client", result["client_name"])
		assert.NotNil(t, result["redirect_uris"])
		assert.NotNil(t, result["grant_types"])
		assert.NotNil(t, result["response_types"])
		assert.NotEmpty(t, result["token_endpoint_auth_method"])
		assert.NotNil(t, result["client_id_issued_at"])
	})

	t.Run("happy path with defaults sets authorization_code and code response type", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			CreateClientFunc: func(_ context.Context, c *model.OAuthClient) error {
				c.ID = 1
				return nil
			},
		}
		h, _ := newHandlerWithRepo(repo)

		body := `{"client_name":"My App","redirect_uris":["https://example.com/cb"]}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Register(rr, req)

		require.Equal(t, http.StatusCreated, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)

		grantTypes, ok := result["grant_types"].([]any)
		require.True(t, ok)
		assert.Contains(t, grantTypes, "authorization_code")

		responseTypes, ok := result["response_types"].([]any)
		require.True(t, ok)
		assert.Contains(t, responseTypes, "code")

		assert.Equal(t, "none", result["token_endpoint_auth_method"])
		assert.Equal(t, authscope.DefaultScopes(), result["scope"])
	})

	t.Run("happy path with confidential client includes client_secret", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindUserByIDFunc: func(_ context.Context, _ int64) (*model.User, error) {
				return &model.User{ID: 42, IsAdmin: true}, nil
			},
			CreateClientFunc: func(_ context.Context, c *model.OAuthClient) error {
				c.ID = 1
				return nil
			},
		}
		cfg := defaultOAuthConfig()
		cfg.RegistrationRequireAuth = true
		h, store := newHandlerWithRepoAndConfig(repo, cfg)
		store.session.Values["user_id"] = int64(42)

		body := `{"client_name":"My App","redirect_uris":["https://example.com/cb"],"token_endpoint_auth_method":"client_secret_post"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Register(rr, req)

		require.Equal(t, http.StatusCreated, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.NotEmpty(t, result["client_secret"])
		assert.Equal(t, "client_secret_post", result["token_endpoint_auth_method"])
	})

	t.Run("accepts valid grant types including device_code", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindUserByIDFunc: func(_ context.Context, _ int64) (*model.User, error) {
				return &model.User{ID: 42, IsAdmin: true}, nil
			},
			CreateClientFunc: func(_ context.Context, c *model.OAuthClient) error {
				c.ID = 1
				return nil
			},
		}
		cfg := defaultOAuthConfig()
		cfg.RegistrationRequireAuth = true
		h, store := newHandlerWithRepoAndConfig(repo, cfg)
		store.session.Values["user_id"] = int64(42)

		body := `{"client_name":"My App","redirect_uris":["https://example.com/cb"],"grant_types":["authorization_code","refresh_token","urn:ietf:params:oauth:grant-type:device_code"]}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Register(rr, req)

		require.Equal(t, http.StatusCreated, rr.Code)
	})

	t.Run("returns server_error when repository fails", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			CreateClientFunc: func(_ context.Context, c *model.OAuthClient) error {
				return errors.New("database error")
			},
		}
		h, _ := newHandlerWithRepo(repo)

		body := `{"client_name":"My App","redirect_uris":["https://example.com/cb"]}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Register(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "server_error", result["error"])
	})

	t.Run("accepts client_secret_basic auth method", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindUserByIDFunc: func(_ context.Context, _ int64) (*model.User, error) {
				return &model.User{ID: 42, IsAdmin: true}, nil
			},
			CreateClientFunc: func(_ context.Context, c *model.OAuthClient) error {
				c.ID = 1
				return nil
			},
		}
		cfg := defaultOAuthConfig()
		cfg.RegistrationRequireAuth = true
		h, store := newHandlerWithRepoAndConfig(repo, cfg)
		store.session.Values["user_id"] = int64(42)

		body := `{"client_name":"My App","redirect_uris":["https://example.com/cb"],"token_endpoint_auth_method":"client_secret_basic"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Register(rr, req)

		require.Equal(t, http.StatusCreated, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "client_secret_basic", result["token_endpoint_auth_method"])
		assert.NotEmpty(t, result["client_secret"])
	})

	t.Run("authenticated registration with scope and software fields", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			FindUserByIDFunc: func(_ context.Context, _ int64) (*model.User, error) {
				return &model.User{ID: 42, IsAdmin: true}, nil
			},
			CreateClientFunc: func(_ context.Context, c *model.OAuthClient) error {
				c.ID = 1
				return nil
			},
		}
		cfg := defaultOAuthConfig()
		cfg.RegistrationRequireAuth = true
		h, store := newHandlerWithRepoAndConfig(repo, cfg)
		store.session.Values["user_id"] = int64(42)

		body := `{"client_name":"My App","redirect_uris":["https://example.com/cb"],"scope":"mcp:access documents:read","software_id":"my-soft-id","software_version":"1.0.0"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Register(rr, req)

		require.Equal(t, http.StatusCreated, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "mcp:access documents:read", result["scope"])
	})

	t.Run("unauthenticated registration forces public client", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			CreateClientFunc: func(_ context.Context, c *model.OAuthClient) error {
				c.ID = 1
				return nil
			},
		}
		cfg := defaultOAuthConfig()
		cfg.RegistrationRequireAuth = false
		h, _ := newHandlerWithRepoAndConfig(repo, cfg)

		body := `{"client_name":"My App","redirect_uris":["https://example.com/cb"],"token_endpoint_auth_method":"client_secret_basic"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Register(rr, req)

		require.Equal(t, http.StatusCreated, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "none", result["token_endpoint_auth_method"])
	})

	t.Run("unauthenticated registration rejects device code grant", func(t *testing.T) {
		t.Parallel()
		cfg := defaultOAuthConfig()
		cfg.RegistrationRequireAuth = false
		h, _ := newHandlerWithRepoAndConfig(&mockOAuthRepo{}, cfg)

		body := `{"client_name":"My App","redirect_uris":["https://example.com/cb"],"grant_types":["urn:ietf:params:oauth:grant-type:device_code"]}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Register(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_client_metadata", result["error"])
		assert.Contains(t, result["error_description"], "Device code grant")
	})

	t.Run("unauthenticated registration ignores requested scope", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			CreateClientFunc: func(_ context.Context, c *model.OAuthClient) error {
				c.ID = 1
				return nil
			},
		}
		cfg := defaultOAuthConfig()
		cfg.RegistrationRequireAuth = false
		h, _ := newHandlerWithRepoAndConfig(repo, cfg)

		body := `{"client_name":"My App","redirect_uris":["https://example.com/cb"],"scope":"admin documents:write"}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Register(rr, req)

		require.Equal(t, http.StatusCreated, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, authscope.DefaultScopes(), result["scope"])
	})

	t.Run("rejects redirect_uris array longer than 10 entries (security.md L1)", func(t *testing.T) {
		t.Parallel()
		h, _ := newHandlerWithRepoAndConfig(&mockOAuthRepo{}, defaultOAuthConfig())

		var uris strings.Builder
		uris.WriteByte('[')
		for i := range 11 {
			if i > 0 {
				uris.WriteByte(',')
			}
			uris.WriteString(`"https://example.com/cb`)
			uris.WriteByte(byte('0' + i%10))
			uris.WriteString(`"`)
		}
		uris.WriteByte(']')
		body := `{"client_name":"too-many","redirect_uris":` + uris.String() + `}`

		req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Register(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "invalid_client_metadata", result["error"])
		assert.Contains(t, result["error_description"], "at most 10")
	})

	t.Run("accepts redirect_uris array of exactly 10 entries", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			CreateClientFunc: func(_ context.Context, c *model.OAuthClient) error {
				c.ID = 1
				return nil
			},
		}
		h, _ := newHandlerWithRepoAndConfig(repo, defaultOAuthConfig())

		var uris strings.Builder
		uris.WriteByte('[')
		for i := range 10 {
			if i > 0 {
				uris.WriteByte(',')
			}
			uris.WriteString(`"https://example.com/cb`)
			uris.WriteByte(byte('0' + i))
			uris.WriteString(`"`)
		}
		uris.WriteByte(']')
		body := `{"client_name":"exactly-10","redirect_uris":` + uris.String() + `}`

		req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Register(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
	})

	t.Run("unauthenticated registration succeeds with valid request", func(t *testing.T) {
		t.Parallel()
		repo := &mockOAuthRepo{
			CreateClientFunc: func(_ context.Context, c *model.OAuthClient) error {
				c.ID = 1
				return nil
			},
		}
		cfg := defaultOAuthConfig()
		cfg.RegistrationRequireAuth = false
		h, _ := newHandlerWithRepoAndConfig(repo, cfg)

		body := `{"client_name":"MCP Remote","redirect_uris":["https://example.com/cb"]}`
		req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.Register(rr, req)

		require.Equal(t, http.StatusCreated, rr.Code)
		result := decodeOAuthJSON(t, rr.Body)
		assert.Equal(t, "none", result["token_endpoint_auth_method"])
		assert.Nil(t, result["client_secret"])
		assert.NotEmpty(t, result["client_id"])
	})
}
