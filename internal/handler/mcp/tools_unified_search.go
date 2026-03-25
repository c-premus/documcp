package mcphandler

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	authmiddleware "github.com/c-premus/documcp/internal/auth/middleware"
	authscope "github.com/c-premus/documcp/internal/auth/scope"
	"github.com/c-premus/documcp/internal/client/kiwix"
	"github.com/c-premus/documcp/internal/dto"
	"github.com/c-premus/documcp/internal/search"
)

// --- Response types ---

type unifiedSearchResult struct {
	Source      string  `json:"source"`
	UUID        string  `json:"uuid,omitempty"`
	Title       string  `json:"title,omitempty"`
	Description string  `json:"description,omitempty"`
	Score       float64 `json:"score,omitempty"`
	Archive     string  `json:"archive,omitempty"` // ZIM archive name (zim_article results only)
	Path        string  `json:"path,omitempty"`    // Article path for read_zim_article (zim_article results only)
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
			"ZIM Archives, and ZIM article content.\n\n" +
			"Returns results ranked by relevance with a `source` field indicating the content type. " +
			"Use for discovery -- then use type-specific tools (search_documents, search_zim, " +
			"search_git_templates) for deep content search with full filter options.\n\n" +
			"**Sources** (filter with `types` param):\n" +
			"- `document` -- Uploaded documents (PDF, DOCX, XLSX, HTML, Markdown)\n" +
			"- `git_template` -- Git template README and metadata\n" +
			"- `zim_archive` -- ZIM archive metadata (DevDocs, Wikipedia, Stack Exchange)\n" +
			"- `zim_article` -- ZIM archive article content (searched via Kiwix). Results include " +
			"`archive` and `path` fields for follow-up with `read_zim_article`.",
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

	// Build type filter set for quick lookup.
	typeSet := make(map[string]bool, len(input.Types))
	for _, t := range input.Types {
		typeSet[t] = true
	}
	noFilter := len(typeSet) == 0 // empty = search all

	// Map user-facing type names to Meilisearch index UIDs.
	var indexes []string
	if !noFilter {
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

	// Determine which Meilisearch indexes will be queried.
	queriedIndexes := indexes
	if len(queriedIndexes) == 0 {
		queriedIndexes = []string{search.IndexDocuments, search.IndexZimArchives, search.IndexGitTemplates}
	}
	sourcesSearched := make([]string, 0, len(queriedIndexes)+1)
	for _, idx := range queriedIndexes {
		sourcesSearched = append(sourcesSearched, indexToSource(idx))
	}

	// Decide whether to fan out to Kiwix archives.
	wantZimArticles := noFilter || typeSet["zim_article"]
	// Skip Meilisearch entirely if only zim_article is requested.
	skipMeilisearch := !noFilter && len(typeSet) == 1 && typeSet["zim_article"]

	start := time.Now()

	type meiliResult struct {
		results []unifiedSearchResult
		err     error
	}
	type kiwixResult struct {
		results  []unifiedSearchResult
		searched []string
	}

	meiliCh := make(chan meiliResult, 1)
	kiwixCh := make(chan kiwixResult, 1)

	// Launch Meilisearch search.
	go func() {
		if skipMeilisearch {
			meiliCh <- meiliResult{}
			return
		}
		resp, err := h.searcher.FederatedSearch(ctx, search.FederatedSearchParams{
			Query:        input.Query,
			Indexes:      indexes,
			Limit:        limit,
			Offset:       int64(input.Offset),
			IndexFilters: indexFilters,
		})
		if err != nil {
			meiliCh <- meiliResult{err: err}
			return
		}
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
		meiliCh <- meiliResult{results: results}
	}()

	// Launch Kiwix fan-out.
	go func() {
		if !wantZimArticles {
			kiwixCh <- kiwixResult{}
			return
		}
		results, searched := h.searchKiwixArchives(ctx, input.Query, h.federatedPerArchiveLimit)
		kiwixCh <- kiwixResult{results: results, searched: searched}
	}()

	mr := <-meiliCh
	kr := <-kiwixCh

	if mr.err != nil {
		return nil, unifiedSearchResponse{}, fmt.Errorf("unified search: %w", mr.err)
	}

	processingMs := int(time.Since(start).Milliseconds())

	// Merge results.
	allResults := make([]unifiedSearchResult, 0, len(mr.results)+len(kr.results))
	allResults = append(allResults, mr.results...)
	allResults = append(allResults, kr.results...)

	// Sort by score descending.
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Score > allResults[j].Score
	})

	// Truncate to limit.
	if int64(len(allResults)) > limit {
		allResults = allResults[:limit]
	}

	// Update sources searched.
	if len(kr.searched) > 0 {
		sourcesSearched = append(sourcesSearched, "zim_article")
	}
	if skipMeilisearch {
		sourcesSearched = []string{"zim_article"}
	}

	return nil, unifiedSearchResponse{
		Success:          true,
		Query:            input.Query,
		Results:          allResults,
		Total:            len(allResults),
		SourcesSearched:  sourcesSearched,
		ProcessingTimeMs: processingMs,
	}, nil
}

// searchKiwixArchives fans out search queries to all searchable Kiwix archives
// in parallel. Archives with fulltext index use fulltext search; others fall
// back to suggest (title matching). Returns merged results and archive names searched.
func (h *Handler) searchKiwixArchives(ctx context.Context, query string, perArchiveLimit int) (results []unifiedSearchResult, searched []string) {
	if h.kiwixFactory == nil || h.zimArchiveRepo == nil {
		return nil, nil
	}

	kiwixClient, err := h.kiwixFactory.Get(ctx)
	if err != nil {
		h.logger.Warn("kiwix client unavailable for federated search", "error", err)
		return nil, nil
	}

	// Use Meilisearch to find archives relevant to the query (by title,
	// description, tags). This narrows the fan-out to only topically relevant
	// archives instead of blindly picking the largest ones.
	var selectedNames map[string]bool
	if h.searcher != nil {
		resp, searchErr := h.searcher.Search(ctx, search.SearchParams{
			Query:    query,
			IndexUID: search.IndexZimArchives,
			Limit:    int64(h.federatedMaxArchives),
		})
		if searchErr != nil {
			h.logger.Warn("meilisearch archive selection failed, falling back to DB", "error", searchErr)
		} else if resp != nil && len(resp.Hits) > 0 {
			selectedNames = make(map[string]bool, len(resp.Hits))
			for _, hit := range resp.Hits {
				var m map[string]any
				if decodeErr := hit.DecodeInto(&m); decodeErr == nil {
					if name, ok := m["name"].(string); ok && name != "" {
						selectedNames[name] = true
					}
				}
			}
		}
	}

	archives, err := h.zimArchiveRepo.ListSearchable(ctx)
	if err != nil {
		h.logger.Warn("failed to list searchable archives", "error", err)
		return nil, nil
	}
	if len(archives) == 0 {
		return nil, nil
	}

	// Filter to Meilisearch-selected archives when available.
	if len(selectedNames) > 0 {
		filtered := archives[:0]
		for _, a := range archives {
			if selectedNames[a.Name] {
				filtered = append(filtered, a)
			}
		}
		if len(filtered) > 0 {
			archives = filtered
		}
		// If no DB archives match (e.g. stale index), fall through to full list.
	}

	// Fetch catalog to determine fulltext index availability per archive.
	catalog, err := kiwixClient.FetchCatalog(ctx)
	if err != nil {
		h.logger.Warn("failed to fetch kiwix catalog for federated search", "error", err)
		return nil, nil
	}
	ftMap := make(map[string]bool, len(catalog))
	for i := range catalog {
		if catalog[i].HasFulltextIndex {
			ftMap[catalog[i].Name] = true
		}
	}

	// Sort: fulltext-capable first, then by article count (already sorted by article_count DESC from DB).
	sort.SliceStable(archives, func(i, j int) bool {
		iFT := ftMap[archives[i].Name]
		jFT := ftMap[archives[j].Name]
		if iFT != jFT {
			return iFT
		}
		return false // preserve DB order (article_count DESC)
	})

	// Cap at max archives.
	if len(archives) > h.federatedMaxArchives {
		archives = archives[:h.federatedMaxArchives]
	}

	// Create a deadline for the entire fan-out.
	fanoutCtx, cancel := context.WithTimeout(ctx, h.federatedSearchTimeout)
	defer cancel()

	type archiveResult struct {
		archiveName string
		results     []kiwix.SearchResult
	}

	resultsCh := make(chan archiveResult, len(archives))
	var wg sync.WaitGroup

	for i := range archives {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()

			searchType := "suggest"
			if ftMap[name] {
				searchType = "fulltext"
			}

			results, searchErr := kiwixClient.Search(fanoutCtx, name, query, searchType, perArchiveLimit)
			if searchErr != nil {
				h.logger.Debug("kiwix archive search failed",
					"archive", name,
					"error", searchErr,
				)
				return
			}
			if len(results) > 0 {
				resultsCh <- archiveResult{archiveName: name, results: results}
			}
		}(archives[i].Name)
	}

	// Close channel when all goroutines finish.
	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	// Collect results and build unified search entries.
	var unified []unifiedSearchResult
	searchedArchives := make([]string, 0, len(archives))

	for ar := range resultsCh {
		searchedArchives = append(searchedArchives, ar.archiveName)
		for rank, sr := range ar.results {
			// Synthetic score: 0.5 base, decremented by rank position.
			// Places Kiwix results below high-confidence Meilisearch results
			// but above low-confidence ones.
			score := 0.5 - float64(rank)*0.01

			description := sr.Snippet
			if description == "" {
				description = "Article in " + ar.archiveName
			}

			unified = append(unified, unifiedSearchResult{
				Source:      "zim_article",
				Title:       sr.Title,
				Description: description,
				Score:       score,
				Archive:     ar.archiveName,
				Path:        sr.Path,
			})
		}
	}

	return unified, searchedArchives
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
