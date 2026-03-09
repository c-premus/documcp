package model

import (
	"database/sql"
	"testing"
)

func TestExternalService_ParseConfig(t *testing.T) {
	t.Parallel()

	t.Run("null config", func(t *testing.T) {
		t.Parallel()
		es := &ExternalService{Config: sql.NullString{Valid: false}}
		var dest map[string]any
		if err := es.ParseConfig(&dest); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dest != nil {
			t.Fatal("expected nil dest")
		}
	})

	t.Run("valid config", func(t *testing.T) {
		t.Parallel()
		es := &ExternalService{Config: sql.NullString{
			String: `{"timeout":30,"retries":3}`,
			Valid:  true,
		}}
		var dest map[string]any
		if err := es.ParseConfig(&dest); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dest["timeout"] != float64(30) {
			t.Errorf("timeout = %v, want 30", dest["timeout"])
		}
		if dest["retries"] != float64(3) {
			t.Errorf("retries = %v, want 3", dest["retries"])
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		t.Parallel()
		es := &ExternalService{Config: sql.NullString{String: `bad-json`, Valid: true}}
		var dest map[string]any
		if err := es.ParseConfig(&dest); err == nil {
			t.Fatal("expected error")
		}
	})
}
