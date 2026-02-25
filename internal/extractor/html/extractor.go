// Package html extracts text content from HTML files by sanitizing and
// converting them to Markdown.
package html

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/microcosm-cc/bluemonday"

	"git.999.haus/chris/DocuMCP-go/internal/extractor"
)

// supportedMIMETypes lists the MIME types this extractor handles.
var supportedMIMETypes = map[string]bool{
	"text/html":             true,
	"application/xhtml+xml": true,
}

// titleRe matches the content of an HTML <title> tag.
var titleRe = regexp.MustCompile(`(?i)<title[^>]*>(.*?)</title>`)

// HTMLExtractor extracts text content from HTML files.
type HTMLExtractor struct {
	policy *bluemonday.Policy
}

// New creates an HTMLExtractor with a UGC sanitization policy.
func New() *HTMLExtractor {
	return &HTMLExtractor{
		policy: bluemonday.UGCPolicy(),
	}
}

// Supports reports whether this extractor can handle the given MIME type.
func (e *HTMLExtractor) Supports(mimeType string) bool {
	return supportedMIMETypes[mimeType]
}

// Extract reads the HTML file at filePath, sanitizes it, converts it to
// Markdown, and returns the extracted content with word count and metadata.
func (e *HTMLExtractor) Extract(ctx context.Context, filePath string) (*extractor.ExtractedContent, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("html extract: %w", err)
	}

	raw, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("html extract: reading file: %w", err)
	}

	htmlContent := string(raw)

	// Extract title before sanitization since bluemonday strips <title>.
	metadata := make(map[string]any)
	if match := titleRe.FindStringSubmatch(htmlContent); len(match) > 1 {
		title := strings.TrimSpace(match[1])
		if title != "" {
			metadata["title"] = title
		}
	}

	sanitized := e.policy.Sanitize(htmlContent)

	markdown, err := htmltomarkdown.ConvertString(sanitized)
	if err != nil {
		return nil, fmt.Errorf("html extract: converting to markdown: %w", err)
	}

	markdown = strings.TrimSpace(markdown)
	wordCount := len(strings.Fields(markdown))

	return &extractor.ExtractedContent{
		Content:   markdown,
		Metadata:  metadata,
		WordCount: wordCount,
	}, nil
}
