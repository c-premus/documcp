package stringutil

import "strings"

// Slugify converts a human-readable name to a URL-friendly slug.
// It lowercases, replaces spaces/hyphens/underscores with a single hyphen,
// strips all other non-alphanumeric characters, and trims leading/trailing hyphens.
func Slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		if r == ' ' || r == '-' || r == '_' {
			return '-'
		}
		return -1
	}, s)

	// Collapse consecutive hyphens.
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")

	return s
}
