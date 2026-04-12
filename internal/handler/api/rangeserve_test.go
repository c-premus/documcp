package api

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/c-premus/documcp/internal/storage"
)

func TestParseSingleByteRange(t *testing.T) {
	t.Parallel()
	const size = int64(100)

	cases := []struct {
		name       string
		header     string
		size       int64
		wantOK     bool
		wantStart  int64
		wantLength int64
	}{
		{"empty header", "", size, false, 0, 0},
		{"zero size object", "bytes=0-9", 0, false, 0, 0},
		{"missing prefix", "0-9", size, false, 0, 0},
		{"wrong unit", "items=0-9", size, false, 0, 0},
		{"explicit range 0-9", "bytes=0-9", size, true, 0, 10},
		{"explicit range 50-59", "bytes=50-59", size, true, 50, 10},
		{"end-of-object range", "bytes=90-99", size, true, 90, 10},
		{"clamped end 90-200", "bytes=90-200", size, true, 90, 10},
		{"open-ended 50-", "bytes=50-", size, true, 50, 50},
		{"open-ended 0-", "bytes=0-", size, true, 0, 100},
		{"suffix -10 (last N bytes)", "bytes=-10", size, true, 90, 10},
		{"suffix -0 is invalid", "bytes=-0", size, false, 0, 0},
		{"suffix larger than object", "bytes=-200", size, true, 0, 100},
		{"multi-range rejected", "bytes=0-9,20-29", size, false, 0, 0},
		{"start beyond object", "bytes=200-", size, false, 0, 0},
		{"start at EOF", "bytes=100-", size, false, 0, 0},
		{"reversed range", "bytes=50-40", size, false, 0, 0},
		{"no dash", "bytes=50", size, false, 0, 0},
		{"non-numeric", "bytes=a-b", size, false, 0, 0},
		{"mixed case prefix", "BYTES=0-9", size, true, 0, 10},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			start, length, ok := parseSingleByteRange(tc.header, tc.size)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !tc.wantOK {
				return
			}
			if start != tc.wantStart {
				t.Errorf("start = %d, want %d", start, tc.wantStart)
			}
			if length != tc.wantLength {
				t.Errorf("length = %d, want %d", length, tc.wantLength)
			}
		})
	}
}

func TestEtagMatches(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		header string
		etag   string
		want   bool
	}{
		{"exact match", `"abc"`, `"abc"`, true},
		{"wildcard always matches", `*`, `"anything"`, true},
		{"weak client matches strong server", `W/"abc"`, `"abc"`, true},
		{"strong client matches weak server", `"abc"`, `W/"abc"`, true},
		{"weak matches weak", `W/"abc"`, `W/"abc"`, true},
		{"no match", `"abc"`, `"def"`, false},
		{"list with match", `"x", "abc", "y"`, `"abc"`, true},
		{"list without match", `"x", "y"`, `"abc"`, false},
		{"empty header", ``, `"abc"`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := etagMatches(tc.header, tc.etag); got != tc.want {
				t.Errorf("etagMatches(%q, %q) = %v, want %v", tc.header, tc.etag, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// serveBlob integration tests use FSBlob against a t.TempDir(), exercising
// the full 200 / 206 / 304 / 404 code paths without needing gofakes3.
// ---------------------------------------------------------------------------

func newServeBlobTestBlob(t *testing.T, key string, content []byte) storage.Blob {
	t.Helper()
	root := t.TempDir()
	b, err := storage.NewFSBlob(root)
	if err != nil {
		t.Fatalf("NewFSBlob: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })

	w, err := b.NewWriter(context.Background(), key, nil)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	if _, err := w.Write(content); err != nil {
		_ = w.Close()
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return b
}

func TestServeBlob_FullResponse(t *testing.T) {
	t.Parallel()
	blob := newServeBlobTestBlob(t, "pdf/full.bin", []byte("hello world"))

	req := httptest.NewRequest(http.MethodGet, "/download", http.NoBody)
	rr := httptest.NewRecorder()

	if err := serveBlob(rr, req, blob, "pdf/full.bin", "full.bin", "application/octet-stream"); err != nil {
		t.Fatalf("serveBlob: %v", err)
	}

	res := rr.Result()
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "application/octet-stream" {
		t.Errorf("Content-Type = %q, want application/octet-stream", ct)
	}
	if cl := res.Header.Get("Content-Length"); cl != "11" {
		t.Errorf("Content-Length = %q, want 11", cl)
	}
	if res.Header.Get("ETag") == "" {
		t.Error("ETag should be set from FSBlob weak validator")
	}
	body, _ := io.ReadAll(res.Body)
	if !bytes.Equal(body, []byte("hello world")) {
		t.Errorf("body = %q, want %q", body, "hello world")
	}
}

func TestServeBlob_RangeResponse(t *testing.T) {
	t.Parallel()
	blob := newServeBlobTestBlob(t, "pdf/range.bin", []byte("0123456789"))

	req := httptest.NewRequest(http.MethodGet, "/download", http.NoBody)
	req.Header.Set("Range", "bytes=3-6")
	rr := httptest.NewRecorder()

	if err := serveBlob(rr, req, blob, "pdf/range.bin", "range.bin", ""); err != nil {
		t.Fatalf("serveBlob: %v", err)
	}

	res := rr.Result()
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusPartialContent {
		t.Errorf("status = %d, want 206", res.StatusCode)
	}
	if cr := res.Header.Get("Content-Range"); cr != "bytes 3-6/10" {
		t.Errorf("Content-Range = %q, want bytes 3-6/10", cr)
	}
	if cl := res.Header.Get("Content-Length"); cl != "4" {
		t.Errorf("Content-Length = %q, want 4", cl)
	}
	body, _ := io.ReadAll(res.Body)
	if string(body) != "3456" {
		t.Errorf("body = %q, want %q", body, "3456")
	}
}

func TestServeBlob_IfNoneMatchReturns304(t *testing.T) {
	t.Parallel()
	blob := newServeBlobTestBlob(t, "pdf/304.bin", []byte("cached"))

	// First request establishes the ETag.
	initReq := httptest.NewRequest(http.MethodGet, "/download", http.NoBody)
	initRR := httptest.NewRecorder()
	if err := serveBlob(initRR, initReq, blob, "pdf/304.bin", "304.bin", ""); err != nil {
		t.Fatalf("initial serveBlob: %v", err)
	}
	etag := initRR.Result().Header.Get("ETag")
	if etag == "" {
		t.Fatal("initial response missing ETag")
	}

	// Conditional request: should 304.
	req := httptest.NewRequest(http.MethodGet, "/download", http.NoBody)
	req.Header.Set("If-None-Match", etag)
	rr := httptest.NewRecorder()
	if err := serveBlob(rr, req, blob, "pdf/304.bin", "304.bin", ""); err != nil {
		t.Fatalf("conditional serveBlob: %v", err)
	}
	if rr.Result().StatusCode != http.StatusNotModified {
		t.Errorf("status = %d, want 304", rr.Result().StatusCode)
	}
}

func TestServeBlob_IfModifiedSinceReturns304(t *testing.T) {
	t.Parallel()
	blob := newServeBlobTestBlob(t, "pdf/ims.bin", []byte("cached"))

	req := httptest.NewRequest(http.MethodGet, "/download", http.NoBody)
	// A future date guarantees the object is not newer than the client copy.
	req.Header.Set("If-Modified-Since", time.Now().Add(time.Hour).Format(http.TimeFormat))
	rr := httptest.NewRecorder()
	if err := serveBlob(rr, req, blob, "pdf/ims.bin", "ims.bin", ""); err != nil {
		t.Fatalf("serveBlob: %v", err)
	}
	if rr.Result().StatusCode != http.StatusNotModified {
		t.Errorf("status = %d, want 304", rr.Result().StatusCode)
	}
}

func TestServeBlob_HEADReturnsHeadersOnly(t *testing.T) {
	t.Parallel()
	blob := newServeBlobTestBlob(t, "pdf/head.bin", []byte("not returned in body"))

	req := httptest.NewRequest(http.MethodHead, "/download", http.NoBody)
	rr := httptest.NewRecorder()
	if err := serveBlob(rr, req, blob, "pdf/head.bin", "head.bin", ""); err != nil {
		t.Fatalf("serveBlob HEAD: %v", err)
	}
	res := rr.Result()
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", res.StatusCode)
	}
	if res.Header.Get("Content-Length") != "20" {
		t.Errorf("Content-Length = %q, want 20", res.Header.Get("Content-Length"))
	}
	body, _ := io.ReadAll(res.Body)
	if len(body) != 0 {
		t.Errorf("HEAD body = %q, want empty", body)
	}
}

func TestServeBlob_MissingKeyReturns404(t *testing.T) {
	t.Parallel()
	blob := newServeBlobTestBlob(t, "pdf/present.bin", []byte("x"))

	req := httptest.NewRequest(http.MethodGet, "/download", http.NoBody)
	rr := httptest.NewRecorder()
	// serveBlob wraps ErrNotExist into a 404 via errorResponse and returns nil.
	if err := serveBlob(rr, req, blob, "pdf/missing.bin", "missing.bin", ""); err != nil {
		t.Fatalf("serveBlob: %v", err)
	}
	if rr.Result().StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Result().StatusCode)
	}
	if rr.Body.String() == "" {
		t.Error("404 body should contain an error message")
	}
}
