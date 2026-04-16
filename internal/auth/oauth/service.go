package oauth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	authscope "github.com/c-premus/documcp/internal/auth/scope"
	"github.com/c-premus/documcp/internal/config"
	"github.com/c-premus/documcp/internal/model"
)

// tokenReplayTotal counts detected OAuth token replays. A non-zero rate on
// this counter is evidence of interception (stolen code or refresh token).
// Labeled by replay type so alerts can distinguish auth-code replay
// (security.md M1) from refresh-token reuse (M2). Registered at init time
// on the default Prometheus registry.
var tokenReplayTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "documcp",
		Subsystem: "oauth",
		Name:      "token_replay_total",
		Help:      "Total detected OAuth token replays (auth-code or refresh-token reuse).",
	},
	[]string{"type"},
)

func init() {
	prometheus.MustRegister(tokenReplayTotal)
}

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
	FindAuthorizationCodeByCodeIncludingRevoked(ctx context.Context, codeHash string) (*model.OAuthAuthorizationCode, error)
	RevokeAuthorizationCode(ctx context.Context, id int64) error
	// Access Tokens
	CreateAccessToken(ctx context.Context, token *model.OAuthAccessToken) error
	FindAccessTokenByID(ctx context.Context, id int64) (*model.OAuthAccessToken, error)
	FindAccessTokenByToken(ctx context.Context, tokenHash string) (*model.OAuthAccessToken, error)
	RevokeAccessToken(ctx context.Context, id int64) error
	RevokeTokenPair(ctx context.Context, accessTokenID, refreshTokenID int64) error
	RevokeTokenFamilyByAuthorizationCodeID(ctx context.Context, authCodeID int64) (int64, error)
	// Refresh Tokens
	CreateRefreshToken(ctx context.Context, token *model.OAuthRefreshToken) error
	FindRefreshTokenByToken(ctx context.Context, tokenHash string) (*model.OAuthRefreshToken, error)
	FindRefreshTokenByTokenIgnoringRevocation(ctx context.Context, tokenHash string) (*model.OAuthRefreshToken, error)
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

	// hmacKey is the server-side key for HMAC-SHA256 token hashing. When
	// non-empty, token hashes are keyed, preventing offline brute-force if
	// the database is compromised. When nil, falls back to plain SHA-256
	// (still safe for high-entropy tokens).
	hmacKey []byte

	// hmacWarnOnce ensures the SHA-256 fallback warning logs only once per
	// service instance when hmacKey is nil.
	hmacWarnOnce sync.Once
}

// NewService creates a new OAuth service. hmacKey keys token hashing when
// non-empty; pass nil to fall back to plain SHA-256 (appropriate for tests).
func NewService(repo OAuthRepo, oauthCfg config.OAuthConfig, appURL string, logger *slog.Logger, hmacKey []byte) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		repo:    repo,
		config:  oauthCfg,
		appURL:  appURL,
		logger:  logger,
		hmacKey: hmacKey,
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
	id, tokenHash, err := s.ParseToken(bearerToken)
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

// issueTokenPair creates a new access token and refresh token pair. The
// resource argument is the RFC 8707 audience the token is bound to; empty
// means unbound (token will fail audience-checked middleware).
//
// authCodeID is the authorization code that originally authorized this
// lineage — set for code-grant issuance and propagated unchanged through
// refresh rotations. Zero means "no parent auth code" (device flow). It is
// stored on the access-token row so that replay detection in either the
// code-exchange or the refresh path can revoke every descendant in the
// lineage with one query (security.md M1 + M2).
func (s *Service) issueTokenPair(ctx context.Context, clientID int64, userID sql.NullInt64, scope, resource string, authCodeID int64) (*TokenResult, error) {
	// Final scope validation gate
	if scope != "" {
		if invalid := authscope.ValidateAll(scope); len(invalid) > 0 {
			return nil, fmt.Errorf("invalid scopes in token: %s", strings.Join(invalid, ", "))
		}
	}

	// Generate access token
	accessTokenPair, err := s.GenerateToken()
	if err != nil {
		return nil, fmt.Errorf("generating access token: %w", err)
	}

	accessToken := &model.OAuthAccessToken{
		Token:     accessTokenPair.Hash,
		ClientID:  clientID,
		UserID:    userID,
		Scope:     sql.NullString{String: scope, Valid: scope != ""},
		Resource:  sql.NullString{String: resource, Valid: resource != ""},
		ExpiresAt: time.Now().Add(s.config.AccessTokenLifetime),
		Revoked:   false,
	}

	if authCodeID > 0 {
		accessToken.AuthorizationCodeID = sql.NullInt64{Int64: authCodeID, Valid: true}
	}

	if err = s.repo.CreateAccessToken(ctx, accessToken); err != nil {
		return nil, fmt.Errorf("creating access token: %w", err)
	}
	accessTokenPair.SetID(accessToken.ID)

	// Generate refresh token
	refreshTokenPair, err := s.GenerateToken()
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

