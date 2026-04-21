package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/repository"
	"github.com/c-premus/documcp/internal/service"
	"github.com/c-premus/documcp/internal/testutil"
)

// ---------------------------------------------------------------------------
// Mock OAuth client repository
// ---------------------------------------------------------------------------

type mockOAuthClientRepo struct {
	listClientsFn                    func(ctx context.Context, query string, limit, offset int) ([]model.OAuthClient, int, error)
	createClientFn                   func(ctx context.Context, client *model.OAuthClient) error
	findClientByIDFn                 func(ctx context.Context, id int64) (*model.OAuthClient, error)
	deleteClientFn                   func(ctx context.Context, id int64) error
	findActiveScopeGrantsWithUsersFn func(ctx context.Context, clientID int64) ([]repository.ScopeGrantWithUser, error)
	deleteScopeGrantFn               func(ctx context.Context, id int64, clientID int64) error
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

func (m *mockOAuthClientRepo) FindClientByID(ctx context.Context, id int64) (*model.OAuthClient, error) {
	if m.findClientByIDFn != nil {
		return m.findClientByIDFn(ctx, id)
	}
	return nil, errors.New("not found")
}

func (m *mockOAuthClientRepo) DeleteClient(ctx context.Context, id int64) error {
	if m.deleteClientFn != nil {
		return m.deleteClientFn(ctx, id)
	}
	return nil
}

func (m *mockOAuthClientRepo) FindActiveScopeGrantsWithUsers(ctx context.Context, clientID int64) ([]repository.ScopeGrantWithUser, error) {
	if m.findActiveScopeGrantsWithUsersFn != nil {
		return m.findActiveScopeGrantsWithUsersFn(ctx, clientID)
	}
	return nil, nil
}

func (m *mockOAuthClientRepo) DeleteScopeGrant(ctx context.Context, id int64, clientID int64) error {
	if m.deleteScopeGrantFn != nil {
		return m.deleteScopeGrantFn(ctx, id, clientID)
	}
	return nil
}

// newTestOAuthClientHandler creates an OAuthClientHandler with the mock repo
// wired into both the handler (for List/Delete/Show/ScopeGrants) and the
// service (for Create/RegisterClient).
func newTestOAuthClientHandler(repo *mockOAuthClientRepo) *OAuthClientHandler {
	logger := testutil.DiscardLogger()
	svc := service.NewOAuthClientService(repo, logger)
	return NewOAuthClientHandler(repo, svc, logger)
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
				},
			}, 1, nil
		},
	}

	h := newTestOAuthClientHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/oauth-clients", http.NoBody)
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
			return nil, 0, errors.New("db error")
		},
	}

	h := newTestOAuthClientHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/oauth-clients", http.NoBody)
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

	h := newTestOAuthClientHandler(repo)

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

	h := newTestOAuthClientHandler(&mockOAuthClientRepo{})

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

	h := newTestOAuthClientHandler(&mockOAuthClientRepo{})

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
			return errors.New("constraint violation")
		},
	}

	h := newTestOAuthClientHandler(repo)

	body := `{"client_name":"Fail App"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/oauth-clients", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// Tests: Delete
// ---------------------------------------------------------------------------

func TestOAuthClientHandler_Delete_Success(t *testing.T) {
	t.Parallel()

	h := newTestOAuthClientHandler(&mockOAuthClientRepo{})

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/oauth-clients/1", http.NoBody)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	h.Delete(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestOAuthClientHandler_Delete_InvalidID(t *testing.T) {
	t.Parallel()

	h := newTestOAuthClientHandler(&mockOAuthClientRepo{})

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "abc")
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/oauth-clients/abc", http.NoBody)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	h.Delete(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestOAuthClientHandler_Delete_Error(t *testing.T) {
	t.Parallel()

	repo := &mockOAuthClientRepo{
		deleteClientFn: func(_ context.Context, _ int64) error {
			return errors.New("db error")
		},
	}

	h := newTestOAuthClientHandler(repo)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/oauth-clients/1", http.NoBody)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	h.Delete(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestOAuthClientHandler_Delete_NotFound(t *testing.T) {
	t.Parallel()

	repo := &mockOAuthClientRepo{
		deleteClientFn: func(_ context.Context, _ int64) error {
			return sql.ErrNoRows
		},
	}

	h := newTestOAuthClientHandler(repo)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "999")
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/oauth-clients/999", http.NoBody)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	h.Delete(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// ---------------------------------------------------------------------------
// Tests: toOAuthClientResponse
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Tests: Show
// ---------------------------------------------------------------------------

func TestOAuthClientHandler_Show_Success(t *testing.T) {
	t.Parallel()

	repo := &mockOAuthClientRepo{
		findClientByIDFn: func(_ context.Context, id int64) (*model.OAuthClient, error) {
			if id == 1 {
				return &model.OAuthClient{
					ID:                      1,
					ClientID:                "client-1",
					ClientName:              "Test Client",
					RedirectURIs:            `["https://example.com/callback"]`,
					GrantTypes:              `["authorization_code"]`,
					ResponseTypes:           `["code"]`,
					TokenEndpointAuthMethod: "client_secret_post",
				}, nil
			}
			return nil, errors.New("not found")
		},
	}

	h := newTestOAuthClientHandler(repo)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req := httptest.NewRequest(http.MethodGet, "/api/admin/oauth-clients/1", http.NoBody)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	h.Show(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding: %v", err)
	}

	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatal("expected data object in response")
	}
	if data["client_id"] != "client-1" {
		t.Errorf("client_id = %v, want client-1", data["client_id"])
	}
}

func TestOAuthClientHandler_Show_InvalidID(t *testing.T) {
	t.Parallel()

	h := newTestOAuthClientHandler(&mockOAuthClientRepo{})

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "abc")
	req := httptest.NewRequest(http.MethodGet, "/api/admin/oauth-clients/abc", http.NoBody)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	h.Show(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestOAuthClientHandler_Show_NotFound(t *testing.T) {
	t.Parallel()

	repo := &mockOAuthClientRepo{
		findClientByIDFn: func(_ context.Context, _ int64) (*model.OAuthClient, error) {
			return nil, errors.New("not found")
		},
	}

	h := newTestOAuthClientHandler(repo)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "999")
	req := httptest.NewRequest(http.MethodGet, "/api/admin/oauth-clients/999", http.NoBody)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	h.Show(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

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
	}

	resp := toOAuthClientResponse(client)

	if resp.ClientID != "test-client" {
		t.Errorf("ClientID = %q, want %q", resp.ClientID, "test-client")
	}
	if len(resp.RedirectURIs) != 1 || resp.RedirectURIs[0] != "https://example.com" {
		t.Errorf("RedirectURIs = %v, want [https://example.com]", resp.RedirectURIs)
	}
}

func TestToOAuthClientResponse_AllBranches(t *testing.T) {
	t.Parallel()

	t.Run("nil redirect URIs and grant types fall back to empty slices", func(t *testing.T) {
		t.Parallel()

		// Invalid JSON causes ParseRedirectURIs / ParseGrantTypes to return nil.
		client := &model.OAuthClient{
			RedirectURIs:  "not-valid-json",
			GrantTypes:    "also-not-valid",
			ResponseTypes: `["code"]`,
		}

		resp := toOAuthClientResponse(client)

		if resp.RedirectURIs == nil {
			t.Error("RedirectURIs should be empty slice, not nil")
		}
		if len(resp.RedirectURIs) != 0 {
			t.Errorf("RedirectURIs = %v, want []", resp.RedirectURIs)
		}
		if resp.GrantTypes == nil {
			t.Error("GrantTypes should be empty slice, not nil")
		}
	})

	t.Run("invalid ResponseTypes JSON falls back to empty slice", func(t *testing.T) {
		t.Parallel()

		client := &model.OAuthClient{
			RedirectURIs:  `["https://example.com"]`,
			GrantTypes:    `["authorization_code"]`,
			ResponseTypes: "not-json",
		}

		resp := toOAuthClientResponse(client)

		if resp.ResponseTypes == nil {
			t.Error("ResponseTypes should be empty slice, not nil")
		}
		if len(resp.ResponseTypes) != 0 {
			t.Errorf("ResponseTypes = %v, want []", resp.ResponseTypes)
		}
	})

	t.Run("valid LastUsedAt is formatted as RFC3339", func(t *testing.T) {
		t.Parallel()

		now := time.Now().Truncate(time.Second).UTC()
		client := &model.OAuthClient{
			RedirectURIs:  `["https://example.com"]`,
			GrantTypes:    `["authorization_code"]`,
			ResponseTypes: `["code"]`,
			LastUsedAt:    sql.NullTime{Time: now, Valid: true},
		}

		resp := toOAuthClientResponse(client)

		if resp.LastUsedAt == nil {
			t.Fatal("LastUsedAt should be non-nil when Valid=true")
		}
		if *resp.LastUsedAt != now.Format(time.RFC3339) {
			t.Errorf("LastUsedAt = %q, want %q", *resp.LastUsedAt, now.Format(time.RFC3339))
		}
	})

	t.Run("valid Scope, CreatedAt, UpdatedAt are populated", func(t *testing.T) {
		t.Parallel()

		now := time.Now().Truncate(time.Second)
		client := &model.OAuthClient{
			RedirectURIs:  `["https://example.com"]`,
			GrantTypes:    `["authorization_code"]`,
			ResponseTypes: `["code"]`,
			Scope:         sql.NullString{String: "documents:read", Valid: true},
			CreatedAt:     sql.NullTime{Time: now, Valid: true},
			UpdatedAt:     sql.NullTime{Time: now, Valid: true},
		}

		resp := toOAuthClientResponse(client)

		if resp.Scope != "documents:read" {
			t.Errorf("Scope = %q, want %q", resp.Scope, "documents:read")
		}
		wantTime := now.Format(time.RFC3339)
		if resp.CreatedAt != wantTime {
			t.Errorf("CreatedAt = %q, want %q", resp.CreatedAt, wantTime)
		}
		if resp.UpdatedAt != wantTime {
			t.Errorf("UpdatedAt = %q, want %q", resp.UpdatedAt, wantTime)
		}
	})
}

// ---------------------------------------------------------------------------
// Tests: ListScopeGrants
// ---------------------------------------------------------------------------

// TestOAuthClientHandler_ListScopeGrants_IncludesGranterIdentity is a
// regression guard for the bug where the admin UI rendered "User #1" because
// the handler only emitted the raw granted_by integer. The fix LEFT JOINs
// users so email and name ride along with each grant; this test asserts both
// fields reach the JSON envelope.
func TestOAuthClientHandler_ListScopeGrants_IncludesGranterIdentity(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	repo := &mockOAuthClientRepo{
		findClientByIDFn: func(_ context.Context, id int64) (*model.OAuthClient, error) {
			return &model.OAuthClient{ID: id, ClientID: "client-1"}, nil
		},
		findActiveScopeGrantsWithUsersFn: func(_ context.Context, _ int64) ([]repository.ScopeGrantWithUser, error) {
			return []repository.ScopeGrantWithUser{
				{
					OAuthClientScopeGrant: model.OAuthClientScopeGrant{
						ID:        10,
						ClientID:  1,
						Scope:     "documents:read",
						GrantedBy: 42,
						GrantedAt: now,
					},
					GrantedByEmail: sql.NullString{String: "alice@example.com", Valid: true},
					GrantedByName:  sql.NullString{String: "Alice Admin", Valid: true},
				},
				{
					// Deleted granter — user row gone, fields NULL.
					OAuthClientScopeGrant: model.OAuthClientScopeGrant{
						ID:        11,
						ClientID:  1,
						Scope:     "documents:read",
						GrantedBy: 99,
						GrantedAt: now,
					},
				},
			}, nil
		},
	}

	h := newTestOAuthClientHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/oauth-clients/1/scope-grants", http.NoBody)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	h.ListScopeGrants(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body struct {
		Data []scopeGrantResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if len(body.Data) != 2 {
		t.Fatalf("got %d grants, want 2", len(body.Data))
	}

	if body.Data[0].GrantedByEmail == nil || *body.Data[0].GrantedByEmail != "alice@example.com" {
		t.Errorf("GrantedByEmail[0] = %v, want alice@example.com", body.Data[0].GrantedByEmail)
	}
	if body.Data[0].GrantedByName == nil || *body.Data[0].GrantedByName != "Alice Admin" {
		t.Errorf("GrantedByName[0] = %v, want Alice Admin", body.Data[0].GrantedByName)
	}

	// Deleted granter: granted_by integer still present, email + name null.
	if body.Data[1].GrantedBy != 99 {
		t.Errorf("GrantedBy[1] = %d, want 99", body.Data[1].GrantedBy)
	}
	if body.Data[1].GrantedByEmail != nil {
		t.Errorf("GrantedByEmail[1] = %v, want nil (deleted user)", *body.Data[1].GrantedByEmail)
	}
	if body.Data[1].GrantedByName != nil {
		t.Errorf("GrantedByName[1] = %v, want nil (deleted user)", *body.Data[1].GrantedByName)
	}
}
