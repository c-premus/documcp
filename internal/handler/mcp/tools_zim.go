package mcphandler

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"git.999.haus/chris/DocuMCP-go/internal/dto"
)

// --- Response types ---

type listZimArchivesResponse struct {
	Success  bool             `json:"success"`
	Archives []zimArchiveItem `json:"archives"`
	Count    int              `json:"count"`
}

type zimArchiveItem struct {
	Name         string `json:"name"`
	Title        string `json:"title"`
	Description  string `json:"description,omitempty"`
	Category     string `json:"category,omitempty"`
	Language     string `json:"language"`
	ArticleCount int64  `json:"article_count"`
	FileSize     int64  `json:"file_size"`
}

type zimSearchResult struct {
	Title   string  `json:"title"`
	Path    string  `json:"path"`
	Snippet string  `json:"snippet,omitempty"`
	Score   float64 `json:"score,omitempty"`
}

type searchZimResponse struct {
	Success    bool              `json:"success"`
	Archive    string            `json:"archive"`
	Query      string            `json:"query"`
	SearchType string            `json:"search_type"`
	Results    []zimSearchResult `json:"results"`
	Count      int               `json:"count"`
	Message    string            `json:"message,omitempty"`
}

type readZimArticleResponse struct {
	Success        bool   `json:"success"`
	Archive        string `json:"archive"`
	Path           string `json:"path"`
	Title          string `json:"title,omitempty"`
	Content        string `json:"content,omitempty"`
	Truncated      bool   `json:"truncated"`
	OriginalLength int    `json:"original_length"`
	Message        string `json:"message,omitempty"`
}

// --- Tool registration ---

// registerZimTools registers ZIM archive tools (list, search, read).
func (h *Handler) registerZimTools() {
	h.registerListZimArchives()
	h.registerSearchZim()
	h.registerReadZimArticle()
}

func (h *Handler) registerListZimArchives() {
	mcp.AddTool(h.server, &mcp.Tool{
		Name: "list_zim_archives",
		Description: "List available ZIM archives with optional filtering.\n\n" +
			"**Archive Categories:**\n" +
			"- `devdocs`: Programming documentation (Laravel, PHP, JavaScript, Python, etc.)\n" +
			"- `wikipedia`: Encyclopedia content in various languages\n" +
			"- `stack_exchange`: Q&A from Stack Overflow and related sites\n" +
			"- `other`: Various documentation and reference materials\n\n" +
			"Returns archive name, title, description, category, language, article count, and file size.\n\n" +
			"**Workflow:** Use the `name` field with `search_zim` to search within an archive, " +
			"or with `read_zim_article` to read specific articles.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}, h.handleListZimArchives)
}

func (h *Handler) registerSearchZim() {
	mcp.AddTool(h.server, &mcp.Tool{
		Name: "search_zim",
		Description: "Search for articles within a ZIM archive.\n\n" +
			"**Search Types:**\n" +
			"- `fulltext` (default): Content search across article text -- works for all archives\n" +
			"- `suggest`: Fast title matching -- works best for well-indexed archives " +
			"(e.g., DevDocs, Wikipedia). May return no results for smaller or custom archives.\n\n" +
			"Returns article title, path, and snippet. Paths can be used with `read_zim_article`.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}, h.handleSearchZim)
}

func (h *Handler) registerReadZimArticle() {
	mcp.AddTool(h.server, &mcp.Tool{
		Name: "read_zim_article",
		Description: "Read article content from a ZIM archive.\n\n" +
			"**Content Options:**\n" +
			"- `summary_only`: Returns lead section only (before table of contents)\n" +
			"- `max_paragraphs`: Limits content to first N paragraphs\n\n" +
			"Returns archive name, path, title, and plain text content (converted from HTML).",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}, h.handleReadZimArticle)
}

// --- Tool handlers ---

func (h *Handler) handleListZimArchives(ctx context.Context, _ *mcp.CallToolRequest, input dto.ListZimArchivesInput) (*mcp.CallToolResult, listZimArchivesResponse, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	archives, err := h.zimArchiveRepo.List(ctx, input.Category, input.Language, input.Query, limit)
	if err != nil {
		return nil, listZimArchivesResponse{}, fmt.Errorf("listing zim archives: %w", err)
	}

	items := make([]zimArchiveItem, 0, len(archives))
	for i := range archives {
		item := zimArchiveItem{
			Name:         archives[i].Name,
			Title:        archives[i].Title,
			Language:     archives[i].Language,
			ArticleCount: archives[i].ArticleCount,
			FileSize:     archives[i].FileSize,
		}
		if archives[i].Description.Valid {
			item.Description = archives[i].Description.String
		}
		if archives[i].Category.Valid {
			item.Category = archives[i].Category.String
		}
		items = append(items, item)
	}

	resp := listZimArchivesResponse{
		Success:  true,
		Archives: items,
		Count:    len(items),
	}
	return nil, resp, nil
}

func (h *Handler) handleSearchZim(ctx context.Context, _ *mcp.CallToolRequest, input dto.SearchZimInput) (*mcp.CallToolResult, searchZimResponse, error) {
	if h.kiwixClient == nil {
		return nil, searchZimResponse{
			Success: false,
			Archive: input.Archive,
			Query:   input.Query,
			Results: []zimSearchResult{},
			Message: "Kiwix service not configured",
		}, nil
	}

	searchType := input.SearchType
	if searchType == "" {
		searchType = "fulltext"
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	results, err := h.kiwixClient.Search(ctx, input.Archive, input.Query, searchType, limit)
	if err != nil {
		return nil, searchZimResponse{}, fmt.Errorf("searching zim archive %s: %w", input.Archive, err)
	}

	items := make([]zimSearchResult, 0, len(results))
	for _, r := range results {
		items = append(items, zimSearchResult{
			Title:   r.Title,
			Path:    r.Path,
			Snippet: r.Snippet,
			Score:   r.Score,
		})
	}

	return nil, searchZimResponse{
		Success:    true,
		Archive:    input.Archive,
		Query:      input.Query,
		SearchType: searchType,
		Results:    items,
		Count:      len(items),
	}, nil
}

func (h *Handler) handleReadZimArticle(ctx context.Context, _ *mcp.CallToolRequest, input dto.ReadZimArticleInput) (*mcp.CallToolResult, readZimArticleResponse, error) {
	if h.kiwixClient == nil {
		return nil, readZimArticleResponse{
			Success: false,
			Archive: input.Archive,
			Path:    input.Path,
			Message: "Kiwix service not configured",
		}, nil
	}

	article, err := h.kiwixClient.ReadArticle(ctx, input.Archive, input.Path)
	if err != nil {
		return nil, readZimArticleResponse{}, fmt.Errorf("reading zim article: %w", err)
	}

	content := article.Content
	originalLength := len(content)
	content, truncated := truncateContent(content, input.SummaryOnly, input.MaxParagraphs)

	return nil, readZimArticleResponse{
		Success:        true,
		Archive:        input.Archive,
		Path:           input.Path,
		Title:          article.Title,
		Content:        content,
		Truncated:      truncated,
		OriginalLength: originalLength,
	}, nil
}
