package api //nolint:revive // package name matches REST API handler directory convention

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"strings"

	authmiddleware "github.com/c-premus/documcp/internal/auth/middleware"
	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/repository"
	"github.com/c-premus/documcp/internal/search"
)

// documentSearcher performs full-text search queries.
type documentSearcher interface {
	Search(ctx context.Context, params search.SearchParams) (*search.SearchResponse, error)
	FederatedSearch(ctx context.Context, params search.FederatedSearchParams) (*search.FederatedSearchResponse, error)
}

// searchQueryLister retrieves popular search queries.
type searchQueryLister interface {
	PopularQueries(ctx context.Context, limit int) ([]repository.PopularQuery, error)
}

// titleSuggester provides title autocomplete suggestions.
type titleSuggester interface {
	SuggestTitles(ctx context.Context, prefix string, limit int, userID *int64, isAdmin bool) ([]repository.TitleSuggestion, error)
}

// SearchHandler handles REST API endpoints for search.
type SearchHandler struct {
	searcher    documentSearcher
	queryLister searchQueryLister
	suggester   titleSuggester
	logger      *slog.Logger
}

// NewSearchHandler creates a new SearchHandler.
func NewSearchHandler(searcher documentSearcher, queryLister searchQueryLister, suggester titleSuggester, logger *slog.Logger) *SearchHandler {
	return &SearchHandler{
		searcher:    searcher,
		queryLister: queryLister,
		suggester:   suggester,
		logger:      logger,
	}
}

// Search handles GET /api/search — full-text search across documents.
func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		errorResponse(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}
	if len(query) > search.MaxQueryLength {
		errorResponse(w, http.StatusBadRequest, fmt.Sprintf("query must be at most %d characters", search.MaxQueryLength))
		return
	}

	limitInt, offsetInt := parsePagination(r, 20, 100)
	limit, offset := int64(limitInt), int64(offsetInt)

	params := search.SearchParams{
		Query:    query,
		IndexUID: search.IndexDocuments,
		Limit:    limit,
		Offset:   offset,
	}

	// Scope search results by user visibility: admins see all documents,
	// non-admins see only public documents and their own.
	if user, ok := authmiddleware.UserFromContext(r.Context()); ok {
		params.UserID = &user.ID
		params.IsAdmin = user.IsAdmin
	}

	if ft := r.URL.Query().Get("file_type"); ft != "" {
		if !model.ValidFileTypes[ft] {
			errorResponse(w, http.StatusBadRequest, "invalid file_type filter")
			return
		}
		params.FileType = ft
	}

	// tags filter — accepts repeated ?tags=a&tags=b OR a single comma-list
	// ?tags=a,b. Documents must match every tag (AND logic, per dto.mcp).
	if rawTags := r.URL.Query()["tags"]; len(rawTags) > 0 {
		params.Tags = expandTagsParam(rawTags)
	}

	// include_snippets flag-gates the ts_headline highlight pass. Off by
	// default because ts_headline reads full content per hit (LIMIT 100 over
	// 10 MB docs is up to 1 GB scanned). Matches the MCP search_documents
	// contract.
	if r.URL.Query().Get("include_snippets") == "true" {
		params.WithSnippets = true
	}

	resp, err := h.searcher.Search(r.Context(), params)
	if err != nil {
		h.logger.Error("searching documents", "error", err)
		errorResponse(w, http.StatusInternalServerError, "search failed")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": resp.Hits,
		"meta": map[string]any{
			"query":              query,
			"total":              resp.EstimatedTotal,
			"processing_time_ms": resp.ProcessingTimeMs,
			"limit":              limit,
			"offset":             offset,
		},
	})
}

// unifiedSearchHint mirrors MCP's discovery-only hint: paginated deep search
// lives on the type-specific endpoints, not here.
const unifiedSearchHint = "unified search returns a single page of top-ranked " +
	"results. For paginated deep search use /api/search (documents), " +
	"/api/zim-archives/{uuid}/search, or /api/git-templates/{uuid}/files/search " +
	"with the same query."

// indexUIDToSource maps internal index UIDs to the user-facing `source` enum
// returned on each result and used as `totals` keys. Keeps REST aligned with
// MCP's unified_search naming.
var indexUIDToSource = map[string]string{
	search.IndexDocuments:    "document",
	search.IndexGitTemplates: "git_template",
	search.IndexZimArchives:  "zim_archive",
}

// FederatedSearch handles GET /api/search/unified — discovery-only search
// across all FTS sources. Aligns with the MCP `unified_search` tool
// (audit C1): pagination is unsupported (callers requesting `offset` get a
// 400), the response exposes per-source pre-limit totals and a hint that
// names the paginated endpoints for deep search.
func (h *SearchHandler) FederatedSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		errorResponse(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}
	if len(query) > search.MaxQueryLength {
		errorResponse(w, http.StatusBadRequest, fmt.Sprintf("query must be at most %d characters", search.MaxQueryLength))
		return
	}
	if r.URL.Query().Has("offset") {
		errorResponse(w, http.StatusBadRequest,
			"offset is not supported on /api/search/unified — use /api/search, /api/zim-archives/{uuid}/search, or /api/git-templates/{uuid}/files/search for paginated deep search")
		return
	}

	limitInt, _ := parsePagination(r, 20, 100)
	limit := int64(limitInt)

	indexes, sourcesSearched := parseFederatedTypes(r.URL.Query()["types"])

	fedParams := search.FederatedSearchParams{
		Query:   query,
		Indexes: indexes,
		Limit:   limit,
	}
	if user, ok := authmiddleware.UserFromContext(r.Context()); ok {
		fedParams.UserID = &user.ID
		fedParams.IsAdmin = user.IsAdmin
	}

	resp, err := h.searcher.FederatedSearch(r.Context(), fedParams)
	if err != nil {
		h.logger.Error("federated search", "error", err)
		errorResponse(w, http.StatusInternalServerError, "search failed")
		return
	}

	// Build per-source totals keyed by the user-facing source names.
	// sourcesSearched already lists exactly the FTS sources that were queried;
	// seed each key so zero-match sources still appear in the map.
	totals := make(map[string]int64, len(sourcesSearched))
	for _, src := range sourcesSearched {
		totals[src] = 0
	}
	for indexUID, count := range resp.SourceTotals {
		if src, ok := indexUIDToSource[indexUID]; ok {
			totals[src] = count
		}
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"query":              query,
		"results":            resp.Hits,
		"returned":           len(resp.Hits),
		"totals":             totals,
		"sources_searched":   sourcesSearched,
		"processing_time_ms": resp.ProcessingTimeMs,
		"hint":               unifiedSearchHint,
	})
}

// parseFederatedTypes validates the `types` query param and returns the
// matching search-index UIDs plus the user-facing source names that will be
// searched. An empty types param means "all three FTS sources".
func parseFederatedTypes(rawTypes []string) (indexes, sourcesSearched []string) {
	if len(rawTypes) == 0 {
		return nil, []string{"document", "git_template", "zim_archive"}
	}

	// Accept either repeated ?types=document&types=git_template or a single
	// comma-list ?types=document,git_template — matches expandTagsParam.
	seen := make(map[string]bool, 3)
	typeMap := map[string]string{
		"document":     search.IndexDocuments,
		"git_template": search.IndexGitTemplates,
		"zim_archive":  search.IndexZimArchives,
	}
	for _, raw := range rawTypes {
		for token := range strings.SplitSeq(raw, ",") {
			t := strings.TrimSpace(token)
			idx, ok := typeMap[t]
			if !ok || seen[t] {
				continue
			}
			seen[t] = true
			indexes = append(indexes, idx)
			sourcesSearched = append(sourcesSearched, t)
		}
	}
	return indexes, sourcesSearched
}

// Popular handles GET /api/search/popular — returns popular search queries.
func (h *SearchHandler) Popular(w http.ResponseWriter, r *http.Request) {
	limit, _ := parsePagination(r, 10, 50)

	queries, err := h.queryLister.PopularQueries(r.Context(), limit)
	if err != nil {
		h.logger.Error("fetching popular queries", "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to fetch popular queries")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{"data": queries})
}

// autocompleteResult is the JSON representation of a title suggestion.
type autocompleteResult struct {
	UUID             string `json:"uuid"`
	Title            string `json:"title"`
	HighlightedTitle string `json:"highlighted_title"`
}

// Autocomplete handles GET /api/search/autocomplete — returns title suggestions.
func (h *SearchHandler) Autocomplete(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if len(query) < 2 || len(query) > 100 {
		errorResponse(w, http.StatusBadRequest, "query parameter must be between 2 and 100 characters")
		return
	}

	limit, _ := parsePagination(r, 5, 10)

	// Apply caller-scoped visibility: admins see all non-deleted docs;
	// authenticated users see public + own; unauth sees public-only.
	var userID *int64
	var isAdmin bool
	if user, ok := authmiddleware.UserFromContext(r.Context()); ok {
		userID = &user.ID
		isAdmin = user.IsAdmin
	}

	suggestions, err := h.suggester.SuggestTitles(r.Context(), query, limit, userID, isAdmin)
	if err != nil {
		h.logger.Error("fetching title suggestions", "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to fetch suggestions")
		return
	}

	results := make([]autocompleteResult, len(suggestions))
	for i, s := range suggestions {
		results[i] = autocompleteResult{
			UUID:             s.UUID,
			Title:            s.Title,
			HighlightedTitle: highlightPrefix(s.Title, query),
		}
	}

	jsonResponse(w, http.StatusOK, map[string]any{"data": results})
}

// highlightPrefix wraps the matched prefix portion of title in <em> tags.
// The match is case-insensitive. Both segments are HTML-escaped to prevent XSS.
// Uses rune-based slicing to correctly handle multi-byte UTF-8 characters.
func highlightPrefix(title, prefix string) string {
	prefixRunes := []rune(prefix)
	titleRunes := []rune(title)
	if len(prefixRunes) == 0 || len(prefixRunes) > len(titleRunes) {
		return html.EscapeString(title)
	}
	titlePrefix := string(titleRunes[:len(prefixRunes)])
	if !strings.EqualFold(titlePrefix, prefix) {
		return html.EscapeString(title)
	}
	titleRest := string(titleRunes[len(prefixRunes):])
	return "<em>" + html.EscapeString(titlePrefix) + "</em>" + html.EscapeString(titleRest)
}
