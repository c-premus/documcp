package html_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.999.haus/chris/DocuMCP-go/internal/extractor/html"
)

func TestHTMLExtractor_Extract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		htmlContent     string
		wantContains    []string
		wantNotContains []string
		wantTitle       string
		wantHasTitle    bool
		wantWordCount   int
	}{
		{
			name:         "simple HTML converts to markdown with text preserved",
			htmlContent:  `<html><body><p>Hello world</p></body></html>`,
			wantContains: []string{"Hello world"},
			wantHasTitle: false,
		},
		{
			name:         "title extracted from title tag",
			htmlContent:  `<html><head><title>My Page Title</title></head><body><p>Body text</p></body></html>`,
			wantContains: []string{"Body text"},
			wantTitle:    "My Page Title",
			wantHasTitle: true,
		},
		{
			name:            "script tags are stripped during sanitization",
			htmlContent:     `<html><body><p>Safe content</p><script>alert('xss')</script></body></html>`,
			wantContains:    []string{"Safe content"},
			wantNotContains: []string{"alert", "script", "xss"},
			wantHasTitle:    false,
		},
		{
			name:          "empty HTML produces empty content",
			htmlContent:   ``,
			wantWordCount: 0,
			wantHasTitle:  false,
		},
		{
			name:         "heading tags convert to markdown headings",
			htmlContent:  `<html><body><h1>Main Heading</h1><p>Paragraph</p></body></html>`,
			wantContains: []string{"Main Heading", "Paragraph"},
			wantHasTitle: false,
		},
		{
			name:         "title tag with extra whitespace is trimmed",
			htmlContent:  `<html><head><title>  Spaced Title  </title></head><body><p>text</p></body></html>`,
			wantTitle:    "Spaced Title",
			wantHasTitle: true,
		},
		{
			name:         "empty title tag does not produce title metadata",
			htmlContent:  `<html><head><title>   </title></head><body><p>text</p></body></html>`,
			wantHasTitle: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			filePath := filepath.Join(dir, "test.html")
			if err := os.WriteFile(filePath, []byte(tt.htmlContent), 0o644); err != nil {
				t.Fatalf("writing temp file: %v", err)
			}

			ext := html.New()
			result, err := ext.Extract(context.Background(), filePath)
			if err != nil {
				t.Fatalf("Extract() unexpected error: %v", err)
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(result.Content, want) {
					t.Errorf("Content does not contain %q, got: %q", want, result.Content)
				}
			}

			for _, notWant := range tt.wantNotContains {
				if strings.Contains(result.Content, notWant) {
					t.Errorf("Content should not contain %q, got: %q", notWant, result.Content)
				}
			}

			if tt.wantWordCount > 0 && result.WordCount != tt.wantWordCount {
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

func TestHTMLExtractor_Extract_NonExistentFile(t *testing.T) {
	t.Parallel()

	ext := html.New()
	_, err := ext.Extract(context.Background(), "/nonexistent/path/file.html")
	if err == nil {
		t.Fatal("Extract() expected error for non-existent file, got nil")
	}
}

func TestHTMLExtractor_Extract_CancelledContext(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.html")
	if err := os.WriteFile(filePath, []byte("<p>hello</p>"), 0o644); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ext := html.New()
	_, err := ext.Extract(ctx, filePath)
	if err == nil {
		t.Fatal("Extract() expected error for cancelled context, got nil")
	}
}

func TestHTMLExtractor_Supports(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mimeType string
		want     bool
	}{
		{"text/html", true},
		{"application/xhtml+xml", true},
		{"text/plain", false},
		{"application/pdf", false},
		{"", false},
	}

	ext := html.New()
	for _, tt := range tests {
		t.Run(tt.mimeType, func(t *testing.T) {
			t.Parallel()

			if got := ext.Supports(tt.mimeType); got != tt.want {
				t.Errorf("Supports(%q) = %v, want %v", tt.mimeType, got, tt.want)
			}
		})
	}
}
