package mcphandler

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"testing"
)

func TestSubstituteVariables(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		variables      map[string]string
		wantContent    string
		wantUnresolved []string
	}{
		{
			name:           "no placeholders returns original content",
			content:        "Hello, world!",
			variables:      map[string]string{"name": "Go"},
			wantContent:    "Hello, world!",
			wantUnresolved: nil,
		},
		{
			name:           "single variable substituted",
			content:        "Hello, {{name}}!",
			variables:      map[string]string{"name": "Go"},
			wantContent:    "Hello, Go!",
			wantUnresolved: nil,
		},
		{
			name:           "multiple variables all resolved",
			content:        "{{greeting}}, {{name}}! Welcome to {{project}}.",
			variables:      map[string]string{"greeting": "Hello", "name": "Alice", "project": "DocuMCP"},
			wantContent:    "Hello, Alice! Welcome to DocuMCP.",
			wantUnresolved: nil,
		},
		{
			name:           "some variables unresolved",
			content:        "{{greeting}}, {{name}}! Project: {{project}}",
			variables:      map[string]string{"greeting": "Hi"},
			wantContent:    "Hi, {{name}}! Project: {{project}}",
			wantUnresolved: []string{"name", "project"},
		},
		{
			name:           "all variables unresolved",
			content:        "{{foo}} and {{bar}}",
			variables:      map[string]string{},
			wantContent:    "{{foo}} and {{bar}}",
			wantUnresolved: []string{"foo", "bar"},
		},
		{
			name:           "empty variables map",
			content:        "{{key}} placeholder",
			variables:      map[string]string{},
			wantContent:    "{{key}} placeholder",
			wantUnresolved: []string{"key"},
		},
		{
			name:           "duplicate placeholder counted once in unresolved",
			content:        "{{name}} is {{name}}",
			variables:      map[string]string{},
			wantContent:    "{{name}} is {{name}}",
			wantUnresolved: []string{"name"},
		},
		{
			name:           "duplicate placeholder substituted everywhere",
			content:        "{{name}} is {{name}}",
			variables:      map[string]string{"name": "Alice"},
			wantContent:    "Alice is Alice",
			wantUnresolved: nil,
		},
		{
			name:           "no variables returns original with empty unresolved",
			content:        "Plain text without any variables.",
			variables:      map[string]string{},
			wantContent:    "Plain text without any variables.",
			wantUnresolved: nil,
		},
		{
			name:           "empty content",
			content:        "",
			variables:      map[string]string{"key": "val"},
			wantContent:    "",
			wantUnresolved: nil,
		},
		{
			name:           "variable with underscores",
			content:        "Project: {{project_name}}, Desc: {{project_description}}",
			variables:      map[string]string{"project_name": "MyApp"},
			wantContent:    "Project: MyApp, Desc: {{project_description}}",
			wantUnresolved: []string{"project_description"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotContent, gotUnresolved := substituteVariables(tt.content, tt.variables)

			if gotContent != tt.wantContent {
				t.Errorf("content = %q, want %q", gotContent, tt.wantContent)
			}

			if len(gotUnresolved) != len(tt.wantUnresolved) {
				t.Fatalf("unresolved count = %d, want %d; got %v", len(gotUnresolved), len(tt.wantUnresolved), gotUnresolved)
			}
			for i, u := range gotUnresolved {
				if u != tt.wantUnresolved[i] {
					t.Errorf("unresolved[%d] = %q, want %q", i, u, tt.wantUnresolved[i])
				}
			}
		})
	}
}

func TestParseVariablesJSON(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    map[string]string
		wantErr bool
	}{
		{
			name:    "empty string returns empty map",
			raw:     "",
			want:    map[string]string{},
			wantErr: false,
		},
		{
			name:    "valid JSON decoded",
			raw:     `{"project_name":"DocuMCP","author":"Chris"}`,
			want:    map[string]string{"project_name": "DocuMCP", "author": "Chris"},
			wantErr: false,
		},
		{
			name:    "single key-value pair",
			raw:     `{"key":"value"}`,
			want:    map[string]string{"key": "value"},
			wantErr: false,
		},
		{
			name:    "empty JSON object",
			raw:     `{}`,
			want:    map[string]string{},
			wantErr: false,
		},
		{
			name:    "invalid JSON returns error",
			raw:     `{not json}`,
			want:    nil,
			wantErr: true,
		},
		{
			name:    "array JSON returns error",
			raw:     `["a","b"]`,
			want:    nil,
			wantErr: true,
		},
		{
			name:    "non-string values returns error",
			raw:     `{"key": 123}`,
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseVariablesJSON(tt.raw)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("map size = %d, want %d; got %v", len(got), len(tt.want), got)
			}
			for k, wantVal := range tt.want {
				gotVal, ok := got[k]
				if !ok {
					t.Errorf("missing key %q", k)
					continue
				}
				if gotVal != wantVal {
					t.Errorf("got[%q] = %q, want %q", k, gotVal, wantVal)
				}
			}
		})
	}
}

func TestBuildZip(t *testing.T) {
	t.Run("creates valid zip with entries", func(t *testing.T) {
		entries := []archiveEntry{
			{path: "README.md", content: "# Hello\n\nWelcome to the project."},
			{path: "src/main.go", content: "package main\n\nfunc main() {}"},
			{path: "docs/guide.md", content: "Guide content here."},
		}

		var buf bytes.Buffer
		if err := buildZip(&buf, entries); err != nil {
			t.Fatalf("buildZip error: %v", err)
		}

		// Verify the zip is valid and contains expected files.
		reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			t.Fatalf("opening zip reader: %v", err)
		}

		if len(reader.File) != len(entries) {
			t.Fatalf("zip file count = %d, want %d", len(reader.File), len(entries))
		}

		for i, f := range reader.File {
			if f.Name != entries[i].path {
				t.Errorf("file[%d].Name = %q, want %q", i, f.Name, entries[i].path)
			}

			rc, err := f.Open()
			if err != nil {
				t.Fatalf("opening zip entry %q: %v", f.Name, err)
			}
			data, err := io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				t.Fatalf("reading zip entry %q: %v", f.Name, err)
			}

			if string(data) != entries[i].content {
				t.Errorf("file[%d] content = %q, want %q", i, string(data), entries[i].content)
			}
		}
	})

	t.Run("creates valid zip with no entries", func(t *testing.T) {
		var buf bytes.Buffer
		if err := buildZip(&buf, nil); err != nil {
			t.Fatalf("buildZip error: %v", err)
		}

		reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			t.Fatalf("opening zip reader: %v", err)
		}

		if len(reader.File) != 0 {
			t.Errorf("zip file count = %d, want 0", len(reader.File))
		}
	})

	t.Run("handles empty content entries", func(t *testing.T) {
		entries := []archiveEntry{
			{path: ".gitkeep", content: ""},
		}

		var buf bytes.Buffer
		if err := buildZip(&buf, entries); err != nil {
			t.Fatalf("buildZip error: %v", err)
		}

		reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			t.Fatalf("opening zip reader: %v", err)
		}

		if len(reader.File) != 1 {
			t.Fatalf("zip file count = %d, want 1", len(reader.File))
		}

		rc, err := reader.File[0].Open()
		if err != nil {
			t.Fatalf("opening zip entry: %v", err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("reading zip entry: %v", err)
		}

		if len(data) != 0 {
			t.Errorf("content length = %d, want 0", len(data))
		}
	})
}

func TestBuildTarGz(t *testing.T) {
	t.Run("creates valid tar.gz with entries", func(t *testing.T) {
		entries := []archiveEntry{
			{path: "README.md", content: "# Hello\n\nWelcome."},
			{path: "src/main.go", content: "package main\n\nfunc main() {}"},
			{path: "config.yaml", content: "key: value\n"},
		}

		var buf bytes.Buffer
		if err := buildTarGz(&buf, entries); err != nil {
			t.Fatalf("buildTarGz error: %v", err)
		}

		// Decompress and read tar entries.
		gr, err := gzip.NewReader(&buf)
		if err != nil {
			t.Fatalf("creating gzip reader: %v", err)
		}
		defer func() { _ = gr.Close() }()

		tr := tar.NewReader(gr)
		var idx int
		for {
			hdr, err := tr.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatalf("reading tar header: %v", err)
			}

			if idx >= len(entries) {
				t.Fatalf("more tar entries than expected")
			}

			if hdr.Name != entries[idx].path {
				t.Errorf("entry[%d].Name = %q, want %q", idx, hdr.Name, entries[idx].path)
			}
			if hdr.Size != int64(len(entries[idx].content)) {
				t.Errorf("entry[%d].Size = %d, want %d", idx, hdr.Size, len(entries[idx].content))
			}
			if hdr.Mode != 0o600 {
				t.Errorf("entry[%d].Mode = %o, want %o", idx, hdr.Mode, 0o600)
			}

			data, err := io.ReadAll(tr)
			if err != nil {
				t.Fatalf("reading tar entry %q: %v", hdr.Name, err)
			}
			if string(data) != entries[idx].content {
				t.Errorf("entry[%d] content = %q, want %q", idx, string(data), entries[idx].content)
			}

			idx++
		}

		if idx != len(entries) {
			t.Errorf("tar entry count = %d, want %d", idx, len(entries))
		}
	})

	t.Run("creates valid tar.gz with no entries", func(t *testing.T) {
		var buf bytes.Buffer
		if err := buildTarGz(&buf, nil); err != nil {
			t.Fatalf("buildTarGz error: %v", err)
		}

		gr, err := gzip.NewReader(&buf)
		if err != nil {
			t.Fatalf("creating gzip reader: %v", err)
		}
		defer func() { _ = gr.Close() }()

		tr := tar.NewReader(gr)
		_, err = tr.Next()
		if !errors.Is(err, io.EOF) {
			t.Errorf("expected EOF for empty archive, got: %v", err)
		}
	})

	t.Run("handles entries with empty content", func(t *testing.T) {
		entries := []archiveEntry{
			{path: ".gitkeep", content: ""},
		}

		var buf bytes.Buffer
		if err := buildTarGz(&buf, entries); err != nil {
			t.Fatalf("buildTarGz error: %v", err)
		}

		gr, err := gzip.NewReader(&buf)
		if err != nil {
			t.Fatalf("creating gzip reader: %v", err)
		}
		defer func() { _ = gr.Close() }()

		tr := tar.NewReader(gr)
		hdr, err := tr.Next()
		if err != nil {
			t.Fatalf("reading tar header: %v", err)
		}

		if hdr.Name != ".gitkeep" {
			t.Errorf("Name = %q, want %q", hdr.Name, ".gitkeep")
		}
		if hdr.Size != 0 {
			t.Errorf("Size = %d, want 0", hdr.Size)
		}

		data, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("reading tar entry: %v", err)
		}
		if len(data) != 0 {
			t.Errorf("content length = %d, want 0", len(data))
		}
	})

	t.Run("handles entries with unicode content", func(t *testing.T) {
		entries := []archiveEntry{
			{path: "unicode.txt", content: "Hello, \n\n\t"},
		}

		var buf bytes.Buffer
		if err := buildTarGz(&buf, entries); err != nil {
			t.Fatalf("buildTarGz error: %v", err)
		}

		gr, err := gzip.NewReader(&buf)
		if err != nil {
			t.Fatalf("creating gzip reader: %v", err)
		}
		defer func() { _ = gr.Close() }()

		tr := tar.NewReader(gr)
		hdr, err := tr.Next()
		if err != nil {
			t.Fatalf("reading tar header: %v", err)
		}

		data, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("reading tar entry: %v", err)
		}

		wantContent := "Hello, \n\n\t"
		if string(data) != wantContent {
			t.Errorf("content = %q, want %q", string(data), wantContent)
		}
		if hdr.Size != int64(len(wantContent)) {
			t.Errorf("Size = %d, want %d", hdr.Size, len(wantContent))
		}
	})
}
