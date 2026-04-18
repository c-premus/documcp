package model

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// GitTemplateStatus represents the sync state of a git template.
type GitTemplateStatus string

// Possible GitTemplateStatus values.
const (
	GitTemplateStatusPending GitTemplateStatus = "pending"
	GitTemplateStatusSynced  GitTemplateStatus = "synced"
	GitTemplateStatusFailed  GitTemplateStatus = "failed"
)

// GitTemplate represents a row in the "git_templates" table.
type GitTemplate struct {
	ID             int64             `db:"id" json:"id"`
	UUID           string            `db:"uuid" json:"uuid"`
	Name           string            `db:"name" json:"name"`
	Slug           string            `db:"slug" json:"slug"`
	Description    sql.NullString    `db:"description" json:"description"`
	RepositoryURL  string            `db:"repository_url" json:"repository_url"`
	Branch         string            `db:"branch" json:"branch"`
	GitToken       sql.NullString    `db:"git_token" json:"-"`
	ReadmeContent  sql.NullString    `db:"readme_content" json:"readme_content"`
	Manifest       json.RawMessage   `db:"manifest" json:"manifest,omitempty"`
	Category       sql.NullString    `db:"category" json:"category"`
	Tags           json.RawMessage   `db:"tags" json:"tags,omitempty"`
	UserID         sql.NullInt64     `db:"user_id" json:"user_id"`
	IsPublic       bool              `db:"is_public" json:"is_public"`
	IsEnabled      bool              `db:"is_enabled" json:"is_enabled"`
	Status         GitTemplateStatus `db:"status" json:"status"`
	ErrorMessage   sql.NullString    `db:"error_message" json:"error_message"`
	LastSyncedAt   sql.NullTime      `db:"last_synced_at" json:"last_synced_at"`
	LastCommitSHA  sql.NullString    `db:"last_commit_sha" json:"last_commit_sha"`
	FileCount      int               `db:"file_count" json:"file_count"`
	TotalSizeBytes int64             `db:"total_size_bytes" json:"total_size_bytes"`
	FilePaths      sql.NullString    `db:"file_paths" json:"-"`
	SearchVector   any               `db:"search_vector" json:"-"`
	CreatedAt      sql.NullTime      `db:"created_at" json:"created_at"`
	UpdatedAt      sql.NullTime      `db:"updated_at" json:"updated_at"`
	DeletedAt      sql.NullTime      `db:"deleted_at" json:"deleted_at"`
}

// ParseTags decodes the JSONB tags column into a string slice.
// Returns (nil, nil) when tags is absent.
func (gt *GitTemplate) ParseTags() ([]string, error) {
	if len(gt.Tags) == 0 {
		return nil, nil
	}
	var tags []string
	if err := json.Unmarshal(gt.Tags, &tags); err != nil {
		return nil, fmt.Errorf("unmarshaling git template tags: %w", err)
	}
	return tags, nil
}

// ParseManifest decodes the JSONB manifest column into the provided destination.
// Returns nil when manifest is absent.
func (gt *GitTemplate) ParseManifest(dest any) error {
	if len(gt.Manifest) == 0 {
		return nil
	}
	if err := json.Unmarshal(gt.Manifest, dest); err != nil {
		return fmt.Errorf("unmarshaling git template manifest: %w", err)
	}
	return nil
}

// GitTemplateFile represents a row in the "git_template_files" table.
type GitTemplateFile struct {
	ID            int64          `db:"id" json:"id"`
	UUID          string         `db:"uuid" json:"uuid"`
	GitTemplateID int64          `db:"git_template_id" json:"git_template_id"`
	Path          string         `db:"path" json:"path"`
	Filename      string         `db:"filename" json:"filename"`
	Extension     sql.NullString `db:"extension" json:"extension"`
	Content       sql.NullString `db:"content" json:"content"`
	IsCompressed  bool           `db:"is_compressed" json:"is_compressed"`
	SizeBytes     int64          `db:"size_bytes" json:"size_bytes"`
	ContentHash   sql.NullString `db:"content_hash" json:"content_hash"`
	IsEssential   bool           `db:"is_essential" json:"is_essential"`
	Variables     sql.NullString `db:"variables" json:"variables"`
	SearchVector  any            `db:"search_vector" json:"-"`
	CreatedAt     sql.NullTime   `db:"created_at" json:"created_at"`
	UpdatedAt     sql.NullTime   `db:"updated_at" json:"updated_at"`
}

// ParseVariables decodes the JSON variables string into the provided destination.
func (f *GitTemplateFile) ParseVariables(dest any) error {
	if !f.Variables.Valid {
		return nil
	}
	if err := json.Unmarshal([]byte(f.Variables.String), dest); err != nil {
		return fmt.Errorf("unmarshaling git template file variables: %w", err)
	}
	return nil
}
