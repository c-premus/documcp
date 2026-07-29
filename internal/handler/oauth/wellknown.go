package oauthhandler

import (
	"net/http"
	"sort"
	"strings"

	authscope "github.com/c-premus/documcp/internal/auth/scope"
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
		"scopes_supported":                 allScopesSorted(),
		"protected_resources":              []string{issuer},
		"resource_indicators_supported":    true,
		// RFC 9207 §2.3 — required whenever the AS emits the iss parameter in
		// authorization responses, which Authorize{Approve,Deny} now do. MCP
		// clients key their iss validation on this flag; the MCP authorization
		// spec expects the SHOULD to become a MUST in a future revision.
		"authorization_response_iss_parameter_supported": true,
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
		"scopes_supported":         scopesForResource(suffix, h.mcpEndpoint),
		"bearer_methods_supported": []string{"header"},
	}

	jsonResponse(w, http.StatusOK, metadata)
}

// scopesForResource returns the RFC 9728 scopes_supported list for the
// protected resource identified by the path suffix of the metadata request.
//
// The two protected resources have disjoint scope vocabularies, so a single
// combined list would misdirect clients: an MCP client following the spec's
// scope-selection strategy requests everything in scopes_supported when the
// 401 carries no scope challenge, and would otherwise ask for REST-only
// authority it can never exercise over /documcp.
func scopesForResource(suffix, mcpEndpoint string) []string {
	if mcpEndpoint != "" && strings.TrimSuffix(suffix, "/") == strings.TrimSuffix(mcpEndpoint, "/") {
		return authscope.ParseScopes(authscope.MCPResourceScopes())
	}
	return authscope.ParseScopes(authscope.APIResourceScopes())
}

// allScopesSorted returns all registered OAuth scopes in sorted order.
func allScopesSorted() []string {
	scopes := make([]string, 0, len(authscope.All))
	for s := range authscope.All {
		scopes = append(scopes, s)
	}
	sort.Strings(scopes)
	return scopes
}
