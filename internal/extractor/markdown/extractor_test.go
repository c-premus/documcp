package markdown_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"git.999.haus/chris/DocuMCP-go/internal/extractor/markdown"
)

func TestMarkdownExtractor_Extract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		content       string
		wantContent   string
		wantWordCount int
		wantTitle     string
		wantHasTitle  bool
	}{
		{
			name:          "pass-through content is returned unchanged",
			content:       "Hello, world!\n\nThis is a test.",
			wantContent:   "Hello, world!\n\nThis is a test.",
			wantWordCount: 6,
			wantHasTitle:  false,
		},
		{
			name:          "word count matches number of whitespace-separated tokens",
			content:       "one two three four five",
			wantWordCount: 5,
			wantHasTitle:  false,
		},
		{
			name:          "title extracted from first ATX heading",
			content:       "# My Document\n\nSome body text here.",
			wantContent:   "# My Document\n\nSome body text here.",
			wantWordCount: 7,
			wantTitle:     "My Document",
			wantHasTitle:  true,
		},
		{
			name:          "title extracted from heading with extra spaces",
			content:       "#   Spaced Title  \n\nBody.",
			wantTitle:     "Spaced Title",
			wantHasTitle:  true,
			wantWordCount: 4,
		},
		{
			name:          "empty file returns empty content and zero word count",
			content:       "",
			wantContent:   "",
			wantWordCount: 0,
			wantHasTitle:  false,
		},
		{
			name:          "file with no heading produces no title in metadata",
			content:       "Just some text without any heading.\n\nAnother paragraph.",
			wantWordCount: 8,
			wantHasTitle:  false,
		},
		{
			name:          "h2 heading is not treated as title",
			content:       "## Secondary Heading\n\nBody text.",
			wantWordCount: 5,
			wantHasTitle:  false,
		},
		{
			name:          "h3 heading is not treated as title",
			content:       "### Third Level\n\nBody.",
			wantWordCount: 4,
			wantHasTitle:  false,
		},
		{
			name:          "first h1 wins when multiple h1 headings exist",
			content:       "# First Title\n\n# Second Title\n\nBody.",
			wantTitle:     "First Title",
			wantHasTitle:  true,
			wantWordCount: 7,
		},
		{
			name:          "heading with only hash and space but no text is not a title",
			content:       "# \n\nBody text.",
			wantHasTitle:  false,
			wantWordCount: 3,
		},
		{
			name:          "title found on non-first line",
			content:       "Some preamble text.\n\n# Late Title\n\nBody.",
			wantTitle:     "Late Title",
			wantHasTitle:  true,
			wantWordCount: 7,
		},
		{
			name:          "content with unicode characters",
			content:       "# Ubersicht\n\nDies ist ein Test mit Umlauten.",
			wantTitle:     "Ubersicht",
			wantHasTitle:  true,
			wantWordCount: 8,
		},
		{
			name:          "whitespace-only file returns zero word count",
			content:       "   \n\n   \t\n",
			wantWordCount: 0,
			wantHasTitle:  false,
		},
		{
			name:          "content preserved exactly including trailing newline",
			content:       "Line one\nLine two\n",
			wantContent:   "Line one\nLine two\n",
			wantWordCount: 4,
			wantHasTitle:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			filePath := filepath.Join(dir, "test.md")
			if err := os.WriteFile(filePath, []byte(tt.content), 0o644); err != nil {
				t.Fatalf("writing temp file: %v", err)
			}

			ext := markdown.New()
			result, err := ext.Extract(context.Background(), filePath)
			if err != nil {
				t.Fatalf("Extract() unexpected error: %v", err)
			}

			if tt.wantContent != "" && result.Content != tt.wantContent {
				t.Errorf("Content = %q, want %q", result.Content, tt.wantContent)
			}

			if result.WordCount != tt.wantWordCount {
				t.Errorf("WordCount = %d, want %d", result.WordCount, tt.wantWordCount)
			}

			title, hasTitle := result.Metadata["title"]
			if hasTitle != tt.wantHasTitle {
				t.Errorf("has title = %v, want %v (metadata: %v)", hasTitle, tt.wantHasTitle, result.Metadata)
			}
			if tt.wantHasTitle {
				if got, ok := title.(string); !ok || got != tt.wantTitle {
					t.Errorf("title = %v, want %q", title, tt.wantTitle)
				}
			}
		})
	}
}

func TestMarkdownExtractor_Extract_NonExistentFile(t *testing.T) {
	t.Parallel()

	ext := markdown.New()
	_, err := ext.Extract(context.Background(), "/nonexistent/path/file.md")
	if err == nil {
		t.Fatal("Extract() expected error for non-existent file, got nil")
	}
}

func TestMarkdownExtractor_Extract_CanceledContext(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.md")
	if err := os.WriteFile(filePath, []byte("content"), 0o644); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	ext := markdown.New()
	_, err := ext.Extract(ctx, filePath)
	if err == nil {
		t.Fatal("Extract() expected error for canceled context, got nil")
	}
}

func TestMarkdownExtractor_Supports(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mimeType string
		want     bool
	}{
		{"text/markdown", true},
		{"text/plain", true},
		{"text/html", false},
		{"application/pdf", false},
		{"", false},
	}

	ext := markdown.New()
	for _, tt := range tests {
		t.Run(tt.mimeType, func(t *testing.T) {
			t.Parallel()

			if got := ext.Supports(tt.mimeType); got != tt.want {
				t.Errorf("Supports(%q) = %v, want %v", tt.mimeType, got, tt.want)
			}
		})
	}
}
