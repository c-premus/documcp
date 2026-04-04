package oauth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	authscope "github.com/c-premus/documcp/internal/auth/scope"
	"github.com/c-premus/documcp/internal/model"
)

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
	// Scope validation: verify all scope tokens are known.
	// The handler narrows scope to user entitlements before calling this method.
	if params.Scope != "" {
		if invalid := authscope.ValidateAll(params.Scope); len(invalid) > 0 {
			return "", fmt.Errorf("invalid scopes: %s", strings.Join(invalid, ", "))
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

// ExchangeAuthorizationCode exchanges an auth code for tokens.
func (s *Service) ExchangeAuthorizationCode(ctx context.Context, params ExchangeAuthorizationCodeParams) (*TokenResult, error) {
	// Parse the authorization code
	codeID, codeHash, err := ParseToken(params.Code)
	if err != nil {
		return nil, errors.New("invalid authorization code")
	}

	// Look up the client
	client, err := s.repo.FindClientByClientID(ctx, params.ClientID)
	if err != nil {
		return nil, errors.New("invalid client credentials")
	}

	// Verify client secret for confidential clients
	err = s.verifyClientAuth(client, params.ClientSecret)
	if err != nil {
		return nil, err
	}

	// Look up the authorization code by hash
	authCode, err := s.repo.FindAuthorizationCodeByCode(ctx, codeHash)
	if err != nil {
		return nil, errors.New("invalid authorization code")
	}

	// Verify code belongs to this client and matches the ID
	if authCode.ClientID != client.ID || authCode.ID != codeID {
		return nil, errors.New("invalid authorization code")
	}

	// Check expiry
	if time.Now().After(authCode.ExpiresAt) {
		return nil, errors.New("authorization code is expired or revoked")
	}

	// Verify redirect URI
	if authCode.RedirectURI != params.RedirectURI {
		return nil, errors.New("redirect URI mismatch")
	}

	// PKCE verification
	if authCode.CodeChallenge.Valid && authCode.CodeChallenge.String != "" {
		if params.CodeVerifier == "" {
			return nil, errors.New("code verifier required for PKCE")
		}
		if !VerifyPKCE(authCode.CodeChallenge.String, params.CodeVerifier) {
			return nil, errors.New("invalid PKCE code verifier")
		}
	} else if params.CodeVerifier != "" {
		// RFC 9700: reject code_verifier when no code_challenge was used
		return nil, errors.New("unexpected code_verifier - authorization request did not include code_challenge")
	}

	// Atomically revoke the authorization code (one-time use).
	// Returns sql.ErrNoRows if a concurrent request already consumed it.
	if err := s.repo.RevokeAuthorizationCode(ctx, authCode.ID); err != nil {
		return nil, errors.New("authorization code has already been used")
	}

	// Issue tokens
	scope := ""
	if authCode.Scope.Valid {
		scope = authCode.Scope.String
	}

	// Defense-in-depth: narrow scope to client's registered scope.
	// The authorize handler already intersects, but enforce here too so a
	// regression in the handler cannot produce over-scoped tokens.
	if scope != "" && client.Scope.Valid && client.Scope.String != "" {
		narrowed := authscope.Intersect(scope, client.Scope.String)
		if narrowed != scope {
			s.logger.Warn("narrowed auth code scope to client registration",
				"original_scope", scope, "effective_scope", narrowed,
				"client_id", params.ClientID,
			)
			scope = narrowed
		}
		if scope == "" {
			return nil, errors.New("none of the granted scopes are allowed for this client")
		}
	}

	return s.issueTokenPair(ctx, client.ID, authCode.UserID, scope)
}
