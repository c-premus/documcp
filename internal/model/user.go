package model

import "database/sql"

// User represents a row in the "users" table.
type User struct {
	ID              int64          `db:"id" json:"id"`
	Name            string         `db:"name" json:"name"`
	Email           string         `db:"email" json:"email"`
	OIDCSub         sql.NullString `db:"oidc_sub" json:"oidc_sub"`
	OIDCProvider    sql.NullString `db:"oidc_provider" json:"oidc_provider"`
	EmailVerifiedAt sql.NullTime   `db:"email_verified_at" json:"email_verified_at"`
	IsAdmin         bool           `db:"is_admin" json:"is_admin"`
	Password        sql.NullString `db:"password" json:"-"`
	RememberToken   sql.NullString `db:"remember_token" json:"-"`
	CreatedAt       sql.NullTime   `db:"created_at" json:"created_at"`
	UpdatedAt       sql.NullTime   `db:"updated_at" json:"updated_at"`
}
