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
	client, err := s.clients.FindClientByClientID(ctx, params.ClientID)
	if err != nil {
		return errors.New("invalid client credentials")
	}
	err = s.verifyClientAuth(client, params.ClientSecret)
	if err != nil {
		return err
	}

	// Parse the token. Per RFC 7009, return success even for invalid tokens.
	_, tokenHash, ok := s.parseTokenOrZero(params.Token)
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
	token, err := s.accessTokens.FindAccessTokenByToken(ctx, tokenHash)
	if err != nil {
		return
	}
	// Verify token belongs to the requesting client
	if token.ClientID != clientID {
		return
	}
	if err = s.accessTokens.RevokeAccessToken(ctx, token.ID); err != nil {
		s.logger.Warn("failed to revoke access token during revocation",
			"access_token_id", token.ID, "error", err)
	}
	// Also revoke associated refresh tokens
	if err = s.refreshTokens.RevokeRefreshTokenByAccessTokenID(ctx, token.ID); err != nil {
		s.logger.Warn("failed to revoke associated refresh tokens",
			"access_token_id", token.ID, "error", err)
	}
}

func (s *Service) tryRevokeRefreshToken(ctx context.Context, tokenHash string, clientID int64) {
	token, err := s.refreshTokens.FindRefreshTokenByToken(ctx, tokenHash)
	if err != nil {
		return
	}
	// Verify the refresh token's access token belongs to the requesting client
	accessToken, err := s.accessTokens.FindAccessTokenByID(ctx, token.AccessTokenID)
	if err != nil || accessToken.ClientID != clientID {
		return
	}
	if err = s.refreshTokens.RevokeRefreshToken(ctx, token.ID); err != nil {
		s.logger.Warn("failed to revoke refresh token during revocation",
			"refresh_token_id", token.ID, "error", err)
	}
	// Also revoke the associated access token
	if err = s.accessTokens.RevokeAccessToken(ctx, token.AccessTokenID); err != nil {
		s.logger.Warn("failed to revoke associated access token",
			"access_token_id", token.AccessTokenID, "error", err)
	}
}
