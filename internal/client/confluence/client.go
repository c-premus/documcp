// Package confluence provides an HTTP client for the Confluence REST API v1.
// It supports listing spaces, searching pages via CQL or full-text queries,
// and reading page content with automatic XHTML-to-Markdown conversion.
package confluence

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"git.999.haus/chris/DocuMCP-go/internal/security"
)

// Cache TTL constants.
const (
	spaceCacheTTL  = 1 * time.Hour
	searchCacheTTL = 5 * time.Minute
	pageCacheTTL   = 10 * time.Minute
)

// Client communicates with a Confluence instance over HTTP.
type Client struct {
	baseURL    string
	email      string
	apiToken   string
	httpClient *http.Client
	cache      *cache
	logger     *slog.Logger
}

// NewClient creates a new Confluence REST API client.
// All requests use Basic Auth with the provided email and API token.
// It validates the base URL against SSRF attacks.
func NewClient(baseURL, email, apiToken string, logger *slog.Logger) (*Client, error) {
	if err := security.ValidateExternalURL(baseURL); err != nil {
		return nil, fmt.Errorf("invalid confluence base URL: %w", err)
	}

	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		email:    email,
		apiToken: apiToken,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		cache:  newCache(),
		logger: logger,
	}, nil
}

// Health checks connectivity to the Confluence instance.
// It issues a lightweight GET request and returns nil on HTTP 200.
func (c *Client) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := c.doGet(ctx, "/rest/api/settings/lookandfeel", nil)
	if err != nil {
		return fmt.Errorf("confluence health check: %w", err)
	}
	return nil
}

// ListSpaces returns Confluence spaces, optionally filtered by type and name/key query.
// Results are cached for 1 hour.
func (c *Client) ListSpaces(ctx context.Context, spaceType, query string, limit int) ([]Space, error) {
	if limit <= 0 {
		limit = 50
	}

	cacheKey := fmt.Sprintf("spaces:%s:%d", spaceType, limit)
	if cached, ok := c.cache.get(cacheKey); ok {
		c.logger.Debug("spaces cache hit", "key", cacheKey)
		spaces := cached.([]Space)
		return filterSpaces(spaces, query), nil
	}

	params := url.Values{}
	if spaceType != "" {
		params.Set("type", spaceType)
	}
	params.Set("limit", strconv.Itoa(limit))
	params.Set("expand", "description.plain")

	body, err := c.doGet(ctx, "/rest/api/space", params)
	if err != nil {
		return nil, fmt.Errorf("listing spaces: %w", err)
	}

	var resp apiSpaceResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decoding spaces response: %w", err)
	}

	spaces := make([]Space, 0, len(resp.Results))
	for _, s := range resp.Results {
		space := Space{
			ID:          strconv.Itoa(s.ID),
			Key:         s.Key,
			Name:        s.Name,
			Description: s.Description.Plain.Value,
			Type:        s.Type,
			Status:      s.Status,
		}
		if s.Homepage != nil {
			space.HomepageID = s.Homepage.ID
		}
		if s.Icon != nil && s.Icon.Path != "" {
			space.IconURL = c.baseURL + s.Icon.Path
		}
		spaces = append(spaces, space)
	}

	c.cache.set(cacheKey, spaces, spaceCacheTTL)
	c.logger.Debug("spaces fetched and cached", "count", len(spaces))

	return filterSpaces(spaces, query), nil
}

// SearchPages searches for pages using CQL or a simple text query.
// If CQL is provided it takes precedence. A Space filter is ANDed into the
// CQL expression. Results are cached for 5 minutes.
func (c *Client) SearchPages(ctx context.Context, params SearchPagesParams) (SearchResult, error) {
	cql := buildSearchCQL(params)
	if cql == "" {
		return SearchResult{}, fmt.Errorf("searching pages: either cql or query is required")
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 25
	}

	cacheKey := fmt.Sprintf("search:%s:%d:%d", cql, limit, params.Start)
	if cached, ok := c.cache.get(cacheKey); ok {
		c.logger.Debug("search cache hit", "cql", cql)
		return cached.(SearchResult), nil
	}

	qp := url.Values{}
	qp.Set("cql", cql)
	qp.Set("limit", strconv.Itoa(limit))
	qp.Set("start", strconv.Itoa(params.Start))

	body, err := c.doGet(ctx, "/rest/api/content/search", qp)
	if err != nil {
		return SearchResult{}, fmt.Errorf("searching pages: %w", err)
	}

	var resp apiSearchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return SearchResult{}, fmt.Errorf("decoding search response: %w", err)
	}

	pages := make([]PageSummary, 0, len(resp.Results))
	for _, r := range resp.Results {
		ps := PageSummary{
			ID:       r.Content.ID,
			Title:    r.Content.Title,
			SpaceKey: r.Content.Space.Key,
			WebURL:   r.Content.Links.WebUI,
			Excerpt:  stripHTML(r.Excerpt),
		}
		if r.Content.Metadata != nil {
			for _, l := range r.Content.Metadata.Labels.Results {
				ps.Labels = append(ps.Labels, l.Name)
			}
		}
		pages = append(pages, ps)
	}

	result := SearchResult{
		Pages:   pages,
		Total:   resp.TotalSize,
		Start:   resp.Start,
		Limit:   resp.Limit,
		HasMore: resp.Links.Next != "",
		CQL:     cql,
	}

	c.cache.set(cacheKey, result, searchCacheTTL)
	c.logger.Debug("search results cached", "cql", cql, "count", len(pages))

	return result, nil
}

// ReadPage fetches a full Confluence page by ID and converts its storage
// format content to Markdown. Results are cached for 10 minutes.
func (c *Client) ReadPage(ctx context.Context, pageID string) (*Page, error) {
	cacheKey := "page:" + pageID
	if cached, ok := c.cache.get(cacheKey); ok {
		c.logger.Debug("page cache hit", "page_id", pageID)
		p := cached.(*Page)
		return p, nil
	}

	params := url.Values{}
	params.Set("expand", "body.storage,version,ancestors,metadata.labels,space")

	body, err := c.doGet(ctx, "/rest/api/content/"+pageID, params)
	if err != nil {
		return nil, fmt.Errorf("reading page %s: %w", pageID, err)
	}

	var raw apiContentResult
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decoding page %s: %w", pageID, err)
	}

	page := contentResultToPage(raw)
	c.cache.set(cacheKey, page, pageCacheTTL)
	c.logger.Debug("page fetched and cached", "page_id", pageID, "title", page.Title)

	return page, nil
}

// ReadPageByTitle looks up a page by space key and exact title.
// It returns the first matching result.
func (c *Client) ReadPageByTitle(ctx context.Context, spaceKey, title string) (*Page, error) {
	params := url.Values{}
	params.Set("spaceKey", spaceKey)
	params.Set("title", title)
	params.Set("expand", "body.storage,version,ancestors,metadata.labels,space")

	body, err := c.doGet(ctx, "/rest/api/content", params)
	if err != nil {
		return nil, fmt.Errorf("reading page by title %q in space %s: %w", title, spaceKey, err)
	}

	var resp apiContentResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decoding page-by-title response: %w", err)
	}

	if len(resp.Results) == 0 {
		return nil, fmt.Errorf("page %q not found in space %s", title, spaceKey)
	}

	page := contentResultToPage(resp.Results[0])
	return page, nil
}

// doGet executes an authenticated GET request and returns the response body.
func (c *Client) doGet(ctx context.Context, path string, params url.Values) ([]byte, error) {
	u := c.baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request for %s: %w", path, err)
	}

	// Basic Auth: base64(email:apiToken).
	creds := base64.StdEncoding.EncodeToString([]byte(c.email + ":" + c.apiToken))
	req.Header.Set("Authorization", "Basic "+creds)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request for %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 10 MB max
	if err != nil {
		return nil, fmt.Errorf("reading response body from %s: %w", path, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status %d from %s: %s", resp.StatusCode, path, truncate(string(body), 200))
	}

	return body, nil
}

// buildSearchCQL constructs a CQL string from SearchPagesParams.
// User-supplied values are escaped to prevent CQL injection.
func buildSearchCQL(params SearchPagesParams) string {
	var cql string

	switch {
	case params.CQL != "":
		cql = params.CQL
	case params.Query != "":
		cql = fmt.Sprintf(`text~"%s"`, escapeCQL(params.Query))
	default:
		return ""
	}

	if params.Space != "" {
		cql = fmt.Sprintf(`space="%s" AND (%s)`, escapeCQL(params.Space), cql)
	}

	return cql
}

// escapeCQL escapes double quotes and backslashes in user input to prevent
// CQL injection when embedding values in CQL query strings.
func escapeCQL(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

// contentResultToPage converts a raw API content result into a Page with
// Markdown content.
func contentResultToPage(raw apiContentResult) *Page {
	page := &Page{
		ID:        raw.ID,
		Title:     raw.Title,
		SpaceKey:  raw.Space.Key,
		WebURL:    raw.Links.WebUI,
		Version:   raw.Version.Number,
		UpdatedAt: raw.Version.When,
		CreatedAt: raw.History.CreatedDate,
		Content:   storageToMarkdown(raw.Body.Storage.Value),
	}

	for _, a := range raw.Ancestors {
		page.Ancestors = append(page.Ancestors, a.ID)
	}

	if len(page.Ancestors) > 0 {
		page.ParentID = page.Ancestors[len(page.Ancestors)-1]
	}

	if raw.Metadata != nil {
		for _, l := range raw.Metadata.Labels.Results {
			page.Labels = append(page.Labels, l.Name)
		}
	}

	return page
}

// filterSpaces returns spaces whose name or key contain the query string
// (case-insensitive). If query is empty, all spaces are returned.
func filterSpaces(spaces []Space, query string) []Space {
	if query == "" {
		return spaces
	}
	q := strings.ToLower(query)
	filtered := make([]Space, 0)
	for _, s := range spaces {
		if strings.Contains(strings.ToLower(s.Name), q) ||
			strings.Contains(strings.ToLower(s.Key), q) {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

// stripHTML removes HTML tags from a string, used for cleaning excerpts.
func stripHTML(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// truncate returns at most n characters of s, appending "..." if truncated.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
