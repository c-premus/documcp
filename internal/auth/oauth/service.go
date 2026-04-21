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

// ClientRepo accesses OAuth client registrations.
type ClientRepo interface {
	CreateClient(ctx context.Context, client *model.OAuthClient) error
	FindClientByClientID(ctx context.Context, clientID string) (*model.OAuthClient, error)
	FindClientByID(ctx context.Context, id int64) (*model.OAuthClient, error)
	TouchClientLastUsed(ctx context.Context, clientID int64) error
	UpdateClientScope(ctx context.Context, clientID int64, scope string) error
}

// AuthCodeRepo accesses authorization code rows.
type AuthCodeRepo interface {
	CreateAuthorizationCode(ctx context.Context, code *model.OAuthAuthorizationCode) error
	FindAuthorizationCodeByCode(ctx context.Context, codeHash string) (*model.OAuthAuthorizationCode, error)
	FindAuthorizationCodeByCodeIncludingRevoked(ctx context.Context, codeHash string) (*model.OAuthAuthorizationCode, error)
	RevokeAuthorizationCode(ctx context.Context, id int64) error
}

// AccessTokenRepo accesses bearer access token rows.
type AccessTokenRepo interface {
	CreateAccessToken(ctx context.Context, token *model.OAuthAccessToken) error
	FindAccessTokenByID(ctx context.Context, id int64) (*model.OAuthAccessToken, error)
	FindAccessTokenByToken(ctx context.Context, tokenHash string) (*model.OAuthAccessToken, error)
	RevokeAccessToken(ctx context.Context, id int64) error
	RevokeTokenPair(ctx context.Context, accessTokenID, refreshTokenID int64) error
	RevokeTokenFamilyByAuthorizationCodeID(ctx context.Context, authCodeID int64) (int64, error)
	RevokeUserTokensSince(ctx context.Context, userID int64, since time.Time) (int64, error)
}

// RefreshTokenRepo accesses refresh token rows.
type RefreshTokenRepo interface {
	CreateRefreshToken(ctx context.Context, token *model.OAuthRefreshToken) error
	FindRefreshTokenByToken(ctx context.Context, tokenHash string) (*model.OAuthRefreshToken, error)
	FindRefreshTokenByTokenIgnoringRevocation(ctx context.Context, tokenHash string) (*model.OAuthRefreshToken, error)
	RevokeRefreshToken(ctx context.Context, id int64) error
	RevokeRefreshTokenByAccessTokenID(ctx context.Context, accessTokenID int64) error
}

// DeviceCodeRepo accesses device code rows (RFC 8628).
type DeviceCodeRepo interface {
	CreateDeviceCode(ctx context.Context, dc *model.OAuthDeviceCode) error
	FindDeviceCodeByDeviceCode(ctx context.Context, deviceCodeHash string) (*model.OAuthDeviceCode, error)
	FindDeviceCodeByUserCode(ctx context.Context, userCode string) (*model.OAuthDeviceCode, error)
	UpdateDeviceCodeStatus(ctx context.Context, id int64, status model.DeviceCodeStatus, userID *int64) error
	UpdateDeviceCodeStatusAndScope(ctx context.Context, id int64, status model.DeviceCodeStatus, userID *int64, scope string) error
	ExchangeDeviceCodeStatus(ctx context.Context, id int64) error
	UpdateDeviceCodeLastPolled(ctx context.Context, id int64, interval int) error
}

// ScopeGrantRepo accesses time-bounded client scope grants recorded at
// consent time.
type ScopeGrantRepo interface {
	UpsertScopeGrant(ctx context.Context, grant *model.OAuthClientScopeGrant) error
	FindActiveScopeGrants(ctx context.Context, clientID int64) ([]model.OAuthClientScopeGrant, error)
}

// UserLookup provides the read-only user queries the OAuth service needs.
type UserLookup interface {
	FindUserByID(ctx context.Context, id int64) (*model.User, error)
}

// OAuthRepo composes every role-specific interface used by the OAuth service,
// retained so a single repository value can satisfy the whole surface at
// NewService construction time. Individual code paths should accept the
// narrowest interface they need, not this umbrella.
//
//nolint:revive // OAuthRepo is intentionally named to distinguish from other repository interfaces
type OAuthRepo interface {
	ClientRepo
	AuthCodeRepo
	AccessTokenRepo
	RefreshTokenRepo
	DeviceCodeRepo
	ScopeGrantRepo
	UserLookup
}

// Service orchestrates OAuth 2.1 operations. The role-specific repository
// fields (clients, authCodes, accessTokens, refreshTokens, deviceCodes,
// scopeGrants, users) are narrow interfaces on the same backing repository;
// they make the access pattern of each method explicit and shrink mocks.
type Service struct {
	clients       ClientRepo
	authCodes     AuthCodeRepo
	accessTokens  AccessTokenRepo
	refreshTokens RefreshTokenRepo
	deviceCodes   DeviceCodeRepo
	scopeGrants   ScopeGrantRepo
	users         UserLookup

	config config.OAuthConfig
	appURL string
	logger *slog.Logger

	// hmacKeys holds the HMAC signing keys used for token hashing, newest-first.
	// hmacKeys[0] is the PRIMARY — every fresh hash is signed with it.
	// Subsequent entries are retired keys accepted on verify paths so a token
	// minted before rotation still authenticates (security M2). NewService
	// rejects an empty slice, so hashToken always has at least one key
	// available — there is no silent SHA-256 fallback (security L4).
	hmacKeys []HMACKey
}

// NewService creates a new OAuth service. repo must satisfy the composite
// OAuthRepo interface; it is split across the role-specific fields
// internally. hmacKeys is required non-empty — hmacKeys[0] is the primary
// signing key, the remainder (if any) are rotation keys accepted on verify.
// Returns an error when hmacKeys is empty so production builds fail-boot
// rather than silently falling back to unkeyed SHA-256 (security L4).
func NewService(repo OAuthRepo, oauthCfg config.OAuthConfig, appURL string, logger *slog.Logger, hmacKeys []HMACKey) (*Service, error) {
	if len(hmacKeys) == 0 {
		return nil, errors.New("oauth.NewService: at least one HMAC key is required (see security L4)")
	}
	for i, k := range hmacKeys {
		if len(k.Key) == 0 {
			return nil, fmt.Errorf("oauth.NewService: hmacKeys[%d].Key is empty", i)
		}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		clients:       repo,
		authCodes:     repo,
		accessTokens:  repo,
		refreshTokens: repo,
		deviceCodes:   repo,
		scopeGrants:   repo,
		users:         repo,
		config:        oauthCfg,
		appURL:        appURL,
		logger:        logger,
		hmacKeys:      hmacKeys,
	}, nil
}

// ClientTouchTimeout returns the configured timeout for background client
// last_used_at updates. Used by auth middleware for fire-and-forget goroutines.
func (s *Service) ClientTouchTimeout() time.Duration {
	return s.config.ClientTouchTimeout
}

// FindClient looks up an active client by its public client_id.
func (s *Service) FindClient(ctx context.Context, clientID string) (*model.OAuthClient, error) {
	return s.clients.FindClientByClientID(ctx, clientID)
}

// FindUserByID returns a user by ID.
func (s *Service) FindUserByID(ctx context.Context, id int64) (*model.User, error) {
	return s.users.FindUserByID(ctx, id)
}

// RevokeUserTokensSince revokes every live access + refresh token minted for
// userID at or after since. Used by session logout to invalidate OAuth grants
// issued during the session being terminated (security L7).
func (s *Service) RevokeUserTokensSince(ctx context.Context, userID int64, since time.Time) (int64, error) {
	return s.accessTokens.RevokeUserTokensSince(ctx, userID, since)
}

// FindClientByInternalID looks up a client by its database primary key.
func (s *Service) FindClientByInternalID(ctx context.Context, id int64) (*model.OAuthClient, error) {
	return s.clients.FindClientByID(ctx, id)
}

// TouchClientLastUsed records that a client's token was used.
func (s *Service) TouchClientLastUsed(ctx context.Context, clientID int64) error {
	return s.clients.TouchClientLastUsed(ctx, clientID)
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
	if err := s.scopeGrants.UpsertScopeGrant(ctx, grant); err != nil {
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
	grants, err := s.scopeGrants.FindActiveScopeGrants(ctx, clientID)
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
	return s.deviceCodes.FindDeviceCodeByUserCode(ctx, userCode)
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
// During HMAC key rotation, every configured key is tried — a token minted
// under a retired key still authenticates until the operator drops the key.
func (s *Service) ValidateAccessToken(ctx context.Context, bearerToken string) (*model.OAuthAccessToken, error) {
	id, hashes, err := s.parseTokenCandidateHashes(bearerToken)
	if err != nil {
		return nil, errors.New("invalid token format")
	}

	for _, hash := range hashes {
		token, findErr := s.accessTokens.FindAccessTokenByToken(ctx, hash)
		if findErr != nil {
			continue
		}
		if token.ID != id {
			continue
		}
		return token, nil
	}
	return nil, errors.New("invalid or expired token")
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

	if err = s.accessTokens.CreateAccessToken(ctx, accessToken); err != nil {
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

	if err := s.refreshTokens.CreateRefreshToken(ctx, refreshToken); err != nil {
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
