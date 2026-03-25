// Package pdf extracts text content from PDF files using pdftotext (poppler-utils).
// It requires no CGO — all PDF processing is done by shelling out to external tools.
package pdf

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/c-premus/documcp/internal/extractor"
)

const mimeTypePDF = "application/pdf"

// PDFExtractor extracts text from PDF files via the pdftotext CLI tool.
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
// It shells out to pdftotext for text extraction and pdfinfo for metadata.
// The provided context controls the timeout of both commands.
func (e *PDFExtractor) Extract(ctx context.Context, filePath string) (*extractor.ExtractedContent, error) {
	text, err := extractText(ctx, filePath)
	if err != nil {
		return nil, fmt.Errorf("extracting PDF text: %w", err)
	}

	metadata := extractMetadata(ctx, filePath)

	return &extractor.ExtractedContent{
		Content:   text,
		Metadata:  metadata,
		WordCount: len(strings.Fields(text)),
	}, nil
}

// extractText runs pdftotext to convert the PDF to plain text.
func extractText(ctx context.Context, filePath string) (string, error) {
	pdftotextPath, err := exec.LookPath("pdftotext")
	if err != nil {
		return "", fmt.Errorf("pdftotext not found; install poppler-utils: %w", err)
	}

	cmd := exec.CommandContext(ctx, pdftotextPath, "-layout", filePath, "-")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("running pdftotext: %s: %w", strings.TrimSpace(stderr.String()), err)
	}

	return stdout.String(), nil
}

// extractMetadata runs pdfinfo and parses known metadata fields.
// It returns a best-effort result — if pdfinfo is unavailable or fails,
// an empty map is returned.
func extractMetadata(ctx context.Context, filePath string) map[string]any {
	metadata := make(map[string]any)

	pdfinfoPath, err := exec.LookPath("pdfinfo")
	if err != nil {
		return metadata
	}

	cmd := exec.CommandContext(ctx, pdfinfoPath, filePath)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return metadata
	}

	knownKeys := map[string]bool{
		"Title":    true,
		"Author":   true,
		"Pages":    true,
		"Creator":  true,
		"Producer": true,
	}

	scanner := bufio.NewScanner(&stdout)
	for scanner.Scan() {
		line := scanner.Text()
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if knownKeys[key] && value != "" {
			metadata[key] = value
		}
	}

	return metadata
}
