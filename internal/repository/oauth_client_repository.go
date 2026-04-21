package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/c-premus/documcp/internal/database"
	"github.com/c-premus/documcp/internal/model"
)

// OAuthRepository handles OAuth-related persistence for clients, tokens, codes, and users.
type OAuthRepository struct {
	db     *pgxpool.Pool
	logger *slog.Logger
}

// NewOAuthRepository creates a new OAuthRepository.
func NewOAuthRepository(db *pgxpool.Pool, logger *slog.Logger) *OAuthRepository {
	return &OAuthRepository{db: db, logger: logger}
}

//nolint:godot // ---------------------------------------------------------------------------
// Clients.
//nolint:godot // ---------------------------------------------------------------------------

// CreateClient inserts a new OAuth client and sets the generated ID and timestamps.
func (r *OAuthRepository) CreateClient(ctx context.Context, client *model.OAuthClient) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO oauth_clients (
			client_id, client_secret, client_secret_expires_at, client_name,
			software_id, software_version, redirect_uris, grant_types,
			response_types, token_endpoint_auth_method, scope, user_id,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8,
			$9, $10, $11, $12,
			NOW(), NOW()
		) RETURNING id, created_at, updated_at`,
		client.ClientID, client.ClientSecret, client.ClientSecretExpiresAt, client.ClientName,
		client.SoftwareID, client.SoftwareVersion, client.RedirectURIs, client.GrantTypes,
		client.ResponseTypes, client.TokenEndpointAuthMethod, client.Scope, client.UserID,
	).Scan(&client.ID, &client.CreatedAt, &client.UpdatedAt)
	if err != nil {
		return fmt.Errorf("creating oauth client %q: %w", client.ClientName, err)
	}
	return nil
}

// FindClientByClientID returns an OAuth client by its public client_id.
func (r *OAuthRepository) FindClientByClientID(ctx context.Context, clientID string) (*model.OAuthClient, error) {
	client, err := database.Get[model.OAuthClient](ctx, r.db,
		`SELECT * FROM oauth_clients WHERE client_id = $1`, clientID)
	if err != nil {
		return nil, fmt.Errorf("finding oauth client by client_id %s: %w", clientID, err)
	}
	return &client, nil
}

// FindClientByID returns an OAuth client by its primary key.
func (r *OAuthRepository) FindClientByID(ctx context.Context, id int64) (*model.OAuthClient, error) {
	client, err := database.Get[model.OAuthClient](ctx, r.db,
		`SELECT * FROM oauth_clients WHERE id = $1`, id)
	if err != nil {
		return nil, fmt.Errorf("finding oauth client by id %d: %w", id, err)
	}
	return &client, nil
}

// ListClients returns a paginated list of OAuth clients with optional search query.
func (r *OAuthRepository) ListClients(ctx context.Context, query string, limit, offset int) ([]model.OAuthClient, int, error) {
	where := "1=1"
	args := []any{}
	argIdx := 1

	if query != "" {
		where = fmt.Sprintf("(client_name ILIKE $%d OR client_id ILIKE $%d)", argIdx, argIdx+1)
		likeQuery := "%" + escapeLike(query) + "%"
		args = append(args, likeQuery, likeQuery)
		argIdx += 2
	}

	if limit <= 0 {
		limit = 20
	}

	// COUNT(*) OVER () rolls the filtered total into the same scan as the
	// page; single round trip replaces the prior COUNT+SELECT pair.
	selectQuery := fmt.Sprintf(
		"SELECT *, COUNT(*) OVER () AS total FROM oauth_clients WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d",
		where, argIdx, argIdx+1,
	)
	args = append(args, limit, offset)

	rows, err := database.Select[oauthClientListRow](ctx, r.db, selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing oauth clients: %w", err)
	}
	clients := make([]model.OAuthClient, len(rows))
	var total int
	for i := range rows {
		clients[i] = rows[i].OAuthClient
		if i == 0 {
			total = int(rows[i].Total)
		}
	}
	return clients, total, nil
}

// oauthClientListRow extends model.OAuthClient with the windowed
// COUNT(*) OVER () total so a single scan yields both the page and the
// true filtered total.
type oauthClientListRow struct {
	model.OAuthClient
	Total int64 `db:"total"`
}

// DeleteClient permanently removes an OAuth client and all associated tokens,
// authorization codes, and device codes via database CASCADE.
func (r *OAuthRepository) DeleteClient(ctx context.Context, clientID int64) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM oauth_clients WHERE id = $1`, clientID)
	if err != nil {
		return fmt.Errorf("deleting oauth client %d: %w", clientID, err)
	}
	if tag.RowsAffected() == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// TouchClientLastUsed updates last_used_at to NOW() for the given client.
func (r *OAuthRepository) TouchClientLastUsed(ctx context.Context, clientID int64) error {
	_, err := r.db.Exec(ctx,
		`UPDATE oauth_clients SET last_used_at = NOW() WHERE id = $1`, clientID)
	if err != nil {
		return fmt.Errorf("touching last_used_at for oauth client %d: %w", clientID, err)
	}
	return nil
}

// UpdateClientScope replaces the scope column for the given client.
func (r *OAuthRepository) UpdateClientScope(ctx context.Context, clientID int64, scope string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE oauth_clients SET scope = $1, updated_at = NOW() WHERE id = $2`, scope, clientID)
	if err != nil {
		return fmt.Errorf("updating scope for oauth client %d: %w", clientID, err)
	}
	return nil
}

// CountClients returns the total number of OAuth clients.
func (r *OAuthRepository) CountClients(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM oauth_clients`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting oauth clients: %w", err)
	}
	return count, nil
}

//nolint:godot // ---------------------------------------------------------------------------
// Scope Grants.
//nolint:godot // ---------------------------------------------------------------------------

// UpsertScopeGrant creates or updates a scope grant for a client-user pair.
// On conflict (same client + user), the scope and TTL are refreshed.
func (r *OAuthRepository) UpsertScopeGrant(ctx context.Context, grant *model.OAuthClientScopeGrant) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO oauth_client_scope_grants (client_id, scope, granted_by, expires_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (client_id, granted_by) DO UPDATE
			SET scope = $2, expires_at = $4, updated_at = NOW()
		RETURNING id, granted_at, created_at, updated_at`,
		grant.ClientID, grant.Scope, grant.GrantedBy, grant.ExpiresAt,
	).Scan(&grant.ID, &grant.GrantedAt, &grant.CreatedAt, &grant.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upserting scope grant for client %d: %w", grant.ClientID, err)
	}
	return nil
}

// FindActiveScopeGrants returns non-expired grants for a client.
func (r *OAuthRepository) FindActiveScopeGrants(ctx context.Context, clientID int64) ([]model.OAuthClientScopeGrant, error) {
	grants, err := database.Select[model.OAuthClientScopeGrant](ctx, r.db,
		`SELECT id, client_id, scope, granted_by, granted_at, expires_at, created_at, updated_at
		FROM oauth_client_scope_grants
		WHERE client_id = $1 AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY granted_at`, clientID)
	if err != nil {
		return nil, fmt.Errorf("finding active scope grants for client %d: %w", clientID, err)
	}
	return grants, nil
}

// ScopeGrantWithUser carries a scope grant alongside the granter's identity
// fields. Used by the admin scope-grants listing so the UI can render a real
// email instead of "User #N". Email and Name are nullable to cover the case
// where the granter's user row was deleted after the grant was recorded.
type ScopeGrantWithUser struct {
	model.OAuthClientScopeGrant
	GrantedByEmail sql.NullString `db:"granted_by_email"`
	GrantedByName  sql.NullString `db:"granted_by_name"`
}

// FindActiveScopeGrantsWithUsers returns non-expired grants for a client with
// the granter's email and name joined in. LEFT JOIN so a grant whose granter
// was deleted still surfaces (with NULL user fields).
func (r *OAuthRepository) FindActiveScopeGrantsWithUsers(ctx context.Context, clientID int64) ([]ScopeGrantWithUser, error) {
	rows, err := database.Select[ScopeGrantWithUser](ctx, r.db,
		`SELECT g.id, g.client_id, g.scope, g.granted_by, g.granted_at, g.expires_at, g.created_at, g.updated_at,
			u.email AS granted_by_email, u.name AS granted_by_name
		FROM oauth_client_scope_grants g
		LEFT JOIN users u ON u.id = g.granted_by
		WHERE g.client_id = $1 AND (g.expires_at IS NULL OR g.expires_at > NOW())
		ORDER BY g.granted_at`, clientID)
	if err != nil {
		return nil, fmt.Errorf("finding active scope grants with users for client %d: %w", clientID, err)
	}
	return rows, nil
}

// DeleteScopeGrant removes a scope grant by ID scoped to the given client.
// Returns sql.ErrNoRows if no grant with that ID belongs to the client — this
// scoping prevents a DELETE on /oauth-clients/{a}/scope-grants/{b} from
// silently deleting a grant that belongs to a different client (audit-log
// integrity) even though the route is admin-only.
func (r *OAuthRepository) DeleteScopeGrant(ctx context.Context, id, clientID int64) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM oauth_client_scope_grants WHERE id = $1 AND client_id = $2`,
		id, clientID)
	if err != nil {
		return fmt.Errorf("deleting scope grant %d for client %d: %w", id, clientID, err)
	}
	if tag.RowsAffected() == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// DeleteExpiredScopeGrants removes all grants past their expiry.
func (r *OAuthRepository) DeleteExpiredScopeGrants(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM oauth_client_scope_grants WHERE expires_at IS NOT NULL AND expires_at < NOW()`)
	if err != nil {
		return 0, fmt.Errorf("deleting expired scope grants: %w", err)
	}
	return tag.RowsAffected(), nil
}
