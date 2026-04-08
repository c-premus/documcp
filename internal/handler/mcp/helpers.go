package mcphandler

import (
	"database/sql"
	"strings"
	"time"

	"github.com/c-premus/documcp/internal/model"
)

// clampPagination enforces a default and maximum limit on a pagination value.
func clampPagination(limit, defaultLimit, maxLimit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

// clampOffset returns offset clamped to a minimum of 0.
func clampOffset(offset int) int {
	return max(offset, 0)
}

// formatNullTime formats a sql.NullTime as RFC3339.
// Returns an empty string when the time is not valid (NULL).
func formatNullTime(t sql.NullTime) string {
	if !t.Valid {
		return ""
	}
	return t.Time.Format(time.RFC3339)
}

// tagNames extracts tag name strings from a slice of DocumentTag models.
func tagNames(tags []model.DocumentTag) []string {
	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = t.Tag
	}
	return names
}

// truncateContent applies summary_only or max_paragraphs truncation to content.
// It returns the (possibly truncated) content and whether truncation occurred.
func truncateContent(content string, summaryOnly bool, maxParagraphs int) (string, bool) {
	if content == "" {
		return content, false
	}

	if summaryOnly {
		// Return content before the first heading (# or ##).
		lines := strings.Split(content, "\n")
		var result []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") && len(result) > 0 {
				break
			}
			result = append(result, line)
		}
		truncated := strings.Join(result, "\n")
		return strings.TrimSpace(truncated), len(truncated) < len(content)
	}

	if maxParagraphs > 100 {
		maxParagraphs = 100
	}
	if maxParagraphs > 0 {
		paragraphs := strings.Split(content, "\n\n")
		if maxParagraphs < len(paragraphs) {
			truncated := strings.Join(paragraphs[:maxParagraphs], "\n\n")
			return truncated, true
		}
	}

	return content, false
}

// truncate shortens s to maxLen characters, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// defaultArg returns the value for key from args, or fallback if empty/missing.
func defaultArg(args map[string]string, key, fallback string) string {
	if v := args[key]; v != "" {
		return v
	}
	return fallback
}
