package testutil

import (
	"github.com/c-premus/documcp/internal/model"
)

// SearchQueryOption configures a SearchQuery created by NewSearchQuery.
type SearchQueryOption func(*model.SearchQuery)

// NewSearchQuery returns a SearchQuery with sensible defaults.
func NewSearchQuery(opts ...SearchQueryOption) *model.SearchQuery {
	sq := &model.SearchQuery{
		Query:        "test search",
		ResultsCount: 10,
	}
	for _, opt := range opts {
		opt(sq)
	}
	return sq
}

// WithSearchQueryUserID sets the search query user ID on the builder.
func WithSearchQueryUserID(uid int64) SearchQueryOption {
	return func(sq *model.SearchQuery) { sq.UserID = nullInt64(uid) }
}

// WithSearchQueryQuery sets the search query text on the builder.
func WithSearchQueryQuery(q string) SearchQueryOption {
	return func(sq *model.SearchQuery) { sq.Query = q }
}

// WithSearchQueryResultsCount sets the search query results count on the builder.
func WithSearchQueryResultsCount(n int) SearchQueryOption {
	return func(sq *model.SearchQuery) { sq.ResultsCount = n }
}

// WithSearchQueryFilters sets the search query filters on the builder.
func WithSearchQueryFilters(filters string) SearchQueryOption {
	return func(sq *model.SearchQuery) { sq.Filters = nullString(filters) }
}
