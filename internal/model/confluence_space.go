package model

import "database/sql"

// ConfluenceSpace represents a row in the "confluence_spaces" table.
type ConfluenceSpace struct {
	ID                int64          `db:"id" json:"id"`
	UUID              string         `db:"uuid" json:"uuid"`
	ConfluenceID      string         `db:"confluence_id" json:"confluence_id"`
	Key               string         `db:"key" json:"key"`
	Name              string         `db:"name" json:"name"`
	Description       sql.NullString `db:"description" json:"description"`
	Type              string         `db:"type" json:"type"`
	Status            string         `db:"status" json:"status"`
	HomepageID        sql.NullString `db:"homepage_id" json:"homepage_id"`
	IconURL           sql.NullString `db:"icon_url" json:"icon_url"`
	ExternalServiceID sql.NullInt64  `db:"external_service_id" json:"external_service_id"`
	IsEnabled         bool           `db:"is_enabled" json:"is_enabled"`
	IsSearchable      bool           `db:"is_searchable" json:"is_searchable"`
	LastSyncedAt      sql.NullTime   `db:"last_synced_at" json:"last_synced_at"`
	CreatedAt         sql.NullTime   `db:"created_at" json:"created_at"`
	UpdatedAt         sql.NullTime   `db:"updated_at" json:"updated_at"`
}
