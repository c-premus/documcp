package archive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"testing"
)

func TestBuildZip(t *testing.T) {
	t.Parallel()

	t.Run("creates valid zip with entries", func(t *testing.T) {
		t.Parallel()

		entries := []Entry{
			{Path: "README.md", Content: "# Hello"},
			{Path: "src/main.go", Content: "package main"},
		}
		var buf bytes.Buffer
		if err := BuildZip(&buf, entries); err != nil {
			t.Fatalf("BuildZip error: %v", err)
		}

		r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			t.Fatalf("reading zip: %v", err)
		}
		if len(r.File) != 2 {
			t.Fatalf("zip file count = %d, want 2", len(r.File))
		}
		if r.File[0].Name != "README.md" {
			t.Errorf("file[0].Name = %q, want README.md", r.File[0].Name)
		}
	})

	t.Run("empty entries produces valid empty zip", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		if err := BuildZip(&buf, nil); err != nil {
			t.Fatalf("BuildZip error: %v", err)
		}
		r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			t.Fatalf("reading zip: %v", err)
		}
		if len(r.File) != 0 {
			t.Errorf("zip file count = %d, want 0", len(r.File))
		}
	})

	t.Run("skips path traversal entries", func(t *testing.T) {
		t.Parallel()

		entries := []Entry{
			{Path: "good.txt", Content: "ok"},
			{Path: "../escape.txt", Content: "bad"},
			{Path: "../../etc/passwd", Content: "bad"},
			{Path: "/absolute/path.txt", Content: "bad"},
			{Path: "nested/../ok.txt", Content: "cleaned"},
		}
		var buf bytes.Buffer
		if err := BuildZip(&buf, entries); err != nil {
			t.Fatalf("BuildZip error: %v", err)
		}

		r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			t.Fatalf("reading zip: %v", err)
		}
		// Only "good.txt" and "nested/../ok.txt" (cleaned to "ok.txt") should survive
		if len(r.File) != 2 {
			names := make([]string, len(r.File))
			for i, f := range r.File {
				names[i] = f.Name
			}
			t.Fatalf("zip file count = %d, want 2; files: %v", len(r.File), names)
		}
	})
}

func TestBuildTarGz(t *testing.T) {
	t.Parallel()

	t.Run("creates valid tar.gz with entries", func(t *testing.T) {
		t.Parallel()

		entries := []Entry{
			{Path: "README.md", Content: "# Hello"},
			{Path: "src/main.go", Content: "package main"},
		}
		var buf bytes.Buffer
		if err := BuildTarGz(&buf, entries); err != nil {
			t.Fatalf("BuildTarGz error: %v", err)
		}

		gr, err := gzip.NewReader(&buf)
		if err != nil {
			t.Fatalf("gzip reader: %v", err)
		}
		defer gr.Close()

		tr := tar.NewReader(gr)
		var count int
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("tar next: %v", err)
			}
			count++
			if count == 1 && hdr.Name != "README.md" {
				t.Errorf("entry[0].Name = %q, want README.md", hdr.Name)
			}
		}
		if count != 2 {
			t.Errorf("tar entry count = %d, want 2", count)
		}
	})

	t.Run("skips path traversal entries", func(t *testing.T) {
		t.Parallel()

		entries := []Entry{
			{Path: "good.txt", Content: "ok"},
			{Path: "../escape.txt", Content: "bad"},
			{Path: "/absolute/path.txt", Content: "bad"},
		}
		var buf bytes.Buffer
		if err := BuildTarGz(&buf, entries); err != nil {
			t.Fatalf("BuildTarGz error: %v", err)
		}

		gr, err := gzip.NewReader(&buf)
		if err != nil {
			t.Fatalf("gzip reader: %v", err)
		}
		defer gr.Close()

		tr := tar.NewReader(gr)
		var count int
		for {
			_, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("tar next: %v", err)
			}
			count++
		}
		if count != 1 {
			t.Errorf("tar entry count = %d, want 1 (only good.txt)", count)
		}
	})
}
