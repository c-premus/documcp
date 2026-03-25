package extractor_test

import (
	"strings"
	"testing"

	"github.com/c-premus/documcp/internal/extractor"
	"github.com/c-premus/documcp/internal/extractor/html"
	"github.com/c-premus/documcp/internal/extractor/markdown"
	"github.com/c-premus/documcp/internal/extractor/pdf"
)

func TestRegistry_ForMIMEType_WithRealExtractors(t *testing.T) {
	t.Parallel()

	mdExt := markdown.New()
	htmlExt := html.New()
	pdfExt := pdf.New()
	registry := extractor.NewRegistry(mdExt, htmlExt, pdfExt)

	tests := []struct {
		name     string
		mimeType string
		wantErr  bool
	}{
		{
			name:     "returns markdown extractor for text/markdown",
			mimeType: "text/markdown",
			wantErr:  false,
		},
		{
			name:     "returns markdown extractor for text/plain",
			mimeType: "text/plain",
			wantErr:  false,
		},
		{
			name:     "returns HTML extractor for text/html",
			mimeType: "text/html",
			wantErr:  false,
		},
		{
			name:     "returns HTML extractor for application/xhtml+xml",
			mimeType: "application/xhtml+xml",
			wantErr:  false,
		},
		{
			name:     "returns PDF extractor for application/pdf",
			mimeType: "application/pdf",
			wantErr:  false,
		},
		{
			name:     "returns error for unsupported MIME type",
			mimeType: "application/octet-stream",
			wantErr:  true,
		},
		{
			name:     "returns error for empty MIME type",
			mimeType: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ext, err := registry.ForMIMEType(tt.mimeType)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ForMIMEType(%q) expected error, got nil", tt.mimeType)
				}
				if !strings.Contains(err.Error(), tt.mimeType) {
					t.Errorf("error should mention MIME type %q, got: %v", tt.mimeType, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("ForMIMEType(%q) unexpected error: %v", tt.mimeType, err)
			}

			if !ext.Supports(tt.mimeType) {
				t.Errorf("returned extractor does not support %q", tt.mimeType)
			}
		})
	}
}

func TestRegistry_ForMIMEType_RealExtractors_ReturnsFirstMatch(t *testing.T) {
	t.Parallel()

	// Both markdown extractors support text/plain. The registry should
	// return the first one registered.
	first := markdown.New()
	second := markdown.New()
	registry := extractor.NewRegistry(first, second)

	ext, err := registry.ForMIMEType("text/plain")
	if err != nil {
		t.Fatalf("ForMIMEType() unexpected error: %v", err)
	}

	// Since both are *MarkdownExtractor we can compare pointers to verify
	// the first registered extractor is returned.
	if ext != first {
		t.Error("ForMIMEType() should return the first matching extractor")
	}
}
