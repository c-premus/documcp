package oauthhandler

import (
	"net/http"
	"strings"
)

// AuthorizationServerMetadata handles GET /.well-known/oauth-authorization-server (RFC 8414).
func (h *Handler) AuthorizationServerMetadata(w http.ResponseWriter, r *http.Request) {
	issuer := h.appURL

	metadata := map[string]any{
		"issuer":                        issuer,
		"authorization_endpoint":        issuer + "/oauth/authorize",
		"token_endpoint":                issuer + "/oauth/token",
		"revocation_endpoint":           issuer + "/oauth/revoke",
		"registration_endpoint":         issuer + "/oauth/register",
		"device_authorization_endpoint": issuer + "/oauth/device/code",
		"response_types_supported":      []string{"code"},
		"grant_types_supported": []string{
			"authorization_code",
			"refresh_token",
			"urn:ietf:params:oauth:grant-type:device_code",
		},
		"token_endpoint_auth_methods_supported": []string{
			"none",
			"client_secret_basic",
			"client_secret_post",
		},
		"code_challenge_methods_supported": []string{"S256"},
		"scopes_supported":                 []string{"mcp:access", "mcp:read", "mcp:write"},
		"protected_resources":              []string{issuer},
	}

	jsonResponse(w, http.StatusOK, metadata)
}

// ProtectedResourceMetadata handles GET /.well-known/oauth-protected-resource[/{path}] (RFC 9728).
func (h *Handler) ProtectedResourceMetadata(w http.ResponseWriter, r *http.Request) {
	issuer := h.appURL

	// Determine resource path — the URL path after /.well-known/oauth-protected-resource
	resource := issuer
	prefix := "/.well-known/oauth-protected-resource"
	suffix := strings.TrimPrefix(r.URL.Path, prefix)
	if suffix != "" && suffix != "/" {
		resource = issuer + suffix
	}

	metadata := map[string]any{
		"resource":                 resource,
		"authorization_servers":    []string{issuer},
		"scopes_supported":         []string{"mcp:access", "mcp:read", "mcp:write"},
		"bearer_methods_supported": []string{"header"},
	}

	jsonResponse(w, http.StatusOK, metadata)
}
