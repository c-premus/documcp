package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"git.999.haus/chris/DocuMCP-go/internal/model"
)

// ---------------------------------------------------------------------------
// Mock OAuth client repository
// ---------------------------------------------------------------------------

type mockOAuthClientRepo struct {
	listClientsFn      func(ctx context.Context, query string, limit, offset int) ([]model.OAuthClient, int, error)
	createClientFn     func(ctx context.Context, client *model.OAuthClient) error
	deactivateClientFn func(ctx context.Context, id int64) error
}

func (m *mockOAuthClientRepo) ListClients(ctx context.Context, query string, limit, offset int) ([]model.OAuthClient, int, error) {
	if m.listClientsFn != nil {
		return m.listClientsFn(ctx, query, limit, offset)
	}
	return nil, 0, nil
}

func (m *mockOAuthClientRepo) CreateClient(ctx context.Context, client *model.OAuthClient) error {
	if m.createClientFn != nil {
		return m.createClientFn(ctx, client)
	}
	return nil
}

func (m *mockOAuthClientRepo) DeactivateClient(ctx context.Context, id int64) error {
	if m.deactivateClientFn != nil {
		return m.deactivateClientFn(ctx, id)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Tests: List
// ---------------------------------------------------------------------------

func TestOAuthClientHandler_List_Success(t *testing.T) {
	t.Parallel()

	repo := &mockOAuthClientRepo{
		listClientsFn: func(_ context.Context, _ string, _, _ int) ([]model.OAuthClient, int, error) {
			return []model.OAuthClient{
				{
					ID:                      1,
					ClientID:                "client-1",
					ClientName:              "Test Client",
					RedirectURIs:            `["https://example.com/callback"]`,
					GrantTypes:              `["authorization_code"]`,
					ResponseTypes:           `["code"]`,
					TokenEndpointAuthMethod: "client_secret_post",
					IsActive:                true,
				},
			}, 1, nil
		},
	}

	h := NewOAuthClientHandler(repo, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/admin/oauth-clients", nil)
	rec := httptest.NewRecorder()

	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding: %v", err)
	}

	data, ok := resp["data"].([]any)
	if !ok || len(data) != 1 {
		t.Errorf("expected 1 client, got %v", resp["data"])
	}

	meta := resp["meta"].(map[string]any)
	if meta["total"].(float64) != 1 {
		t.Errorf("total = %v, want 1", meta["total"])
	}
}

func TestOAuthClientHandler_List_Error(t *testing.T) {
	t.Parallel()

	repo := &mockOAuthClientRepo{
		listClientsFn: func(_ context.Context, _ string, _, _ int) ([]model.OAuthClient, int, error) {
			return nil, 0, fmt.Errorf("db error")
		},
	}

	h := NewOAuthClientHandler(repo, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/admin/oauth-clients", nil)
	rec := httptest.NewRecorder()

	h.List(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// Tests: Create
// ---------------------------------------------------------------------------

func TestOAuthClientHandler_Create_Success(t *testing.T) {
	t.Parallel()

	repo := &mockOAuthClientRepo{
		createClientFn: func(_ context.Context, client *model.OAuthClient) error {
			client.ID = 1
			return nil
		},
	}

	h := NewOAuthClientHandler(repo, discardLogger())

	body := `{"client_name":"My App","redirect_uris":["https://example.com/cb"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/oauth-clients", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding: %v", err)
	}

	data := resp["data"].(map[string]any)
	if data["client_id"] == nil || data["client_id"] == "" {
		t.Error("expected non-empty client_id")
	}
	if data["client_secret"] == nil || data["client_secret"] == "" {
		t.Error("expected non-empty client_secret")
	}
}

func TestOAuthClientHandler_Create_MissingName(t *testing.T) {
	t.Parallel()

	h := NewOAuthClientHandler(&mockOAuthClientRepo{}, discardLogger())

	body := `{"redirect_uris":["https://example.com/cb"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/oauth-clients", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestOAuthClientHandler_Create_InvalidJSON(t *testing.T) {
	t.Parallel()

	h := NewOAuthClientHandler(&mockOAuthClientRepo{}, discardLogger())

	req := httptest.NewRequest(http.MethodPost, "/api/admin/oauth-clients", bytes.NewBufferString("{bad"))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestOAuthClientHandler_Create_RepoError(t *testing.T) {
	t.Parallel()

	repo := &mockOAuthClientRepo{
		createClientFn: func(_ context.Context, _ *model.OAuthClient) error {
			return fmt.Errorf("constraint violation")
		},
	}

	h := NewOAuthClientHandler(repo, discardLogger())

	body := `{"client_name":"Fail App"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/oauth-clients", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// Tests: Revoke
// ---------------------------------------------------------------------------

func TestOAuthClientHandler_Revoke_Success(t *testing.T) {
	t.Parallel()

	h := NewOAuthClientHandler(&mockOAuthClientRepo{}, discardLogger())

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req := httptest.NewRequest(http.MethodPost, "/api/admin/oauth-clients/1/revoke", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	h.Revoke(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestOAuthClientHandler_Revoke_InvalidID(t *testing.T) {
	t.Parallel()

	h := NewOAuthClientHandler(&mockOAuthClientRepo{}, discardLogger())

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "abc")
	req := httptest.NewRequest(http.MethodPost, "/api/admin/oauth-clients/abc/revoke", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	h.Revoke(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestOAuthClientHandler_Revoke_Error(t *testing.T) {
	t.Parallel()

	repo := &mockOAuthClientRepo{
		deactivateClientFn: func(_ context.Context, _ int64) error {
			return fmt.Errorf("db error")
		},
	}

	h := NewOAuthClientHandler(repo, discardLogger())

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req := httptest.NewRequest(http.MethodPost, "/api/admin/oauth-clients/1/revoke", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	h.Revoke(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// Tests: toOAuthClientResponse
// ---------------------------------------------------------------------------

func TestToOAuthClientResponse(t *testing.T) {
	t.Parallel()

	client := &model.OAuthClient{
		ID:                      1,
		ClientID:                "test-client",
		ClientName:              "Test",
		RedirectURIs:            `["https://example.com"]`,
		GrantTypes:              `["authorization_code"]`,
		ResponseTypes:           `["code"]`,
		TokenEndpointAuthMethod: "client_secret_post",
		IsActive:                true,
	}

	resp := toOAuthClientResponse(client)

	if resp.ClientID != "test-client" {
		t.Errorf("ClientID = %q, want %q", resp.ClientID, "test-client")
	}
	if len(resp.RedirectURIs) != 1 || resp.RedirectURIs[0] != "https://example.com" {
		t.Errorf("RedirectURIs = %v, want [https://example.com]", resp.RedirectURIs)
	}
	if !resp.IsActive {
		t.Error("expected IsActive to be true")
	}
}
