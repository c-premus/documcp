package kiwix

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestClient creates a Client pointing at the given httptest.Server.
// It constructs the client directly to bypass SSRF validation (test servers
// bind to localhost).
func newTestClient(t *testing.T, serverURL string) *Client {
	t.Helper()
	return &Client{
		baseURL:            strings.TrimRight(serverURL, "/"),
		httpClient:         &http.Client{Timeout: 10 * time.Second},
		cache:              newCache(),
		logger:             slog.Default(),
		healthCheckTimeout: 5 * time.Second,
		cacheTTL:           1 * time.Hour,
	}
}

// catalogForArchive returns a minimal OPDS XML catalog with a single entry
// matching the given archive name. If ftindex is true, the tags include
// _ftindex:yes so fulltext search is allowed.
func catalogForArchive(name string, ftindex bool) string {
	tags := "test"
	if ftindex {
		tags = "test;_ftindex:yes"
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <id>urn:uuid:test-001</id>
    <title>Test Archive</title>
    <name>%s</name>
    <tags>%s</tags>
    <articleCount>100</articleCount>
    <mediaCount>0</mediaCount>
    <link rel="http://opds-spec.org/acquisition/open-access" type="application/x-zim" href="https://example.test/archive.zim" length="1024" />
  </entry>
</feed>`, name, tags)
}

// sampleOPDSCatalog returns a valid OPDS XML catalog with two entries for use
// in tests. The XML exercises Name, Tags, Creator, Links, and all metadata.
func sampleOPDSCatalog() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <id>urn:uuid:aaa-111</id>
    <title>DevDocs Go</title>
    <summary>Go programming language docs</summary>
    <language>eng</language>
    <name>devdocs-go</name>
    <tags>devdocs;go;programming</tags>
    <author><name>DevDocs</name></author>
    <articleCount>1500</articleCount>
    <mediaCount>200</mediaCount>
    <favicon>/meta?name=devdocs-go&amp;content=favicon</favicon>
    <link rel="http://opds-spec.org/image/thumbnail" href="/thumb/devdocs-go.png" type="image/png"/>
    <link rel="http://opds-spec.org/acquisition/open-access" type="application/x-zim" href="https://example.test/devdocs-go.zim" length="52428800" />
  </entry>
  <entry>
    <id>urn:uuid:bbb-222</id>
    <title>Wikipedia EN</title>
    <summary>English Wikipedia</summary>
    <language>eng</language>
    <tags>wikipedia;nopic</tags>
    <author><name>Wikipedia</name></author>
    <articleCount>6000000</articleCount>
    <mediaCount>0</mediaCount>
    <link rel="http://opds-spec.org/image" href="/thumb/wikipedia_en.png" type="image/png"/>
    <link rel="http://opds-spec.org/acquisition/open-access" type="application/x-zim" href="https://example.test/wikipedia_en.zim" length="104857600" />
  </entry>
</feed>`
}

// --- Health ---

func TestHealth(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
		errSubstr  string
	}{
		{
			name:       "returns nil on HTTP 200",
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "returns error on HTTP 500",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
			errSubstr:  "status 500",
		},
		{
			name:       "returns error on HTTP 404",
			statusCode: http.StatusNotFound,
			wantErr:    true,
			errSubstr:  "status 404",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/catalog/root.xml" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
			}))
			defer srv.Close()

			client := newTestClient(t, srv.URL)
			err := client.Health(context.Background())

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errSubstr)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestHealth_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block until the request context is done so the server handler
		// exits promptly after the client gives up, avoiding a slow Close.
		<-r.Context().Done()
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	// Use a very short context deadline to avoid waiting the full 5 seconds.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := client.Health(ctx)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestHealth_UnreachableServer(t *testing.T) {
	client := newTestClient(t, "http://127.0.0.1:1") // port 1 is not listening
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := client.Health(ctx)
	if err == nil {
		t.Fatal("expected connection error, got nil")
	}
	if !strings.Contains(err.Error(), "performing health check") {
		t.Errorf("error %q should mention performing health check", err.Error())
	}
}

// --- FetchCatalog ---

func TestFetchCatalog(t *testing.T) {
	t.Run("parses valid OPDS catalog", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/catalog/root.xml" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/xml")
			_, _ = fmt.Fprint(w, sampleOPDSCatalog())
		}))
		defer srv.Close()

		client := newTestClient(t, srv.URL)
		entries, err := client.FetchCatalog(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(entries))
		}

		// Verify first entry.
		e := entries[0]
		if e.ID != "urn:uuid:aaa-111" {
			t.Errorf("ID = %q, want %q", e.ID, "urn:uuid:aaa-111")
		}
		if e.Title != "DevDocs Go" {
			t.Errorf("Title = %q, want %q", e.Title, "DevDocs Go")
		}
		if e.Name != "devdocs-go" {
			t.Errorf("Name = %q, want %q", e.Name, "devdocs-go")
		}
		if e.Category != "devdocs" {
			t.Errorf("Category = %q, want %q", e.Category, "devdocs")
		}
		if e.Language != "eng" {
			t.Errorf("Language = %q, want %q", e.Language, "eng")
		}
		if e.Creator != "DevDocs" {
			t.Errorf("Creator = %q, want %q", e.Creator, "DevDocs")
		}
		if e.Publisher != "Kiwix" {
			t.Errorf("Publisher = %q, want %q", e.Publisher, "Kiwix")
		}
		if e.ArticleCount != 1500 {
			t.Errorf("ArticleCount = %d, want 1500", e.ArticleCount)
		}
		if e.MediaCount != 200 {
			t.Errorf("MediaCount = %d, want 200", e.MediaCount)
		}
		if e.FileSize != 52428800 {
			t.Errorf("FileSize = %d, want 52428800", e.FileSize)
		}
		// Favicon should come from the <favicon> element, not the link.
		if e.Favicon != "/meta?name=devdocs-go&content=favicon" {
			t.Errorf("Favicon = %q, want /meta?name=devdocs-go&content=favicon", e.Favicon)
		}
		if len(e.Tags) != 3 {
			t.Errorf("Tags length = %d, want 3", len(e.Tags))
		}

		// Verify second entry derives favicon from link (no <favicon> element).
		e2 := entries[1]
		if e2.Category != "wikipedia" {
			t.Errorf("Category = %q, want %q", e2.Category, "wikipedia")
		}
		if e2.Favicon != "/thumb/wikipedia_en.png" {
			t.Errorf("Favicon = %q, want /thumb/wikipedia_en.png", e2.Favicon)
		}
		// Name should be derived from ID since <name> is empty.
		if e2.Name != "urn:uuid:bbb-222" {
			// deriveNameFromID only strips path prefix if "/" present.
			t.Errorf("Name = %q, want %q", e2.Name, "urn:uuid:bbb-222")
		}
	})

	t.Run("returns cached result on second call", func(t *testing.T) {
		callCount := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			w.Header().Set("Content-Type", "application/xml")
			_, _ = fmt.Fprint(w, sampleOPDSCatalog())
		}))
		defer srv.Close()

		client := newTestClient(t, srv.URL)

		// First call populates cache.
		entries1, err := client.FetchCatalog(context.Background())
		if err != nil {
			t.Fatalf("first call: %v", err)
		}

		// Second call should hit cache, not the server.
		entries2, err := client.FetchCatalog(context.Background())
		if err != nil {
			t.Fatalf("second call: %v", err)
		}

		if callCount != 1 {
			t.Errorf("expected 1 HTTP call (cached), got %d", callCount)
		}
		if len(entries1) != len(entries2) {
			t.Errorf("cached entries length mismatch: %d vs %d", len(entries1), len(entries2))
		}
	})

	t.Run("returns error on server error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer srv.Close()

		client := newTestClient(t, srv.URL)
		_, err := client.FetchCatalog(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "status 503") {
			t.Errorf("error %q should contain 'status 503'", err.Error())
		}
	})

	t.Run("returns error on malformed XML", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/xml")
			_, _ = fmt.Fprint(w, "<broken xml><<>>>>>")
		}))
		defer srv.Close()

		client := newTestClient(t, srv.URL)
		_, err := client.FetchCatalog(context.Background())
		if err == nil {
			t.Fatal("expected error for malformed XML, got nil")
		}
		if !strings.Contains(err.Error(), "parsing catalog XML") {
			t.Errorf("error %q should mention parsing catalog XML", err.Error())
		}
	})

	t.Run("returns empty slice for empty feed", func(t *testing.T) {
		emptyFeed := `<?xml version="1.0" encoding="UTF-8"?><feed xmlns="http://www.w3.org/2005/Atom"></feed>`
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/xml")
			_, _ = fmt.Fprint(w, emptyFeed)
		}))
		defer srv.Close()

		client := newTestClient(t, srv.URL)
		entries, err := client.FetchCatalog(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 0 {
			t.Errorf("expected 0 entries, got %d", len(entries))
		}
	})
}

// --- Search ---

func TestSearch(t *testing.T) {
	t.Run("suggest search sends correct query params", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/catalog/root.xml" {
				w.Header().Set("Content-Type", "application/xml")
				_, _ = fmt.Fprint(w, catalogForArchive("devdocs-go", false))
				return
			}
			if r.URL.Path != "/suggest" {
				t.Errorf("path = %q, want /suggest", r.URL.Path)
			}
			if r.URL.Query().Get("term") != "golang" {
				t.Errorf("term = %q, want golang", r.URL.Query().Get("term"))
			}
			if r.URL.Query().Get("count") != "5" {
				t.Errorf("count = %q, want 5", r.URL.Query().Get("count"))
			}
			if r.URL.Query().Get("content") != "devdocs-go" {
				t.Errorf("content = %q, want devdocs-go", r.URL.Query().Get("content"))
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `[
				{"label":"<b>Go</b> Tutorial","value":"Go Tutorial","kind":"path","path":"A/Go_Tutorial"},
				{"label":"<b>Go</b> Modules","value":"Go Modules","kind":"path","path":"A/Go_Modules"}
			]`)
		}))
		defer srv.Close()

		client := newTestClient(t, srv.URL)
		results, err := client.Search(context.Background(), "devdocs-go", "golang", "suggest", 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		if results[0].Title != "Go Tutorial" {
			t.Errorf("Title = %q, want %q", results[0].Title, "Go Tutorial")
		}
		if results[0].Path != "A/Go_Tutorial" {
			t.Errorf("Path = %q, want %q", results[0].Path, "A/Go_Tutorial")
		}
		if results[1].Title != "Go Modules" {
			t.Errorf("Title = %q, want %q", results[1].Title, "Go Modules")
		}
	})

	t.Run("fulltext search sends correct query params and parses XML", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/catalog/root.xml" {
				w.Header().Set("Content-Type", "application/xml")
				_, _ = fmt.Fprint(w, catalogForArchive("devdocs-go", true))
				return
			}
			if r.URL.Path != "/search" {
				t.Errorf("path = %q, want /search", r.URL.Path)
			}
			if r.URL.Query().Get("pattern") != "concurrency" {
				t.Errorf("pattern = %q, want concurrency", r.URL.Query().Get("pattern"))
			}
			if r.URL.Query().Get("pageLength") != "3" {
				t.Errorf("pageLength = %q, want 3", r.URL.Query().Get("pageLength"))
			}
			if r.URL.Query().Get("content") != "devdocs-go" {
				t.Errorf("content = %q, want devdocs-go", r.URL.Query().Get("content"))
			}
			if r.URL.Query().Get("format") != "xml" {
				t.Errorf("format = %q, want xml", r.URL.Query().Get("format"))
			}

			w.Header().Set("Content-Type", "application/xml")
			_, _ = fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel>
  <item>
    <title>Goroutines</title>
    <link>/content/devdocs-go/A/Goroutines</link>
    <description>Concurrency primitives in Go</description>
  </item>
  <item>
    <title>Channels</title>
    <link>/content/devdocs-go/A/Channels</link>
    <description>Channel communication</description>
  </item>
</channel></rss>`)
		}))
		defer srv.Close()

		client := newTestClient(t, srv.URL)
		results, err := client.Search(context.Background(), "devdocs-go", "concurrency", "fulltext", 3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		if results[0].Title != "Goroutines" {
			t.Errorf("Title = %q, want %q", results[0].Title, "Goroutines")
		}
		if results[0].Path != "A/Goroutines" {
			t.Errorf("Path = %q, want %q", results[0].Path, "A/Goroutines")
		}
		if results[0].Snippet != "Concurrency primitives in Go" {
			t.Errorf("Snippet = %q, want %q", results[0].Snippet, "Concurrency primitives in Go")
		}
	})

	t.Run("default limit is 10 when zero or negative", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/catalog/root.xml" {
				w.Header().Set("Content-Type", "application/xml")
				_, _ = fmt.Fprint(w, catalogForArchive("archive", false))
				return
			}
			if r.URL.Query().Get("count") != "10" {
				t.Errorf("count = %q, want 10", r.URL.Query().Get("count"))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `[]`)
		}))
		defer srv.Close()

		client := newTestClient(t, srv.URL)

		_, err := client.Search(context.Background(), "archive", "query", "suggest", 0)
		if err != nil {
			t.Fatalf("limit=0: %v", err)
		}

		_, err = client.Search(context.Background(), "archive", "query", "suggest", -5)
		if err != nil {
			t.Fatalf("limit=-5: %v", err)
		}
	})

	t.Run("returns error for unsupported search type", func(t *testing.T) {
		client := newTestClient(t, "http://unused")
		_, err := client.Search(context.Background(), "archive", "query", "fuzzy", 10)
		if err == nil {
			t.Fatal("expected error for unsupported search type, got nil")
		}
		if !strings.Contains(err.Error(), "unsupported search type") {
			t.Errorf("error %q should contain 'unsupported search type'", err.Error())
		}
	})

	t.Run("returns error on server error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/catalog/root.xml" {
				w.Header().Set("Content-Type", "application/xml")
				_, _ = fmt.Fprint(w, catalogForArchive("archive", false))
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		client := newTestClient(t, srv.URL)
		_, err := client.Search(context.Background(), "archive", "query", "suggest", 10)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "status 500") {
			t.Errorf("error %q should contain 'status 500'", err.Error())
		}
	})

	t.Run("falls back to fulltext on malformed suggest JSON", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/catalog/root.xml" {
				w.Header().Set("Content-Type", "application/xml")
				_, _ = fmt.Fprint(w, catalogForArchive("archive", false))
				return
			}
			if r.URL.Path == "/suggest" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, `{not valid json]`)
				return
			}
			// Fulltext fallback returns valid XML with no results.
			w.Header().Set("Content-Type", "application/xml")
			_, _ = fmt.Fprint(w, `<?xml version="1.0"?><rss><channel></channel></rss>`)
		}))
		defer srv.Close()

		client := newTestClient(t, srv.URL)
		results, err := client.Search(context.Background(), "archive", "query", "suggest", 10)
		if err != nil {
			t.Fatalf("expected fallback to succeed, got error: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results from fallback, got %d", len(results))
		}
	})

	t.Run("fulltext returns empty slice for XML with no items", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/catalog/root.xml" {
				w.Header().Set("Content-Type", "application/xml")
				_, _ = fmt.Fprint(w, catalogForArchive("archive", true))
				return
			}
			w.Header().Set("Content-Type", "application/xml")
			_, _ = fmt.Fprint(w, `<?xml version="1.0"?><rss><channel></channel></rss>`)
		}))
		defer srv.Close()

		client := newTestClient(t, srv.URL)
		results, err := client.Search(context.Background(), "archive", "nonexistent", "fulltext", 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})

	t.Run("suggest returns empty slice for empty JSON array", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/catalog/root.xml" {
				w.Header().Set("Content-Type", "application/xml")
				_, _ = fmt.Fprint(w, catalogForArchive("archive", false))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `[]`)
		}))
		defer srv.Close()

		client := newTestClient(t, srv.URL)
		results, err := client.Search(context.Background(), "archive", "nothing", "suggest", 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})
}

// --- ReadArticle ---

func TestReadArticle(t *testing.T) {
	t.Run("returns article with title extracted from HTML", func(t *testing.T) {
		articleHTML := `<!DOCTYPE html>
<html>
<head><title>Goroutines - Go Documentation</title></head>
<body>
<h1>Goroutines</h1>
<p>A goroutine is a lightweight thread managed by the Go runtime.</p>
<p>You start one with the <code>go</code> keyword.</p>
</body>
</html>`

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/catalog/root.xml" {
				w.Header().Set("Content-Type", "application/xml")
				_, _ = fmt.Fprint(w, catalogForArchive("devdocs-go", false))
				return
			}
			if r.URL.Path != "/devdocs-go/A/Goroutines" {
				t.Errorf("path = %q, want /devdocs-go/A/Goroutines", r.URL.Path)
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprint(w, articleHTML)
		}))
		defer srv.Close()

		client := newTestClient(t, srv.URL)
		article, err := client.ReadArticle(context.Background(), "devdocs-go", "A/Goroutines")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if article.Title != "Goroutines - Go Documentation" {
			t.Errorf("Title = %q, want %q", article.Title, "Goroutines - Go Documentation")
		}
		if article.MIMEType != "text/html; charset=utf-8" {
			t.Errorf("MIMEType = %q, want %q", article.MIMEType, "text/html; charset=utf-8")
		}
		// Content should be plain text with HTML tags stripped.
		if strings.Contains(article.Content, "<p>") {
			t.Error("Content should not contain HTML tags")
		}
		if !strings.Contains(article.Content, "goroutine is a lightweight thread") {
			t.Error("Content should contain article text")
		}
	})

	t.Run("falls back to path segment when no title tag", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/catalog/root.xml" {
				w.Header().Set("Content-Type", "application/xml")
				_, _ = fmt.Fprint(w, catalogForArchive("archive", false))
				return
			}
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<html><body><p>No title here.</p></body></html>`)
		}))
		defer srv.Close()

		client := newTestClient(t, srv.URL)
		article, err := client.ReadArticle(context.Background(), "archive", "A/MyArticle")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if article.Title != "MyArticle" {
			t.Errorf("Title = %q, want %q (last path segment)", article.Title, "MyArticle")
		}
	})

	t.Run("strips scripts and styles from content", func(t *testing.T) {
		htmlWithScripts := `<html>
<head><title>Test</title>
<script>alert('xss')</script>
<style>body { color: red; }</style>
</head>
<body><p>Clean content here.</p></body>
</html>`

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/catalog/root.xml" {
				w.Header().Set("Content-Type", "application/xml")
				_, _ = fmt.Fprint(w, catalogForArchive("archive", false))
				return
			}
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, htmlWithScripts)
		}))
		defer srv.Close()

		client := newTestClient(t, srv.URL)
		article, err := client.ReadArticle(context.Background(), "archive", "A/Test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if strings.Contains(article.Content, "alert") {
			t.Error("Content should not contain script content")
		}
		if strings.Contains(article.Content, "color: red") {
			t.Error("Content should not contain style content")
		}
		if !strings.Contains(article.Content, "Clean content here.") {
			t.Error("Content should contain body text")
		}
	})

	t.Run("returns error on server error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/catalog/root.xml" {
				w.Header().Set("Content-Type", "application/xml")
				_, _ = fmt.Fprint(w, catalogForArchive("archive", false))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		client := newTestClient(t, srv.URL)
		_, err := client.ReadArticle(context.Background(), "archive", "A/Missing")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "status 404") {
			t.Errorf("error %q should contain 'status 404'", err.Error())
		}
	})
}

// --- validateArticlePath ---

func TestValidateArticlePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid simple path",
			path:    "A/My_Article",
			wantErr: false,
		},
		{
			name:    "valid nested path",
			path:    "A/Category/Subcategory/Article",
			wantErr: false,
		},
		{
			name:    "rejects leading slash",
			path:    "/A/Article",
			wantErr: true,
			errMsg:  "must not start with /",
		},
		{
			name:    "rejects dot-dot traversal",
			path:    "A/../../../etc/passwd",
			wantErr: true,
			errMsg:  "dot-dot",
		},
		{
			name:    "rejects dot-dot at start",
			path:    "../secrets",
			wantErr: true,
			errMsg:  "dot-dot",
		},
		{
			name:    "rejects null bytes",
			path:    "A/Article\x00.html",
			wantErr: true,
			errMsg:  "null bytes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateArticlePath(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestReadArticle_PathTraversal(t *testing.T) {
	// Server should never be called when path validation fails.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called for invalid paths")
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	paths := []string{
		"/etc/passwd",
		"../../../etc/shadow",
		"A/path\x00injection",
	}

	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			_, err := client.ReadArticle(context.Background(), "archive", p)
			if err == nil {
				t.Fatalf("expected error for path %q, got nil", p)
			}
			if !strings.Contains(err.Error(), "invalid article path") {
				t.Errorf("error %q should mention 'invalid article path'", err.Error())
			}
		})
	}
}

// --- Internal helpers ---

func TestDeriveNameFromID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want string
	}{
		{
			name: "UUID-style ID with no slash",
			id:   "urn:uuid:aaa-bbb-ccc",
			want: "urn:uuid:aaa-bbb-ccc",
		},
		{
			name: "path-style ID",
			id:   "/opds/devdocs-go.zim",
			want: "devdocs-go",
		},
		{
			name: "path-style ID without .zim",
			id:   "/opds/wikipedia_en",
			want: "wikipedia_en",
		},
		{
			name: "simple filename",
			id:   "catalog/myarchive.zim",
			want: "myarchive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveNameFromID(tt.id)
			if got != tt.want {
				t.Errorf("deriveNameFromID(%q) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestClassifyCategory(t *testing.T) {
	tests := []struct {
		name string
		tags string
		want string
	}{
		{name: "devdocs tag", tags: "devdocs;go;programming", want: "devdocs"},
		{name: "wikipedia tag", tags: "wikipedia;nopic", want: "wikipedia"},
		{name: "stack_exchange tag", tags: "stack_exchange;programming", want: "stack_exchange"},
		{name: "stackexchange variant", tags: "stackexchange;cooking", want: "stack_exchange"},
		{name: "mixed case DevDocs", tags: "DevDocs;Python", want: "devdocs"},
		{name: "unknown tags", tags: "gutenberg;books", want: "other"},
		{name: "empty tags", tags: "", want: "other"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyCategory(tt.tags)
			if got != tt.want {
				t.Errorf("classifyCategory(%q) = %q, want %q", tt.tags, got, tt.want)
			}
		})
	}
}

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string
	}{
		{
			name: "standard title tag",
			html: `<html><head><title>Hello World</title></head></html>`,
			want: "Hello World",
		},
		{
			name: "title with HTML entities",
			html: `<html><head><title>Go &amp; Concurrency</title></head></html>`,
			want: "Go & Concurrency",
		},
		{
			name: "no title tag",
			html: `<html><head></head><body>No title</body></html>`,
			want: "",
		},
		{
			name: "empty title tag",
			html: `<html><head><title>  </title></head></html>`,
			want: "",
		},
		{
			name: "title with surrounding whitespace",
			html: `<html><head><title>  Trimmed  </title></head></html>`,
			want: "Trimmed",
		},
		{
			name: "title inside comment ignored",
			html: `<html><head><!-- <title>Wrong</title> --><title>Right</title></head></html>`,
			want: "Right",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTitle(tt.html)
			if got != tt.want {
				t.Errorf("extractTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHTMLToPlainText(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string // substring that must appear
		deny string // substring that must not appear
	}{
		{
			name: "strips paragraph tags",
			html: `<p>Hello</p><p>World</p>`,
			want: "Hello",
			deny: "<p>",
		},
		{
			name: "replaces br with newline",
			html: `Line one<br>Line two<br/>Line three`,
			want: "Line one\nLine two\nLine three",
		},
		{
			name: "removes script blocks",
			html: `<p>Before</p><script>var x = 1;</script><p>After</p>`,
			want: "After",
			deny: "var x",
		},
		{
			name: "removes style blocks",
			html: `<p>Content</p><style>.cls { color: red; }</style>`,
			want: "Content",
			deny: "color",
		},
		{
			name: "decodes HTML entities",
			html: `<p>5 &gt; 3 &amp; 2 &lt; 4</p>`,
			want: "5 > 3 & 2 < 4",
		},
		{
			name: "empty string returns empty",
			html: "",
			want: "",
		},
		{
			name: "attribute containing greater-than sign",
			html: `<p><img alt="a > b">visible text</p>`,
			want: "visible text",
			deny: "alt",
		},
		{
			name: "HTML comment with tags inside",
			html: `<p>visible</p><!-- <p>hidden</p> --><p>also visible</p>`,
			want: "also visible",
			deny: "hidden",
		},
		{
			name: "nested list items",
			html: `<ul><li>first</li><li>second</li><li>third</li></ul>`,
			want: "first",
		},
		{
			name: "heading hierarchy",
			html: `<h1>Title</h1><h2>Subtitle</h2><p>Body text</p>`,
			want: "Body text",
		},
		{
			name: "script with angle brackets in code",
			html: `<p>before</p><script>if (a<b) { x(); }</script><p>after</p>`,
			want: "after",
			deny: "x()",
		},
		{
			name: "table content preserved",
			html: `<table><tr><td>cell one</td><td>cell two</td></tr></table>`,
			want: "cell one",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := htmlToPlainText(tt.html)
			if tt.want != "" && !strings.Contains(got, tt.want) {
				t.Errorf("htmlToPlainText() = %q, should contain %q", got, tt.want)
			}
			if tt.deny != "" && strings.Contains(got, tt.deny) {
				t.Errorf("htmlToPlainText() = %q, should NOT contain %q", got, tt.deny)
			}
		})
	}
}

func TestParseFulltextResponse(t *testing.T) {
	t.Run("parses XML RSS items", func(t *testing.T) {
		body := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel>
  <item>
    <title>First Article</title>
    <link>/content/archive_2025-01/A/First_Article</link>
    <description>This is the first snippet</description>
  </item>
  <item>
    <title>Second Article</title>
    <link>/content/archive_2025-01/A/Second_Article</link>
    <description>This is the second snippet</description>
  </item>
</channel></rss>`

		results, err := parseFulltextResponse([]byte(body))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		if results[0].Title != "First Article" {
			t.Errorf("Title[0] = %q, want %q", results[0].Title, "First Article")
		}
		if results[0].Path != "A/First_Article" {
			t.Errorf("Path[0] = %q, want %q", results[0].Path, "A/First_Article")
		}
		if results[0].Snippet != "This is the first snippet" {
			t.Errorf("Snippet[0] = %q, want %q", results[0].Snippet, "This is the first snippet")
		}
		if results[1].Title != "Second Article" {
			t.Errorf("Title[1] = %q, want %q", results[1].Title, "Second Article")
		}
	})

	t.Run("returns error for invalid XML", func(t *testing.T) {
		_, err := parseFulltextResponse([]byte("{not xml}"))
		if err == nil {
			t.Fatal("expected error for invalid XML, got nil")
		}
	})

	t.Run("returns empty for empty channel", func(t *testing.T) {
		body := `<?xml version="1.0"?><rss><channel></channel></rss>`
		results, err := parseFulltextResponse([]byte(body))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})

	t.Run("decodes HTML entities in title", func(t *testing.T) {
		body := `<?xml version="1.0"?><rss><channel>
  <item>
    <title>Tom &amp; Jerry</title>
    <link>/content/arc_2025-01/path/article</link>
    <description>A classic</description>
  </item>
</channel></rss>`
		results, err := parseFulltextResponse([]byte(body))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Title != "Tom & Jerry" {
			t.Errorf("Title = %q, want %q", results[0].Title, "Tom & Jerry")
		}
	})

	t.Run("falls back to path title when all titles are identical", func(t *testing.T) {
		body := `<?xml version="1.0"?><rss><channel>
  <item>
    <title>100R</title>
    <link>/content/arc_2025-01/site/satellite_phone.html</link>
    <description>desc1</description>
  </item>
  <item>
    <title>100R</title>
    <link>/content/arc_2025-01/site/orca.html</link>
    <description>desc2</description>
  </item>
</channel></rss>`
		results, err := parseFulltextResponse([]byte(body))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		if results[0].Title != "Satellite Phone" {
			t.Errorf("Title[0] = %q, want %q", results[0].Title, "Satellite Phone")
		}
		if results[1].Title != "Orca" {
			t.Errorf("Title[1] = %q, want %q", results[1].Title, "Orca")
		}
	})
}

func TestParseSuggestResponse(t *testing.T) {
	t.Run("parses valid JSON array", func(t *testing.T) {
		data := `[
			{"label":"<b>Go</b> Functions","value":"Go Functions","kind":"path","path":"A/Go_Functions"},
			{"label":"<b>Go</b> Interfaces","value":"Go Interfaces","kind":"path","path":"A/Go_Interfaces"}
		]`

		results, err := parseSuggestResponse([]byte(data))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		if results[0].Title != "Go Functions" {
			t.Errorf("Title = %q, want %q", results[0].Title, "Go Functions")
		}
		if results[0].Path != "A/Go_Functions" {
			t.Errorf("Path = %q, want %q", results[0].Path, "A/Go_Functions")
		}
	})

	t.Run("returns empty slice for empty array", func(t *testing.T) {
		results, err := parseSuggestResponse([]byte(`[]`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		_, err := parseSuggestResponse([]byte(`not json`))
		if err == nil {
			t.Fatal("expected error for invalid JSON, got nil")
		}
	})
}

// --- NewClient ---

// defaultTestConfig returns a ClientConfig with default values for testing.
func defaultTestConfig(baseURL string) ClientConfig {
	return ClientConfig{
		BaseURL:            baseURL,
		HTTPTimeout:        10 * time.Second,
		HealthCheckTimeout: 5 * time.Second,
		CacheTTL:           1 * time.Hour,
		SSRFDialerTimeout:  10 * time.Second,
	}
}

func TestNewClient(t *testing.T) {
	t.Run("trims trailing slash from base URL", func(t *testing.T) {
		client, err := NewClient(defaultTestConfig("http://example.com:8080/"), slog.Default())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if client.baseURL != "http://example.com:8080" {
			t.Errorf("baseURL = %q, want %q", client.baseURL, "http://example.com:8080")
		}
	})

	t.Run("handles URL without trailing slash", func(t *testing.T) {
		client, err := NewClient(defaultTestConfig("http://example.com:8080"), slog.Default())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if client.baseURL != "http://example.com:8080" {
			t.Errorf("baseURL = %q, want %q", client.baseURL, "http://example.com:8080")
		}
	})

	t.Run("sets 10 second HTTP client timeout", func(t *testing.T) {
		client, err := NewClient(defaultTestConfig("http://example.com:8080"), slog.Default())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if client.httpClient.Timeout != 10*time.Second {
			t.Errorf("timeout = %v, want 10s", client.httpClient.Timeout)
		}
	})

	t.Run("initializes cache", func(t *testing.T) {
		client, err := NewClient(defaultTestConfig("http://example.com:8080"), slog.Default())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if client.cache == nil {
			t.Fatal("cache should not be nil")
		}
	})

	t.Run("rejects localhost URL", func(t *testing.T) {
		_, err := NewClient(defaultTestConfig("http://localhost:8080"), slog.Default())
		if err == nil {
			t.Fatal("expected error for localhost URL")
		}
	})

	t.Run("allows private IP URL for internal services", func(t *testing.T) {
		_, err := NewClient(defaultTestConfig("http://192.168.1.1:8080"), slog.Default())
		if err != nil {
			t.Fatalf("expected no error for private IP URL, got: %v", err)
		}
	})
}

// --- Context cancellation ---

func TestSearch_ContextCanceled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	_, err := client.Search(ctx, "archive", "query", "suggest", 10)
	if err == nil {
		t.Fatal("expected error for canceled context, got nil")
	}
}

// --- HasFulltextIndex ---

func TestHasFulltextIndex(t *testing.T) {
	t.Run("returns true when archive has fulltext index", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/xml")
			_, _ = fmt.Fprint(w, catalogForArchive("devdocs-go", true))
		}))
		defer srv.Close()

		client := newTestClient(t, srv.URL)
		got := client.HasFulltextIndex(context.Background(), "devdocs-go")
		if !got {
			t.Error("expected true for archive with _ftindex:yes, got false")
		}
	})

	t.Run("returns false when archive lacks fulltext index", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/xml")
			_, _ = fmt.Fprint(w, catalogForArchive("devdocs-go", false))
		}))
		defer srv.Close()

		client := newTestClient(t, srv.URL)
		got := client.HasFulltextIndex(context.Background(), "devdocs-go")
		if got {
			t.Error("expected false for archive without _ftindex:yes, got true")
		}
	})

	t.Run("returns false when archive not found in catalog", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/xml")
			_, _ = fmt.Fprint(w, catalogForArchive("other-archive", true))
		}))
		defer srv.Close()

		client := newTestClient(t, srv.URL)
		got := client.HasFulltextIndex(context.Background(), "nonexistent")
		if got {
			t.Error("expected false for archive not in catalog, got true")
		}
	})

	t.Run("returns false when catalog fetch fails", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		client := newTestClient(t, srv.URL)
		got := client.HasFulltextIndex(context.Background(), "devdocs-go")
		if got {
			t.Error("expected false when catalog fetch fails, got true")
		}
	})

	t.Run("uses cached catalog on repeated calls", func(t *testing.T) {
		callCount := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			w.Header().Set("Content-Type", "application/xml")
			_, _ = fmt.Fprint(w, catalogForArchive("devdocs-go", true))
		}))
		defer srv.Close()

		client := newTestClient(t, srv.URL)

		_ = client.HasFulltextIndex(context.Background(), "devdocs-go")
		_ = client.HasFulltextIndex(context.Background(), "devdocs-go")

		if callCount != 1 {
			t.Errorf("expected 1 HTTP call (cached), got %d", callCount)
		}
	})
}

// --- titleFromPath ---

func TestTitleFromPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "strips extension and title-cases",
			path: "site/satellite_phone.html",
			want: "Satellite Phone",
		},
		{
			name: "handles hyphens",
			path: "docs/my-great-article.html",
			want: "My Great Article",
		},
		{
			name: "handles URL-encoded spaces",
			path: "A/Hello%20World.html",
			want: "Hello World",
		},
		{
			name: "no directory prefix",
			path: "goroutines.html",
			want: "Goroutines",
		},
		{
			name: "no extension",
			path: "A/Channels",
			want: "Channels",
		},
		{
			name: "empty path returns Untitled",
			path: "",
			want: "Untitled",
		},
		{
			name: "deeply nested path uses last segment",
			path: "a/b/c/d/my_article.htm",
			want: "My Article",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := titleFromPath(tt.path)
			if got != tt.want {
				t.Errorf("titleFromPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// --- ReadArticle archive name validation ---

func TestReadArticle_InvalidArchiveName(t *testing.T) {
	// Server should never be called when archive name validation fails.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called for invalid archive names")
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	names := []string{
		"",
		"archive/traversal",
		"archive\\traversal",
		"archive..secret",
		"archive\x00null",
	}

	for _, n := range names {
		t.Run(fmt.Sprintf("rejects %q", n), func(t *testing.T) {
			_, err := client.ReadArticle(context.Background(), n, "A/Article")
			if err == nil {
				t.Fatalf("expected error for archive name %q, got nil", n)
			}
			if !strings.Contains(err.Error(), "invalid archive name") {
				t.Errorf("error %q should mention 'invalid archive name'", err.Error())
			}
		})
	}
}

// --- Context cancellation ---

func TestReadArticle_ContextCanceled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	_, err := client.ReadArticle(ctx, "archive", "A/Article")
	if err == nil {
		t.Fatal("expected error for canceled context, got nil")
	}
}
