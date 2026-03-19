package repository

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jmoiron/sqlx"

	"git.999.haus/chris/DocuMCP-go/internal/model"
)

// OAuthRepository handles OAuth-related persistence for clients, tokens, codes, and users.
type OAuthRepository struct {
	db     *sqlx.DB
	logger *slog.Logger
}

// NewOAuthRepository creates a new OAuthRepository.
func NewOAuthRepository(db *sqlx.DB, logger *slog.Logger) *OAuthRepository {
	return &OAuthRepository{db: db, logger: logger}
}

//nolint:godot // ---------------------------------------------------------------------------
// Clients.
//nolint:godot // ---------------------------------------------------------------------------

// CreateClient inserts a new OAuth client and sets the generated ID and timestamps.
func (r *OAuthRepository) CreateClient(ctx context.Context, client *model.OAuthClient) error {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO oauth_clients (
			client_id, client_secret, client_secret_expires_at, client_name,
			software_id, software_version, redirect_uris, grant_types,
			response_types, token_endpoint_auth_method, scope, user_id,
			is_active, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8,
			$9, $10, $11, $12,
			$13, NOW(), NOW()
		) RETURNING id, created_at, updated_at`,
		client.ClientID, client.ClientSecret, client.ClientSecretExpiresAt, client.ClientName,
		client.SoftwareID, client.SoftwareVersion, client.RedirectURIs, client.GrantTypes,
		client.ResponseTypes, client.TokenEndpointAuthMethod, client.Scope, client.UserID,
		client.IsActive,
	).Scan(&client.ID, &client.CreatedAt, &client.UpdatedAt)
	if err != nil {
		return fmt.Errorf("creating oauth client %q: %w", client.ClientName, err)
	}
	return nil
}

// FindClientByClientID returns an active OAuth client by its public client_id.
func (r *OAuthRepository) FindClientByClientID(ctx context.Context, clientID string) (*model.OAuthClient, error) {
	var client model.OAuthClient
	err := r.db.GetContext(ctx, &client,
		`SELECT * FROM oauth_clients WHERE client_id = $1 AND is_active = true`, clientID)
	if err != nil {
		return nil, fmt.Errorf("finding oauth client by client_id %s: %w", clientID, err)
	}
	return &client, nil
}

// FindClientByID returns an OAuth client by its primary key.
func (r *OAuthRepository) FindClientByID(ctx context.Context, id int64) (*model.OAuthClient, error) {
	var client model.OAuthClient
	err := r.db.GetContext(ctx, &client,
		`SELECT * FROM oauth_clients WHERE id = $1`, id)
	if err != nil {
		return nil, fmt.Errorf("finding oauth client by id %d: %w", id, err)
	}
	return &client, nil
}

//nolint:godot // ---------------------------------------------------------------------------
// Authorization Codes.
//nolint:godot // ---------------------------------------------------------------------------

// CreateAuthorizationCode inserts a new authorization code and sets the generated ID and timestamps.
func (r *OAuthRepository) CreateAuthorizationCode(ctx context.Context, code *model.OAuthAuthorizationCode) error {
	err := r.db.QueryRowContext(ctx,
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
	var code model.OAuthAuthorizationCode
	err := r.db.GetContext(ctx, &code,
		`SELECT * FROM oauth_authorization_codes WHERE code = $1 AND revoked = false`, codeHash)
	if err != nil {
		return nil, fmt.Errorf("finding authorization code: %w", err)
	}
	return &code, nil
}

// RevokeAuthorizationCode marks an authorization code as revoked.
func (r *OAuthRepository) RevokeAuthorizationCode(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE oauth_authorization_codes SET revoked = true, updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("revoking authorization code %d: %w", id, err)
	}
	return nil
}

//nolint:godot // ---------------------------------------------------------------------------
// Access Tokens.
//nolint:godot // ---------------------------------------------------------------------------

// CreateAccessToken inserts a new access token and sets the generated ID and timestamps.
func (r *OAuthRepository) CreateAccessToken(ctx context.Context, token *model.OAuthAccessToken) error {
	err := r.db.QueryRowContext(ctx,
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
	var token model.OAuthAccessToken
	err := r.db.GetContext(ctx, &token,
		`SELECT * FROM oauth_access_tokens WHERE id = $1`, id)
	if err != nil {
		return nil, fmt.Errorf("finding access token by id %d: %w", id, err)
	}
	return &token, nil
}

// FindAccessTokenByToken returns a valid (non-revoked, non-expired) access token by its hash.
func (r *OAuthRepository) FindAccessTokenByToken(ctx context.Context, tokenHash string) (*model.OAuthAccessToken, error) {
	var token model.OAuthAccessToken
	err := r.db.GetContext(ctx, &token,
		`SELECT * FROM oauth_access_tokens WHERE token = $1 AND revoked = false AND expires_at > NOW()`, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("finding access token: %w", err)
	}
	return &token, nil
}

// RevokeAccessToken marks an access token as revoked.
func (r *OAuthRepository) RevokeAccessToken(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE oauth_access_tokens SET revoked = true, updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("revoking access token %d: %w", id, err)
	}
	return nil
}

//nolint:godot // ---------------------------------------------------------------------------
// Refresh Tokens.
//nolint:godot // ---------------------------------------------------------------------------

// CreateRefreshToken inserts a new refresh token and sets the generated ID and timestamps.
func (r *OAuthRepository) CreateRefreshToken(ctx context.Context, token *model.OAuthRefreshToken) error {
	err := r.db.QueryRowContext(ctx,
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
	var token model.OAuthRefreshToken
	err := r.db.GetContext(ctx, &token,
		`SELECT * FROM oauth_refresh_tokens WHERE token = $1 AND revoked = false AND expires_at > NOW()`, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("finding refresh token: %w", err)
	}
	return &token, nil
}

// RevokeRefreshToken marks a refresh token as revoked.
func (r *OAuthRepository) RevokeRefreshToken(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE oauth_refresh_tokens SET revoked = true, updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("revoking refresh token %d: %w", id, err)
	}
	return nil
}

// RevokeRefreshTokenByAccessTokenID revokes all refresh tokens associated with an access token.
func (r *OAuthRepository) RevokeRefreshTokenByAccessTokenID(ctx context.Context, accessTokenID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE oauth_refresh_tokens SET revoked = true, updated_at = NOW() WHERE access_token_id = $1`, accessTokenID)
	if err != nil {
		return fmt.Errorf("revoking refresh tokens for access token %d: %w", accessTokenID, err)
	}
	return nil
}

//nolint:godot // ---------------------------------------------------------------------------
// Device Codes.
//nolint:godot // ---------------------------------------------------------------------------

// CreateDeviceCode inserts a new device code and sets the generated ID and timestamps.
func (r *OAuthRepository) CreateDeviceCode(ctx context.Context, dc *model.OAuthDeviceCode) error {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO oauth_device_codes (
			device_code, user_code, client_id, user_id, scope,
			verification_uri, verification_uri_complete, interval,
			last_polled_at, status, expires_at,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8,
			$9, $10, $11,
			NOW(), NOW()
		) RETURNING id, created_at, updated_at`,
		dc.DeviceCode, dc.UserCode, dc.ClientID, dc.UserID, dc.Scope,
		dc.VerificationURI, dc.VerificationURIComplete, dc.Interval,
		dc.LastPolledAt, dc.Status, dc.ExpiresAt,
	).Scan(&dc.ID, &dc.CreatedAt, &dc.UpdatedAt)
	if err != nil {
		return fmt.Errorf("creating device code: %w", err)
	}
	return nil
}

// FindDeviceCodeByDeviceCode returns a device code by its hash.
func (r *OAuthRepository) FindDeviceCodeByDeviceCode(ctx context.Context, deviceCodeHash string) (*model.OAuthDeviceCode, error) {
	var dc model.OAuthDeviceCode
	err := r.db.GetContext(ctx, &dc,
		`SELECT * FROM oauth_device_codes WHERE device_code = $1`, deviceCodeHash)
	if err != nil {
		return nil, fmt.Errorf("finding device code: %w", err)
	}
	return &dc, nil
}

// FindDeviceCodeByUserCode returns a pending, non-expired device code by its user code.
// The comparison normalizes the user code by removing dashes and ignoring case.
func (r *OAuthRepository) FindDeviceCodeByUserCode(ctx context.Context, userCode string) (*model.OAuthDeviceCode, error) {
	var dc model.OAuthDeviceCode
	err := r.db.GetContext(ctx, &dc,
		`SELECT * FROM oauth_device_codes
		WHERE UPPER(REPLACE(user_code, '-', '')) = UPPER(REPLACE($1, '-', ''))
			AND status = 'pending' AND expires_at > NOW()`, userCode)
	if err != nil {
		return nil, fmt.Errorf("finding device code by user code: %w", err)
	}
	return &dc, nil
}

// UpdateDeviceCodeStatus updates the status and optionally the user_id of a device code.
func (r *OAuthRepository) UpdateDeviceCodeStatus(ctx context.Context, id int64, status string, userID *int64) error {
	var err error
	if userID != nil {
		_, err = r.db.ExecContext(ctx,
			`UPDATE oauth_device_codes SET status = $1, user_id = $2, updated_at = NOW() WHERE id = $3`,
			status, *userID, id)
	} else {
		_, err = r.db.ExecContext(ctx,
			`UPDATE oauth_device_codes SET status = $1, updated_at = NOW() WHERE id = $2`,
			status, id)
	}
	if err != nil {
		return fmt.Errorf("updating device code %d status to %s: %w", id, status, err)
	}
	return nil
}

// UpdateDeviceCodeLastPolled updates the last_polled_at timestamp and polling interval.
func (r *OAuthRepository) UpdateDeviceCodeLastPolled(ctx context.Context, id int64, interval int) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE oauth_device_codes SET last_polled_at = NOW(), interval = $1, updated_at = NOW() WHERE id = $2`,
		interval, id)
	if err != nil {
		return fmt.Errorf("updating device code %d last polled: %w", id, err)
	}
	return nil
}

//nolint:godot // ---------------------------------------------------------------------------
// Users.
//nolint:godot // ---------------------------------------------------------------------------

// FindUserByID returns a user by its primary key.
func (r *OAuthRepository) FindUserByID(ctx context.Context, id int64) (*model.User, error) {
	var user model.User
	err := r.db.GetContext(ctx, &user,
		`SELECT * FROM users WHERE id = $1`, id)
	if err != nil {
		return nil, fmt.Errorf("finding user by id %d: %w", id, err)
	}
	return &user, nil
}

// FindUserByEmail returns a user by their email address.
func (r *OAuthRepository) FindUserByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	err := r.db.GetContext(ctx, &user,
		`SELECT * FROM users WHERE email = $1`, email)
	if err != nil {
		return nil, fmt.Errorf("finding user by email %s: %w", email, err)
	}
	return &user, nil
}

// FindUserByOIDCSub returns a user by their OIDC subject identifier.
func (r *OAuthRepository) FindUserByOIDCSub(ctx context.Context, sub string) (*model.User, error) {
	var user model.User
	err := r.db.GetContext(ctx, &user,
		`SELECT * FROM users WHERE oidc_sub = $1`, sub)
	if err != nil {
		return nil, fmt.Errorf("finding user by oidc_sub %s: %w", sub, err)
	}
	return &user, nil
}

// CreateUser inserts a new user and sets the generated ID and timestamps.
func (r *OAuthRepository) CreateUser(ctx context.Context, user *model.User) error {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO users (
			name, email, oidc_sub, oidc_provider, email_verified_at,
			is_admin, password, remember_token,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8,
			NOW(), NOW()
		) RETURNING id, created_at, updated_at`,
		user.Name, user.Email, user.OIDCSub, user.OIDCProvider, user.EmailVerifiedAt,
		user.IsAdmin, user.Password, user.RememberToken,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return fmt.Errorf("creating user %q: %w", user.Email, err)
	}
	return nil
}

// UpdateUser updates a user's profile fields.
func (r *OAuthRepository) UpdateUser(ctx context.Context, user *model.User) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET
			name = $1, email = $2, oidc_sub = $3, oidc_provider = $4, updated_at = NOW()
		WHERE id = $5`,
		user.Name, user.Email, user.OIDCSub, user.OIDCProvider, user.ID)
	if err != nil {
		return fmt.Errorf("updating user %d: %w", user.ID, err)
	}
	return nil
}

// ListUsers returns a paginated list of users with optional search query.
func (r *OAuthRepository) ListUsers(ctx context.Context, query string, limit, offset int) ([]model.User, int, error) {
	where := "1=1"
	args := []any{}
	argIdx := 1

	if query != "" {
		where = fmt.Sprintf("(name ILIKE $%d OR email ILIKE $%d)", argIdx, argIdx+1)
		likeQuery := "%" + query + "%"
		args = append(args, likeQuery, likeQuery)
		argIdx += 2
	}

	countQuery := "SELECT COUNT(*) FROM users WHERE " + where
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting users: %w", err)
	}

	if limit <= 0 {
		limit = 20
	}

	selectQuery := fmt.Sprintf(
		"SELECT * FROM users WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d",
		where, argIdx, argIdx+1,
	)
	args = append(args, limit, offset)

	var users []model.User
	if err := r.db.SelectContext(ctx, &users, selectQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("listing users: %w", err)
	}
	return users, total, nil
}

// ToggleAdmin toggles the is_admin flag for a user.
func (r *OAuthRepository) ToggleAdmin(ctx context.Context, userID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET is_admin = NOT is_admin, updated_at = NOW() WHERE id = $1`, userID)
	if err != nil {
		return fmt.Errorf("toggling admin for user %d: %w", userID, err)
	}
	return nil
}

// DeleteUser hard-deletes a user by ID.
func (r *OAuthRepository) DeleteUser(ctx context.Context, userID int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
	if err != nil {
		return fmt.Errorf("deleting user %d: %w", userID, err)
	}
	return nil
}

// ListClients returns a paginated list of OAuth clients with optional search query.
func (r *OAuthRepository) ListClients(ctx context.Context, query string, limit, offset int) ([]model.OAuthClient, int, error) {
	where := "1=1"
	args := []any{}
	argIdx := 1

	if query != "" {
		where = fmt.Sprintf("(client_name ILIKE $%d OR client_id ILIKE $%d)", argIdx, argIdx+1)
		likeQuery := "%" + query + "%"
		args = append(args, likeQuery, likeQuery)
		argIdx += 2
	}

	countQuery := "SELECT COUNT(*) FROM oauth_clients WHERE " + where
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting oauth clients: %w", err)
	}

	if limit <= 0 {
		limit = 20
	}

	selectQuery := fmt.Sprintf(
		"SELECT * FROM oauth_clients WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d",
		where, argIdx, argIdx+1,
	)
	args = append(args, limit, offset)

	var clients []model.OAuthClient
	if err := r.db.SelectContext(ctx, &clients, selectQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("listing oauth clients: %w", err)
	}
	return clients, total, nil
}

// DeactivateClient marks an OAuth client as inactive.
func (r *OAuthRepository) DeactivateClient(ctx context.Context, clientID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE oauth_clients SET is_active = false, updated_at = NOW() WHERE id = $1`, clientID)
	if err != nil {
		return fmt.Errorf("deactivating oauth client %d: %w", clientID, err)
	}
	return nil
}

// CountUsers returns the total number of users.
func (r *OAuthRepository) CountUsers(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting users: %w", err)
	}
	return count, nil
}

// CountClients returns the total number of OAuth clients.
func (r *OAuthRepository) CountClients(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM oauth_clients`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting oauth clients: %w", err)
	}
	return count, nil
}

// PurgeExpiredTokens deletes expired/revoked tokens older than retentionDays.
// Order: refresh tokens first (FK to access tokens), then access tokens, then auth codes, then device codes.
func (r *OAuthRepository) PurgeExpiredTokens(ctx context.Context, retentionDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("beginning purge expired tokens transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	var totalAffected int64

	// 1. Refresh tokens (FK dependency on access tokens).
	result, err := tx.ExecContext(ctx,
		`DELETE FROM oauth_refresh_tokens
		WHERE (revoked = true OR expires_at < NOW())
			AND created_at < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("purging expired refresh tokens: %w", err)
	}
	n, _ := result.RowsAffected()
	totalAffected += n
	r.logger.Info("purged expired refresh tokens", "count", n)

	// 2. Access tokens.
	result, err = tx.ExecContext(ctx,
		`DELETE FROM oauth_access_tokens
		WHERE (revoked = true OR expires_at < NOW())
			AND created_at < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("purging expired access tokens: %w", err)
	}
	n, _ = result.RowsAffected()
	totalAffected += n
	r.logger.Info("purged expired access tokens", "count", n)

	// 3. Authorization codes.
	result, err = tx.ExecContext(ctx,
		`DELETE FROM oauth_authorization_codes
		WHERE (revoked = true OR expires_at < NOW())
			AND created_at < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("purging expired authorization codes: %w", err)
	}
	n, _ = result.RowsAffected()
	totalAffected += n
	r.logger.Info("purged expired authorization codes", "count", n)

	// 4. Device codes.
	result, err = tx.ExecContext(ctx,
		`DELETE FROM oauth_device_codes
		WHERE (status IN ('expired', 'used') OR expires_at < NOW())
			AND created_at < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("purging expired device codes: %w", err)
	}
	n, _ = result.RowsAffected()
	totalAffected += n
	r.logger.Info("purged expired device codes", "count", n)

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("committing purge expired tokens transaction: %w", err)
	}

	return totalAffected, nil
}
