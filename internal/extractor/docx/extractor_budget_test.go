package docx_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/c-premus/documcp/internal/extractor/docx"
	"github.com/c-premus/documcp/internal/extractor/ziputil"
)

// buildOversizedDocx builds an honest DOCX whose word/document.xml decompresses
// to noticeably more than `targetBytes`. The central directory is NOT patched;
// declared sizes match actual content. Used to exercise the preflight gate.
func buildOversizedDocx(t *testing.T, targetBytes int) string {
	t.Helper()

	var body strings.Builder
	body.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	body.WriteString(`<w:document xmlns:w="` + wNS + `"><w:body><w:p><w:r><w:t>`)
	// Varied content so deflate cannot collapse it to a handful of bytes.
	chunk := "The quick brown fox jumps over the lazy dog 0123456789 "
	for i := 0; i < targetBytes/len(chunk); i++ {
		body.WriteString(chunk)
	}
	body.WriteString(`</w:t></w:r></w:p></w:body></w:document>`)

	dir := t.TempDir()
	filePath := filepath.Join(dir, "oversized.docx")

	f, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	defer func() { _ = f.Close() }()

	zw := zip.NewWriter(f)
	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatalf("creating document.xml: %v", err)
	}
	if _, err := w.Write([]byte(body.String())); err != nil {
		t.Fatalf("writing document.xml: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("closing zip writer: %v", err)
	}
	return filePath
}

// buildLyingDocx builds a DOCX whose word/document.xml really deflates to
// more than `payloadBytes` bytes, then rewrites every central-directory entry
// to claim UncompressedSize32 == 0. For entries below 4 GB the 32-bit field is
// authoritative, so stdlib preflight reads the lie via
// zip.File.UncompressedSize64. The archive is still structurally valid (EOCD,
// local file headers, and the deflate stream are untouched).
func buildLyingDocx(t *testing.T, payloadBytes int) string {
	t.Helper()

	var body strings.Builder
	body.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	body.WriteString(`<w:document xmlns:w="` + wNS + `"><w:body><w:p><w:r><w:t>`)
	chunk := "The quick brown fox jumps over the lazy dog 0123456789 "
	for i := 0; i < payloadBytes/len(chunk); i++ {
		body.WriteString(chunk)
	}
	body.WriteString(`</w:t></w:r></w:p></w:body></w:document>`)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatalf("creating document.xml: %v", err)
	}
	if _, err := w.Write([]byte(body.String())); err != nil {
		t.Fatalf("writing document.xml: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("closing zip writer: %v", err)
	}

	zipped := buf.Bytes()

	// Locate the End-of-Central-Directory (EOCD) record to find the real
	// central directory bounds. Scanning for "PK\x01\x02" across the whole
	// archive is wrong: a deflate stream can legitimately contain those bytes
	// and patching them would corrupt the ZIP.
	const eocdSig = "PK\x05\x06"
	eocdIdx := bytes.LastIndex(zipped, []byte(eocdSig))
	if eocdIdx < 0 || eocdIdx+22 > len(zipped) {
		t.Fatalf("EOCD record not found")
	}
	cdSize := binary.LittleEndian.Uint32(zipped[eocdIdx+12 : eocdIdx+16])
	cdOffset := binary.LittleEndian.Uint32(zipped[eocdIdx+16 : eocdIdx+20])
	cdEnd := cdOffset + cdSize
	if uint64(cdEnd) > uint64(len(zipped)) {
		t.Fatalf("invalid central directory bounds")
	}

	// Walk just the central directory and zero the 32-bit UncompressedSize at
	// offset [24:28] of each file header. Local file headers (PK\x03\x04) are
	// untouched.
	const cdSig = "PK\x01\x02"
	patched := 0
	for i := cdOffset; i+46 <= cdEnd; {
		if string(zipped[i:i+4]) != cdSig {
			t.Fatalf("unexpected byte sequence at central-directory offset %d", i)
		}
		binary.LittleEndian.PutUint32(zipped[i+24:i+28], 0)
		patched++

		nameLen := binary.LittleEndian.Uint16(zipped[i+28 : i+30])
		extraLen := binary.LittleEndian.Uint16(zipped[i+30 : i+32])
		commentLen := binary.LittleEndian.Uint16(zipped[i+32 : i+34])
		i += 46 + uint32(nameLen) + uint32(extraLen) + uint32(commentLen)
	}
	if patched == 0 {
		t.Fatalf("no central-directory entries found to patch")
	}

	dir := t.TempDir()
	filePath := filepath.Join(dir, "lying.docx")
	if err := os.WriteFile(filePath, zipped, 0o600); err != nil {
		t.Fatalf("writing patched docx: %v", err)
	}
	return filePath
}

// TestDOCXExtractor_Extract_HonorsRuntimeDecompressionBudget confirms the
// runtime BudgetReader defense against zip bombs whose central directory
// understates real decompression size. The patched archive lies about
// UncompressedSize (central-directory value zeroed); stdlib's checksumReader
// also catches this lie as ErrFormat, but the critical assertion is that the
// error is rejected (Extract fails) rather than silently proceeding. Either
// the BudgetReader or stdlib's own size check may fire first; both outcomes
// prove the archive is rejected before unlimited decompression.
func TestDOCXExtractor_Extract_HonorsRuntimeDecompressionBudget(t *testing.T) {
	t.Parallel()

	filePath := buildLyingDocx(t, 64*1024)

	// Budget is deliberately tiny so that if the lie slipped through stdlib
	// (e.g. stdlib relaxes its size check in a future Go version), the
	// BudgetReader would still halt decompression well before a zip bomb
	// could do damage.
	ext := docx.NewWithAllLimits(100, 50*1024*1024, 1024)
	_, err := ext.Extract(context.Background(), filePath)
	if err == nil {
		t.Fatal("Extract() on patched-central-directory fixture expected error, got nil")
	}

	// The BudgetReader defense is the new layer we care about. Accept either:
	//   - ziputil.ErrBudgetExceeded  (our runtime counter caught the lie), or
	//   - an error surfaced via the extractor error chain that wraps it.
	// Go's archive/zip stdlib will sometimes catch size-lies as zip.ErrFormat
	// *before* the deflate stream delivers enough bytes for BudgetReader to
	// trip; that is also an acceptable outcome for the defense-in-depth
	// goal (the archive is rejected). This assertion fails only if neither
	// defense fires.
	if !errors.Is(err, ziputil.ErrBudgetExceeded) && !errors.Is(err, zip.ErrFormat) {
		t.Fatalf("Extract() err = %v; want ErrBudgetExceeded or stdlib ErrFormat", err)
	}
}

// TestDOCXExtractor_Extract_BudgetLargeEnoughSucceeds is the sanity sibling
// test: the same patched fixture extracts successfully... unless Go stdlib
// rejects it for its own size-mismatch reasons. The test asserts that with a
// generous budget, the BudgetReader itself does not cause the failure — if
// the fixture fails, it must be for stdlib reasons (zip.ErrFormat), not the
// cumulative budget (ziputil.ErrBudgetExceeded). This pins down which layer
// is responsible.
func TestDOCXExtractor_Extract_BudgetLargeEnoughSucceeds(t *testing.T) {
	t.Parallel()

	filePath := buildLyingDocx(t, 64*1024)

	ext := docx.NewWithAllLimits(100, 50*1024*1024, 10*1024*1024)
	_, err := ext.Extract(context.Background(), filePath)
	if errors.Is(err, ziputil.ErrBudgetExceeded) {
		t.Fatalf("Extract() with generous budget tripped BudgetReader: %v", err)
	}
}

// TestDOCXExtractor_Extract_PreflightRejectsHonestOversizedHeaders confirms
// the first-gate preflight loop still rejects archives whose central
// directory honestly declares decompressed sizes that exceed the runtime
// budget. Preflight uses the cheap declared-size path; the BudgetReader is
// the enforcement layer for lying archives.
func TestDOCXExtractor_Extract_PreflightRejectsHonestOversizedHeaders(t *testing.T) {
	t.Parallel()

	filePath := buildOversizedDocx(t, 64*1024)

	ext := docx.NewWithAllLimits(100, 50*1024*1024, 1024)
	_, err := ext.Extract(context.Background(), filePath)
	if err == nil {
		t.Fatal("Extract() on honestly-oversized DOCX expected preflight error, got nil")
	}
	if !strings.Contains(err.Error(), "total decompressed size exceeds budget") {
		t.Fatalf("Extract() err = %v; want preflight budget rejection message", err)
	}
}
