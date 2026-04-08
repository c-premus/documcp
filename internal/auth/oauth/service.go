package oauth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	authscope "github.com/c-premus/documcp/internal/auth/scope"
	"github.com/c-premus/documcp/internal/config"
	"github.com/c-premus/documcp/internal/model"
)

// safeStateRegexp allows only safe characters in the state parameter.
var safeStateRegexp = regexp.MustCompile(`^[a-zA-Z0-9._~()'\-]+$`)

//nolint:revive // OAuthRepo is intentionally named to distinguish from other repository interfaces
type OAuthRepo interface {
	// Clients
	CreateClient(ctx context.Context, client *model.OAuthClient) error
	FindClientByClientID(ctx context.Context, clientID string) (*model.OAuthClient, error)
	FindClientByID(ctx context.Context, id int64) (*model.OAuthClient, error)
	TouchClientLastUsed(ctx context.Context, clientID int64) error
	UpdateClientScope(ctx context.Context, clientID int64, scope string) error
	// Authorization Codes
	CreateAuthorizationCode(ctx context.Context, code *model.OAuthAuthorizationCode) error
	FindAuthorizationCodeByCode(ctx context.Context, codeHash string) (*model.OAuthAuthorizationCode, error)
	RevokeAuthorizationCode(ctx context.Context, id int64) error
	// Access Tokens
	CreateAccessToken(ctx context.Context, token *model.OAuthAccessToken) error
	FindAccessTokenByID(ctx context.Context, id int64) (*model.OAuthAccessToken, error)
	FindAccessTokenByToken(ctx context.Context, tokenHash string) (*model.OAuthAccessToken, error)
	RevokeAccessToken(ctx context.Context, id int64) error
	RevokeTokenPair(ctx context.Context, accessTokenID, refreshTokenID int64) error
	// Refresh Tokens
	CreateRefreshToken(ctx context.Context, token *model.OAuthRefreshToken) error
	FindRefreshTokenByToken(ctx context.Context, tokenHash string) (*model.OAuthRefreshToken, error)
	RevokeRefreshToken(ctx context.Context, id int64) error
	RevokeRefreshTokenByAccessTokenID(ctx context.Context, accessTokenID int64) error
	// Device Codes
	CreateDeviceCode(ctx context.Context, dc *model.OAuthDeviceCode) error
	FindDeviceCodeByDeviceCode(ctx context.Context, deviceCodeHash string) (*model.OAuthDeviceCode, error)
	FindDeviceCodeByUserCode(ctx context.Context, userCode string) (*model.OAuthDeviceCode, error)
	UpdateDeviceCodeStatus(ctx context.Context, id int64, status model.DeviceCodeStatus, userID *int64) error
	UpdateDeviceCodeStatusAndScope(ctx context.Context, id int64, status model.DeviceCodeStatus, userID *int64, scope string) error
	ExchangeDeviceCodeStatus(ctx context.Context, id int64) error
	UpdateDeviceCodeLastPolled(ctx context.Context, id int64, interval int) error
	// Scope Grants
	UpsertScopeGrant(ctx context.Context, grant *model.OAuthClientScopeGrant) error
	FindActiveScopeGrants(ctx context.Context, clientID int64) ([]model.OAuthClientScopeGrant, error)
	// Users
	FindUserByID(ctx context.Context, id int64) (*model.User, error)
}

// Service orchestrates OAuth 2.1 operations.
type Service struct {
	repo   OAuthRepo
	config config.OAuthConfig
	appURL string
	logger *slog.Logger
}

// NewService creates a new OAuth service.
func NewService(repo OAuthRepo, oauthCfg config.OAuthConfig, appURL string, logger *slog.Logger) *Service {
	return &Service{
		repo:   repo,
		config: oauthCfg,
		appURL: appURL,
		logger: logger,
	}
}

// ClientTouchTimeout returns the configured timeout for background client
// last_used_at updates. Used by auth middleware for fire-and-forget goroutines.
func (s *Service) ClientTouchTimeout() time.Duration {
	return s.config.ClientTouchTimeout
}

// FindClient looks up an active client by its public client_id.
func (s *Service) FindClient(ctx context.Context, clientID string) (*model.OAuthClient, error) {
	return s.repo.FindClientByClientID(ctx, clientID)
}

// FindUserByID returns a user by ID.
func (s *Service) FindUserByID(ctx context.Context, id int64) (*model.User, error) {
	return s.repo.FindUserByID(ctx, id)
}

// FindClientByInternalID looks up a client by its database primary key.
func (s *Service) FindClientByInternalID(ctx context.Context, id int64) (*model.OAuthClient, error) {
	return s.repo.FindClientByID(ctx, id)
}

// TouchClientLastUsed records that a client's token was used.
func (s *Service) TouchClientLastUsed(ctx context.Context, clientID int64) error {
	return s.repo.TouchClientLastUsed(ctx, clientID)
}

// GrantClientScope records a time-bounded scope expansion for a client,
// created when a user approves consent for scopes beyond the client's base
// registration. The grant is upserted per (client, user) pair — re-approval
// refreshes the TTL and widens the granted scope.
func (s *Service) GrantClientScope(ctx context.Context, clientID int64, additionalScopes string, grantedByUserID int64) error {
	if additionalScopes == "" {
		return nil
	}

	var expiresAt sql.NullTime
	if s.config.ScopeGrantTTL > 0 {
		expiresAt = sql.NullTime{Time: time.Now().Add(s.config.ScopeGrantTTL), Valid: true}
	}

	grant := &model.OAuthClientScopeGrant{
		ClientID:  clientID,
		Scope:     additionalScopes,
		GrantedBy: grantedByUserID,
		ExpiresAt: expiresAt,
	}
	if err := s.repo.UpsertScopeGrant(ctx, grant); err != nil {
		return fmt.Errorf("upserting scope grant: %w", err)
	}

	s.logger.Info("granted client scope",
		"client_id", clientID,
		"scope", additionalScopes,
		"granted_by", grantedByUserID,
		"expires_at", expiresAt,
	)
	return nil
}

// EffectiveClientScope returns the union of a client's base registered scope
// and all active (non-expired) scope grants.
func (s *Service) EffectiveClientScope(ctx context.Context, clientID int64, baseScope string) (string, error) {
	grants, err := s.repo.FindActiveScopeGrants(ctx, clientID)
	if err != nil {
		return baseScope, fmt.Errorf("finding active scope grants: %w", err)
	}
	effective := baseScope
	for i := range grants {
		effective = authscope.Union(effective, grants[i].Scope)
	}
	return effective, nil
}

// FindDeviceCodeByUserCode looks up a pending device code by user code.
func (s *Service) FindDeviceCodeByUserCode(ctx context.Context, userCode string) (*model.OAuthDeviceCode, error) {
	return s.repo.FindDeviceCodeByUserCode(ctx, userCode)
}

// TokenResult holds the token endpoint response.
type TokenResult struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// ValidateAccessToken validates a bearer token and returns the access token model.
func (s *Service) ValidateAccessToken(ctx context.Context, bearerToken string) (*model.OAuthAccessToken, error) {
	id, tokenHash, err := ParseToken(bearerToken)
	if err != nil {
		return nil, errors.New("invalid token format")
	}

	token, err := s.repo.FindAccessTokenByToken(ctx, tokenHash)
	if err != nil {
		return nil, errors.New("invalid or expired token")
	}

	if token.ID != id {
		return nil, errors.New("invalid or expired token")
	}

	return token, nil
}

// ValidateState checks that a state parameter has a safe format.
func ValidateState(state string) bool {
	if len(state) > 500 {
		return false
	}
	return safeStateRegexp.MatchString(state)
}

// verifyClientAuth checks client secret for confidential clients.
func (s *Service) verifyClientAuth(client *model.OAuthClient, secret string) error {
	if client.TokenEndpointAuthMethod == "none" {
		return nil
	}
	if !client.ClientSecret.Valid || client.ClientSecret.String == "" {
		return errors.New("invalid client credentials")
	}
	if !VerifySecret(client.ClientSecret.String, secret) {
		return errors.New("invalid client credentials")
	}
	return nil
}

// issueTokenPair creates a new access token and refresh token pair.
func (s *Service) issueTokenPair(ctx context.Context, clientID int64, userID sql.NullInt64, scope string) (*TokenResult, error) {
	// Final scope validation gate
	if scope != "" {
		if invalid := authscope.ValidateAll(scope); len(invalid) > 0 {
			return nil, fmt.Errorf("invalid scopes in token: %s", strings.Join(invalid, ", "))
		}
	}

	// Generate access token
	accessTokenPair, err := GenerateToken()
	if err != nil {
		return nil, fmt.Errorf("generating access token: %w", err)
	}

	accessToken := &model.OAuthAccessToken{
		Token:     accessTokenPair.Hash,
		ClientID:  clientID,
		UserID:    userID,
		Scope:     sql.NullString{String: scope, Valid: scope != ""},
		ExpiresAt: time.Now().Add(s.config.AccessTokenLifetime),
		Revoked:   false,
	}

	if err = s.repo.CreateAccessToken(ctx, accessToken); err != nil {
		return nil, fmt.Errorf("creating access token: %w", err)
	}
	accessTokenPair.SetID(accessToken.ID)

	// Generate refresh token
	refreshTokenPair, err := GenerateToken()
	if err != nil {
		return nil, fmt.Errorf("generating refresh token: %w", err)
	}

	refreshToken := &model.OAuthRefreshToken{
		Token:         refreshTokenPair.Hash,
		AccessTokenID: accessToken.ID,
		ExpiresAt:     time.Now().Add(s.config.RefreshTokenLifetime),
		Revoked:       false,
	}

	if err := s.repo.CreateRefreshToken(ctx, refreshToken); err != nil {
		return nil, fmt.Errorf("creating refresh token: %w", err)
	}
	refreshTokenPair.SetID(refreshToken.ID)

	return &TokenResult{
		AccessToken:  accessTokenPair.Plaintext,
		TokenType:    "Bearer",
		ExpiresIn:    int(s.config.AccessTokenLifetime.Seconds()),
		RefreshToken: refreshTokenPair.Plaintext,
		Scope:        scope,
	}, nil
}

// parseTokenOrZero attempts to parse a token string into its ID and hash.
// Returns false if the token is malformed.
func parseTokenOrZero(plaintext string) (id int64, hash string, ok bool) {
	id, hash, err := ParseToken(plaintext)
	return id, hash, err == nil
}
