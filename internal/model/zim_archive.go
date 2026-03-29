package model

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// ZimArchive represents a row in the "zim_archives" table.
type ZimArchive struct {
	ID                   int64          `db:"id" json:"id"`
	UUID                 string         `db:"uuid" json:"uuid"`
	Name                 string         `db:"name" json:"name"`
	Slug                 string         `db:"slug" json:"slug"`
	KiwixID              sql.NullString `db:"kiwix_id" json:"kiwix_id"`
	Title                string         `db:"title" json:"title"`
	Description          sql.NullString `db:"description" json:"description"`
	Language             string         `db:"language" json:"language"`
	Category             sql.NullString `db:"category" json:"category"`
	Creator              sql.NullString `db:"creator" json:"creator"`
	Publisher            sql.NullString `db:"publisher" json:"publisher"`
	Favicon              sql.NullString `db:"favicon" json:"favicon"`
	ArticleCount         int64          `db:"article_count" json:"article_count"`
	MediaCount           int64          `db:"media_count" json:"media_count"`
	FileSize             int64          `db:"file_size" json:"file_size"`
	Tags                 sql.NullString `db:"tags" json:"tags"`
	ExternalServiceID    sql.NullInt64  `db:"external_service_id" json:"external_service_id"`
	IsEnabled            bool           `db:"is_enabled" json:"is_enabled"`
	IsSearchable         bool           `db:"is_searchable" json:"is_searchable"`
	LastSyncedAt         sql.NullTime   `db:"last_synced_at" json:"last_synced_at"`
	CreatedAt sql.NullTime `db:"created_at" json:"created_at"`
	UpdatedAt            sql.NullTime   `db:"updated_at" json:"updated_at"`
}

// ParseTags decodes the JSON tags string into a string slice.
func (za *ZimArchive) ParseTags() ([]string, error) {
	if !za.Tags.Valid {
		return nil, nil
	}
	var tags []string
	if err := json.Unmarshal([]byte(za.Tags.String), &tags); err != nil {
		return nil, fmt.Errorf("unmarshaling zim archive tags: %w", err)
	}
	return tags, nil
}
