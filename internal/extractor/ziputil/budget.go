// Package ziputil provides shared helpers for decompressing ZIP-based document
// formats (DOCX, EPUB) safely.
//
// BudgetReader enforces a cumulative decompressed-byte budget across multiple
// readers that share one counter. This defends against zip bombs whose central
// directory understates real decompression size: preflight checks that sum
// f.UncompressedSize64 trust attacker-supplied metadata, whereas BudgetReader
// counts bytes actually handed to the caller.
//
// Usage (extractor side):
//
//	var total int64
//	rc, _ := f.Open()
//	defer rc.Close()
//	br := ziputil.NewBudgetReader(io.LimitReader(rc, maxPerFile), &total, maxTotal)
//	// pass br to decoder; reuse &total for every entry in the archive
package ziputil

import (
	"errors"
	"io"
)

// ErrBudgetExceeded is returned by BudgetReader.Read when cumulative reads
// across all readers sharing the counter exceed the configured budget.
var ErrBudgetExceeded = errors.New("decompression budget exceeded")

// BudgetReader wraps an io.Reader and tracks cumulative bytes against a shared
// counter. When *total exceeds budget, Read returns ErrBudgetExceeded alongside
// any bytes it managed to read on that call, so callers consume what arrived
// before halting.
//
// The counter is a shared pointer by design: every ZIP entry in an archive
// points its own BudgetReader at the same *int64 so the budget applies across
// entries, not just within one file.
//
// BudgetReader is not safe for concurrent use. Each extractor processes entries
// sequentially within a single Extract call; that invariant holds today.
type BudgetReader struct {
	r      io.Reader
	total  *int64
	budget int64
}

// NewBudgetReader wraps r so every Read adds n to *total and halts with
// ErrBudgetExceeded once *total > budget. total must not be nil; budget should
// be positive.
func NewBudgetReader(r io.Reader, total *int64, budget int64) *BudgetReader {
	return &BudgetReader{r: r, total: total, budget: budget}
}

// Read implements io.Reader. Bytes read are added to *b.total; once the
// cumulative total exceeds b.budget, Read returns (n, ErrBudgetExceeded) where
// n reflects the bytes actually placed in p on this call.
func (b *BudgetReader) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)
	*b.total += int64(n)
	if *b.total > b.budget {
		return n, ErrBudgetExceeded
	}
	return n, err
}
