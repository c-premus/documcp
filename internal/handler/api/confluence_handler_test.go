package api

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"git.999.haus/chris/DocuMCP-go/internal/model"
)

// ---------------------------------------------------------------------------
// toConfluenceSpaceResponse tests
// ---------------------------------------------------------------------------

func TestToConfluenceSpaceResponse(t *testing.T) {
	t.Parallel()

	t.Run("maps all required fields", func(t *testing.T) {
		t.Parallel()

		cs := &model.ConfluenceSpace{
			UUID:         "conf-uuid-1",
			ConfluenceID: "12345",
			Key:          "DEV",
			Name:         "Development",
			Type:         "global",
			Status:       "current",
			IsSearchable: true,
		}

		resp := toConfluenceSpaceResponse(cs)

		if resp.UUID != "conf-uuid-1" {
			t.Errorf("UUID = %q, want conf-uuid-1", resp.UUID)
		}
		if resp.ConfluenceID != "12345" {
			t.Errorf("ConfluenceID = %q, want 12345", resp.ConfluenceID)
		}
		if resp.Key != "DEV" {
			t.Errorf("Key = %q, want DEV", resp.Key)
		}
		if resp.Name != "Development" {
			t.Errorf("Name = %q, want Development", resp.Name)
		}
		if resp.Type != "global" {
			t.Errorf("Type = %q, want global", resp.Type)
		}
		if resp.Status != "current" {
			t.Errorf("Status = %q, want current", resp.Status)
		}
		if !resp.IsSearchable {
			t.Error("IsSearchable = false, want true")
		}
	})

	t.Run("is_searchable false when model is false", func(t *testing.T) {
		t.Parallel()

		cs := &model.ConfluenceSpace{
			UUID:         "conf-uuid-2",
			ConfluenceID: "67890",
			Key:          "PRIV",
			Name:         "Private",
			Type:         "personal",
			Status:       "current",
			IsSearchable: false,
		}

		resp := toConfluenceSpaceResponse(cs)

		if resp.IsSearchable {
			t.Error("IsSearchable = true, want false")
		}
	})

	t.Run("maps optional NullString fields when valid", func(t *testing.T) {
		t.Parallel()

		cs := &model.ConfluenceSpace{
			UUID:         "conf-uuid-3",
			ConfluenceID: "111",
			Key:          "DOCS",
			Name:         "Documentation",
			Type:         "global",
			Status:       "current",
			Description:  sql.NullString{String: "Team docs space", Valid: true},
			HomepageID:   sql.NullString{String: "page-42", Valid: true},
			IconURL:      sql.NullString{String: "https://cdn.example.com/icon.png", Valid: true},
		}

		resp := toConfluenceSpaceResponse(cs)

		if resp.Description != "Team docs space" {
			t.Errorf("Description = %q, want Team docs space", resp.Description)
		}
		if resp.HomepageID != "page-42" {
			t.Errorf("HomepageID = %q, want page-42", resp.HomepageID)
		}
		if resp.IconURL != "https://cdn.example.com/icon.png" {
			t.Errorf("IconURL = %q, want https://cdn.example.com/icon.png", resp.IconURL)
		}
	})

	t.Run("null optional fields produce empty strings", func(t *testing.T) {
		t.Parallel()

		cs := &model.ConfluenceSpace{
			UUID:         "conf-uuid-4",
			ConfluenceID: "222",
			Key:          "MIN",
			Name:         "Minimal",
			Type:         "global",
			Status:       "current",
		}

		resp := toConfluenceSpaceResponse(cs)

		if resp.Description != "" {
			t.Errorf("Description = %q, want empty", resp.Description)
		}
		if resp.HomepageID != "" {
			t.Errorf("HomepageID = %q, want empty", resp.HomepageID)
		}
		if resp.IconURL != "" {
			t.Errorf("IconURL = %q, want empty", resp.IconURL)
		}
		if resp.LastSyncedAt != "" {
			t.Errorf("LastSyncedAt = %q, want empty", resp.LastSyncedAt)
		}
		if resp.CreatedAt != "" {
			t.Errorf("CreatedAt = %q, want empty", resp.CreatedAt)
		}
		if resp.UpdatedAt != "" {
			t.Errorf("UpdatedAt = %q, want empty", resp.UpdatedAt)
		}
	})

	t.Run("timestamps formatted as RFC3339 when valid", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2025, 8, 10, 16, 45, 0, 0, time.UTC)
		cs := &model.ConfluenceSpace{
			UUID:         "conf-uuid-5",
			ConfluenceID: "333",
			Key:          "TIME",
			Name:         "Timed",
			Type:         "global",
			Status:       "current",
			LastSyncedAt: sql.NullTime{Time: now, Valid: true},
			CreatedAt:    sql.NullTime{Time: now, Valid: true},
			UpdatedAt:    sql.NullTime{Time: now, Valid: true},
		}

		resp := toConfluenceSpaceResponse(cs)
		want := "2025-08-10T16:45:00Z"

		if resp.LastSyncedAt != want {
			t.Errorf("LastSyncedAt = %q, want %q", resp.LastSyncedAt, want)
		}
		if resp.CreatedAt != want {
			t.Errorf("CreatedAt = %q, want %q", resp.CreatedAt, want)
		}
		if resp.UpdatedAt != want {
			t.Errorf("UpdatedAt = %q, want %q", resp.UpdatedAt, want)
		}
	})

	t.Run("mixed valid and null timestamps", func(t *testing.T) {
		t.Parallel()

		created := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		cs := &model.ConfluenceSpace{
			UUID:         "conf-uuid-6",
			ConfluenceID: "444",
			Key:          "MIX",
			Name:         "Mixed",
			Type:         "global",
			Status:       "current",
			CreatedAt:    sql.NullTime{Time: created, Valid: true},
			// UpdatedAt and LastSyncedAt are null (not set)
		}

		resp := toConfluenceSpaceResponse(cs)

		if resp.CreatedAt != "2024-01-01T00:00:00Z" {
			t.Errorf("CreatedAt = %q, want 2024-01-01T00:00:00Z", resp.CreatedAt)
		}
		if resp.UpdatedAt != "" {
			t.Errorf("UpdatedAt = %q, want empty for null time", resp.UpdatedAt)
		}
		if resp.LastSyncedAt != "" {
			t.Errorf("LastSyncedAt = %q, want empty for null time", resp.LastSyncedAt)
		}
	})

	t.Run("different space types", func(t *testing.T) {
		t.Parallel()

		types := []string{"global", "personal", "collaboration"}
		for _, spaceType := range types {
			t.Run(spaceType, func(t *testing.T) {
				t.Parallel()

				cs := &model.ConfluenceSpace{
					UUID:         "conf-uuid-type",
					ConfluenceID: "555",
					Key:          "T",
					Name:         "Type Test",
					Type:         spaceType,
					Status:       "current",
				}

				resp := toConfluenceSpaceResponse(cs)

				if resp.Type != spaceType {
					t.Errorf("Type = %q, want %q", resp.Type, spaceType)
				}
			})
		}
	})

	t.Run("empty string fields preserved", func(t *testing.T) {
		t.Parallel()

		cs := &model.ConfluenceSpace{
			UUID:         "",
			ConfluenceID: "",
			Key:          "",
			Name:         "",
			Type:         "",
			Status:       "",
		}

		resp := toConfluenceSpaceResponse(cs)

		if resp.UUID != "" {
			t.Errorf("UUID = %q, want empty", resp.UUID)
		}
		if resp.Key != "" {
			t.Errorf("Key = %q, want empty", resp.Key)
		}
		if resp.Name != "" {
			t.Errorf("Name = %q, want empty", resp.Name)
		}
	})

	t.Run("description with special characters", func(t *testing.T) {
		t.Parallel()

		cs := &model.ConfluenceSpace{
			UUID:         "conf-uuid-special",
			ConfluenceID: "666",
			Key:          "SPEC",
			Name:         "Special",
			Type:         "global",
			Status:       "current",
			Description:  sql.NullString{String: `Docs with "quotes" & <html> chars`, Valid: true},
		}

		resp := toConfluenceSpaceResponse(cs)

		want := `Docs with "quotes" & <html> chars`
		if resp.Description != want {
			t.Errorf("Description = %q, want %q", resp.Description, want)
		}
	})
}

// ---------------------------------------------------------------------------
// ConfluenceHandler early-return path tests (nil confluenceClient)
// ---------------------------------------------------------------------------

func newTestConfluenceHandler() *ConfluenceHandler {
	return &ConfluenceHandler{
		repo:             nil,
		confluenceClient: nil,
		logger:           slog.New(slog.DiscardHandler),
	}
}

func TestConfluenceHandler_SearchPages_NilClient(t *testing.T) {
	t.Parallel()

	t.Run("returns 503 when confluence client not configured", func(t *testing.T) {
		t.Parallel()

		h := newTestConfluenceHandler()
		req := httptest.NewRequest(http.MethodGet, "/api/confluence/pages?query=test", http.NoBody)
		rr := httptest.NewRecorder()

		h.SearchPages(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusServiceUnavailable)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "Confluence integration not configured" {
			t.Errorf("message = %v, want 'Confluence integration not configured'", msg)
		}
	})
}

func TestConfluenceHandler_ReadPage_NilClient(t *testing.T) {
	t.Parallel()

	t.Run("returns 503 when confluence client not configured", func(t *testing.T) {
		t.Parallel()

		h := newTestConfluenceHandler()
		req := httptest.NewRequest(http.MethodGet, "/api/confluence/pages/12345", http.NoBody)
		req = chiContext(req, map[string]string{"pageId": "12345"})
		rr := httptest.NewRecorder()

		h.ReadPage(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusServiceUnavailable)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "Confluence integration not configured" {
			t.Errorf("message = %v, want 'Confluence integration not configured'", msg)
		}
	})
}

func TestNewConfluenceHandler(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	h := NewConfluenceHandler(nil, nil, logger)

	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if h.logger != logger {
		t.Error("logger not set correctly")
	}
	if h.confluenceClient != nil {
		t.Error("confluenceClient should be nil")
	}
}

func TestConfluenceHandler_SearchPages_MissingParams(t *testing.T) {
	t.Parallel()

	t.Run("returns 503 before checking params when client is nil", func(t *testing.T) {
		t.Parallel()

		h := newTestConfluenceHandler()
		// Even without query params, the nil client check comes first.
		req := httptest.NewRequest(http.MethodGet, "/api/confluence/pages", http.NoBody)
		rr := httptest.NewRecorder()

		h.SearchPages(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusServiceUnavailable)
		}
	})
}
