package oauth

import (
	"context"
	"errors"
	"fmt"
	"slices"

	authscope "github.com/c-premus/documcp/internal/auth/scope"
)

//nolint:godot // ---------------------------------------------------------------------------
// Refresh Token.
//nolint:godot // ---------------------------------------------------------------------------

// RefreshTokenParams holds the input for refreshing tokens.
type RefreshTokenParams struct {
	RefreshToken string
	ClientID     string
	ClientSecret string
	Scope        string
	// Resource is an optional RFC 8707 audience. When non-empty it must equal
	// the resource bound to the original access token; refresh cannot widen
	// or change the audience (RFC 8707 §2.2).
	Resource string
}

// RefreshAccessToken exchanges a refresh token for new tokens (rotation).
func (s *Service) RefreshAccessToken(ctx context.Context, params RefreshTokenParams) (*TokenResult, error) {
	// Parse the refresh token
	_, tokenHash, err := ParseToken(params.RefreshToken)
	if err != nil {
		return nil, errors.New("refresh token not found")
	}

	// Look up the client
	client, err := s.repo.FindClientByClientID(ctx, params.ClientID)
	if err != nil {
		return nil, errors.New("invalid client credentials")
	}

	// Verify client supports refresh_token grant type
	grantTypes, err := client.ParseGrantTypes()
	if err != nil || !slices.Contains(grantTypes, "refresh_token") {
		return nil, ErrUnsupportedGrant
	}

	// Verify client secret
	err = s.verifyClientAuth(client, params.ClientSecret)
	if err != nil {
		return nil, err
	}

	// Look up the refresh token
	refreshToken, err := s.repo.FindRefreshTokenByToken(ctx, tokenHash)
	if err != nil {
		return nil, errors.New("refresh token not found")
	}

	// Verify the refresh token's access token belongs to this client
	accessToken, err := s.repo.FindAccessTokenByID(ctx, refreshToken.AccessTokenID)
	if err != nil {
		return nil, errors.New("associated access token not found")
	}
	if accessToken.ClientID != client.ID {
		return nil, errors.New("refresh token does not belong to this client")
	}

	// Atomically revoke old tokens (rotation) to prevent TOCTOU race conditions.
	if err := s.repo.RevokeTokenPair(ctx, accessToken.ID, refreshToken.ID); err != nil {
		return nil, fmt.Errorf("revoking old token pair: %w", err)
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
			return nil, errors.New("requested scope exceeds original grant")
		}
		scope = params.Scope
	}

	// Resource is inherited from the original access token; client cannot
	// widen or change the audience on refresh.
	resource := ""
	if accessToken.Resource.Valid {
		resource = accessToken.Resource.String
	}
	if params.Resource != "" && params.Resource != resource {
		return nil, errors.New("invalid resource: does not match original token audience")
	}

	return s.issueTokenPair(ctx, client.ID, accessToken.UserID, scope, resource)
}
