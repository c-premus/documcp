// Package markdown extracts content from Markdown files.
// Since Markdown is the target format, this is a pass-through extractor
// that reads the file and returns its content unchanged.
package markdown

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/c-premus/documcp/internal/extractor"
)

// supportedMIMETypes lists the MIME types this extractor handles.
var supportedMIMETypes = map[string]bool{
	"text/markdown": true,
	"text/plain":    true,
}

// MarkdownExtractor reads Markdown files and returns their content as-is.
//
//nolint:revive // exported stutter is intentional; renaming would be a breaking change
type MarkdownExtractor struct{}

// New creates a new MarkdownExtractor.
func New() *MarkdownExtractor {
	return &MarkdownExtractor{}
}

// Extract reads the file at filePath and returns its content unchanged.
func (e *MarkdownExtractor) Extract(ctx context.Context, filePath string) (*extractor.ExtractedContent, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("markdown extract %q: %w", filePath, err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading markdown file %q: %w", filePath, err)
	}

	content := string(data)
	metadata := make(map[string]any)

	if title, ok := extractTitle(content); ok {
		metadata["title"] = title
	}

	return &extractor.ExtractedContent{
		Content:   content,
		Metadata:  metadata,
		WordCount: len(strings.Fields(content)),
	}, nil
}

// Supports reports whether this extractor can handle the given MIME type.
func (e *MarkdownExtractor) Supports(mimeType string) bool {
	return supportedMIMETypes[mimeType]
}

// extractTitle returns the text of the first ATX heading (# Title) found in
// the content. It returns false if no heading is present.
func extractTitle(content string) (string, bool) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if after, found := strings.CutPrefix(line, "# "); found {
			title := strings.TrimSpace(after)
			if title != "" {
				return title, true
			}
		}
	}
	return "", false
}
