package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	authmiddleware "github.com/c-premus/documcp/internal/auth/middleware"
	"github.com/c-premus/documcp/internal/model"
)

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestAuthHandler_Me_ValidUser(t *testing.T) {
	t.Parallel()

	h := NewAuthHandler(discardLogger())

	user := &model.User{ID: 42, Email: "test@example.com", Name: "Test User", IsAdmin: true}
	ctx := context.WithValue(context.Background(), authmiddleware.UserContextKey, user)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()

	h.Me(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
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

func TestAuthHandler_Me_NoUser(t *testing.T) {
	t.Parallel()

	h := NewAuthHandler(discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", http.NoBody)
	rec := httptest.NewRecorder()

	h.Me(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestNewAuthHandler(t *testing.T) {
	t.Parallel()

	h := NewAuthHandler(discardLogger())
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
}
