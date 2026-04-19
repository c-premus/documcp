// Package docx extracts text content from DOCX files.
//
// DOCX files are ZIP archives containing XML. This extractor reads
// word/document.xml for body text, word/header*.xml and word/footer*.xml
// for header/footer text, and docProps/core.xml for metadata.
package docx

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/c-premus/documcp/internal/extractor"
	"github.com/c-premus/documcp/internal/extractor/ziputil"
)

const (
	mimeType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

	// defaultMaxZIPFiles limits the number of files processed from a DOCX ZIP archive.
	defaultMaxZIPFiles = 100

	// defaultMaxDecompressedFileSize is the maximum decompressed size per file (50 MiB).
	defaultMaxDecompressedFileSize = 50 * 1024 * 1024

	// defaultMaxTotalDecompressed is the cumulative decompression budget across all files (100 MiB).
	// Prevents zip bombs where many small entries each decompress within per-file limits.
	defaultMaxTotalDecompressed = 100 * 1024 * 1024
)

// wordprocessingML namespace.
const wNS = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"

// coreProperties represents the Dublin Core metadata in docProps/core.xml.
type coreProperties struct {
	Title       string `xml:"title"`
	Creator     string `xml:"creator"`
	Description string `xml:"description"`
}

// DOCXExtractor extracts text and metadata from DOCX files.
//
//nolint:revive // exported stutter is intentional; renaming would be a breaking change
type DOCXExtractor struct {
	maxZIPFiles             int
	maxDecompressedFileSize int64
	maxTotalDecompressed    int64
}

// Compile-time check that DOCXExtractor implements extractor.Extractor.
var _ extractor.Extractor = (*DOCXExtractor)(nil)

// New creates a new DOCXExtractor with default limits.
func New() *DOCXExtractor {
	return &DOCXExtractor{
		maxZIPFiles:             defaultMaxZIPFiles,
		maxDecompressedFileSize: defaultMaxDecompressedFileSize,
		maxTotalDecompressed:    defaultMaxTotalDecompressed,
	}
}

// NewWithLimits creates a DOCXExtractor with configurable limits.
// Zero values fall back to defaults.
func NewWithLimits(maxZIPFiles int, maxExtractedText int64) *DOCXExtractor {
	e := New()
	if maxZIPFiles > 0 {
		e.maxZIPFiles = maxZIPFiles
	}
	if maxExtractedText > 0 {
		e.maxDecompressedFileSize = maxExtractedText
	}
	return e
}

// NewWithAllLimits creates a DOCXExtractor with all three resource limits
// configurable. maxTotalDecompressed is the cumulative runtime decompression
// budget across every entry processed by a single Extract call; it is the
// authoritative defense against zip bombs whose central-directory headers
// understate real decompression size. Zero or negative values fall back to
// package defaults.
func NewWithAllLimits(maxZIPFiles int, maxFileSize, maxTotalDecompressed int64) *DOCXExtractor {
	e := NewWithLimits(maxZIPFiles, maxFileSize)
	if maxTotalDecompressed > 0 {
		e.maxTotalDecompressed = maxTotalDecompressed
	}
	return e
}

// Supports reports whether this extractor handles the given MIME type.
func (e *DOCXExtractor) Supports(mime string) bool {
	return mime == mimeType
}

// Extract reads the DOCX file at filePath and returns its text content.
func (e *DOCXExtractor) Extract(ctx context.Context, filePath string) (*extractor.ExtractedContent, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("extracting docx %q: %w", filePath, err)
	}

	zr, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("opening docx %q: %w", filePath, err)
	}
	defer func() { _ = zr.Close() }()

	if len(zr.File) > e.maxZIPFiles {
		return nil, fmt.Errorf("docx %q contains %d files, exceeding limit of %d", filePath, len(zr.File), e.maxZIPFiles)
	}

	// Pre-flight check: verify cumulative decompressed size from ZIP directory
	// headers. This is a first gate against zip bombs; per-file io.LimitReader
	// in parseZipXML is the enforcement layer.
	var totalUncompressed uint64
	budget := uint64(max(e.maxTotalDecompressed, 0)) //nolint:gosec // maxTotalDecompressed is always positive (default 100 MiB)
	for _, f := range zr.File {
		totalUncompressed += f.UncompressedSize64
		if totalUncompressed > budget {
			return nil, fmt.Errorf("docx %q total decompressed size exceeds budget of %d bytes", filePath, e.maxTotalDecompressed)
		}
	}

	// decompressed tracks cumulative bytes actually handed to the decoders
	// across every entry read below. Shared with BudgetReader so a lying
	// central directory cannot bypass the per-file io.LimitReader gate.
	var decompressed int64

	content, err := extractText(zr, e.maxDecompressedFileSize, &decompressed, e.maxTotalDecompressed)
	if err != nil {
		return nil, fmt.Errorf("extracting text from %q: %w", filePath, err)
	}

	metadata := extractMetadata(zr, e.maxDecompressedFileSize, &decompressed, e.maxTotalDecompressed)

	return &extractor.ExtractedContent{
		Content:   content,
		Metadata:  metadata,
		WordCount: len(strings.Fields(content)),
	}, nil
}

// extractText parses word/document.xml (body), word/header*.xml (headers),
// and word/footer*.xml (footers), returning the concatenated text in reading
// order: headers, body, footers.
//
// total and totalMax are shared across every entry read: total accumulates
// decompressed bytes handed to the XML decoder, and parsing halts with
// ziputil.ErrBudgetExceeded once total exceeds totalMax.
func extractText(zr *zip.ReadCloser, maxFileSize int64, total *int64, totalMax int64) (string, error) {
	docFile, err := findFile(zr, "word/document.xml")
	if err != nil {
		return "", fmt.Errorf("finding document.xml: %w", err)
	}

	body, err := parseZipXML(docFile, maxFileSize, total, totalMax)
	if err != nil {
		return "", fmt.Errorf("parsing document.xml: %w", err)
	}

	// Collect header and footer text (optional — silently skip malformed-XML
	// failures). Budget-exhaustion errors must NOT be swallowed here: they
	// indicate a zip bomb and have to propagate to the caller.
	var sections []string

	for _, f := range findFilesByPrefix(zr, "word/header") {
		text, parseErr := parseZipXML(f, maxFileSize, total, totalMax)
		if errors.Is(parseErr, ziputil.ErrBudgetExceeded) {
			return "", parseErr
		}
		if parseErr == nil && text != "" {
			sections = append(sections, text)
		}
	}

	sections = append(sections, body)

	for _, f := range findFilesByPrefix(zr, "word/footer") {
		text, parseErr := parseZipXML(f, maxFileSize, total, totalMax)
		if errors.Is(parseErr, ziputil.ErrBudgetExceeded) {
			return "", parseErr
		}
		if parseErr == nil && text != "" {
			sections = append(sections, text)
		}
	}

	return strings.Join(sections, "\n\n"), nil
}

// parseZipXML opens a ZIP file entry and parses its WordprocessingML content.
// The reader chain is LimitReader (per-file cap) wrapped by BudgetReader
// (cumulative cap across the archive).
func parseZipXML(f *zip.File, maxFileSize int64, total *int64, totalMax int64) (string, error) {
	rc, err := f.Open()
	if err != nil {
		return "", fmt.Errorf("opening %s: %w", f.Name, err)
	}
	defer func() { _ = rc.Close() }()

	br := ziputil.NewBudgetReader(io.LimitReader(rc, maxFileSize), total, totalMax)
	return parseDocument(br)
}

// parseDocument decodes the XML from word/document.xml and returns paragraphs
// joined by double newlines. It extracts text only from <w:t> elements inside
// <w:p> elements, matching the WordprocessingML structure.
func parseDocument(r io.Reader) (string, error) {
	decoder := xml.NewDecoder(r)

	var paragraphs []string
	var inParagraph bool
	var inText bool
	var currentParagraph strings.Builder

	for {
		tok, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("decoding document XML: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Space == wNS {
				switch t.Name.Local {
				case "p":
					inParagraph = true
					currentParagraph.Reset()
				case "t":
					if inParagraph {
						inText = true
					}
				}
			}
		case xml.EndElement:
			if t.Name.Space == wNS {
				switch t.Name.Local {
				case "p":
					text := strings.TrimSpace(currentParagraph.String())
					if text != "" {
						paragraphs = append(paragraphs, text)
					}
					inParagraph = false
				case "t":
					inText = false
				}
			}
		case xml.CharData:
			if inText {
				currentParagraph.Write(t)
			}
		}
	}

	return strings.Join(paragraphs, "\n\n"), nil
}

// extractMetadata reads docProps/core.xml and returns any title, creator, or
// description fields found. total and totalMax share the cumulative
// decompression budget with extractText so metadata parsing cannot smuggle
// bytes past the zip-bomb defense.
func extractMetadata(zr *zip.ReadCloser, maxFileSize int64, total *int64, totalMax int64) map[string]any {
	f, err := findFile(zr, "docProps/core.xml")
	if err != nil {
		return nil
	}

	rc, err := f.Open()
	if err != nil {
		return nil
	}
	defer func() { _ = rc.Close() }()

	br := ziputil.NewBudgetReader(io.LimitReader(rc, maxFileSize), total, totalMax)
	var props coreProperties
	if err := xml.NewDecoder(br).Decode(&props); err != nil {
		return nil
	}

	metadata := make(map[string]any)
	if props.Title != "" {
		metadata["title"] = props.Title
	}
	if props.Creator != "" {
		metadata["creator"] = props.Creator
	}
	if props.Description != "" {
		metadata["description"] = props.Description
	}

	if len(metadata) == 0 {
		return nil
	}
	return metadata
}

// findFile locates a file by exact name inside the ZIP archive.
func findFile(zr *zip.ReadCloser, name string) (*zip.File, error) {
	for _, f := range zr.File {
		if f.Name == name {
			return f, nil
		}
	}
	return nil, fmt.Errorf("file %q not found in archive", name)
}

// findFilesByPrefix returns all files whose name starts with prefix.
func findFilesByPrefix(zr *zip.ReadCloser, prefix string) []*zip.File {
	var matches []*zip.File
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, prefix) {
			matches = append(matches, f)
		}
	}
	return matches
}
