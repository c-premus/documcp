package model

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// DocumentStatus represents the processing state of a document.
type DocumentStatus string

// Possible DocumentStatus values.
const (
	DocumentStatusPending  DocumentStatus = "pending"
	DocumentStatusUploaded DocumentStatus = "uploaded"
	DocumentStatusIndexed  DocumentStatus = "indexed"
	DocumentStatusFailed   DocumentStatus = "failed"
)

// Document represents a row in the "documents" table.
type Document struct {
	ID                   int64          `db:"id" json:"id"`
	UUID                 string         `db:"uuid" json:"uuid"`
	Title                string         `db:"title" json:"title"`
	Description          sql.NullString `db:"description" json:"description"`
	FileType             string         `db:"file_type" json:"file_type"`
	FilePath             string         `db:"file_path" json:"file_path"`
	FileSize             int64          `db:"file_size" json:"file_size"`
	MIMEType             string         `db:"mime_type" json:"mime_type"`
	URL                  sql.NullString `db:"url" json:"url"`
	Content              sql.NullString `db:"content" json:"content"`
	ContentHash          sql.NullString `db:"content_hash" json:"content_hash"`
	Metadata             sql.NullString `db:"metadata" json:"metadata"`
	ProcessedAt          sql.NullTime   `db:"processed_at" json:"processed_at"`
	WordCount            sql.NullInt64  `db:"word_count" json:"word_count"`
	UserID               sql.NullInt64  `db:"user_id" json:"user_id"`
	IsPublic             bool           `db:"is_public" json:"is_public"`
	Status               DocumentStatus `db:"status" json:"status"`
	SearchVector         any            `db:"search_vector" json:"-"`
	ErrorMessage         sql.NullString `db:"error_message" json:"error_message"`
	CreatedAt sql.NullTime `db:"created_at" json:"created_at"`
	UpdatedAt            sql.NullTime   `db:"updated_at" json:"updated_at"`
	DeletedAt            sql.NullTime   `db:"deleted_at" json:"deleted_at"`
}

// ParseMetadata decodes the JSON metadata string into the provided destination.
func (d *Document) ParseMetadata(dest any) error {
	if !d.Metadata.Valid {
		return nil
	}
	if err := json.Unmarshal([]byte(d.Metadata.String), dest); err != nil {
		return fmt.Errorf("unmarshaling document metadata: %w", err)
	}
	return nil
}

// DocumentVersion represents a row in the "document_versions" table.
type DocumentVersion struct {
	ID         int64          `db:"id" json:"id"`
	DocumentID int64          `db:"document_id" json:"document_id"`
	Version    int            `db:"version" json:"version"`
	FilePath   string         `db:"file_path" json:"file_path"`
	Content    sql.NullString `db:"content" json:"content"`
	Metadata   sql.NullString `db:"metadata" json:"metadata"`
	CreatedAt  sql.NullTime   `db:"created_at" json:"created_at"`
	UpdatedAt  sql.NullTime   `db:"updated_at" json:"updated_at"`
}

// ParseMetadata decodes the JSON metadata string into the provided destination.
func (dv *DocumentVersion) ParseMetadata(dest any) error {
	if !dv.Metadata.Valid {
		return nil
	}
	if err := json.Unmarshal([]byte(dv.Metadata.String), dest); err != nil {
		return fmt.Errorf("unmarshaling document version metadata: %w", err)
	}
	return nil
}

// DocumentTag represents a row in the "document_tags" table.
type DocumentTag struct {
	ID         int64        `db:"id" json:"id"`
	DocumentID int64        `db:"document_id" json:"document_id"`
	Tag        string       `db:"tag" json:"tag"`
	CreatedAt  sql.NullTime `db:"created_at" json:"created_at"`
	UpdatedAt  sql.NullTime `db:"updated_at" json:"updated_at"`
}
