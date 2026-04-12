// Package epub extracts text content from EPUB files.
//
// EPUB files are ZIP archives containing XHTML chapter files and XML metadata.
// This extractor reads META-INF/container.xml to locate the OPF package file,
// parses Dublin Core metadata, then extracts text from XHTML chapters in spine
// (reading) order.
package epub

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/microcosm-cc/bluemonday"

	"github.com/c-premus/documcp/internal/extractor"
)

const (
	mimeType = "application/epub+zip"

	// defaultMaxZIPFiles limits the number of files in an EPUB ZIP archive.
	// Higher than DOCX because EPUBs legitimately contain images, CSS, fonts.
	defaultMaxZIPFiles = 500

	// defaultMaxDecompressedFileSize is the maximum decompressed size per file (50 MiB).
	defaultMaxDecompressedFileSize = 50 * 1024 * 1024

	// defaultMaxTotalDecompressed is the cumulative decompression budget (200 MiB).
	defaultMaxTotalDecompressed = 200 * 1024 * 1024

	// xhtmlMediaType is the MIME type for XHTML chapter files in the OPF manifest.
	xhtmlMediaType = "application/xhtml+xml"
)

// container represents META-INF/container.xml.
type container struct {
	Rootfiles []rootfile `xml:"rootfiles>rootfile"`
}

// rootfile is a single rootfile entry in container.xml.
type rootfile struct {
	FullPath  string `xml:"full-path,attr"`
	MediaType string `xml:"media-type,attr"`
}

// opfPackage represents the OPF package document.
type opfPackage struct {
	Metadata opfMetadata `xml:"metadata"`
	Manifest opfManifest `xml:"manifest"`
	Spine    opfSpine    `xml:"spine"`
}

// opfMetadata holds Dublin Core metadata from the OPF file.
type opfMetadata struct {
	Title       string   `xml:"http://purl.org/dc/elements/1.1/ title"`
	Creator     string   `xml:"http://purl.org/dc/elements/1.1/ creator"`
	Description string   `xml:"http://purl.org/dc/elements/1.1/ description"`
	Subjects    []string `xml:"http://purl.org/dc/elements/1.1/ subject"`
	Publisher   string   `xml:"http://purl.org/dc/elements/1.1/ publisher"`
	Date        string   `xml:"http://purl.org/dc/elements/1.1/ date"`
	Language    string   `xml:"http://purl.org/dc/elements/1.1/ language"`
	Identifier  string   `xml:"http://purl.org/dc/elements/1.1/ identifier"`
}

// opfManifest holds the manifest items.
type opfManifest struct {
	Items []opfItem `xml:"item"`
}

// opfItem is a single manifest item.
type opfItem struct {
	ID        string `xml:"id,attr"`
	Href      string `xml:"href,attr"`
	MediaType string `xml:"media-type,attr"`
}

// opfSpine holds the reading order.
type opfSpine struct {
	ItemRefs []opfItemRef `xml:"itemref"`
}

// opfItemRef is a single spine itemref.
type opfItemRef struct {
	IDRef string `xml:"idref,attr"`
}

// EPUBExtractor extracts text and metadata from EPUB files.
//
//nolint:revive // exported stutter is intentional for consistency with other extractors
type EPUBExtractor struct {
	maxZIPFiles             int
	maxDecompressedFileSize int64
	maxTotalDecompressed    int64
	policy                  *bluemonday.Policy
}

// Compile-time check that EPUBExtractor implements extractor.Extractor.
var _ extractor.Extractor = (*EPUBExtractor)(nil)

// New creates a new EPUBExtractor with default limits.
func New() *EPUBExtractor {
	return &EPUBExtractor{
		maxZIPFiles:             defaultMaxZIPFiles,
		maxDecompressedFileSize: defaultMaxDecompressedFileSize,
		maxTotalDecompressed:    defaultMaxTotalDecompressed,
		policy:                  bluemonday.UGCPolicy(),
	}
}

// NewWithLimits creates an EPUBExtractor with configurable limits.
// Zero values fall back to defaults.
func NewWithLimits(maxZIPFiles int, maxExtractedText int64) *EPUBExtractor {
	e := New()
	if maxZIPFiles > 0 {
		e.maxZIPFiles = maxZIPFiles
	}
	if maxExtractedText > 0 {
		e.maxDecompressedFileSize = maxExtractedText
	}
	return e
}

// Supports reports whether this extractor handles the given MIME type.
func (e *EPUBExtractor) Supports(mime string) bool {
	return mime == mimeType
}

// Extract reads the EPUB file at filePath and returns its text content with
// metadata. Chapter text is extracted in spine (reading) order. Dublin Core
// metadata is baked into a header block for FTS discoverability and also
// returned in the Metadata map for the analyze endpoint.
func (e *EPUBExtractor) Extract(ctx context.Context, filePath string) (*extractor.ExtractedContent, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("extracting epub %q: %w", filePath, err)
	}

	zr, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("opening epub %q: %w", filePath, err)
	}
	defer func() { _ = zr.Close() }()

	if len(zr.File) > e.maxZIPFiles {
		return nil, fmt.Errorf("epub %q contains %d files, exceeding limit of %d", filePath, len(zr.File), e.maxZIPFiles)
	}

	// Pre-flight check: verify cumulative decompressed size from ZIP directory
	// headers. First gate against zip bombs; per-file io.LimitReader is the
	// enforcement layer.
	var totalUncompressed uint64
	budget := uint64(max(e.maxTotalDecompressed, 0)) //nolint:gosec // maxTotalDecompressed is always positive
	for _, f := range zr.File {
		totalUncompressed += f.UncompressedSize64
		if totalUncompressed > budget {
			return nil, fmt.Errorf("epub %q total decompressed size exceeds budget of %d bytes", filePath, e.maxTotalDecompressed)
		}
	}

	// Locate and parse OPF package document.
	opfPath, err := findContainerRootfile(zr, e.maxDecompressedFileSize)
	if err != nil {
		return nil, fmt.Errorf("reading epub container %q: %w", filePath, err)
	}

	pkg, err := parseOPF(zr, opfPath, e.maxDecompressedFileSize)
	if err != nil {
		return nil, fmt.Errorf("parsing epub OPF %q: %w", filePath, err)
	}

	// Build manifest ID → item lookup.
	manifest := make(map[string]opfItem, len(pkg.Manifest.Items))
	for _, item := range pkg.Manifest.Items {
		manifest[item.ID] = item
	}

	// Extract chapter text in spine order.
	opfDir := path.Dir(opfPath)
	var chapters []string
	for _, ref := range pkg.Spine.ItemRefs {
		item, ok := manifest[ref.IDRef]
		if !ok || item.MediaType != xhtmlMediaType {
			continue
		}

		// Resolve chapter path relative to OPF directory.
		href, unescErr := url.PathUnescape(item.Href)
		if unescErr != nil {
			continue
		}
		chapterPath := path.Join(opfDir, href)

		text, extractErr := extractChapterText(zr, chapterPath, e.maxDecompressedFileSize, e.policy)
		if extractErr != nil {
			continue // skip unreadable chapters
		}
		if text != "" {
			chapters = append(chapters, text)
		}
	}

	if len(chapters) == 0 {
		return nil, fmt.Errorf("epub %q contains no extractable chapter content", filePath)
	}

	// Build content: metadata header + chapter text.
	header := buildMetadataHeader(pkg.Metadata)
	var content string
	if header != "" {
		content = header + "\n\n---\n\n" + strings.Join(chapters, "\n\n---\n\n")
	} else {
		content = strings.Join(chapters, "\n\n---\n\n")
	}

	metadata := buildMetadataMap(pkg.Metadata)

	return &extractor.ExtractedContent{
		Content:   content,
		Metadata:  metadata,
		WordCount: len(strings.Fields(content)),
	}, nil
}

// findContainerRootfile parses META-INF/container.xml and returns the full-path
// of the first rootfile entry.
func findContainerRootfile(zr *zip.ReadCloser, maxFileSize int64) (string, error) {
	f, err := findFile(zr, "META-INF/container.xml")
	if err != nil {
		return "", fmt.Errorf("finding container.xml: %w", err)
	}

	rc, err := f.Open()
	if err != nil {
		return "", fmt.Errorf("opening container.xml: %w", err)
	}
	defer func() { _ = rc.Close() }()

	var c container
	if err := xml.NewDecoder(io.LimitReader(rc, maxFileSize)).Decode(&c); err != nil {
		return "", fmt.Errorf("decoding container.xml: %w", err)
	}

	if len(c.Rootfiles) == 0 {
		return "", errors.New("container.xml has no rootfile entries")
	}

	return c.Rootfiles[0].FullPath, nil
}

// parseOPF parses the OPF package document at opfPath inside the ZIP archive.
func parseOPF(zr *zip.ReadCloser, opfPath string, maxFileSize int64) (*opfPackage, error) {
	f, err := findFile(zr, opfPath)
	if err != nil {
		return nil, fmt.Errorf("finding OPF %q: %w", opfPath, err)
	}

	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("opening OPF %q: %w", opfPath, err)
	}
	defer func() { _ = rc.Close() }()

	var pkg opfPackage
	if err := xml.NewDecoder(io.LimitReader(rc, maxFileSize)).Decode(&pkg); err != nil {
		return nil, fmt.Errorf("decoding OPF %q: %w", opfPath, err)
	}

	return &pkg, nil
}

// extractChapterText opens a chapter XHTML file from the ZIP, sanitizes it,
// and converts it to markdown text.
func extractChapterText(zr *zip.ReadCloser, chapterPath string, maxFileSize int64, policy *bluemonday.Policy) (string, error) {
	f, err := findFile(zr, chapterPath)
	if err != nil {
		return "", fmt.Errorf("finding chapter %q: %w", chapterPath, err)
	}

	rc, err := f.Open()
	if err != nil {
		return "", fmt.Errorf("opening chapter %q: %w", chapterPath, err)
	}
	defer func() { _ = rc.Close() }()

	raw, err := io.ReadAll(io.LimitReader(rc, maxFileSize))
	if err != nil {
		return "", fmt.Errorf("reading chapter %q: %w", chapterPath, err)
	}

	sanitized := policy.SanitizeBytes(raw)

	markdown, err := htmltomarkdown.ConvertString(string(sanitized))
	if err != nil {
		return "", fmt.Errorf("converting chapter %q to markdown: %w", chapterPath, err)
	}

	return strings.TrimSpace(markdown), nil
}

// buildMetadataHeader creates a markdown header from OPF metadata for baking
// into extracted content. This makes metadata FTS-searchable (e.g., searching
// for an author name finds the document even if the name doesn't appear in
// chapter text).
func buildMetadataHeader(meta opfMetadata) string {
	var lines []string

	if meta.Title != "" {
		lines = append(lines, "# "+meta.Title, "")
	}

	if meta.Creator != "" {
		lines = append(lines, "**Author:** "+meta.Creator)
	}
	if meta.Publisher != "" {
		lines = append(lines, "**Publisher:** "+meta.Publisher)
	}
	if meta.Date != "" {
		lines = append(lines, "**Published:** "+meta.Date)
	}
	if len(meta.Subjects) > 0 {
		lines = append(lines, "**Subjects:** "+strings.Join(meta.Subjects, ", "))
	}
	if meta.Language != "" {
		lines = append(lines, "**Language:** "+meta.Language)
	}
	if meta.Identifier != "" {
		lines = append(lines, "**Identifier:** "+meta.Identifier)
	}

	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

// buildMetadataMap creates a metadata map from OPF metadata for the analyze
// endpoint. Only non-empty values are included.
func buildMetadataMap(meta opfMetadata) map[string]any {
	m := make(map[string]any)

	if meta.Title != "" {
		m["title"] = meta.Title
	}
	if meta.Creator != "" {
		m["creator"] = meta.Creator
	}
	if meta.Description != "" {
		m["description"] = meta.Description
	}
	if len(meta.Subjects) > 0 {
		m["subjects"] = meta.Subjects
	}
	if meta.Publisher != "" {
		m["publisher"] = meta.Publisher
	}
	if meta.Date != "" {
		m["date"] = meta.Date
	}
	if meta.Language != "" {
		m["language"] = meta.Language
	}
	if meta.Identifier != "" {
		m["identifier"] = meta.Identifier
	}

	if len(m) == 0 {
		return nil
	}
	return m
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
