package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	oauth "github.com/c-premus/documcp/internal/auth/oauth"
	authscope "github.com/c-premus/documcp/internal/auth/scope"
	"github.com/c-premus/documcp/internal/model"
)

// OAuthClientRepo defines the repository methods the OAuth client service needs.
type OAuthClientRepo interface {
	CreateClient(ctx context.Context, client *model.OAuthClient) error
}

// Sentinel errors for OAuth client registration.
var (
	// ErrClientNameRequired indicates the client name was empty.
	ErrClientNameRequired = errors.New("client_name is required")

	// ErrClientNameTooLong indicates the client name exceeds 255 characters.
	ErrClientNameTooLong = errors.New("client_name must not exceed 255 characters")

	// ErrInvalidRedirectURI indicates a redirect URI is malformed.
	ErrInvalidRedirectURI = errors.New("invalid redirect URI")

	// ErrRedirectURINotHTTPS indicates a non-loopback redirect URI does not use HTTPS.
	ErrRedirectURINotHTTPS = errors.New("redirect URIs must use HTTPS for non-loopback hosts")

	// ErrInvalidGrantType indicates an unsupported grant type was requested.
	ErrInvalidGrantType = errors.New("invalid grant type")

	// ErrInvalidAuthMethod indicates an unsupported token_endpoint_auth_method.
	ErrInvalidAuthMethod = errors.New("invalid token_endpoint_auth_method")

	// ErrInvalidScopes indicates one or more scope values are not recognized.
	ErrInvalidScopes = errors.New("invalid scopes")
)

// RegisterClientInput holds the input for registering a new OAuth client.
type RegisterClientInput struct {
	ClientName              string
	RedirectURIs            []string
	GrantTypes              []string
	TokenEndpointAuthMethod string
	Scope                   string
}

// RegisterClientResult holds the created client and the plaintext secret.
type RegisterClientResult struct {
	Client          *model.OAuthClient
	PlaintextSecret string
}

// OAuthClientService handles OAuth client registration business logic.
type OAuthClientService struct {
	repo   OAuthClientRepo
	logger *slog.Logger
}

// NewOAuthClientService creates a new OAuthClientService.
func NewOAuthClientService(repo OAuthClientRepo, logger *slog.Logger) *OAuthClientService {
	return &OAuthClientService{repo: repo, logger: logger}
}

// validGrantTypes is the whitelist of allowed grant types.
var validGrantTypes = map[string]bool{
	"authorization_code": true,
	"refresh_token":      true,
	"urn:ietf:params:oauth:grant-type:device_code": true,
}

// validAuthMethods is the whitelist of allowed token endpoint auth methods.
var validAuthMethods = map[string]bool{
	"none":                 true,
	"client_secret_basic":  true,
	"client_secret_post":   true,
}

// RegisterClient validates input, generates credentials, and persists a new
// OAuth client. It returns the created client and the plaintext secret (which
// is not stored).
func (s *OAuthClientService) RegisterClient(ctx context.Context, input RegisterClientInput) (*RegisterClientResult, error) {
	if err := s.validateInput(input); err != nil {
		return nil, err
	}

	// Generate client credentials.
	clientID := uuid.New().String()
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, fmt.Errorf("generating client secret: %w", err)
	}
	plaintextSecret := hex.EncodeToString(secretBytes)

	hashedSecret, err := bcrypt.GenerateFromPassword([]byte(plaintextSecret), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hashing client secret: %w", err)
	}

	// Apply defaults.
	redirectURIs := input.RedirectURIs
	if redirectURIs == nil {
		redirectURIs = []string{}
	}
	grantTypes := input.GrantTypes
	if grantTypes == nil {
		grantTypes = []string{"authorization_code"}
	}
	responseTypes := []string{"code"}
	authMethod := input.TokenEndpointAuthMethod
	if authMethod == "" {
		authMethod = "client_secret_post"
	}

	redirectURIsJSON, _ := json.Marshal(redirectURIs)
	grantTypesJSON, _ := json.Marshal(grantTypes)
	responseTypesJSON, _ := json.Marshal(responseTypes)

	client := &model.OAuthClient{
		ClientID:                clientID,
		ClientSecret:            sql.NullString{String: string(hashedSecret), Valid: true},
		ClientName:              input.ClientName,
		RedirectURIs:            string(redirectURIsJSON),
		GrantTypes:              string(grantTypesJSON),
		ResponseTypes:           string(responseTypesJSON),
		TokenEndpointAuthMethod: authMethod,
	}
	if input.Scope != "" {
		client.Scope = sql.NullString{String: input.Scope, Valid: true}
	}

	if err := s.repo.CreateClient(ctx, client); err != nil {
		return nil, fmt.Errorf("creating oauth client: %w", err)
	}

	return &RegisterClientResult{
		Client:          client,
		PlaintextSecret: plaintextSecret,
	}, nil
}

// validateInput checks all business rules for client registration input.
func (s *OAuthClientService) validateInput(input RegisterClientInput) error {
	if input.ClientName == "" {
		return ErrClientNameRequired
	}
	if len(input.ClientName) > 255 {
		return ErrClientNameTooLong
	}

	// Validate scopes.
	if input.Scope != "" {
		if invalid := authscope.ValidateAll(input.Scope); len(invalid) > 0 {
			return fmt.Errorf("%w: %s", ErrInvalidScopes, strings.Join(invalid, ", "))
		}
	}

	// Validate redirect URIs.
	for _, uri := range input.RedirectURIs {
		parsed, err := url.ParseRequestURI(uri)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrInvalidRedirectURI, uri)
		}
		if parsed.Scheme != "https" && !oauth.IsLoopbackHost(parsed.Hostname()) {
			return ErrRedirectURINotHTTPS
		}
	}

	// Validate grant types.
	for _, gt := range input.GrantTypes {
		if !validGrantTypes[gt] {
			return fmt.Errorf("%w: %s", ErrInvalidGrantType, gt)
		}
	}

	// Validate auth method.
	if input.TokenEndpointAuthMethod != "" {
		if !validAuthMethods[input.TokenEndpointAuthMethod] {
			return ErrInvalidAuthMethod
		}
	}

	return nil
}
