package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/c-premus/documcp/internal/database"
	"github.com/c-premus/documcp/internal/model"
)

//nolint:godot // ---------------------------------------------------------------------------
// Authorization Codes.
//nolint:godot // ---------------------------------------------------------------------------

// CreateAuthorizationCode inserts a new authorization code and sets the generated ID and timestamps.
func (r *OAuthRepository) CreateAuthorizationCode(ctx context.Context, code *model.OAuthAuthorizationCode) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO oauth_authorization_codes (
			code, client_id, user_id, redirect_uri, scope,
			code_challenge, code_challenge_method, expires_at, revoked,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			NOW(), NOW()
		) RETURNING id, created_at, updated_at`,
		code.Code, code.ClientID, code.UserID, code.RedirectURI, code.Scope,
		code.CodeChallenge, code.CodeChallengeMethod, code.ExpiresAt, code.Revoked,
	).Scan(&code.ID, &code.CreatedAt, &code.UpdatedAt)
	if err != nil {
		return fmt.Errorf("creating authorization code: %w", err)
	}
	return nil
}

// FindAuthorizationCodeByCode returns a non-revoked authorization code by its hash.
func (r *OAuthRepository) FindAuthorizationCodeByCode(ctx context.Context, codeHash string) (*model.OAuthAuthorizationCode, error) {
	code, err := database.Get[model.OAuthAuthorizationCode](ctx, r.db,
		`SELECT * FROM oauth_authorization_codes WHERE code = $1 AND revoked = false`, codeHash)
	if err != nil {
		return nil, fmt.Errorf("finding authorization code: %w", err)
	}
	return &code, nil
}

// RevokeAuthorizationCode atomically marks an authorization code as revoked.
// Returns sql.ErrNoRows if the code was already revoked (prevents double-exchange).
func (r *OAuthRepository) RevokeAuthorizationCode(ctx context.Context, id int64) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE oauth_authorization_codes SET revoked = true, updated_at = NOW() WHERE id = $1 AND revoked = false`, id)
	if err != nil {
		return fmt.Errorf("revoking authorization code %d: %w", id, err)
	}
	n := tag.RowsAffected()
	if n == 0 {
		return fmt.Errorf("authorization code %d already consumed: %w", id, sql.ErrNoRows)
	}
	return nil
}

//nolint:godot // ---------------------------------------------------------------------------
// Access Tokens.
//nolint:godot // ---------------------------------------------------------------------------

// CreateAccessToken inserts a new access token and sets the generated ID and timestamps.
func (r *OAuthRepository) CreateAccessToken(ctx context.Context, token *model.OAuthAccessToken) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO oauth_access_tokens (
			token, client_id, user_id, scope, expires_at, revoked,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			NOW(), NOW()
		) RETURNING id, created_at, updated_at`,
		token.Token, token.ClientID, token.UserID, token.Scope, token.ExpiresAt, token.Revoked,
	).Scan(&token.ID, &token.CreatedAt, &token.UpdatedAt)
	if err != nil {
		return fmt.Errorf("creating access token: %w", err)
	}
	return nil
}

// FindAccessTokenByID returns an access token by its primary key.
func (r *OAuthRepository) FindAccessTokenByID(ctx context.Context, id int64) (*model.OAuthAccessToken, error) {
	token, err := database.Get[model.OAuthAccessToken](ctx, r.db,
		`SELECT * FROM oauth_access_tokens WHERE id = $1`, id)
	if err != nil {
		return nil, fmt.Errorf("finding access token by id %d: %w", id, err)
	}
	return &token, nil
}

// FindAccessTokenByToken returns a valid (non-revoked, non-expired) access token by its hash.
func (r *OAuthRepository) FindAccessTokenByToken(ctx context.Context, tokenHash string) (*model.OAuthAccessToken, error) {
	token, err := database.Get[model.OAuthAccessToken](ctx, r.db,
		`SELECT * FROM oauth_access_tokens WHERE token = $1 AND revoked = false AND expires_at > NOW()`, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("finding access token: %w", err)
	}
	return &token, nil
}

// RevokeAccessToken marks an access token as revoked.
func (r *OAuthRepository) RevokeAccessToken(ctx context.Context, id int64) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE oauth_access_tokens SET revoked = true, updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("revoking access token %d: %w", id, err)
	}
	n := tag.RowsAffected()
	if n == 0 {
		return fmt.Errorf("access token %d not found: %w", id, sql.ErrNoRows)
	}
	return nil
}

//nolint:godot // ---------------------------------------------------------------------------
// Refresh Tokens.
//nolint:godot // ---------------------------------------------------------------------------

// CreateRefreshToken inserts a new refresh token and sets the generated ID and timestamps.
func (r *OAuthRepository) CreateRefreshToken(ctx context.Context, token *model.OAuthRefreshToken) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO oauth_refresh_tokens (
			token, access_token_id, expires_at, revoked,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4,
			NOW(), NOW()
		) RETURNING id, created_at, updated_at`,
		token.Token, token.AccessTokenID, token.ExpiresAt, token.Revoked,
	).Scan(&token.ID, &token.CreatedAt, &token.UpdatedAt)
	if err != nil {
		return fmt.Errorf("creating refresh token: %w", err)
	}
	return nil
}

// FindRefreshTokenByToken returns a valid (non-revoked, non-expired) refresh token by its hash.
func (r *OAuthRepository) FindRefreshTokenByToken(ctx context.Context, tokenHash string) (*model.OAuthRefreshToken, error) {
	token, err := database.Get[model.OAuthRefreshToken](ctx, r.db,
		`SELECT * FROM oauth_refresh_tokens WHERE token = $1 AND revoked = false AND expires_at > NOW()`, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("finding refresh token: %w", err)
	}
	return &token, nil
}

// RevokeRefreshToken marks a refresh token as revoked.
func (r *OAuthRepository) RevokeRefreshToken(ctx context.Context, id int64) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE oauth_refresh_tokens SET revoked = true, updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("revoking refresh token %d: %w", id, err)
	}
	n := tag.RowsAffected()
	if n == 0 {
		return fmt.Errorf("refresh token %d not found: %w", id, sql.ErrNoRows)
	}
	return nil
}

// RevokeRefreshTokenByAccessTokenID revokes all refresh tokens associated with an access token.
func (r *OAuthRepository) RevokeRefreshTokenByAccessTokenID(ctx context.Context, accessTokenID int64) error {
	_, err := r.db.Exec(ctx,
		`UPDATE oauth_refresh_tokens SET revoked = true, updated_at = NOW() WHERE access_token_id = $1`, accessTokenID)
	if err != nil {
		return fmt.Errorf("revoking refresh tokens for access token %d: %w", accessTokenID, err)
	}
	return nil
}

// RevokeTokenPair atomically revokes an access token and its refresh token in a single transaction.
func (r *OAuthRepository) RevokeTokenPair(ctx context.Context, accessTokenID, refreshTokenID int64) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning token revocation tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op; error is irrelevant

	if _, err := tx.Exec(ctx,
		`UPDATE oauth_access_tokens SET revoked = true, updated_at = NOW() WHERE id = $1`, accessTokenID); err != nil {
		return fmt.Errorf("revoking access token %d: %w", accessTokenID, err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE oauth_refresh_tokens SET revoked = true, updated_at = NOW() WHERE id = $1`, refreshTokenID); err != nil {
		return fmt.Errorf("revoking refresh token %d: %w", refreshTokenID, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing token revocation: %w", err)
	}
	return nil
}

// PurgeExpiredTokens deletes expired/revoked tokens older than retentionDays.
// Order: refresh tokens first (FK to access tokens), then access tokens, then auth codes, then device codes.
func (r *OAuthRepository) PurgeExpiredTokens(ctx context.Context, retentionDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("beginning purge expired tokens transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op; error is irrelevant

	var totalAffected int64

	// 1. Refresh tokens (FK dependency on access tokens).
	tag, err := tx.Exec(ctx,
		`DELETE FROM oauth_refresh_tokens
		WHERE (revoked = true OR expires_at < NOW())
			AND created_at < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("purging expired refresh tokens: %w", err)
	}
	n := tag.RowsAffected()
	totalAffected += n
	r.logger.Info("purged expired refresh tokens", "count", n)

	// 2. Access tokens.
	tag, err = tx.Exec(ctx,
		`DELETE FROM oauth_access_tokens
		WHERE (revoked = true OR expires_at < NOW())
			AND created_at < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("purging expired access tokens: %w", err)
	}
	n = tag.RowsAffected()
	totalAffected += n
	r.logger.Info("purged expired access tokens", "count", n)

	// 3. Authorization codes.
	tag, err = tx.Exec(ctx,
		`DELETE FROM oauth_authorization_codes
		WHERE (revoked = true OR expires_at < NOW())
			AND created_at < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("purging expired authorization codes: %w", err)
	}
	n = tag.RowsAffected()
	totalAffected += n
	r.logger.Info("purged expired authorization codes", "count", n)

	// 4. Device codes.
	tag, err = tx.Exec(ctx,
		`DELETE FROM oauth_device_codes
		WHERE (status IN ('expired', 'used') OR expires_at < NOW())
			AND created_at < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("purging expired device codes: %w", err)
	}
	n = tag.RowsAffected()
	totalAffected += n
	r.logger.Info("purged expired device codes", "count", n)

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("committing purge expired tokens transaction: %w", err)
	}

	return totalAffected, nil
}
