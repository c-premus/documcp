package api

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"git.999.haus/chris/DocuMCP-go/internal/search"
)

// SearchHandler handles REST API endpoints for search.
type SearchHandler struct {
	searcher *search.Searcher
	logger   *slog.Logger
}

// NewSearchHandler creates a new SearchHandler.
func NewSearchHandler(searcher *search.Searcher, logger *slog.Logger) *SearchHandler {
	return &SearchHandler{searcher: searcher, logger: logger}
}

// Search handles GET /api/search — full-text search across documents.
func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		errorResponse(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	limit, _ := strconv.ParseInt(r.URL.Query().Get("limit"), 10, 64)
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	offset, _ := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)

	var filters []string
	if ft := r.URL.Query().Get("file_type"); ft != "" {
		filters = append(filters, "file_type = \""+ft+"\"")
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
		"data":               results,
		"query":              query,
		"total":              resp.EstimatedTotalHits,
		"processing_time_ms": resp.ProcessingTimeMs,
		"limit":              limit,
		"offset":             offset,
	})
}

// FederatedSearch handles GET /api/search/unified — search across all indexes.
func (h *SearchHandler) FederatedSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		errorResponse(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	limit, _ := strconv.ParseInt(r.URL.Query().Get("limit"), 10, 64)
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	offset, _ := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)

	// Parse optional source types filter.
	var indexes []string
	if types := r.URL.Query().Get("types"); types != "" {
		for _, t := range strings.Split(types, ",") {
			switch strings.TrimSpace(t) {
			case "document":
				indexes = append(indexes, search.IndexDocuments)
			case "zim_archive":
				indexes = append(indexes, search.IndexZimArchives)
			case "confluence_space":
				indexes = append(indexes, search.IndexConfluenceSpaces)
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
		"data":               results,
		"query":              query,
		"total":              resp.EstimatedTotalHits,
		"processing_time_ms": resp.ProcessingTimeMs,
		"limit":              limit,
		"offset":             offset,
	})
}
