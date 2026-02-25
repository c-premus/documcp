package docx_test

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.999.haus/chris/DocuMCP-go/internal/extractor/docx"
)

// wNS is the WordprocessingML namespace used in document.xml.
const wNS = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"

// createDocx builds a minimal DOCX file (ZIP archive) containing
// word/document.xml and optionally docProps/core.xml, then writes it to
// the returned file path inside t.TempDir().
func createDocx(t *testing.T, paragraphs []string, metadata *coreProps) string {
	t.Helper()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.docx")

	f, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	defer func() { _ = f.Close() }()

	zw := zip.NewWriter(f)
	defer func() { _ = zw.Close() }()

	// Write word/document.xml
	docXML := buildDocumentXML(paragraphs)
	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatalf("creating document.xml in zip: %v", err)
	}
	if _, err := w.Write([]byte(docXML)); err != nil {
		t.Fatalf("writing document.xml: %v", err)
	}

	// Optionally write docProps/core.xml
	if metadata != nil {
		coreXML := buildCoreXML(metadata)
		cw, err := zw.Create("docProps/core.xml")
		if err != nil {
			t.Fatalf("creating core.xml in zip: %v", err)
		}
		if _, err := cw.Write([]byte(coreXML)); err != nil {
			t.Fatalf("writing core.xml: %v", err)
		}
	}

	return filePath
}

// coreProps mirrors the metadata fields the docx extractor reads.
type coreProps struct {
	Title       string
	Creator     string
	Description string
}

// buildDocumentXML produces a minimal word/document.xml with <w:p>/<w:t> elements.
func buildDocumentXML(paragraphs []string) string {
	var b strings.Builder
	b.WriteString(xml.Header)
	b.WriteString(`<w:document xmlns:w="` + wNS + `"><w:body>`)
	for _, p := range paragraphs {
		b.WriteString(`<w:p><w:r><w:t>`)
		_ = xml.EscapeText(&b, []byte(p))
		b.WriteString(`</w:t></w:r></w:p>`)
	}
	b.WriteString(`</w:body></w:document>`)
	return b.String()
}

// buildCoreXML produces a minimal docProps/core.xml.
func buildCoreXML(props *coreProps) string {
	var b strings.Builder
	b.WriteString(xml.Header)
	b.WriteString(`<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/">`)
	if props.Title != "" {
		b.WriteString(`<dc:title>`)
		_ = xml.EscapeText(&b, []byte(props.Title))
		b.WriteString(`</dc:title>`)
	}
	if props.Creator != "" {
		b.WriteString(`<dc:creator>`)
		_ = xml.EscapeText(&b, []byte(props.Creator))
		b.WriteString(`</dc:creator>`)
	}
	if props.Description != "" {
		b.WriteString(`<dc:description>`)
		_ = xml.EscapeText(&b, []byte(props.Description))
		b.WriteString(`</dc:description>`)
	}
	b.WriteString(`</cp:coreProperties>`)
	return b.String()
}

func TestDOCXExtractor_Extract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		paragraphs    []string
		metadata      *coreProps
		wantContains  []string
		wantWordCount int
		wantTitle     string
		wantHasTitle  bool
	}{
		{
			name:          "single paragraph extracted",
			paragraphs:    []string{"Hello world from DOCX"},
			wantContains:  []string{"Hello world from DOCX"},
			wantWordCount: 4,
			wantHasTitle:  false,
		},
		{
			name:          "multiple paragraphs joined by double newlines",
			paragraphs:    []string{"First paragraph", "Second paragraph"},
			wantContains:  []string{"First paragraph", "Second paragraph"},
			wantWordCount: 4,
			wantHasTitle:  false,
		},
		{
			name:          "empty document produces empty content",
			paragraphs:    []string{},
			wantWordCount: 0,
			wantHasTitle:  false,
		},
		{
			name:       "metadata title extracted from core.xml",
			paragraphs: []string{"Body text"},
			metadata: &coreProps{
				Title:   "My Document Title",
				Creator: "Test Author",
			},
			wantContains:  []string{"Body text"},
			wantWordCount: 2,
			wantTitle:     "My Document Title",
			wantHasTitle:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			filePath := createDocx(t, tt.paragraphs, tt.metadata)

			ext := docx.New()
			result, err := ext.Extract(context.Background(), filePath)
			if err != nil {
				t.Fatalf("Extract() unexpected error: %v", err)
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(result.Content, want) {
					t.Errorf("Content does not contain %q, got: %q", want, result.Content)
				}
			}

			if result.WordCount != tt.wantWordCount {
				t.Errorf("WordCount = %d, want %d", result.WordCount, tt.wantWordCount)
			}

			if tt.wantHasTitle {
				title, ok := result.Metadata["title"]
				if !ok {
					t.Fatalf("expected title in metadata, got: %v", result.Metadata)
				}
				if got, ok := title.(string); !ok || got != tt.wantTitle {
					t.Errorf("title = %v, want %q", title, tt.wantTitle)
				}
			}
		})
	}
}

func TestDOCXExtractor_Extract_NonExistentFile(t *testing.T) {
	t.Parallel()

	ext := docx.New()
	_, err := ext.Extract(context.Background(), "/nonexistent/path/file.docx")
	if err == nil {
		t.Fatal("Extract() expected error for non-existent file, got nil")
	}
}

func TestDOCXExtractor_Extract_InvalidZip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "bad.docx")
	if err := os.WriteFile(filePath, []byte("this is not a zip file"), 0o644); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}

	ext := docx.New()
	_, err := ext.Extract(context.Background(), filePath)
	if err == nil {
		t.Fatal("Extract() expected error for invalid ZIP, got nil")
	}
}

func TestDOCXExtractor_Extract_CancelledContext(t *testing.T) {
	t.Parallel()

	filePath := createDocx(t, []string{"text"}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ext := docx.New()
	_, err := ext.Extract(ctx, filePath)
	if err == nil {
		t.Fatal("Extract() expected error for cancelled context, got nil")
	}
}

func TestDOCXExtractor_Supports(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mimeType string
		want     bool
	}{
		{"application/vnd.openxmlformats-officedocument.wordprocessingml.document", true},
		{"text/html", false},
		{"application/pdf", false},
		{"text/plain", false},
		{"", false},
	}

	ext := docx.New()
	for _, tt := range tests {
		t.Run(tt.mimeType, func(t *testing.T) {
			t.Parallel()

			if got := ext.Supports(tt.mimeType); got != tt.want {
				t.Errorf("Supports(%q) = %v, want %v", tt.mimeType, got, tt.want)
			}
		})
	}
}
