package search

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/meilisearch/meilisearch-go"

	"git.999.haus/chris/DocuMCP-go/internal/observability"
)

// SearchResult holds a normalized result from any index.
//
//nolint:revive // exported stutter is intentional; renaming would be a breaking change
type SearchResult struct {
	UUID        string  `json:"uuid"`
	Title       string  `json:"title"`
	Description string  `json:"description,omitempty"`
	Source      string  `json:"source"`
	Score       float64 `json:"score,omitempty"`

	// Source-specific fields returned as-is.
	Extra map[string]any `json:"extra,omitempty"`
}

// SearchParams holds parameters for searching a single index.
//
//nolint:revive // exported stutter is intentional; renaming would be a breaking change
type SearchParams struct {
	Query    string
	IndexUID string
	Filters  string
	Limit    int64
	Offset   int64
	Sort     []string
}

// FederatedSearchParams holds parameters for searching across multiple indexes.
type FederatedSearchParams struct {
	Query   string
	Indexes []string // Index UIDs to search. Empty = all 4 indexes.
	Limit   int64
	Offset  int64
}

// Searcher performs search queries against Meilisearch indexes.
type Searcher struct {
	client  *Client
	logger  *slog.Logger
	metrics *observability.Metrics
}

// NewSearcher creates a new Searcher backed by the given Client.
func NewSearcher(client *Client, logger *slog.Logger) *Searcher {
	return &Searcher{client: client, logger: logger}
}

// SetMetrics enables Prometheus latency recording for search operations.
func (s *Searcher) SetMetrics(m *observability.Metrics) {
	s.metrics = m
}

// Search performs a search on a single index.
func (s *Searcher) Search(ctx context.Context, params SearchParams) (*meilisearch.SearchResponse, error) {
	idx := s.client.ms.Index(params.IndexUID)

	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}

	req := &meilisearch.SearchRequest{
		Limit:                 limit,
		Offset:                params.Offset,
		Sort:                  params.Sort,
		Filter:                params.Filters,
		AttributesToHighlight: []string{"title", "description"},
		HighlightPreTag:       "<em>",
		HighlightPostTag:      "</em>",
	}

	start := time.Now()
	resp, err := idx.SearchWithContext(ctx, params.Query, req)
	if err != nil {
		return nil, fmt.Errorf("searching index %q: %w", params.IndexUID, err)
	}
	if s.metrics != nil {
		s.metrics.SearchLatency.WithLabelValues(params.IndexUID).Observe(time.Since(start).Seconds())
	}

	return resp, nil
}

// FederatedSearch searches across multiple indexes in a single request using
// Meilisearch's multi-search federation feature.
func (s *Searcher) FederatedSearch(ctx context.Context, params FederatedSearchParams) (*meilisearch.MultiSearchResponse, error) {
	indexes := params.Indexes
	if len(indexes) == 0 {
		indexes = []string{
			IndexDocuments,
			IndexZimArchives,
			IndexConfluenceSpaces,
			IndexGitTemplates,
		}
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}

	queries := make([]*meilisearch.SearchRequest, 0, len(indexes))
	for _, idx := range indexes {
		queries = append(queries, &meilisearch.SearchRequest{
			IndexUID: idx,
			Query:    params.Query,
			Filter:   softDeleteFilter(idx),
		})
	}

	start := time.Now()
	resp, err := s.client.ms.MultiSearchWithContext(ctx, &meilisearch.MultiSearchRequest{
		Federation: &meilisearch.MultiSearchFederation{
			Limit:  limit,
			Offset: params.Offset,
		},
		Queries: queries,
	})
	if err != nil {
		return nil, fmt.Errorf("federated search: %w", err)
	}
	if s.metrics != nil {
		s.metrics.SearchLatency.WithLabelValues("federated").Observe(time.Since(start).Seconds())
	}

	return resp, nil
}

// softDeleteFilter returns the default filter to exclude soft-deleted records
// for indexes that support it.
func softDeleteFilter(indexUID string) string {
	switch indexUID {
	case IndexDocuments, IndexConfluenceSpaces, IndexGitTemplates:
		return "__soft_deleted = false"
	default:
		return ""
	}
}

// NormalizeHits converts Meilisearch hits from a response into SearchResult
// values with normalized fields.
func NormalizeHits(hits meilisearch.Hits, source string) []SearchResult {
	results := make([]SearchResult, 0, len(hits))
	for _, hit := range hits {
		// Decode each Hit (map[string]json.RawMessage) into map[string]any.
		var m map[string]any
		if err := hit.DecodeInto(&m); err != nil {
			continue
		}

		result := SearchResult{
			Source: source,
			Extra:  m,
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
	return results
}
