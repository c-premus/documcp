package search

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/meilisearch/meilisearch-go"

	"github.com/c-premus/documcp/internal/observability"
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
	Query         string
	Indexes       []string          // Index UIDs to search. Empty = all 3 indexes.
	Limit         int64
	Offset        int64
	IndexFilters  map[string]string // Additional per-index filters (appended to default soft-delete filter).
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
			IndexGitTemplates,
		}
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}

	queries := make([]*meilisearch.SearchRequest, 0, len(indexes))
	for _, idx := range indexes {
		filter := softDeleteFilter(idx)
		if extra, ok := params.IndexFilters[idx]; ok && extra != "" {
			if filter != "" {
				filter += " AND " + extra
			} else {
				filter = extra
			}
		}
		queries = append(queries, &meilisearch.SearchRequest{
			IndexUID: idx,
			Query:    params.Query,
			Filter:   filter,
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
	case IndexDocuments, IndexGitTemplates:
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

// NormalizeFederatedHits converts federated Meilisearch hits into SearchResults.
// Unlike NormalizeHits, it derives the source per-hit from _federation.indexUid.
func NormalizeFederatedHits(hits meilisearch.Hits) []SearchResult {
	if hits == nil {
		return []SearchResult{}
	}
	results := make([]SearchResult, 0, hits.Len())
	for _, hit := range hits {
		var m map[string]any
		if err := hit.DecodeInto(&m); err != nil {
			continue
		}

		// Determine source from _federation metadata.
		source := ""
		if fed, ok := m["_federation"].(map[string]any); ok {
			if idx, ok := fed["indexUid"].(string); ok {
				source = idx
			}
		}

		uuid, _ := m["uuid"].(string)
		title, _ := m["title"].(string)
		if title == "" {
			title, _ = m["name"].(string)
		}
		description, _ := m["description"].(string)
		score, _ := m["_rankingScore"].(float64)

		results = append(results, SearchResult{
			UUID:        uuid,
			Title:       title,
			Description: description,
			Source:      source,
			Score:       score,
			Extra:       m,
		})
	}
	return results
}

// ExtraString extracts a string value from a SearchResult.Extra map.
func ExtraString(extra map[string]any, key string) string {
	if v, ok := extra[key].(string); ok {
		return v
	}
	return ""
}

// ExtraFloat64 extracts a float64 value from a SearchResult.Extra map.
func ExtraFloat64(extra map[string]any, key string) float64 {
	if v, ok := extra[key].(float64); ok {
		return v
	}
	return 0
}

// ExtraInt extracts an int value (from JSON float64) from a SearchResult.Extra map.
func ExtraInt(extra map[string]any, key string) int {
	if v, ok := extra[key].(float64); ok {
		return int(v)
	}
	return 0
}

// ExtraStringSlice extracts a string slice from a SearchResult.Extra map.
// JSON arrays unmarshal as []any, so each element is individually type-asserted.
func ExtraStringSlice(extra map[string]any, key string) []string {
	arr, ok := extra[key].([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}
