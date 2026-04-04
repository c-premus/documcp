// Package archive provides utilities for building in-memory zip and tar.gz
// archives from a list of path/content entries. All paths are sanitized to
// prevent directory-traversal attacks (entries with ".." prefixes or absolute
// paths are silently skipped).
package archive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"path/filepath"
	"strings"
)

// Entry holds a single file's path and content for archive creation.
type Entry struct {
	Path    string
	Content string
}

// BuildZip writes a zip archive containing the given entries to w.
// Entries with paths that would escape the archive root are skipped.
func BuildZip(w *bytes.Buffer, entries []Entry) error {
	zw := zip.NewWriter(w)
	for _, e := range entries {
		clean := filepath.ToSlash(filepath.Clean(e.Path))
		if clean == ".." || strings.HasPrefix(clean, "../") || filepath.IsAbs(e.Path) {
			continue // skip paths that would escape the archive root
		}
		fw, err := zw.Create(clean)
		if err != nil {
			return fmt.Errorf("creating zip entry %q: %w", e.Path, err)
		}
		if _, err := fw.Write([]byte(e.Content)); err != nil {
			return fmt.Errorf("writing zip entry %q: %w", e.Path, err)
		}
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("closing zip writer: %w", err)
	}
	return nil
}

// BuildTarGz writes a gzip-compressed tar archive containing the given entries to w.
// Entries with paths that would escape the archive root are skipped.
func BuildTarGz(w *bytes.Buffer, entries []Entry) error {
	gw := gzip.NewWriter(w)
	tw := tar.NewWriter(gw)

	for _, e := range entries {
		clean := filepath.ToSlash(filepath.Clean(e.Path))
		if clean == ".." || strings.HasPrefix(clean, "../") || filepath.IsAbs(e.Path) {
			continue // skip paths that would escape the archive root
		}
		hdr := &tar.Header{
			Name: clean,
			Mode: 0o600,
			Size: int64(len(e.Content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("writing tar header for %q: %w", e.Path, err)
		}
		if _, err := tw.Write([]byte(e.Content)); err != nil {
			return fmt.Errorf("writing tar entry %q: %w", e.Path, err)
		}
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("closing tar writer: %w", err)
	}
	if err := gw.Close(); err != nil {
		return fmt.Errorf("closing gzip writer: %w", err)
	}
	return nil
}
