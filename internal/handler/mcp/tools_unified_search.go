package mcphandler

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	authmiddleware "github.com/c-premus/documcp/internal/auth/middleware"
	authscope "github.com/c-premus/documcp/internal/auth/scope"
	"github.com/c-premus/documcp/internal/client/kiwix"
	"github.com/c-premus/documcp/internal/dto"
	"github.com/c-premus/documcp/internal/model"
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

	limit := int64(clampPagination(input.Limit, 20, 100))

	// Build type filter set for quick lookup.
	typeSet := make(map[string]bool, len(input.Types))
	for _, t := range input.Types {
		typeSet[t] = true
	}
	noFilter := len(typeSet) == 0 // empty = search all

	// Map user-facing type names to search index UIDs.
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

	// Restrict document visibility based on authentication context.
	// M2M tokens (no user) see only public documents; non-admin users see own + public.
	var fedUserID *int64
	var fedIsPublic *bool
	var fedIsAdmin bool
	user, _ := authmiddleware.UserFromContext(ctx)
	switch {
	case user == nil:
		pub := true
		fedIsPublic = &pub
	case user.IsAdmin:
		fedIsAdmin = true
	default:
		fedUserID = &user.ID
	}

	// Determine which indexes will be queried.
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
	// Skip FTS entirely if only zim_article is requested.
	skipFTS := !noFilter && len(typeSet) == 1 && typeSet["zim_article"]

	start := time.Now()

	type ftsResult struct {
		results []unifiedSearchResult
		err     error
	}
	type kiwixResult struct {
		results  []unifiedSearchResult
		searched []string
	}

	ftsCh := make(chan ftsResult, 1)
	kiwixCh := make(chan kiwixResult, 1)

	// Launch FTS search.
	go func() {
		if skipFTS {
			ftsCh <- ftsResult{}
			return
		}
		resp, err := h.searcher.FederatedSearch(ctx, search.FederatedSearchParams{
			Query:    input.Query,
			Indexes:  indexes,
			Limit:    limit,
			Offset:   int64(input.Offset),
			UserID:   fedUserID,
			IsPublic: fedIsPublic,
			IsAdmin:  fedIsAdmin,
		})
		if err != nil {
			ftsCh <- ftsResult{err: err}
			return
		}
		results := make([]unifiedSearchResult, 0, len(resp.Hits))
		for _, sr := range resp.Hits {
			results = append(results, unifiedSearchResult{
				Source:      indexToSource(sr.Source),
				UUID:        sr.UUID,
				Title:       sr.Title,
				Description: sr.Description,
				Score:       sr.Score,
			})
		}
		ftsCh <- ftsResult{results: results}
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

	mr := <-ftsCh
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
	if skipFTS {
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

	generalArchives, devdocsArchives, err := h.selectArchives(ctx, query)
	if err != nil {
		h.logger.Warn("failed to select archives for federated search", "error", err)
		return nil, nil
	}
	if len(generalArchives) == 0 && len(devdocsArchives) == 0 {
		return nil, nil
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

	// Sort general archives: fulltext-capable first, then by article count.
	sort.SliceStable(generalArchives, func(i, j int) bool {
		iFT := ftMap[generalArchives[i].Name]
		jFT := ftMap[generalArchives[j].Name]
		if iFT != jFT {
			return iFT
		}
		return false // preserve DB order (article_count DESC)
	})

	// Cap general archives at max.
	if len(generalArchives) > h.federatedMaxArchives {
		generalArchives = generalArchives[:h.federatedMaxArchives]
	}

	// Cap DevDocs archives at the same limit as general archives. With
	// hundreds of DevDocs archives, uncapped fan-out overwhelms Kiwix Serve
	// with concurrent requests that all timeout.
	if len(devdocsArchives) > h.federatedMaxArchives {
		devdocsArchives = devdocsArchives[:h.federatedMaxArchives]
	}

	// Merge general + DevDocs into a single fan-out list.
	archives := make([]model.ZimArchive, 0, len(generalArchives)+len(devdocsArchives))
	archives = append(archives, generalArchives...)
	archives = append(archives, devdocsArchives...)

	// Build query tokens for suggest fallback. The suggest endpoint does
	// title matching which works poorly with verbose multi-word queries
	// like "os/exec Package exec import". We try individual tokens so
	// that "os/exec" can match the article title "os/exec".
	queryTokens := strings.Fields(query)

	// Create a deadline for the entire fan-out.
	fanoutCtx, cancel := context.WithTimeout(ctx, h.federatedSearchTimeout)
	defer cancel()

	type archiveResult struct {
		archiveName string
		results     []kiwix.SearchResult
	}

	resultsCh := make(chan archiveResult, len(archives))
	var wg sync.WaitGroup

	// Limit concurrent Kiwix requests to avoid overwhelming the server.
	// Matches MaxIdleConnsPerHost on the Kiwix HTTP transport so all
	// goroutines can reuse persistent connections without head-of-line blocking.
	sem := make(chan struct{}, 10)

	for i := range archives {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			searchType := "suggest"
			if ftMap[name] {
				searchType = "fulltext"
			}

			if searchType == "fulltext" {
				// Fulltext search handles multi-word queries natively.
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
				return
			}

			// Suggest (title matching): try the full query first, then
			// fall back to individual tokens until we find results.
			results, searchErr := kiwixClient.Search(fanoutCtx, name, query, "suggest", perArchiveLimit)
			if searchErr == nil && len(results) > 0 {
				resultsCh <- archiveResult{archiveName: name, results: results}
				return
			}
			for _, token := range queryTokens {
				if len(token) < 2 {
					continue
				}
				results, searchErr = kiwixClient.Search(fanoutCtx, name, token, "suggest", perArchiveLimit)
				if searchErr != nil {
					continue
				}
				if len(results) > 0 {
					resultsCh <- archiveResult{archiveName: name, results: results}
					return
				}
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
			// Places Kiwix results below high-confidence FTS results
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

// selectArchives returns the list of ZIM archives to fan out to. It uses FTS
// to find topically relevant archives and falls back to the full searchable
// list when FTS returns no hits. DevDocs archives are separated out and
// returned in a second slice so they can be fanned out with their own budget
// (they are small and highly relevant for programming queries).
func (h *Handler) selectArchives(ctx context.Context, query string) (general, devdocs []model.ZimArchive, err error) {
	archives, err := h.zimArchiveRepo.ListSearchable(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("listing searchable archives: %w", err)
	}
	if len(archives) == 0 {
		return nil, nil, nil
	}

	// Split DevDocs archives from general archives. DevDocs are small
	// (<1000 articles each) and get their own fan-out budget so they
	// don't compete with large archives for the federatedMaxArchives cap.
	var generalArchives, devdocsArchives []model.ZimArchive
	for i := range archives {
		if archives[i].Category.Valid && archives[i].Category.String == "devdocs" {
			devdocsArchives = append(devdocsArchives, archives[i])
		} else {
			generalArchives = append(generalArchives, archives[i])
		}
	}

	// Use FTS to find archives relevant to the query (by title, description,
	// tags). Request enough results to cover both general and DevDocs buckets.
	// This narrows the fan-out to only topically relevant archives instead of
	// blindly picking the largest ones.
	var selectedNames map[string]bool
	if h.searcher != nil {
		resp, searchErr := h.searcher.Search(ctx, search.SearchParams{
			Query:    query,
			IndexUID: search.IndexZimArchives,
			Limit:    int64(h.federatedMaxArchives * 3), // extra headroom for category split
		})
		if searchErr != nil {
			h.logger.Warn("search archive selection failed, falling back to DB", "error", searchErr)
		} else if resp != nil && len(resp.Hits) > 0 {
			selectedNames = make(map[string]bool, len(resp.Hits))
			for _, hit := range resp.Hits {
				if name := search.ExtraString(hit.Extra, "name"); name != "" {
					selectedNames[name] = true
				}
			}
		}
	}

	// Filter both general and DevDocs archives to FTS-selected ones when
	// available. This prevents hundreds of irrelevant DevDocs archives from
	// being fanned out.
	if len(selectedNames) > 0 {
		filtered := generalArchives[:0]
		for i := range generalArchives {
			if selectedNames[generalArchives[i].Name] {
				filtered = append(filtered, generalArchives[i])
			}
		}
		if len(filtered) > 0 {
			generalArchives = filtered
		}

		filteredDD := devdocsArchives[:0]
		for i := range devdocsArchives {
			if selectedNames[devdocsArchives[i].Name] {
				filteredDD = append(filteredDD, devdocsArchives[i])
			}
		}
		if len(filteredDD) > 0 {
			devdocsArchives = filteredDD
		}
		// If no DB archives match (e.g. stale index), fall through to full list.
	}

	return generalArchives, devdocsArchives, nil
}

// indexToSource maps a search index UID to a user-facing source name.
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
