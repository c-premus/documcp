package pdf_test

import (
	"context"
	"testing"

	"git.999.haus/chris/DocuMCP-go/internal/extractor/pdf"
)

func TestPDFExtractor_Supports(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mimeType string
		want     bool
	}{
		{"application/pdf", true},
		{"text/html", false},
		{"text/plain", false},
		{"text/markdown", false},
		{"application/vnd.openxmlformats-officedocument.wordprocessingml.document", false},
		{"", false},
	}

	ext := pdf.New()
	for _, tt := range tests {
		t.Run(tt.mimeType, func(t *testing.T) {
			t.Parallel()

			if got := ext.Supports(tt.mimeType); got != tt.want {
				t.Errorf("Supports(%q) = %v, want %v", tt.mimeType, got, tt.want)
			}
		})
	}
}

func TestPDFExtractor_Extract_NonExistentFile(t *testing.T) {
	t.Parallel()

	ext := pdf.New()
	_, err := ext.Extract(context.Background(), "/nonexistent/path/file.pdf")
	if err == nil {
		t.Fatal("Extract() expected error for non-existent file, got nil")
	}
}

func TestPDFExtractor_Extract_CancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ext := pdf.New()
	_, err := ext.Extract(ctx, "/any/path/file.pdf")
	// The PDF extractor does not check ctx.Err() upfront like the others;
	// instead the cancelled context propagates through exec.CommandContext.
	// Either way, we expect an error.
	if err == nil {
		t.Fatal("Extract() expected error for cancelled context, got nil")
	}
}
