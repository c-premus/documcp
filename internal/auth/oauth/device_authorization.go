package oauth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	authscope "github.com/c-premus/documcp/internal/auth/scope"
	"github.com/c-premus/documcp/internal/model"
)

// Sentinel errors for OAuth grant validation.
var (
	ErrInvalidClient    = errors.New("invalid or inactive client")
	ErrUnsupportedGrant = errors.New("client does not support the requested grant type")
)

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
		return nil, ErrInvalidClient
	}

	// Check client supports device_code grant
	grantTypes, err := client.ParseGrantTypes()
	if err != nil {
		return nil, ErrUnsupportedGrant
	}
	if !slices.Contains(grantTypes, "urn:ietf:params:oauth:grant-type:device_code") {
		return nil, ErrUnsupportedGrant
	}

	// Validate requested scopes
	if params.Scope != "" {
		if invalid := authscope.ValidateAll(params.Scope); len(invalid) > 0 {
			return nil, fmt.Errorf("invalid scopes: %s", strings.Join(invalid, ", "))
		}
		// NOTE: We do not check IsSubset against the client's registered scope
		// here. The client scope may be expanded when a user with broader
		// entitlements approves the device code (see AuthorizeDeviceCode).
		// Scope narrowing happens at approval time, not at device code creation.
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
		Status:                  model.DeviceCodeStatusPending,
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
// When approved, the device code scope is narrowed to the intersection of the
// requested scope and the approving user's entitlements (admin vs regular).
func (s *Service) AuthorizeDeviceCode(ctx context.Context, userCode string, userID int64, approved bool) error {
	dc, err := s.repo.FindDeviceCodeByUserCode(ctx, userCode)
	if err != nil {
		return errors.New("invalid or expired user code")
	}

	if time.Now().After(dc.ExpiresAt) {
		return errors.New("device code has expired")
	}

	if dc.Status != model.DeviceCodeStatusPending {
		return errors.New("device code is not in valid state for authorization")
	}

	if !approved {
		return s.repo.UpdateDeviceCodeStatus(ctx, dc.ID, model.DeviceCodeStatusDenied, &userID)
	}

	// Narrow scope to the approving user's entitlements.
	scope := ""
	if dc.Scope.Valid {
		scope = dc.Scope.String
	}
	if scope != "" {
		user, err := s.repo.FindUserByID(ctx, userID)
		if err != nil {
			return fmt.Errorf("looking up approving user: %w", err)
		}
		// Record a time-bounded scope grant (replaces permanent scope widening).
		userEntitlements := authscope.UserScopes(user.IsAdmin)
		if entitled := authscope.Intersect(scope, userEntitlements); entitled != "" {
			if grantErr := s.GrantClientScope(ctx, dc.ClientID, entitled, userID); grantErr != nil {
				s.logger.Error("granting client scope for device flow", "error", grantErr)
			}
		}
		scope = authscope.Intersect(scope, userEntitlements)
		if scope == "" {
			return errors.New("none of the requested scopes are available to your account")
		}
	}

	return s.repo.UpdateDeviceCodeStatusAndScope(ctx, dc.ID, model.DeviceCodeStatusAuthorized, &userID, scope)
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
	case model.DeviceCodeStatusPending:
		return nil, &DeviceCodeError{Code: "authorization_pending", Description: "The authorization request is still pending"}
	case model.DeviceCodeStatusDenied:
		return nil, &DeviceCodeError{Code: "access_denied", Description: "The user denied the authorization request"}
	case model.DeviceCodeStatusExchanged:
		return nil, &DeviceCodeError{Code: "invalid_grant", Description: "Device code has already been used"}
	case model.DeviceCodeStatusAuthorized:
		// Atomically mark as exchanged — prevents concurrent polls from minting
		// multiple token pairs (TOCTOU race). Returns error if already consumed.
		if err := s.repo.ExchangeDeviceCodeStatus(ctx, dc.ID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, &DeviceCodeError{Code: "invalid_grant", Description: "Device code has already been used"}
			}
			return nil, fmt.Errorf("exchanging device code status: %w", err)
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
