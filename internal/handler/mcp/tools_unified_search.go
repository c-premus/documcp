package mcphandler

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

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
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input dto.UnifiedSearchInput,
) (*mcp.CallToolResult, unifiedSearchResponse, error) {
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
			"document":         search.IndexDocuments,
			"git_template":     search.IndexGitTemplates,
			"zim_archive":      search.IndexZimArchives,
			"confluence_space": search.IndexConfluenceSpaces,
		}
		for _, t := range input.Types {
			if idx, ok := typeMap[t]; ok {
				indexes = append(indexes, idx)
			}
		}
	}

	start := time.Now()

	resp, err := h.searcher.FederatedSearch(ctx, search.FederatedSearchParams{
		Query:   input.Query,
		Indexes: indexes,
		Limit:   limit,
		Offset:  int64(input.Offset),
	})
	if err != nil {
		return nil, unifiedSearchResponse{}, fmt.Errorf("unified search: %w", err)
	}

	processingMs := int(time.Since(start).Milliseconds())

	// Build result list from federated hits.
	results := make([]unifiedSearchResult, 0, len(resp.Hits))
	sourcesMap := make(map[string]bool)

	for _, hit := range resp.Hits {
		var m map[string]any
		if err := hit.DecodeInto(&m); err != nil {
			continue
		}

		// Determine source from the federation index.
		source := ""
		if idx, ok := m["_federation"].(map[string]any); ok {
			if idxUID, ok := idx["indexUid"].(string); ok {
				source = indexToSource(idxUID)
			}
		}
		if source == "" {
			// Fallback: check known fields.
			if _, ok := m["file_type"]; ok {
				source = "document"
			} else if _, ok := m["article_count"]; ok {
				source = "zim_archive"
			} else if _, ok := m["readme_content"]; ok {
				source = "git_template"
			} else if _, ok := m["key"]; ok {
				source = "confluence_space"
			}
		}

		sourcesMap[source] = true

		result := unifiedSearchResult{
			Source: source,
		}
		if v, ok := m["uuid"].(string); ok {
			result.UUID = v
		}
		if v, ok := m["title"].(string); ok {
			result.Title = v
		} else if v, ok := m["name"].(string); ok {
			result.Title = v
		}
		if v, ok := m["description"].(string); ok {
			result.Description = v
		}
		if v, ok := m["_rankingScore"].(float64); ok {
			result.Score = v
		}

		results = append(results, result)
	}

	sources := make([]string, 0, len(sourcesMap))
	for s := range sourcesMap {
		sources = append(sources, s)
	}

	return nil, unifiedSearchResponse{
		Success:          true,
		Query:            input.Query,
		Results:          results,
		Total:            len(results),
		SourcesSearched:  sources,
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
	case search.IndexConfluenceSpaces:
		return "confluence_space"
	default:
		return indexUID
	}
}
