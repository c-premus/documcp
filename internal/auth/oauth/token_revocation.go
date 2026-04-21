package oauth

import (
	"context"
	"errors"
)

//nolint:godot // ---------------------------------------------------------------------------
// Token Revocation (RFC 7009).
//nolint:godot // ---------------------------------------------------------------------------

// ErrInvalidClientCredentials signals that `/oauth/revoke` rejected the
// caller's client authentication — the handler maps this to a 401
// `invalid_client` response per RFC 6749 §5.2. Any other error returned
// from RevokeToken is swallowed in the handler (200 OK per RFC 7009 §2.2).
var ErrInvalidClientCredentials = errors.New("invalid client credentials")

// RevokeTokenParams holds the input for token revocation.
type RevokeTokenParams struct {
	Token         string
	ClientID      string
	ClientSecret  string
	TokenTypeHint string
}

// RevokeToken revokes an access or refresh token. Per RFC 7009 §2.2 the
// endpoint responds 200 even when the token is unknown; the only failure
// worth surfacing to the caller is a client-authentication failure
// (ErrInvalidClientCredentials), which the handler converts to 401
// invalid_client.
func (s *Service) RevokeToken(ctx context.Context, params RevokeTokenParams) error {
	// Verify client
	client, err := s.clients.FindClientByClientID(ctx, params.ClientID)
	if err != nil {
		return ErrInvalidClientCredentials
	}
	if err := s.verifyClientAuth(client, params.ClientSecret); err != nil {
		return ErrInvalidClientCredentials
	}

	// Parse the token. Per RFC 7009, return success even for invalid tokens.
	// Candidate hashes cover every configured HMAC key so rotation doesn't
	// silently skip revocation for tokens minted under a retired key.
	_, tokenHashes, ok := s.parseTokenCandidateHashesOrZero(params.Token)
	if !ok {
		return nil
	}

	for _, tokenHash := range tokenHashes {
		// Try to revoke based on hint (verify client ownership per RFC 7009 §2.1)
		switch params.TokenTypeHint {
		case "refresh_token":
			s.tryRevokeRefreshToken(ctx, tokenHash, client.ID)
		case "access_token":
			s.tryRevokeAccessToken(ctx, tokenHash, client.ID)
		default:
			s.tryRevokeAccessToken(ctx, tokenHash, client.ID)
			s.tryRevokeRefreshToken(ctx, tokenHash, client.ID)
		}
	}

	return nil
}

// parseTokenCandidateHashesOrZero is the boolean-result variant of
// parseTokenCandidateHashes, matching the shape of parseTokenOrZero.
func (s *Service) parseTokenCandidateHashesOrZero(plaintext string) (id int64, hashes []string, ok bool) {
	id, hashes, err := s.parseTokenCandidateHashes(plaintext)
	return id, hashes, err == nil
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
