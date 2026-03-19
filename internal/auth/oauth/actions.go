package oauth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	authscope "git.999.haus/chris/DocuMCP-go/internal/auth/scope"
	"git.999.haus/chris/DocuMCP-go/internal/config"
	"git.999.haus/chris/DocuMCP-go/internal/model"
)

//nolint:revive // OAuthRepo is intentionally named to distinguish from other repository interfaces
type OAuthRepo interface {
	// Clients
	CreateClient(ctx context.Context, client *model.OAuthClient) error
	FindClientByClientID(ctx context.Context, clientID string) (*model.OAuthClient, error)
	FindClientByID(ctx context.Context, id int64) (*model.OAuthClient, error)
	// Authorization Codes
	CreateAuthorizationCode(ctx context.Context, code *model.OAuthAuthorizationCode) error
	FindAuthorizationCodeByCode(ctx context.Context, codeHash string) (*model.OAuthAuthorizationCode, error)
	RevokeAuthorizationCode(ctx context.Context, id int64) error
	// Access Tokens
	CreateAccessToken(ctx context.Context, token *model.OAuthAccessToken) error
	FindAccessTokenByID(ctx context.Context, id int64) (*model.OAuthAccessToken, error)
	FindAccessTokenByToken(ctx context.Context, tokenHash string) (*model.OAuthAccessToken, error)
	RevokeAccessToken(ctx context.Context, id int64) error
	// Refresh Tokens
	CreateRefreshToken(ctx context.Context, token *model.OAuthRefreshToken) error
	FindRefreshTokenByToken(ctx context.Context, tokenHash string) (*model.OAuthRefreshToken, error)
	RevokeRefreshToken(ctx context.Context, id int64) error
	RevokeRefreshTokenByAccessTokenID(ctx context.Context, accessTokenID int64) error
	// Device Codes
	CreateDeviceCode(ctx context.Context, dc *model.OAuthDeviceCode) error
	FindDeviceCodeByDeviceCode(ctx context.Context, deviceCodeHash string) (*model.OAuthDeviceCode, error)
	FindDeviceCodeByUserCode(ctx context.Context, userCode string) (*model.OAuthDeviceCode, error)
	UpdateDeviceCodeStatus(ctx context.Context, id int64, status string, userID *int64) error
	UpdateDeviceCodeLastPolled(ctx context.Context, id int64, interval int) error
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

// safeStateRegexp allows only safe characters in the state parameter.
var safeStateRegexp = regexp.MustCompile(`^[a-zA-Z0-9._~()'\-]+$`)

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

// FindDeviceCodeByUserCode looks up a pending device code by user code.
func (s *Service) FindDeviceCodeByUserCode(ctx context.Context, userCode string) (*model.OAuthDeviceCode, error) {
	return s.repo.FindDeviceCodeByUserCode(ctx, userCode)
}

//nolint:godot // ---------------------------------------------------------------------------
// Client Registration (RFC 7591).
//nolint:godot // ---------------------------------------------------------------------------

// RegisterClientParams holds the input for dynamic client registration.
type RegisterClientParams struct {
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	Scope                   string   `json:"scope"`
	SoftwareID              string   `json:"software_id"`
	SoftwareVersion         string   `json:"software_version"`
}

// RegisterClientResult holds the result of client registration.
type RegisterClientResult struct {
	ClientID                string   `json:"client_id"`
	ClientSecret            string   `json:"client_secret,omitempty"`
	ClientIDIssuedAt        int64    `json:"client_id_issued_at"`
	ClientSecretExpiresAt   int64    `json:"client_secret_expires_at,omitempty"`
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	Scope                   string   `json:"scope"`
}

// RegisterClient creates a new OAuth client per RFC 7591.
func (s *Service) RegisterClient(ctx context.Context, params RegisterClientParams) (*RegisterClientResult, error) {
	// Defaults
	if len(params.GrantTypes) == 0 {
		params.GrantTypes = []string{"authorization_code"}
	}
	if len(params.ResponseTypes) == 0 {
		params.ResponseTypes = []string{"code"}
	}
	if params.TokenEndpointAuthMethod == "" {
		params.TokenEndpointAuthMethod = "none"
	}
	if params.Scope == "" {
		params.Scope = authscope.DefaultScopes()
	}
	if invalid := authscope.ValidateAll(params.Scope); len(invalid) > 0 {
		return nil, fmt.Errorf("invalid scopes: %s", strings.Join(invalid, ", "))
	}

	clientID := uuid.New().String()

	redirectURIsJSON, err := json.Marshal(params.RedirectURIs)
	if err != nil {
		return nil, fmt.Errorf("marshaling redirect_uris: %w", err)
	}
	grantTypesJSON, err := json.Marshal(params.GrantTypes)
	if err != nil {
		return nil, fmt.Errorf("marshaling grant_types: %w", err)
	}
	responseTypesJSON, err := json.Marshal(params.ResponseTypes)
	if err != nil {
		return nil, fmt.Errorf("marshaling response_types: %w", err)
	}

	client := &model.OAuthClient{
		ClientID:                clientID,
		ClientName:              params.ClientName,
		RedirectURIs:            string(redirectURIsJSON),
		GrantTypes:              string(grantTypesJSON),
		ResponseTypes:           string(responseTypesJSON),
		TokenEndpointAuthMethod: params.TokenEndpointAuthMethod,
		Scope:                   sql.NullString{String: params.Scope, Valid: params.Scope != ""},
		SoftwareID:              sql.NullString{String: params.SoftwareID, Valid: params.SoftwareID != ""},
		SoftwareVersion:         sql.NullString{String: params.SoftwareVersion, Valid: params.SoftwareVersion != ""},
		IsActive:                true,
	}

	var plainSecret string
	isConfidential := params.TokenEndpointAuthMethod != "none"

	if isConfidential {
		plain, hashed, err := GenerateClientSecret()
		if err != nil {
			return nil, fmt.Errorf("generating client secret: %w", err)
		}
		plainSecret = plain
		client.ClientSecret = sql.NullString{String: hashed, Valid: true}
	}

	if err := s.repo.CreateClient(ctx, client); err != nil {
		return nil, fmt.Errorf("creating client: %w", err)
	}

	result := &RegisterClientResult{
		ClientID:                clientID,
		ClientIDIssuedAt:        client.CreatedAt.Time.Unix(),
		ClientName:              params.ClientName,
		RedirectURIs:            params.RedirectURIs,
		GrantTypes:              params.GrantTypes,
		ResponseTypes:           params.ResponseTypes,
		TokenEndpointAuthMethod: params.TokenEndpointAuthMethod,
		Scope:                   params.Scope,
	}

	if isConfidential {
		result.ClientSecret = plainSecret
		result.ClientSecretExpiresAt = 0
	}

	return result, nil
}

//nolint:godot // ---------------------------------------------------------------------------
// Authorization Code Generation.
//nolint:godot // ---------------------------------------------------------------------------

// GenerateAuthorizationCodeParams holds the input for generating an auth code.
type GenerateAuthorizationCodeParams struct {
	ClientID            int64
	UserID              int64
	RedirectURI         string
	Scope               string
	CodeChallenge       string
	CodeChallengeMethod string
}

// GenerateAuthorizationCode creates a new authorization code.
func (s *Service) GenerateAuthorizationCode(ctx context.Context, params GenerateAuthorizationCodeParams) (string, error) {
	// Validate requested scopes are known
	if params.Scope != "" {
		if invalid := authscope.ValidateAll(params.Scope); len(invalid) > 0 {
			return "", fmt.Errorf("invalid scopes: %s", strings.Join(invalid, ", "))
		}
	}

	// Verify requested scope is a subset of the client's allowed scope
	if params.Scope != "" {
		client, err := s.repo.FindClientByID(ctx, params.ClientID)
		if err != nil {
			return "", fmt.Errorf("looking up client: %w", err)
		}
		if client == nil {
			return "", fmt.Errorf("client not found")
		}
		clientScope := ""
		if client.Scope.Valid {
			clientScope = client.Scope.String
		}
		if clientScope != "" && !authscope.IsSubset(params.Scope, clientScope) {
			return "", fmt.Errorf("requested scope exceeds client's allowed scope")
		}
	}

	token, err := GenerateToken()
	if err != nil {
		return "", fmt.Errorf("generating authorization code: %w", err)
	}

	code := &model.OAuthAuthorizationCode{
		Code:        token.Hash,
		ClientID:    params.ClientID,
		UserID:      sql.NullInt64{Int64: params.UserID, Valid: params.UserID > 0},
		RedirectURI: params.RedirectURI,
		Scope:       sql.NullString{String: params.Scope, Valid: params.Scope != ""},
		CodeChallenge: sql.NullString{
			String: params.CodeChallenge,
			Valid:  params.CodeChallenge != "",
		},
		CodeChallengeMethod: sql.NullString{
			String: params.CodeChallengeMethod,
			Valid:  params.CodeChallengeMethod != "",
		},
		ExpiresAt: time.Now().Add(s.config.AuthCodeLifetime),
		Revoked:   false,
	}

	if err := s.repo.CreateAuthorizationCode(ctx, code); err != nil {
		return "", fmt.Errorf("persisting authorization code: %w", err)
	}

	token.SetID(code.ID)
	return token.Plaintext, nil
}

//nolint:godot // ---------------------------------------------------------------------------
// Authorization Code Exchange.
//nolint:godot // ---------------------------------------------------------------------------

// ExchangeAuthorizationCodeParams holds the input for exchanging an auth code.
type ExchangeAuthorizationCodeParams struct {
	Code         string
	ClientID     string
	ClientSecret string
	RedirectURI  string
	CodeVerifier string
}

// TokenResult holds the token endpoint response.
type TokenResult struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// ExchangeAuthorizationCode exchanges an auth code for tokens.
func (s *Service) ExchangeAuthorizationCode(ctx context.Context, params ExchangeAuthorizationCodeParams) (*TokenResult, error) {
	// Parse the authorization code
	codeID, codeHash, err := ParseToken(params.Code)
	if err != nil {
		return nil, fmt.Errorf("invalid authorization code")
	}

	// Look up the client
	client, err := s.repo.FindClientByClientID(ctx, params.ClientID)
	if err != nil {
		return nil, fmt.Errorf("invalid client credentials")
	}

	// Verify client secret for confidential clients
	err = s.verifyClientAuth(client, params.ClientSecret)
	if err != nil {
		return nil, err
	}

	// Look up the authorization code by hash
	authCode, err := s.repo.FindAuthorizationCodeByCode(ctx, codeHash)
	if err != nil {
		return nil, fmt.Errorf("invalid authorization code")
	}

	// Verify code belongs to this client and matches the ID
	if authCode.ClientID != client.ID || authCode.ID != codeID {
		return nil, fmt.Errorf("invalid authorization code")
	}

	// Check expiry
	if time.Now().After(authCode.ExpiresAt) {
		return nil, fmt.Errorf("authorization code is expired or revoked")
	}

	// Verify redirect URI
	if authCode.RedirectURI != params.RedirectURI {
		return nil, fmt.Errorf("redirect URI mismatch")
	}

	// PKCE verification
	if authCode.CodeChallenge.Valid && authCode.CodeChallenge.String != "" {
		if params.CodeVerifier == "" {
			return nil, fmt.Errorf("code verifier required for PKCE")
		}
		if !VerifyPKCE(authCode.CodeChallenge.String, params.CodeVerifier) {
			return nil, fmt.Errorf("invalid PKCE code verifier")
		}
	} else if params.CodeVerifier != "" {
		// RFC 9700: reject code_verifier when no code_challenge was used
		return nil, fmt.Errorf("unexpected code_verifier - authorization request did not include code_challenge")
	}

	// Revoke the authorization code (one-time use)
	if err := s.repo.RevokeAuthorizationCode(ctx, authCode.ID); err != nil {
		return nil, fmt.Errorf("revoking authorization code: %w", err)
	}

	// Issue tokens
	scope := ""
	if authCode.Scope.Valid {
		scope = authCode.Scope.String
	}

	// Defense-in-depth: validate scope against client's allowed scope
	if scope != "" {
		clientScope := ""
		if client.Scope.Valid {
			clientScope = client.Scope.String
		}
		if clientScope != "" && !authscope.IsSubset(scope, clientScope) {
			return nil, fmt.Errorf("authorization code scope exceeds client's allowed scope")
		}
	}

	return s.issueTokenPair(ctx, client.ID, authCode.UserID, scope)
}

//nolint:godot // ---------------------------------------------------------------------------
// Refresh Token.
//nolint:godot // ---------------------------------------------------------------------------

// RefreshTokenParams holds the input for refreshing tokens.
type RefreshTokenParams struct {
	RefreshToken string
	ClientID     string
	ClientSecret string
	Scope        string
}

// RefreshAccessToken exchanges a refresh token for new tokens (rotation).
func (s *Service) RefreshAccessToken(ctx context.Context, params RefreshTokenParams) (*TokenResult, error) {
	// Parse the refresh token
	_, tokenHash, err := ParseToken(params.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("refresh token not found")
	}

	// Look up the client
	client, err := s.repo.FindClientByClientID(ctx, params.ClientID)
	if err != nil {
		return nil, fmt.Errorf("invalid client credentials")
	}

	// Verify client secret
	err = s.verifyClientAuth(client, params.ClientSecret)
	if err != nil {
		return nil, err
	}

	// Look up the refresh token
	refreshToken, err := s.repo.FindRefreshTokenByToken(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("refresh token not found")
	}

	// Verify the refresh token's access token belongs to this client
	accessToken, err := s.repo.FindAccessTokenByID(ctx, refreshToken.AccessTokenID)
	if err != nil {
		return nil, fmt.Errorf("associated access token not found")
	}
	if accessToken.ClientID != client.ID {
		return nil, fmt.Errorf("refresh token does not belong to this client")
	}

	// Revoke old tokens (rotation)
	if err := s.repo.RevokeAccessToken(ctx, accessToken.ID); err != nil {
		return nil, fmt.Errorf("revoking old access token: %w", err)
	}
	if err := s.repo.RevokeRefreshToken(ctx, refreshToken.ID); err != nil {
		return nil, fmt.Errorf("revoking old refresh token: %w", err)
	}

	// Use original scope unless a narrower scope is requested.
	// Per RFC 6749 Section 6, the requested scope MUST NOT include any
	// scope not originally granted by the resource owner.
	originalScope := ""
	if accessToken.Scope.Valid {
		originalScope = accessToken.Scope.String
	}
	scope := originalScope
	if params.Scope != "" {
		if !authscope.IsSubset(params.Scope, originalScope) {
			return nil, fmt.Errorf("requested scope exceeds original grant")
		}
		scope = params.Scope
	}

	return s.issueTokenPair(ctx, client.ID, accessToken.UserID, scope)
}

//nolint:godot // ---------------------------------------------------------------------------
// Token Revocation (RFC 7009).
//nolint:godot // ---------------------------------------------------------------------------

// RevokeTokenParams holds the input for token revocation.
type RevokeTokenParams struct {
	Token         string
	ClientID      string
	ClientSecret  string
	TokenTypeHint string
}

// RevokeToken revokes an access or refresh token. Per RFC 7009, always succeeds.
func (s *Service) RevokeToken(ctx context.Context, params RevokeTokenParams) error {
	// Verify client
	client, err := s.repo.FindClientByClientID(ctx, params.ClientID)
	if err != nil {
		return fmt.Errorf("invalid client credentials")
	}
	err = s.verifyClientAuth(client, params.ClientSecret)
	if err != nil {
		return err
	}

	// Parse the token
	id, tokenHash, err := ParseToken(params.Token)
	if err != nil {
		// Per RFC 7009, return success even for invalid tokens
		return nil
	}

	// Try to revoke based on hint
	switch params.TokenTypeHint {
	case "refresh_token":
		s.tryRevokeRefreshToken(ctx, tokenHash, id)
	case "access_token":
		s.tryRevokeAccessToken(ctx, tokenHash)
	default:
		// Try access token first, then refresh token
		s.tryRevokeAccessToken(ctx, tokenHash)
		s.tryRevokeRefreshToken(ctx, tokenHash, id)
	}

	return nil
}

func (s *Service) tryRevokeAccessToken(ctx context.Context, tokenHash string) {
	token, err := s.repo.FindAccessTokenByToken(ctx, tokenHash)
	if err != nil {
		return
	}
	_ = s.repo.RevokeAccessToken(ctx, token.ID)
	// Also revoke associated refresh tokens
	_ = s.repo.RevokeRefreshTokenByAccessTokenID(ctx, token.ID)
}

func (s *Service) tryRevokeRefreshToken(ctx context.Context, tokenHash string, _ int64) {
	token, err := s.repo.FindRefreshTokenByToken(ctx, tokenHash)
	if err != nil {
		return
	}
	_ = s.repo.RevokeRefreshToken(ctx, token.ID)
	// Also revoke the associated access token
	_ = s.repo.RevokeAccessToken(ctx, token.AccessTokenID)
}

//nolint:godot // ---------------------------------------------------------------------------
// Device Authorization (RFC 8628).
//nolint:godot // ---------------------------------------------------------------------------

// DeviceAuthorizationParams holds the input for device authorization.
type DeviceAuthorizationParams struct {
	ClientID string
	Scope    string
}

// DeviceAuthorizationResult holds the device authorization response.
type DeviceAuthorizationResult struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// GenerateDeviceCode creates a new device authorization code (RFC 8628).
func (s *Service) GenerateDeviceCode(ctx context.Context, params DeviceAuthorizationParams) (*DeviceAuthorizationResult, error) {
	// Look up the client
	client, err := s.repo.FindClientByClientID(ctx, params.ClientID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("invalid or inactive client")
		}
		return nil, fmt.Errorf("invalid or inactive client")
	}

	// Check client supports device_code grant
	grantTypes, err := client.ParseGrantTypes()
	if err != nil {
		return nil, fmt.Errorf("client does not support device_code grant type")
	}
	if !slices.Contains(grantTypes, "urn:ietf:params:oauth:grant-type:device_code") {
		return nil, fmt.Errorf("client does not support device_code grant type")
	}

	// Validate requested scopes
	if params.Scope != "" {
		if invalid := authscope.ValidateAll(params.Scope); len(invalid) > 0 {
			return nil, fmt.Errorf("invalid scopes: %s", strings.Join(invalid, ", "))
		}
		// Verify requested scope is a subset of client's allowed scope
		clientScope := ""
		if client.Scope.Valid {
			clientScope = client.Scope.String
		}
		if clientScope != "" && !authscope.IsSubset(params.Scope, clientScope) {
			return nil, fmt.Errorf("requested scope exceeds client's allowed scope")
		}
	}

	// Generate device code token
	token, err := GenerateToken()
	if err != nil {
		return nil, fmt.Errorf("generating device code: %w", err)
	}

	// Generate user code
	userCode, err := GenerateUserCode()
	if err != nil {
		return nil, fmt.Errorf("generating user code: %w", err)
	}

	verificationURI := s.appURL + "/oauth/device"
	verificationURIComplete := verificationURI + "?user_code=" + userCode

	interval := max(int(s.config.DeviceCodeInterval.Seconds()), 5)

	dc := &model.OAuthDeviceCode{
		DeviceCode:              token.Hash,
		UserCode:                userCode,
		ClientID:                client.ID,
		Scope:                   sql.NullString{String: params.Scope, Valid: params.Scope != ""},
		VerificationURI:         verificationURI,
		VerificationURIComplete: sql.NullString{String: verificationURIComplete, Valid: true},
		Interval:                interval,
		Status:                  "pending",
		ExpiresAt:               time.Now().Add(s.config.DeviceCodeLifetime),
	}

	if err := s.repo.CreateDeviceCode(ctx, dc); err != nil {
		return nil, fmt.Errorf("persisting device code: %w", err)
	}

	token.SetID(dc.ID)

	return &DeviceAuthorizationResult{
		DeviceCode:              token.Plaintext,
		UserCode:                userCode,
		VerificationURI:         verificationURI,
		VerificationURIComplete: verificationURIComplete,
		ExpiresIn:               int(s.config.DeviceCodeLifetime.Seconds()),
		Interval:                interval,
	}, nil
}

// AuthorizeDeviceCode approves or denies a device authorization request.
func (s *Service) AuthorizeDeviceCode(ctx context.Context, userCode string, userID int64, approved bool) error {
	dc, err := s.repo.FindDeviceCodeByUserCode(ctx, userCode)
	if err != nil {
		return fmt.Errorf("invalid or expired user code")
	}

	if time.Now().After(dc.ExpiresAt) {
		return fmt.Errorf("device code has expired")
	}

	if dc.Status != "pending" {
		return fmt.Errorf("device code is not in valid state for authorization")
	}

	status := "authorized"
	if !approved {
		status = "denied"
	}

	return s.repo.UpdateDeviceCodeStatus(ctx, dc.ID, status, &userID)
}

// DeviceCodeError represents an error in the device code exchange flow.
type DeviceCodeError struct {
	Code        string
	Description string
}

func (e *DeviceCodeError) Error() string {
	return e.Description
}

// ExchangeDeviceCodeParams holds the input for exchanging a device code.
type ExchangeDeviceCodeParams struct {
	DeviceCode   string
	ClientID     string
	ClientSecret string
}

// ExchangeDeviceCode polls for and exchanges a device code for tokens.
func (s *Service) ExchangeDeviceCode(ctx context.Context, params ExchangeDeviceCodeParams) (*TokenResult, error) {
	// Look up the client
	client, err := s.repo.FindClientByClientID(ctx, params.ClientID)
	if err != nil {
		return nil, &DeviceCodeError{Code: "invalid_client", Description: "Invalid client"}
	}
	if err = s.verifyClientAuth(client, params.ClientSecret); err != nil {
		return nil, &DeviceCodeError{Code: "invalid_client", Description: "Invalid client credentials"}
	}

	// Parse the device code
	_, deviceCodeHash, err := ParseToken(params.DeviceCode)
	if err != nil {
		return nil, &DeviceCodeError{Code: "invalid_grant", Description: "Invalid device code"}
	}

	// Look up the device code
	dc, err := s.repo.FindDeviceCodeByDeviceCode(ctx, deviceCodeHash)
	if err != nil {
		return nil, &DeviceCodeError{Code: "invalid_grant", Description: "Invalid device code"}
	}

	// Verify client matches
	if dc.ClientID != client.ID {
		return nil, &DeviceCodeError{Code: "invalid_grant", Description: "Invalid device code"}
	}

	// Check expiry
	if time.Now().After(dc.ExpiresAt) {
		return nil, &DeviceCodeError{Code: "expired_token", Description: "The device code has expired"}
	}

	// Check polling rate
	if dc.LastPolledAt.Valid {
		elapsed := time.Since(dc.LastPolledAt.Time)
		if elapsed < time.Duration(dc.Interval)*time.Second {
			// Increase interval by 5 seconds, capped at 300s per RFC 8628.
			newInterval := min(dc.Interval+5, 300)
			_ = s.repo.UpdateDeviceCodeLastPolled(ctx, dc.ID, newInterval)
			return nil, &DeviceCodeError{
				Code:        "slow_down",
				Description: fmt.Sprintf("Polling too fast. Increase interval to %d seconds", newInterval),
			}
		}
	}

	// Update last polled
	_ = s.repo.UpdateDeviceCodeLastPolled(ctx, dc.ID, dc.Interval)

	switch dc.Status {
	case "pending":
		return nil, &DeviceCodeError{Code: "authorization_pending", Description: "The authorization request is still pending"}
	case "denied":
		return nil, &DeviceCodeError{Code: "access_denied", Description: "The user denied the authorization request"}
	case "exchanged":
		return nil, &DeviceCodeError{Code: "invalid_grant", Description: "Device code has already been used"}
	case "authorized":
		// Mark as exchanged
		if err := s.repo.UpdateDeviceCodeStatus(ctx, dc.ID, "exchanged", nil); err != nil {
			return nil, fmt.Errorf("updating device code status: %w", err)
		}

		scope := ""
		if dc.Scope.Valid {
			scope = dc.Scope.String
		}

		return s.issueTokenPair(ctx, client.ID, dc.UserID, scope)
	default:
		return nil, &DeviceCodeError{Code: "invalid_grant", Description: "Device code not in valid state for exchange"}
	}
}

//nolint:godot // ---------------------------------------------------------------------------
// Token Validation (for middleware).
//nolint:godot // ---------------------------------------------------------------------------

// ValidateAccessToken validates a bearer token and returns the access token model.
func (s *Service) ValidateAccessToken(ctx context.Context, bearerToken string) (*model.OAuthAccessToken, error) {
	id, tokenHash, err := ParseToken(bearerToken)
	if err != nil {
		return nil, fmt.Errorf("invalid token format")
	}

	token, err := s.repo.FindAccessTokenByToken(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired token")
	}

	if token.ID != id {
		return nil, fmt.Errorf("invalid or expired token")
	}

	return token, nil
}

//nolint:godot // ---------------------------------------------------------------------------
// Consent Validation.
//nolint:godot // ---------------------------------------------------------------------------

// ValidateState checks that a state parameter has a safe format.
func ValidateState(state string) bool {
	if len(state) > 500 {
		return false
	}
	return safeStateRegexp.MatchString(state)
}

//nolint:godot // ---------------------------------------------------------------------------
// Internal helpers.
//nolint:godot // ---------------------------------------------------------------------------


// verifyClientAuth checks client secret for confidential clients.
func (s *Service) verifyClientAuth(client *model.OAuthClient, secret string) error {
	if client.TokenEndpointAuthMethod == "none" {
		return nil
	}
	if !client.ClientSecret.Valid || client.ClientSecret.String == "" {
		return fmt.Errorf("invalid client credentials")
	}
	if !VerifySecret(client.ClientSecret.String, secret) {
		return fmt.Errorf("invalid client credentials")
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
