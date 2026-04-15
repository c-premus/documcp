package epub_test

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/c-premus/documcp/internal/extractor/epub"
)

// epubMeta holds metadata options for creating test EPUB files.
type epubMeta struct {
	Title       string
	Creator     string
	Description string
	Subjects    []string
	Publisher   string
	Date        string
	Language    string
	Identifier  string
}

// epubOpts holds all options for building a test EPUB.
type epubOpts struct {
	meta     *epubMeta
	chapters []string // XHTML body content for each chapter
	opfDir   string   // directory prefix for OPF (e.g., "OEBPS"); empty = root
}

// createTestEPUB builds a minimal valid EPUB in t.TempDir() and returns the path.
func createTestEPUB(t *testing.T, opts epubOpts) string {
	t.Helper()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.epub")

	f, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	defer func() { _ = f.Close() }()

	zw := zip.NewWriter(f)
	defer func() { _ = zw.Close() }()

	opfDir := opts.opfDir
	opfPath := "content.opf"
	if opfDir != "" {
		opfPath = opfDir + "/" + opfPath
	}

	// Write mimetype (uncompressed, as per EPUB spec).
	writeZipEntry(t, zw, "mimetype", "application/epub+zip")

	// Write META-INF/container.xml.
	writeZipEntry(t, zw, "META-INF/container.xml", buildContainerXML(opfPath))

	// Build chapter manifest items and spine refs.
	var refs []chapterRef
	for i, body := range opts.chapters {
		id := "ch" + itoa(i+1)
		href := "chapter" + itoa(i+1) + ".xhtml"
		refs = append(refs, chapterRef{id: id, href: href})

		chapterPath := href
		if opfDir != "" {
			chapterPath = opfDir + "/" + href
		}
		writeZipEntry(t, zw, chapterPath, buildXHTML("Chapter "+itoa(i+1), body))
	}

	// Write OPF.
	writeZipEntry(t, zw, opfPath, buildOPF(opts.meta, refs))

	return filePath
}

// itoa converts a small int to string without importing strconv.
func itoa(n int) string {
	if n < 0 {
		return "-" + itoa(-n)
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}

// writeZipEntry creates a file in the ZIP and writes content.
func writeZipEntry(t *testing.T, zw *zip.Writer, name, content string) {
	t.Helper()
	w, err := zw.Create(name)
	if err != nil {
		t.Fatalf("creating %s in zip: %v", name, err)
	}
	if _, err := w.Write([]byte(content)); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
}

// buildContainerXML produces META-INF/container.xml pointing to the given OPF path.
func buildContainerXML(opfPath string) string {
	var b strings.Builder
	b.WriteString(xml.Header)
	b.WriteString(`<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0">`)
	b.WriteString(`<rootfiles>`)
	b.WriteString(`<rootfile full-path="`)
	_ = xml.EscapeText(&b, []byte(opfPath))
	b.WriteString(`" media-type="application/oebps-package+xml"/>`)
	b.WriteString(`</rootfiles>`)
	b.WriteString(`</container>`)
	return b.String()
}

// chapterRef is used internally by buildOPF.
type chapterRef struct {
	id   string
	href string
}

// buildOPF produces an OPF package document.
func buildOPF(meta *epubMeta, chapters []chapterRef) string {
	var b strings.Builder
	b.WriteString(xml.Header)
	b.WriteString(`<package xmlns="http://www.idpf.org/2007/opf" version="3.0">`)

	// Metadata.
	b.WriteString(`<metadata xmlns:dc="http://purl.org/dc/elements/1.1/">`)
	if meta != nil {
		if meta.Title != "" {
			b.WriteString(`<dc:title>`)
			_ = xml.EscapeText(&b, []byte(meta.Title))
			b.WriteString(`</dc:title>`)
		}
		if meta.Creator != "" {
			b.WriteString(`<dc:creator>`)
			_ = xml.EscapeText(&b, []byte(meta.Creator))
			b.WriteString(`</dc:creator>`)
		}
		if meta.Description != "" {
			b.WriteString(`<dc:description>`)
			_ = xml.EscapeText(&b, []byte(meta.Description))
			b.WriteString(`</dc:description>`)
		}
		for _, subj := range meta.Subjects {
			b.WriteString(`<dc:subject>`)
			_ = xml.EscapeText(&b, []byte(subj))
			b.WriteString(`</dc:subject>`)
		}
		if meta.Publisher != "" {
			b.WriteString(`<dc:publisher>`)
			_ = xml.EscapeText(&b, []byte(meta.Publisher))
			b.WriteString(`</dc:publisher>`)
		}
		if meta.Date != "" {
			b.WriteString(`<dc:date>`)
			_ = xml.EscapeText(&b, []byte(meta.Date))
			b.WriteString(`</dc:date>`)
		}
		if meta.Language != "" {
			b.WriteString(`<dc:language>`)
			_ = xml.EscapeText(&b, []byte(meta.Language))
			b.WriteString(`</dc:language>`)
		}
		if meta.Identifier != "" {
			b.WriteString(`<dc:identifier>`)
			_ = xml.EscapeText(&b, []byte(meta.Identifier))
			b.WriteString(`</dc:identifier>`)
		}
	}
	b.WriteString(`</metadata>`)

	// Manifest.
	b.WriteString(`<manifest>`)
	for _, ch := range chapters {
		b.WriteString(`<item id="`)
		_ = xml.EscapeText(&b, []byte(ch.id))
		b.WriteString(`" href="`)
		_ = xml.EscapeText(&b, []byte(ch.href))
		b.WriteString(`" media-type="application/xhtml+xml"/>`)
	}
	b.WriteString(`</manifest>`)

	// Spine.
	b.WriteString(`<spine>`)
	for _, ch := range chapters {
		b.WriteString(`<itemref idref="`)
		_ = xml.EscapeText(&b, []byte(ch.id))
		b.WriteString(`"/>`)
	}
	b.WriteString(`</spine>`)

	b.WriteString(`</package>`)
	return b.String()
}

// buildXHTML produces a minimal XHTML chapter document.
func buildXHTML(title, body string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.WriteString(`<!DOCTYPE html>`)
	b.WriteString(`<html xmlns="http://www.w3.org/1999/xhtml">`)
	b.WriteString(`<head><title>`)
	_ = xml.EscapeText(&b, []byte(title))
	b.WriteString(`</title></head>`)
	b.WriteString(`<body>`)
	b.WriteString(body)
	b.WriteString(`</body></html>`)
	return b.String()
}

func TestEPUBExtractor_Extract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		opts         epubOpts
		wantContains []string
		wantNotEmpty bool
	}{
		{
			name: "single chapter extracted",
			opts: epubOpts{
				chapters: []string{"<p>Hello from EPUB chapter one.</p>"},
			},
			wantContains: []string{"Hello from EPUB chapter one."},
			wantNotEmpty: true,
		},
		{
			name: "multiple chapters in spine order",
			opts: epubOpts{
				chapters: []string{
					"<p>AAA first chapter content.</p>",
					"<p>BBB second chapter content.</p>",
					"<p>CCC third chapter content.</p>",
				},
			},
			wantContains: []string{"AAA first chapter", "BBB second chapter", "CCC third chapter"},
			wantNotEmpty: true,
		},
		{
			name: "metadata extracted into content header",
			opts: epubOpts{
				meta: &epubMeta{
					Title:   "Test Book Title",
					Creator: "Jane Author",
				},
				chapters: []string{"<p>Chapter body text.</p>"},
			},
			wantContains: []string{"Test Book Title", "Jane Author", "Chapter body text."},
			wantNotEmpty: true,
		},
		{
			name: "full metadata in content header",
			opts: epubOpts{
				meta: &epubMeta{
					Title:      "Full Metadata Book",
					Creator:    "John Writer",
					Publisher:  "Big Publisher",
					Date:       "2024-06-15",
					Subjects:   []string{"Fiction", "Adventure"},
					Language:   "en",
					Identifier: "isbn:978-0-123456-78-9",
				},
				chapters: []string{"<p>Content here.</p>"},
			},
			wantContains: []string{
				"Full Metadata Book",
				"John Writer",
				"Big Publisher",
				"2024-06-15",
				"Fiction, Adventure",
				"isbn:978-0-123456-78-9",
			},
			wantNotEmpty: true,
		},
		{
			name: "OPF in subdirectory with relative chapter paths",
			opts: epubOpts{
				opfDir:   "OEBPS",
				chapters: []string{"<p>Subdirectory chapter content.</p>"},
			},
			wantContains: []string{"Subdirectory chapter content."},
			wantNotEmpty: true,
		},
		{
			name: "no metadata produces no header",
			opts: epubOpts{
				chapters: []string{"<p>Just content.</p>"},
			},
			wantContains: []string{"Just content."},
			wantNotEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			filePath := createTestEPUB(t, tt.opts)

			ext := epub.New()
			result, err := ext.Extract(context.Background(), filePath)
			if err != nil {
				t.Fatalf("Extract() unexpected error: %v", err)
			}

			if tt.wantNotEmpty && result.Content == "" {
				t.Fatal("Extract() returned empty content")
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(result.Content, want) {
					t.Errorf("Content does not contain %q\ngot: %q", want, result.Content)
				}
			}

			if tt.wantNotEmpty && result.WordCount == 0 {
				t.Error("WordCount = 0, want > 0")
			}
		})
	}
}

func TestEPUBExtractor_Extract_ChapterOrder(t *testing.T) {
	t.Parallel()

	filePath := createTestEPUB(t, epubOpts{
		chapters: []string{
			"<p>Alpha first chapter here</p>",
			"<p>Bravo second chapter here</p>",
			"<p>Charlie third chapter here</p>",
		},
	})

	ext := epub.New()
	result, err := ext.Extract(context.Background(), filePath)
	if err != nil {
		t.Fatalf("Extract() unexpected error: %v", err)
	}

	idxA := strings.Index(result.Content, "Alpha first")
	idxB := strings.Index(result.Content, "Bravo second")
	idxC := strings.Index(result.Content, "Charlie third")

	if idxA == -1 || idxB == -1 || idxC == -1 {
		t.Fatalf("missing chapter markers in content:\n%s", result.Content)
	}
	if idxA >= idxB || idxB >= idxC {
		t.Errorf("chapters not in spine order: A=%d B=%d C=%d", idxA, idxB, idxC)
	}
}

func TestEPUBExtractor_Extract_MetadataMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		meta             *epubMeta
		wantMetadataKeys []string
		wantNoMetadata   bool
	}{
		{
			name:           "no metadata when OPF has no dc elements",
			meta:           nil,
			wantNoMetadata: true,
		},
		{
			name:             "title only",
			meta:             &epubMeta{Title: "Solo Title"},
			wantMetadataKeys: []string{"title"},
		},
		{
			name: "all metadata fields populated",
			meta: &epubMeta{
				Title:       "Full Book",
				Creator:     "Author",
				Description: "A description.",
				Subjects:    []string{"Fiction"},
				Publisher:   "Pub",
				Date:        "2024",
				Language:    "en",
				Identifier:  "isbn:123",
			},
			wantMetadataKeys: []string{"title", "creator", "description", "subjects", "publisher", "date", "language", "identifier"},
		},
		{
			name:           "empty metadata struct produces no metadata",
			meta:           &epubMeta{},
			wantNoMetadata: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			filePath := createTestEPUB(t, epubOpts{
				meta:     tt.meta,
				chapters: []string{"<p>body</p>"},
			})

			ext := epub.New()
			result, err := ext.Extract(context.Background(), filePath)
			if err != nil {
				t.Fatalf("Extract() unexpected error: %v", err)
			}

			if tt.wantNoMetadata {
				if len(result.Metadata) > 0 {
					t.Errorf("expected nil or empty metadata, got %v", result.Metadata)
				}
				return
			}

			for _, key := range tt.wantMetadataKeys {
				if _, ok := result.Metadata[key]; !ok {
					t.Errorf("expected key %q in metadata, got %v", key, result.Metadata)
				}
			}
		})
	}
}

func TestEPUBExtractor_Extract_SubjectsAsSlice(t *testing.T) {
	t.Parallel()

	filePath := createTestEPUB(t, epubOpts{
		meta: &epubMeta{
			Title:    "Subjects Test",
			Subjects: []string{"Science", "Technology", "Education"},
		},
		chapters: []string{"<p>body</p>"},
	})

	ext := epub.New()
	result, err := ext.Extract(context.Background(), filePath)
	if err != nil {
		t.Fatalf("Extract() unexpected error: %v", err)
	}

	subjects, ok := result.Metadata["subjects"]
	if !ok {
		t.Fatal("expected subjects in metadata")
	}

	subjectSlice, ok := subjects.([]string)
	if !ok {
		t.Fatalf("subjects should be []string, got %T", subjects)
	}

	if len(subjectSlice) != 3 {
		t.Errorf("subjects length = %d, want 3", len(subjectSlice))
	}

	want := []string{"Science", "Technology", "Education"}
	for i, s := range want {
		if i < len(subjectSlice) && subjectSlice[i] != s {
			t.Errorf("subjects[%d] = %q, want %q", i, subjectSlice[i], s)
		}
	}
}

func TestEPUBExtractor_Extract_NonExistentFile(t *testing.T) {
	t.Parallel()

	ext := epub.New()
	_, err := ext.Extract(context.Background(), "/nonexistent/path/file.epub")
	if err == nil {
		t.Fatal("Extract() expected error for non-existent file, got nil")
	}
}

func TestEPUBExtractor_Extract_InvalidZip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "bad.epub")
	if err := os.WriteFile(filePath, []byte("this is not a zip file"), 0o600); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}

	ext := epub.New()
	_, err := ext.Extract(context.Background(), filePath)
	if err == nil {
		t.Fatal("Extract() expected error for invalid ZIP, got nil")
	}
}

func TestEPUBExtractor_Extract_CanceledContext(t *testing.T) {
	t.Parallel()

	filePath := createTestEPUB(t, epubOpts{
		chapters: []string{"<p>text</p>"},
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ext := epub.New()
	_, err := ext.Extract(ctx, filePath)
	if err == nil {
		t.Fatal("Extract() expected error for canceled context, got nil")
	}
}

func TestEPUBExtractor_Extract_MissingContainerXML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "no-container.epub")

	f, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}

	zw := zip.NewWriter(f)
	w, err := zw.Create("dummy.txt")
	if err != nil {
		t.Fatalf("creating dummy in zip: %v", err)
	}
	if _, err = w.Write([]byte("hello")); err != nil {
		t.Fatalf("writing dummy: %v", err)
	}
	if err = zw.Close(); err != nil {
		t.Fatalf("closing zip writer: %v", err)
	}
	if err = f.Close(); err != nil {
		t.Fatalf("closing file: %v", err)
	}

	ext := epub.New()
	_, err = ext.Extract(context.Background(), filePath)
	if err == nil {
		t.Fatal("Extract() expected error for missing container.xml, got nil")
	}
}

func TestEPUBExtractor_Extract_ZipBombFileCount(t *testing.T) {
	t.Parallel()

	// Create an EPUB with many files, then use NewWithLimits with a low cap.
	dir := t.TempDir()
	filePath := filepath.Join(dir, "many-files.epub")

	f, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}

	zw := zip.NewWriter(f)
	// Write 10 dummy files.
	for i := range 10 {
		w, err := zw.Create("file" + itoa(i) + ".txt")
		if err != nil {
			t.Fatalf("creating file in zip: %v", err)
		}
		if _, err := w.Write([]byte("data")); err != nil {
			t.Fatalf("writing file: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("closing zip writer: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("closing file: %v", err)
	}

	ext := epub.NewWithLimits(2, 0) // max 2 files
	_, err = ext.Extract(context.Background(), filePath)
	if err == nil {
		t.Fatal("Extract() expected error for zip bomb file count, got nil")
	}
	if !strings.Contains(err.Error(), "exceeding limit") {
		t.Errorf("error should mention exceeding limit, got: %v", err)
	}
}

func TestEPUBExtractor_Extract_NoChapters(t *testing.T) {
	t.Parallel()

	filePath := createTestEPUB(t, epubOpts{
		chapters: []string{},
	})

	ext := epub.New()
	_, err := ext.Extract(context.Background(), filePath)
	if err == nil {
		t.Fatal("Extract() expected error for EPUB with no chapters, got nil")
	}
	if !strings.Contains(err.Error(), "no extractable chapter content") {
		t.Errorf("error should mention no extractable content, got: %v", err)
	}
}

func TestEPUBExtractor_Supports(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mimeType string
		want     bool
	}{
		{"application/epub+zip", true},
		{"text/html", false},
		{"application/pdf", false},
		{"application/zip", false},
		{"", false},
		{"APPLICATION/EPUB+ZIP", false},
	}

	ext := epub.New()
	for _, tt := range tests {
		t.Run(tt.mimeType, func(t *testing.T) {
			t.Parallel()

			if got := ext.Supports(tt.mimeType); got != tt.want {
				t.Errorf("Supports(%q) = %v, want %v", tt.mimeType, got, tt.want)
			}
		})
	}
}

func TestEPUBExtractor_Extract_SampleEPUB(t *testing.T) {
	t.Parallel()

	_, thisFile, _, _ := runtime.Caller(0)
	fixturePath := filepath.Join(filepath.Dir(thisFile), "..", "..", "testutil", "testdata", "sample.epub")

	ext := epub.New()
	result, err := ext.Extract(context.Background(), fixturePath)
	if err != nil {
		t.Fatalf("Extract() unexpected error: %v", err)
	}

	if result.Content == "" {
		t.Fatal("Extract() Content is empty, want non-empty")
	}

	if result.WordCount <= 0 {
		t.Errorf("Extract() WordCount = %d, want > 0", result.WordCount)
	}

	// Verify metadata was extracted from OPF Dublin Core.
	if result.Metadata == nil {
		t.Fatal("Extract() Metadata is nil, want non-nil map")
	}

	if title, ok := result.Metadata["title"].(string); !ok || title == "" {
		t.Errorf("Metadata[\"title\"] = %v, want non-empty string", result.Metadata["title"])
	} else if title != "EPUB 3.0 Specification" {
		t.Errorf("Metadata[\"title\"] = %q, want %q", title, "EPUB 3.0 Specification")
	}

	if creator, ok := result.Metadata["creator"].(string); !ok || creator == "" {
		t.Errorf("Metadata[\"creator\"] = %v, want non-empty string", result.Metadata["creator"])
	}

	if lang, ok := result.Metadata["language"].(string); !ok || lang != "en" {
		t.Errorf("Metadata[\"language\"] = %v, want \"en\"", result.Metadata["language"])
	}

	// Verify metadata is baked into content header for FTS.
	if !strings.Contains(result.Content, "EPUB 3.0 Specification") {
		t.Error("Content should contain title in metadata header")
	}
	if !strings.Contains(result.Content, "EPUB 3 Working Group") {
		t.Error("Content should contain author in metadata header")
	}

	// Verify chapter text was extracted (not just metadata).
	if !strings.Contains(result.Content, "Open Container Format") {
		t.Error("Content should contain chapter text about Open Container Format")
	}

	// Sanity check: real EPUB should produce substantial content.
	if result.WordCount < 1000 {
		t.Errorf("WordCount = %d, want > 1000 for EPUB 3.0 spec", result.WordCount)
	}
}
