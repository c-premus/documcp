package mcphandler

import "strings"

// validFileTypes is the whitelist of allowed document file types.
var validFileTypes = map[string]bool{
	"markdown": true,
	"pdf":      true,
	"docx":     true,
	"xlsx":     true,
	"html":     true,
}

// isValidFileType returns true if the file type is in the whitelist.
func isValidFileType(ft string) bool {
	return validFileTypes[strings.ToLower(ft)]
}

// sanitizeFilterValue escapes a string for safe use in a Meilisearch filter value.
// It escapes backslashes and double quotes which are meaningful in filter syntax.
func sanitizeFilterValue(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
