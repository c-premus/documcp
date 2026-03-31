// Package pdf extracts text content from PDF files using pure Go libraries.
// No external tools (poppler-utils) or CGO required.
package pdf

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	lpdf "github.com/ledongthuc/pdf"
	pdfcpuapi "github.com/pdfcpu/pdfcpu/pkg/api"

	"github.com/c-premus/documcp/internal/extractor"
)

const (
	mimeTypePDF = "application/pdf"

	// maxExtractedTextSize is the maximum size of extracted text (50 MiB).
	maxExtractedTextSize = 50 * 1024 * 1024
)

// PDFExtractor extracts text from PDF files via pure Go libraries.
//
//nolint:revive // exported stutter is intentional; renaming would be a breaking change
type PDFExtractor struct{}

// New creates a new PDFExtractor.
func New() *PDFExtractor {
	return &PDFExtractor{}
}

// Supports reports whether this extractor handles the given MIME type.
func (e *PDFExtractor) Supports(mimeType string) bool {
	return mimeType == mimeTypePDF
}

// Extract reads the PDF at filePath and returns its text content and metadata.
// It uses ledongthuc/pdf for text extraction and pdfcpu for metadata.
func (e *PDFExtractor) Extract(ctx context.Context, filePath string) (*extractor.ExtractedContent, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context canceled before PDF extraction: %w", err)
	}

	text, err := extractText(filePath)
	if err != nil {
		return nil, fmt.Errorf("extracting PDF text: %w", err)
	}

	metadata := extractMetadata(filePath)

	return &extractor.ExtractedContent{
		Content:   text,
		Metadata:  metadata,
		WordCount: len(strings.Fields(text)),
	}, nil
}

// extractText uses ledongthuc/pdf to extract plain text from a PDF file.
func extractText(filePath string) (string, error) {
	f, r, err := lpdf.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("opening PDF: %w", err)
	}
	defer func() { _ = f.Close() }()

	reader, err := r.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("reading PDF text: %w", err)
	}

	b, err := io.ReadAll(io.LimitReader(reader, maxExtractedTextSize+1))
	if err != nil {
		return "", fmt.Errorf("reading PDF text output: %w", err)
	}
	if len(b) > maxExtractedTextSize {
		return "", fmt.Errorf("extracted PDF text exceeds %d bytes limit", maxExtractedTextSize)
	}

	return string(b), nil
}

// extractMetadata uses pdfcpu to read PDF document properties.
// It returns a best-effort result — if metadata extraction fails,
// an empty map is returned.
func extractMetadata(filePath string) map[string]any {
	metadata := make(map[string]any)

	f, err := os.Open(filePath)
	if err != nil {
		return metadata
	}
	defer func() { _ = f.Close() }()

	props, err := pdfcpuapi.Properties(f, nil)
	if err != nil {
		return metadata
	}

	knownKeys := []string{"Title", "Author", "Pages", "Creator", "Producer"}
	for _, key := range knownKeys {
		if v, ok := props[key]; ok && v != "" {
			metadata[key] = v
		}
	}

	return metadata
}
