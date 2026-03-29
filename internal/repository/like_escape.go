package repository

import "strings"

// escapeLike escapes PostgreSQL LIKE/ILIKE special characters (%, _, \)
// so they match literally when used inside a LIKE pattern.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}
