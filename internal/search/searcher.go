// Package search provides PostgreSQL full-text search across documents,
// ZIM archives, and Git templates using tsvector/tsquery and pg_trgm.
package search

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/c-premus/documcp/internal/observability"
)

// Index UIDs identify which table to search.
const (
	IndexDocuments    = "documents"
	IndexZimArchives  = "zim_archives"
	IndexGitTemplates = "git_templates"
)

// tsConfig is the PostgreSQL text search configuration used for all FTS queries.
const tsConfig = "documcp_english"

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

// SearchResponse holds results from a single-index search.
//
//nolint:revive // exported stutter is intentional
type SearchResponse struct {
	Hits             []SearchResult
	EstimatedTotal   int64
	ProcessingTimeMs int64
}

// FederatedSearchResponse holds results from a multi-index search.
type FederatedSearchResponse struct {
	Hits           []SearchResult
	EstimatedTotal int64
	// SourceTotals carries the pre-LIMIT match count per source index UID
	// (e.g. IndexDocuments, IndexGitTemplates, IndexZimArchives). Sources
	// with zero matches are absent — callers that need them to appear
	// should seed their result map with the queried index UIDs.
	SourceTotals     map[string]int64
	ProcessingTimeMs int64
}

// sqlSourceToIndex maps the SQL `source` column value emitted by each
// federated union arm to its index UID. The SQL uses the MCP-facing
// singular form; SourceTotals keys are normalized to the plural index
// UID so callers can look up by the same Indexes they passed in
// FederatedSearchParams.
var sqlSourceToIndex = map[string]string{
	"document":     IndexDocuments,
	"zim_archive":  IndexZimArchives,
	"git_template": IndexGitTemplates,
}

// SearchParams holds parameters for searching a single index.
//
//nolint:revive // exported stutter is intentional
type SearchParams struct {
	Query    string
	IndexUID string
	Limit    int64
	Offset   int64
	Sort     []string

	// Structured filters
	FileType string   // documents only
	Tags     []string // documents only
	Category string   // git_templates / zim_archives
	Language string   // zim_archives only
	UserID   *int64   // access control
	IsPublic *bool    // access control
	IsAdmin  bool     // skip visibility filter
}

// FederatedSearchParams holds parameters for searching across multiple indexes.
type FederatedSearchParams struct {
	Query    string
	Indexes  []string // Index UIDs to search. Empty = all 3 indexes.
	Limit    int64
	Offset   int64
	UserID   *int64
	IsPublic *bool
	IsAdmin  bool
}

// Searcher performs search queries against PostgreSQL full-text search indexes.
type Searcher struct {
	db      *pgxpool.Pool
	logger  *slog.Logger
	metrics *observability.Metrics
}

// NewSearcher creates a new Searcher backed by the given connection pool.
func NewSearcher(db *pgxpool.Pool, logger *slog.Logger) *Searcher {
	return &Searcher{db: db, logger: logger}
}

// SetMetrics enables Prometheus latency recording for search operations.
func (s *Searcher) SetMetrics(m *observability.Metrics) {
	s.metrics = m
}

// Search performs a full-text search on a single index (table).
func (s *Searcher) Search(ctx context.Context, params SearchParams) (*SearchResponse, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}

	expanded := ExpandSynonyms(params.Query)

	start := time.Now()

	var resp *SearchResponse
	var err error

	switch params.IndexUID {
	case IndexDocuments:
		resp, err = s.searchDocuments(ctx, expanded, params, limit)
	case IndexZimArchives:
		resp, err = s.searchZimArchives(ctx, expanded, params, limit)
	case IndexGitTemplates:
		resp, err = s.searchGitTemplates(ctx, expanded, params, limit)
	default:
		return nil, fmt.Errorf("unknown index %q", params.IndexUID)
	}

	if err != nil {
		return nil, err
	}

	resp.ProcessingTimeMs = time.Since(start).Milliseconds()

	if s.metrics != nil {
		s.metrics.SearchLatency.WithLabelValues(params.IndexUID).Observe(time.Since(start).Seconds())
	}

	return resp, nil
}

// FederatedSearch searches across multiple indexes in a single request.
func (s *Searcher) FederatedSearch(ctx context.Context, params FederatedSearchParams) (*FederatedSearchResponse, error) {
	indexes := params.Indexes
	if len(indexes) == 0 {
		indexes = []string{IndexDocuments, IndexZimArchives, IndexGitTemplates}
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}

	expanded := ExpandSynonyms(params.Query)
	start := time.Now()

	// Build UNION ALL query across selected indexes.
	var unions []string
	args := []any{expanded} // $1 = query
	argIdx := 2

	for _, idx := range indexes {
		switch idx {
		case IndexDocuments:
			clause, newArgs, nextIdx := s.documentUnionClause(expanded, params, argIdx)
			unions = append(unions, clause)
			args = append(args, newArgs...)
			argIdx = nextIdx
		case IndexZimArchives:
			unions = append(unions, zimArchiveUnionClause())
		case IndexGitTemplates:
			unions = append(unions, gitTemplateUnionClause())
		}
	}

	if len(unions) == 0 {
		return &FederatedSearchResponse{Hits: []SearchResult{}}, nil
	}

	sql := fmt.Sprintf(`
		SELECT uuid, title, description, source, rank, extra,
		       COUNT(*) OVER () AS total,
		       COUNT(*) OVER (PARTITION BY source) AS source_total
		FROM (%s) federated
		ORDER BY rank DESC
		LIMIT $%d OFFSET $%d`,
		strings.Join(unions, " UNION ALL "),
		argIdx, argIdx+1,
	)
	args = append(args, limit, params.Offset)

	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("federated search: %w", err)
	}
	defer rows.Close()

	var hits []SearchResult
	var total int64
	sourceTotals := map[string]int64{}
	for rows.Next() {
		var r SearchResult
		var extra map[string]any
		var sourceTotal int64
		if err := rows.Scan(&r.UUID, &r.Title, &r.Description, &r.Source, &r.Score, &extra, &total, &sourceTotal); err != nil {
			return nil, fmt.Errorf("scanning federated result: %w", err)
		}
		r.Extra = extra
		hits = append(hits, r)
		// The SQL source column emits the MCP-facing singular name
		// ('document', 'zim_archive', 'git_template') while the rest of
		// this package keys caller-facing maps by the plural index UID
		// (IndexDocuments="documents", etc.). Translate so SourceTotals
		// keys match the Indexes the caller passed in FederatedSearchParams.
		if idx, ok := sqlSourceToIndex[r.Source]; ok {
			if _, seen := sourceTotals[idx]; !seen {
				sourceTotals[idx] = sourceTotal
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating federated results: %w", err)
	}

	if hits == nil {
		hits = []SearchResult{}
	}

	elapsed := time.Since(start)
	if s.metrics != nil {
		s.metrics.SearchLatency.WithLabelValues("federated").Observe(elapsed.Seconds())
	}

	return &FederatedSearchResponse{
		Hits:             hits,
		EstimatedTotal:   total,
		SourceTotals:     sourceTotals,
		ProcessingTimeMs: elapsed.Milliseconds(),
	}, nil
}

// searchDocuments searches the documents table with FTS, falling back to trigram.
func (s *Searcher) searchDocuments(ctx context.Context, query string, params SearchParams, limit int64) (*SearchResponse, error) {
	where, args, argIdx := s.buildDocumentFilters(params, 2) // $1 = query

	sql := fmt.Sprintf(`
		SELECT d.id, d.uuid, d.title, COALESCE(d.description, '') AS description,
			   ts_rank_cd(d.search_vector, websearch_to_tsquery('%s', $1)) AS rank,
			   jsonb_build_object(
				   'file_type', d.file_type,
				   'word_count', COALESCE(d.word_count, 0),
				   'content', COALESCE(d.content, ''),
				   'is_public', d.is_public,
				   'status', d.status,
				   'created_at', COALESCE(to_char(d.created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), ''),
				   'updated_at', COALESCE(to_char(d.updated_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), '')
			   ) AS extra,
			   COUNT(*) OVER () AS total
		FROM documents d
		WHERE d.search_vector @@ websearch_to_tsquery('%s', $1)
		  AND d.deleted_at IS NULL
		  %s
		ORDER BY rank DESC
		LIMIT $%d OFFSET $%d`,
		tsConfig, tsConfig, where, argIdx, argIdx+1,
	)

	allArgs := append([]any{query}, args...)
	allArgs = append(allArgs, limit, params.Offset)
	rows, err := s.db.Query(ctx, sql, allArgs...)
	if err != nil {
		return nil, fmt.Errorf("searching documents: %w", err)
	}
	hits, ids, total, err := scanDocumentResults(rows)
	if err != nil {
		return nil, err
	}

	// Fallback to trigram similarity if FTS returned nothing.
	if len(hits) == 0 && params.Query != "" {
		hits, ids, err = s.trigramFallbackDocuments(ctx, params, limit)
		if err != nil {
			s.logger.Warn("trigram fallback failed", "error", err)
		}
		total = int64(len(hits))
	}

	if err := s.attachTags(ctx, hits, ids); err != nil {
		s.logger.Warn("fetching tags for search hits", "error", err)
	}

	if hits == nil {
		hits = []SearchResult{}
	}

	return &SearchResponse{
		Hits:           hits,
		EstimatedTotal: total,
	}, nil
}

// searchZimArchives searches the zim_archives table with FTS.
func (s *Searcher) searchZimArchives(ctx context.Context, query string, params SearchParams, limit int64) (*SearchResponse, error) {
	var whereClauses []string
	var args []any
	args = append(args, query) // $1
	argIdx := 2

	if params.Language != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("za.language = $%d", argIdx))
		args = append(args, params.Language)
		argIdx++
	}
	if params.Category != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("za.category = $%d", argIdx))
		args = append(args, params.Category)
		argIdx++
	}
	_ = argIdx

	where := ""
	if len(whereClauses) > 0 {
		where = "AND " + strings.Join(whereClauses, " AND ")
	}

	sql := fmt.Sprintf(`
		SELECT za.uuid, za.title, COALESCE(za.description, '') AS description,
			   ts_rank_cd(za.search_vector, websearch_to_tsquery('%s', $1)) AS rank,
			   jsonb_build_object(
				   'name', za.name,
				   'language', za.language,
				   'category', COALESCE(za.category, ''),
				   'creator', COALESCE(za.creator, ''),
				   'article_count', za.article_count
			   ) AS extra,
			   COUNT(*) OVER () AS total
		FROM zim_archives za
		WHERE za.search_vector @@ websearch_to_tsquery('%s', $1)
		  AND za.is_enabled = true
		  %s
		ORDER BY rank DESC
		LIMIT $%d OFFSET $%d`,
		tsConfig, tsConfig, where, argIdx, argIdx+1,
	)
	args = append(args, limit, params.Offset)

	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("searching zim archives: %w", err)
	}
	hits, total, err := scanResultsWithTotal(rows, "zim_archive")
	if err != nil {
		return nil, err
	}
	if hits == nil {
		hits = []SearchResult{}
	}

	return &SearchResponse{
		Hits:           hits,
		EstimatedTotal: total,
	}, nil
}

// searchGitTemplates searches the git_templates table with FTS.
func (s *Searcher) searchGitTemplates(ctx context.Context, query string, params SearchParams, limit int64) (*SearchResponse, error) {
	var whereClauses []string
	var args []any
	args = append(args, query) // $1
	argIdx := 2

	if params.Category != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("gt.category = $%d", argIdx))
		args = append(args, params.Category)
		argIdx++
	}

	where := ""
	if len(whereClauses) > 0 {
		where = "AND " + strings.Join(whereClauses, " AND ")
	}

	sql := fmt.Sprintf(`
		SELECT gt.uuid, gt.name AS title, COALESCE(gt.description, '') AS description,
			   ts_rank_cd(gt.search_vector, websearch_to_tsquery('%s', $1)) AS rank,
			   jsonb_build_object(
				   'name', gt.name,
				   'slug', gt.slug,
				   'category', COALESCE(gt.category, ''),
				   'file_count', gt.file_count,
				   'status', gt.status,
				   'is_public', gt.is_public
			   ) AS extra,
			   COUNT(*) OVER () AS total
		FROM git_templates gt
		WHERE gt.search_vector @@ websearch_to_tsquery('%s', $1)
		  AND gt.deleted_at IS NULL
		  AND gt.is_enabled = true
		  %s
		ORDER BY rank DESC
		LIMIT $%d OFFSET $%d`,
		tsConfig, tsConfig, where, argIdx, argIdx+1,
	)
	args = append(args, limit, params.Offset)

	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("searching git templates: %w", err)
	}
	hits, total, err := scanResultsWithTotal(rows, "git_template")
	if err != nil {
		return nil, err
	}
	if hits == nil {
		hits = []SearchResult{}
	}

	return &SearchResponse{
		Hits:           hits,
		EstimatedTotal: total,
	}, nil
}

// FileSearchResult holds a file-level match from git_template_files.
type FileSearchResult struct {
	TemplateUUID string  `json:"template_uuid"`
	TemplateName string  `json:"template_name"`
	FilePath     string  `json:"file_path"`
	Filename     string  `json:"filename"`
	Score        float64 `json:"score"`
}

// SearchGitTemplateFiles searches file content within git_template_files using FTS.
func (s *Searcher) SearchGitTemplateFiles(ctx context.Context, query string, limit int64) ([]FileSearchResult, error) {
	if limit <= 0 {
		limit = 20
	}

	expanded := ExpandSynonyms(query)

	sql := fmt.Sprintf(`
		SELECT gt.uuid, gt.name, f.path, f.filename,
		       ts_rank_cd(f.search_vector, websearch_to_tsquery('%s', $1)) AS rank
		FROM git_template_files f
		JOIN git_templates gt ON gt.id = f.git_template_id
		WHERE f.search_vector @@ websearch_to_tsquery('%s', $1)
		  AND gt.deleted_at IS NULL
		  AND gt.is_enabled = true
		ORDER BY rank DESC
		LIMIT $2`,
		tsConfig, tsConfig,
	)

	rows, err := s.db.Query(ctx, sql, expanded, limit)
	if err != nil {
		return nil, fmt.Errorf("searching git template files: %w", err)
	}
	defer rows.Close()

	var results []FileSearchResult
	for rows.Next() {
		var r FileSearchResult
		if err := rows.Scan(&r.TemplateUUID, &r.TemplateName, &r.FilePath, &r.Filename, &r.Score); err != nil {
			return nil, fmt.Errorf("scanning git template file result: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating git template file results: %w", err)
	}
	return results, nil
}

// buildDocumentFilters builds WHERE clauses and args for document search filters.
// Returns (where string, args, next arg index).
func (s *Searcher) buildDocumentFilters(params SearchParams, startIdx int) (where string, filterArgs []any, nextIdx int) {
	var clauses []string
	idx := startIdx

	if params.FileType != "" {
		clauses = append(clauses, fmt.Sprintf("AND d.file_type = $%d", idx))
		filterArgs = append(filterArgs, params.FileType)
		idx++
	}
	if len(params.Tags) > 0 {
		clauses = append(clauses, fmt.Sprintf(
			"AND EXISTS (SELECT 1 FROM document_tags dt WHERE dt.document_id = d.id AND dt.tag = ANY($%d))", idx))
		filterArgs = append(filterArgs, params.Tags)
		idx++
	}

	// Visibility filter
	if !params.IsAdmin {
		switch {
		case params.UserID != nil:
			clauses = append(clauses, fmt.Sprintf("AND (d.user_id = $%d OR d.is_public = true)", idx))
			filterArgs = append(filterArgs, *params.UserID)
			idx++
		default:
			// No user context (M2M) or explicit public filter — public only
			clauses = append(clauses, "AND d.is_public = true")
		}
	}

	return strings.Join(clauses, " "), filterArgs, idx
}

// trigramFallbackDocuments uses pg_trgm similarity for fuzzy matching on titles.
// Returns hits and their document IDs for downstream tag batching.
func (s *Searcher) trigramFallbackDocuments(ctx context.Context, params SearchParams, limit int64) ([]SearchResult, []int64, error) {
	where, args, argIdx := s.buildDocumentFilters(params, 2)

	sql := fmt.Sprintf(`
		SELECT d.id, d.uuid, d.title, COALESCE(d.description, '') AS description,
			   similarity(d.title, $1) AS rank,
			   jsonb_build_object(
				   'file_type', d.file_type,
				   'word_count', COALESCE(d.word_count, 0),
				   'content', COALESCE(d.content, ''),
				   'is_public', d.is_public,
				   'status', d.status
			   ) AS extra,
			   0::bigint AS total
		FROM documents d
		WHERE d.title %% $1
		  AND d.deleted_at IS NULL
		  %s
		ORDER BY rank DESC
		LIMIT $%d`,
		where, argIdx,
	)

	allArgs := append([]any{params.Query}, args...)
	allArgs = append(allArgs, limit)
	rows, err := s.db.Query(ctx, sql, allArgs...)
	if err != nil {
		return nil, nil, fmt.Errorf("trigram search: %w", err)
	}
	hits, ids, _, err := scanDocumentResults(rows)
	return hits, ids, err
}

// documentUnionClause builds the documents branch of a UNION ALL federated query.
func (s *Searcher) documentUnionClause(_ string, params FederatedSearchParams, argIdx int) (clause string, clauseArgs []any, nextIdx int) {
	var clauses []string

	// Visibility filter
	if !params.IsAdmin {
		switch {
		case params.UserID != nil:
			clauses = append(clauses, fmt.Sprintf("AND (d.user_id = $%d OR d.is_public = true)", argIdx))
			clauseArgs = append(clauseArgs, *params.UserID)
			argIdx++
		default:
			clauses = append(clauses, "AND d.is_public = true")
		}
	}

	where := strings.Join(clauses, " ")

	clause = fmt.Sprintf(`
		SELECT d.uuid, d.title, COALESCE(d.description, '') AS description,
			   'document'::text AS source,
			   ts_rank_cd(d.search_vector, websearch_to_tsquery('%s', $1)) AS rank,
			   jsonb_build_object(
				   'file_type', d.file_type,
				   'word_count', COALESCE(d.word_count, 0),
				   'is_public', d.is_public
			   ) AS extra
		FROM documents d
		WHERE d.search_vector @@ websearch_to_tsquery('%s', $1)
		  AND d.deleted_at IS NULL
		  %s`,
		tsConfig, tsConfig, where,
	)

	return clause, clauseArgs, argIdx
}

// zimArchiveUnionClause builds the zim_archives branch of a UNION ALL.
func zimArchiveUnionClause() string {
	return fmt.Sprintf(`
		SELECT za.uuid, za.title, COALESCE(za.description, '') AS description,
			   'zim_archive'::text AS source,
			   ts_rank_cd(za.search_vector, websearch_to_tsquery('%s', $1)) AS rank,
			   jsonb_build_object(
				   'name', za.name,
				   'language', za.language,
				   'category', COALESCE(za.category, '')
			   ) AS extra
		FROM zim_archives za
		WHERE za.search_vector @@ websearch_to_tsquery('%s', $1)
		  AND za.is_enabled = true`,
		tsConfig, tsConfig,
	)
}

// gitTemplateUnionClause builds the git_templates branch of a UNION ALL.
func gitTemplateUnionClause() string {
	return fmt.Sprintf(`
		SELECT gt.uuid, gt.name AS title, COALESCE(gt.description, '') AS description,
			   'git_template'::text AS source,
			   ts_rank_cd(gt.search_vector, websearch_to_tsquery('%s', $1)) AS rank,
			   jsonb_build_object(
				   'name', gt.name,
				   'category', COALESCE(gt.category, ''),
				   'file_count', gt.file_count
			   ) AS extra
		FROM git_templates gt
		WHERE gt.search_vector @@ websearch_to_tsquery('%s', $1)
		  AND gt.deleted_at IS NULL
		  AND gt.is_enabled = true`,
		tsConfig, tsConfig,
	)
}

// scanResultsWithTotal scans rows carrying uuid, title, description, rank,
// extra, and a windowed total count. Used by single-index search queries.
func scanResultsWithTotal(rows pgx.Rows, source string) ([]SearchResult, int64, error) {
	defer rows.Close()

	var results []SearchResult
	var total int64
	for rows.Next() {
		var r SearchResult
		var extra map[string]any
		if err := rows.Scan(&r.UUID, &r.Title, &r.Description, &r.Score, &extra, &total); err != nil {
			return nil, 0, fmt.Errorf("scanning %s result: %w", source, err)
		}
		r.Source = source
		r.Extra = extra
		results = append(results, r)
	}
	return results, total, rows.Err()
}

// scanDocumentResults scans the documents FTS/trigram projection, which carries
// the primary key id as its first column for post-scan tag batching.
func scanDocumentResults(rows pgx.Rows) (hits []SearchResult, ids []int64, total int64, err error) {
	defer rows.Close()

	for rows.Next() {
		var id int64
		var r SearchResult
		var extra map[string]any
		if scanErr := rows.Scan(&id, &r.UUID, &r.Title, &r.Description, &r.Score, &extra, &total); scanErr != nil {
			return nil, nil, 0, fmt.Errorf("scanning document result: %w", scanErr)
		}
		r.Source = "document"
		r.Extra = extra
		hits = append(hits, r)
		ids = append(ids, id)
	}
	return hits, ids, total, rows.Err()
}

// attachTags batch-loads tags for the given document IDs and writes them back
// into each SearchResult's Extra["tags"] as []any. Replaces a per-row
// correlated subquery that rebuilt the array during FTS scan.
func (s *Searcher) attachTags(ctx context.Context, hits []SearchResult, ids []int64) error {
	for i := range hits {
		if hits[i].Extra == nil {
			hits[i].Extra = map[string]any{}
		}
		hits[i].Extra["tags"] = []any{}
	}
	if len(ids) == 0 {
		return nil
	}

	rows, err := s.db.Query(ctx,
		`SELECT document_id, tag FROM document_tags WHERE document_id = ANY($1) ORDER BY document_id, tag`,
		ids)
	if err != nil {
		return fmt.Errorf("batch-fetching document tags: %w", err)
	}
	defer rows.Close()

	tagsByID := make(map[int64][]any, len(ids))
	for rows.Next() {
		var docID int64
		var tag string
		if err := rows.Scan(&docID, &tag); err != nil {
			return fmt.Errorf("scanning document tag: %w", err)
		}
		tagsByID[docID] = append(tagsByID[docID], tag)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating document tags: %w", err)
	}

	for i, id := range ids {
		if tags, ok := tagsByID[id]; ok {
			hits[i].Extra["tags"] = tags
		}
	}
	return nil
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
