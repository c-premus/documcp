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

	authmiddleware "git.999.haus/chris/DocuMCP-go/internal/auth/middleware"
	"git.999.haus/chris/DocuMCP-go/internal/model"
)

// ---------------------------------------------------------------------------
// Mock user repository
// ---------------------------------------------------------------------------

type mockUserRepo struct {
	listUsersFn    func(ctx context.Context, query string, limit, offset int) ([]model.User, int, error)
	findUserByIDFn func(ctx context.Context, id int64) (*model.User, error)
	createUserFn   func(ctx context.Context, user *model.User) error
	updateUserFn   func(ctx context.Context, user *model.User) error
	deleteUserFn   func(ctx context.Context, id int64) error
	toggleAdminFn  func(ctx context.Context, id int64) error
}

func (m *mockUserRepo) ListUsers(ctx context.Context, query string, limit, offset int) ([]model.User, int, error) {
	if m.listUsersFn != nil {
		return m.listUsersFn(ctx, query, limit, offset)
	}
	return nil, 0, nil
}

func (m *mockUserRepo) FindUserByID(ctx context.Context, id int64) (*model.User, error) {
	if m.findUserByIDFn != nil {
		return m.findUserByIDFn(ctx, id)
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockUserRepo) CreateUser(ctx context.Context, user *model.User) error {
	if m.createUserFn != nil {
		return m.createUserFn(ctx, user)
	}
	return nil
}

func (m *mockUserRepo) UpdateUser(ctx context.Context, user *model.User) error {
	if m.updateUserFn != nil {
		return m.updateUserFn(ctx, user)
	}
	return nil
}

func (m *mockUserRepo) DeleteUser(ctx context.Context, id int64) error {
	if m.deleteUserFn != nil {
		return m.deleteUserFn(ctx, id)
	}
	return nil
}

func (m *mockUserRepo) ToggleAdmin(ctx context.Context, id int64) error {
	if m.toggleAdminFn != nil {
		return m.toggleAdminFn(ctx, id)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func chiCtxWithParam(r *http.Request, val string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", val)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func withAuthUser(r *http.Request, user *model.User) *http.Request {
	ctx := context.WithValue(r.Context(), authmiddleware.UserContextKey, user)
	return r.WithContext(ctx)
}

// ---------------------------------------------------------------------------
// Tests: List
// ---------------------------------------------------------------------------

func TestUserHandler_List_Success(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepo{
		listUsersFn: func(_ context.Context, query string, limit, offset int) ([]model.User, int, error) {
			return []model.User{
				{ID: 1, Name: "Alice", Email: "alice@example.com"},
				{ID: 2, Name: "Bob", Email: "bob@example.com"},
			}, 2, nil
		},
	}

	h := NewUserHandler(repo, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/admin/users?limit=10&offset=0", nil)
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
	if !ok || len(data) != 2 {
		t.Errorf("expected 2 users, got %v", resp["data"])
	}

	meta := resp["meta"].(map[string]any)
	if meta["total"].(float64) != 2 {
		t.Errorf("total = %v, want 2", meta["total"])
	}
}

func TestUserHandler_List_DefaultLimitOffset(t *testing.T) {
	t.Parallel()

	var gotLimit, gotOffset int
	repo := &mockUserRepo{
		listUsersFn: func(_ context.Context, _ string, limit, offset int) ([]model.User, int, error) {
			gotLimit = limit
			gotOffset = offset
			return nil, 0, nil
		},
	}

	h := NewUserHandler(repo, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	rec := httptest.NewRecorder()

	h.List(rec, req)

	if gotLimit != 20 {
		t.Errorf("default limit = %d, want 20", gotLimit)
	}
	if gotOffset != 0 {
		t.Errorf("default offset = %d, want 0", gotOffset)
	}
}

func TestUserHandler_List_Error(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepo{
		listUsersFn: func(_ context.Context, _ string, _, _ int) ([]model.User, int, error) {
			return nil, 0, fmt.Errorf("db error")
		},
	}

	h := NewUserHandler(repo, discardLogger())
	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	rec := httptest.NewRecorder()

	h.List(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// Tests: Show
// ---------------------------------------------------------------------------

func TestUserHandler_Show_Success(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepo{
		findUserByIDFn: func(_ context.Context, id int64) (*model.User, error) {
			return &model.User{ID: id, Name: "Alice", Email: "alice@example.com"}, nil
		},
	}

	h := NewUserHandler(repo, discardLogger())
	req := chiCtxWithParam(httptest.NewRequest(http.MethodGet, "/api/admin/users/1", nil), "1")
	rec := httptest.NewRecorder()

	h.Show(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestUserHandler_Show_NotFound(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepo{
		findUserByIDFn: func(_ context.Context, _ int64) (*model.User, error) {
			return nil, fmt.Errorf("not found")
		},
	}

	h := NewUserHandler(repo, discardLogger())
	req := chiCtxWithParam(httptest.NewRequest(http.MethodGet, "/api/admin/users/999", nil), "999")
	rec := httptest.NewRecorder()

	h.Show(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestUserHandler_Show_InvalidID(t *testing.T) {
	t.Parallel()

	h := NewUserHandler(&mockUserRepo{}, discardLogger())
	req := chiCtxWithParam(httptest.NewRequest(http.MethodGet, "/api/admin/users/abc", nil), "abc")
	rec := httptest.NewRecorder()

	h.Show(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// ---------------------------------------------------------------------------
// Tests: Create
// ---------------------------------------------------------------------------

func TestUserHandler_Create_Success(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepo{
		createUserFn: func(_ context.Context, user *model.User) error {
			user.ID = 42
			return nil
		},
	}

	h := NewUserHandler(repo, discardLogger())

	body := `{"name":"Charlie","email":"charlie@example.com","is_admin":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
}

func TestUserHandler_Create_MissingFields(t *testing.T) {
	t.Parallel()

	h := NewUserHandler(&mockUserRepo{}, discardLogger())

	body := `{"name":"Charlie"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUserHandler_Create_InvalidJSON(t *testing.T) {
	t.Parallel()

	h := NewUserHandler(&mockUserRepo{}, discardLogger())

	req := httptest.NewRequest(http.MethodPost, "/api/admin/users", bytes.NewBufferString("{invalid"))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUserHandler_Create_RepoError(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepo{
		createUserFn: func(_ context.Context, _ *model.User) error {
			return fmt.Errorf("unique constraint violation")
		},
	}

	h := NewUserHandler(repo, discardLogger())

	body := `{"name":"Charlie","email":"charlie@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// Tests: Update
// ---------------------------------------------------------------------------

func TestUserHandler_Update_Success(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepo{
		findUserByIDFn: func(_ context.Context, _ int64) (*model.User, error) {
			return &model.User{ID: 1, Name: "Alice", Email: "alice@example.com"}, nil
		},
	}

	h := NewUserHandler(repo, discardLogger())

	body := `{"name":"Alice Updated"}`
	req := chiCtxWithParam(httptest.NewRequest(http.MethodPut, "/api/admin/users/1", bytes.NewBufferString(body)), "1")
	rec := httptest.NewRecorder()

	h.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestUserHandler_Update_NotFound(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepo{
		findUserByIDFn: func(_ context.Context, _ int64) (*model.User, error) {
			return nil, fmt.Errorf("not found")
		},
	}

	h := NewUserHandler(repo, discardLogger())

	body := `{"name":"Updated"}`
	req := chiCtxWithParam(httptest.NewRequest(http.MethodPut, "/api/admin/users/999", bytes.NewBufferString(body)), "999")
	rec := httptest.NewRecorder()

	h.Update(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// ---------------------------------------------------------------------------
// Tests: Delete
// ---------------------------------------------------------------------------

func TestUserHandler_Delete_Success(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepo{}

	h := NewUserHandler(repo, discardLogger())

	// Set a different user as the current user so self-deletion check passes.
	req := chiCtxWithParam(httptest.NewRequest(http.MethodDelete, "/api/admin/users/2", nil), "2")
	req = withAuthUser(req, &model.User{ID: 99, Name: "Admin"})
	rec := httptest.NewRecorder()

	h.Delete(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestUserHandler_Delete_SelfDeletion(t *testing.T) {
	t.Parallel()

	h := NewUserHandler(&mockUserRepo{}, discardLogger())

	req := chiCtxWithParam(httptest.NewRequest(http.MethodDelete, "/api/admin/users/42", nil), "42")
	req = withAuthUser(req, &model.User{ID: 42, Name: "Self"})
	rec := httptest.NewRecorder()

	h.Delete(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUserHandler_Delete_RepoError(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepo{
		deleteUserFn: func(_ context.Context, _ int64) error {
			return fmt.Errorf("db error")
		},
	}

	h := NewUserHandler(repo, discardLogger())

	req := chiCtxWithParam(httptest.NewRequest(http.MethodDelete, "/api/admin/users/2", nil), "2")
	req = withAuthUser(req, &model.User{ID: 99})
	rec := httptest.NewRecorder()

	h.Delete(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// Tests: ToggleAdmin
// ---------------------------------------------------------------------------

func TestUserHandler_ToggleAdmin_Success(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepo{
		findUserByIDFn: func(_ context.Context, id int64) (*model.User, error) {
			return &model.User{ID: id, Name: "Bob", Email: "bob@example.com", IsAdmin: true}, nil
		},
	}

	h := NewUserHandler(repo, discardLogger())

	req := chiCtxWithParam(httptest.NewRequest(http.MethodPost, "/api/admin/users/2/toggle-admin", nil), "2")
	req = withAuthUser(req, &model.User{ID: 99})
	rec := httptest.NewRecorder()

	h.ToggleAdmin(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestUserHandler_ToggleAdmin_SelfDemotion(t *testing.T) {
	t.Parallel()

	h := NewUserHandler(&mockUserRepo{}, discardLogger())

	req := chiCtxWithParam(httptest.NewRequest(http.MethodPost, "/api/admin/users/42/toggle-admin", nil), "42")
	req = withAuthUser(req, &model.User{ID: 42})
	rec := httptest.NewRecorder()

	h.ToggleAdmin(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// ---------------------------------------------------------------------------
// Tests: newUserResponse
// ---------------------------------------------------------------------------

func TestNewUserResponse(t *testing.T) {
	t.Parallel()

	user := &model.User{
		ID:      1,
		Name:    "Alice",
		Email:   "alice@example.com",
		IsAdmin: true,
	}

	resp := newUserResponse(user)

	if resp.ID != 1 {
		t.Errorf("ID = %d, want 1", resp.ID)
	}
	if resp.Name != "Alice" {
		t.Errorf("Name = %q, want %q", resp.Name, "Alice")
	}
	if resp.Email != "alice@example.com" {
		t.Errorf("Email = %q, want %q", resp.Email, "alice@example.com")
	}
	if !resp.IsAdmin {
		t.Error("IsAdmin should be true")
	}
	// Null optional fields should be empty strings.
	if resp.OIDCSub != "" {
		t.Errorf("OIDCSub = %q, want empty", resp.OIDCSub)
	}
}
