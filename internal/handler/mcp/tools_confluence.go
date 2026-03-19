package mcphandler

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"git.999.haus/chris/DocuMCP-go/internal/client/confluence"
	"git.999.haus/chris/DocuMCP-go/internal/dto"
)

// --- Response types ---

type listConfluenceSpacesResponse struct {
	Success bool                  `json:"success"`
	Spaces  []confluenceSpaceItem `json:"spaces"`
	Count   int                   `json:"count"`
}

type confluenceSpaceItem struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"`
}

type confluenceSearchResult struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	SpaceKey string   `json:"space_key"`
	WebURL   string   `json:"web_url"`
	Excerpt  string   `json:"excerpt,omitempty"`
	Labels   []string `json:"labels,omitempty"`
}

type searchConfluenceResponse struct {
	Success bool                     `json:"success"`
	Results []confluenceSearchResult `json:"results"`
	Count   int                      `json:"count"`
	Message string                   `json:"message,omitempty"`
}

type confluencePageDetail struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	SpaceKey  string   `json:"space_key"`
	WebURL    string   `json:"web_url"`
	Version   int      `json:"version"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
	Labels    []string `json:"labels,omitempty"`
	Ancestors []string `json:"ancestors,omitempty"`
}

type readConfluencePageResponse struct {
	Success        bool                  `json:"success"`
	Page           *confluencePageDetail `json:"page,omitempty"`
	Content        string                `json:"content,omitempty"`
	OriginalLength int                   `json:"original_length,omitempty"`
	Truncated      bool                  `json:"truncated,omitempty"`
	Message        string                `json:"message,omitempty"`
}

// --- Tool registration ---

// registerConfluenceTools registers Confluence tools (list spaces, search, read page).
func (h *Handler) registerConfluenceTools() {
	h.registerListConfluenceSpaces()
	h.registerSearchConfluence()
	h.registerReadConfluencePage()
}

func (h *Handler) registerListConfluenceSpaces() {
	mcp.AddTool(h.server, &mcp.Tool{
		Name: "list_confluence_spaces",
		Description: "List available Confluence spaces.\n\n" +
			"**Space Types:**\n" +
			"- `global`: Organization-wide spaces\n" +
			"- `personal`: User-specific spaces\n" +
			"- `knowledge_base`: Knowledge base spaces\n\n" +
			"Returns space keys, names, and descriptions. Space keys can be used with " +
			"`search_confluence` to filter results.\n\n" +
			"**Workflow:** Use the `key` field as `space` parameter in `search_confluence` " +
			"to search within a specific space.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}, h.handleListConfluenceSpaces)
}

func (h *Handler) registerSearchConfluence() {
	mcp.AddTool(h.server, &mcp.Tool{
		Name: "search_confluence",
		Description: "Search Confluence pages using text queries.\n\n" +
			"**Search Methods:**\n" +
			"- `query`: Full-text search across page content\n" +
			"- `space`: Filter results to a specific space by key\n\n" +
			"Returns page ID, title, space key, URL, excerpt, and labels. Max 50 results.\n\n" +
			"**Workflow:** Use `id` from results with `read_confluence_page` (as `page_id`) " +
			"to fetch full page content.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}, h.handleSearchConfluence)
}

func (h *Handler) registerReadConfluencePage() {
	mcp.AddTool(h.server, &mcp.Tool{
		Name: "read_confluence_page",
		Description: "Read the full content of a Confluence page as markdown.\n\n" +
			"**Page Identification:**\n" +
			"- `page_id`: Confluence page ID (from search results)\n" +
			"- `space_key` + `title`: Space key and exact page title\n\n" +
			"**Content Options:**\n" +
			"- `summary_only`: Returns first section only\n" +
			"- `max_paragraphs`: Limits content to first N paragraphs\n\n" +
			"Returns page metadata (ID, title, space, URL, version, dates, labels, ancestors) " +
			"and markdown content.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}, h.handleReadConfluencePage)
}

// --- Tool handlers ---

func (h *Handler) handleListConfluenceSpaces(ctx context.Context, _ *mcp.CallToolRequest, input dto.ListConfluenceSpacesInput) (*mcp.CallToolResult, listConfluenceSpacesResponse, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	spaces, err := h.confluenceSpaceRepo.List(ctx, input.Type, input.Query, limit)
	if err != nil {
		return nil, listConfluenceSpacesResponse{}, fmt.Errorf("listing confluence spaces: %w", err)
	}

	items := make([]confluenceSpaceItem, 0, len(spaces))
	for _, s := range spaces {
		item := confluenceSpaceItem{
			Key:  s.Key,
			Name: s.Name,
			Type: s.Type,
		}
		if s.Description.Valid {
			item.Description = s.Description.String
		}
		items = append(items, item)
	}

	resp := listConfluenceSpacesResponse{
		Success: true,
		Spaces:  items,
		Count:   len(items),
	}
	return nil, resp, nil
}

func (h *Handler) handleSearchConfluence(ctx context.Context, _ *mcp.CallToolRequest, input dto.SearchConfluenceInput) (*mcp.CallToolResult, searchConfluenceResponse, error) {
	if h.confluenceClient == nil {
		return nil, searchConfluenceResponse{
			Success: false,
			Results: []confluenceSearchResult{},
			Message: "Confluence service not configured",
		}, nil
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 25
	}
	if limit > 50 {
		limit = 50
	}

	result, err := h.confluenceClient.SearchPages(ctx, confluence.SearchPagesParams{
		Query: input.Query,
		Space: input.Space,
		Limit: limit,
	})
	if err != nil {
		return nil, searchConfluenceResponse{}, fmt.Errorf("searching confluence: %w", err)
	}

	items := make([]confluenceSearchResult, 0, len(result.Pages))
	for _, p := range result.Pages {
		items = append(items, confluenceSearchResult{
			ID:       p.ID,
			Title:    p.Title,
			SpaceKey: p.SpaceKey,
			WebURL:   p.WebURL,
			Excerpt:  p.Excerpt,
			Labels:   p.Labels,
		})
	}

	return nil, searchConfluenceResponse{
		Success: true,
		Results: items,
		Count:   len(items),
	}, nil
}

func (h *Handler) handleReadConfluencePage(ctx context.Context, _ *mcp.CallToolRequest, input dto.ReadConfluencePageInput) (*mcp.CallToolResult, readConfluencePageResponse, error) {
	if h.confluenceClient == nil {
		return nil, readConfluencePageResponse{
			Success: false,
			Message: "Confluence service not configured",
		}, nil
	}

	var page *confluence.Page
	var err error

	if input.PageID != "" {
		page, err = h.confluenceClient.ReadPage(ctx, input.PageID)
	} else if input.SpaceKey != "" && input.Title != "" {
		page, err = h.confluenceClient.ReadPageByTitle(ctx, input.SpaceKey, input.Title)
	} else {
		return nil, readConfluencePageResponse{
			Success: false,
			Message: "Either page_id or both space_key and title are required",
		}, nil
	}

	if err != nil {
		return nil, readConfluencePageResponse{}, fmt.Errorf("reading confluence page: %w", err)
	}

	content := page.Content
	originalLength := len(content)
	content, truncated := truncateContent(content, input.SummaryOnly, input.MaxParagraphs)

	meta := &confluencePageDetail{
		ID:        page.ID,
		Title:     page.Title,
		SpaceKey:  page.SpaceKey,
		WebURL:    page.WebURL,
		Version:   page.Version,
		CreatedAt: page.CreatedAt,
		UpdatedAt: page.UpdatedAt,
		Labels:    page.Labels,
		Ancestors: page.Ancestors,
	}

	return nil, readConfluencePageResponse{
		Success:        true,
		Page:           meta,
		Content:        content,
		OriginalLength: originalLength,
		Truncated:      truncated,
	}, nil
}
