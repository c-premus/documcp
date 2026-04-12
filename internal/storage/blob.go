// Package storage defines a Blob interface and implementations for local
// filesystem and S3-compatible object stores. Documents, uploads, and worker
// scratch data flow through this abstraction instead of touching the
// filesystem directly so replicas can share state via object storage.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"time"
)

// ErrNotExist is returned when a key does not exist in the blob store.
// Backends must ensure errors.Is(err, fs.ErrNotExist) and
// errors.Is(err, ErrNotExist) both succeed for missing-key errors.
var ErrNotExist = fs.ErrNotExist

// ErrInvalidKey is returned when a key fails interface-level validation
// (empty, absolute, contains NUL, contains "." or ".." component).
var ErrInvalidKey = errors.New("invalid blob key")

// Attrs describes an object's metadata.
type Attrs struct {
	// Size in bytes.
	Size int64
	// ModTime is the last-modified timestamp reported by the backend.
	ModTime time.Time
	// ContentType, if set by the backend. May be empty.
	ContentType string
	// ETag is an opaque validator used for HTTP conditional requests.
	// S3 backends typically return the object's MD5; the filesystem backend
	// returns a weak "size-modtime" tag.
	ETag string
}

// WriterOpts configures a blob write operation. All fields are optional.
type WriterOpts struct {
	// ContentType sets the Content-Type metadata on the stored object.
	// The filesystem backend ignores this; S3 backends apply it.
	ContentType string
}

// Iterator iterates blob keys. Usage:
//
//	it := blob.List(ctx, "pdf/")
//	for it.Next(ctx) {
//	    key := it.Key()
//	    ...
//	}
//	if err := it.Err(); err != nil { ... }
type Iterator interface {
	// Next advances to the next key. Returns false when exhausted or on error.
	Next(ctx context.Context) bool
	// Key returns the current key. Only valid after Next returns true.
	Key() string
	// Err returns the first error encountered during iteration, if any.
	Err() error
}

// Blob is the blob storage abstraction. Keys are forward-slash separated and
// must not begin with "/", contain "." or ".." components, or contain NUL.
// Implementations must be safe for concurrent use.
type Blob interface {
	// NewWriter returns a writer that stores the object at key on Close.
	// Callers must Close the writer; discarding it may leak a partial upload.
	NewWriter(ctx context.Context, key string, opts *WriterOpts) (io.WriteCloser, error)

	// NewReader opens the object at key for streaming reads.
	// Returns an error wrapping ErrNotExist when the key does not exist.
	NewReader(ctx context.Context, key string) (io.ReadCloser, error)

	// NewRangeReader reads length bytes starting at offset. A negative length
	// reads to end-of-object. Used to satisfy HTTP Range requests.
	NewRangeReader(ctx context.Context, key string, offset, length int64) (io.ReadCloser, error)

	// Delete removes the object at key. Succeeds (no error) when the key is
	// already absent.
	Delete(ctx context.Context, key string) error

	// Stat returns metadata for the object at key.
	// Returns an error wrapping ErrNotExist when the key does not exist.
	Stat(ctx context.Context, key string) (Attrs, error)

	// List returns an iterator over keys with the given prefix. Pass "" to
	// list all keys. Iteration order is backend-defined.
	List(ctx context.Context, prefix string) Iterator

	// Close releases any resources held by the backend. Safe to call
	// multiple times.
	Close() error
}

// ValidateKey rejects keys that are empty, absolute, or contain "..", ".",
// empty components, or NUL bytes. Backends call this at the entry point of
// every operation.
func ValidateKey(key string) error {
	if key == "" {
		return fmt.Errorf("%w: empty", ErrInvalidKey)
	}
	if strings.HasPrefix(key, "/") {
		return fmt.Errorf("%w: absolute path", ErrInvalidKey)
	}
	if strings.ContainsRune(key, '\x00') {
		return fmt.Errorf("%w: contains NUL", ErrInvalidKey)
	}
	for part := range strings.SplitSeq(key, "/") {
		if part == ".." || part == "." || part == "" {
			return fmt.Errorf("%w: invalid component %q", ErrInvalidKey, part)
		}
	}
	return nil
}
