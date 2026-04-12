package storage_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"strings"
	"testing"

	"github.com/c-premus/documcp/internal/storage"
)

// blobFactory constructs a fresh Blob backend for each subtest. The returned
// blob must be empty.
type blobFactory func(t *testing.T) storage.Blob

// RunBlobSuite exercises a Blob implementation against the shared contract.
// Used by both the filesystem and S3 backends.
func RunBlobSuite(t *testing.T, newBlob blobFactory) {
	t.Helper()

	t.Run("WriteThenRead", func(t *testing.T) {
		t.Parallel()
		b := newBlob(t)
		ctx := context.Background()
		writeKey(t, b, "pdf/a.pdf", []byte("hello world"))

		got := readAll(t, b, "pdf/a.pdf")
		if string(got) != "hello world" {
			t.Errorf("read = %q, want %q", got, "hello world")
		}

		// Key that doesn't exist.
		_, err := b.NewReader(ctx, "pdf/nope.pdf")
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("NewReader missing key: err = %v, want fs.ErrNotExist", err)
		}
	})

	t.Run("OverwriteReplacesContent", func(t *testing.T) {
		t.Parallel()
		b := newBlob(t)
		writeKey(t, b, "k/v", []byte("first"))
		writeKey(t, b, "k/v", []byte("second"))
		got := readAll(t, b, "k/v")
		if string(got) != "second" {
			t.Errorf("overwritten content = %q, want %q", got, "second")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		t.Parallel()
		b := newBlob(t)
		ctx := context.Background()
		writeKey(t, b, "del/me.txt", []byte("bye"))
		if err := b.Delete(ctx, "del/me.txt"); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		_, err := b.NewReader(ctx, "del/me.txt")
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("post-delete NewReader: err = %v, want fs.ErrNotExist", err)
		}
	})

	t.Run("DeleteMissingIsNoop", func(t *testing.T) {
		t.Parallel()
		b := newBlob(t)
		if err := b.Delete(context.Background(), "never/existed.txt"); err != nil {
			t.Errorf("Delete missing key should be no-op, got: %v", err)
		}
	})

	t.Run("Stat", func(t *testing.T) {
		t.Parallel()
		b := newBlob(t)
		payload := []byte("size me up")
		writeKey(t, b, "stat/obj.bin", payload)

		attrs, err := b.Stat(context.Background(), "stat/obj.bin")
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if attrs.Size != int64(len(payload)) {
			t.Errorf("Stat.Size = %d, want %d", attrs.Size, len(payload))
		}
		if attrs.ETag == "" {
			t.Error("Stat.ETag should not be empty")
		}
		if attrs.ModTime.IsZero() {
			t.Error("Stat.ModTime should not be zero")
		}

		_, err = b.Stat(context.Background(), "stat/missing.bin")
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Stat missing: err = %v, want fs.ErrNotExist", err)
		}
	})

	t.Run("NewRangeReader", func(t *testing.T) {
		t.Parallel()
		b := newBlob(t)
		writeKey(t, b, "range/data.bin", []byte("0123456789"))

		// Middle slice.
		rc, err := b.NewRangeReader(context.Background(), "range/data.bin", 3, 4)
		if err != nil {
			t.Fatalf("NewRangeReader: %v", err)
		}
		got, _ := io.ReadAll(rc)
		_ = rc.Close()
		if string(got) != "3456" {
			t.Errorf("range [3,4) = %q, want %q", got, "3456")
		}

		// Offset with length -1 reads to end.
		rc2, err := b.NewRangeReader(context.Background(), "range/data.bin", 7, -1)
		if err != nil {
			t.Fatalf("NewRangeReader to end: %v", err)
		}
		got2, _ := io.ReadAll(rc2)
		_ = rc2.Close()
		if string(got2) != "789" {
			t.Errorf("range [7,EOF) = %q, want %q", got2, "789")
		}

		// Offset 0, length equal to size reads everything.
		rc3, err := b.NewRangeReader(context.Background(), "range/data.bin", 0, 10)
		if err != nil {
			t.Fatalf("NewRangeReader full: %v", err)
		}
		got3, _ := io.ReadAll(rc3)
		_ = rc3.Close()
		if string(got3) != "0123456789" {
			t.Errorf("range [0,10) = %q, want %q", got3, "0123456789")
		}
	})

	t.Run("List", func(t *testing.T) {
		t.Parallel()
		b := newBlob(t)
		writeKey(t, b, "pdf/a.pdf", []byte("a"))
		writeKey(t, b, "pdf/b.pdf", []byte("b"))
		writeKey(t, b, "html/c.html", []byte("c"))

		// All keys.
		allKeys := collectKeys(t, b, "")
		if len(allKeys) != 3 {
			t.Errorf("List all: got %d keys, want 3 (%v)", len(allKeys), allKeys)
		}

		// Prefixed keys.
		pdfKeys := collectKeys(t, b, "pdf/")
		if len(pdfKeys) != 2 {
			t.Errorf("List pdf/: got %d keys, want 2 (%v)", len(pdfKeys), pdfKeys)
		}
		for _, k := range pdfKeys {
			if !strings.HasPrefix(k, "pdf/") {
				t.Errorf("List pdf/: got key %q without prefix", k)
			}
		}

		// Empty prefix on empty blob.
		b2 := newBlob(t)
		emptyKeys := collectKeys(t, b2, "")
		if len(emptyKeys) != 0 {
			t.Errorf("List empty blob: got %d keys, want 0", len(emptyKeys))
		}
	})

	t.Run("RejectsInvalidKey", func(t *testing.T) {
		t.Parallel()
		b := newBlob(t)
		ctx := context.Background()
		bad := []string{
			"",
			"/abs/path",
			"..",
			"../etc/passwd",
			"a/../b",
			"a//b",
			"./a",
			"a/./b",
			"null\x00byte",
		}
		for _, key := range bad {
			if _, err := b.NewWriter(ctx, key, nil); !errors.Is(err, storage.ErrInvalidKey) {
				t.Errorf("NewWriter(%q): err = %v, want ErrInvalidKey", key, err)
			}
			if _, err := b.NewReader(ctx, key); !errors.Is(err, storage.ErrInvalidKey) {
				t.Errorf("NewReader(%q): err = %v, want ErrInvalidKey", key, err)
			}
		}
	})
}

// writeKey writes content to key, failing the test on any error.
func writeKey(t *testing.T, b storage.Blob, key string, content []byte) {
	t.Helper()
	w, err := b.NewWriter(context.Background(), key, nil)
	if err != nil {
		t.Fatalf("NewWriter(%q): %v", key, err)
	}
	if _, err := io.Copy(w, bytes.NewReader(content)); err != nil {
		_ = w.Close()
		t.Fatalf("write(%q): %v", key, err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close(%q): %v", key, err)
	}
}

// readAll opens key and returns its full contents.
func readAll(t *testing.T, b storage.Blob, key string) []byte {
	t.Helper()
	r, err := b.NewReader(context.Background(), key)
	if err != nil {
		t.Fatalf("NewReader(%q): %v", key, err)
	}
	defer func() { _ = r.Close() }()
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read(%q): %v", key, err)
	}
	return got
}

// collectKeys drains a List iterator into a slice.
func collectKeys(t *testing.T, b storage.Blob, prefix string) []string {
	t.Helper()
	ctx := context.Background()
	var keys []string
	it := b.List(ctx, prefix)
	for it.Next(ctx) {
		keys = append(keys, it.Key())
	}
	if err := it.Err(); err != nil {
		t.Fatalf("List(%q) iterator err: %v", prefix, err)
	}
	return keys
}

// TestFSBlob runs the shared suite against the filesystem backend.
func TestFSBlob(t *testing.T) {
	t.Parallel()
	RunBlobSuite(t, func(t *testing.T) storage.Blob {
		t.Helper()
		b, err := storage.NewFSBlob(t.TempDir())
		if err != nil {
			t.Fatalf("NewFSBlob: %v", err)
		}
		return b
	})
}

// TestFSBlob_NewFSBlob_ValidatesBaseDir covers the construction-time guards.
func TestFSBlob_NewFSBlob_ValidatesBaseDir(t *testing.T) {
	t.Parallel()

	if _, err := storage.NewFSBlob(""); err == nil {
		t.Error("NewFSBlob(\"\"): want error, got nil")
	}
	if _, err := storage.NewFSBlob("/path/does/not/exist/xyz"); err == nil {
		t.Error("NewFSBlob(nonexistent): want error, got nil")
	}
}
