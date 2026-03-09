package model

import (
	"database/sql"
	"testing"
)

func TestSearchQuery_ParseFilters(t *testing.T) {
	t.Parallel()

	t.Run("null filters", func(t *testing.T) {
		t.Parallel()
		sq := &SearchQuery{Filters: sql.NullString{Valid: false}}
		var dest map[string]any
		if err := sq.ParseFilters(&dest); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dest != nil {
			t.Fatal("expected nil dest")
		}
	})

	t.Run("valid filters", func(t *testing.T) {
		t.Parallel()
		sq := &SearchQuery{Filters: sql.NullString{
			String: `{"file_type":"pdf","is_public":true}`,
			Valid:  true,
		}}
		var dest map[string]any
		if err := sq.ParseFilters(&dest); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dest["file_type"] != "pdf" {
			t.Errorf("file_type = %v, want pdf", dest["file_type"])
		}
		if dest["is_public"] != true {
			t.Errorf("is_public = %v, want true", dest["is_public"])
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		t.Parallel()
		sq := &SearchQuery{Filters: sql.NullString{String: `not-json`, Valid: true}}
		var dest map[string]any
		if err := sq.ParseFilters(&dest); err == nil {
			t.Fatal("expected error")
		}
	})
}
