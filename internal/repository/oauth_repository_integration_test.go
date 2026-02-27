//go:build integration

package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.999.haus/chris/DocuMCP-go/internal/model"
	"git.999.haus/chris/DocuMCP-go/internal/testutil"
)

// ---------------------------------------------------------------------------
// Users
// ---------------------------------------------------------------------------

func TestOAuthRepository_Users(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewOAuthRepository(testDB, discardLogger())

	// Create a user.
	user := testutil.NewUser(
		testutil.WithUserID(0),
		testutil.WithUserName("Alice"),
		testutil.WithUserEmail("alice@example.com"),
		testutil.WithUserOIDCSub("oidc-sub-alice"),
		testutil.WithUserOIDCProvider("test-provider"),
	)
	require.NoError(t, repo.CreateUser(ctx, user))
	require.NotZero(t, user.ID)

	t.Run("FindUserByID", func(t *testing.T) {
		found, err := repo.FindUserByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, user.ID, found.ID)
		assert.Equal(t, "Alice", found.Name)
		assert.Equal(t, "alice@example.com", found.Email)
	})

	t.Run("FindUserByEmail", func(t *testing.T) {
		found, err := repo.FindUserByEmail(ctx, "alice@example.com")
		require.NoError(t, err)
		assert.Equal(t, user.ID, found.ID)
	})

	t.Run("FindUserByOIDCSub", func(t *testing.T) {
		found, err := repo.FindUserByOIDCSub(ctx, "oidc-sub-alice")
		require.NoError(t, err)
		assert.Equal(t, user.ID, found.ID)
	})

	t.Run("UpdateUser", func(t *testing.T) {
		user.Name = "Alice Updated"
		user.Email = "alice-updated@example.com"
		require.NoError(t, repo.UpdateUser(ctx, user))

		found, err := repo.FindUserByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, "Alice Updated", found.Name)
		assert.Equal(t, "alice-updated@example.com", found.Email)
	})

	t.Run("ToggleAdmin", func(t *testing.T) {
		// Initially not admin.
		found, err := repo.FindUserByID(ctx, user.ID)
		require.NoError(t, err)
		assert.False(t, found.IsAdmin)

		// Toggle on.
		require.NoError(t, repo.ToggleAdmin(ctx, user.ID))
		found, err = repo.FindUserByID(ctx, user.ID)
		require.NoError(t, err)
		assert.True(t, found.IsAdmin)

		// Toggle off.
		require.NoError(t, repo.ToggleAdmin(ctx, user.ID))
		found, err = repo.FindUserByID(ctx, user.ID)
		require.NoError(t, err)
		assert.False(t, found.IsAdmin)
	})

	t.Run("ListUsers_NoQuery", func(t *testing.T) {
		// Create a second user for list tests.
		u2 := testutil.NewUser(
			testutil.WithUserID(0),
			testutil.WithUserName("Bob"),
			testutil.WithUserEmail("bob@example.com"),
		)
		require.NoError(t, repo.CreateUser(ctx, u2))

		users, total, err := repo.ListUsers(ctx, "", 20, 0)
		require.NoError(t, err)
		assert.Equal(t, 2, total)
		assert.Len(t, users, 2)
	})

	t.Run("ListUsers_WithQuery", func(t *testing.T) {
		users, total, err := repo.ListUsers(ctx, "Bob", 20, 0)
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		require.Len(t, users, 1)
		assert.Equal(t, "Bob", users[0].Name)
	})

	t.Run("CountUsers", func(t *testing.T) {
		count, err := repo.CountUsers(ctx)
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})
}

// ---------------------------------------------------------------------------
// Clients
// ---------------------------------------------------------------------------

func TestOAuthRepository_Clients(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewOAuthRepository(testDB, discardLogger())

	client := testutil.NewOAuthClient(
		testutil.WithOAuthClientID(0),
		testutil.WithOAuthClientClientID("client-abc"),
		testutil.WithOAuthClientName("My App"),
		testutil.WithOAuthClientSecret("s3cret"),
		testutil.WithOAuthClientScope("read write"),
	)
	require.NoError(t, repo.CreateClient(ctx, client))
	require.NotZero(t, client.ID)

	t.Run("FindClientByClientID", func(t *testing.T) {
		found, err := repo.FindClientByClientID(ctx, "client-abc")
		require.NoError(t, err)
		assert.Equal(t, client.ID, found.ID)
		assert.Equal(t, "My App", found.ClientName)
	})

	t.Run("FindClientByID", func(t *testing.T) {
		found, err := repo.FindClientByID(ctx, client.ID)
		require.NoError(t, err)
		assert.Equal(t, "client-abc", found.ClientID)
	})

	t.Run("DeactivateClient", func(t *testing.T) {
		require.NoError(t, repo.DeactivateClient(ctx, client.ID))

		// FindClientByClientID only returns active clients.
		_, err := repo.FindClientByClientID(ctx, "client-abc")
		require.Error(t, err)

		// FindClientByID still returns the client (it does not filter by is_active).
		found, err := repo.FindClientByID(ctx, client.ID)
		require.NoError(t, err)
		assert.False(t, found.IsActive)
	})

	t.Run("ListClients_WithQuery", func(t *testing.T) {
		// Create a second active client for listing.
		c2 := testutil.NewOAuthClient(
			testutil.WithOAuthClientID(0),
			testutil.WithOAuthClientClientID("client-xyz"),
			testutil.WithOAuthClientName("Another App"),
		)
		require.NoError(t, repo.CreateClient(ctx, c2))

		clients, total, err := repo.ListClients(ctx, "Another", 20, 0)
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		require.Len(t, clients, 1)
		assert.Equal(t, "Another App", clients[0].ClientName)
	})

	t.Run("CountClients", func(t *testing.T) {
		count, err := repo.CountClients(ctx)
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})
}

// ---------------------------------------------------------------------------
// Authorization Codes
// ---------------------------------------------------------------------------

func TestOAuthRepository_AuthorizationCodes(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewOAuthRepository(testDB, discardLogger())

	// FK dependencies: user and client.
	user := testutil.NewUser(
		testutil.WithUserID(0),
		testutil.WithUserEmail("authcode-user@example.com"),
	)
	require.NoError(t, repo.CreateUser(ctx, user))

	client := testutil.NewOAuthClient(
		testutil.WithOAuthClientID(0),
		testutil.WithOAuthClientClientID("authcode-client"),
	)
	require.NoError(t, repo.CreateClient(ctx, client))

	code := &model.OAuthAuthorizationCode{
		Code:                "hashed-code-abc123",
		ClientID:            client.ID,
		UserID:              sql.NullInt64{Int64: user.ID, Valid: true},
		RedirectURI:         "http://localhost:8080/callback",
		Scope:               sql.NullString{String: "read", Valid: true},
		CodeChallenge:       sql.NullString{String: "challenge-value", Valid: true},
		CodeChallengeMethod: sql.NullString{String: "S256", Valid: true},
		ExpiresAt:           time.Now().Add(10 * time.Minute),
		Revoked:             false,
	}
	require.NoError(t, repo.CreateAuthorizationCode(ctx, code))
	require.NotZero(t, code.ID)

	t.Run("FindAuthorizationCodeByCode", func(t *testing.T) {
		found, err := repo.FindAuthorizationCodeByCode(ctx, "hashed-code-abc123")
		require.NoError(t, err)
		assert.Equal(t, code.ID, found.ID)
		assert.Equal(t, client.ID, found.ClientID)
		assert.Equal(t, "http://localhost:8080/callback", found.RedirectURI)
	})

	t.Run("RevokeAuthorizationCode", func(t *testing.T) {
		require.NoError(t, repo.RevokeAuthorizationCode(ctx, code.ID))

		// FindAuthorizationCodeByCode only returns non-revoked codes.
		_, err := repo.FindAuthorizationCodeByCode(ctx, "hashed-code-abc123")
		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// Access Tokens
// ---------------------------------------------------------------------------

func TestOAuthRepository_AccessTokens(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewOAuthRepository(testDB, discardLogger())

	// FK dependencies.
	user := testutil.NewUser(
		testutil.WithUserID(0),
		testutil.WithUserEmail("token-user@example.com"),
	)
	require.NoError(t, repo.CreateUser(ctx, user))

	client := testutil.NewOAuthClient(
		testutil.WithOAuthClientID(0),
		testutil.WithOAuthClientClientID("token-client"),
	)
	require.NoError(t, repo.CreateClient(ctx, client))

	token := &model.OAuthAccessToken{
		Token:     "hashed-access-token-xyz",
		ClientID:  client.ID,
		UserID:    sql.NullInt64{Int64: user.ID, Valid: true},
		Scope:     sql.NullString{String: "read write", Valid: true},
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Revoked:   false,
	}
	require.NoError(t, repo.CreateAccessToken(ctx, token))
	require.NotZero(t, token.ID)

	t.Run("FindAccessTokenByToken", func(t *testing.T) {
		found, err := repo.FindAccessTokenByToken(ctx, "hashed-access-token-xyz")
		require.NoError(t, err)
		assert.Equal(t, token.ID, found.ID)
		assert.Equal(t, client.ID, found.ClientID)
	})

	t.Run("FindAccessTokenByID", func(t *testing.T) {
		found, err := repo.FindAccessTokenByID(ctx, token.ID)
		require.NoError(t, err)
		assert.Equal(t, "hashed-access-token-xyz", found.Token)
	})

	t.Run("RevokeAccessToken", func(t *testing.T) {
		require.NoError(t, repo.RevokeAccessToken(ctx, token.ID))

		// FindAccessTokenByToken only returns non-revoked, non-expired tokens.
		_, err := repo.FindAccessTokenByToken(ctx, "hashed-access-token-xyz")
		require.Error(t, err)

		// FindAccessTokenByID still returns the token regardless of revoked status.
		found, err := repo.FindAccessTokenByID(ctx, token.ID)
		require.NoError(t, err)
		assert.True(t, found.Revoked)
	})
}

// ---------------------------------------------------------------------------
// Refresh Tokens
// ---------------------------------------------------------------------------

func TestOAuthRepository_RefreshTokens(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewOAuthRepository(testDB, discardLogger())

	// FK dependencies: user -> client -> access token.
	user := testutil.NewUser(
		testutil.WithUserID(0),
		testutil.WithUserEmail("refresh-user@example.com"),
	)
	require.NoError(t, repo.CreateUser(ctx, user))

	client := testutil.NewOAuthClient(
		testutil.WithOAuthClientID(0),
		testutil.WithOAuthClientClientID("refresh-client"),
	)
	require.NoError(t, repo.CreateClient(ctx, client))

	accessToken := &model.OAuthAccessToken{
		Token:     "hashed-at-for-refresh",
		ClientID:  client.ID,
		UserID:    sql.NullInt64{Int64: user.ID, Valid: true},
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Revoked:   false,
	}
	require.NoError(t, repo.CreateAccessToken(ctx, accessToken))

	refreshToken := &model.OAuthRefreshToken{
		Token:         "hashed-refresh-token-abc",
		AccessTokenID: accessToken.ID,
		ExpiresAt:     time.Now().Add(24 * time.Hour),
		Revoked:       false,
	}
	require.NoError(t, repo.CreateRefreshToken(ctx, refreshToken))
	require.NotZero(t, refreshToken.ID)

	t.Run("FindRefreshTokenByToken", func(t *testing.T) {
		found, err := repo.FindRefreshTokenByToken(ctx, "hashed-refresh-token-abc")
		require.NoError(t, err)
		assert.Equal(t, refreshToken.ID, found.ID)
		assert.Equal(t, accessToken.ID, found.AccessTokenID)
	})

	t.Run("RevokeRefreshTokenByAccessTokenID", func(t *testing.T) {
		require.NoError(t, repo.RevokeRefreshTokenByAccessTokenID(ctx, accessToken.ID))

		// FindRefreshTokenByToken only returns non-revoked, non-expired tokens.
		_, err := repo.FindRefreshTokenByToken(ctx, "hashed-refresh-token-abc")
		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// Device Codes
// ---------------------------------------------------------------------------

func TestOAuthRepository_DeviceCodes(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewOAuthRepository(testDB, discardLogger())

	// FK dependency: client.
	client := testutil.NewOAuthClient(
		testutil.WithOAuthClientID(0),
		testutil.WithOAuthClientClientID("device-client"),
	)
	require.NoError(t, repo.CreateClient(ctx, client))

	dc := &model.OAuthDeviceCode{
		DeviceCode:      "hashed-device-code-999",
		UserCode:        "ABCD-EFGH",
		ClientID:        client.ID,
		Scope:           sql.NullString{String: "read", Valid: true},
		VerificationURI: "https://example.com/device",
		VerificationURIComplete: sql.NullString{
			String: "https://example.com/device?code=ABCD-EFGH",
			Valid:  true,
		},
		Interval:  5,
		Status:    "pending",
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}
	require.NoError(t, repo.CreateDeviceCode(ctx, dc))
	require.NotZero(t, dc.ID)

	t.Run("FindDeviceCodeByDeviceCode", func(t *testing.T) {
		found, err := repo.FindDeviceCodeByDeviceCode(ctx, "hashed-device-code-999")
		require.NoError(t, err)
		assert.Equal(t, dc.ID, found.ID)
		assert.Equal(t, "ABCD-EFGH", found.UserCode)
		assert.Equal(t, "pending", found.Status)
	})

	t.Run("FindDeviceCodeByUserCode", func(t *testing.T) {
		found, err := repo.FindDeviceCodeByUserCode(ctx, "ABCD-EFGH")
		require.NoError(t, err)
		assert.Equal(t, dc.ID, found.ID)

		// Case-insensitive and dash-insensitive lookup.
		found2, err := repo.FindDeviceCodeByUserCode(ctx, "abcdefgh")
		require.NoError(t, err)
		assert.Equal(t, dc.ID, found2.ID)
	})

	t.Run("UpdateDeviceCodeStatus", func(t *testing.T) {
		// Create a user to approve the device code.
		user := testutil.NewUser(
			testutil.WithUserID(0),
			testutil.WithUserEmail("device-user@example.com"),
		)
		require.NoError(t, repo.CreateUser(ctx, user))

		userID := user.ID
		require.NoError(t, repo.UpdateDeviceCodeStatus(ctx, dc.ID, "approved", &userID))

		found, err := repo.FindDeviceCodeByDeviceCode(ctx, "hashed-device-code-999")
		require.NoError(t, err)
		assert.Equal(t, "approved", found.Status)
		assert.True(t, found.UserID.Valid)
		assert.Equal(t, user.ID, found.UserID.Int64)
	})

	t.Run("UpdateDeviceCodeLastPolled", func(t *testing.T) {
		require.NoError(t, repo.UpdateDeviceCodeLastPolled(ctx, dc.ID, 10))

		found, err := repo.FindDeviceCodeByDeviceCode(ctx, "hashed-device-code-999")
		require.NoError(t, err)
		assert.True(t, found.LastPolledAt.Valid)
		assert.Equal(t, 10, found.Interval)
	})
}
