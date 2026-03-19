package model

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// SearchQuery represents a row in the "search_queries" table.
type SearchQuery struct {
	ID           int64          `db:"id" json:"id"`
	UserID       sql.NullInt64  `db:"user_id" json:"user_id"`
	Query        string         `db:"query" json:"query"`
	ResultsCount int            `db:"results_count" json:"results_count"`
	Filters      sql.NullString `db:"filters" json:"filters"`
	CreatedAt    sql.NullTime   `db:"created_at" json:"created_at"`
	UpdatedAt    sql.NullTime   `db:"updated_at" json:"updated_at"`
}

// ParseFilters decodes the JSON filters string into the provided destination.
func (sq *SearchQuery) ParseFilters(dest any) error {
	if !sq.Filters.Valid {
		return nil
	}
	if err := json.Unmarshal([]byte(sq.Filters.String), dest); err != nil {
		return fmt.Errorf("unmarshaling search query filters: %w", err)
	}
	return nil
}
