package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/sessions"

	"git.999.haus/chris/DocuMCP-go/internal/model"
)

// ---------------------------------------------------------------------------
// Mock user finder
// ---------------------------------------------------------------------------

type mockAuthUserFinder struct {
	findUserByIDFn func(ctx context.Context, id int64) (*model.User, error)
}

func (m *mockAuthUserFinder) FindUserByID(ctx context.Context, id int64) (*model.User, error) {
	if m.findUserByIDFn != nil {
		return m.findUserByIDFn(ctx, id)
	}
	return nil, fmt.Errorf("user not found")
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestAuthHandler_Me_ValidSession(t *testing.T) {
	t.Parallel()

	store := sessions.NewCookieStore([]byte("test-secret-32-bytes-long!!!!!!!"))
	finder := &mockAuthUserFinder{
		findUserByIDFn: func(_ context.Context, id int64) (*model.User, error) {
			return &model.User{ID: id, Email: "test@example.com", Name: "Test User", IsAdmin: true}, nil
		},
	}

	h := NewAuthHandler(store, finder, discardLogger())

	// Create a request with a valid session cookie.
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	rec := httptest.NewRecorder()

	// Pre-set the session via the store.
	session, _ := store.Get(req, "documcp_session")
	session.Values["user_id"] = int64(42)
	if err := session.Save(req, rec); err != nil {
		t.Fatalf("saving session: %v", err)
	}

	// Copy the Set-Cookie to a new request.
	cookies := rec.Result().Cookies()
	req2 := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	for _, c := range cookies {
		req2.AddCookie(c)
	}
	rec2 := httptest.NewRecorder()

	h.Me(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec2.Code, http.StatusOK)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec2.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatal("expected 'data' key in response")
	}
	if data["email"] != "test@example.com" {
		t.Errorf("email = %v, want %q", data["email"], "test@example.com")
	}
	if data["name"] != "Test User" {
		t.Errorf("name = %v, want %q", data["name"], "Test User")
	}
	if data["is_admin"] != true {
		t.Errorf("is_admin = %v, want true", data["is_admin"])
	}
}

func TestAuthHandler_Me_NoSession(t *testing.T) {
	t.Parallel()

	store := sessions.NewCookieStore([]byte("test-secret-32-bytes-long!!!!!!!"))
	finder := &mockAuthUserFinder{}
	h := NewAuthHandler(store, finder, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	rec := httptest.NewRecorder()

	h.Me(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthHandler_Me_UserNotFound(t *testing.T) {
	t.Parallel()

	store := sessions.NewCookieStore([]byte("test-secret-32-bytes-long!!!!!!!"))
	finder := &mockAuthUserFinder{
		findUserByIDFn: func(_ context.Context, _ int64) (*model.User, error) {
			return nil, fmt.Errorf("no rows")
		},
	}

	h := NewAuthHandler(store, finder, discardLogger())

	// Build request with a valid session containing user_id.
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	rec := httptest.NewRecorder()
	session, _ := store.Get(req, "documcp_session")
	session.Values["user_id"] = int64(999)
	if err := session.Save(req, rec); err != nil {
		t.Fatalf("saving session: %v", err)
	}

	cookies := rec.Result().Cookies()
	req2 := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	for _, c := range cookies {
		req2.AddCookie(c)
	}
	rec2 := httptest.NewRecorder()

	h.Me(rec2, req2)

	if rec2.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec2.Code, http.StatusUnauthorized)
	}
}

func TestNewAuthHandler(t *testing.T) {
	t.Parallel()

	store := sessions.NewCookieStore([]byte("test-secret"))
	h := NewAuthHandler(store, &mockAuthUserFinder{}, discardLogger())
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
}
