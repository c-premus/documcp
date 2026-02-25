package mcphandler

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

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

type searchConfluenceResponse struct {
	Success bool   `json:"success"`
	CQL     string `json:"cql,omitempty"`
	Results []any  `json:"results"`
	Count   int    `json:"count"`
	Message string `json:"message,omitempty"`
}

type readConfluencePageResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
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
		Description: "Search Confluence pages using CQL (Confluence Query Language) or simple text queries.\n\n" +
			"**Search Methods:**\n" +
			"- `query`: Full-text search across page content\n" +
			"- `cql`: Advanced search using Confluence Query Language\n\n" +
			"**CQL Examples:**\n" +
			"- `text ~ \"search term\"` - Full-text search\n" +
			"- `space = \"DEV\" AND type = page` - Pages in specific space\n" +
			"- `title ~ \"API\"` - Title contains text\n" +
			"- `label = \"documentation\"` - Pages with label\n" +
			"- `lastModified >= \"2024-01-01\"` - Recently modified\n\n" +
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

func (h *Handler) handleSearchConfluence(_ context.Context, _ *mcp.CallToolRequest, _ dto.SearchConfluenceInput) (*mcp.CallToolResult, searchConfluenceResponse, error) {
	resp := searchConfluenceResponse{
		Success: false,
		Results: []any{},
		Count:   0,
		Message: "Confluence search integration not yet implemented",
	}
	return nil, resp, nil
}

func (h *Handler) handleReadConfluencePage(_ context.Context, _ *mcp.CallToolRequest, _ dto.ReadConfluencePageInput) (*mcp.CallToolResult, readConfluencePageResponse, error) {
	resp := readConfluencePageResponse{
		Success: false,
		Message: "Confluence page reading not yet implemented",
	}
	return nil, resp, nil
}
