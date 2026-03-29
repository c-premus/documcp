package mcphandler

import (
	"context"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	authscope "github.com/c-premus/documcp/internal/auth/scope"
	"github.com/c-premus/documcp/internal/dto"
)

// --- Response types ---

type listZimArchivesResponse struct {
	Success  bool             `json:"success"`
	Archives []zimArchiveItem `json:"archives"`
	Count    int              `json:"count"`
}

type zimArchiveItem struct {
	Name             string `json:"name"`
	Title            string `json:"title"`
	Description      string `json:"description,omitempty"`
	Category         string `json:"category,omitempty"`
	Language         string `json:"language"`
	ArticleCount     int64  `json:"article_count"`
	FileSize         int64  `json:"file_size"`
	HasFulltextIndex bool   `json:"has_fulltext_index"`
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
	Fallback   bool              `json:"fallback,omitempty"` // true when fulltext was requested but suggest was used
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
			"Returns archive name, title, description, category, language, article count, file size, " +
			"and `has_fulltext_index` (whether deep content search is available vs title matching only).\n\n" +
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
			"- `fulltext` (default): Content search across article text. If the archive lacks a " +
			"fulltext index, automatically falls back to `suggest` (title matching) and sets " +
			"`fallback: true` in the response.\n" +
			"- `suggest`: Fast title matching.\n\n" +
			"Check `has_fulltext_index` from `list_zim_archives` to see which archives support " +
			"deep content search vs title matching only.\n\n" +
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
	if err := requireMCPScope(ctx, authscope.MCPRead); err != nil {
		return nil, listZimArchivesResponse{}, errors.New("mcp:read scope required")
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	archives, err := h.zimArchiveRepo.List(ctx, input.Category, input.Language, input.Query, limit, 0)
	if err != nil {
		return nil, listZimArchivesResponse{}, fmt.Errorf("listing zim archives: %w", err)
	}

	items := make([]zimArchiveItem, 0, len(archives))
	for i := range archives {
		item := zimArchiveItem{
			Name:             archives[i].Name,
			Title:            archives[i].Title,
			Language:         archives[i].Language,
			ArticleCount:     archives[i].ArticleCount,
			FileSize:         archives[i].FileSize,
			HasFulltextIndex: archives[i].HasFulltextIndex,
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
	if err := requireMCPScope(ctx, authscope.MCPRead); err != nil {
		return nil, searchZimResponse{}, errors.New("mcp:read scope required")
	}
	kiwixClient, _ := h.getKiwixClient(ctx)
	if kiwixClient == nil {
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

	// Detect whether the client will fall back from fulltext to suggest.
	fallback := searchType == "fulltext" && !kiwixClient.HasFulltextIndex(ctx, input.Archive)

	results, err := kiwixClient.Search(ctx, input.Archive, input.Query, searchType, limit)
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

	effectiveType := searchType
	if fallback {
		effectiveType = "suggest"
	}

	resp := searchZimResponse{
		Success:    true,
		Archive:    input.Archive,
		Query:      input.Query,
		SearchType: effectiveType,
		Results:    items,
		Count:      len(items),
		Fallback:   fallback,
	}
	if fallback {
		resp.Message = "Fulltext search not available for this archive; results are from title matching (suggest)."
	}

	return nil, resp, nil
}

func (h *Handler) handleReadZimArticle(ctx context.Context, _ *mcp.CallToolRequest, input dto.ReadZimArticleInput) (*mcp.CallToolResult, readZimArticleResponse, error) {
	if err := requireMCPScope(ctx, authscope.MCPRead); err != nil {
		return nil, readZimArticleResponse{}, errors.New("mcp:read scope required")
	}
	kiwixClient, _ := h.getKiwixClient(ctx)
	if kiwixClient == nil {
		return nil, readZimArticleResponse{
			Success: false,
			Archive: input.Archive,
			Path:    input.Path,
			Message: "Kiwix service not configured",
		}, nil
	}

	article, err := kiwixClient.ReadArticle(ctx, input.Archive, input.Path)
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
