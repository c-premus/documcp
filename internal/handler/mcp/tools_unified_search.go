package mcphandler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	authmiddleware "git.999.haus/chris/DocuMCP-go/internal/auth/middleware"
	authscope "git.999.haus/chris/DocuMCP-go/internal/auth/scope"
	"git.999.haus/chris/DocuMCP-go/internal/dto"
	"git.999.haus/chris/DocuMCP-go/internal/search"
)

// --- Response types ---

type unifiedSearchResult struct {
	Source      string  `json:"source"`
	UUID        string  `json:"uuid,omitempty"`
	Title       string  `json:"title,omitempty"`
	Description string  `json:"description,omitempty"`
	Score       float64 `json:"score,omitempty"`
}

type unifiedSearchResponse struct {
	Success          bool                  `json:"success"`
	Message          string                `json:"message,omitempty"`
	Query            string                `json:"query"`
	Results          []unifiedSearchResult `json:"results"`
	Total            int                   `json:"total"`
	SourcesSearched  []string              `json:"sources_searched"`
	ProcessingTimeMs int                   `json:"processing_time_ms"`
}

// --- Tool registration ---

// registerUnifiedSearchTool registers the unified cross-source search tool.
func (h *Handler) registerUnifiedSearchTool() {
	mcp.AddTool(h.server, &mcp.Tool{
		Name: "unified_search",
		Description: "Search across ALL content types in a single request: Documents, Git Templates, " +
			"and ZIM Archives.\n\n" +
			"Returns results ranked by relevance with a `source` field indicating the content type. " +
			"Use for discovery -- then use type-specific tools (search_documents, search_zim, " +
			"search_git_templates) for deep content search with full filter options.\n\n" +
			"**Sources** (filter with `types` param):\n" +
			"- `document` -- Uploaded documents (PDF, DOCX, XLSX, HTML, Markdown)\n" +
			"- `git_template` -- Git template README and metadata\n" +
			"- `zim_archive` -- ZIM archive metadata (DevDocs, Wikipedia, Stack Exchange)",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}, h.handleUnifiedSearch)
}

// --- Tool handler ---

func (h *Handler) handleUnifiedSearch(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input dto.UnifiedSearchInput,
) (*mcp.CallToolResult, unifiedSearchResponse, error) {
	if err := requireMCPScope(ctx, authscope.MCPRead); err != nil {
		return nil, unifiedSearchResponse{}, errors.New("mcp:read scope required")
	}
	if h.searcher == nil {
		return nil, unifiedSearchResponse{
			Success:         false,
			Message:         "Search service not configured",
			Query:           input.Query,
			Results:         []unifiedSearchResult{},
			Total:           0,
			SourcesSearched: []string{},
		}, nil
	}

	limit := int64(input.Limit)
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// Map user-facing type names to index UIDs
	var indexes []string
	if len(input.Types) > 0 {
		typeMap := map[string]string{
			"document":     search.IndexDocuments,
			"git_template": search.IndexGitTemplates,
			"zim_archive":  search.IndexZimArchives,
		}
		for _, t := range input.Types {
			if idx, ok := typeMap[t]; ok {
				indexes = append(indexes, idx)
			}
		}
	}

	// Non-admin users can only see their own documents and public documents.
	var indexFilters map[string]string
	if user, _ := authmiddleware.UserFromContext(ctx); user != nil && !user.IsAdmin {
		indexFilters = map[string]string{
			search.IndexDocuments: fmt.Sprintf("(user_id = %d OR is_public = true)", user.ID),
		}
	}

	// Determine which indexes will be queried (matches FederatedSearch default).
	queriedIndexes := indexes
	if len(queriedIndexes) == 0 {
		queriedIndexes = []string{search.IndexDocuments, search.IndexZimArchives, search.IndexGitTemplates}
	}
	sourcesSearched := make([]string, 0, len(queriedIndexes))
	for _, idx := range queriedIndexes {
		sourcesSearched = append(sourcesSearched, indexToSource(idx))
	}

	start := time.Now()

	resp, err := h.searcher.FederatedSearch(ctx, search.FederatedSearchParams{
		Query:        input.Query,
		Indexes:      indexes,
		Limit:        limit,
		Offset:       int64(input.Offset),
		IndexFilters: indexFilters,
	})
	if err != nil {
		return nil, unifiedSearchResponse{}, fmt.Errorf("unified search: %w", err)
	}

	processingMs := int(time.Since(start).Milliseconds())

	// Build result list from federated hits.
	normalized := search.NormalizeFederatedHits(resp.Hits)
	results := make([]unifiedSearchResult, 0, len(normalized))
	for _, sr := range normalized {
		results = append(results, unifiedSearchResult{
			Source:      indexToSource(sr.Source),
			UUID:        sr.UUID,
			Title:       sr.Title,
			Description: sr.Description,
			Score:       sr.Score,
		})
	}

	return nil, unifiedSearchResponse{
		Success:          true,
		Query:            input.Query,
		Results:          results,
		Total:            len(results),
		SourcesSearched:  sourcesSearched,
		ProcessingTimeMs: processingMs,
	}, nil
}

// indexToSource maps a Meilisearch index UID to a user-facing source name.
func indexToSource(indexUID string) string {
	switch indexUID {
	case search.IndexDocuments:
		return "document"
	case search.IndexGitTemplates:
		return "git_template"
	case search.IndexZimArchives:
		return "zim_archive"
	default:
		return indexUID
	}
}
