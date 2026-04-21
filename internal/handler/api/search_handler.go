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

// FederatedSearch handles GET /api/search/unified — search across all indexes.
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

	limitInt, offsetInt := parsePagination(r, 20, 100)
	limit, offset := int64(limitInt), int64(offsetInt)

	// Parse optional source types filter.
	var indexes []string
	if types := r.URL.Query().Get("types"); types != "" {
		for t := range strings.SplitSeq(types, ",") {
			switch strings.TrimSpace(t) {
			case "document":
				indexes = append(indexes, search.IndexDocuments)
			case "zim_archive":
				indexes = append(indexes, search.IndexZimArchives)
			case "git_template":
				indexes = append(indexes, search.IndexGitTemplates)
			}
		}
	}

	fedParams := search.FederatedSearchParams{
		Query:   query,
		Indexes: indexes,
		Limit:   limit,
		Offset:  offset,
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
