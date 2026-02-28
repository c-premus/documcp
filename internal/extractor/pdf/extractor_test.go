package pdf_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
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

func TestPDFExtractor_Extract_SamplePDF(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("pdftotext not available")
	}

	_, thisFile, _, _ := runtime.Caller(0)
	fixturePath := filepath.Join(filepath.Dir(thisFile), "..", "..", "testutil", "testdata", "sample.pdf")

	ext := pdf.New()
	result, err := ext.Extract(context.Background(), fixturePath)
	if err != nil {
		t.Fatalf("Extract() unexpected error: %v", err)
	}

	if result.Content == "" {
		t.Error("Extract() Content is empty, want non-empty")
	}

	if result.WordCount <= 0 {
		t.Errorf("Extract() WordCount = %d, want > 0", result.WordCount)
	}

	if result.Metadata == nil {
		t.Fatal("Extract() Metadata is nil, want non-nil map")
	}

	if pages, ok := result.Metadata["Pages"]; ok {
		pagesStr, isString := pages.(string)
		if !isString {
			t.Errorf("Extract() Metadata[\"Pages\"] type = %T, want string", pages)
		} else if pagesStr == "" {
			t.Error("Extract() Metadata[\"Pages\"] is empty, want non-empty string")
		}
	}
}
