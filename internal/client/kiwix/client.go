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
			Transport: security.SafeTransportAllowPrivate(cfg.SSRFDialerTimeout),
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
// the best available results.
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
	if searchType == "fulltext" && !hasFTIndex {
		c.logger.Debug("fulltext index not available, falling back to suggest",
			"archive", archiveName,
		)
		searchType = "suggest"
	}

	results, err := c.doSearch(ctx, contentID, query, searchType, limit)
	if err != nil {
		return nil, err
	}

	return results, nil
}

// doSearch performs the actual HTTP request for a search query.
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
		return parseFulltextResponse(body), nil
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
		results = append(results, SearchResult{
			Title: html.UnescapeString(s.Value),
			Path:  s.Path,
		})
	}

	return results, nil
}

// Regex patterns for parsing fulltext search HTML results.
var (
	fulltextArticleRe = regexp.MustCompile(`<a[^>]+href="([^"]+)"[^>]*>([^<]+)</a>`)
	fulltextSnippetRe = regexp.MustCompile(`<cite>([^<]*)</cite>`)
)

// parseFulltextResponse extracts search results from the Kiwix fulltext
// search HTML page. This is a best-effort parser since the response is HTML.
func parseFulltextResponse(data []byte) []SearchResult {
	body := string(data)

	links := fulltextArticleRe.FindAllStringSubmatch(body, -1)
	snippets := fulltextSnippetRe.FindAllStringSubmatch(body, -1)

	var results []SearchResult
	for i, match := range links {
		if len(match) < 3 {
			continue
		}

		// Strip the /content/{archive}/ prefix from fulltext search result hrefs.
		// Kiwix returns hrefs like /content/archive_2025-11/path/to/article
		// We want just path/to/article for use in ReadArticle.
		href := match[1]
		if rest, ok := strings.CutPrefix(href, "/content/"); ok {
			if _, after, found := strings.Cut(rest, "/"); found {
				href = after
			}
		}

		// Skip pagination links and other non-article hrefs.
		if strings.HasPrefix(href, "/") || strings.HasPrefix(href, "?") {
			continue
		}

		result := SearchResult{
			Title: strings.TrimSpace(html.UnescapeString(match[2])),
			Path:  href,
		}
		if i < len(snippets) && len(snippets[i]) >= 2 {
			result.Snippet = html.UnescapeString(snippets[i][1])
		}

		results = append(results, result)
	}

	return results
}

// Regex patterns for HTML to plain text conversion.
var (
	scriptRe     = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	styleRe      = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	htmlTagRe    = regexp.MustCompile(`<[^>]+>`)
	whitespaceRe  = regexp.MustCompile(`[ \t]+`)
	blankLineWSRe = regexp.MustCompile(`(?m)^[ \t]+$`)
	blankLinesRe  = regexp.MustCompile(`\n{3,}`)
	titleRe      = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
)

// extractTitle pulls the text content from an HTML <title> tag.
func extractTitle(htmlContent string) string {
	match := titleRe.FindStringSubmatch(htmlContent)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(html.UnescapeString(match[1]))
}

// htmlToPlainText converts HTML content to plain text by stripping tags,
// removing script/style elements, and decoding HTML entities.
func htmlToPlainText(content string) string {
	// Remove script and style blocks.
	text := scriptRe.ReplaceAllString(content, "")
	text = styleRe.ReplaceAllString(text, "")

	// Replace block-level tags with newlines for readability.
	blockReplacer := strings.NewReplacer(
		"<br>", "\n",
		"<br/>", "\n",
		"<br />", "\n",
		"</p>", "\n\n",
		"</div>", "\n",
		"</li>", "\n",
		"</tr>", "\n",
		"</h1>", "\n\n",
		"</h2>", "\n\n",
		"</h3>", "\n\n",
		"</h4>", "\n\n",
		"</h5>", "\n\n",
		"</h6>", "\n\n",
	)
	text = blockReplacer.Replace(text)

	// Strip remaining HTML tags.
	text = htmlTagRe.ReplaceAllString(text, "")

	// Decode HTML entities.
	text = html.UnescapeString(text)

	// Normalize whitespace.
	text = whitespaceRe.ReplaceAllString(text, " ")
	text = blankLineWSRe.ReplaceAllString(text, "")
	text = blankLinesRe.ReplaceAllString(text, "\n\n")
	text = strings.TrimSpace(text)

	return text
}
