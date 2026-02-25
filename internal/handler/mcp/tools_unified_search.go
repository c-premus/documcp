package mcphandler

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"git.999.haus/chris/DocuMCP-go/internal/dto"
)

// --- Response types ---

type unifiedSearchResponse struct {
	Success          bool     `json:"success"`
	Message          string   `json:"message,omitempty"`
	Query            string   `json:"query"`
	Results          []any    `json:"results"`
	Total            int      `json:"total"`
	SourcesSearched  []string `json:"sources_searched"`
	ProcessingTimeMs int      `json:"processing_time_ms"`
}

// --- Tool registration ---

// registerUnifiedSearchTool registers the unified cross-source search tool.
func (h *Handler) registerUnifiedSearchTool() {
	mcp.AddTool(h.server, &mcp.Tool{
		Name: "unified_search",
		Description: "Search across ALL content types in a single request: Documents, Git Templates, " +
			"ZIM Archives, and Confluence Spaces.\n\n" +
			"Returns results ranked by relevance with a `source` field indicating the content type. " +
			"Use for discovery -- then use type-specific tools (search_documents, search_zim, " +
			"search_confluence, search_git_templates) for deep content search with full filter options.\n\n" +
			"**Sources** (filter with `types` param):\n" +
			"- `document` -- Uploaded documents (PDF, DOCX, XLSX, HTML, Markdown)\n" +
			"- `git_template` -- Git template README and metadata\n" +
			"- `zim_archive` -- ZIM archive metadata (DevDocs, Wikipedia, Stack Exchange)\n" +
			"- `confluence_space` -- Confluence space metadata",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}, h.handleUnifiedSearch)
}

// --- Tool handler ---

func (h *Handler) handleUnifiedSearch(
	_ context.Context,
	_ *mcp.CallToolRequest,
	input dto.UnifiedSearchInput,
) (*mcp.CallToolResult, unifiedSearchResponse, error) {
	return nil, unifiedSearchResponse{
		Success:          false,
		Message:          "Meilisearch unified search not yet implemented",
		Query:            input.Query,
		Results:          []any{},
		Total:            0,
		SourcesSearched:  []string{},
		ProcessingTimeMs: 0,
	}, nil
}
