package oauth

import (
	"context"
	"errors"
)

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
		return errors.New("invalid client credentials")
	}
	err = s.verifyClientAuth(client, params.ClientSecret)
	if err != nil {
		return err
	}

	// Parse the token. Per RFC 7009, return success even for invalid tokens.
	_, tokenHash, ok := parseTokenOrZero(params.Token)
	if !ok {
		return nil
	}

	// Try to revoke based on hint (verify client ownership per RFC 7009 §2.1)
	switch params.TokenTypeHint {
	case "refresh_token":
		s.tryRevokeRefreshToken(ctx, tokenHash, client.ID)
	case "access_token":
		s.tryRevokeAccessToken(ctx, tokenHash, client.ID)
	default:
		// Try access token first, then refresh token
		s.tryRevokeAccessToken(ctx, tokenHash, client.ID)
		s.tryRevokeRefreshToken(ctx, tokenHash, client.ID)
	}

	return nil
}

func (s *Service) tryRevokeAccessToken(ctx context.Context, tokenHash string, clientID int64) {
	token, err := s.repo.FindAccessTokenByToken(ctx, tokenHash)
	if err != nil {
		return
	}
	// Verify token belongs to the requesting client
	if token.ClientID != clientID {
		return
	}
	_ = s.repo.RevokeAccessToken(ctx, token.ID)
	// Also revoke associated refresh tokens
	_ = s.repo.RevokeRefreshTokenByAccessTokenID(ctx, token.ID)
}

func (s *Service) tryRevokeRefreshToken(ctx context.Context, tokenHash string, clientID int64) {
	token, err := s.repo.FindRefreshTokenByToken(ctx, tokenHash)
	if err != nil {
		return
	}
	// Verify the refresh token's access token belongs to the requesting client
	accessToken, err := s.repo.FindAccessTokenByID(ctx, token.AccessTokenID)
	if err != nil || accessToken.ClientID != clientID {
		return
	}
	_ = s.repo.RevokeRefreshToken(ctx, token.ID)
	// Also revoke the associated access token
	_ = s.repo.RevokeAccessToken(ctx, token.AccessTokenID)
}
