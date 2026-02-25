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
		{
			name:            "style tags are stripped during sanitization",
			htmlContent:     `<html><body><style>body{color:red}</style><p>Styled text</p></body></html>`,
			wantContains:    []string{"Styled text"},
			wantNotContains: []string{"color:red", "style"},
			wantHasTitle:    false,
		},
		{
			name:            "inline event handlers are stripped",
			htmlContent:     `<html><body><p onclick="alert('xss')">Click me</p></body></html>`,
			wantContains:    []string{"Click me"},
			wantNotContains: []string{"onclick", "alert"},
			wantHasTitle:    false,
		},
		{
			name:         "anchor tags preserve link text",
			htmlContent:  `<html><body><a href="https://example.com">Example Link</a></body></html>`,
			wantContains: []string{"Example Link"},
			wantHasTitle: false,
		},
		{
			name:         "unordered list items are preserved",
			htmlContent:  `<html><body><ul><li>First</li><li>Second</li><li>Third</li></ul></body></html>`,
			wantContains: []string{"First", "Second", "Third"},
			wantHasTitle: false,
		},
		{
			name:         "nested tags preserve innermost text",
			htmlContent:  `<html><body><p><strong><em>Bold italic</em></strong></p></body></html>`,
			wantContains: []string{"Bold italic"},
			wantHasTitle: false,
		},
		{
			name:          "whitespace-only body produces zero word count",
			htmlContent:   `<html><body>   </body></html>`,
			wantWordCount: 0,
			wantHasTitle:  false,
		},
		{
			name:         "title with HTML entities",
			htmlContent:  `<html><head><title>Tom &amp; Jerry</title></head><body><p>Cartoon</p></body></html>`,
			wantTitle:    "Tom &amp; Jerry",
			wantHasTitle: true,
		},
		{
			name:         "case-insensitive title tag matching",
			htmlContent:  `<html><head><TITLE>Upper Title</TITLE></head><body><p>text</p></body></html>`,
			wantTitle:    "Upper Title",
			wantHasTitle: true,
		},
		{
			name:          "word count counts correctly for multi-paragraph HTML",
			htmlContent:   `<html><body><p>one two</p><p>three four five</p></body></html>`,
			wantContains:  []string{"one two", "three four five"},
			wantWordCount: 5,
			wantHasTitle:  false,
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
