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
			code_challenge, code_challenge_method, resource, expires_at, revoked,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			NOW(), NOW()
		) RETURNING id, created_at, updated_at`,
		code.Code, code.ClientID, code.UserID, code.RedirectURI, code.Scope,
		code.CodeChallenge, code.CodeChallengeMethod, code.Resource, code.ExpiresAt, code.Revoked,
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

// FindAuthorizationCodeByCodeIncludingRevoked returns an authorization code
// by hash without filtering out revoked rows. Callers must inspect
// code.Revoked. Used for replay detection in ExchangeAuthorizationCode
// (security.md M1): if the primary lookup fails with ErrNoRows but this
// method returns a revoked row, the caller is presenting a previously-
// consumed code, which is evidence of interception.
func (r *OAuthRepository) FindAuthorizationCodeByCodeIncludingRevoked(ctx context.Context, codeHash string) (*model.OAuthAuthorizationCode, error) {
	code, err := database.Get[model.OAuthAuthorizationCode](ctx, r.db,
		`SELECT * FROM oauth_authorization_codes WHERE code = $1`, codeHash)
	if err != nil {
		return nil, fmt.Errorf("finding authorization code (incl revoked): %w", err)
	}
	return &code, nil
}

// RevokeTokenFamilyByAuthorizationCodeID revokes every access and refresh
// token descended from the given authorization code. Used on replay
// detection: when we discover a consumed code or refresh token is being
// presented, we assume the lineage is compromised and invalidate every
// in-flight token issued under that grant.
//
// Returns the number of access tokens marked revoked (refresh tokens are
// counted separately inside the transaction but not returned — access-token
// count is the interesting signal for alerting).
func (r *OAuthRepository) RevokeTokenFamilyByAuthorizationCodeID(ctx context.Context, authCodeID int64) (int64, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("beginning token family revocation tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op; error is irrelevant

	// Revoke refresh tokens whose access token is in this family first —
	// FK would cascade on delete but we only mark revoked, so we must do
	// it explicitly. Use a correlated subquery to scope by family.
	if _, execErr := tx.Exec(ctx,
		`UPDATE oauth_refresh_tokens
		   SET revoked = true, updated_at = NOW()
		 WHERE revoked = false
		   AND access_token_id IN (
		       SELECT id FROM oauth_access_tokens WHERE authorization_code_id = $1
		   )`, authCodeID); execErr != nil {
		return 0, fmt.Errorf("revoking refresh tokens in family: %w", execErr)
	}

	tag, err := tx.Exec(ctx,
		`UPDATE oauth_access_tokens
		   SET revoked = true, updated_at = NOW()
		 WHERE revoked = false AND authorization_code_id = $1`, authCodeID)
	if err != nil {
		return 0, fmt.Errorf("revoking access tokens in family: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("committing token family revocation: %w", err)
	}

	return tag.RowsAffected(), nil
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
			token, client_id, user_id, scope, resource, authorization_code_id,
			expires_at, revoked,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8,
			NOW(), NOW()
		) RETURNING id, created_at, updated_at`,
		token.Token, token.ClientID, token.UserID, token.Scope, token.Resource, token.AuthorizationCodeID,
		token.ExpiresAt, token.Revoked,
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

// FindRefreshTokenByTokenIgnoringRevocation returns a refresh token by hash
// without filtering by revoked or expiry. Callers must inspect the returned
// token's flags. Used for replay detection in RefreshAccessToken
// (security.md M2): when the primary lookup fails, a revoked hit here is
// evidence the refresh token was intercepted after successful rotation.
func (r *OAuthRepository) FindRefreshTokenByTokenIgnoringRevocation(ctx context.Context, tokenHash string) (*model.OAuthRefreshToken, error) {
	token, err := database.Get[model.OAuthRefreshToken](ctx, r.db,
		`SELECT * FROM oauth_refresh_tokens WHERE token = $1`, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("finding refresh token (incl revoked): %w", err)
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

// RevokeUserTokensSince marks every live access token minted for userID at or
// after since as revoked, and cascades to their refresh tokens. Used by session
// logout to invalidate OAuth grants minted during the session being terminated
// (L7). Returns the number of access tokens revoked so callers can log it.
//
// since is typically the session's login_at timestamp. Tokens older than the
// session are intentionally left alone — they belong to earlier grants that
// the user may still want to keep.
func (r *OAuthRepository) RevokeUserTokensSince(ctx context.Context, userID int64, since time.Time) (int64, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("beginning revoke-since tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op; error is irrelevant

	if _, execErr := tx.Exec(ctx,
		`UPDATE oauth_refresh_tokens
		   SET revoked = true, updated_at = NOW()
		 WHERE revoked = false
		   AND access_token_id IN (
		       SELECT id FROM oauth_access_tokens
		        WHERE user_id = $1 AND created_at >= $2
		   )`, userID, since); execErr != nil {
		return 0, fmt.Errorf("revoking refresh tokens since %s: %w", since.Format(time.RFC3339), execErr)
	}

	tag, err := tx.Exec(ctx,
		`UPDATE oauth_access_tokens
		   SET revoked = true, updated_at = NOW()
		 WHERE revoked = false
		   AND user_id = $1
		   AND created_at >= $2`, userID, since)
	if err != nil {
		return 0, fmt.Errorf("revoking access tokens since %s: %w", since.Format(time.RFC3339), err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("committing revoke-since: %w", err)
	}

	return tag.RowsAffected(), nil
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
