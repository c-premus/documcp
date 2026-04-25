package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	authmiddleware "github.com/c-premus/documcp/internal/auth/middleware"
	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/testutil"
)

// ---------------------------------------------------------------------------
// Mock user repository
// ---------------------------------------------------------------------------

type mockUserRepo struct {
	listUsersFn    func(ctx context.Context, query string, limit, offset int) ([]model.User, int, error)
	findUserByIDFn func(ctx context.Context, id int64) (*model.User, error)
	deleteUserFn   func(ctx context.Context, id int64) error
	toggleAdminFn  func(ctx context.Context, id int64) error
}

// recordingSessionRevoker captures every call so tests can assert on what the
// handler asked the store to do without spinning up miniredis.
type recordingSessionRevoker struct {
	listFn       func(ctx context.Context, userID int64) ([]string, error)
	revokeOneFn  func(ctx context.Context, sessionID string) error
	revokeAllFn  func(ctx context.Context, userID int64) (int, error)
	listCalls    []int64
	revokeOneIDs []string
	revokeAllIDs []int64
}

func (m *recordingSessionRevoker) ListUserSessions(ctx context.Context, userID int64) ([]string, error) {
	m.listCalls = append(m.listCalls, userID)
	if m.listFn != nil {
		return m.listFn(ctx, userID)
	}
	return nil, nil
}

func (m *recordingSessionRevoker) RevokeSession(ctx context.Context, sessionID string) error {
	m.revokeOneIDs = append(m.revokeOneIDs, sessionID)
	if m.revokeOneFn != nil {
		return m.revokeOneFn(ctx, sessionID)
	}
	return nil
}

func (m *recordingSessionRevoker) RevokeUserSessions(ctx context.Context, userID int64) (int, error) {
	m.revokeAllIDs = append(m.revokeAllIDs, userID)
	if m.revokeAllFn != nil {
		return m.revokeAllFn(ctx, userID)
	}
	return 0, nil
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
	return nil, errors.New("not found")
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

func chiCtxWithParams(r *http.Request, kv ...string) *http.Request {
	rctx := chi.NewRouteContext()
	for i := 0; i+1 < len(kv); i += 2 {
		rctx.URLParams.Add(kv[i], kv[i+1])
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
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

	h := NewUserHandler(repo, nil, testutil.DiscardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/admin/users?limit=10&offset=0", http.NoBody)
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

	h := NewUserHandler(repo, nil, testutil.DiscardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", http.NoBody)
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
			return nil, 0, errors.New("db error")
		},
	}

	h := NewUserHandler(repo, nil, testutil.DiscardLogger())
	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", http.NoBody)
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

	h := NewUserHandler(repo, nil, testutil.DiscardLogger())
	req := chiCtxWithParam(httptest.NewRequest(http.MethodGet, "/api/admin/users/1", http.NoBody), "1")
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
			return nil, errors.New("not found")
		},
	}

	h := NewUserHandler(repo, nil, testutil.DiscardLogger())
	req := chiCtxWithParam(httptest.NewRequest(http.MethodGet, "/api/admin/users/999", http.NoBody), "999")
	rec := httptest.NewRecorder()

	h.Show(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestUserHandler_Show_InvalidID(t *testing.T) {
	t.Parallel()

	h := NewUserHandler(&mockUserRepo{}, nil, testutil.DiscardLogger())
	req := chiCtxWithParam(httptest.NewRequest(http.MethodGet, "/api/admin/users/abc", http.NoBody), "abc")
	rec := httptest.NewRecorder()

	h.Show(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// ---------------------------------------------------------------------------
// Tests: Delete
// ---------------------------------------------------------------------------

func TestUserHandler_Delete_Success(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepo{}

	h := NewUserHandler(repo, nil, testutil.DiscardLogger())

	// Set a different user as the current user so self-deletion check passes.
	req := chiCtxWithParam(httptest.NewRequest(http.MethodDelete, "/api/admin/users/2", http.NoBody), "2")
	req = withAuthUser(req, &model.User{ID: 99, Name: "Admin"})
	rec := httptest.NewRecorder()

	h.Delete(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestUserHandler_Delete_SelfDeletion(t *testing.T) {
	t.Parallel()

	h := NewUserHandler(&mockUserRepo{}, nil, testutil.DiscardLogger())

	req := chiCtxWithParam(httptest.NewRequest(http.MethodDelete, "/api/admin/users/42", http.NoBody), "42")
	req = withAuthUser(req, &model.User{ID: 42, Name: "Self"})
	rec := httptest.NewRecorder()

	h.Delete(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUserHandler_Delete_RevokesUserSessions(t *testing.T) {
	t.Parallel()

	rev := &recordingSessionRevoker{
		revokeAllFn: func(_ context.Context, _ int64) (int, error) { return 3, nil },
	}
	h := NewUserHandler(&mockUserRepo{}, rev, testutil.DiscardLogger())

	req := chiCtxWithParam(httptest.NewRequest(http.MethodDelete, "/api/admin/users/2", http.NoBody), "2")
	req = withAuthUser(req, &model.User{ID: 99})
	rec := httptest.NewRecorder()

	h.Delete(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(rev.revokeAllIDs) != 1 || rev.revokeAllIDs[0] != 2 {
		t.Fatalf("RevokeUserSessions calls = %v, want [2]", rev.revokeAllIDs)
	}
}

func TestUserHandler_Delete_RevokerErrorDoesNotFailRequest(t *testing.T) {
	t.Parallel()

	rev := &recordingSessionRevoker{
		revokeAllFn: func(_ context.Context, _ int64) (int, error) {
			return 0, errors.New("redis down")
		},
	}
	h := NewUserHandler(&mockUserRepo{}, rev, testutil.DiscardLogger())

	req := chiCtxWithParam(httptest.NewRequest(http.MethodDelete, "/api/admin/users/2", http.NoBody), "2")
	req = withAuthUser(req, &model.User{ID: 99})
	rec := httptest.NewRecorder()

	h.Delete(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (revoker error must not fail user delete)", rec.Code, http.StatusOK)
	}
}

func TestUserHandler_Delete_RepoError(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepo{
		deleteUserFn: func(_ context.Context, _ int64) error {
			return errors.New("db error")
		},
	}

	h := NewUserHandler(repo, nil, testutil.DiscardLogger())

	req := chiCtxWithParam(httptest.NewRequest(http.MethodDelete, "/api/admin/users/2", http.NoBody), "2")
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

	h := NewUserHandler(repo, nil, testutil.DiscardLogger())

	req := chiCtxWithParam(httptest.NewRequest(http.MethodPost, "/api/admin/users/2/toggle-admin", http.NoBody), "2")
	req = withAuthUser(req, &model.User{ID: 99})
	rec := httptest.NewRecorder()

	h.ToggleAdmin(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestUserHandler_ToggleAdmin_SelfDemotion(t *testing.T) {
	t.Parallel()

	h := NewUserHandler(&mockUserRepo{}, nil, testutil.DiscardLogger())

	req := chiCtxWithParam(httptest.NewRequest(http.MethodPost, "/api/admin/users/42/toggle-admin", http.NoBody), "42")
	req = withAuthUser(req, &model.User{ID: 42})
	rec := httptest.NewRecorder()

	h.ToggleAdmin(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUserHandler_ToggleAdmin_DemoteRevokesSessions(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepo{
		findUserByIDFn: func(_ context.Context, id int64) (*model.User, error) {
			return &model.User{ID: id, IsAdmin: false}, nil // post-toggle state: demoted
		},
	}
	rev := &recordingSessionRevoker{
		revokeAllFn: func(_ context.Context, _ int64) (int, error) { return 1, nil },
	}
	h := NewUserHandler(repo, rev, testutil.DiscardLogger())

	req := chiCtxWithParam(httptest.NewRequest(http.MethodPost, "/api/admin/users/5/toggle-admin", http.NoBody), "5")
	req = withAuthUser(req, &model.User{ID: 99})
	rec := httptest.NewRecorder()

	h.ToggleAdmin(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(rev.revokeAllIDs) != 1 || rev.revokeAllIDs[0] != 5 {
		t.Fatalf("RevokeUserSessions calls = %v, want [5]", rev.revokeAllIDs)
	}
}

func TestUserHandler_ToggleAdmin_PromoteDoesNotRevokeSessions(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepo{
		findUserByIDFn: func(_ context.Context, id int64) (*model.User, error) {
			return &model.User{ID: id, IsAdmin: true}, nil // post-toggle state: promoted
		},
	}
	rev := &recordingSessionRevoker{}
	h := NewUserHandler(repo, rev, testutil.DiscardLogger())

	req := chiCtxWithParam(httptest.NewRequest(http.MethodPost, "/api/admin/users/5/toggle-admin", http.NoBody), "5")
	req = withAuthUser(req, &model.User{ID: 99})
	rec := httptest.NewRecorder()

	h.ToggleAdmin(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(rev.revokeAllIDs) != 0 {
		t.Fatalf("RevokeUserSessions calls = %v, want none on promote", rev.revokeAllIDs)
	}
}

func TestUserHandler_ToggleAdmin_RepoError(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepo{
		toggleAdminFn: func(_ context.Context, _ int64) error {
			return errors.New("db error")
		},
	}

	h := NewUserHandler(repo, nil, testutil.DiscardLogger())
	req := chiCtxWithParam(httptest.NewRequest(http.MethodPost, "/api/admin/users/5/toggle-admin", http.NoBody), "5")
	req = withAuthUser(req, &model.User{ID: 99})
	rec := httptest.NewRecorder()

	h.ToggleAdmin(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestUserHandler_ToggleAdmin_FindAfterToggleError(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepo{
		toggleAdminFn: func(_ context.Context, _ int64) error { return nil },
		findUserByIDFn: func(_ context.Context, _ int64) (*model.User, error) {
			return nil, errors.New("not found")
		},
	}

	h := NewUserHandler(repo, nil, testutil.DiscardLogger())
	req := chiCtxWithParam(httptest.NewRequest(http.MethodPost, "/api/admin/users/5/toggle-admin", http.NoBody), "5")
	req = withAuthUser(req, &model.User{ID: 99})
	rec := httptest.NewRecorder()

	h.ToggleAdmin(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// ---------------------------------------------------------------------------
// Tests: newUserResponse
// ---------------------------------------------------------------------------

func TestNewUserResponse(t *testing.T) {
	t.Parallel()

	t.Run("null optional fields are empty strings", func(t *testing.T) {
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
		if resp.OIDCSub != "" {
			t.Errorf("OIDCSub = %q, want empty", resp.OIDCSub)
		}
		if resp.OIDCProvider != "" {
			t.Errorf("OIDCProvider = %q, want empty", resp.OIDCProvider)
		}
		if resp.EmailVerifiedAt != "" {
			t.Errorf("EmailVerifiedAt = %q, want empty", resp.EmailVerifiedAt)
		}
		if resp.CreatedAt != "" {
			t.Errorf("CreatedAt = %q, want empty", resp.CreatedAt)
		}
		if resp.UpdatedAt != "" {
			t.Errorf("UpdatedAt = %q, want empty", resp.UpdatedAt)
		}
	})

	t.Run("valid optional fields are populated", func(t *testing.T) {
		t.Parallel()

		now := time.Now().Truncate(time.Second)
		user := &model.User{
			ID:              2,
			Name:            "Bob",
			Email:           "bob@example.com",
			IsAdmin:         false,
			OIDCSub:         sql.NullString{String: "sub-123", Valid: true},
			OIDCProvider:    sql.NullString{String: "google", Valid: true},
			EmailVerifiedAt: sql.NullTime{Time: now, Valid: true},
			CreatedAt:       sql.NullTime{Time: now, Valid: true},
			UpdatedAt:       sql.NullTime{Time: now, Valid: true},
		}

		resp := newUserResponse(user)

		if resp.OIDCSub != "sub-123" {
			t.Errorf("OIDCSub = %q, want %q", resp.OIDCSub, "sub-123")
		}
		if resp.OIDCProvider != "google" {
			t.Errorf("OIDCProvider = %q, want %q", resp.OIDCProvider, "google")
		}
		wantTime := now.Format(time.RFC3339)
		if resp.EmailVerifiedAt != wantTime {
			t.Errorf("EmailVerifiedAt = %q, want %q", resp.EmailVerifiedAt, wantTime)
		}
		if resp.CreatedAt != wantTime {
			t.Errorf("CreatedAt = %q, want %q", resp.CreatedAt, wantTime)
		}
		if resp.UpdatedAt != wantTime {
			t.Errorf("UpdatedAt = %q, want %q", resp.UpdatedAt, wantTime)
		}
	})
}

// ---------------------------------------------------------------------------
// Tests: ListSessions / RevokeSession / RevokeAllSessions
// ---------------------------------------------------------------------------

func TestUserHandler_ListSessions_ReturnsIDs(t *testing.T) {
	t.Parallel()

	rev := &recordingSessionRevoker{
		listFn: func(_ context.Context, _ int64) ([]string, error) {
			return []string{"abc", "def"}, nil
		},
	}
	h := NewUserHandler(&mockUserRepo{}, rev, testutil.DiscardLogger())

	req := chiCtxWithParam(httptest.NewRequest(http.MethodGet, "/api/admin/users/7/sessions", http.NoBody), "7")
	rec := httptest.NewRecorder()

	h.ListSessions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	data, ok := resp["data"].([]any)
	if !ok {
		t.Fatalf("data = %v, want array", resp["data"])
	}
	if len(data) != 2 {
		t.Fatalf("len(data) = %d, want 2", len(data))
	}
}

func TestUserHandler_ListSessions_NilStoreReturnsEmpty(t *testing.T) {
	t.Parallel()

	h := NewUserHandler(&mockUserRepo{}, nil, testutil.DiscardLogger())

	req := chiCtxWithParam(httptest.NewRequest(http.MethodGet, "/api/admin/users/7/sessions", http.NoBody), "7")
	rec := httptest.NewRecorder()

	h.ListSessions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestUserHandler_RevokeSession_DeletesByID(t *testing.T) {
	t.Parallel()

	rev := &recordingSessionRevoker{}
	h := NewUserHandler(&mockUserRepo{}, rev, testutil.DiscardLogger())

	req := chiCtxWithParams(
		httptest.NewRequest(http.MethodDelete, "/api/admin/users/7/sessions/abc", http.NoBody),
		"id", "7",
		"sessionID", "abc",
	)
	rec := httptest.NewRecorder()

	h.RevokeSession(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(rev.revokeOneIDs) != 1 || rev.revokeOneIDs[0] != "abc" {
		t.Fatalf("RevokeSession calls = %v, want [abc]", rev.revokeOneIDs)
	}
}

func TestUserHandler_RevokeSession_MissingIDIs400(t *testing.T) {
	t.Parallel()

	h := NewUserHandler(&mockUserRepo{}, &recordingSessionRevoker{}, testutil.DiscardLogger())

	req := chiCtxWithParam(httptest.NewRequest(http.MethodDelete, "/api/admin/users/7/sessions/", http.NoBody), "7")
	rec := httptest.NewRecorder()

	h.RevokeSession(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUserHandler_RevokeSession_NilStoreIs503(t *testing.T) {
	t.Parallel()

	h := NewUserHandler(&mockUserRepo{}, nil, testutil.DiscardLogger())

	req := chiCtxWithParams(
		httptest.NewRequest(http.MethodDelete, "/api/admin/users/7/sessions/abc", http.NoBody),
		"id", "7",
		"sessionID", "abc",
	)
	rec := httptest.NewRecorder()

	h.RevokeSession(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestUserHandler_RevokeAllSessions_ReturnsCount(t *testing.T) {
	t.Parallel()

	rev := &recordingSessionRevoker{
		revokeAllFn: func(_ context.Context, _ int64) (int, error) { return 4, nil },
	}
	h := NewUserHandler(&mockUserRepo{}, rev, testutil.DiscardLogger())

	req := chiCtxWithParam(httptest.NewRequest(http.MethodDelete, "/api/admin/users/7/sessions", http.NoBody), "7")
	rec := httptest.NewRecorder()

	h.RevokeAllSessions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	data := resp["data"].(map[string]any)
	if data["revoked"].(float64) != 4 {
		t.Fatalf("revoked = %v, want 4", data["revoked"])
	}
}
