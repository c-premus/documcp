package dto

import (
	"database/sql"
	"time"

	"github.com/c-premus/documcp/internal/model"
)

// FormatNullTime formats a sql.NullTime as RFC3339.
// Returns an empty string when the time is not valid (NULL).
func FormatNullTime(t sql.NullTime) string {
	if !t.Valid {
		return ""
	}
	return t.Time.Format(time.RFC3339)
}

// TagNames extracts tag name strings from a slice of DocumentTag models.
func TagNames(tags []model.DocumentTag) []string {
	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = t.Tag
	}
	return names
}
