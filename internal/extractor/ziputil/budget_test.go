package ziputil_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/c-premus/documcp/internal/extractor/ziputil"
)

func TestBudgetReader_UnderBudget(t *testing.T) {
	var total int64
	src := strings.NewReader("hello world")
	br := ziputil.NewBudgetReader(src, &total, 100)

	got, err := io.ReadAll(br)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != "hello world" {
		t.Errorf("read = %q, want %q", got, "hello world")
	}
	if total != 11 {
		t.Errorf("total = %d, want 11", total)
	}
}

func TestBudgetReader_ExceedsBudget(t *testing.T) {
	var total int64
	src := strings.NewReader("0123456789")
	br := ziputil.NewBudgetReader(src, &total, 5)

	got, err := io.ReadAll(br)
	if !errors.Is(err, ziputil.ErrBudgetExceeded) {
		t.Fatalf("err = %v, want ErrBudgetExceeded", err)
	}
	// Callers may have received some bytes before the budget trips; exact count
	// depends on reader chunking. We only require that the total counter
	// crossed the threshold.
	if total <= 5 {
		t.Errorf("total = %d, expected > 5", total)
	}
	// And that we did not read more than the underlying source.
	if len(got) > 10 {
		t.Errorf("read %d bytes, source was only 10", len(got))
	}
}

func TestBudgetReader_SharedCounterAcrossReaders(t *testing.T) {
	// Simulate two ZIP entries sharing one budget: first fits, second trips it.
	var total int64
	const budget int64 = 15

	first := ziputil.NewBudgetReader(strings.NewReader("0123456789"), &total, budget)
	if _, err := io.ReadAll(first); err != nil {
		t.Fatalf("first reader unexpected error: %v", err)
	}
	if total != 10 {
		t.Fatalf("after first read, total = %d, want 10", total)
	}

	second := ziputil.NewBudgetReader(strings.NewReader("abcdefghij"), &total, budget)
	_, err := io.ReadAll(second)
	if !errors.Is(err, ziputil.ErrBudgetExceeded) {
		t.Fatalf("second reader err = %v, want ErrBudgetExceeded", err)
	}
	if total <= budget {
		t.Errorf("total = %d, want > %d", total, budget)
	}
}

func TestBudgetReader_EmptySource(t *testing.T) {
	var total int64
	br := ziputil.NewBudgetReader(bytes.NewReader(nil), &total, 100)

	n, err := br.Read(make([]byte, 8))
	if !errors.Is(err, io.EOF) {
		t.Errorf("err = %v, want io.EOF", err)
	}
	if n != 0 {
		t.Errorf("n = %d, want 0", n)
	}
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
}

func TestBudgetReader_BudgetAtZero(t *testing.T) {
	// A zero budget should trip on any non-empty read.
	var total int64
	br := ziputil.NewBudgetReader(strings.NewReader("x"), &total, 0)

	_, err := io.ReadAll(br)
	if !errors.Is(err, ziputil.ErrBudgetExceeded) {
		t.Errorf("err = %v, want ErrBudgetExceeded", err)
	}
}
