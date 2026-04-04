package testutil

import (
	"time"

	"github.com/c-premus/documcp/internal/model"
)

// UserOption configures a User created by NewUser.
type UserOption func(*model.User)

// NewUser returns a User with sensible defaults.
func NewUser(opts ...UserOption) *model.User {
	now := nullTime(time.Now())
	u := &model.User{
		ID:        1,
		Name:      "Test User",
		Email:     "test@example.com",
		IsAdmin:   false,
		CreatedAt: now,
		UpdatedAt: now,
	}
	for _, opt := range opts {
		opt(u)
	}
	return u
}

// WithUserID sets the user ID on the builder.
func WithUserID(id int64) UserOption {
	return func(u *model.User) { u.ID = id }
}

// WithUserName sets the user name on the builder.
func WithUserName(name string) UserOption {
	return func(u *model.User) { u.Name = name }
}

// WithUserEmail sets the user email on the builder.
func WithUserEmail(email string) UserOption {
	return func(u *model.User) { u.Email = email }
}

// WithUserIsAdmin sets the user admin flag on the builder.
func WithUserIsAdmin(admin bool) UserOption {
	return func(u *model.User) { u.IsAdmin = admin }
}

// WithUserOIDCSub sets the user OIDC subject on the builder.
func WithUserOIDCSub(sub string) UserOption {
	return func(u *model.User) { u.OIDCSub = nullString(sub) }
}

// WithUserOIDCProvider sets the user OIDC provider on the builder.
func WithUserOIDCProvider(provider string) UserOption {
	return func(u *model.User) { u.OIDCProvider = nullString(provider) }
}

// WithUserPassword sets the user password on the builder.
func WithUserPassword(pw string) UserOption {
	return func(u *model.User) { u.Password = nullString(pw) }
}
