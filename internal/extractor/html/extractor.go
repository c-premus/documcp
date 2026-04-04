// Package html extracts text content from HTML files by sanitizing and
// converting them to Markdown.
package html //nolint:revive // package name matches directory convention; internal package shadowing is acceptable

import (
	"context"
	"fmt"
	"os"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/microcosm-cc/bluemonday"
	nethtml "golang.org/x/net/html"

	"github.com/c-premus/documcp/internal/extractor"
)

// supportedMIMETypes lists the MIME types this extractor handles.
var supportedMIMETypes = map[string]bool{
	"text/html":             true,
	"application/xhtml+xml": true,
}

// HTMLExtractor extracts text content from HTML files.
//
//nolint:revive // exported stutter is intentional; renaming would be a breaking change
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
	if title := extractTitle(htmlContent); title != "" {
		metadata["title"] = title
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

// extractTitle parses HTML and returns the text content of the first <title>
// element. Returns empty string if no title is found or parsing fails.
func extractTitle(htmlContent string) string {
	doc, err := nethtml.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return ""
	}

	var title string
	var find func(*nethtml.Node) bool
	find = func(n *nethtml.Node) bool {
		if n.Type == nethtml.ElementNode && n.Data == "title" {
			var buf strings.Builder
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == nethtml.TextNode {
					buf.WriteString(c.Data)
				}
			}
			title = strings.TrimSpace(buf.String())
			return true
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if find(c) {
				return true
			}
		}
		return false
	}
	find(doc)

	return title
}
