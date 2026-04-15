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
	// Resource is the RFC 8707 audience the resulting access token will be
	// bound to. Empty means the client did not supply a `resource` parameter
	// at /oauth/authorize; the issued token will not be usable at any
	// audience-checked resource server.
	Resource string
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
		Resource: sql.NullString{
			String: params.Resource,
			Valid:  params.Resource != "",
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
	// Resource is the optional RFC 8707 audience the client is requesting on
	// the token. When non-empty it must equal the resource captured at
	// /oauth/authorize; widening is forbidden (RFC 8707 §2.2).
	Resource string
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

	// Verify client supports authorization_code grant type
	grantTypes, err := client.ParseGrantTypes()
	if err != nil || !slices.Contains(grantTypes, "authorization_code") {
		return nil, ErrUnsupportedGrant
	}

	// Verify client secret for confidential clients
	err = s.verifyClientAuth(client, params.ClientSecret)
	if err != nil {
		return nil, err
	}

	// Look up the authorization code by hash. A non-revoked hit is the
	// happy path. If the lookup fails with no-rows, dispatch to the replay
	// detector before returning — a revoked match means the code is being
	// replayed (security.md M1 / OAuth 2.1 §4.1.3).
	authCode, err := s.repo.FindAuthorizationCodeByCode(ctx, codeHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.handleAuthCodeReusePossible(ctx, codeHash, codeID, params.ClientID)
		}
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

	// RFC 8707 §2.2: if the client supplies a resource on the token request
	// it must equal the resource captured at /oauth/authorize. Single-resource
	// schema makes "subset" equivalent to "equal or absent".
	resource := ""
	if authCode.Resource.Valid {
		resource = authCode.Resource.String
	}
	if params.Resource != "" {
		if resource == "" || params.Resource != resource {
			return nil, errors.New("invalid resource: does not match authorization request")
		}
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

	// Defense-in-depth: narrow scope to client's effective scope (base + grants).
	// The authorize handler already intersects, but enforce here too so a
	// regression in the handler cannot produce over-scoped tokens.
	if scope != "" {
		baseScope := ""
		if client.Scope.Valid {
			baseScope = client.Scope.String
		}
		clientEffective, effErr := s.EffectiveClientScope(ctx, client.ID, baseScope)
		if effErr != nil {
			s.logger.Error("computing effective client scope for token exchange", "error", effErr)
			clientEffective = baseScope
		}
		if clientEffective != "" {
			narrowed := authscope.Intersect(scope, clientEffective)
			if narrowed != scope {
				s.logger.Warn("narrowed auth code scope to effective client scope",
					"original_scope", scope, "effective_scope", narrowed,
					"client_id", params.ClientID,
				)
				scope = narrowed
			}
			if scope == "" {
				return nil, errors.New("none of the granted scopes are allowed for this client")
			}
		}
	}

	return s.issueTokenPair(ctx, client.ID, authCode.UserID, scope, resource, authCode.ID)
}

// handleAuthCodeReusePossible runs when the primary code lookup misses with
// sql.ErrNoRows. If the same hash exists with revoked=true, the code is
// being replayed — evidence the token lineage is compromised — and every
// access/refresh token descending from that code is revoked. Non-fatal on
// error. Security.md M1.
func (s *Service) handleAuthCodeReusePossible(ctx context.Context, codeHash string, codeID int64, clientID string) {
	replayed, err := s.repo.FindAuthorizationCodeByCodeIncludingRevoked(ctx, codeHash)
	if err != nil || !replayed.Revoked || replayed.ID != codeID {
		return
	}

	revoked, famErr := s.repo.RevokeTokenFamilyByAuthorizationCodeID(ctx, replayed.ID)
	if famErr != nil {
		s.logger.Error("revoking token family on auth-code replay",
			"error", famErr, "auth_code_id", replayed.ID, "client_id", clientID)
		return
	}
	s.logger.Warn("oauth auth-code replay detected",
		"auth_code_id", replayed.ID, "client_id", clientID,
		"tokens_revoked", revoked)
	tokenReplayTotal.WithLabelValues("authcode").Inc()
}
