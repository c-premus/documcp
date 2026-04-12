package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
)

// FSBlob is a filesystem-backed Blob rooted at a base directory. All file
// operations go through *os.Root, which uses openat2(RESOLVE_BENEATH) on
// Linux to prevent symlink escapes at the syscall level. Callers never need
// security.SafeStoragePath — the root handle itself is the boundary.
type FSBlob struct {
	rootPath string
	root     *os.Root
}

// NewFSBlob returns a filesystem-backed Blob. baseDir must exist and be a
// directory; FSBlob takes ownership of an *os.Root handle until Close is
// called.
func NewFSBlob(baseDir string) (*FSBlob, error) {
	if baseDir == "" {
		return nil, errors.New("fsblob: baseDir is required")
	}
	r, err := os.OpenRoot(baseDir)
	if err != nil {
		return nil, fmt.Errorf("fsblob: open root %q: %w", baseDir, err)
	}
	return &FSBlob{rootPath: baseDir, root: r}, nil
}

// Root returns the base directory. Exposed for wiring code that still needs
// a plain filesystem path (worker temp dir, git clone dir). New call sites
// should prefer the Blob interface.
func (b *FSBlob) Root() string { return b.rootPath }

// NewWriter creates or overwrites the object at key. Parent directories are
// created on demand with mode 0o750.
func (b *FSBlob) NewWriter(_ context.Context, key string, _ *WriterOpts) (io.WriteCloser, error) {
	if err := ValidateKey(key); err != nil {
		return nil, err
	}
	if err := b.mkdirAll(path.Dir(key), 0o750); err != nil {
		return nil, fmt.Errorf("fsblob: create parent dir: %w", err)
	}
	f, err := b.root.Create(key)
	if err != nil {
		return nil, fmt.Errorf("fsblob: create file: %w", err)
	}
	return f, nil
}

// mkdirAll emulates os.MkdirAll using os.Root.Mkdir (which creates only one
// level at a time). Stops at "." / "" / "/" which represent the root itself.
func (b *FSBlob) mkdirAll(dir string, perm os.FileMode) error {
	if dir == "." || dir == "" || dir == "/" {
		return nil
	}
	parent := path.Dir(dir)
	if parent != dir {
		if err := b.mkdirAll(parent, perm); err != nil {
			return err
		}
	}
	if err := b.root.Mkdir(dir, perm); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}
	return nil
}

// NewReader opens the object at key for reading.
func (b *FSBlob) NewReader(_ context.Context, key string) (io.ReadCloser, error) {
	if err := ValidateKey(key); err != nil {
		return nil, err
	}
	f, err := b.root.Open(key)
	if err != nil {
		return nil, err // errors.Is(err, fs.ErrNotExist) works through *PathError
	}
	return f, nil
}

// NewRangeReader reads length bytes starting at offset. A negative length
// reads to end-of-object.
func (b *FSBlob) NewRangeReader(_ context.Context, key string, offset, length int64) (io.ReadCloser, error) {
	if err := ValidateKey(key); err != nil {
		return nil, err
	}
	if offset < 0 {
		return nil, fmt.Errorf("fsblob: negative offset %d", offset)
	}
	f, err := b.root.Open(key)
	if err != nil {
		return nil, err
	}
	if offset > 0 {
		if _, seekErr := f.Seek(offset, io.SeekStart); seekErr != nil {
			_ = f.Close()
			return nil, fmt.Errorf("fsblob: seek: %w", seekErr)
		}
	}
	if length < 0 {
		return f, nil
	}
	return &rangeReadCloser{r: io.LimitReader(f, length), c: f}, nil
}

// rangeReadCloser bundles a limited reader with the underlying file closer.
type rangeReadCloser struct {
	r io.Reader
	c io.Closer
}

func (r *rangeReadCloser) Read(p []byte) (int, error) { return r.r.Read(p) }

// Close releases the underlying file.
func (r *rangeReadCloser) Close() error { return r.c.Close() }

// Delete removes the object at key. Returns nil when the key is already absent.
func (b *FSBlob) Delete(_ context.Context, key string) error {
	if err := ValidateKey(key); err != nil {
		return err
	}
	if err := b.root.Remove(key); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("fsblob: remove: %w", err)
	}
	return nil
}

// Stat returns the object's size and modification time. The ETag is a weak
// validator derived from size and mtime — sufficient for HTTP conditional GETs
// but not for byte-exact comparison (use S3Blob for strong validators).
func (b *FSBlob) Stat(_ context.Context, key string) (Attrs, error) {
	if err := ValidateKey(key); err != nil {
		return Attrs{}, err
	}
	info, err := b.root.Stat(key)
	if err != nil {
		return Attrs{}, err
	}
	return Attrs{
		Size:    info.Size(),
		ModTime: info.ModTime(),
		ETag:    fmt.Sprintf(`W/"%d-%d"`, info.Size(), info.ModTime().Unix()),
	}, nil
}

// List returns an iterator over keys under prefix. Walks the filesystem
// eagerly into a slice — acceptable for DocuMCP's expected object count per
// cleanup run (thousands, not millions).
func (b *FSBlob) List(ctx context.Context, prefix string) Iterator {
	return newFSIterator(ctx, b.root, prefix)
}

// Close releases the root handle.
func (b *FSBlob) Close() error { return b.root.Close() }

// fsIterator walks the filesystem eagerly on construction and yields keys
// from an internal slice. Directory entries are skipped.
type fsIterator struct {
	keys []string
	idx  int
	err  error
}

func newFSIterator(ctx context.Context, root *os.Root, prefix string) *fsIterator {
	it := &fsIterator{}

	start := "."
	if prefix != "" {
		start = path.Clean(prefix)
		if _, err := root.Stat(start); errors.Is(err, fs.ErrNotExist) {
			return it
		}
	}

	walkErr := fs.WalkDir(root.FS(), start, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if d.IsDir() {
			return nil
		}
		// fs.WalkDir on os.Root.FS() yields paths relative to the root.
		it.keys = append(it.keys, p)
		return nil
	})
	if walkErr != nil && !errors.Is(walkErr, fs.ErrNotExist) {
		it.err = walkErr
	}
	return it
}

// Next advances to the next key. Returns false when exhausted or on error.
func (it *fsIterator) Next(_ context.Context) bool {
	if it.err != nil || it.idx >= len(it.keys) {
		return false
	}
	it.idx++
	return true
}

// Key returns the current key. Only valid after Next returns true.
func (it *fsIterator) Key() string {
	if it.idx == 0 || it.idx > len(it.keys) {
		return ""
	}
	return it.keys[it.idx-1]
}

// Err returns the first error encountered during iteration, if any.
func (it *fsIterator) Err() error { return it.err }
