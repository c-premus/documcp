package git

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/c-premus/documcp/internal/testutil"
)

// TestSafeHTTPTransport_BlocksLoopbackClone is the regression guard for
// docs/audit/security.md H1 (carryover from 2026-04-18 audit). Before the
// installSafeHTTPTransport fix, gogit.PlainCloneContext used
// http.DefaultTransport and would happily dial 127.0.0.1. After the fix,
// the dial is rejected by SafeTransportAllowPrivate's DialContext check
// because loopback is always blocked even when private RFC 1918 is
// permitted.
//
// This test does not need a real git server — the rejection happens at
// TCP dial, well before go-git can begin the /info/refs handshake. The
// error is wrapped through sanitizeErr so we match on a stable substring.
func TestSafeHTTPTransport_BlocksLoopbackClone(t *testing.T) {
	t.Parallel()

	c := NewClient(t.TempDir(), 0, 0, testutil.DiscardLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.Clone(ctx, CloneParams{
		URL:    "http://127.0.0.1:65535/repo.git",
		Branch: "main",
		Dest:   t.TempDir() + "/clone",
	})
	if err == nil {
		t.Fatal("expected loopback clone to fail, got nil error")
	}
	// sanitizeErr redacts URL/token details but keeps the SSRF rejection
	// signal via "blocked" from security.SafeTransportAllowPrivate's
	// DialContext check.
	if !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("expected SSRF-blocked error, got: %v", err)
	}
}
