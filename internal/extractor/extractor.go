// Package extractor provides content extraction from various file formats.
// Each format has its own sub-package implementing the Extractor interface.
package extractor

import "context"

// ExtractedContent holds the result of extracting text from a file.
type ExtractedContent struct {
	Content   string         `json:"content"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	WordCount int            `json:"word_count"`
}

// Extractor extracts text content from a file.
type Extractor interface {
	// Extract reads the file at filePath and returns its text content.
	Extract(ctx context.Context, filePath string) (*ExtractedContent, error)

	// Supports reports whether this extractor can handle the given MIME type.
	Supports(mimeType string) bool
}
