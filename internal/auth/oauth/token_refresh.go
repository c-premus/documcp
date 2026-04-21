package oauth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	authscope "github.com/c-premus/documcp/internal/auth/scope"
	"github.com/c-premus/documcp/internal/model"
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
	// Parse the refresh token. Candidate hashes cover every configured HMAC
	// key so tokens minted before a rotation still verify.
	_, tokenHashes, err := s.parseTokenCandidateHashes(params.RefreshToken)
	if err != nil {
		return nil, errors.New("refresh token not found")
	}

	// Look up the client
	client, err := s.clients.FindClientByClientID(ctx, params.ClientID)
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

	// Look up the refresh token (non-revoked, non-expired). On primary
	// miss for every candidate hash, dispatch the reuse detector: a
	// revoked-row hit means the refresh token was rotated away and is now
	// being replayed — evidence of token theft (security.md M2 / OAuth 2.1
	// §4.3.2). We revoke every descendant in the grant's lineage via the
	// shared authorization_code_id.
	var (
		refreshToken *model.OAuthRefreshToken
		findErr      error
	)
	for _, tokenHash := range tokenHashes {
		refreshToken, findErr = s.refreshTokens.FindRefreshTokenByToken(ctx, tokenHash)
		if findErr == nil {
			break
		}
		if !errors.Is(findErr, sql.ErrNoRows) {
			return nil, errors.New("refresh token not found")
		}
	}
	if refreshToken == nil {
		for _, tokenHash := range tokenHashes {
			s.handleRefreshTokenReusePossible(ctx, tokenHash, params.ClientID)
		}
		return nil, errors.New("refresh token not found")
	}

	// Verify the refresh token's access token belongs to this client
	accessToken, err := s.accessTokens.FindAccessTokenByID(ctx, refreshToken.AccessTokenID)
	if err != nil {
		return nil, errors.New("associated access token not found")
	}
	if accessToken.ClientID != client.ID {
		return nil, errors.New("refresh token does not belong to this client")
	}

	// Atomically revoke old tokens (rotation) to prevent TOCTOU race conditions.
	if err := s.accessTokens.RevokeTokenPair(ctx, accessToken.ID, refreshToken.ID); err != nil {
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

	// Propagate the original authorization_code_id through rotation so that
	// a later replay detection can revoke the entire grant's lineage in
	// one query (security.md M2).
	authCodeID := int64(0)
	if accessToken.AuthorizationCodeID.Valid {
		authCodeID = accessToken.AuthorizationCodeID.Int64
	}

	return s.issueTokenPair(ctx, client.ID, accessToken.UserID, scope, resource, authCodeID)
}

// handleRefreshTokenReusePossible detects refresh-token reuse: the primary
// lookup missed, but the token may still exist with revoked=true. If so,
// we revoke every access/refresh pair that descends from the same
// authorization code. Non-fatal on error — the outer handler still
// returns "refresh token not found" to the caller regardless. Security.md
// M2.
func (s *Service) handleRefreshTokenReusePossible(ctx context.Context, tokenHash, clientID string) {
	replayed, err := s.refreshTokens.FindRefreshTokenByTokenIgnoringRevocation(ctx, tokenHash)
	if err != nil || !replayed.Revoked {
		return
	}

	// Walk the lineage: revoked refresh → access token → original auth code.
	access, err := s.accessTokens.FindAccessTokenByID(ctx, replayed.AccessTokenID)
	if err != nil || !access.AuthorizationCodeID.Valid {
		s.logger.Warn("oauth refresh-token reuse detected but lineage unknown",
			"refresh_token_id", replayed.ID, "client_id", clientID)
		tokenReplayTotal.WithLabelValues("refresh").Inc()
		return
	}

	revoked, famErr := s.accessTokens.RevokeTokenFamilyByAuthorizationCodeID(ctx, access.AuthorizationCodeID.Int64)
	if famErr != nil {
		s.logger.Error("revoking token family on refresh reuse",
			"error", famErr, "auth_code_id", access.AuthorizationCodeID.Int64, "client_id", clientID)
		return
	}
	s.logger.Warn("oauth refresh-token reuse detected",
		"auth_code_id", access.AuthorizationCodeID.Int64,
		"client_id", clientID, "tokens_revoked", revoked)
	tokenReplayTotal.WithLabelValues("refresh").Inc()
}
