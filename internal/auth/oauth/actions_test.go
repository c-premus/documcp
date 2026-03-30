package oauth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authscope "github.com/c-premus/documcp/internal/auth/scope"
	"github.com/c-premus/documcp/internal/config"
	"github.com/c-premus/documcp/internal/model"
)

// ---------------------------------------------------------------------------
// Mock repository (callback-based, matching existing patterns)
// ---------------------------------------------------------------------------

type mockOAuthRepo struct {
	// Clients
	CreateClientFunc         func(ctx context.Context, client *model.OAuthClient) error
	FindClientByClientIDFunc func(ctx context.Context, clientID string) (*model.OAuthClient, error)
	FindClientByIDFunc       func(ctx context.Context, id int64) (*model.OAuthClient, error)
	TouchClientLastUsedFunc  func(ctx context.Context, clientID int64) error
	UpdateClientScopeFunc    func(ctx context.Context, clientID int64, scope string) error
	// Authorization Codes
	CreateAuthorizationCodeFunc     func(ctx context.Context, code *model.OAuthAuthorizationCode) error
	FindAuthorizationCodeByCodeFunc func(ctx context.Context, codeHash string) (*model.OAuthAuthorizationCode, error)
	RevokeAuthorizationCodeFunc     func(ctx context.Context, id int64) error
	// Access Tokens
	CreateAccessTokenFunc      func(ctx context.Context, token *model.OAuthAccessToken) error
	FindAccessTokenByIDFunc    func(ctx context.Context, id int64) (*model.OAuthAccessToken, error)
	FindAccessTokenByTokenFunc func(ctx context.Context, tokenHash string) (*model.OAuthAccessToken, error)
	RevokeAccessTokenFunc      func(ctx context.Context, id int64) error
	RevokeTokenPairFunc        func(ctx context.Context, accessTokenID, refreshTokenID int64) error
	// Refresh Tokens
	CreateRefreshTokenFunc                func(ctx context.Context, token *model.OAuthRefreshToken) error
	FindRefreshTokenByTokenFunc           func(ctx context.Context, tokenHash string) (*model.OAuthRefreshToken, error)
	RevokeRefreshTokenFunc                func(ctx context.Context, id int64) error
	RevokeRefreshTokenByAccessTokenIDFunc func(ctx context.Context, accessTokenID int64) error
	// Device Codes
	CreateDeviceCodeFunc           func(ctx context.Context, dc *model.OAuthDeviceCode) error
	FindDeviceCodeByDeviceCodeFunc func(ctx context.Context, deviceCodeHash string) (*model.OAuthDeviceCode, error)
	FindDeviceCodeByUserCodeFunc   func(ctx context.Context, userCode string) (*model.OAuthDeviceCode, error)
	UpdateDeviceCodeStatusFunc     func(ctx context.Context, id int64, status string, userID *int64) error
	UpdateDeviceCodeLastPolledFunc func(ctx context.Context, id int64, interval int) error
	// Users
	FindUserByIDFunc func(ctx context.Context, id int64) (*model.User, error)
}

func (m *mockOAuthRepo) CreateClient(ctx context.Context, client *model.OAuthClient) error {
	if m.CreateClientFunc != nil {
		return m.CreateClientFunc(ctx, client)
	}
	return nil
}

func (m *mockOAuthRepo) FindClientByClientID(ctx context.Context, clientID string) (*model.OAuthClient, error) {
	if m.FindClientByClientIDFunc != nil {
		return m.FindClientByClientIDFunc(ctx, clientID)
	}
	return nil, sql.ErrNoRows
}

func (m *mockOAuthRepo) FindClientByID(ctx context.Context, id int64) (*model.OAuthClient, error) {
	if m.FindClientByIDFunc != nil {
		return m.FindClientByIDFunc(ctx, id)
	}
	return nil, sql.ErrNoRows
}

func (m *mockOAuthRepo) TouchClientLastUsed(ctx context.Context, clientID int64) error {
	if m.TouchClientLastUsedFunc != nil {
		return m.TouchClientLastUsedFunc(ctx, clientID)
	}
	return nil
}

func (m *mockOAuthRepo) UpdateClientScope(ctx context.Context, clientID int64, scope string) error {
	if m.UpdateClientScopeFunc != nil {
		return m.UpdateClientScopeFunc(ctx, clientID, scope)
	}
	return nil
}

func (m *mockOAuthRepo) CreateAuthorizationCode(ctx context.Context, code *model.OAuthAuthorizationCode) error {
	if m.CreateAuthorizationCodeFunc != nil {
		return m.CreateAuthorizationCodeFunc(ctx, code)
	}
	return nil
}

func (m *mockOAuthRepo) FindAuthorizationCodeByCode(ctx context.Context, codeHash string) (*model.OAuthAuthorizationCode, error) {
	if m.FindAuthorizationCodeByCodeFunc != nil {
		return m.FindAuthorizationCodeByCodeFunc(ctx, codeHash)
	}
	return nil, sql.ErrNoRows
}

func (m *mockOAuthRepo) RevokeAuthorizationCode(ctx context.Context, id int64) error {
	if m.RevokeAuthorizationCodeFunc != nil {
		return m.RevokeAuthorizationCodeFunc(ctx, id)
	}
	return nil
}

func (m *mockOAuthRepo) CreateAccessToken(ctx context.Context, token *model.OAuthAccessToken) error {
	if m.CreateAccessTokenFunc != nil {
		return m.CreateAccessTokenFunc(ctx, token)
	}
	return nil
}

func (m *mockOAuthRepo) FindAccessTokenByID(ctx context.Context, id int64) (*model.OAuthAccessToken, error) {
	if m.FindAccessTokenByIDFunc != nil {
		return m.FindAccessTokenByIDFunc(ctx, id)
	}
	return nil, sql.ErrNoRows
}

func (m *mockOAuthRepo) FindAccessTokenByToken(ctx context.Context, tokenHash string) (*model.OAuthAccessToken, error) {
	if m.FindAccessTokenByTokenFunc != nil {
		return m.FindAccessTokenByTokenFunc(ctx, tokenHash)
	}
	return nil, sql.ErrNoRows
}

func (m *mockOAuthRepo) RevokeAccessToken(ctx context.Context, id int64) error {
	if m.RevokeAccessTokenFunc != nil {
		return m.RevokeAccessTokenFunc(ctx, id)
	}
	return nil
}

func (m *mockOAuthRepo) RevokeTokenPair(ctx context.Context, accessTokenID, refreshTokenID int64) error {
	if m.RevokeTokenPairFunc != nil {
		return m.RevokeTokenPairFunc(ctx, accessTokenID, refreshTokenID)
	}
	return nil
}

func (m *mockOAuthRepo) CreateRefreshToken(ctx context.Context, token *model.OAuthRefreshToken) error {
	if m.CreateRefreshTokenFunc != nil {
		return m.CreateRefreshTokenFunc(ctx, token)
	}
	return nil
}

func (m *mockOAuthRepo) FindRefreshTokenByToken(ctx context.Context, tokenHash string) (*model.OAuthRefreshToken, error) {
	if m.FindRefreshTokenByTokenFunc != nil {
		return m.FindRefreshTokenByTokenFunc(ctx, tokenHash)
	}
	return nil, sql.ErrNoRows
}

func (m *mockOAuthRepo) RevokeRefreshToken(ctx context.Context, id int64) error {
	if m.RevokeRefreshTokenFunc != nil {
		return m.RevokeRefreshTokenFunc(ctx, id)
	}
	return nil
}

func (m *mockOAuthRepo) RevokeRefreshTokenByAccessTokenID(ctx context.Context, accessTokenID int64) error {
	if m.RevokeRefreshTokenByAccessTokenIDFunc != nil {
		return m.RevokeRefreshTokenByAccessTokenIDFunc(ctx, accessTokenID)
	}
	return nil
}

func (m *mockOAuthRepo) CreateDeviceCode(ctx context.Context, dc *model.OAuthDeviceCode) error {
	if m.CreateDeviceCodeFunc != nil {
		return m.CreateDeviceCodeFunc(ctx, dc)
	}
	return nil
}

func (m *mockOAuthRepo) FindDeviceCodeByDeviceCode(ctx context.Context, deviceCodeHash string) (*model.OAuthDeviceCode, error) {
	if m.FindDeviceCodeByDeviceCodeFunc != nil {
		return m.FindDeviceCodeByDeviceCodeFunc(ctx, deviceCodeHash)
	}
	return nil, sql.ErrNoRows
}

func (m *mockOAuthRepo) FindDeviceCodeByUserCode(ctx context.Context, userCode string) (*model.OAuthDeviceCode, error) {
	if m.FindDeviceCodeByUserCodeFunc != nil {
		return m.FindDeviceCodeByUserCodeFunc(ctx, userCode)
	}
	return nil, sql.ErrNoRows
}

func (m *mockOAuthRepo) UpdateDeviceCodeStatus(ctx context.Context, id int64, status string, userID *int64) error {
	if m.UpdateDeviceCodeStatusFunc != nil {
		return m.UpdateDeviceCodeStatusFunc(ctx, id, status, userID)
	}
	return nil
}

func (m *mockOAuthRepo) UpdateDeviceCodeLastPolled(ctx context.Context, id int64, interval int) error {
	if m.UpdateDeviceCodeLastPolledFunc != nil {
		return m.UpdateDeviceCodeLastPolledFunc(ctx, id, interval)
	}
	return nil
}

func (m *mockOAuthRepo) FindUserByID(ctx context.Context, id int64) (*model.User, error) {
	if m.FindUserByIDFunc != nil {
		return m.FindUserByIDFunc(ctx, id)
	}
	return nil, sql.ErrNoRows
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func testConfig() config.OAuthConfig {
	return config.OAuthConfig{
		AuthCodeLifetime:     10 * time.Minute,
		AccessTokenLifetime:  1 * time.Hour,
		RefreshTokenLifetime: 24 * time.Hour,
		DeviceCodeLifetime:   15 * time.Minute,
		DeviceCodeInterval:   5 * time.Second,
	}
}

func testService(repo OAuthRepo) *Service {
	return NewService(repo, testConfig(), "https://app.example.com", slog.Default())
}

// makeTokenPlaintext generates a real token via GenerateToken, assigns it an ID,
// and returns the full plaintext string "id|random" along with the hash.
func makeTokenPlaintext(t *testing.T, id int64) (plaintext, hash string) {
	t.Helper()
	tp, err := GenerateToken()
	require.NoError(t, err)
	hash = tp.Hash
	tp.SetID(id)
	plaintext = tp.Plaintext
	return plaintext, hash
}

// s256Challenge computes the S256 PKCE challenge from a verifier.
func s256Challenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// makePublicClient returns a model.OAuthClient configured as a public client.
func makePublicClient(id int64, clientID string) *model.OAuthClient {
	return &model.OAuthClient{
		ID:                      id,
		ClientID:                clientID,
		TokenEndpointAuthMethod: "none",
		GrantTypes:              `["authorization_code"]`,
		IsActive:                true,
		CreatedAt:               sql.NullTime{Time: time.Now(), Valid: true},
	}
}

// makeConfidentialClient returns a model.OAuthClient configured as a confidential client.
func makeConfidentialClient(t *testing.T, id int64, clientID, plainSecret string) *model.OAuthClient {
	t.Helper()
	hashed, err := HashSecret(plainSecret)
	require.NoError(t, err)
	return &model.OAuthClient{
		ID:                      id,
		ClientID:                clientID,
		ClientSecret:            sql.NullString{String: hashed, Valid: true},
		TokenEndpointAuthMethod: "client_secret_post",
		GrantTypes:              `["authorization_code"]`,
		IsActive:                true,
		CreatedAt:               sql.NullTime{Time: time.Now(), Valid: true},
	}
}

// deviceClientID is the default client ID used by makeDeviceClient.
const deviceClientID = "550e8400-e29b-41d4-a716-446655440004"

// makeDeviceClient returns a model.OAuthClient that supports the device_code grant.
func makeDeviceClient() *model.OAuthClient {
	return &model.OAuthClient{
		ID:                      100,
		ClientID:                deviceClientID,
		TokenEndpointAuthMethod: "none",
		GrantTypes:              `["authorization_code","urn:ietf:params:oauth:grant-type:device_code"]`,
		IsActive:                true,
		CreatedAt:               sql.NullTime{Time: time.Now(), Valid: true},
	}
}

// ---------------------------------------------------------------------------
// TestRegisterClient
// ---------------------------------------------------------------------------

func TestRegisterClient(t *testing.T) {
	t.Parallel()

	t.Run("happy path with defaults for public client", func(t *testing.T) {
		t.Parallel()

		var capturedClient *model.OAuthClient
		repo := &mockOAuthRepo{
			CreateClientFunc: func(_ context.Context, client *model.OAuthClient) error {
				capturedClient = client
				client.ID = 1
				client.CreatedAt = sql.NullTime{Time: time.Now(), Valid: true}
				return nil
			},
		}
		svc := testService(repo)

		result, err := svc.RegisterClient(context.Background(), RegisterClientParams{
			ClientName:   "Test App",
			RedirectURIs: []string{"http://localhost:8080/callback"},
		})

		require.NoError(t, err)
		require.NotNil(t, result)

		assert.NotEmpty(t, result.ClientID, "client_id should be generated")
		assert.Equal(t, "Test App", result.ClientName)
		assert.Equal(t, []string{"http://localhost:8080/callback"}, result.RedirectURIs)
		assert.Equal(t, []string{"authorization_code"}, result.GrantTypes, "default grant_types")
		assert.Equal(t, []string{"code"}, result.ResponseTypes, "default response_types")
		assert.Equal(t, "none", result.TokenEndpointAuthMethod, "default auth method")
		assert.Equal(t, authscope.DefaultScopes(), result.Scope, "default scope")
		assert.Empty(t, result.ClientSecret, "public client has no secret")
		assert.Positive(t, result.ClientIDIssuedAt)

		// Verify persisted model
		require.NotNil(t, capturedClient)
		assert.True(t, capturedClient.IsActive)
		assert.False(t, capturedClient.ClientSecret.Valid, "public client stores no secret")
	})

	t.Run("confidential client receives secret", func(t *testing.T) {
		t.Parallel()

		repo := &mockOAuthRepo{
			CreateClientFunc: func(_ context.Context, client *model.OAuthClient) error {
				client.ID = 2
				client.CreatedAt = sql.NullTime{Time: time.Now(), Valid: true}
				return nil
			},
		}
		svc := testService(repo)

		result, err := svc.RegisterClient(context.Background(), RegisterClientParams{
			ClientName:              "Confidential App",
			RedirectURIs:            []string{"https://example.com/callback"},
			TokenEndpointAuthMethod: "client_secret_post",
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotEmpty(t, result.ClientSecret, "confidential client should receive a secret")
		assert.Equal(t, int64(0), result.ClientSecretExpiresAt)
		assert.Equal(t, "client_secret_post", result.TokenEndpointAuthMethod)
	})

	t.Run("custom grant types and response types are preserved", func(t *testing.T) {
		t.Parallel()

		repo := &mockOAuthRepo{
			CreateClientFunc: func(_ context.Context, client *model.OAuthClient) error {
				client.ID = 3
				client.CreatedAt = sql.NullTime{Time: time.Now(), Valid: true}
				return nil
			},
		}
		svc := testService(repo)

		result, err := svc.RegisterClient(context.Background(), RegisterClientParams{
			ClientName:    "Custom App",
			RedirectURIs:  []string{"https://example.com/cb"},
			GrantTypes:    []string{"authorization_code", "refresh_token"},
			ResponseTypes: []string{"code", "token"},
			Scope:         "documents:read documents:write",
		})

		require.NoError(t, err)
		assert.Equal(t, []string{"authorization_code", "refresh_token"}, result.GrantTypes)
		assert.Equal(t, []string{"code", "token"}, result.ResponseTypes)
		assert.Equal(t, "documents:read documents:write", result.Scope)
	})

	t.Run("database error propagates", func(t *testing.T) {
		t.Parallel()

		repo := &mockOAuthRepo{
			CreateClientFunc: func(_ context.Context, _ *model.OAuthClient) error {
				return errors.New("connection refused")
			},
		}
		svc := testService(repo)

		result, err := svc.RegisterClient(context.Background(), RegisterClientParams{
			ClientName:   "Failing App",
			RedirectURIs: []string{"http://localhost/cb"},
		})

		assert.Nil(t, result)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "creating client")
	})

	t.Run("software_id and software_version are stored", func(t *testing.T) {
		t.Parallel()

		var capturedClient *model.OAuthClient
		repo := &mockOAuthRepo{
			CreateClientFunc: func(_ context.Context, client *model.OAuthClient) error {
				capturedClient = client
				client.ID = 4
				client.CreatedAt = sql.NullTime{Time: time.Now(), Valid: true}
				return nil
			},
		}
		svc := testService(repo)

		_, err := svc.RegisterClient(context.Background(), RegisterClientParams{
			ClientName:      "MCP Client",
			RedirectURIs:    []string{"http://localhost/cb"},
			SoftwareID:      "550e8400-e29b-41d4-a716-446655440000",
			SoftwareVersion: "1.2.3",
		})

		require.NoError(t, err)
		require.NotNil(t, capturedClient)
		assert.True(t, capturedClient.SoftwareID.Valid)
		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", capturedClient.SoftwareID.String)
		assert.True(t, capturedClient.SoftwareVersion.Valid)
		assert.Equal(t, "1.2.3", capturedClient.SoftwareVersion.String)
	})

	t.Run("nil redirect URIs marshals to null JSON array", func(t *testing.T) {
		t.Parallel()

		var capturedClient *model.OAuthClient
		repo := &mockOAuthRepo{
			CreateClientFunc: func(_ context.Context, client *model.OAuthClient) error {
				capturedClient = client
				client.ID = 5
				client.CreatedAt = sql.NullTime{Time: time.Now(), Valid: true}
				return nil
			},
		}
		svc := testService(repo)

		_, err := svc.RegisterClient(context.Background(), RegisterClientParams{
			ClientName: "No Redirect",
		})

		require.NoError(t, err)
		require.NotNil(t, capturedClient)
		assert.Equal(t, "null", capturedClient.RedirectURIs)
	})

	t.Run("invalid scopes returns error", func(t *testing.T) {
		t.Parallel()

		repo := &mockOAuthRepo{}
		svc := testService(repo)

		_, err := svc.RegisterClient(context.Background(), RegisterClientParams{
			ClientName:   "Bad Scopes App",
			RedirectURIs: []string{"http://localhost/cb"},
			Scope:        "mcp:access bogus:scope",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid scopes")
		assert.Contains(t, err.Error(), "bogus:scope")
	})

	t.Run("valid custom scopes succeeds", func(t *testing.T) {
		t.Parallel()

		repo := &mockOAuthRepo{
			CreateClientFunc: func(_ context.Context, client *model.OAuthClient) error {
				client.ID = 6
				client.CreatedAt = sql.NullTime{Time: time.Now(), Valid: true}
				return nil
			},
		}
		svc := testService(repo)

		result, err := svc.RegisterClient(context.Background(), RegisterClientParams{
			ClientName:   "Valid Scopes App",
			RedirectURIs: []string{"http://localhost/cb"},
			Scope:        "mcp:access documents:read documents:write",
		})

		require.NoError(t, err)
		assert.Equal(t, "mcp:access documents:read documents:write", result.Scope)
	})
}

// ---------------------------------------------------------------------------
// TestGenerateAuthorizationCode
// ---------------------------------------------------------------------------

func TestGenerateAuthorizationCode(t *testing.T) {
	t.Parallel()

	t.Run("happy path returns parseable token", func(t *testing.T) {
		t.Parallel()

		var capturedCode *model.OAuthAuthorizationCode
		repo := &mockOAuthRepo{
			FindClientByIDFunc: func(_ context.Context, id int64) (*model.OAuthClient, error) {
				return &model.OAuthClient{
					ID:    id,
					Scope: sql.NullString{String: authscope.DefaultScopes(), Valid: true},
				}, nil
			},
			CreateAuthorizationCodeFunc: func(_ context.Context, code *model.OAuthAuthorizationCode) error {
				capturedCode = code
				code.ID = 10
				return nil
			},
		}
		svc := testService(repo)

		plaintext, err := svc.GenerateAuthorizationCode(context.Background(), GenerateAuthorizationCodeParams{
			ClientID:    1,
			UserID:      42,
			RedirectURI: "http://localhost:8080/callback",
			Scope:       "mcp:access",
		})

		require.NoError(t, err)
		assert.NotEmpty(t, plaintext)

		// Parse the returned token
		id, hash, parseErr := ParseToken(plaintext)
		require.NoError(t, parseErr)
		assert.Equal(t, int64(10), id)
		assert.Equal(t, capturedCode.Code, hash, "hash stored in DB should match parsed hash")
	})

	t.Run("stores PKCE challenge", func(t *testing.T) {
		t.Parallel()

		var capturedCode *model.OAuthAuthorizationCode
		repo := &mockOAuthRepo{
			CreateAuthorizationCodeFunc: func(_ context.Context, code *model.OAuthAuthorizationCode) error {
				capturedCode = code
				code.ID = 11
				return nil
			},
		}
		svc := testService(repo)

		verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
		challenge := s256Challenge(verifier)

		_, err := svc.GenerateAuthorizationCode(context.Background(), GenerateAuthorizationCodeParams{
			ClientID:            1,
			UserID:              42,
			RedirectURI:         "http://localhost:8080/callback",
			CodeChallenge:       challenge,
			CodeChallengeMethod: "S256",
		})

		require.NoError(t, err)
		require.NotNil(t, capturedCode)
		assert.True(t, capturedCode.CodeChallenge.Valid)
		assert.Equal(t, challenge, capturedCode.CodeChallenge.String)
		assert.True(t, capturedCode.CodeChallengeMethod.Valid)
		assert.Equal(t, "S256", capturedCode.CodeChallengeMethod.String)
	})

	t.Run("stores scope when provided", func(t *testing.T) {
		t.Parallel()

		var capturedCode *model.OAuthAuthorizationCode
		repo := &mockOAuthRepo{
			FindClientByIDFunc: func(_ context.Context, id int64) (*model.OAuthClient, error) {
				return &model.OAuthClient{
					ID:    id,
					Scope: sql.NullString{String: "documents:read documents:write", Valid: true},
				}, nil
			},
			CreateAuthorizationCodeFunc: func(_ context.Context, code *model.OAuthAuthorizationCode) error {
				capturedCode = code
				code.ID = 12
				return nil
			},
		}
		svc := testService(repo)

		_, err := svc.GenerateAuthorizationCode(context.Background(), GenerateAuthorizationCodeParams{
			ClientID:    1,
			UserID:      42,
			RedirectURI: "http://localhost/callback",
			Scope:       "documents:read documents:write",
		})

		require.NoError(t, err)
		require.NotNil(t, capturedCode)
		assert.True(t, capturedCode.Scope.Valid)
		assert.Equal(t, "documents:read documents:write", capturedCode.Scope.String)
	})

	t.Run("empty scope sets null", func(t *testing.T) {
		t.Parallel()

		var capturedCode *model.OAuthAuthorizationCode
		repo := &mockOAuthRepo{
			CreateAuthorizationCodeFunc: func(_ context.Context, code *model.OAuthAuthorizationCode) error {
				capturedCode = code
				code.ID = 13
				return nil
			},
		}
		svc := testService(repo)

		_, err := svc.GenerateAuthorizationCode(context.Background(), GenerateAuthorizationCodeParams{
			ClientID:    1,
			UserID:      42,
			RedirectURI: "http://localhost/callback",
			Scope:       "",
		})

		require.NoError(t, err)
		require.NotNil(t, capturedCode)
		assert.False(t, capturedCode.Scope.Valid)
	})

	t.Run("database error propagates", func(t *testing.T) {
		t.Parallel()

		repo := &mockOAuthRepo{
			CreateAuthorizationCodeFunc: func(_ context.Context, _ *model.OAuthAuthorizationCode) error {
				return errors.New("disk full")
			},
		}
		svc := testService(repo)

		_, err := svc.GenerateAuthorizationCode(context.Background(), GenerateAuthorizationCodeParams{
			ClientID:    1,
			UserID:      42,
			RedirectURI: "http://localhost/callback",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "persisting authorization code")
	})

	t.Run("sets expiration from config", func(t *testing.T) {
		t.Parallel()

		var capturedCode *model.OAuthAuthorizationCode
		repo := &mockOAuthRepo{
			CreateAuthorizationCodeFunc: func(_ context.Context, code *model.OAuthAuthorizationCode) error {
				capturedCode = code
				code.ID = 14
				return nil
			},
		}
		svc := testService(repo)

		before := time.Now()
		_, err := svc.GenerateAuthorizationCode(context.Background(), GenerateAuthorizationCodeParams{
			ClientID:    1,
			UserID:      42,
			RedirectURI: "http://localhost/callback",
		})

		require.NoError(t, err)
		require.NotNil(t, capturedCode)

		expectedExpiry := before.Add(10 * time.Minute)
		assert.WithinDuration(t, expectedExpiry, capturedCode.ExpiresAt, 2*time.Second)
		assert.False(t, capturedCode.Revoked)
	})

	t.Run("invalid scope returns error", func(t *testing.T) {
		t.Parallel()

		repo := &mockOAuthRepo{}
		svc := testService(repo)

		_, err := svc.GenerateAuthorizationCode(context.Background(), GenerateAuthorizationCodeParams{
			ClientID:    1,
			UserID:      42,
			RedirectURI: "http://localhost/callback",
			Scope:       "mcp:access bogus:scope",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid scopes")
		assert.Contains(t, err.Error(), "bogus:scope")
	})

	t.Run("scope exceeding client scope succeeds (no client scope check)", func(t *testing.T) {
		t.Parallel()

		repo := &mockOAuthRepo{
			CreateAuthorizationCodeFunc: func(_ context.Context, code *model.OAuthAuthorizationCode) error {
				code.ID = 15
				return nil
			},
		}
		svc := testService(repo)

		plaintext, err := svc.GenerateAuthorizationCode(context.Background(), GenerateAuthorizationCodeParams{
			ClientID:    1,
			UserID:      42,
			RedirectURI: "http://localhost/callback",
			Scope:       "documents:read documents:write",
		})

		require.NoError(t, err)
		assert.NotEmpty(t, plaintext)
	})

	t.Run("valid scope subset of client scope succeeds", func(t *testing.T) {
		t.Parallel()

		repo := &mockOAuthRepo{
			CreateAuthorizationCodeFunc: func(_ context.Context, code *model.OAuthAuthorizationCode) error {
				code.ID = 16
				return nil
			},
		}
		svc := testService(repo)

		plaintext, err := svc.GenerateAuthorizationCode(context.Background(), GenerateAuthorizationCodeParams{
			ClientID:    1,
			UserID:      42,
			RedirectURI: "http://localhost/callback",
			Scope:       "documents:read",
		})

		require.NoError(t, err)
		assert.NotEmpty(t, plaintext)
	})
}

// ---------------------------------------------------------------------------
// TestExchangeAuthorizationCode
// ---------------------------------------------------------------------------

func TestExchangeAuthorizationCode(t *testing.T) {
	t.Parallel()

	const (
		testClientID     = "550e8400-e29b-41d4-a716-446655440001"
		testRedirectURI  = "http://localhost:8080/callback"
		testClientDBID   = int64(100)
		testAuthCodeDBID = int64(200)
	)

	// Helper to build a valid auth code + token pair for the happy path
	setupValidExchange := func(t *testing.T) (codePlaintext string, codeHash string, client *model.OAuthClient, authCode *model.OAuthAuthorizationCode) {
		t.Helper()
		codePlaintext, codeHash = makeTokenPlaintext(t, testAuthCodeDBID)
		client = makePublicClient(testClientDBID, testClientID)
		authCode = &model.OAuthAuthorizationCode{
			ID:          testAuthCodeDBID,
			Code:        codeHash,
			ClientID:    testClientDBID,
			UserID:      sql.NullInt64{Int64: 42, Valid: true},
			RedirectURI: testRedirectURI,
			Scope:       sql.NullString{String: "mcp:access", Valid: true},
			ExpiresAt:   time.Now().Add(5 * time.Minute),
			Revoked:     false,
		}
		return
	}

	t.Run("happy path issues access and refresh tokens", func(t *testing.T) {
		t.Parallel()

		codePlaintext, codeHash, client, authCode := setupValidExchange(t)

		var accessTokenCreated, refreshTokenCreated bool
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, cid string) (*model.OAuthClient, error) {
				if cid == testClientID {
					return client, nil
				}
				return nil, sql.ErrNoRows
			},
			FindAuthorizationCodeByCodeFunc: func(_ context.Context, hash string) (*model.OAuthAuthorizationCode, error) {
				if hash == codeHash {
					return authCode, nil
				}
				return nil, sql.ErrNoRows
			},
			RevokeAuthorizationCodeFunc: func(_ context.Context, id int64) error {
				assert.Equal(t, testAuthCodeDBID, id)
				return nil
			},
			CreateAccessTokenFunc: func(_ context.Context, token *model.OAuthAccessToken) error {
				accessTokenCreated = true
				token.ID = 300
				assert.Equal(t, testClientDBID, token.ClientID)
				assert.Equal(t, int64(42), token.UserID.Int64)
				assert.Equal(t, "mcp:access", token.Scope.String)
				return nil
			},
			CreateRefreshTokenFunc: func(_ context.Context, token *model.OAuthRefreshToken) error {
				refreshTokenCreated = true
				token.ID = 400
				assert.Equal(t, int64(300), token.AccessTokenID)
				return nil
			},
		}
		svc := testService(repo)

		result, err := svc.ExchangeAuthorizationCode(context.Background(), ExchangeAuthorizationCodeParams{
			Code:        codePlaintext,
			ClientID:    testClientID,
			RedirectURI: testRedirectURI,
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotEmpty(t, result.AccessToken)
		assert.NotEmpty(t, result.RefreshToken)
		assert.Equal(t, "Bearer", result.TokenType)
		assert.Equal(t, int(testConfig().AccessTokenLifetime.Seconds()), result.ExpiresIn)
		assert.Equal(t, "mcp:access", result.Scope)
		assert.True(t, accessTokenCreated, "access token should have been created")
		assert.True(t, refreshTokenCreated, "refresh token should have been created")
	})

	t.Run("invalid code format returns error", func(t *testing.T) {
		t.Parallel()
		svc := testService(&mockOAuthRepo{})

		_, err := svc.ExchangeAuthorizationCode(context.Background(), ExchangeAuthorizationCodeParams{
			Code:     "not-a-valid-token",
			ClientID: testClientID,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid authorization code")
	})

	t.Run("unknown client returns error", func(t *testing.T) {
		t.Parallel()

		codePlaintext, _, _, _ := setupValidExchange(t)

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return nil, sql.ErrNoRows
			},
		}
		svc := testService(repo)

		_, err := svc.ExchangeAuthorizationCode(context.Background(), ExchangeAuthorizationCodeParams{
			Code:     codePlaintext,
			ClientID: "unknown-client-id",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid client credentials")
	})

	t.Run("confidential client with wrong secret returns error", func(t *testing.T) {
		t.Parallel()

		codePlaintext, _, _, _ := setupValidExchange(t)
		client := makeConfidentialClient(t, testClientDBID, testClientID, "correct-secret")

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
		}
		svc := testService(repo)

		_, err := svc.ExchangeAuthorizationCode(context.Background(), ExchangeAuthorizationCodeParams{
			Code:         codePlaintext,
			ClientID:     testClientID,
			ClientSecret: "wrong-secret",
			RedirectURI:  testRedirectURI,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid client credentials")
	})

	t.Run("confidential client with correct secret succeeds", func(t *testing.T) {
		t.Parallel()

		codePlaintext, codeHash, _, authCode := setupValidExchange(t)
		client := makeConfidentialClient(t, testClientDBID, testClientID, "correct-secret")

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
			FindAuthorizationCodeByCodeFunc: func(_ context.Context, hash string) (*model.OAuthAuthorizationCode, error) {
				if hash == codeHash {
					return authCode, nil
				}
				return nil, sql.ErrNoRows
			},
			RevokeAuthorizationCodeFunc: func(_ context.Context, _ int64) error { return nil },
			CreateAccessTokenFunc:       func(_ context.Context, t *model.OAuthAccessToken) error { t.ID = 301; return nil },
			CreateRefreshTokenFunc:      func(_ context.Context, t *model.OAuthRefreshToken) error { t.ID = 401; return nil },
		}
		svc := testService(repo)

		result, err := svc.ExchangeAuthorizationCode(context.Background(), ExchangeAuthorizationCodeParams{
			Code:         codePlaintext,
			ClientID:     testClientID,
			ClientSecret: "correct-secret",
			RedirectURI:  testRedirectURI,
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotEmpty(t, result.AccessToken)
	})

	t.Run("expired authorization code returns error", func(t *testing.T) {
		t.Parallel()

		codePlaintext, codeHash, client, authCode := setupValidExchange(t)
		authCode.ExpiresAt = time.Now().Add(-1 * time.Minute) // expired

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
			FindAuthorizationCodeByCodeFunc: func(_ context.Context, hash string) (*model.OAuthAuthorizationCode, error) {
				if hash == codeHash {
					return authCode, nil
				}
				return nil, sql.ErrNoRows
			},
		}
		svc := testService(repo)

		_, err := svc.ExchangeAuthorizationCode(context.Background(), ExchangeAuthorizationCodeParams{
			Code:        codePlaintext,
			ClientID:    testClientID,
			RedirectURI: testRedirectURI,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "expired or revoked")
	})

	t.Run("redirect URI mismatch returns error", func(t *testing.T) {
		t.Parallel()

		codePlaintext, codeHash, client, authCode := setupValidExchange(t)

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
			FindAuthorizationCodeByCodeFunc: func(_ context.Context, hash string) (*model.OAuthAuthorizationCode, error) {
				if hash == codeHash {
					return authCode, nil
				}
				return nil, sql.ErrNoRows
			},
		}
		svc := testService(repo)

		_, err := svc.ExchangeAuthorizationCode(context.Background(), ExchangeAuthorizationCodeParams{
			Code:        codePlaintext,
			ClientID:    testClientID,
			RedirectURI: "https://evil.com/callback",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "redirect URI mismatch")
	})

	t.Run("PKCE valid verifier succeeds", func(t *testing.T) {
		t.Parallel()

		verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
		challenge := s256Challenge(verifier)

		codePlaintext, codeHash, client, authCode := setupValidExchange(t)
		authCode.CodeChallenge = sql.NullString{String: challenge, Valid: true}
		authCode.CodeChallengeMethod = sql.NullString{String: "S256", Valid: true}

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
			FindAuthorizationCodeByCodeFunc: func(_ context.Context, hash string) (*model.OAuthAuthorizationCode, error) {
				if hash == codeHash {
					return authCode, nil
				}
				return nil, sql.ErrNoRows
			},
			RevokeAuthorizationCodeFunc: func(_ context.Context, _ int64) error { return nil },
			CreateAccessTokenFunc:       func(_ context.Context, t *model.OAuthAccessToken) error { t.ID = 302; return nil },
			CreateRefreshTokenFunc:      func(_ context.Context, t *model.OAuthRefreshToken) error { t.ID = 402; return nil },
		}
		svc := testService(repo)

		result, err := svc.ExchangeAuthorizationCode(context.Background(), ExchangeAuthorizationCodeParams{
			Code:         codePlaintext,
			ClientID:     testClientID,
			RedirectURI:  testRedirectURI,
			CodeVerifier: verifier,
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotEmpty(t, result.AccessToken)
	})

	t.Run("PKCE invalid verifier returns error", func(t *testing.T) {
		t.Parallel()

		verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
		challenge := s256Challenge(verifier)

		codePlaintext, codeHash, client, authCode := setupValidExchange(t)
		authCode.CodeChallenge = sql.NullString{String: challenge, Valid: true}
		authCode.CodeChallengeMethod = sql.NullString{String: "S256", Valid: true}

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
			FindAuthorizationCodeByCodeFunc: func(_ context.Context, hash string) (*model.OAuthAuthorizationCode, error) {
				if hash == codeHash {
					return authCode, nil
				}
				return nil, sql.ErrNoRows
			},
		}
		svc := testService(repo)

		_, err := svc.ExchangeAuthorizationCode(context.Background(), ExchangeAuthorizationCodeParams{
			Code:         codePlaintext,
			ClientID:     testClientID,
			RedirectURI:  testRedirectURI,
			CodeVerifier: "wrong-verifier",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid PKCE code verifier")
	})

	t.Run("PKCE challenge present but no verifier returns error", func(t *testing.T) {
		t.Parallel()

		codePlaintext, codeHash, client, authCode := setupValidExchange(t)
		authCode.CodeChallenge = sql.NullString{String: "some-challenge", Valid: true}

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
			FindAuthorizationCodeByCodeFunc: func(_ context.Context, hash string) (*model.OAuthAuthorizationCode, error) {
				if hash == codeHash {
					return authCode, nil
				}
				return nil, sql.ErrNoRows
			},
		}
		svc := testService(repo)

		_, err := svc.ExchangeAuthorizationCode(context.Background(), ExchangeAuthorizationCodeParams{
			Code:        codePlaintext,
			ClientID:    testClientID,
			RedirectURI: testRedirectURI,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "code verifier required")
	})

	t.Run("code_verifier without code_challenge returns error per RFC 9700", func(t *testing.T) {
		t.Parallel()

		codePlaintext, codeHash, client, authCode := setupValidExchange(t)
		// No code_challenge on the auth code
		authCode.CodeChallenge = sql.NullString{Valid: false}

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
			FindAuthorizationCodeByCodeFunc: func(_ context.Context, hash string) (*model.OAuthAuthorizationCode, error) {
				if hash == codeHash {
					return authCode, nil
				}
				return nil, sql.ErrNoRows
			},
		}
		svc := testService(repo)

		_, err := svc.ExchangeAuthorizationCode(context.Background(), ExchangeAuthorizationCodeParams{
			Code:         codePlaintext,
			ClientID:     testClientID,
			RedirectURI:  testRedirectURI,
			CodeVerifier: "unexpected-verifier",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected code_verifier")
	})

	t.Run("code belonging to different client returns error", func(t *testing.T) {
		t.Parallel()

		codePlaintext, codeHash, client, authCode := setupValidExchange(t)
		authCode.ClientID = 999 // Different client

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
			FindAuthorizationCodeByCodeFunc: func(_ context.Context, hash string) (*model.OAuthAuthorizationCode, error) {
				if hash == codeHash {
					return authCode, nil
				}
				return nil, sql.ErrNoRows
			},
		}
		svc := testService(repo)

		_, err := svc.ExchangeAuthorizationCode(context.Background(), ExchangeAuthorizationCodeParams{
			Code:        codePlaintext,
			ClientID:    testClientID,
			RedirectURI: testRedirectURI,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid authorization code")
	})

	t.Run("code not found in database returns error", func(t *testing.T) {
		t.Parallel()

		codePlaintext, _, client, _ := setupValidExchange(t)

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
			FindAuthorizationCodeByCodeFunc: func(_ context.Context, _ string) (*model.OAuthAuthorizationCode, error) {
				return nil, sql.ErrNoRows
			},
		}
		svc := testService(repo)

		_, err := svc.ExchangeAuthorizationCode(context.Background(), ExchangeAuthorizationCodeParams{
			Code:        codePlaintext,
			ClientID:    testClientID,
			RedirectURI: testRedirectURI,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid authorization code")
	})

	t.Run("auth code scope exceeding client scope logs warning but succeeds", func(t *testing.T) {
		t.Parallel()

		codePlaintext, codeHash, client, authCode := setupValidExchange(t)
		// Client only allows documents:read, but auth code has broader scope
		client.Scope = sql.NullString{String: "documents:read", Valid: true}
		authCode.Scope = sql.NullString{String: "documents:read documents:write", Valid: true}

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
			FindAuthorizationCodeByCodeFunc: func(_ context.Context, hash string) (*model.OAuthAuthorizationCode, error) {
				if hash == codeHash {
					return authCode, nil
				}
				return nil, sql.ErrNoRows
			},
			RevokeAuthorizationCodeFunc: func(_ context.Context, _ int64) error { return nil },
			CreateAccessTokenFunc: func(_ context.Context, token *model.OAuthAccessToken) error {
				token.ID = 300
				return nil
			},
			CreateRefreshTokenFunc: func(_ context.Context, token *model.OAuthRefreshToken) error {
				token.ID = 400
				return nil
			},
		}
		svc := testService(repo)

		result, err := svc.ExchangeAuthorizationCode(context.Background(), ExchangeAuthorizationCodeParams{
			Code:        codePlaintext,
			ClientID:    testClientID,
			RedirectURI: testRedirectURI,
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "documents:read documents:write", result.Scope)
	})
}

// ---------------------------------------------------------------------------
// TestRefreshAccessToken
// ---------------------------------------------------------------------------

func TestRefreshAccessToken(t *testing.T) {
	t.Parallel()

	const (
		testClientID   = "550e8400-e29b-41d4-a716-446655440002"
		testClientDBID = int64(100)
	)

	setupValidRefresh := func(t *testing.T) (refreshPlaintext string, refreshHash string, client *model.OAuthClient, refreshToken *model.OAuthRefreshToken, accessToken *model.OAuthAccessToken) {
		t.Helper()
		refreshPlaintext, refreshHash = makeTokenPlaintext(t, 500)
		client = makePublicClient(testClientDBID, testClientID)
		accessToken = &model.OAuthAccessToken{
			ID:       300,
			Token:    "access-hash",
			ClientID: testClientDBID,
			UserID:   sql.NullInt64{Int64: 42, Valid: true},
			Scope:    sql.NullString{String: "documents:read documents:write", Valid: true},
		}
		refreshToken = &model.OAuthRefreshToken{
			ID:            500,
			Token:         refreshHash,
			AccessTokenID: 300,
		}
		return
	}

	t.Run("happy path rotates tokens", func(t *testing.T) {
		t.Parallel()

		refreshPlaintext, refreshHash, client, refreshTok, accessTok := setupValidRefresh(t)

		var oldAccessRevoked, oldRefreshRevoked bool
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
			FindRefreshTokenByTokenFunc: func(_ context.Context, hash string) (*model.OAuthRefreshToken, error) {
				if hash == refreshHash {
					return refreshTok, nil
				}
				return nil, sql.ErrNoRows
			},
			FindAccessTokenByIDFunc: func(_ context.Context, id int64) (*model.OAuthAccessToken, error) {
				if id == 300 {
					return accessTok, nil
				}
				return nil, sql.ErrNoRows
			},
			RevokeTokenPairFunc: func(_ context.Context, accessID, refreshID int64) error {
				if accessID == 300 {
					oldAccessRevoked = true
				}
				if refreshID == 500 {
					oldRefreshRevoked = true
				}
				return nil
			},
			CreateAccessTokenFunc:  func(_ context.Context, t *model.OAuthAccessToken) error { t.ID = 301; return nil },
			CreateRefreshTokenFunc: func(_ context.Context, t *model.OAuthRefreshToken) error { t.ID = 501; return nil },
		}
		svc := testService(repo)

		result, err := svc.RefreshAccessToken(context.Background(), RefreshTokenParams{
			RefreshToken: refreshPlaintext,
			ClientID:     testClientID,
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotEmpty(t, result.AccessToken)
		assert.NotEmpty(t, result.RefreshToken)
		assert.Equal(t, "Bearer", result.TokenType)
		assert.Equal(t, "documents:read documents:write", result.Scope)
		assert.True(t, oldAccessRevoked, "old access token should be revoked")
		assert.True(t, oldRefreshRevoked, "old refresh token should be revoked")
	})

	t.Run("scope narrowing succeeds", func(t *testing.T) {
		t.Parallel()

		refreshPlaintext, refreshHash, client, refreshTok, accessTok := setupValidRefresh(t)

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
			FindRefreshTokenByTokenFunc: func(_ context.Context, hash string) (*model.OAuthRefreshToken, error) {
				if hash == refreshHash {
					return refreshTok, nil
				}
				return nil, sql.ErrNoRows
			},
			FindAccessTokenByIDFunc: func(_ context.Context, id int64) (*model.OAuthAccessToken, error) {
				if id == 300 {
					return accessTok, nil
				}
				return nil, sql.ErrNoRows
			},
			RevokeAccessTokenFunc:  func(_ context.Context, _ int64) error { return nil },
			RevokeRefreshTokenFunc: func(_ context.Context, _ int64) error { return nil },
			CreateAccessTokenFunc: func(_ context.Context, tok *model.OAuthAccessToken) error {
				tok.ID = 302
				return nil
			},
			CreateRefreshTokenFunc: func(_ context.Context, tok *model.OAuthRefreshToken) error { tok.ID = 502; return nil },
		}
		svc := testService(repo)

		result, err := svc.RefreshAccessToken(context.Background(), RefreshTokenParams{
			RefreshToken: refreshPlaintext,
			ClientID:     testClientID,
			Scope:        "documents:read",
		})

		require.NoError(t, err)
		assert.Equal(t, "documents:read", result.Scope)
	})

	t.Run("scope widening returns error", func(t *testing.T) {
		t.Parallel()

		refreshPlaintext, refreshHash, client, refreshTok, accessTok := setupValidRefresh(t)

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
			FindRefreshTokenByTokenFunc: func(_ context.Context, hash string) (*model.OAuthRefreshToken, error) {
				if hash == refreshHash {
					return refreshTok, nil
				}
				return nil, sql.ErrNoRows
			},
			FindAccessTokenByIDFunc: func(_ context.Context, id int64) (*model.OAuthAccessToken, error) {
				if id == 300 {
					return accessTok, nil
				}
				return nil, sql.ErrNoRows
			},
			RevokeAccessTokenFunc:  func(_ context.Context, _ int64) error { return nil },
			RevokeRefreshTokenFunc: func(_ context.Context, _ int64) error { return nil },
		}
		svc := testService(repo)

		_, err := svc.RefreshAccessToken(context.Background(), RefreshTokenParams{
			RefreshToken: refreshPlaintext,
			ClientID:     testClientID,
			Scope:        "documents:read documents:write admin",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "requested scope exceeds original grant")
	})

	t.Run("invalid refresh token format returns error", func(t *testing.T) {
		t.Parallel()

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return makePublicClient(testClientDBID, testClientID), nil
			},
		}
		svc := testService(repo)

		_, err := svc.RefreshAccessToken(context.Background(), RefreshTokenParams{
			RefreshToken: "bad-format",
			ClientID:     testClientID,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "refresh token not found")
	})

	t.Run("unknown client returns error", func(t *testing.T) {
		t.Parallel()

		refreshPlaintext, _, _, _, _ := setupValidRefresh(t)

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return nil, sql.ErrNoRows
			},
		}
		svc := testService(repo)

		_, err := svc.RefreshAccessToken(context.Background(), RefreshTokenParams{
			RefreshToken: refreshPlaintext,
			ClientID:     "unknown",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid client credentials")
	})

	t.Run("refresh token belonging to different client returns error", func(t *testing.T) {
		t.Parallel()

		refreshPlaintext, refreshHash, client, refreshTok, accessTok := setupValidRefresh(t)
		accessTok.ClientID = 999 // Different client

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
			FindRefreshTokenByTokenFunc: func(_ context.Context, hash string) (*model.OAuthRefreshToken, error) {
				if hash == refreshHash {
					return refreshTok, nil
				}
				return nil, sql.ErrNoRows
			},
			FindAccessTokenByIDFunc: func(_ context.Context, id int64) (*model.OAuthAccessToken, error) {
				if id == 300 {
					return accessTok, nil
				}
				return nil, sql.ErrNoRows
			},
		}
		svc := testService(repo)

		_, err := svc.RefreshAccessToken(context.Background(), RefreshTokenParams{
			RefreshToken: refreshPlaintext,
			ClientID:     testClientID,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "refresh token does not belong to this client")
	})

	t.Run("refresh token not found returns error", func(t *testing.T) {
		t.Parallel()

		refreshPlaintext, _, client, _, _ := setupValidRefresh(t)

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
			FindRefreshTokenByTokenFunc: func(_ context.Context, _ string) (*model.OAuthRefreshToken, error) {
				return nil, sql.ErrNoRows
			},
		}
		svc := testService(repo)

		_, err := svc.RefreshAccessToken(context.Background(), RefreshTokenParams{
			RefreshToken: refreshPlaintext,
			ClientID:     testClientID,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "refresh token not found")
	})

	t.Run("no scope specified keeps original scope", func(t *testing.T) {
		t.Parallel()

		refreshPlaintext, refreshHash, client, refreshTok, accessTok := setupValidRefresh(t)

		var capturedScope string
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
			FindRefreshTokenByTokenFunc: func(_ context.Context, hash string) (*model.OAuthRefreshToken, error) {
				if hash == refreshHash {
					return refreshTok, nil
				}
				return nil, sql.ErrNoRows
			},
			FindAccessTokenByIDFunc: func(_ context.Context, id int64) (*model.OAuthAccessToken, error) {
				if id == 300 {
					return accessTok, nil
				}
				return nil, sql.ErrNoRows
			},
			RevokeAccessTokenFunc:  func(_ context.Context, _ int64) error { return nil },
			RevokeRefreshTokenFunc: func(_ context.Context, _ int64) error { return nil },
			CreateAccessTokenFunc: func(_ context.Context, t *model.OAuthAccessToken) error {
				t.ID = 303
				capturedScope = t.Scope.String
				return nil
			},
			CreateRefreshTokenFunc: func(_ context.Context, t *model.OAuthRefreshToken) error { t.ID = 503; return nil },
		}
		svc := testService(repo)

		result, err := svc.RefreshAccessToken(context.Background(), RefreshTokenParams{
			RefreshToken: refreshPlaintext,
			ClientID:     testClientID,
			Scope:        "", // no scope specified
		})

		require.NoError(t, err)
		assert.Equal(t, "documents:read documents:write", result.Scope)
		assert.Equal(t, "documents:read documents:write", capturedScope)
	})
}

// ---------------------------------------------------------------------------
// TestRevokeToken
// ---------------------------------------------------------------------------

func TestRevokeToken(t *testing.T) {
	t.Parallel()

	const (
		testClientID   = "550e8400-e29b-41d4-a716-446655440003"
		testClientDBID = int64(100)
	)

	t.Run("access_token hint revokes access token and associated refresh tokens", func(t *testing.T) {
		t.Parallel()

		tokenPlaintext, tokenHash := makeTokenPlaintext(t, 300)
		client := makePublicClient(testClientDBID, testClientID)

		var accessRevoked, refreshByCascade bool
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
			FindAccessTokenByTokenFunc: func(_ context.Context, hash string) (*model.OAuthAccessToken, error) {
				if hash == tokenHash {
					return &model.OAuthAccessToken{ID: 300, ClientID: testClientDBID}, nil
				}
				return nil, sql.ErrNoRows
			},
			RevokeAccessTokenFunc: func(_ context.Context, id int64) error {
				if id == 300 {
					accessRevoked = true
				}
				return nil
			},
			RevokeRefreshTokenByAccessTokenIDFunc: func(_ context.Context, atID int64) error {
				if atID == 300 {
					refreshByCascade = true
				}
				return nil
			},
		}
		svc := testService(repo)

		err := svc.RevokeToken(context.Background(), RevokeTokenParams{
			Token:         tokenPlaintext,
			ClientID:      testClientID,
			TokenTypeHint: "access_token",
		})

		require.NoError(t, err)
		assert.True(t, accessRevoked, "access token should be revoked")
		assert.True(t, refreshByCascade, "associated refresh tokens should be cascade-revoked")
	})

	t.Run("refresh_token hint revokes refresh and associated access token", func(t *testing.T) {
		t.Parallel()

		tokenPlaintext, tokenHash := makeTokenPlaintext(t, 500)
		client := makePublicClient(testClientDBID, testClientID)

		var refreshRevoked, accessRevoked bool
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
			FindRefreshTokenByTokenFunc: func(_ context.Context, hash string) (*model.OAuthRefreshToken, error) {
				if hash == tokenHash {
					return &model.OAuthRefreshToken{ID: 500, AccessTokenID: 300}, nil
				}
				return nil, sql.ErrNoRows
			},
			FindAccessTokenByIDFunc: func(_ context.Context, id int64) (*model.OAuthAccessToken, error) {
				if id == 300 {
					return &model.OAuthAccessToken{ID: 300, ClientID: testClientDBID}, nil
				}
				return nil, sql.ErrNoRows
			},
			RevokeRefreshTokenFunc: func(_ context.Context, id int64) error {
				if id == 500 {
					refreshRevoked = true
				}
				return nil
			},
			RevokeAccessTokenFunc: func(_ context.Context, id int64) error {
				if id == 300 {
					accessRevoked = true
				}
				return nil
			},
		}
		svc := testService(repo)

		err := svc.RevokeToken(context.Background(), RevokeTokenParams{
			Token:         tokenPlaintext,
			ClientID:      testClientID,
			TokenTypeHint: "refresh_token",
		})

		require.NoError(t, err)
		assert.True(t, refreshRevoked, "refresh token should be revoked")
		assert.True(t, accessRevoked, "associated access token should be revoked")
	})

	t.Run("no hint tries access first then refresh", func(t *testing.T) {
		t.Parallel()

		tokenPlaintext, tokenHash := makeTokenPlaintext(t, 300)
		client := makePublicClient(testClientDBID, testClientID)

		var accessLookedUp, refreshLookedUp bool
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
			FindAccessTokenByTokenFunc: func(_ context.Context, hash string) (*model.OAuthAccessToken, error) {
				accessLookedUp = true
				if hash == tokenHash {
					return &model.OAuthAccessToken{ID: 300, ClientID: testClientDBID}, nil
				}
				return nil, sql.ErrNoRows
			},
			RevokeAccessTokenFunc:                 func(_ context.Context, _ int64) error { return nil },
			RevokeRefreshTokenByAccessTokenIDFunc: func(_ context.Context, _ int64) error { return nil },
			FindRefreshTokenByTokenFunc: func(_ context.Context, _ string) (*model.OAuthRefreshToken, error) {
				refreshLookedUp = true
				return nil, sql.ErrNoRows
			},
		}
		svc := testService(repo)

		err := svc.RevokeToken(context.Background(), RevokeTokenParams{
			Token:    tokenPlaintext,
			ClientID: testClientID,
		})

		require.NoError(t, err)
		assert.True(t, accessLookedUp, "should try access token")
		assert.True(t, refreshLookedUp, "should also try refresh token")
	})

	t.Run("invalid token format succeeds per RFC 7009", func(t *testing.T) {
		t.Parallel()

		client := makePublicClient(testClientDBID, testClientID)
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
		}
		svc := testService(repo)

		err := svc.RevokeToken(context.Background(), RevokeTokenParams{
			Token:    "not-a-valid-token",
			ClientID: testClientID,
		})

		assert.NoError(t, err, "RFC 7009 requires success even for invalid tokens")
	})

	t.Run("token not found succeeds per RFC 7009", func(t *testing.T) {
		t.Parallel()

		tokenPlaintext, _ := makeTokenPlaintext(t, 999)
		client := makePublicClient(testClientDBID, testClientID)

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
			FindAccessTokenByTokenFunc: func(_ context.Context, _ string) (*model.OAuthAccessToken, error) {
				return nil, sql.ErrNoRows
			},
			FindRefreshTokenByTokenFunc: func(_ context.Context, _ string) (*model.OAuthRefreshToken, error) {
				return nil, sql.ErrNoRows
			},
		}
		svc := testService(repo)

		err := svc.RevokeToken(context.Background(), RevokeTokenParams{
			Token:    tokenPlaintext,
			ClientID: testClientID,
		})

		assert.NoError(t, err, "RFC 7009 requires success even when token not found")
	})

	t.Run("invalid client returns error", func(t *testing.T) {
		t.Parallel()

		tokenPlaintext, _ := makeTokenPlaintext(t, 300)

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return nil, sql.ErrNoRows
			},
		}
		svc := testService(repo)

		err := svc.RevokeToken(context.Background(), RevokeTokenParams{
			Token:    tokenPlaintext,
			ClientID: "unknown-client",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid client credentials")
	})
}

// ---------------------------------------------------------------------------
// TestValidateAccessToken
// ---------------------------------------------------------------------------

func TestValidateAccessToken(t *testing.T) {
	t.Parallel()

	t.Run("valid token returns access token model", func(t *testing.T) {
		t.Parallel()

		tokenPlaintext, tokenHash := makeTokenPlaintext(t, 300)

		expectedToken := &model.OAuthAccessToken{
			ID:       300,
			Token:    tokenHash,
			ClientID: 100,
			UserID:   sql.NullInt64{Int64: 42, Valid: true},
			Scope:    sql.NullString{String: "mcp:access", Valid: true},
		}

		repo := &mockOAuthRepo{
			FindAccessTokenByTokenFunc: func(_ context.Context, hash string) (*model.OAuthAccessToken, error) {
				if hash == tokenHash {
					return expectedToken, nil
				}
				return nil, sql.ErrNoRows
			},
		}
		svc := testService(repo)

		result, err := svc.ValidateAccessToken(context.Background(), tokenPlaintext)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, int64(300), result.ID)
		assert.Equal(t, int64(100), result.ClientID)
		assert.Equal(t, "mcp:access", result.Scope.String)
	})

	t.Run("invalid token format returns error", func(t *testing.T) {
		t.Parallel()

		svc := testService(&mockOAuthRepo{})

		_, err := svc.ValidateAccessToken(context.Background(), "bad-format-no-pipe")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid token format")
	})

	t.Run("token not found returns error", func(t *testing.T) {
		t.Parallel()

		tokenPlaintext, _ := makeTokenPlaintext(t, 300)

		repo := &mockOAuthRepo{
			FindAccessTokenByTokenFunc: func(_ context.Context, _ string) (*model.OAuthAccessToken, error) {
				return nil, sql.ErrNoRows
			},
		}
		svc := testService(repo)

		_, err := svc.ValidateAccessToken(context.Background(), tokenPlaintext)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid or expired token")
	})

	t.Run("ID mismatch returns error", func(t *testing.T) {
		t.Parallel()

		tokenPlaintext, tokenHash := makeTokenPlaintext(t, 300)

		repo := &mockOAuthRepo{
			FindAccessTokenByTokenFunc: func(_ context.Context, hash string) (*model.OAuthAccessToken, error) {
				if hash == tokenHash {
					return &model.OAuthAccessToken{ID: 999, Token: tokenHash}, nil // ID mismatch
				}
				return nil, sql.ErrNoRows
			},
		}
		svc := testService(repo)

		_, err := svc.ValidateAccessToken(context.Background(), tokenPlaintext)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid or expired token")
	})
}

// ---------------------------------------------------------------------------
// TestGenerateDeviceCode
// ---------------------------------------------------------------------------

func TestGenerateDeviceCode(t *testing.T) {
	t.Parallel()

	const (
		testClientID   = "550e8400-e29b-41d4-a716-446655440004"
		testClientDBID = int64(100)
	)

	t.Run("happy path returns device authorization result", func(t *testing.T) {
		t.Parallel()

		client := makeDeviceClient()

		var capturedDC *model.OAuthDeviceCode
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, cid string) (*model.OAuthClient, error) {
				if cid == testClientID {
					return client, nil
				}
				return nil, sql.ErrNoRows
			},
			CreateDeviceCodeFunc: func(_ context.Context, dc *model.OAuthDeviceCode) error {
				capturedDC = dc
				dc.ID = 600
				return nil
			},
		}
		svc := testService(repo)

		result, err := svc.GenerateDeviceCode(context.Background(), DeviceAuthorizationParams{
			ClientID: testClientID,
			Scope:    "mcp:access",
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotEmpty(t, result.DeviceCode)
		assert.NotEmpty(t, result.UserCode)
		assert.Equal(t, "https://app.example.com/oauth/device", result.VerificationURI)
		assert.Contains(t, result.VerificationURIComplete, result.UserCode)
		assert.Equal(t, int(testConfig().DeviceCodeLifetime.Seconds()), result.ExpiresIn)
		assert.GreaterOrEqual(t, result.Interval, 5)

		require.NotNil(t, capturedDC)
		assert.Equal(t, testClientDBID, capturedDC.ClientID)
		assert.Equal(t, "pending", capturedDC.Status)
		assert.True(t, capturedDC.Scope.Valid)
		assert.Equal(t, "mcp:access", capturedDC.Scope.String)
	})

	t.Run("client not found returns error", func(t *testing.T) {
		t.Parallel()

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return nil, sql.ErrNoRows
			},
		}
		svc := testService(repo)

		_, err := svc.GenerateDeviceCode(context.Background(), DeviceAuthorizationParams{
			ClientID: "unknown",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid or inactive client")
	})

	t.Run("client without device_code grant returns error", func(t *testing.T) {
		t.Parallel()

		client := makePublicClient(testClientDBID, testClientID) // only authorization_code

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
		}
		svc := testService(repo)

		_, err := svc.GenerateDeviceCode(context.Background(), DeviceAuthorizationParams{
			ClientID: testClientID,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "client does not support device_code grant type")
	})

	t.Run("invalid grant types JSON returns error", func(t *testing.T) {
		t.Parallel()

		client := makePublicClient(testClientDBID, testClientID)
		client.GrantTypes = "not-valid-json"

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
		}
		svc := testService(repo)

		_, err := svc.GenerateDeviceCode(context.Background(), DeviceAuthorizationParams{
			ClientID: testClientID,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "client does not support device_code grant type")
	})

	t.Run("database error on persist propagates", func(t *testing.T) {
		t.Parallel()

		client := makeDeviceClient()

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
			CreateDeviceCodeFunc: func(_ context.Context, _ *model.OAuthDeviceCode) error {
				return errors.New("unique violation")
			},
		}
		svc := testService(repo)

		_, err := svc.GenerateDeviceCode(context.Background(), DeviceAuthorizationParams{
			ClientID: testClientID,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "persisting device code")
	})

	t.Run("empty scope sets null on model", func(t *testing.T) {
		t.Parallel()

		client := makeDeviceClient()

		var capturedDC *model.OAuthDeviceCode
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
			CreateDeviceCodeFunc: func(_ context.Context, dc *model.OAuthDeviceCode) error {
				capturedDC = dc
				dc.ID = 601
				return nil
			},
		}
		svc := testService(repo)

		_, err := svc.GenerateDeviceCode(context.Background(), DeviceAuthorizationParams{
			ClientID: testClientID,
			Scope:    "",
		})

		require.NoError(t, err)
		require.NotNil(t, capturedDC)
		assert.False(t, capturedDC.Scope.Valid)
	})

	t.Run("invalid scope returns error", func(t *testing.T) {
		t.Parallel()

		client := makeDeviceClient()

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
		}
		svc := testService(repo)

		_, err := svc.GenerateDeviceCode(context.Background(), DeviceAuthorizationParams{
			ClientID: testClientID,
			Scope:    "mcp:access bogus:scope",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid scopes")
		assert.Contains(t, err.Error(), "bogus:scope")
	})

	t.Run("scope exceeding client scope returns error", func(t *testing.T) {
		t.Parallel()

		client := makeDeviceClient()
		client.Scope = sql.NullString{String: "mcp:access documents:read", Valid: true}

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
		}
		svc := testService(repo)

		_, err := svc.GenerateDeviceCode(context.Background(), DeviceAuthorizationParams{
			ClientID: testClientID,
			Scope:    "mcp:access documents:read documents:write",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "requested scope exceeds client's allowed scope")
	})

	t.Run("valid scope subset of client scope succeeds", func(t *testing.T) {
		t.Parallel()

		client := makeDeviceClient()
		client.Scope = sql.NullString{String: "mcp:access documents:read documents:write", Valid: true}

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
			CreateDeviceCodeFunc: func(_ context.Context, dc *model.OAuthDeviceCode) error {
				dc.ID = 602
				return nil
			},
		}
		svc := testService(repo)

		result, err := svc.GenerateDeviceCode(context.Background(), DeviceAuthorizationParams{
			ClientID: testClientID,
			Scope:    "mcp:access documents:read",
		})

		require.NoError(t, err)
		require.NotNil(t, result)
	})
}

// ---------------------------------------------------------------------------
// TestAuthorizeDeviceCode
// ---------------------------------------------------------------------------

func TestAuthorizeDeviceCode(t *testing.T) {
	t.Parallel()

	t.Run("approve sets authorized status", func(t *testing.T) {
		t.Parallel()

		var capturedStatus string
		var capturedUserID *int64
		repo := &mockOAuthRepo{
			FindDeviceCodeByUserCodeFunc: func(_ context.Context, _ string) (*model.OAuthDeviceCode, error) {
				return &model.OAuthDeviceCode{
					ID:        600,
					Status:    "pending",
					ExpiresAt: time.Now().Add(5 * time.Minute),
				}, nil
			},
			UpdateDeviceCodeStatusFunc: func(_ context.Context, id int64, status string, userID *int64) error {
				assert.Equal(t, int64(600), id)
				capturedStatus = status
				capturedUserID = userID
				return nil
			},
		}
		svc := testService(repo)

		err := svc.AuthorizeDeviceCode(context.Background(), "BCDF-GHJK", 42, true)

		require.NoError(t, err)
		assert.Equal(t, "authorized", capturedStatus)
		require.NotNil(t, capturedUserID)
		assert.Equal(t, int64(42), *capturedUserID)
	})

	t.Run("deny sets denied status", func(t *testing.T) {
		t.Parallel()

		var capturedStatus string
		repo := &mockOAuthRepo{
			FindDeviceCodeByUserCodeFunc: func(_ context.Context, _ string) (*model.OAuthDeviceCode, error) {
				return &model.OAuthDeviceCode{
					ID:        601,
					Status:    "pending",
					ExpiresAt: time.Now().Add(5 * time.Minute),
				}, nil
			},
			UpdateDeviceCodeStatusFunc: func(_ context.Context, _ int64, status string, _ *int64) error {
				capturedStatus = status
				return nil
			},
		}
		svc := testService(repo)

		err := svc.AuthorizeDeviceCode(context.Background(), "BCDF-GHJK", 42, false)

		require.NoError(t, err)
		assert.Equal(t, "denied", capturedStatus)
	})

	t.Run("expired device code returns error", func(t *testing.T) {
		t.Parallel()

		repo := &mockOAuthRepo{
			FindDeviceCodeByUserCodeFunc: func(_ context.Context, _ string) (*model.OAuthDeviceCode, error) {
				return &model.OAuthDeviceCode{
					ID:        602,
					Status:    "pending",
					ExpiresAt: time.Now().Add(-1 * time.Minute),
				}, nil
			},
		}
		svc := testService(repo)

		err := svc.AuthorizeDeviceCode(context.Background(), "BCDF-GHJK", 42, true)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "device code has expired")
	})

	t.Run("already used device code returns error", func(t *testing.T) {
		t.Parallel()

		repo := &mockOAuthRepo{
			FindDeviceCodeByUserCodeFunc: func(_ context.Context, _ string) (*model.OAuthDeviceCode, error) {
				return &model.OAuthDeviceCode{
					ID:        603,
					Status:    "authorized",
					ExpiresAt: time.Now().Add(5 * time.Minute),
				}, nil
			},
		}
		svc := testService(repo)

		err := svc.AuthorizeDeviceCode(context.Background(), "BCDF-GHJK", 42, true)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not in valid state")
	})

	t.Run("user code not found returns error", func(t *testing.T) {
		t.Parallel()

		repo := &mockOAuthRepo{
			FindDeviceCodeByUserCodeFunc: func(_ context.Context, _ string) (*model.OAuthDeviceCode, error) {
				return nil, sql.ErrNoRows
			},
		}
		svc := testService(repo)

		err := svc.AuthorizeDeviceCode(context.Background(), "XXXX-XXXX", 42, true)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid or expired user code")
	})
}

// ---------------------------------------------------------------------------
// TestExchangeDeviceCode
// ---------------------------------------------------------------------------

func TestExchangeDeviceCode(t *testing.T) {
	t.Parallel()

	const (
		testClientID   = "550e8400-e29b-41d4-a716-446655440005"
		testClientDBID = int64(100)
	)

	setupDeviceExchange := func(t *testing.T, status string) (deviceCodePlaintext string, deviceCodeHash string, client *model.OAuthClient, dc *model.OAuthDeviceCode) {
		t.Helper()
		deviceCodePlaintext, deviceCodeHash = makeTokenPlaintext(t, 600)
		client = makePublicClient(testClientDBID, testClientID)
		dc = &model.OAuthDeviceCode{
			ID:         600,
			DeviceCode: deviceCodeHash,
			ClientID:   testClientDBID,
			UserID:     sql.NullInt64{Int64: 42, Valid: true},
			Scope:      sql.NullString{String: "mcp:access", Valid: true},
			Status:     status,
			Interval:   5,
			ExpiresAt:  time.Now().Add(5 * time.Minute),
		}
		return
	}

	t.Run("authorized status issues tokens and marks exchanged", func(t *testing.T) {
		t.Parallel()

		dcPlaintext, dcHash, client, dc := setupDeviceExchange(t, "authorized")

		var statusUpdated string
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
			FindDeviceCodeByDeviceCodeFunc: func(_ context.Context, hash string) (*model.OAuthDeviceCode, error) {
				if hash == dcHash {
					return dc, nil
				}
				return nil, sql.ErrNoRows
			},
			UpdateDeviceCodeLastPolledFunc: func(_ context.Context, _ int64, _ int) error { return nil },
			UpdateDeviceCodeStatusFunc: func(_ context.Context, _ int64, status string, _ *int64) error {
				statusUpdated = status
				return nil
			},
			CreateAccessTokenFunc:  func(_ context.Context, t *model.OAuthAccessToken) error { t.ID = 301; return nil },
			CreateRefreshTokenFunc: func(_ context.Context, t *model.OAuthRefreshToken) error { t.ID = 501; return nil },
		}
		svc := testService(repo)

		result, err := svc.ExchangeDeviceCode(context.Background(), ExchangeDeviceCodeParams{
			DeviceCode: dcPlaintext,
			ClientID:   testClientID,
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotEmpty(t, result.AccessToken)
		assert.NotEmpty(t, result.RefreshToken)
		assert.Equal(t, "Bearer", result.TokenType)
		assert.Equal(t, "mcp:access", result.Scope)
		assert.Equal(t, "exchanged", statusUpdated)
	})

	t.Run("pending status returns authorization_pending", func(t *testing.T) {
		t.Parallel()

		dcPlaintext, dcHash, client, dc := setupDeviceExchange(t, "pending")

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
			FindDeviceCodeByDeviceCodeFunc: func(_ context.Context, hash string) (*model.OAuthDeviceCode, error) {
				if hash == dcHash {
					return dc, nil
				}
				return nil, sql.ErrNoRows
			},
			UpdateDeviceCodeLastPolledFunc: func(_ context.Context, _ int64, _ int) error { return nil },
		}
		svc := testService(repo)

		_, err := svc.ExchangeDeviceCode(context.Background(), ExchangeDeviceCodeParams{
			DeviceCode: dcPlaintext,
			ClientID:   testClientID,
		})

		require.Error(t, err)
		var dcErr *DeviceCodeError
		require.ErrorAs(t, err, &dcErr)
		assert.Equal(t, "authorization_pending", dcErr.Code)
	})

	t.Run("denied status returns access_denied", func(t *testing.T) {
		t.Parallel()

		dcPlaintext, dcHash, client, dc := setupDeviceExchange(t, "denied")

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
			FindDeviceCodeByDeviceCodeFunc: func(_ context.Context, hash string) (*model.OAuthDeviceCode, error) {
				if hash == dcHash {
					return dc, nil
				}
				return nil, sql.ErrNoRows
			},
			UpdateDeviceCodeLastPolledFunc: func(_ context.Context, _ int64, _ int) error { return nil },
		}
		svc := testService(repo)

		_, err := svc.ExchangeDeviceCode(context.Background(), ExchangeDeviceCodeParams{
			DeviceCode: dcPlaintext,
			ClientID:   testClientID,
		})

		require.Error(t, err)
		var dcErr *DeviceCodeError
		require.ErrorAs(t, err, &dcErr)
		assert.Equal(t, "access_denied", dcErr.Code)
	})

	t.Run("exchanged status returns invalid_grant", func(t *testing.T) {
		t.Parallel()

		dcPlaintext, dcHash, client, dc := setupDeviceExchange(t, "exchanged")

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
			FindDeviceCodeByDeviceCodeFunc: func(_ context.Context, hash string) (*model.OAuthDeviceCode, error) {
				if hash == dcHash {
					return dc, nil
				}
				return nil, sql.ErrNoRows
			},
			UpdateDeviceCodeLastPolledFunc: func(_ context.Context, _ int64, _ int) error { return nil },
		}
		svc := testService(repo)

		_, err := svc.ExchangeDeviceCode(context.Background(), ExchangeDeviceCodeParams{
			DeviceCode: dcPlaintext,
			ClientID:   testClientID,
		})

		require.Error(t, err)
		var dcErr *DeviceCodeError
		require.ErrorAs(t, err, &dcErr)
		assert.Equal(t, "invalid_grant", dcErr.Code)
		assert.Contains(t, dcErr.Description, "already been used")
	})

	t.Run("expired device code returns expired_token", func(t *testing.T) {
		t.Parallel()

		dcPlaintext, dcHash, client, dc := setupDeviceExchange(t, "pending")
		dc.ExpiresAt = time.Now().Add(-1 * time.Minute)

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
			FindDeviceCodeByDeviceCodeFunc: func(_ context.Context, hash string) (*model.OAuthDeviceCode, error) {
				if hash == dcHash {
					return dc, nil
				}
				return nil, sql.ErrNoRows
			},
		}
		svc := testService(repo)

		_, err := svc.ExchangeDeviceCode(context.Background(), ExchangeDeviceCodeParams{
			DeviceCode: dcPlaintext,
			ClientID:   testClientID,
		})

		require.Error(t, err)
		var dcErr *DeviceCodeError
		require.ErrorAs(t, err, &dcErr)
		assert.Equal(t, "expired_token", dcErr.Code)
	})

	t.Run("polling too fast returns slow_down and increases interval", func(t *testing.T) {
		t.Parallel()

		dcPlaintext, dcHash, client, dc := setupDeviceExchange(t, "pending")
		dc.LastPolledAt = sql.NullTime{Time: time.Now(), Valid: true} // just polled
		dc.Interval = 5

		var updatedInterval int
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
			FindDeviceCodeByDeviceCodeFunc: func(_ context.Context, hash string) (*model.OAuthDeviceCode, error) {
				if hash == dcHash {
					return dc, nil
				}
				return nil, sql.ErrNoRows
			},
			UpdateDeviceCodeLastPolledFunc: func(_ context.Context, _ int64, interval int) error {
				updatedInterval = interval
				return nil
			},
		}
		svc := testService(repo)

		_, err := svc.ExchangeDeviceCode(context.Background(), ExchangeDeviceCodeParams{
			DeviceCode: dcPlaintext,
			ClientID:   testClientID,
		})

		require.Error(t, err)
		var dcErr *DeviceCodeError
		require.ErrorAs(t, err, &dcErr)
		assert.Equal(t, "slow_down", dcErr.Code)
		assert.Equal(t, 10, updatedInterval, "interval should increase by 5")
	})

	t.Run("invalid client returns invalid_client error", func(t *testing.T) {
		t.Parallel()

		dcPlaintext, _, _, _ := setupDeviceExchange(t, "pending")

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return nil, sql.ErrNoRows
			},
		}
		svc := testService(repo)

		_, err := svc.ExchangeDeviceCode(context.Background(), ExchangeDeviceCodeParams{
			DeviceCode: dcPlaintext,
			ClientID:   "unknown",
		})

		require.Error(t, err)
		var dcErr *DeviceCodeError
		require.ErrorAs(t, err, &dcErr)
		assert.Equal(t, "invalid_client", dcErr.Code)
	})

	t.Run("invalid device code format returns invalid_grant", func(t *testing.T) {
		t.Parallel()

		client := makePublicClient(testClientDBID, testClientID)

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
		}
		svc := testService(repo)

		_, err := svc.ExchangeDeviceCode(context.Background(), ExchangeDeviceCodeParams{
			DeviceCode: "bad-format",
			ClientID:   testClientID,
		})

		require.Error(t, err)
		var dcErr *DeviceCodeError
		require.ErrorAs(t, err, &dcErr)
		assert.Equal(t, "invalid_grant", dcErr.Code)
	})

	t.Run("device code belonging to different client returns invalid_grant", func(t *testing.T) {
		t.Parallel()

		dcPlaintext, dcHash, client, dc := setupDeviceExchange(t, "authorized")
		dc.ClientID = 999 // different client

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
			FindDeviceCodeByDeviceCodeFunc: func(_ context.Context, hash string) (*model.OAuthDeviceCode, error) {
				if hash == dcHash {
					return dc, nil
				}
				return nil, sql.ErrNoRows
			},
		}
		svc := testService(repo)

		_, err := svc.ExchangeDeviceCode(context.Background(), ExchangeDeviceCodeParams{
			DeviceCode: dcPlaintext,
			ClientID:   testClientID,
		})

		require.Error(t, err)
		var dcErr *DeviceCodeError
		require.ErrorAs(t, err, &dcErr)
		assert.Equal(t, "invalid_grant", dcErr.Code)
	})

	t.Run("unknown status returns invalid_grant", func(t *testing.T) {
		t.Parallel()

		dcPlaintext, dcHash, client, dc := setupDeviceExchange(t, "unknown_state")

		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, _ string) (*model.OAuthClient, error) {
				return client, nil
			},
			FindDeviceCodeByDeviceCodeFunc: func(_ context.Context, hash string) (*model.OAuthDeviceCode, error) {
				if hash == dcHash {
					return dc, nil
				}
				return nil, sql.ErrNoRows
			},
			UpdateDeviceCodeLastPolledFunc: func(_ context.Context, _ int64, _ int) error { return nil },
		}
		svc := testService(repo)

		_, err := svc.ExchangeDeviceCode(context.Background(), ExchangeDeviceCodeParams{
			DeviceCode: dcPlaintext,
			ClientID:   testClientID,
		})

		require.Error(t, err)
		var dcErr *DeviceCodeError
		require.ErrorAs(t, err, &dcErr)
		assert.Equal(t, "invalid_grant", dcErr.Code)
	})
}

// ---------------------------------------------------------------------------
// TestIsScopeSubset
// ---------------------------------------------------------------------------

func TestIsScopeSubset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		requested string
		original  string
		want      bool
	}{
		{
			name:      "exact match is subset",
			requested: "read write",
			original:  "read write",
			want:      true,
		},
		{
			name:      "single scope subset of multiple",
			requested: "read",
			original:  "read write admin",
			want:      true,
		},
		{
			name:      "superset is not subset",
			requested: "read write admin",
			original:  "read write",
			want:      false,
		},
		{
			name:      "empty requested is subset of anything",
			requested: "",
			original:  "read write",
			want:      true,
		},
		{
			name:      "empty original empty requested",
			requested: "",
			original:  "",
			want:      true,
		},
		{
			name:      "non-empty requested exceeds empty original",
			requested: "read",
			original:  "",
			want:      false,
		},
		{
			name:      "disjoint scopes are not subset",
			requested: "delete",
			original:  "read write",
			want:      false,
		},
		{
			name:      "partial overlap is not subset",
			requested: "read delete",
			original:  "read write",
			want:      false,
		},
		{
			name:      "duplicate scopes in requested",
			requested: "read read",
			original:  "read",
			want:      true,
		},
		{
			name:      "different order is still subset",
			requested: "write read",
			original:  "read write",
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := authscope.IsSubset(tt.requested, tt.original)
			assert.Equal(t, tt.want, got, "authscope.IsSubset(%q, %q)", tt.requested, tt.original)
		})
	}
}

// ---------------------------------------------------------------------------
// TestVerifyClientAuth
// ---------------------------------------------------------------------------

func TestVerifyClientAuth(t *testing.T) {
	t.Parallel()

	t.Run("public client always passes", func(t *testing.T) {
		t.Parallel()

		svc := testService(&mockOAuthRepo{})
		client := makePublicClient(1, "test-client")

		err := svc.verifyClientAuth(client, "")
		require.NoError(t, err)

		err = svc.verifyClientAuth(client, "any-secret")
		assert.NoError(t, err)
	})

	t.Run("confidential client with correct secret passes", func(t *testing.T) {
		t.Parallel()

		svc := testService(&mockOAuthRepo{})
		client := makeConfidentialClient(t, 1, "test-client", "my-secret")

		err := svc.verifyClientAuth(client, "my-secret")
		assert.NoError(t, err)
	})

	t.Run("confidential client with wrong secret fails", func(t *testing.T) {
		t.Parallel()

		svc := testService(&mockOAuthRepo{})
		client := makeConfidentialClient(t, 1, "test-client", "my-secret")

		err := svc.verifyClientAuth(client, "wrong-secret")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid client credentials")
	})

	t.Run("confidential client with missing secret hash fails", func(t *testing.T) {
		t.Parallel()

		svc := testService(&mockOAuthRepo{})
		client := &model.OAuthClient{
			ID:                      1,
			ClientID:                "test-client",
			TokenEndpointAuthMethod: "client_secret_post",
			ClientSecret:            sql.NullString{Valid: false},
		}

		err := svc.verifyClientAuth(client, "some-secret")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid client credentials")
	})

	t.Run("confidential client with empty secret hash fails", func(t *testing.T) {
		t.Parallel()

		svc := testService(&mockOAuthRepo{})
		client := &model.OAuthClient{
			ID:                      1,
			ClientID:                "test-client",
			TokenEndpointAuthMethod: "client_secret_post",
			ClientSecret:            sql.NullString{String: "", Valid: true},
		}

		err := svc.verifyClientAuth(client, "some-secret")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid client credentials")
	})
}

// ---------------------------------------------------------------------------
// TestValidateState
// ---------------------------------------------------------------------------

func TestValidateState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		state string
		want  bool
	}{
		{name: "alphanumeric", state: "abc123XYZ", want: true},
		{name: "with dots and tildes", state: "state.value~1", want: true},
		{name: "with underscores", state: "my_state_value", want: true},
		{name: "with parens and hyphens", state: "state(value)-1", want: true},
		{name: "with single quote", state: "state'value", want: true},
		{name: "empty string fails", state: "", want: false},
		{name: "contains space fails", state: "has space", want: false},
		{name: "contains hash fails", state: "has#hash", want: false},
		{name: "contains angle bracket fails", state: "has<bracket>", want: false},
		{name: "too long fails", state: strings.Repeat("a", 501), want: false},
		{name: "exactly 500 chars passes", state: strings.Repeat("a", 500), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ValidateState(tt.state)
			assert.Equal(t, tt.want, got, "ValidateState(%q)", tt.state)
		})
	}
}

// ---------------------------------------------------------------------------
// TestFindClient (simple delegate)
// ---------------------------------------------------------------------------

func TestFindClient(t *testing.T) {
	t.Parallel()

	t.Run("delegates to repo", func(t *testing.T) {
		t.Parallel()

		expected := makePublicClient(1, "test-client-id")
		repo := &mockOAuthRepo{
			FindClientByClientIDFunc: func(_ context.Context, cid string) (*model.OAuthClient, error) {
				if cid == "test-client-id" {
					return expected, nil
				}
				return nil, sql.ErrNoRows
			},
		}
		svc := testService(repo)

		result, err := svc.FindClient(context.Background(), "test-client-id")
		require.NoError(t, err)
		assert.Equal(t, expected, result)
	})
}

// ---------------------------------------------------------------------------
// TestDeviceCodeError
// ---------------------------------------------------------------------------

func TestDeviceCodeError(t *testing.T) {
	t.Parallel()

	t.Run("Error returns description", func(t *testing.T) {
		t.Parallel()

		err := &DeviceCodeError{
			Code:        "authorization_pending",
			Description: "The authorization request is still pending",
		}

		assert.Equal(t, "The authorization request is still pending", err.Error())
	})

	t.Run("implements error interface", func(t *testing.T) {
		t.Parallel()

		var err error = &DeviceCodeError{Code: "test", Description: "test desc"}
		require.Error(t, err)

		var dcErr *DeviceCodeError
		require.ErrorAs(t, err, &dcErr)
		assert.Equal(t, "test", dcErr.Code)
	})
}

// ---------------------------------------------------------------------------
// TestIssueTokenPair
// ---------------------------------------------------------------------------

func TestIssueTokenPair(t *testing.T) {
	t.Parallel()

	t.Run("creates both access and refresh tokens", func(t *testing.T) {
		t.Parallel()

		var accessCreated, refreshCreated bool
		repo := &mockOAuthRepo{
			CreateAccessTokenFunc: func(_ context.Context, tok *model.OAuthAccessToken) error {
				accessCreated = true
				tok.ID = 300
				assert.Equal(t, int64(100), tok.ClientID)
				assert.Equal(t, int64(42), tok.UserID.Int64)
				assert.True(t, tok.UserID.Valid)
				assert.Equal(t, "mcp:access", tok.Scope.String)
				assert.False(t, tok.Revoked)
				return nil
			},
			CreateRefreshTokenFunc: func(_ context.Context, tok *model.OAuthRefreshToken) error {
				refreshCreated = true
				tok.ID = 400
				assert.Equal(t, int64(300), tok.AccessTokenID)
				assert.False(t, tok.Revoked)
				return nil
			},
		}
		svc := testService(repo)

		result, err := svc.issueTokenPair(
			context.Background(),
			100,
			sql.NullInt64{Int64: 42, Valid: true},
			"mcp:access",
		)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotEmpty(t, result.AccessToken)
		assert.NotEmpty(t, result.RefreshToken)
		assert.Equal(t, "Bearer", result.TokenType)
		assert.Equal(t, int(testConfig().AccessTokenLifetime.Seconds()), result.ExpiresIn)
		assert.Equal(t, "mcp:access", result.Scope)
		assert.True(t, accessCreated)
		assert.True(t, refreshCreated)

		// Both tokens should be parseable
		_, _, err = ParseToken(result.AccessToken)
		require.NoError(t, err)
		_, _, err = ParseToken(result.RefreshToken)
		require.NoError(t, err)
	})

	t.Run("empty scope sets null on token model", func(t *testing.T) {
		t.Parallel()

		repo := &mockOAuthRepo{
			CreateAccessTokenFunc: func(_ context.Context, tok *model.OAuthAccessToken) error {
				tok.ID = 301
				assert.False(t, tok.Scope.Valid, "empty scope should produce null")
				return nil
			},
			CreateRefreshTokenFunc: func(_ context.Context, tok *model.OAuthRefreshToken) error {
				tok.ID = 401
				return nil
			},
		}
		svc := testService(repo)

		result, err := svc.issueTokenPair(
			context.Background(),
			100,
			sql.NullInt64{Int64: 42, Valid: true},
			"",
		)

		require.NoError(t, err)
		assert.Empty(t, result.Scope)
	})

	t.Run("access token creation failure propagates", func(t *testing.T) {
		t.Parallel()

		repo := &mockOAuthRepo{
			CreateAccessTokenFunc: func(_ context.Context, _ *model.OAuthAccessToken) error {
				return errors.New("db error")
			},
		}
		svc := testService(repo)

		_, err := svc.issueTokenPair(
			context.Background(),
			100,
			sql.NullInt64{Int64: 42, Valid: true},
			"mcp:access",
		)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "creating access token")
	})

	t.Run("refresh token creation failure propagates", func(t *testing.T) {
		t.Parallel()

		repo := &mockOAuthRepo{
			CreateAccessTokenFunc: func(_ context.Context, tok *model.OAuthAccessToken) error {
				tok.ID = 302
				return nil
			},
			CreateRefreshTokenFunc: func(_ context.Context, _ *model.OAuthRefreshToken) error {
				return errors.New("db error")
			},
		}
		svc := testService(repo)

		_, err := svc.issueTokenPair(
			context.Background(),
			100,
			sql.NullInt64{Int64: 42, Valid: true},
			"mcp:access",
		)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "creating refresh token")
	})

	t.Run("invalid scope returns error", func(t *testing.T) {
		t.Parallel()

		repo := &mockOAuthRepo{}
		svc := testService(repo)

		_, err := svc.issueTokenPair(
			context.Background(),
			100,
			sql.NullInt64{Int64: 42, Valid: true},
			"mcp:access bogus:scope",
		)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid scopes in token")
		assert.Contains(t, err.Error(), "bogus:scope")
	})
}
