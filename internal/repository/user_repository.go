package repository

import (
	"context"
	"fmt"

	"github.com/c-premus/documcp/internal/database"
	"github.com/c-premus/documcp/internal/model"
)

//nolint:godot // ---------------------------------------------------------------------------
// Users.
//nolint:godot // ---------------------------------------------------------------------------

// FindUserByID returns a user by its primary key.
func (r *OAuthRepository) FindUserByID(ctx context.Context, id int64) (*model.User, error) {
	user, err := database.Get[model.User](ctx, r.db,
		`SELECT * FROM users WHERE id = $1`, id)
	if err != nil {
		return nil, fmt.Errorf("finding user by id %d: %w", id, err)
	}
	return &user, nil
}

// FindUserByEmail returns a user by their email address.
func (r *OAuthRepository) FindUserByEmail(ctx context.Context, email string) (*model.User, error) {
	user, err := database.Get[model.User](ctx, r.db,
		`SELECT * FROM users WHERE email = $1`, email)
	if err != nil {
		return nil, fmt.Errorf("finding user by email %s: %w", email, err)
	}
	return &user, nil
}

// FindUserByOIDCSub returns a user by their OIDC subject identifier.
func (r *OAuthRepository) FindUserByOIDCSub(ctx context.Context, sub string) (*model.User, error) {
	user, err := database.Get[model.User](ctx, r.db,
		`SELECT * FROM users WHERE oidc_sub = $1`, sub)
	if err != nil {
		return nil, fmt.Errorf("finding user by oidc_sub %s: %w", sub, err)
	}
	return &user, nil
}

// CreateUser inserts a new user and sets the generated ID and timestamps.
func (r *OAuthRepository) CreateUser(ctx context.Context, user *model.User) error {
	err := r.db.QueryRow(ctx,
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
	_, err := r.db.Exec(ctx,
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
		likeQuery := "%" + escapeLike(query) + "%"
		args = append(args, likeQuery, likeQuery)
		argIdx += 2
	}

	if limit <= 0 {
		limit = 20
	}

	// COUNT(*) OVER () collapses the prior COUNT+SELECT pair into one scan.
	selectQuery := fmt.Sprintf(
		"SELECT *, COUNT(*) OVER () AS total FROM users WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d",
		where, argIdx, argIdx+1,
	)
	args = append(args, limit, offset)

	rows, err := database.Select[userListRow](ctx, r.db, selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing users: %w", err)
	}
	users := make([]model.User, len(rows))
	var total int
	for i := range rows {
		users[i] = rows[i].User
		if i == 0 {
			total = int(rows[i].Total)
		}
	}
	return users, total, nil
}

// userListRow extends model.User with the windowed COUNT(*) OVER () total
// so a single scan yields both the page and the true filtered total.
type userListRow struct {
	model.User
	Total int64 `db:"total"`
}

// ToggleAdmin toggles the is_admin flag for a user.
func (r *OAuthRepository) ToggleAdmin(ctx context.Context, userID int64) error {
	_, err := r.db.Exec(ctx,
		`UPDATE users SET is_admin = NOT is_admin, updated_at = NOW() WHERE id = $1`, userID)
	if err != nil {
		return fmt.Errorf("toggling admin for user %d: %w", userID, err)
	}
	return nil
}

// DeleteUser hard-deletes a user by ID.
func (r *OAuthRepository) DeleteUser(ctx context.Context, userID int64) error {
	_, err := r.db.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	if err != nil {
		return fmt.Errorf("deleting user %d: %w", userID, err)
	}
	return nil
}

// CountUsers returns the total number of users.
func (r *OAuthRepository) CountUsers(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting users: %w", err)
	}
	return count, nil
}
