package model

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// OAuthClient represents a row in the "oauth_clients" table.
type OAuthClient struct {
	ID                      int64          `db:"id" json:"id"`
	ClientID                string         `db:"client_id" json:"client_id"`
	ClientSecret            sql.NullString `db:"client_secret" json:"-"`
	ClientSecretExpiresAt   sql.NullTime   `db:"client_secret_expires_at" json:"client_secret_expires_at"`
	ClientName              string         `db:"client_name" json:"client_name"`
	SoftwareID              sql.NullString `db:"software_id" json:"software_id"`
	SoftwareVersion         sql.NullString `db:"software_version" json:"software_version"`
	RedirectURIs            string         `db:"redirect_uris" json:"redirect_uris"`
	GrantTypes              string         `db:"grant_types" json:"grant_types"`
	ResponseTypes           string         `db:"response_types" json:"response_types"`
	TokenEndpointAuthMethod string         `db:"token_endpoint_auth_method" json:"token_endpoint_auth_method"`
	Scope                   sql.NullString `db:"scope" json:"scope"`
	UserID                  sql.NullInt64  `db:"user_id" json:"user_id"`
	LastUsedAt              sql.NullTime   `db:"last_used_at"  json:"last_used_at"`
	CreatedAt               sql.NullTime   `db:"created_at"    json:"created_at"`
	UpdatedAt               sql.NullTime   `db:"updated_at" json:"updated_at"`
}

// OAuthClientScopeGrant represents a time-bounded scope expansion for an
// OAuth client, created when a user approves consent for scopes beyond the
// client's base registration. One grant per (client, user) pair.
type OAuthClientScopeGrant struct {
	ID        int64        `db:"id"         json:"id"`
	ClientID  int64        `db:"client_id"  json:"client_id"`
	Scope     string       `db:"scope"      json:"scope"`
	GrantedBy int64        `db:"granted_by" json:"granted_by"`
	GrantedAt time.Time    `db:"granted_at" json:"granted_at"`
	ExpiresAt sql.NullTime `db:"expires_at" json:"expires_at"`
	CreatedAt sql.NullTime `db:"created_at" json:"created_at"`
	UpdatedAt sql.NullTime `db:"updated_at" json:"updated_at"`
}

// ParseRedirectURIs decodes the JSON redirect_uris string into a string slice.
func (c *OAuthClient) ParseRedirectURIs() ([]string, error) {
	var uris []string
	if err := json.Unmarshal([]byte(c.RedirectURIs), &uris); err != nil {
		return nil, fmt.Errorf("unmarshaling redirect URIs: %w", err)
	}
	return uris, nil
}

// ParseGrantTypes decodes the JSON grant_types string into a string slice.
func (c *OAuthClient) ParseGrantTypes() ([]string, error) {
	var types []string
	if err := json.Unmarshal([]byte(c.GrantTypes), &types); err != nil {
		return nil, fmt.Errorf("unmarshaling grant types: %w", err)
	}
	return types, nil
}

// ParseResponseTypes decodes the JSON response_types string into a string slice.
func (c *OAuthClient) ParseResponseTypes() ([]string, error) {
	var types []string
	if err := json.Unmarshal([]byte(c.ResponseTypes), &types); err != nil {
		return nil, fmt.Errorf("unmarshaling response types: %w", err)
	}
	return types, nil
}

// OAuthAuthorizationCode represents a row in the "oauth_authorization_codes" table.
type OAuthAuthorizationCode struct {
	ID                  int64          `db:"id" json:"id"`
	Code                string         `db:"code" json:"code"`
	ClientID            int64          `db:"client_id" json:"client_id"`
	UserID              sql.NullInt64  `db:"user_id" json:"user_id"`
	RedirectURI         string         `db:"redirect_uri" json:"redirect_uri"`
	Scope               sql.NullString `db:"scope" json:"scope"`
	CodeChallenge       sql.NullString `db:"code_challenge" json:"code_challenge"`
	CodeChallengeMethod sql.NullString `db:"code_challenge_method" json:"code_challenge_method"`
	Resource            sql.NullString `db:"resource" json:"resource"`
	ExpiresAt           time.Time      `db:"expires_at" json:"expires_at"`
	Revoked             bool           `db:"revoked" json:"revoked"`
	CreatedAt           sql.NullTime   `db:"created_at" json:"created_at"`
	UpdatedAt           sql.NullTime   `db:"updated_at" json:"updated_at"`
}

// OAuthAccessToken represents a row in the "oauth_access_tokens" table.
type OAuthAccessToken struct {
	ID        int64          `db:"id" json:"id"`
	Token     string         `db:"token" json:"token"`
	ClientID  int64          `db:"client_id" json:"client_id"`
	UserID    sql.NullInt64  `db:"user_id" json:"user_id"`
	Scope     sql.NullString `db:"scope" json:"scope"`
	Resource  sql.NullString `db:"resource" json:"resource"`
	ExpiresAt time.Time      `db:"expires_at" json:"expires_at"`
	Revoked   bool           `db:"revoked" json:"revoked"`
	CreatedAt sql.NullTime   `db:"created_at" json:"created_at"`
	UpdatedAt sql.NullTime   `db:"updated_at" json:"updated_at"`
}

// OAuthRefreshToken represents a row in the "oauth_refresh_tokens" table.
type OAuthRefreshToken struct {
	ID            int64        `db:"id" json:"id"`
	Token         string       `db:"token" json:"token"`
	AccessTokenID int64        `db:"access_token_id" json:"access_token_id"`
	ExpiresAt     time.Time    `db:"expires_at" json:"expires_at"`
	Revoked       bool         `db:"revoked" json:"revoked"`
	CreatedAt     sql.NullTime `db:"created_at" json:"created_at"`
	UpdatedAt     sql.NullTime `db:"updated_at" json:"updated_at"`
}

// DeviceCodeStatus represents the authorization state of an OAuth device code.
type DeviceCodeStatus string

// Possible DeviceCodeStatus values.
const (
	DeviceCodeStatusPending    DeviceCodeStatus = "pending"
	DeviceCodeStatusAuthorized DeviceCodeStatus = "authorized"
	DeviceCodeStatusDenied     DeviceCodeStatus = "denied"
	DeviceCodeStatusExchanged  DeviceCodeStatus = "exchanged"
)

// OAuthDeviceCode represents a row in the "oauth_device_codes" table.
type OAuthDeviceCode struct {
	ID                      int64            `db:"id" json:"id"`
	DeviceCode              string           `db:"device_code" json:"device_code"`
	UserCode                string           `db:"user_code" json:"user_code"`
	ClientID                int64            `db:"client_id" json:"client_id"`
	UserID                  sql.NullInt64    `db:"user_id" json:"user_id"`
	Scope                   sql.NullString   `db:"scope" json:"scope"`
	Resource                sql.NullString   `db:"resource" json:"resource"`
	VerificationURI         string           `db:"verification_uri" json:"verification_uri"`
	VerificationURIComplete sql.NullString   `db:"verification_uri_complete" json:"verification_uri_complete"`
	Interval                int              `db:"interval" json:"interval"`
	LastPolledAt            sql.NullTime     `db:"last_polled_at" json:"last_polled_at"`
	Status                  DeviceCodeStatus `db:"status" json:"status"`
	ExpiresAt               time.Time        `db:"expires_at" json:"expires_at"`
	CreatedAt               sql.NullTime     `db:"created_at" json:"created_at"`
	UpdatedAt               sql.NullTime     `db:"updated_at" json:"updated_at"`
}
