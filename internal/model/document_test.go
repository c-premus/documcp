package model

import (
	"database/sql"
	"testing"
)

func TestDocument_ParseMetadata(t *testing.T) {
	t.Parallel()

	t.Run("null metadata returns nil", func(t *testing.T) {
		t.Parallel()
		d := &Document{Metadata: sql.NullString{Valid: false}}
		var dest map[string]any
		if err := d.ParseMetadata(&dest); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dest != nil {
			t.Fatal("expected nil dest for null metadata")
		}
	})

	t.Run("valid JSON object", func(t *testing.T) {
		t.Parallel()
		d := &Document{Metadata: sql.NullString{
			String: `{"author":"Jane","pages":42}`,
			Valid:  true,
		}}
		var dest map[string]any
		if err := d.ParseMetadata(&dest); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dest["author"] != "Jane" {
			t.Errorf("author = %v, want Jane", dest["author"])
		}
		if dest["pages"] != float64(42) {
			t.Errorf("pages = %v, want 42", dest["pages"])
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		t.Parallel()
		d := &Document{Metadata: sql.NullString{String: `{bad`, Valid: true}}
		var dest map[string]any
		if err := d.ParseMetadata(&dest); err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})
}

func TestDocumentVersion_ParseMetadata(t *testing.T) {
	t.Parallel()

	t.Run("null metadata returns nil", func(t *testing.T) {
		t.Parallel()
		dv := &DocumentVersion{Metadata: sql.NullString{Valid: false}}
		var dest map[string]any
		if err := dv.ParseMetadata(&dest); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dest != nil {
			t.Fatal("expected nil dest")
		}
	})

	t.Run("valid JSON", func(t *testing.T) {
		t.Parallel()
		dv := &DocumentVersion{Metadata: sql.NullString{
			String: `{"version":2}`,
			Valid:  true,
		}}
		var dest map[string]any
		if err := dv.ParseMetadata(&dest); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dest["version"] != float64(2) {
			t.Errorf("version = %v, want 2", dest["version"])
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		t.Parallel()
		dv := &DocumentVersion{Metadata: sql.NullString{String: `bad`, Valid: true}}
		var dest map[string]any
		if err := dv.ParseMetadata(&dest); err == nil {
			t.Fatal("expected error")
		}
	})
}
