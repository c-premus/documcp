// Package kiwix provides an HTTP client for communicating with a Kiwix Serve
// instance. Kiwix Serve exposes ZIM archives via HTTP, providing an OPDS
// catalog, search, and article retrieval.
package kiwix

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	nethtml "golang.org/x/net/html"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/c-premus/documcp/internal/security"
)

const (
	// catalogCacheKey is the cache key for the OPDS catalog.
	catalogCacheKey = "catalog"
	// maxResponseBytes limits the size of HTTP response bodies read from Kiwix Serve.
	maxResponseBytes = 10 * 1024 * 1024 // 10 MB
)

// ClientConfig holds configuration for a Kiwix HTTP client.
type ClientConfig struct {
	BaseURL            string
	HTTPTimeout        time.Duration
	HealthCheckTimeout time.Duration
	CacheTTL           time.Duration
	SSRFDialerTimeout  time.Duration
}

// Client communicates with a Kiwix Serve instance over HTTP.
type Client struct {
	baseURL            string
	httpClient         *http.Client
	cache              *cache
	logger             *slog.Logger
	healthCheckTimeout time.Duration
	cacheTTL           time.Duration
}

// NewClient creates a new Kiwix HTTP client targeting the given base URL.
// It validates the base URL against SSRF attacks.
func NewClient(cfg ClientConfig, logger *slog.Logger) (*Client, error) {
	if err := security.ValidateExternalURL(cfg.BaseURL, true); err != nil {
		return nil, fmt.Errorf("invalid kiwix base URL: %w", err)
	}

	return &Client{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		httpClient: &http.Client{
			Timeout:   cfg.HTTPTimeout,
			Transport: otelhttp.NewTransport(security.SafeTransportAllowPrivate(cfg.SSRFDialerTimeout)),
		},
		cache:              newCache(),
		logger:             logger,
		healthCheckTimeout: cfg.HealthCheckTimeout,
		cacheTTL:           cfg.CacheTTL,
	}, nil
}

// Health checks whether the Kiwix Serve instance is reachable by fetching
// the OPDS catalog root. Returns nil on HTTP 200, an error otherwise.
func (c *Client) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, c.healthCheckTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/catalog/root.xml", http.NoBody)
	if err != nil {
		return fmt.Errorf("creating health request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("performing health check: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}

// FetchCatalog retrieves and parses the OPDS catalog from the Kiwix Serve
// instance. Results are cached for one hour.
func (c *Client) FetchCatalog(ctx context.Context) ([]CatalogEntry, error) {
	if cached, ok := c.cache.get(catalogCacheKey); ok {
		entries, _ := cached.([]CatalogEntry)
		c.logger.Debug("returning cached catalog", "count", len(entries))
		return entries, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/catalog/root.xml", http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating catalog request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching catalog: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("catalog request returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("reading catalog response: %w", err)
	}

	entries, err := parseCatalog(body)
	if err != nil {
		return nil, fmt.Errorf("parsing catalog XML: %w", err)
	}

	c.cache.set(catalogCacheKey, entries, c.cacheTTL)
	c.logger.Info("catalog fetched and cached", "count", len(entries))

	return entries, nil
}

// HasFulltextIndex checks whether an archive has a fulltext search index
// by looking it up in the (cached) catalog.
func (c *Client) HasFulltextIndex(ctx context.Context, archiveName string) bool {
	_, hasFT := c.resolveContentID(ctx, archiveName)
	return hasFT
}

// resolveContentID looks up the versioned content ID for an archive name.
// It fetches the catalog (from cache) and returns the ContentID for the
// matching entry. Falls back to archiveName if not found.
func (c *Client) resolveContentID(ctx context.Context, archiveName string) (string, bool) {
	entries, err := c.FetchCatalog(ctx)
	if err != nil {
		return archiveName, false
	}
	for i := range entries {
		if entries[i].Name == archiveName {
			return entries[i].ContentID, entries[i].HasFulltextIndex
		}
	}
	return archiveName, false
}

// Search queries the Kiwix Serve instance for articles matching the given
// query. The searchType must be "suggest" or "fulltext".
//
// When fulltext is requested but the archive lacks a fulltext index, Search
// automatically falls back to suggest (title matching) so callers always get
// the best available results. If the chosen search type fails at runtime,
// the other type is attempted as a last resort.
func (c *Client) Search(ctx context.Context, archiveName, query, searchType string, limit int) ([]SearchResult, error) {
	if searchType != "suggest" && searchType != "fulltext" {
		return nil, fmt.Errorf("unsupported search type %q (must be suggest or fulltext)", searchType)
	}

	if limit <= 0 {
		limit = 10
	}

	contentID, hasFTIndex := c.resolveContentID(ctx, archiveName)

	// Auto-fallback: if fulltext was requested but the archive lacks the
	// index, transparently switch to suggest so the caller still gets results.
	primary := searchType
	if primary == "fulltext" && !hasFTIndex {
		c.logger.Debug("fulltext index not available, falling back to suggest",
			"archive", archiveName,
		)
		primary = "suggest"
	}

	results, err := c.doSearch(ctx, contentID, query, primary, limit)
	if err == nil {
		return results, nil
	}

	// Try the other search type as a last resort.
	fallback := "fulltext"
	if primary == "fulltext" {
		fallback = "suggest"
	}
	c.logger.Debug("search failed, trying fallback",
		"archive", archiveName, "primary", primary, "fallback", fallback, "error", err,
	)

	results, fallbackErr := c.doSearch(ctx, contentID, query, fallback, limit)
	if fallbackErr != nil {
		// Return the original error — the fallback was best-effort.
		return nil, err
	}

	return results, nil
}

// doSearch performs the actual HTTP request for a search query.
// contentID is the versioned content identifier (e.g., "100r.co_en_all_2025-09").
func (c *Client) doSearch(ctx context.Context, contentID, query, searchType string, limit int) ([]SearchResult, error) {
	var reqURL string
	switch searchType {
	case "suggest":
		params := url.Values{
			"term":    {query},
			"count":   {strconv.Itoa(limit)},
			"content": {contentID},
		}
		reqURL = c.baseURL + "/suggest?" + params.Encode()
	case "fulltext":
		params := url.Values{
			"pattern":    {query},
			"pageLength": {strconv.Itoa(limit)},
			"content":    {contentID},
			"format":     {"xml"},
		}
		reqURL = c.baseURL + "/search?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating search request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("performing search: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("reading search response: %w", err)
	}

	switch searchType {
	case "suggest":
		return parseSuggestResponse(body)
	default: // fulltext (validated above)
		return parseFulltextResponse(body)
	}
}

// ReadArticle fetches an article from a ZIM archive and returns it as plain text.
// The articlePath is validated to prevent path traversal attacks.
func (c *Client) ReadArticle(ctx context.Context, archiveName, articlePath string) (*Article, error) {
	if err := validateArchiveName(archiveName); err != nil {
		return nil, fmt.Errorf("invalid archive name: %w", err)
	}
	if err := validateArticlePath(articlePath); err != nil {
		return nil, fmt.Errorf("invalid article path: %w", err)
	}

	contentID, _ := c.resolveContentID(ctx, archiveName)
	reqURL := c.baseURL + "/" + contentID + "/" + articlePath

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating article request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching article: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("article request returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("reading article response: %w", err)
	}

	mimeType := resp.Header.Get("Content-Type")
	content := string(body)

	// Extract title from HTML <title> tag if present.
	title := extractTitle(content)
	if title == "" {
		// Fall back to the last segment of the article path.
		parts := strings.Split(articlePath, "/")
		title = parts[len(parts)-1]
	}

	// Convert HTML to plain text.
	plainText := htmlToPlainText(content)

	return &Article{
		Title:    title,
		Content:  plainText,
		MIMEType: mimeType,
	}, nil
}

// validateArticlePath rejects paths that could be used for path traversal or
// contain unsafe characters.
func validateArticlePath(path string) error {
	if strings.HasPrefix(path, "/") {
		return errors.New("path must not start with /")
	}
	if strings.Contains(path, "..") {
		return errors.New("path must not contain dot-dot segments")
	}
	if strings.ContainsRune(path, 0) {
		return errors.New("path must not contain null bytes")
	}
	return nil
}

// validateArchiveName rejects archive names that could be used for path traversal.
func validateArchiveName(name string) error {
	if name == "" {
		return errors.New("archive name must not be empty")
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return errors.New("archive name must not contain path separators")
	}
	if strings.Contains(name, "..") {
		return errors.New("archive name must not contain dot-dot segments")
	}
	if strings.ContainsRune(name, 0) {
		return errors.New("archive name must not contain null bytes")
	}
	return nil
}

// parseCatalog decodes OPDS XML into a slice of CatalogEntry.
func parseCatalog(data []byte) ([]CatalogEntry, error) {
	var feed opdsFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		return nil, fmt.Errorf("parsing kiwix catalog: %w", err)
	}

	entries := make([]CatalogEntry, 0, len(feed.Entries))
	for i := range feed.Entries {
		e := &feed.Entries[i]
		name := e.Name
		if name == "" {
			name = deriveNameFromID(e.ID)
		}

		category := classifyCategory(e.Tags)

		var tags []string
		if e.Tags != "" {
			for t := range strings.SplitSeq(e.Tags, ";") {
				t = strings.TrimSpace(t)
				if t != "" {
					tags = append(tags, t)
				}
			}
		}

		var favicon string
		if e.Favicon != "" {
			favicon = e.Favicon
		} else {
			for _, link := range e.Links {
				if link.Rel == "http://opds-spec.org/image/thumbnail" || link.Rel == "http://opds-spec.org/image" {
					favicon = link.Href
					break
				}
			}
		}

		// Extract versioned content ID from the text/html link.
		var contentID string
		for _, link := range e.Links {
			if link.Type == "text/html" && strings.HasPrefix(link.Href, "/content/") {
				contentID = strings.TrimPrefix(link.Href, "/content/")
				break
			}
		}
		if contentID == "" {
			contentID = name // fallback to base name
		}

		hasFTIndex := strings.Contains(strings.ToLower(e.Tags), "_ftindex:yes")

		entries = append(entries, CatalogEntry{
			ID:               e.ID,
			Title:            e.Title,
			Description:      e.Summary,
			Language:         e.Language,
			Category:         category,
			Creator:          e.Creator,
			Publisher:        "Kiwix",
			Favicon:          favicon,
			ArticleCount:     e.ArticleCount,
			MediaCount:       e.MediaCount,
			FileSize:         e.Size,
			Tags:             tags,
			Name:             name,
			ContentID:        contentID,
			HasFulltextIndex: hasFTIndex,
		})
	}

	return entries, nil
}

// deriveNameFromID extracts an archive name from a Kiwix catalog ID.
// Kiwix IDs often look like "urn:uuid:some-uuid" or contain the ZIM filename.
func deriveNameFromID(id string) string {
	// Try to extract from a path-like ID.
	if idx := strings.LastIndex(id, "/"); idx >= 0 {
		name := id[idx+1:]
		name = strings.TrimSuffix(name, ".zim")
		return name
	}
	return id
}

// classifyCategory maps Kiwix tags to a normalized category string.
func classifyCategory(tags string) string {
	lower := strings.ToLower(tags)
	switch {
	case strings.Contains(lower, "devdocs"):
		return "devdocs"
	case strings.Contains(lower, "wikipedia"):
		return "wikipedia"
	case strings.Contains(lower, "stack_exchange"), strings.Contains(lower, "stackexchange"):
		return "stack_exchange"
	default:
		return "other"
	}
}

// parseSuggestResponse parses the JSON response from the Kiwix suggest API.
func parseSuggestResponse(data []byte) ([]SearchResult, error) {
	var suggestions []suggestResult
	if err := json.Unmarshal(data, &suggestions); err != nil {
		return nil, fmt.Errorf("parsing suggest JSON: %w", err)
	}

	results := make([]SearchResult, 0, len(suggestions))
	for _, s := range suggestions {
		if s.Path == "" {
			continue // skip pattern suggestions (no article path)
		}
		// Use value (clean text title); label contains HTML markup (<b> tags).
		results = append(results, SearchResult{
			Title: html.UnescapeString(s.Value),
			Path:  s.Path,
		})
	}

	return results, nil
}

// searchRSSFeed is the XML structure for Kiwix fulltext search results (format=xml).
type searchRSSFeed struct {
	XMLName xml.Name      `xml:"rss"`
	Channel searchChannel `xml:"channel"`
}

type searchChannel struct {
	Items []searchItem `xml:"item"`
}

type searchItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
}

// parseFulltextResponse parses the XML (RSS) response from a Kiwix fulltext
// search request (format=xml).
func parseFulltextResponse(data []byte) ([]SearchResult, error) {
	var feed searchRSSFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		return nil, fmt.Errorf("parsing search XML: %w", err)
	}

	var results []SearchResult
	for _, item := range feed.Channel.Items {
		// Strip the /content/{archive}/ prefix from search result links.
		// Kiwix returns links like /content/archive_2025-11/path/to/article
		// We want just path/to/article for use in ReadArticle.
		href := item.Link
		if rest, ok := strings.CutPrefix(href, "/content/"); ok {
			if _, after, found := strings.Cut(rest, "/"); found {
				href = after
			}
		}

		// Skip non-article links.
		if strings.HasPrefix(href, "/") || strings.HasPrefix(href, "?") {
			continue
		}

		title := strings.TrimSpace(item.Title)
		results = append(results, SearchResult{
			Title:   title,
			Path:    href,
			Snippet: html.UnescapeString(item.Description),
		})
	}

	// If all results have the same title (archive name), derive from paths.
	if len(results) > 1 {
		allSame := true
		for i := 1; i < len(results); i++ {
			if results[i].Title != results[0].Title {
				allSame = false
				break
			}
		}
		if allSame {
			for i := range results {
				results[i].Title = titleFromPath(results[i].Path)
			}
		}
	}

	return results, nil
}

// titleFromPath derives a human-readable title from a ZIM article URL path.
// Used as a fallback when Kiwix fulltext results show the archive name instead
// of individual article titles.
// Example: "100r.co/site/satellite_phone.html" → "Satellite Phone"
func titleFromPath(path string) string {
	// Use the last path segment.
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		path = path[idx+1:]
	}
	// Strip file extension.
	if dot := strings.LastIndex(path, "."); dot >= 0 {
		path = path[:dot]
	}
	// URL-decode (e.g. %20 → space).
	if decoded, err := url.PathUnescape(path); err == nil {
		path = decoded
	}
	// Replace underscores and hyphens with spaces.
	path = strings.NewReplacer("_", " ", "-", " ").Replace(path)
	// Title-case each word.
	path = strings.TrimSpace(path)
	if path == "" {
		return "Untitled"
	}
	words := strings.Fields(path)
	for i, w := range words {
		if w != "" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// Regex patterns for whitespace normalization (post-processing).
var (
	whitespaceRe  = regexp.MustCompile(`[ \t]+`)
	blankLineWSRe = regexp.MustCompile(`(?m)^[ \t]+$`)
	blankLinesRe  = regexp.MustCompile(`\n{3,}`)
)

// blockElements lists HTML elements that produce line breaks in plain text.
var blockElements = map[string]bool{
	"p": true, "div": true, "br": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"li": true, "tr": true, "blockquote": true, "pre": true, "hr": true,
	"section": true, "article": true, "header": true, "footer": true,
	"aside": true, "nav": true, "details": true, "summary": true,
	"figure": true, "figcaption": true,
	"dd": true, "dt": true, "dl": true, "ol": true, "ul": true,
	"table": true, "thead": true, "tbody": true, "tfoot": true,
	"fieldset": true,
}

// skipElements lists HTML elements whose entire subtree should be ignored.
var skipElements = map[string]bool{
	"script": true, "style": true, "head": true,
}

// extractTitle pulls the text content from the first HTML <title> element.
func extractTitle(htmlContent string) string {
	doc, err := nethtml.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return ""
	}
	var title string
	var find func(*nethtml.Node) bool
	find = func(n *nethtml.Node) bool {
		if n.Type == nethtml.ElementNode && n.Data == "title" {
			var buf strings.Builder
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == nethtml.TextNode {
					buf.WriteString(c.Data)
				}
			}
			title = strings.TrimSpace(buf.String())
			return true
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if find(c) {
				return true
			}
		}
		return false
	}
	find(doc)
	return title
}

// htmlToPlainText converts HTML content to plain text using the x/net/html
// tokenizer. It skips script/style/head subtrees, emits text nodes, and
// inserts line breaks after block-level elements.
func htmlToPlainText(content string) string {
	doc, err := nethtml.Parse(strings.NewReader(content))
	if err != nil {
		return content
	}

	var buf strings.Builder
	var walk func(*nethtml.Node)
	walk = func(n *nethtml.Node) {
		if n.Type == nethtml.ElementNode && skipElements[n.Data] {
			return
		}

		if n.Type == nethtml.ElementNode && n.Data == "br" {
			buf.WriteByte('\n')
		}

		if n.Type == nethtml.TextNode {
			buf.WriteString(n.Data)
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}

		if n.Type == nethtml.ElementNode && n.Data != "br" && blockElements[n.Data] {
			buf.WriteString("\n\n")
		}
	}
	walk(doc)

	// Normalize whitespace.
	text := whitespaceRe.ReplaceAllString(buf.String(), " ")
	text = blankLineWSRe.ReplaceAllString(text, "")
	text = blankLinesRe.ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}
