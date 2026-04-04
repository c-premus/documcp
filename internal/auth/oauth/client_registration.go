package oauth

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	authscope "github.com/c-premus/documcp/internal/auth/scope"
	"github.com/c-premus/documcp/internal/model"
)

//nolint:godot // ---------------------------------------------------------------------------
// Client Registration (RFC 7591).
//nolint:godot // ---------------------------------------------------------------------------

// RegisterClientParams holds the input for dynamic client registration.
type RegisterClientParams struct {
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	Scope                   string   `json:"scope"`
	SoftwareID              string   `json:"software_id"`
	SoftwareVersion         string   `json:"software_version"`
}

// RegisterClientResult holds the result of client registration.
type RegisterClientResult struct {
	ClientID                string   `json:"client_id"`
	ClientSecret            string   `json:"client_secret,omitempty"`
	ClientIDIssuedAt        int64    `json:"client_id_issued_at"`
	ClientSecretExpiresAt   int64    `json:"client_secret_expires_at,omitempty"`
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	Scope                   string   `json:"scope"`
}

// RegisterClient creates a new OAuth client per RFC 7591.
func (s *Service) RegisterClient(ctx context.Context, params RegisterClientParams) (*RegisterClientResult, error) {
	// Defaults
	if len(params.GrantTypes) == 0 {
		params.GrantTypes = []string{"authorization_code"}
	}
	if len(params.ResponseTypes) == 0 {
		params.ResponseTypes = []string{"code"}
	}
	if params.TokenEndpointAuthMethod == "" {
		params.TokenEndpointAuthMethod = "none"
	}
	if params.Scope == "" {
		params.Scope = authscope.DefaultScopes()
	}
	if invalid := authscope.ValidateAll(params.Scope); len(invalid) > 0 {
		return nil, fmt.Errorf("invalid scopes: %s", strings.Join(invalid, ", "))
	}

	clientID := uuid.New().String()

	redirectURIsJSON, err := json.Marshal(params.RedirectURIs)
	if err != nil {
		return nil, fmt.Errorf("marshaling redirect_uris: %w", err)
	}
	grantTypesJSON, err := json.Marshal(params.GrantTypes)
	if err != nil {
		return nil, fmt.Errorf("marshaling grant_types: %w", err)
	}
	responseTypesJSON, err := json.Marshal(params.ResponseTypes)
	if err != nil {
		return nil, fmt.Errorf("marshaling response_types: %w", err)
	}

	client := &model.OAuthClient{
		ClientID:                clientID,
		ClientName:              params.ClientName,
		RedirectURIs:            string(redirectURIsJSON),
		GrantTypes:              string(grantTypesJSON),
		ResponseTypes:           string(responseTypesJSON),
		TokenEndpointAuthMethod: params.TokenEndpointAuthMethod,
		Scope:                   sql.NullString{String: params.Scope, Valid: params.Scope != ""},
		SoftwareID:              sql.NullString{String: params.SoftwareID, Valid: params.SoftwareID != ""},
		SoftwareVersion:         sql.NullString{String: params.SoftwareVersion, Valid: params.SoftwareVersion != ""},
	}

	var plainSecret string
	isConfidential := params.TokenEndpointAuthMethod != "none"

	if isConfidential {
		plain, hashed, err := GenerateClientSecret()
		if err != nil {
			return nil, fmt.Errorf("generating client secret: %w", err)
		}
		plainSecret = plain
		client.ClientSecret = sql.NullString{String: hashed, Valid: true}
	}

	if err := s.repo.CreateClient(ctx, client); err != nil {
		return nil, fmt.Errorf("creating client: %w", err)
	}

	result := &RegisterClientResult{
		ClientID:                clientID,
		ClientIDIssuedAt:        client.CreatedAt.Time.Unix(),
		ClientName:              params.ClientName,
		RedirectURIs:            params.RedirectURIs,
		GrantTypes:              params.GrantTypes,
		ResponseTypes:           params.ResponseTypes,
		TokenEndpointAuthMethod: params.TokenEndpointAuthMethod,
		Scope:                   params.Scope,
	}

	if isConfidential {
		result.ClientSecret = plainSecret
		result.ClientSecretExpiresAt = 0
	}

	return result, nil
}
