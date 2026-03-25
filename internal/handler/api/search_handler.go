package api //nolint:revive // package name matches REST API handler directory convention

import (
	"context"
	"html"
	"log/slog"
	"net/http"
	"strings"

	"github.com/c-premus/documcp/internal/repository"
	"github.com/c-premus/documcp/internal/search"
)

// searchQueryLister retrieves popular search queries.
type searchQueryLister interface {
	PopularQueries(ctx context.Context, limit int) ([]repository.PopularQuery, error)
}

// titleSuggester provides title autocomplete suggestions.
type titleSuggester interface {
	SuggestTitles(ctx context.Context, prefix string, limit int) ([]repository.TitleSuggestion, error)
}

// SearchHandler handles REST API endpoints for search.
type SearchHandler struct {
	searcher    *search.Searcher
	queryLister searchQueryLister
	suggester   titleSuggester
	logger      *slog.Logger
}

// NewSearchHandler creates a new SearchHandler.
func NewSearchHandler(searcher *search.Searcher, queryLister searchQueryLister, suggester titleSuggester, logger *slog.Logger) *SearchHandler {
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

	limitInt, offsetInt := parsePagination(r, 20, 100)
	limit, offset := int64(limitInt), int64(offsetInt)

	var filters []string
	if ft := r.URL.Query().Get("file_type"); ft != "" {
		switch ft {
		case "pdf", "docx", "xlsx", "html", "markdown":
			filters = append(filters, "file_type = \""+ft+"\"")
		default:
			errorResponse(w, http.StatusBadRequest, "invalid file_type filter")
			return
		}
	}
	// Always exclude soft-deleted documents.
	filters = append(filters, "__soft_deleted = false")

	params := search.SearchParams{
		Query:    query,
		IndexUID: search.IndexDocuments,
		Filters:  strings.Join(filters, " AND "),
		Limit:    limit,
		Offset:   offset,
	}

	resp, err := h.searcher.Search(r.Context(), params)
	if err != nil {
		h.logger.Error("searching documents", "error", err)
		errorResponse(w, http.StatusInternalServerError, "search failed")
		return
	}

	results := search.NormalizeHits(resp.Hits, "document")

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": results,
		"meta": map[string]any{
			"query":              query,
			"total":              resp.EstimatedTotalHits,
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

	resp, err := h.searcher.FederatedSearch(r.Context(), search.FederatedSearchParams{
		Query:   query,
		Indexes: indexes,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		h.logger.Error("federated search", "error", err)
		errorResponse(w, http.StatusInternalServerError, "search failed")
		return
	}

	results := search.NormalizeHits(resp.Hits, "federated")

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": results,
		"meta": map[string]any{
			"query":              query,
			"total":              resp.EstimatedTotalHits,
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

	suggestions, err := h.suggester.SuggestTitles(r.Context(), query, limit)
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
func highlightPrefix(title, prefix string) string {
	if prefix == "" || len(prefix) > len(title) {
		return html.EscapeString(title)
	}
	if !strings.EqualFold(title[:len(prefix)], prefix) {
		return html.EscapeString(title)
	}
	return "<em>" + html.EscapeString(title[:len(prefix)]) + "</em>" + html.EscapeString(title[len(prefix):])
}
