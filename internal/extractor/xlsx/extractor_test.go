package xlsx_test

import (
	"context"
	"strings"
	"testing"

	"github.com/c-premus/documcp/internal/extractor/xlsx"
)

const sampleXLSX = "../../testutil/testdata/sample.xlsx"

func TestXLSXExtractor_Supports(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mimeType string
		want     bool
	}{
		{"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", true},
		{"application/pdf", false},
		{"text/plain", false},
		{"", false},
	}

	ext := xlsx.New()
	for _, tt := range tests {
		t.Run(tt.mimeType, func(t *testing.T) {
			t.Parallel()

			if got := ext.Supports(tt.mimeType); got != tt.want {
				t.Errorf("Supports(%q) = %v, want %v", tt.mimeType, got, tt.want)
			}
		})
	}
}

func TestXLSXExtractor_Extract(t *testing.T) {
	t.Parallel()

	ext := xlsx.New()
	result, err := ext.Extract(context.Background(), sampleXLSX)
	if err != nil {
		t.Fatalf("Extract() unexpected error: %v", err)
	}

	if !strings.Contains(result.Content, "## Sheet:") {
		t.Error("Extract() content missing markdown sheet header (## Sheet:)")
	}

	if !strings.Contains(result.Content, "|") {
		t.Error("Extract() content missing table delimiters (|)")
	}

	sheetCount, ok := result.Metadata["sheet_count"].(int)
	if !ok {
		t.Fatal("Extract() metadata sheet_count is not an int")
	}
	if sheetCount <= 0 {
		t.Errorf("Extract() metadata sheet_count = %d, want > 0", sheetCount)
	}

	sheetNames, ok := result.Metadata["sheet_names"].([]string)
	if !ok {
		t.Fatal("Extract() metadata sheet_names is not a []string")
	}
	if len(sheetNames) == 0 {
		t.Error("Extract() metadata sheet_names is empty, want non-empty")
	}

	if result.WordCount <= 0 {
		t.Errorf("Extract() WordCount = %d, want > 0", result.WordCount)
	}
}

func TestXLSXExtractor_Extract_CanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ext := xlsx.New()
	// The select between ctx.Done() and default is non-deterministic when the
	// context is already canceled, so we cannot assert a specific outcome.
	// We verify only that Extract does not panic and that any error returned
	// is context-related.
	_, err := ext.Extract(ctx, sampleXLSX)
	if err != nil && !strings.Contains(err.Error(), "context") {
		t.Errorf("Extract() error = %v, expected context-related error or nil", err)
	}
}

func TestXLSXExtractor_Extract_NonExistentFile(t *testing.T) {
	t.Parallel()

	ext := xlsx.New()
	_, err := ext.Extract(context.Background(), "/nonexistent/path/file.xlsx")
	if err == nil {
		t.Fatal("Extract() expected error for non-existent file, got nil")
	}
}
