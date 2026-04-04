package testutil

import (
	"time"

	"github.com/c-premus/documcp/internal/model"
)

// OAuthClientOption configures an OAuthClient created by NewOAuthClient.
type OAuthClientOption func(*model.OAuthClient)

// NewOAuthClient returns an OAuthClient with sensible defaults.
func NewOAuthClient(opts ...OAuthClientOption) *model.OAuthClient {
	now := nullTime(time.Now())
	c := &model.OAuthClient{
		ID:                      1,
		ClientID:                "test-client-id",
		ClientName:              "Test Client",
		RedirectURIs:            `["http://localhost:8080/callback"]`,
		GrantTypes:              `["authorization_code"]`,
		ResponseTypes:           `["code"]`,
		TokenEndpointAuthMethod: "client_secret_basic",
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// WithOAuthClientID sets the OAuth client primary key ID on the builder.
func WithOAuthClientID(id int64) OAuthClientOption {
	return func(c *model.OAuthClient) { c.ID = id }
}

// WithOAuthClientClientID sets the OAuth client identifier on the builder.
func WithOAuthClientClientID(clientID string) OAuthClientOption {
	return func(c *model.OAuthClient) { c.ClientID = clientID }
}

// WithOAuthClientName sets the OAuth client name on the builder.
func WithOAuthClientName(name string) OAuthClientOption {
	return func(c *model.OAuthClient) { c.ClientName = name }
}

// WithOAuthClientSecret sets the OAuth client secret on the builder.
func WithOAuthClientSecret(secret string) OAuthClientOption {
	return func(c *model.OAuthClient) { c.ClientSecret = nullString(secret) }
}

// WithOAuthClientRedirectURIs sets the OAuth client redirect URIs on the builder.
func WithOAuthClientRedirectURIs(uris string) OAuthClientOption {
	return func(c *model.OAuthClient) { c.RedirectURIs = uris }
}

// WithOAuthClientGrantTypes sets the OAuth client grant types on the builder.
func WithOAuthClientGrantTypes(types string) OAuthClientOption {
	return func(c *model.OAuthClient) { c.GrantTypes = types }
}

// WithOAuthClientScope sets the OAuth client scope on the builder.
func WithOAuthClientScope(scope string) OAuthClientOption {
	return func(c *model.OAuthClient) { c.Scope = nullString(scope) }
}

// WithOAuthClientUserID sets the OAuth client user ID on the builder.
func WithOAuthClientUserID(uid int64) OAuthClientOption {
	return func(c *model.OAuthClient) { c.UserID = nullInt64(uid) }
}
