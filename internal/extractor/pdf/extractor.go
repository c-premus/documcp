// Package pdf extracts text content from PDF files using pure Go libraries.
// No external tools (poppler-utils) or CGO required.
package pdf

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	lpdf "github.com/ledongthuc/pdf"
	pdfcpuapi "github.com/pdfcpu/pdfcpu/pkg/api"

	"github.com/c-premus/documcp/internal/extractor"
)

const (
	mimeTypePDF = "application/pdf"

	// defaultMaxExtractedTextSize is the maximum size of extracted text (50 MiB).
	defaultMaxExtractedTextSize = 50 * 1024 * 1024
)

// PDFExtractor extracts text from PDF files via pure Go libraries.
//
//nolint:revive // exported stutter is intentional; renaming would be a breaking change
type PDFExtractor struct {
	maxExtractedTextSize int64
}

// New creates a new PDFExtractor with default limits.
func New() *PDFExtractor {
	return &PDFExtractor{
		maxExtractedTextSize: defaultMaxExtractedTextSize,
	}
}

// NewWithLimits creates a PDFExtractor with configurable limits.
// Zero values fall back to defaults.
func NewWithLimits(maxExtractedText int64) *PDFExtractor {
	e := New()
	if maxExtractedText > 0 {
		e.maxExtractedTextSize = maxExtractedText
	}
	return e
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

	text, err := extractText(filePath, e.maxExtractedTextSize)
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
func extractText(filePath string, maxSize int64) (string, error) {
	f, r, err := lpdf.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("opening PDF: %w", err)
	}
	defer func() { _ = f.Close() }()

	reader, err := r.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("reading PDF text: %w", err)
	}

	b, err := io.ReadAll(io.LimitReader(reader, maxSize+1))
	if err != nil {
		return "", fmt.Errorf("reading PDF text output: %w", err)
	}
	if int64(len(b)) > maxSize {
		return "", fmt.Errorf("extracted PDF text exceeds %d bytes limit", maxSize)
	}

	return cleanText(string(b)), nil
}

// Compiled regexes for PDF text cleanup.
var (
	// brokenBracketRef matches footnote/endnote references split across lines: [\n1\n] or [\na\n].
	brokenBracketRef = regexp.MustCompile(`\[\s*\n(\w+)\s*\n\]`)
	// excessiveBlankLines collapses 3+ consecutive blank lines to 2.
	excessiveBlankLines = regexp.MustCompile(`\n{4,}`)
)

// cleanText normalizes raw PDF text output.
// PDF text objects (each styling span: bold, italic, link) are emitted as
// separate lines by ledongthuc/pdf. This function rejoins continuation lines
// so that styled inline fragments become contiguous sentences.
func cleanText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = brokenBracketRef.ReplaceAllString(s, "[$1]")
	s = joinContinuationLines(s)
	s = excessiveBlankLines.ReplaceAllString(s, "\n\n\n")
	return s
}

// joinContinuationLines merges lines that are continuations of the previous
// line. A line is a continuation when it starts with a lowercase letter,
// punctuation, or whitespace — indicating it was split at a PDF text object
// boundary (styling change) rather than a real line break.
func joinContinuationLines(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= 1 {
		return s
	}

	var b strings.Builder
	b.Grow(len(s))
	b.WriteString(lines[0])

	// Track last byte written to avoid b.String() copies in the hot loop.
	lastByte := lastByteOf(lines[0])

	for i := 1; i < len(lines); i++ {
		line := lines[i]
		if line == "" {
			// Blank line = paragraph break, keep it.
			b.WriteByte('\n')
			lastByte = '\n'
			continue
		}

		first, _ := utf8.DecodeRuneInString(line)
		if isContinuation(first) {
			// Continuation of previous text object — join without newline.
			// Insert a space if the previous line didn't end with whitespace
			// and this line doesn't start with whitespace or punctuation,
			// to avoid "writingstyle" from "writing\nstyle".
			if lastByte != ' ' && lastByte != '\t' && !isAttaching(first) {
				b.WriteByte(' ')
			}
			b.WriteString(line)
		} else {
			b.WriteByte('\n')
			b.WriteString(line)
		}
		lastByte = line[len(line)-1]
	}

	return b.String()
}

// isAttaching reports whether a rune attaches to the preceding word without
// a space (punctuation, closing brackets, whitespace).
func isAttaching(r rune) bool {
	switch r {
	case ' ', '\t', ',', '.', ';', ':', '!', '?', ')', ']', '}',
		'\u2014', '\u2013', '\u2019', '\u201D': // em dash, en dash, right quotes
		return true
	}
	return false
}

// lastByteOf returns the last byte of s, or 0 if empty.
func lastByteOf(s string) byte {
	if s == "" {
		return 0
	}
	return s[len(s)-1]
}

// isContinuation reports whether a rune at the start of a line indicates it
// is a continuation of the previous line (i.e. a PDF text object boundary,
// not a real line break). Lowercase letters, punctuation, whitespace, and
// opening brackets all signal continuation.
func isContinuation(r rune) bool {
	if r >= 'a' && r <= 'z' {
		return true
	}
	switch r {
	case ' ', '\t', // leading whitespace
		',', '.', ';', ':', '!', '?', // sentence punctuation
		')', ']', '}', // closing brackets
		'\u2014', '\u2013', '\u2026', // em dash, en dash, ellipsis
		'\u201C', '\u201D', '\u2018', '\u2019': // curly quotes
		return true
	}
	return false
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
