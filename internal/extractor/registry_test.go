package extractor_test

import (
	"context"
	"testing"

	"git.999.haus/chris/DocuMCP-go/internal/extractor"
)

// mockExtractor is a simple test double that supports a fixed set of MIME types.
type mockExtractor struct {
	name      string
	mimeTypes []string
}

func (m *mockExtractor) Extract(_ context.Context, _ string) (*extractor.ExtractedContent, error) {
	return &extractor.ExtractedContent{Content: m.name + " content"}, nil
}

func (m *mockExtractor) Supports(mimeType string) bool {
	for _, mt := range m.mimeTypes {
		if mt == mimeType {
			return true
		}
	}
	return false
}

func TestRegistry_ForMIMEType(t *testing.T) {
	pdfExtractor := &mockExtractor{
		name:      "pdf",
		mimeTypes: []string{"application/pdf"},
	}
	htmlExtractor := &mockExtractor{
		name:      "html",
		mimeTypes: []string{"text/html"},
	}
	markdownExtractor := &mockExtractor{
		name:      "markdown",
		mimeTypes: []string{"text/markdown", "text/x-markdown"},
	}

	tests := []struct {
		name       string
		extractors []extractor.Extractor
		mimeType   string
		wantName   string
		wantErr    bool
	}{
		{
			name:       "returns correct extractor for application/pdf",
			extractors: []extractor.Extractor{pdfExtractor, htmlExtractor, markdownExtractor},
			mimeType:   "application/pdf",
			wantName:   "pdf",
			wantErr:    false,
		},
		{
			name:       "returns correct extractor for text/html",
			extractors: []extractor.Extractor{pdfExtractor, htmlExtractor, markdownExtractor},
			mimeType:   "text/html",
			wantName:   "html",
			wantErr:    false,
		},
		{
			name:       "returns correct extractor for text/markdown",
			extractors: []extractor.Extractor{pdfExtractor, htmlExtractor, markdownExtractor},
			mimeType:   "text/markdown",
			wantName:   "markdown",
			wantErr:    false,
		},
		{
			name:       "returns error for unsupported MIME type",
			extractors: []extractor.Extractor{pdfExtractor, htmlExtractor},
			mimeType:   "image/png",
			wantName:   "",
			wantErr:    true,
		},
		{
			name:       "returns error when registry is empty",
			extractors: []extractor.Extractor{},
			mimeType:   "text/plain",
			wantName:   "",
			wantErr:    true,
		},
		{
			name: "first matching extractor wins",
			extractors: []extractor.Extractor{
				&mockExtractor{name: "first-html", mimeTypes: []string{"text/html"}},
				&mockExtractor{name: "second-html", mimeTypes: []string{"text/html"}},
			},
			mimeType: "text/html",
			wantName: "first-html",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := extractor.NewRegistry(tt.extractors...)

			ext, err := registry.ForMIMEType(tt.mimeType)

			if tt.wantErr {
				if err == nil {
					t.Fatal("ForMIMEType() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("ForMIMEType() unexpected error: %v", err)
			}

			// Verify we got the right extractor by calling Extract and checking the content.
			content, err := ext.Extract(context.Background(), "")
			if err != nil {
				t.Fatalf("Extract() unexpected error: %v", err)
			}

			wantContent := tt.wantName + " content"
			if content.Content != wantContent {
				t.Errorf("Extract().Content = %q, want %q (wrong extractor returned)", content.Content, wantContent)
			}
		})
	}
}
