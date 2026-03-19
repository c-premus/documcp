package api

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"git.999.haus/chris/DocuMCP-go/internal/model"
)

// ---------------------------------------------------------------------------
// slugifyTemplateName tests
// ---------------------------------------------------------------------------

func TestSlugifyTemplateName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "simple lowercase name",
			in:   "hello world",
			want: "hello-world",
		},
		{
			name: "mixed case is lowered",
			in:   "Hello World",
			want: "hello-world",
		},
		{
			name: "hyphens preserved",
			in:   "my-template",
			want: "my-template",
		},
		{
			name: "underscores become hyphens",
			in:   "my_template_name",
			want: "my-template-name",
		},
		{
			name: "special characters removed",
			in:   "hello! @world #2024",
			want: "hello-world-2024",
		},
		{
			name: "combining accent removed but base letter kept",
			in:   "cafe\u0301 template",
			want: "cafe-template",
		},
		{
			name: "leading and trailing spaces trimmed",
			in:   "  my template  ",
			want: "my-template",
		},
		{
			name: "empty string",
			in:   "",
			want: "",
		},
		{
			name: "only spaces",
			in:   "   ",
			want: "",
		},
		{
			name: "only special characters",
			in:   "!@#$%^&*()",
			want: "",
		},
		{
			name: "consecutive spaces collapse to single hyphen",
			in:   "hello   world",
			want: "hello-world",
		},
		{
			name: "consecutive hyphens collapse",
			in:   "hello---world",
			want: "hello-world",
		},
		{
			name: "mixed separators collapse",
			in:   "hello - _ world",
			want: "hello-world",
		},
		{
			name: "numbers preserved",
			in:   "template 123",
			want: "template-123",
		},
		{
			name: "leading special chars trimmed",
			in:   "---hello",
			want: "hello",
		},
		{
			name: "trailing special chars trimmed",
			in:   "hello---",
			want: "hello",
		},
		{
			name: "long name with many words",
			in:   "This Is A Really Long Template Name With Many Words",
			want: "this-is-a-really-long-template-name-with-many-words",
		},
		{
			name: "single character",
			in:   "a",
			want: "a",
		},
		{
			name: "single digit",
			in:   "9",
			want: "9",
		},
		{
			name: "dots removed",
			in:   "docker.compose.yml",
			want: "dockercomposeyml",
		},
		{
			name: "CJK characters removed",
			in:   "hello \u4e16\u754c",
			want: "hello",
		},
		{
			name: "emoji removed",
			in:   "rocket \U0001f680 template",
			want: "rocket-template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := slugifyTemplateName(tt.in)
			if got != tt.want {
				t.Errorf("slugifyTemplateName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// substituteTemplateVariables tests
// ---------------------------------------------------------------------------

func TestSubstituteTemplateVariables(t *testing.T) {
	t.Parallel()

	t.Run("substitutes single variable", func(t *testing.T) {
		t.Parallel()

		content := "Hello, {{name}}!"
		vars := map[string]string{"name": "World"}

		got, unresolved := substituteTemplateVariables(content, vars)

		if got != "Hello, World!" {
			t.Errorf("content = %q, want %q", got, "Hello, World!")
		}
		if len(unresolved) != 0 {
			t.Errorf("unresolved = %v, want empty", unresolved)
		}
	})

	t.Run("substitutes multiple variables", func(t *testing.T) {
		t.Parallel()

		content := "{{greeting}}, {{name}}! Welcome to {{place}}."
		vars := map[string]string{
			"greeting": "Hello",
			"name":     "Alice",
			"place":    "Wonderland",
		}

		got, unresolved := substituteTemplateVariables(content, vars)

		want := "Hello, Alice! Welcome to Wonderland."
		if got != want {
			t.Errorf("content = %q, want %q", got, want)
		}
		if len(unresolved) != 0 {
			t.Errorf("unresolved = %v, want empty", unresolved)
		}
	})

	t.Run("tracks unresolved variables", func(t *testing.T) {
		t.Parallel()

		content := "{{found}} and {{missing}}"
		vars := map[string]string{"found": "yes"}

		got, unresolved := substituteTemplateVariables(content, vars)

		if got != "yes and {{missing}}" {
			t.Errorf("content = %q, want %q", got, "yes and {{missing}}")
		}
		if len(unresolved) != 1 || unresolved[0] != "missing" {
			t.Errorf("unresolved = %v, want [missing]", unresolved)
		}
	})

	t.Run("deduplicates unresolved variables", func(t *testing.T) {
		t.Parallel()

		content := "{{x}} and {{x}} and {{x}}"
		vars := map[string]string{}

		_, unresolved := substituteTemplateVariables(content, vars)

		if len(unresolved) != 1 {
			t.Errorf("unresolved count = %d, want 1 (deduped)", len(unresolved))
		}
		if len(unresolved) > 0 && unresolved[0] != "x" {
			t.Errorf("unresolved[0] = %q, want %q", unresolved[0], "x")
		}
	})

	t.Run("no variables in content", func(t *testing.T) {
		t.Parallel()

		content := "plain text with no placeholders"
		vars := map[string]string{"key": "value"}

		got, unresolved := substituteTemplateVariables(content, vars)

		if got != content {
			t.Errorf("content = %q, want %q", got, content)
		}
		if len(unresolved) != 0 {
			t.Errorf("unresolved = %v, want empty", unresolved)
		}
	})

	t.Run("empty content", func(t *testing.T) {
		t.Parallel()

		got, unresolved := substituteTemplateVariables("", map[string]string{"key": "val"})

		if got != "" {
			t.Errorf("content = %q, want empty", got)
		}
		if len(unresolved) != 0 {
			t.Errorf("unresolved = %v, want empty", unresolved)
		}
	})

	t.Run("empty variables map", func(t *testing.T) {
		t.Parallel()

		content := "{{a}} and {{b}}"
		got, unresolved := substituteTemplateVariables(content, map[string]string{})

		if got != content {
			t.Errorf("content = %q, want %q", got, content)
		}
		if len(unresolved) != 2 {
			t.Errorf("unresolved count = %d, want 2", len(unresolved))
		}
	})

	t.Run("replaces same variable multiple times in content", func(t *testing.T) {
		t.Parallel()

		content := "{{x}}-{{x}}-{{x}}"
		vars := map[string]string{"x": "A"}

		got, unresolved := substituteTemplateVariables(content, vars)

		if got != "A-A-A" {
			t.Errorf("content = %q, want %q", got, "A-A-A")
		}
		if len(unresolved) != 0 {
			t.Errorf("unresolved = %v, want empty", unresolved)
		}
	})

	t.Run("variable substituted with empty string", func(t *testing.T) {
		t.Parallel()

		content := "prefix-{{var}}-suffix"
		vars := map[string]string{"var": ""}

		got, unresolved := substituteTemplateVariables(content, vars)

		if got != "prefix--suffix" {
			t.Errorf("content = %q, want %q", got, "prefix--suffix")
		}
		if len(unresolved) != 0 {
			t.Errorf("unresolved = %v, want empty", unresolved)
		}
	})

	t.Run("mixed resolved and unresolved", func(t *testing.T) {
		t.Parallel()

		content := "host={{host}} port={{port}} db={{db}}"
		vars := map[string]string{"host": "localhost", "port": "5432"}

		got, unresolved := substituteTemplateVariables(content, vars)

		if got != "host=localhost port=5432 db={{db}}" {
			t.Errorf("content = %q, want %q", got, "host=localhost port=5432 db={{db}}")
		}
		if len(unresolved) != 1 || unresolved[0] != "db" {
			t.Errorf("unresolved = %v, want [db]", unresolved)
		}
	})
}

// ---------------------------------------------------------------------------
// parseTemplateVariablesJSON tests
// ---------------------------------------------------------------------------

func TestParseTemplateVariablesJSON(t *testing.T) {
	t.Parallel()

	t.Run("valid JSON with key-value pairs", func(t *testing.T) {
		t.Parallel()

		raw := `{"name":"Alice","age":"30"}`
		vars, err := parseTemplateVariablesJSON(raw)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if vars["name"] != "Alice" {
			t.Errorf("vars[name] = %q, want Alice", vars["name"])
		}
		if vars["age"] != "30" {
			t.Errorf("vars[age] = %q, want 30", vars["age"])
		}
	})

	t.Run("empty string returns empty map", func(t *testing.T) {
		t.Parallel()

		vars, err := parseTemplateVariablesJSON("")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if vars == nil {
			t.Fatal("vars should not be nil")
		}
		if len(vars) != 0 {
			t.Errorf("vars length = %d, want 0", len(vars))
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		t.Parallel()

		_, err := parseTemplateVariablesJSON("{not valid json}")

		if err == nil {
			t.Error("expected error for invalid JSON, got nil")
		}
	})

	t.Run("empty JSON object returns empty map", func(t *testing.T) {
		t.Parallel()

		vars, err := parseTemplateVariablesJSON("{}")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(vars) != 0 {
			t.Errorf("vars length = %d, want 0", len(vars))
		}
	})

	t.Run("nested JSON returns error", func(t *testing.T) {
		t.Parallel()

		raw := `{"key":{"nested":"value"}}`
		_, err := parseTemplateVariablesJSON(raw)

		if err == nil {
			t.Error("expected error for nested JSON, got nil")
		}
	})

	t.Run("JSON array returns error", func(t *testing.T) {
		t.Parallel()

		_, err := parseTemplateVariablesJSON(`["a","b"]`)

		if err == nil {
			t.Error("expected error for JSON array, got nil")
		}
	})

	t.Run("JSON with numeric value returns error", func(t *testing.T) {
		t.Parallel()

		_, err := parseTemplateVariablesJSON(`{"key":123}`)

		if err == nil {
			t.Error("expected error for non-string value, got nil")
		}
	})

	t.Run("single key-value pair", func(t *testing.T) {
		t.Parallel()

		vars, err := parseTemplateVariablesJSON(`{"host":"localhost"}`)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(vars) != 1 {
			t.Errorf("vars length = %d, want 1", len(vars))
		}
		if vars["host"] != "localhost" {
			t.Errorf("vars[host] = %q, want localhost", vars["host"])
		}
	})

	t.Run("values with special characters", func(t *testing.T) {
		t.Parallel()

		raw := `{"url":"https://example.com/path?q=1&r=2","path":"/usr/local/bin"}`
		vars, err := parseTemplateVariablesJSON(raw)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if vars["url"] != "https://example.com/path?q=1&r=2" {
			t.Errorf("vars[url] = %q, want https://example.com/path?q=1&r=2", vars["url"])
		}
		if vars["path"] != "/usr/local/bin" {
			t.Errorf("vars[path] = %q, want /usr/local/bin", vars["path"])
		}
	})
}

// ---------------------------------------------------------------------------
// toGitTemplateResponse tests
// ---------------------------------------------------------------------------

func TestToGitTemplateResponse(t *testing.T) {
	t.Parallel()

	t.Run("maps all required fields", func(t *testing.T) {
		t.Parallel()

		gt := &model.GitTemplate{
			UUID:           "uuid-123",
			Name:           "My Template",
			Slug:           "my-template",
			RepositoryURL:  "https://github.com/user/repo",
			Branch:         "main",
			IsPublic:       true,
			Status:         "synced",
			FileCount:      10,
			TotalSizeBytes: 2048,
		}

		resp := toGitTemplateResponse(gt)

		if resp.UUID != "uuid-123" {
			t.Errorf("UUID = %q, want uuid-123", resp.UUID)
		}
		if resp.Name != "My Template" {
			t.Errorf("Name = %q, want My Template", resp.Name)
		}
		if resp.Slug != "my-template" {
			t.Errorf("Slug = %q, want my-template", resp.Slug)
		}
		if resp.RepositoryURL != "https://github.com/user/repo" {
			t.Errorf("RepositoryURL = %q, want https://github.com/user/repo", resp.RepositoryURL)
		}
		if resp.Branch != "main" {
			t.Errorf("Branch = %q, want main", resp.Branch)
		}
		if !resp.IsPublic {
			t.Error("IsPublic = false, want true")
		}
		if resp.Status != "synced" {
			t.Errorf("Status = %q, want synced", resp.Status)
		}
		if resp.FileCount != 10 {
			t.Errorf("FileCount = %d, want 10", resp.FileCount)
		}
		if resp.TotalSizeBytes != 2048 {
			t.Errorf("TotalSizeBytes = %d, want 2048", resp.TotalSizeBytes)
		}
	})

	t.Run("maps optional NullString fields when valid", func(t *testing.T) {
		t.Parallel()

		gt := &model.GitTemplate{
			UUID:          "uuid-1",
			Name:          "Test",
			Slug:          "test",
			RepositoryURL: "https://example.com",
			Branch:        "main",
			Status:        "synced",
			Description:   sql.NullString{String: "A description", Valid: true},
			Category:      sql.NullString{String: "devops", Valid: true},
			ErrorMessage:  sql.NullString{String: "sync failed", Valid: true},
			LastCommitSHA: sql.NullString{String: "abc123def", Valid: true},
		}

		resp := toGitTemplateResponse(gt)

		if resp.Description != "A description" {
			t.Errorf("Description = %q, want A description", resp.Description)
		}
		if resp.Category != "devops" {
			t.Errorf("Category = %q, want devops", resp.Category)
		}
		if resp.ErrorMessage != "sync failed" {
			t.Errorf("ErrorMessage = %q, want sync failed", resp.ErrorMessage)
		}
		if resp.LastCommitSHA != "abc123def" {
			t.Errorf("LastCommitSHA = %q, want abc123def", resp.LastCommitSHA)
		}
	})

	t.Run("null optional fields produce empty strings", func(t *testing.T) {
		t.Parallel()

		gt := &model.GitTemplate{
			UUID:          "uuid-2",
			Name:          "Minimal",
			Slug:          "minimal",
			RepositoryURL: "https://example.com",
			Branch:        "main",
			Status:        "pending",
		}

		resp := toGitTemplateResponse(gt)

		if resp.Description != "" {
			t.Errorf("Description = %q, want empty", resp.Description)
		}
		if resp.Category != "" {
			t.Errorf("Category = %q, want empty", resp.Category)
		}
		if resp.ErrorMessage != "" {
			t.Errorf("ErrorMessage = %q, want empty", resp.ErrorMessage)
		}
		if resp.LastSyncedAt != "" {
			t.Errorf("LastSyncedAt = %q, want empty", resp.LastSyncedAt)
		}
		if resp.LastCommitSHA != "" {
			t.Errorf("LastCommitSHA = %q, want empty", resp.LastCommitSHA)
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

		now := time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC)
		gt := &model.GitTemplate{
			UUID:          "uuid-3",
			Name:          "Timed",
			Slug:          "timed",
			RepositoryURL: "https://example.com",
			Branch:        "main",
			Status:        "synced",
			LastSyncedAt:  sql.NullTime{Time: now, Valid: true},
			CreatedAt:     sql.NullTime{Time: now, Valid: true},
			UpdatedAt:     sql.NullTime{Time: now, Valid: true},
		}

		resp := toGitTemplateResponse(gt)
		want := "2025-06-15T14:30:00Z"

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

	t.Run("tags parsed from JSON when valid", func(t *testing.T) {
		t.Parallel()

		gt := &model.GitTemplate{
			UUID:          "uuid-4",
			Name:          "Tagged",
			Slug:          "tagged",
			RepositoryURL: "https://example.com",
			Branch:        "main",
			Status:        "synced",
			Tags:          sql.NullString{String: `["go","docker","k8s"]`, Valid: true},
		}

		resp := toGitTemplateResponse(gt)

		if len(resp.Tags) != 3 {
			t.Fatalf("Tags length = %d, want 3", len(resp.Tags))
		}
		if resp.Tags[0] != "go" || resp.Tags[1] != "docker" || resp.Tags[2] != "k8s" {
			t.Errorf("Tags = %v, want [go docker k8s]", resp.Tags)
		}
	})

	t.Run("null tags produce empty slice not nil", func(t *testing.T) {
		t.Parallel()

		gt := &model.GitTemplate{
			UUID:          "uuid-5",
			Name:          "No Tags",
			Slug:          "no-tags",
			RepositoryURL: "https://example.com",
			Branch:        "main",
			Status:        "synced",
		}

		resp := toGitTemplateResponse(gt)

		if resp.Tags == nil {
			t.Fatal("Tags should be empty slice, not nil")
		}
		if len(resp.Tags) != 0 {
			t.Errorf("Tags length = %d, want 0", len(resp.Tags))
		}
	})

	t.Run("invalid tags JSON falls back to empty slice", func(t *testing.T) {
		t.Parallel()

		gt := &model.GitTemplate{
			UUID:          "uuid-6",
			Name:          "Bad Tags",
			Slug:          "bad-tags",
			RepositoryURL: "https://example.com",
			Branch:        "main",
			Status:        "synced",
			Tags:          sql.NullString{String: "not json", Valid: true},
		}

		resp := toGitTemplateResponse(gt)

		if resp.Tags == nil {
			t.Fatal("Tags should be empty slice, not nil")
		}
		if len(resp.Tags) != 0 {
			t.Errorf("Tags length = %d, want 0 (fallback for invalid JSON)", len(resp.Tags))
		}
	})

	t.Run("zero file count and size", func(t *testing.T) {
		t.Parallel()

		gt := &model.GitTemplate{
			UUID:          "uuid-7",
			Name:          "Empty",
			Slug:          "empty",
			RepositoryURL: "https://example.com",
			Branch:        "main",
			Status:        "pending",
			FileCount:     0,
		}

		resp := toGitTemplateResponse(gt)

		if resp.FileCount != 0 {
			t.Errorf("FileCount = %d, want 0", resp.FileCount)
		}
		if resp.TotalSizeBytes != 0 {
			t.Errorf("TotalSizeBytes = %d, want 0", resp.TotalSizeBytes)
		}
	})
}

// ---------------------------------------------------------------------------
// buildTemplateArchiveZip tests
// ---------------------------------------------------------------------------

func TestBuildTemplateArchiveZip(t *testing.T) {
	t.Parallel()

	t.Run("creates valid zip with single entry", func(t *testing.T) {
		t.Parallel()

		entries := []templateArchiveEntry{
			{path: "README.md", content: "# Hello"},
		}

		var buf bytes.Buffer
		err := buildTemplateArchiveZip(&buf, entries)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			t.Fatalf("opening zip: %v", err)
		}

		if len(reader.File) != 1 {
			t.Fatalf("zip file count = %d, want 1", len(reader.File))
		}
		if reader.File[0].Name != "README.md" {
			t.Errorf("file name = %q, want README.md", reader.File[0].Name)
		}

		rc, err := reader.File[0].Open()
		if err != nil {
			t.Fatalf("opening file in zip: %v", err)
		}
		defer func() { _ = rc.Close() }()

		data, _ := io.ReadAll(rc)
		if string(data) != "# Hello" {
			t.Errorf("file content = %q, want %q", string(data), "# Hello")
		}
	})

	t.Run("creates zip with multiple entries", func(t *testing.T) {
		t.Parallel()

		entries := []templateArchiveEntry{
			{path: "file1.txt", content: "content1"},
			{path: "dir/file2.txt", content: "content2"},
			{path: "dir/sub/file3.txt", content: "content3"},
		}

		var buf bytes.Buffer
		err := buildTemplateArchiveZip(&buf, entries)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			t.Fatalf("opening zip: %v", err)
		}

		if len(reader.File) != 3 {
			t.Fatalf("zip file count = %d, want 3", len(reader.File))
		}

		wantNames := []string{"file1.txt", "dir/file2.txt", "dir/sub/file3.txt"}
		for i, f := range reader.File {
			if f.Name != wantNames[i] {
				t.Errorf("file[%d] name = %q, want %q", i, f.Name, wantNames[i])
			}
		}
	})

	t.Run("creates valid zip with empty entries", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		err := buildTemplateArchiveZip(&buf, []templateArchiveEntry{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			t.Fatalf("opening empty zip: %v", err)
		}

		if len(reader.File) != 0 {
			t.Errorf("zip file count = %d, want 0", len(reader.File))
		}
	})

	t.Run("handles entry with empty content", func(t *testing.T) {
		t.Parallel()

		entries := []templateArchiveEntry{
			{path: "empty.txt", content: ""},
		}

		var buf bytes.Buffer
		err := buildTemplateArchiveZip(&buf, entries)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			t.Fatalf("opening zip: %v", err)
		}

		rc, _ := reader.File[0].Open()
		defer func() { _ = rc.Close() }()

		data, _ := io.ReadAll(rc)
		if len(data) != 0 {
			t.Errorf("file content length = %d, want 0", len(data))
		}
	})

	t.Run("preserves large content", func(t *testing.T) {
		t.Parallel()

		bigContent := strings.Repeat("x", 100_000)
		entries := []templateArchiveEntry{
			{path: "big.txt", content: bigContent},
		}

		var buf bytes.Buffer
		err := buildTemplateArchiveZip(&buf, entries)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			t.Fatalf("opening zip: %v", err)
		}

		rc, _ := reader.File[0].Open()
		defer func() { _ = rc.Close() }()

		data, _ := io.ReadAll(rc)
		if len(data) != 100_000 {
			t.Errorf("file content length = %d, want 100000", len(data))
		}
	})
}

// ---------------------------------------------------------------------------
// buildTemplateArchiveTarGz tests
// ---------------------------------------------------------------------------

func TestBuildTemplateArchiveTarGz(t *testing.T) {
	t.Parallel()

	t.Run("creates valid tar.gz with single entry", func(t *testing.T) {
		t.Parallel()

		entries := []templateArchiveEntry{
			{path: "README.md", content: "# Hello"},
		}

		var buf bytes.Buffer
		err := buildTemplateArchiveTarGz(&buf, entries)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		gr, err := gzip.NewReader(&buf)
		if err != nil {
			t.Fatalf("opening gzip: %v", err)
		}
		defer func() { _ = gr.Close() }()

		tr := tar.NewReader(gr)
		hdr, err := tr.Next()
		if err != nil {
			t.Fatalf("reading tar header: %v", err)
		}

		if hdr.Name != "README.md" {
			t.Errorf("file name = %q, want README.md", hdr.Name)
		}
		if hdr.Mode != 0644 {
			t.Errorf("file mode = %o, want 0644", hdr.Mode)
		}
		if hdr.Size != int64(len("# Hello")) {
			t.Errorf("file size = %d, want %d", hdr.Size, len("# Hello"))
		}

		data, _ := io.ReadAll(tr)
		if string(data) != "# Hello" {
			t.Errorf("file content = %q, want %q", string(data), "# Hello")
		}

		// No more entries.
		_, err = tr.Next()
		if !errors.Is(err, io.EOF) {
			t.Errorf("expected EOF after single entry, got %v", err)
		}
	})

	t.Run("creates tar.gz with multiple entries", func(t *testing.T) {
		t.Parallel()

		entries := []templateArchiveEntry{
			{path: "file1.txt", content: "content1"},
			{path: "dir/file2.txt", content: "content2"},
		}

		var buf bytes.Buffer
		err := buildTemplateArchiveTarGz(&buf, entries)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		gr, err := gzip.NewReader(&buf)
		if err != nil {
			t.Fatalf("opening gzip: %v", err)
		}
		defer func() { _ = gr.Close() }()

		tr := tar.NewReader(gr)

		wantNames := []string{"file1.txt", "dir/file2.txt"}
		wantContents := []string{"content1", "content2"}

		for i := range wantNames {
			hdr, err := tr.Next()
			if err != nil {
				t.Fatalf("reading tar entry %d: %v", i, err)
			}
			if hdr.Name != wantNames[i] {
				t.Errorf("entry[%d] name = %q, want %q", i, hdr.Name, wantNames[i])
			}
			data, _ := io.ReadAll(tr)
			if string(data) != wantContents[i] {
				t.Errorf("entry[%d] content = %q, want %q", i, string(data), wantContents[i])
			}
		}
	})

	t.Run("creates valid tar.gz with empty entries", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		err := buildTemplateArchiveTarGz(&buf, []templateArchiveEntry{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		gr, err := gzip.NewReader(&buf)
		if err != nil {
			t.Fatalf("opening gzip: %v", err)
		}
		defer func() { _ = gr.Close() }()

		tr := tar.NewReader(gr)
		_, err = tr.Next()
		if !errors.Is(err, io.EOF) {
			t.Errorf("expected EOF for empty archive, got %v", err)
		}
	})

	t.Run("handles entry with empty content", func(t *testing.T) {
		t.Parallel()

		entries := []templateArchiveEntry{
			{path: "empty.txt", content: ""},
		}

		var buf bytes.Buffer
		err := buildTemplateArchiveTarGz(&buf, entries)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		gr, _ := gzip.NewReader(&buf)
		defer func() { _ = gr.Close() }()

		tr := tar.NewReader(gr)
		hdr, _ := tr.Next()
		if hdr.Size != 0 {
			t.Errorf("file size = %d, want 0", hdr.Size)
		}
	})

	t.Run("all entries have mode 0644", func(t *testing.T) {
		t.Parallel()

		entries := []templateArchiveEntry{
			{path: "a.txt", content: "a"},
			{path: "b.txt", content: "b"},
			{path: "c.txt", content: "c"},
		}

		var buf bytes.Buffer
		err := buildTemplateArchiveTarGz(&buf, entries)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		gr, _ := gzip.NewReader(&buf)
		defer func() { _ = gr.Close() }()

		tr := tar.NewReader(gr)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("reading tar header: %v", err)
			}
			if hdr.Mode != 0644 {
				t.Errorf("entry %q mode = %o, want 0644", hdr.Name, hdr.Mode)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// Mock implementation of gitTemplateRepo
// ---------------------------------------------------------------------------

type mockGitTemplateRepo struct {
	ListFn             func(ctx context.Context, category string, limit int) ([]model.GitTemplate, error)
	SearchFn           func(ctx context.Context, query, category string, limit int) ([]model.GitTemplate, error)
	FindByUUIDFn       func(ctx context.Context, uuid string) (*model.GitTemplate, error)
	CreateFn           func(ctx context.Context, tmpl *model.GitTemplate) error
	UpdateFn           func(ctx context.Context, tmpl *model.GitTemplate) error
	SoftDeleteFn       func(ctx context.Context, id int64) error
	FilesForTemplateFn func(ctx context.Context, templateID int64) ([]model.GitTemplateFile, error)
	FindFileByPathFn   func(ctx context.Context, templateID int64, path string) (*model.GitTemplateFile, error)
}

func (m *mockGitTemplateRepo) List(ctx context.Context, category string, limit int) ([]model.GitTemplate, error) {
	return m.ListFn(ctx, category, limit)
}

func (m *mockGitTemplateRepo) Search(ctx context.Context, query, category string, limit int) ([]model.GitTemplate, error) {
	return m.SearchFn(ctx, query, category, limit)
}

func (m *mockGitTemplateRepo) FindByUUID(ctx context.Context, uuid string) (*model.GitTemplate, error) {
	return m.FindByUUIDFn(ctx, uuid)
}

func (m *mockGitTemplateRepo) Create(ctx context.Context, tmpl *model.GitTemplate) error {
	return m.CreateFn(ctx, tmpl)
}

func (m *mockGitTemplateRepo) Update(ctx context.Context, tmpl *model.GitTemplate) error {
	return m.UpdateFn(ctx, tmpl)
}

func (m *mockGitTemplateRepo) SoftDelete(ctx context.Context, id int64) error {
	return m.SoftDeleteFn(ctx, id)
}

func (m *mockGitTemplateRepo) FilesForTemplate(ctx context.Context, templateID int64) ([]model.GitTemplateFile, error) {
	return m.FilesForTemplateFn(ctx, templateID)
}

func (m *mockGitTemplateRepo) FindFileByPath(ctx context.Context, templateID int64, path string) (*model.GitTemplateFile, error) {
	return m.FindFileByPathFn(ctx, templateID, path)
}

// ---------------------------------------------------------------------------
// GitTemplateHandler early-return path tests
// ---------------------------------------------------------------------------

func newTestGitTemplateHandler() *GitTemplateHandler {
	return &GitTemplateHandler{
		repo:   nil,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func newGitTemplateHandlerWithMock(repo *mockGitTemplateRepo) *GitTemplateHandler {
	return &GitTemplateHandler{
		repo:   repo,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestGitTemplateHandler_ReadFile_EmptyPath(t *testing.T) {
	t.Parallel()

	t.Run("returns 400 when file path is empty", func(t *testing.T) {
		t.Parallel()

		h := newTestGitTemplateHandler()
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates/uuid-1/files/", nil)
		req = chiContext(req, map[string]string{"uuid": "uuid-1", "*": ""})
		rr := httptest.NewRecorder()

		h.ReadFile(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "file path is required" {
			t.Errorf("message = %v, want 'file path is required'", msg)
		}
	})
}

func TestGitTemplateHandler_Create_Validation(t *testing.T) {
	t.Parallel()

	t.Run("returns 400 for invalid JSON body", func(t *testing.T) {
		t.Parallel()

		h := newTestGitTemplateHandler()
		req := httptest.NewRequest(http.MethodPost, "/api/git-templates", strings.NewReader("not json"))
		rr := httptest.NewRecorder()

		h.Create(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "invalid JSON body" {
			t.Errorf("message = %v, want 'invalid JSON body'", msg)
		}
	})

	t.Run("returns 400 when name is missing", func(t *testing.T) {
		t.Parallel()

		h := newTestGitTemplateHandler()
		req := httptest.NewRequest(http.MethodPost, "/api/git-templates",
			strings.NewReader(`{"repository_url":"https://github.com/user/repo"}`))
		rr := httptest.NewRecorder()

		h.Create(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "name is required" {
			t.Errorf("message = %v, want 'name is required'", msg)
		}
	})

	t.Run("returns 400 when repository_url is missing", func(t *testing.T) {
		t.Parallel()

		h := newTestGitTemplateHandler()
		req := httptest.NewRequest(http.MethodPost, "/api/git-templates",
			strings.NewReader(`{"name":"My Template"}`))
		rr := httptest.NewRecorder()

		h.Create(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "repository_url is required" {
			t.Errorf("message = %v, want 'repository_url is required'", msg)
		}
	})
}

func TestGitTemplateHandler_Download_Validation(t *testing.T) {
	t.Parallel()

	t.Run("returns 400 for invalid JSON body", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return &model.GitTemplate{ID: 1, UUID: "uuid-1", Name: "Test", Slug: "test"}, nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodPost, "/api/git-templates/uuid-1/download",
			strings.NewReader("not json"))
		req = chiContext(req, map[string]string{"uuid": "uuid-1"})
		rr := httptest.NewRecorder()

		h.Download(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})
}

func TestNewGitTemplateHandler(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewGitTemplateHandler(nil, logger)

	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if h.logger != logger {
		t.Error("logger not set correctly")
	}
}

// ---------------------------------------------------------------------------
// GitTemplateHandler.List tests
// ---------------------------------------------------------------------------

func TestGitTemplateHandler_List(t *testing.T) {
	t.Parallel()

	t.Run("returns templates with default limit", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			ListFn: func(_ context.Context, category string, limit int) ([]model.GitTemplate, error) {
				if category != "" {
					t.Errorf("category = %q, want empty", category)
				}
				if limit != 50 {
					t.Errorf("limit = %d, want 50", limit)
				}
				return []model.GitTemplate{
					{UUID: "t1", Name: "Template One", Slug: "template-one", RepositoryURL: "https://github.com/a/b", Branch: "main", Status: "synced"},
					{UUID: "t2", Name: "Template Two", Slug: "template-two", RepositoryURL: "https://github.com/c/d", Branch: "main", Status: "pending"},
				}, nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates", nil)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		data, ok := body["data"].([]any)
		if !ok {
			t.Fatal("data field missing or not an array")
		}
		if len(data) != 2 {
			t.Errorf("data length = %d, want 2", len(data))
		}

		meta := body["meta"].(map[string]any)
		if total := meta["total"].(float64); total != 2 {
			t.Errorf("meta.total = %v, want 2", total)
		}
	})

	t.Run("passes category filter and custom limit", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			ListFn: func(_ context.Context, category string, limit int) ([]model.GitTemplate, error) {
				if category != "devops" {
					t.Errorf("category = %q, want devops", category)
				}
				if limit != 10 {
					t.Errorf("limit = %d, want 10", limit)
				}
				return []model.GitTemplate{}, nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates?category=devops&limit=10", nil)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})

	t.Run("returns 500 when repo returns error", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			ListFn: func(_ context.Context, _ string, _ int) ([]model.GitTemplate, error) {
				return nil, errors.New("db connection lost")
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates", nil)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "failed to list git templates" {
			t.Errorf("message = %v, want 'failed to list git templates'", msg)
		}
	})

	t.Run("returns empty array when no templates exist", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			ListFn: func(_ context.Context, _ string, _ int) ([]model.GitTemplate, error) {
				return []model.GitTemplate{}, nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates", nil)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		data := body["data"].([]any)
		if len(data) != 0 {
			t.Errorf("data length = %d, want 0", len(data))
		}
	})

	t.Run("negative limit defaults to 50", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			ListFn: func(_ context.Context, _ string, limit int) ([]model.GitTemplate, error) {
				if limit != 50 {
					t.Errorf("limit = %d, want 50 (default)", limit)
				}
				return []model.GitTemplate{}, nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates?limit=-5", nil)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})
}

// ---------------------------------------------------------------------------
// GitTemplateHandler.Show tests
// ---------------------------------------------------------------------------

func TestGitTemplateHandler_Show(t *testing.T) {
	t.Parallel()

	t.Run("returns template when found", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, uuid string) (*model.GitTemplate, error) {
				if uuid != "tmpl-uuid-1" {
					t.Errorf("uuid = %q, want tmpl-uuid-1", uuid)
				}
				return &model.GitTemplate{
					UUID:          "tmpl-uuid-1",
					Name:          "My Template",
					Slug:          "my-template",
					RepositoryURL: "https://github.com/user/repo",
					Branch:        "main",
					Status:        "synced",
					FileCount:     5,
				}, nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates/tmpl-uuid-1", nil)
		req = chiContext(req, map[string]string{"uuid": "tmpl-uuid-1"})
		rr := httptest.NewRecorder()

		h.Show(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		data := body["data"].(map[string]any)
		if data["uuid"] != "tmpl-uuid-1" {
			t.Errorf("uuid = %v, want tmpl-uuid-1", data["uuid"])
		}
		if data["name"] != "My Template" {
			t.Errorf("name = %v, want My Template", data["name"])
		}
	})

	t.Run("returns 404 when template not found", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return nil, fmt.Errorf("finding template: %w", sql.ErrNoRows)
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates/missing", nil)
		req = chiContext(req, map[string]string{"uuid": "missing"})
		rr := httptest.NewRecorder()

		h.Show(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("returns 500 on unexpected error", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return nil, errors.New("connection reset")
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodGet, "/api/git-templates/uuid-1", nil)
		req = chiContext(req, map[string]string{"uuid": "uuid-1"})
		rr := httptest.NewRecorder()

		h.Show(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})
}

// ---------------------------------------------------------------------------
// GitTemplateHandler.Delete tests
// ---------------------------------------------------------------------------

func TestGitTemplateHandler_Delete(t *testing.T) {
	t.Parallel()

	t.Run("deletes template successfully", func(t *testing.T) {
		t.Parallel()

		var deletedID int64
		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return &model.GitTemplate{ID: 42, UUID: "del-uuid"}, nil
			},
			SoftDeleteFn: func(_ context.Context, id int64) error {
				deletedID = id
				return nil
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodDelete, "/api/git-templates/del-uuid", nil)
		req = chiContext(req, map[string]string{"uuid": "del-uuid"})
		rr := httptest.NewRecorder()

		h.Delete(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		if deletedID != 42 {
			t.Errorf("deleted ID = %d, want 42", deletedID)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if msg := body["message"]; msg != "Git template deleted successfully." {
			t.Errorf("message = %v, want 'Git template deleted successfully.'", msg)
		}
	})

	t.Run("returns 404 when template not found", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return nil, fmt.Errorf("not found: %w", sql.ErrNoRows)
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodDelete, "/api/git-templates/missing", nil)
		req = chiContext(req, map[string]string{"uuid": "missing"})
		rr := httptest.NewRecorder()

		h.Delete(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("returns 500 when soft delete fails", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return &model.GitTemplate{ID: 1, UUID: "uuid-1"}, nil
			},
			SoftDeleteFn: func(_ context.Context, _ int64) error {
				return errors.New("db write failed")
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodDelete, "/api/git-templates/uuid-1", nil)
		req = chiContext(req, map[string]string{"uuid": "uuid-1"})
		rr := httptest.NewRecorder()

		h.Delete(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})

	t.Run("returns 500 when find returns unexpected error", func(t *testing.T) {
		t.Parallel()

		repo := &mockGitTemplateRepo{
			FindByUUIDFn: func(_ context.Context, _ string) (*model.GitTemplate, error) {
				return nil, errors.New("timeout")
			},
		}
		h := newGitTemplateHandlerWithMock(repo)
		req := httptest.NewRequest(http.MethodDelete, "/api/git-templates/uuid-1", nil)
		req = chiContext(req, map[string]string{"uuid": "uuid-1"})
		rr := httptest.NewRecorder()

		h.Delete(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})
}

// ---------------------------------------------------------------------------
// GitTemplateHandler.ValidateURL tests
// ---------------------------------------------------------------------------

func TestGitTemplateHandler_ValidateURL(t *testing.T) {
	t.Parallel()

	t.Run("returns valid true for safe URL", func(t *testing.T) {
		t.Parallel()

		h := newTestGitTemplateHandler()
		req := httptest.NewRequest(http.MethodPost, "/api/admin/git-templates/validate-url",
			strings.NewReader(`{"url":"https://github.com/user/repo"}`))
		rr := httptest.NewRecorder()

		h.ValidateURL(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if valid := body["valid"].(bool); !valid {
			t.Error("valid = false, want true")
		}
	})

	t.Run("returns valid false for SSRF-blocked URL", func(t *testing.T) {
		t.Parallel()

		h := newTestGitTemplateHandler()
		req := httptest.NewRequest(http.MethodPost, "/api/admin/git-templates/validate-url",
			strings.NewReader(`{"url":"http://127.0.0.1:8080/secret"}`))
		rr := httptest.NewRecorder()

		h.ValidateURL(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if valid := body["valid"].(bool); valid {
			t.Error("valid = true, want false for localhost URL")
		}
		if _, hasErr := body["error"]; !hasErr {
			t.Error("expected error field in response for blocked URL")
		}
	})

	t.Run("returns 400 for invalid JSON", func(t *testing.T) {
		t.Parallel()

		h := newTestGitTemplateHandler()
		req := httptest.NewRequest(http.MethodPost, "/api/admin/git-templates/validate-url",
			strings.NewReader("not json"))
		rr := httptest.NewRecorder()

		h.ValidateURL(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("returns 400 when url is empty", func(t *testing.T) {
		t.Parallel()

		h := newTestGitTemplateHandler()
		req := httptest.NewRequest(http.MethodPost, "/api/admin/git-templates/validate-url",
			strings.NewReader(`{"url":""}`))
		rr := httptest.NewRecorder()

		h.ValidateURL(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("returns valid false for file scheme URL", func(t *testing.T) {
		t.Parallel()

		h := newTestGitTemplateHandler()
		req := httptest.NewRequest(http.MethodPost, "/api/admin/git-templates/validate-url",
			strings.NewReader(`{"url":"file:///etc/passwd"}`))
		rr := httptest.NewRecorder()

		h.ValidateURL(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if valid := body["valid"].(bool); valid {
			t.Error("valid = true, want false for file:// URL")
		}
	})
}
