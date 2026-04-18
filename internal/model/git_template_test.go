package model

import (
	"database/sql"
	"encoding/json"
	"testing"
)

func TestGitTemplate_ParseTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		tags    json.RawMessage
		want    []string
		wantErr bool
	}{
		{
			name: "null tags",
			tags: nil,
			want: nil,
		},
		{
			name: "empty array",
			tags: json.RawMessage(`[]`),
			want: []string{},
		},
		{
			name: "multiple tags",
			tags: json.RawMessage(`["go","template","docker"]`),
			want: []string{"go", "template", "docker"},
		},
		{
			name:    "invalid JSON",
			tags:    json.RawMessage(`not-json`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gt := &GitTemplate{Tags: tt.tags}
			got, err := gt.ParseTags()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.want == nil {
				if got != nil {
					t.Fatalf("got %v, want nil", got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d tags, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("tag[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestGitTemplate_ParseManifest(t *testing.T) {
	t.Parallel()

	t.Run("null manifest", func(t *testing.T) {
		t.Parallel()
		gt := &GitTemplate{Manifest: nil}
		var dest map[string]any
		if err := gt.ParseManifest(&dest); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dest != nil {
			t.Fatal("expected nil dest")
		}
	})

	t.Run("valid manifest", func(t *testing.T) {
		t.Parallel()
		gt := &GitTemplate{Manifest: json.RawMessage(`{"name":"my-template","version":"1.0"}`)}
		var dest map[string]any
		if err := gt.ParseManifest(&dest); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dest["name"] != "my-template" {
			t.Errorf("name = %v, want my-template", dest["name"])
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		t.Parallel()
		gt := &GitTemplate{Manifest: json.RawMessage(`{bad`)}
		var dest map[string]any
		if err := gt.ParseManifest(&dest); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestGitTemplateFile_ParseVariables(t *testing.T) {
	t.Parallel()

	t.Run("null variables", func(t *testing.T) {
		t.Parallel()
		f := &GitTemplateFile{Variables: sql.NullString{Valid: false}}
		var dest map[string]any
		if err := f.ParseVariables(&dest); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dest != nil {
			t.Fatal("expected nil dest")
		}
	})

	t.Run("valid variables", func(t *testing.T) {
		t.Parallel()
		f := &GitTemplateFile{Variables: sql.NullString{
			String: `{"PROJECT_NAME":"demo","PORT":"8080"}`,
			Valid:  true,
		}}
		var dest map[string]string
		if err := f.ParseVariables(&dest); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dest["PROJECT_NAME"] != "demo" {
			t.Errorf("PROJECT_NAME = %q, want demo", dest["PROJECT_NAME"])
		}
		if dest["PORT"] != "8080" {
			t.Errorf("PORT = %q, want 8080", dest["PORT"])
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		t.Parallel()
		f := &GitTemplateFile{Variables: sql.NullString{String: `bad`, Valid: true}}
		var dest map[string]any
		if err := f.ParseVariables(&dest); err == nil {
			t.Fatal("expected error")
		}
	})
}
