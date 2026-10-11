package xlsx

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/c-premus/documcp/internal/extractor"
)

func TestRunGuarded(t *testing.T) {
	t.Parallel()

	errBoom := errors.New("boom")
	want := &extractor.ExtractedContent{Content: "ok"}

	tests := []struct {
		name        string
		timeout     time.Duration
		fn          func(release <-chan struct{}) (*extractor.ExtractedContent, error)
		wantContent *extractor.ExtractedContent
		wantErr     string
		wantErrIs   error
	}{
		{
			name:    "returns result",
			timeout: time.Minute,
			fn: func(<-chan struct{}) (*extractor.ExtractedContent, error) {
				return want, nil
			},
			wantContent: want,
		},
		{
			name:    "passes error through",
			timeout: time.Minute,
			fn: func(<-chan struct{}) (*extractor.ExtractedContent, error) {
				return nil, errBoom
			},
			wantErrIs: errBoom,
		},
		{
			name:    "recovers panic",
			timeout: time.Minute,
			fn: func(<-chan struct{}) (*extractor.ExtractedContent, error) {
				panic("runtime error: index out of range [0] with length 0")
			},
			wantErr: "xlsx parser panic",
		},
		{
			name:    "times out on a stuck parser",
			timeout: 10 * time.Millisecond,
			fn: func(release <-chan struct{}) (*extractor.ExtractedContent, error) {
				<-release
				return want, nil
			},
			wantErr:   "timed out",
			wantErrIs: context.DeadlineExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			// Unblock the "stuck" goroutine once the test is done so it
			// does not outlive the test binary's leak checks.
			release := make(chan struct{})
			t.Cleanup(func() { close(release) })

			got, err := runGuarded(ctx, func() (*extractor.ExtractedContent, error) {
				return tt.fn(release)
			})

			if tt.wantErr == "" && tt.wantErrIs == nil {
				if err != nil {
					t.Fatalf("runGuarded() unexpected error: %v", err)
				}
				if got != tt.wantContent {
					t.Errorf("runGuarded() = %v, want %v", got, tt.wantContent)
				}
				return
			}
			if err == nil {
				t.Fatal("runGuarded() expected error, got nil")
			}
			if tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("runGuarded() error = %q, want it to contain %q", err, tt.wantErr)
			}
			if tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
				t.Errorf("runGuarded() error = %v, want errors.Is %v", err, tt.wantErrIs)
			}
			if got != nil {
				t.Errorf("runGuarded() content = %v, want nil on error", got)
			}
		})
	}
}

func TestXLSXExtractor_Extract_Timeout(t *testing.T) {
	t.Parallel()

	e := New()
	e.timeout = time.Nanosecond

	_, err := e.Extract(context.Background(), "../../testutil/testdata/sample.xlsx")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Extract() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestXLSXExtractor_Extract_CanceledBeforeStart(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := New().Extract(ctx, "../../testutil/testdata/sample.xlsx")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Extract() error = %v, want context.Canceled", err)
	}
}
