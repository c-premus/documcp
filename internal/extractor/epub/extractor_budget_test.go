package epub

import (
	"archive/zip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/microcosm-cc/bluemonday"

	"github.com/c-premus/documcp/internal/extractor/ziputil"
)

// White-box tests for the cumulative decompression budget wired into the EPUB
// extractor helpers.
//
// The budget reader is a defense-in-depth layer: archive/zip.checksumReader
// already bounds real reads by the central-directory UncompressedSize64 claim,
// and the extractor's preflight sums those claims. These tests drive the
// helpers directly with a pre-seeded shared counter — the only way to exercise
// the runtime-counter branch without subverting the stdlib. They prove the
// BudgetReader is wired into every ZIP-entry open path inside a single
// Extract call, so a future regression that drops the counter from one helper
// would be caught here.

// buildBudgetTestEPUB writes a minimal valid EPUB (stored mimetype, deflated
// container/OPF/chapter) to a temp file and returns the path plus the
// in-archive paths for the chapter and OPF so helper tests can open them
// directly.
func buildBudgetTestEPUB(t *testing.T, chapterBody string) (epubPath, chapterPath, opfPath string) {
	t.Helper()

	opfPath = "OEBPS/content.opf"
	chapterPath = "OEBPS/chapter1.xhtml"
	containerXML := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0">` +
		`<rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>` +
		`</container>`
	opf := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<package xmlns="http://www.idpf.org/2007/opf" version="3.0">` +
		`<metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>t</dc:title></metadata>` +
		`<manifest><item id="ch1" href="chapter1.xhtml" media-type="application/xhtml+xml"/></manifest>` +
		`<spine><itemref idref="ch1"/></spine>` +
		`</package>`
	chapter := `<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE html>` +
		`<html xmlns="http://www.w3.org/1999/xhtml"><head><title>c</title></head>` +
		`<body>` + chapterBody + `</body></html>`

	dir := t.TempDir()
	epubPath = filepath.Join(dir, "test.epub")
	f, err := os.Create(epubPath)
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	defer func() { _ = f.Close() }()

	zw := zip.NewWriter(f)
	mh := &zip.FileHeader{Name: "mimetype", Method: zip.Store}
	mw, err := zw.CreateHeader(mh)
	if err != nil {
		t.Fatalf("create mimetype: %v", err)
	}
	if _, err := mw.Write([]byte("application/epub+zip")); err != nil {
		t.Fatalf("write mimetype: %v", err)
	}
	for _, entry := range []struct{ name, content string }{
		{"META-INF/container.xml", containerXML},
		{opfPath, opf},
		{chapterPath, chapter},
	} {
		w, err := zw.Create(entry.name)
		if err != nil {
			t.Fatalf("create %s: %v", entry.name, err)
		}
		if _, err := w.Write([]byte(entry.content)); err != nil {
			t.Fatalf("write %s: %v", entry.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	return epubPath, chapterPath, opfPath
}

// TestExtractChapterText_TripsBudgetReader proves the BudgetReader is wired
// into extractChapterText: when the shared counter already sits above the cap,
// the chapter read trips ErrBudgetExceeded.
func TestExtractChapterText_TripsBudgetReader(t *testing.T) {
	t.Parallel()

	epubPath, chapterPath, _ := buildBudgetTestEPUB(t, "<p>"+strings.Repeat("x", 512)+"</p>")
	zr, err := zip.OpenReader(epubPath)
	if err != nil {
		t.Fatalf("opening fixture: %v", err)
	}
	defer func() { _ = zr.Close() }()

	total := int64(2048) // already over the 512-byte cap
	_, err = extractChapterText(zr, chapterPath, 10*1024*1024, &total, 512, bluemonday.UGCPolicy())
	if !errors.Is(err, ziputil.ErrBudgetExceeded) {
		t.Fatalf("err = %v, want wrapped ErrBudgetExceeded", err)
	}
}

// TestFindContainerRootfile_TripsBudgetReader proves the same wiring on the
// container.xml path by asserting the shared counter advances. A seeded
// counter can't always force an error here because container.xml is tiny and
// the XML decoder may complete its parse in a single buffered Read — but the
// counter still increments, which is exactly what downstream helpers rely on.
func TestFindContainerRootfile_TripsBudgetReader(t *testing.T) {
	t.Parallel()

	epubPath, _, _ := buildBudgetTestEPUB(t, "<p>ok</p>")
	zr, err := zip.OpenReader(epubPath)
	if err != nil {
		t.Fatalf("opening fixture: %v", err)
	}
	defer func() { _ = zr.Close() }()

	var total int64
	if _, err := findContainerRootfile(zr, 10*1024*1024, &total, 10*1024*1024); err != nil {
		t.Fatalf("findContainerRootfile() unexpected error: %v", err)
	}
	if total == 0 {
		t.Fatal("BudgetReader not wired: total did not advance after reading container.xml")
	}
}

// TestParseOPF_TripsBudgetReader proves the same wiring on the OPF path.
// Like the container.xml test, a seeded counter can't reliably force an error
// because the OPF fits in a single buffered Read — the assertion is that the
// counter advances, which confirms the BudgetReader is on the read path.
func TestParseOPF_TripsBudgetReader(t *testing.T) {
	t.Parallel()

	epubPath, _, opfPath := buildBudgetTestEPUB(t, "<p>ok</p>")
	zr, err := zip.OpenReader(epubPath)
	if err != nil {
		t.Fatalf("opening fixture: %v", err)
	}
	defer func() { _ = zr.Close() }()

	var total int64
	if _, err := parseOPF(zr, opfPath, 10*1024*1024, &total, 10*1024*1024); err != nil {
		t.Fatalf("parseOPF() unexpected error: %v", err)
	}
	if total == 0 {
		t.Fatal("BudgetReader not wired: total did not advance after reading OPF")
	}
}

// TestExtract_BudgetPathNeverFallsBackToNoContentError pins the spine-loop
// behavior change: whichever decompression-budget gate trips first (preflight
// summing central-dir claims, or BudgetReader on real reads), Extract must
// surface that error — not silently swallow it into the generic
// "no extractable chapter content" sentinel that otherwise fires when the
// chapters slice ends up empty.
func TestExtract_BudgetPathNeverFallsBackToNoContentError(t *testing.T) {
	t.Parallel()

	body := strings.Repeat("z", 1024)
	epubPath, _, _ := buildBudgetTestEPUB(t, "<p>"+body+"</p>")

	ext := NewWithAllLimits(0, 10*1024*1024, 1024)
	_, err := ext.Extract(context.Background(), epubPath)
	if err == nil {
		t.Fatal("Extract() expected an error, got nil")
	}
	if strings.Contains(err.Error(), "no extractable chapter content") {
		t.Errorf("budget exhaustion must not surface as generic no-content error: %v", err)
	}
}

// TestExtract_PreflightRejectsHonestOversizedHeaders covers the first gate:
// an EPUB whose honest central-directory sizes sum above the cap is rejected
// before any entry is opened.
func TestExtract_PreflightRejectsHonestOversizedHeaders(t *testing.T) {
	t.Parallel()

	bulk := strings.Repeat("B", 4000)
	epubPath, _, _ := buildBudgetTestEPUB(t, "<p>"+bulk+"</p>")

	ext := NewWithAllLimits(0, 10*1024*1024, 512)
	_, err := ext.Extract(context.Background(), epubPath)
	if err == nil {
		t.Fatal("Extract() expected preflight error, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds budget") {
		t.Errorf("expected preflight budget error, got: %v", err)
	}
	// Preflight runs before BudgetReader; this error is not the runtime
	// sentinel.
	if errors.Is(err, ziputil.ErrBudgetExceeded) {
		t.Errorf("preflight rejection should not wrap ErrBudgetExceeded: %v", err)
	}
}

// TestNewWithAllLimits_Defaults verifies zero values fall back to defaults.
func TestNewWithAllLimits_Defaults(t *testing.T) {
	t.Parallel()

	got := NewWithAllLimits(0, 0, 0)
	want := New()
	if got.maxZIPFiles != want.maxZIPFiles {
		t.Errorf("maxZIPFiles = %d, want %d", got.maxZIPFiles, want.maxZIPFiles)
	}
	if got.maxDecompressedFileSize != want.maxDecompressedFileSize {
		t.Errorf("maxDecompressedFileSize = %d, want %d", got.maxDecompressedFileSize, want.maxDecompressedFileSize)
	}
	if got.maxTotalDecompressed != want.maxTotalDecompressed {
		t.Errorf("maxTotalDecompressed = %d, want %d", got.maxTotalDecompressed, want.maxTotalDecompressed)
	}
}

// TestNewWithAllLimits_Overrides verifies non-zero values override defaults.
func TestNewWithAllLimits_Overrides(t *testing.T) {
	t.Parallel()

	e := NewWithAllLimits(7, 11, 13)
	if e.maxZIPFiles != 7 {
		t.Errorf("maxZIPFiles = %d, want 7", e.maxZIPFiles)
	}
	if e.maxDecompressedFileSize != 11 {
		t.Errorf("maxDecompressedFileSize = %d, want 11", e.maxDecompressedFileSize)
	}
	if e.maxTotalDecompressed != 13 {
		t.Errorf("maxTotalDecompressed = %d, want 13", e.maxTotalDecompressed)
	}
}
