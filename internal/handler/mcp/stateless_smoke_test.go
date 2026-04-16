package mcphandler

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestStatelessHTTPBehavior verifies the HTTP-level contract of Stateless: true:
//   - POST initialize succeeds with no prior session
//   - POST tools/list succeeds without an Mcp-Session-Id header
//   - GET /documcp returns 405 with Allow: POST (no standalone SSE stream)
//
// Regression guard: if Stateless is flipped back to false, (2) and (3) break.
func TestStatelessHTTPBehavior(t *testing.T) {
	h := New(Config{
		ServerName:    "documcp-smoke",
		ServerVersion: "0.0.0",
		Logger:        slog.New(slog.DiscardHandler),
	})
	srv := httptest.NewServer(h)
	defer srv.Close()

	t.Run("POST initialize", func(t *testing.T) {
		body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"smoke","version":"0"}}}`
		status, responseBody := doPost(t, srv.URL, body)
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200. body=%q", status, responseBody)
		}
		if !strings.Contains(responseBody, "documcp-smoke") {
			t.Fatalf("body missing server name: %q", truncForLog(responseBody, 300))
		}
	})

	t.Run("POST tools/list without session id", func(t *testing.T) {
		body := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`
		status, responseBody := doPost(t, srv.URL, body)
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200. body=%q", status, responseBody)
		}
		if !strings.Contains(responseBody, "unified_search") {
			t.Fatalf("body missing unified_search tool: %q", truncForLog(responseBody, 400))
		}
	})

	t.Run("GET / returns 405 with Allow: POST", func(t *testing.T) {
		req, err := http.NewRequest("GET", srv.URL, http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Accept", "text/event-stream")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want 405", resp.StatusCode)
		}
		if got := resp.Header.Get("Allow"); got != "POST" {
			t.Fatalf("Allow = %q, want POST", got)
		}
	})
}

func doPost(t *testing.T, url, body string) (status int, respBody string) {
	t.Helper()
	req, err := http.NewRequest("POST", url, bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-03-26")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func truncForLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
