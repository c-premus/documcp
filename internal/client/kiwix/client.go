// Package kiwix provides an HTTP client for communicating with a Kiwix Serve
// instance. Kiwix Serve exposes ZIM archives via HTTP, providing an OPDS
// catalog, search, and article retrieval.
package kiwix

import (
	"context"
	"encoding/json"
	"encoding/xml"
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

	"git.999.haus/chris/DocuMCP-go/internal/security"
)

const (
	// catalogCacheKey is the cache key for the OPDS catalog.
	catalogCacheKey = "catalog"

	// catalogCacheTTL is how long the parsed catalog is cached.
	catalogCacheTTL = 1 * time.Hour
)

// Client communicates with a Kiwix Serve instance over HTTP.
type Client struct {
	baseURL    string
	httpClient *http.Client
	cache      *cache
	logger     *slog.Logger
}

// NewClient creates a new Kiwix HTTP client targeting the given base URL.
// It validates the base URL against SSRF attacks.
func NewClient(baseURL string, logger *slog.Logger) (*Client, error) {
	if err := security.ValidateExternalURL(baseURL); err != nil {
		return nil, fmt.Errorf("invalid kiwix base URL: %w", err)
	}

	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout:   10 * time.Second,
			Transport: security.SafeTransport(),
		},
		cache:  newCache(),
		logger: logger,
	}, nil
}

// Health checks whether the Kiwix Serve instance is reachable by fetching
// the OPDS catalog root. Returns nil on HTTP 200, an error otherwise.
func (c *Client) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/catalog/root.xml", nil)
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/catalog/root.xml", nil)
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

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("reading catalog response: %w", err)
	}

	entries, err := parseCatalog(body)
	if err != nil {
		return nil, fmt.Errorf("parsing catalog XML: %w", err)
	}

	c.cache.set(catalogCacheKey, entries, catalogCacheTTL)
	c.logger.Info("catalog fetched and cached", "count", len(entries))

	return entries, nil
}

// Search queries the Kiwix Serve instance for articles matching the given
// query. The searchType must be "suggest" or "fulltext".
func (c *Client) Search(ctx context.Context, archiveName, query, searchType string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 10
	}

	var reqURL string
	switch searchType {
	case "suggest":
		params := url.Values{
			"term":    {query},
			"limit":   {strconv.Itoa(limit)},
			"content": {archiveName},
		}
		reqURL = c.baseURL + "/suggest?" + params.Encode()
	case "fulltext":
		params := url.Values{
			"pattern":    {query},
			"pageLength": {strconv.Itoa(limit)},
			"books":      {"name:" + archiveName},
		}
		reqURL = c.baseURL + "/search?" + params.Encode()
	default:
		return nil, fmt.Errorf("unsupported search type %q (must be suggest or fulltext)", searchType)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
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

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("reading search response: %w", err)
	}

	switch searchType {
	case "suggest":
		return parseSuggestResponse(body)
	case "fulltext":
		return parseFulltextResponse(body), nil
	default:
		return nil, nil
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

	reqURL := c.baseURL + "/" + archiveName + "/" + articlePath

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
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

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
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
		return fmt.Errorf("path must not start with /")
	}
	if strings.Contains(path, "..") {
		return fmt.Errorf("path must not contain dot-dot segments")
	}
	if strings.ContainsRune(path, 0) {
		return fmt.Errorf("path must not contain null bytes")
	}
	return nil
}

// validateArchiveName rejects archive names that could be used for path traversal.
func validateArchiveName(name string) error {
	if name == "" {
		return fmt.Errorf("archive name must not be empty")
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("archive name must not contain path separators")
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("archive name must not contain dot-dot segments")
	}
	if strings.ContainsRune(name, 0) {
		return fmt.Errorf("archive name must not contain null bytes")
	}
	return nil
}

// parseCatalog decodes OPDS XML into a slice of CatalogEntry.
func parseCatalog(data []byte) ([]CatalogEntry, error) {
	var feed opdsFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		return nil, err
	}

	entries := make([]CatalogEntry, 0, len(feed.Entries))
	for _, e := range feed.Entries {
		name := e.Name
		if name == "" {
			name = deriveNameFromID(e.ID)
		}

		category := classifyCategory(e.Tags)

		var tags []string
		if e.Tags != "" {
			for _, t := range strings.Split(e.Tags, ";") {
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

		entries = append(entries, CatalogEntry{
			ID:           e.ID,
			Title:        e.Title,
			Description:  e.Summary,
			Language:     e.Language,
			Category:     category,
			Creator:      e.Creator,
			Publisher:    "Kiwix",
			Favicon:      favicon,
			ArticleCount: e.ArticleCount,
			MediaCount:   e.MediaCount,
			FileSize:     e.Size,
			Tags:         tags,
			Name:         name,
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
		results = append(results, SearchResult{
			Title: s.Label,
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

		result := SearchResult{
			Title: html.UnescapeString(match[2]),
			Path:  match[1],
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
	whitespaceRe = regexp.MustCompile(`[ \t]+`)
	blankLinesRe = regexp.MustCompile(`\n{3,}`)
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
	text = blankLinesRe.ReplaceAllString(text, "\n\n")
	text = strings.TrimSpace(text)

	return text
}
