// Package docx extracts text content from DOCX files.
//
// DOCX files are ZIP archives containing XML. This extractor reads
// word/document.xml for body text and docProps/core.xml for metadata.
package docx

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"git.999.haus/chris/DocuMCP-go/internal/extractor"
)

const mimeType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

// wordprocessingML namespace.
const wNS = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"

// coreProperties represents the Dublin Core metadata in docProps/core.xml.
type coreProperties struct {
	Title       string `xml:"title"`
	Creator     string `xml:"creator"`
	Description string `xml:"description"`
}

// DOCXExtractor extracts text and metadata from DOCX files.
type DOCXExtractor struct{}

// Compile-time check that DOCXExtractor implements extractor.Extractor.
var _ extractor.Extractor = (*DOCXExtractor)(nil)

// New creates a new DOCXExtractor.
func New() *DOCXExtractor {
	return &DOCXExtractor{}
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

	content, err := extractText(zr)
	if err != nil {
		return nil, fmt.Errorf("extracting text from %q: %w", filePath, err)
	}

	metadata := extractMetadata(zr)

	return &extractor.ExtractedContent{
		Content:   content,
		Metadata:  metadata,
		WordCount: len(strings.Fields(content)),
	}, nil
}

// extractText parses word/document.xml and returns the concatenated paragraph text.
func extractText(zr *zip.ReadCloser) (string, error) {
	f, err := findFile(zr, "word/document.xml")
	if err != nil {
		return "", fmt.Errorf("finding document.xml: %w", err)
	}

	rc, err := f.Open()
	if err != nil {
		return "", fmt.Errorf("opening document.xml: %w", err)
	}
	defer func() { _ = rc.Close() }()

	return parseDocument(rc)
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
		if err == io.EOF {
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
// description fields found.
func extractMetadata(zr *zip.ReadCloser) map[string]any {
	f, err := findFile(zr, "docProps/core.xml")
	if err != nil {
		return nil
	}

	rc, err := f.Open()
	if err != nil {
		return nil
	}
	defer func() { _ = rc.Close() }()

	var props coreProperties
	if err := xml.NewDecoder(rc).Decode(&props); err != nil {
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

// findFile locates a file by name inside the ZIP archive.
func findFile(zr *zip.ReadCloser, name string) (*zip.File, error) {
	for _, f := range zr.File {
		if f.Name == name {
			return f, nil
		}
	}
	return nil, fmt.Errorf("file %q not found in archive", name)
}
