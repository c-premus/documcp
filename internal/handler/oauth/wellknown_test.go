package oauthhandler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newTestHandler(appURL string) *Handler {
	return &Handler{
		appURL: appURL,
		logger: slog.New(slog.DiscardHandler),
	}
}

func decodeJSON(t *testing.T, body io.Reader) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.NewDecoder(body).Decode(&result); err != nil {
		t.Fatalf("decoding JSON: %v", err)
	}
	return result
}

// ---------------------------------------------------------------------------
// AuthorizationServerMetadata tests
// ---------------------------------------------------------------------------

func TestHandler_AuthorizationServerMetadata(t *testing.T) {
	t.Parallel()

	const appURL = "https://example.com"
	h := newTestHandler(appURL)

	t.Run("returns 200 with correct content type", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", http.NoBody)
		rr := httptest.NewRecorder()

		h.AuthorizationServerMetadata(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
	})

	t.Run("issuer matches app URL", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", http.NoBody)
		rr := httptest.NewRecorder()
		h.AuthorizationServerMetadata(rr, req)

		body := decodeJSON(t, rr.Body)
		if body["issuer"] != appURL {
			t.Errorf("issuer = %v, want %q", body["issuer"], appURL)
		}
	})

	t.Run("authorization endpoint is correct", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", http.NoBody)
		rr := httptest.NewRecorder()
		h.AuthorizationServerMetadata(rr, req)

		body := decodeJSON(t, rr.Body)
		want := appURL + "/oauth/authorize"
		if body["authorization_endpoint"] != want {
			t.Errorf("authorization_endpoint = %v, want %q", body["authorization_endpoint"], want)
		}
	})

	t.Run("token endpoint is correct", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", http.NoBody)
		rr := httptest.NewRecorder()
		h.AuthorizationServerMetadata(rr, req)

		body := decodeJSON(t, rr.Body)
		want := appURL + "/oauth/token"
		if body["token_endpoint"] != want {
			t.Errorf("token_endpoint = %v, want %q", body["token_endpoint"], want)
		}
	})

	t.Run("revocation endpoint is correct", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", http.NoBody)
		rr := httptest.NewRecorder()
		h.AuthorizationServerMetadata(rr, req)

		body := decodeJSON(t, rr.Body)
		want := appURL + "/oauth/revoke"
		if body["revocation_endpoint"] != want {
			t.Errorf("revocation_endpoint = %v, want %q", body["revocation_endpoint"], want)
		}
	})

	t.Run("registration endpoint is correct", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", http.NoBody)
		rr := httptest.NewRecorder()
		h.AuthorizationServerMetadata(rr, req)

		body := decodeJSON(t, rr.Body)
		want := appURL + "/oauth/register"
		if body["registration_endpoint"] != want {
			t.Errorf("registration_endpoint = %v, want %q", body["registration_endpoint"], want)
		}
	})

	t.Run("device authorization endpoint is correct", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", http.NoBody)
		rr := httptest.NewRecorder()
		h.AuthorizationServerMetadata(rr, req)

		body := decodeJSON(t, rr.Body)
		want := appURL + "/oauth/device/code"
		if body["device_authorization_endpoint"] != want {
			t.Errorf("device_authorization_endpoint = %v, want %q", body["device_authorization_endpoint"], want)
		}
	})

	t.Run("response types supported contains code", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", http.NoBody)
		rr := httptest.NewRecorder()
		h.AuthorizationServerMetadata(rr, req)

		body := decodeJSON(t, rr.Body)
		types, ok := body["response_types_supported"].([]any)
		if !ok {
			t.Fatal("response_types_supported is not an array")
		}
		found := false
		for _, rt := range types {
			if rt == "code" {
				found = true
				break
			}
		}
		if !found {
			t.Error("response_types_supported does not contain 'code'")
		}
	})

	t.Run("grant types supported includes required grants", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", http.NoBody)
		rr := httptest.NewRecorder()
		h.AuthorizationServerMetadata(rr, req)

		body := decodeJSON(t, rr.Body)
		grants, ok := body["grant_types_supported"].([]any)
		if !ok {
			t.Fatal("grant_types_supported is not an array")
		}
		required := []string{
			"authorization_code",
			"refresh_token",
			"urn:ietf:params:oauth:grant-type:device_code",
		}
		grantSet := make(map[string]bool)
		for _, g := range grants {
			grantSet[g.(string)] = true
		}
		for _, r := range required {
			if !grantSet[r] {
				t.Errorf("grant_types_supported missing %q", r)
			}
		}
	})

	t.Run("code challenge methods supported contains S256", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", http.NoBody)
		rr := httptest.NewRecorder()
		h.AuthorizationServerMetadata(rr, req)

		body := decodeJSON(t, rr.Body)
		methods, ok := body["code_challenge_methods_supported"].([]any)
		if !ok {
			t.Fatal("code_challenge_methods_supported is not an array")
		}
		if len(methods) != 1 || methods[0] != "S256" {
			t.Errorf("code_challenge_methods_supported = %v, want [S256]", methods)
		}
	})

	t.Run("scopes supported includes mcp scopes", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", http.NoBody)
		rr := httptest.NewRecorder()
		h.AuthorizationServerMetadata(rr, req)

		body := decodeJSON(t, rr.Body)
		scopes, ok := body["scopes_supported"].([]any)
		if !ok {
			t.Fatal("scopes_supported is not an array")
		}
		expected := []string{"mcp:access", "mcp:read", "mcp:write"}
		if len(scopes) != len(expected) {
			t.Fatalf("scopes_supported length = %d, want %d", len(scopes), len(expected))
		}
		for i, want := range expected {
			if scopes[i] != want {
				t.Errorf("scopes_supported[%d] = %v, want %q", i, scopes[i], want)
			}
		}
	})

	t.Run("protected resources includes issuer", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", http.NoBody)
		rr := httptest.NewRecorder()
		h.AuthorizationServerMetadata(rr, req)

		body := decodeJSON(t, rr.Body)
		resources, ok := body["protected_resources"].([]any)
		if !ok {
			t.Fatal("protected_resources is not an array")
		}
		if len(resources) != 1 || resources[0] != appURL {
			t.Errorf("protected_resources = %v, want [%q]", resources, appURL)
		}
	})

	t.Run("token endpoint auth methods supported", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", http.NoBody)
		rr := httptest.NewRecorder()
		h.AuthorizationServerMetadata(rr, req)

		body := decodeJSON(t, rr.Body)
		methods, ok := body["token_endpoint_auth_methods_supported"].([]any)
		if !ok {
			t.Fatal("token_endpoint_auth_methods_supported is not an array")
		}
		expected := []string{"none", "client_secret_basic", "client_secret_post"}
		if len(methods) != len(expected) {
			t.Fatalf("token_endpoint_auth_methods length = %d, want %d", len(methods), len(expected))
		}
		for i, want := range expected {
			if methods[i] != want {
				t.Errorf("token_endpoint_auth_methods_supported[%d] = %v, want %q", i, methods[i], want)
			}
		}
	})

	t.Run("uses different app URL", func(t *testing.T) {
		t.Parallel()

		h2 := newTestHandler("https://other.example.org")
		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", http.NoBody)
		rr := httptest.NewRecorder()
		h2.AuthorizationServerMetadata(rr, req)

		body := decodeJSON(t, rr.Body)
		if body["issuer"] != "https://other.example.org" {
			t.Errorf("issuer = %v, want https://other.example.org", body["issuer"])
		}
		want := "https://other.example.org/oauth/authorize"
		if body["authorization_endpoint"] != want {
			t.Errorf("authorization_endpoint = %v, want %q", body["authorization_endpoint"], want)
		}
	})
}

// ---------------------------------------------------------------------------
// ProtectedResourceMetadata tests
// ---------------------------------------------------------------------------

func TestHandler_ProtectedResourceMetadata(t *testing.T) {
	t.Parallel()

	const appURL = "https://example.com"
	h := newTestHandler(appURL)

	t.Run("returns 200 with correct content type", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", http.NoBody)
		rr := httptest.NewRecorder()

		h.ProtectedResourceMetadata(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
	})

	t.Run("resource defaults to app URL for root path", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", http.NoBody)
		rr := httptest.NewRecorder()
		h.ProtectedResourceMetadata(rr, req)

		body := decodeJSON(t, rr.Body)
		if body["resource"] != appURL {
			t.Errorf("resource = %v, want %q", body["resource"], appURL)
		}
	})

	t.Run("resource defaults to app URL for trailing slash", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource/", http.NoBody)
		rr := httptest.NewRecorder()
		h.ProtectedResourceMetadata(rr, req)

		body := decodeJSON(t, rr.Body)
		if body["resource"] != appURL {
			t.Errorf("resource = %v, want %q", body["resource"], appURL)
		}
	})

	t.Run("resource includes path suffix", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource/api/docs", http.NoBody)
		rr := httptest.NewRecorder()
		h.ProtectedResourceMetadata(rr, req)

		body := decodeJSON(t, rr.Body)
		want := appURL + "/api/docs"
		if body["resource"] != want {
			t.Errorf("resource = %v, want %q", body["resource"], want)
		}
	})

	t.Run("authorization servers includes issuer", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", http.NoBody)
		rr := httptest.NewRecorder()
		h.ProtectedResourceMetadata(rr, req)

		body := decodeJSON(t, rr.Body)
		servers, ok := body["authorization_servers"].([]any)
		if !ok {
			t.Fatal("authorization_servers is not an array")
		}
		if len(servers) != 1 || servers[0] != appURL {
			t.Errorf("authorization_servers = %v, want [%q]", servers, appURL)
		}
	})

	t.Run("scopes supported includes mcp scopes", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", http.NoBody)
		rr := httptest.NewRecorder()
		h.ProtectedResourceMetadata(rr, req)

		body := decodeJSON(t, rr.Body)
		scopes, ok := body["scopes_supported"].([]any)
		if !ok {
			t.Fatal("scopes_supported is not an array")
		}
		expected := []string{"mcp:access", "mcp:read", "mcp:write"}
		if len(scopes) != len(expected) {
			t.Fatalf("scopes_supported length = %d, want %d", len(scopes), len(expected))
		}
		for i, want := range expected {
			if scopes[i] != want {
				t.Errorf("scopes_supported[%d] = %v, want %q", i, scopes[i], want)
			}
		}
	})

	t.Run("bearer methods supported contains header", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", http.NoBody)
		rr := httptest.NewRecorder()
		h.ProtectedResourceMetadata(rr, req)

		body := decodeJSON(t, rr.Body)
		methods, ok := body["bearer_methods_supported"].([]any)
		if !ok {
			t.Fatal("bearer_methods_supported is not an array")
		}
		if len(methods) != 1 || methods[0] != "header" {
			t.Errorf("bearer_methods_supported = %v, want [header]", methods)
		}
	})

	t.Run("uses different app URL", func(t *testing.T) {
		t.Parallel()

		h2 := newTestHandler("https://mcp.example.io")
		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", http.NoBody)
		rr := httptest.NewRecorder()
		h2.ProtectedResourceMetadata(rr, req)

		body := decodeJSON(t, rr.Body)
		if body["resource"] != "https://mcp.example.io" {
			t.Errorf("resource = %v, want https://mcp.example.io", body["resource"])
		}
	})

	t.Run("path suffix with nested path", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource/mcp/v1/tools", http.NoBody)
		rr := httptest.NewRecorder()
		h.ProtectedResourceMetadata(rr, req)

		body := decodeJSON(t, rr.Body)
		want := appURL + "/mcp/v1/tools"
		if body["resource"] != want {
			t.Errorf("resource = %v, want %q", body["resource"], want)
		}
	})
}
