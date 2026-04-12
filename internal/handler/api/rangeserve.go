package api

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/c-premus/documcp/internal/storage"
)

// serveBlob streams a blob to the HTTP response with Range, ETag, and
// If-Modified-Since support. It replaces http.ServeContent for the
// object-storage path, where the source is a streaming *Reader rather than
// an io.ReadSeeker.
//
// Range handling covers the single-range case (bytes=a-b). Multi-range
// requests fall back to a full 200 response, matching http.ServeContent's
// behavior for the rare case and avoiding multipart byteranges.
func serveBlob(
	w http.ResponseWriter,
	r *http.Request,
	blob storage.Blob,
	key string,
	filename string,
	contentType string,
) error {
	ctx := r.Context()
	attrs, err := blob.Stat(ctx, key)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			errorResponse(w, http.StatusNotFound, "file not found")
			return nil
		}
		return fmt.Errorf("stat blob %s: %w", key, err)
	}

	if contentType == "" {
		contentType = attrs.ContentType
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Conditional requests: If-None-Match takes priority over
	// If-Modified-Since per RFC 7232 §6.
	if match := r.Header.Get("If-None-Match"); match != "" && attrs.ETag != "" {
		if etagMatches(match, attrs.ETag) {
			writeNotModified(w, attrs)
			return nil
		}
	} else if ims := r.Header.Get("If-Modified-Since"); ims != "" && !attrs.ModTime.IsZero() {
		if t, parseErr := http.ParseTime(ims); parseErr == nil && !attrs.ModTime.Truncate(time.Second).After(t) {
			writeNotModified(w, attrs)
			return nil
		}
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Accept-Ranges", "bytes")
	if attrs.ETag != "" {
		w.Header().Set("ETag", attrs.ETag)
	}
	if !attrs.ModTime.IsZero() {
		w.Header().Set("Last-Modified", attrs.ModTime.UTC().Format(http.TimeFormat))
	}
	if filename != "" {
		safe := strings.Map(func(r rune) rune {
			if r == '"' || r == '\\' || r < 32 || r == 127 {
				return '_'
			}
			return r
		}, filename)
		w.Header().Set("Content-Disposition", `attachment; filename="`+safe+`"`)
	}

	// HEAD — headers only. Mirrors http.ServeContent semantics.
	if r.Method == http.MethodHead {
		w.Header().Set("Content-Length", strconv.FormatInt(attrs.Size, 10))
		w.WriteHeader(http.StatusOK)
		return nil
	}

	rng := r.Header.Get("Range")
	start, length, ok := parseSingleByteRange(rng, attrs.Size)
	if !ok {
		// No Range header, unparseable Range, or multi-range — serve full body.
		// RFC 7233 says unparseable Range headers are ignored.
		w.Header().Set("Content-Length", strconv.FormatInt(attrs.Size, 10))
		rc, rcErr := blob.NewReader(ctx, key)
		if rcErr != nil {
			return fmt.Errorf("open blob reader %s: %w", key, rcErr)
		}
		defer func() { _ = rc.Close() }()
		if _, copyErr := io.Copy(w, rc); copyErr != nil {
			// Body already partially written — can't switch status.
			return fmt.Errorf("stream blob %s: %w", key, copyErr)
		}
		return nil
	}

	// start is inside [0, size), already validated by parseSingleByteRange,
	// so proceed with 206.
	rc, rcErr := blob.NewRangeReader(ctx, key, start, length)
	if rcErr != nil {
		return fmt.Errorf("open blob range reader %s: %w", key, rcErr)
	}
	defer func() { _ = rc.Close() }()

	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, start+length-1, attrs.Size))
	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	w.WriteHeader(http.StatusPartialContent)
	if _, copyErr := io.Copy(w, rc); copyErr != nil {
		return fmt.Errorf("stream blob range %s: %w", key, copyErr)
	}
	return nil
}

// writeNotModified sends a 304 with the validator headers that clients need
// to keep their cached copy fresh.
func writeNotModified(w http.ResponseWriter, attrs storage.Attrs) {
	if attrs.ETag != "" {
		w.Header().Set("ETag", attrs.ETag)
	}
	if !attrs.ModTime.IsZero() {
		w.Header().Set("Last-Modified", attrs.ModTime.UTC().Format(http.TimeFormat))
	}
	w.WriteHeader(http.StatusNotModified)
}

// etagMatches tests a raw If-None-Match header value against an ETag.
// Supports comma-separated lists and the "*" wildcard. Weak tags (W/"...")
// are compared by their quoted value, matching RFC 7232 §2.3.2 weak
// validation semantics.
func etagMatches(header, etag string) bool {
	header = strings.TrimSpace(header)
	if header == "*" {
		return true
	}
	etagNorm := strings.TrimPrefix(etag, "W/")
	for entry := range strings.SplitSeq(header, ",") {
		candidate := strings.TrimSpace(entry)
		candidateNorm := strings.TrimPrefix(candidate, "W/")
		if candidateNorm == etagNorm {
			return true
		}
	}
	return false
}

// parseSingleByteRange parses an HTTP Range header against an object of the
// given size. It returns (start, length, true) for a valid single-range
// request; (0, 0, false) for missing, malformed, or multi-range headers.
//
// Suffix ranges ("bytes=-N") are accepted and converted to the equivalent
// absolute range. Open-ended ranges ("bytes=N-") are interpreted as
// [N, size-1]. Overflows clamp to size-1.
func parseSingleByteRange(header string, size int64) (start, length int64, ok bool) {
	if header == "" || size <= 0 {
		return 0, 0, false
	}
	header = strings.TrimSpace(header)
	if !strings.HasPrefix(strings.ToLower(header), "bytes=") {
		return 0, 0, false
	}
	spec := header[len("bytes="):]
	// Multi-range: reject and let the caller fall back to full body.
	if strings.Contains(spec, ",") {
		return 0, 0, false
	}
	firstRaw, lastRaw, ok := strings.Cut(spec, "-")
	if !ok {
		return 0, 0, false
	}
	first := strings.TrimSpace(firstRaw)
	last := strings.TrimSpace(lastRaw)

	switch {
	case first == "":
		// Suffix range: bytes=-N → last N bytes.
		n, err := strconv.ParseInt(last, 10, 64)
		if err != nil || n <= 0 {
			return 0, 0, false
		}
		if n > size {
			n = size
		}
		return size - n, n, true
	case last == "":
		// Open-ended: bytes=N-
		s, err := strconv.ParseInt(first, 10, 64)
		if err != nil || s < 0 || s >= size {
			return 0, 0, false
		}
		return s, size - s, true
	default:
		s, err1 := strconv.ParseInt(first, 10, 64)
		e, err2 := strconv.ParseInt(last, 10, 64)
		if err1 != nil || err2 != nil || s < 0 || e < s || s >= size {
			return 0, 0, false
		}
		if e >= size {
			e = size - 1
		}
		return s, e - s + 1, true
	}
}

