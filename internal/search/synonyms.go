package search

import "strings"

// synonymMap defines bidirectional synonym mappings expanded at query time.
var synonymMap = map[string][]string{
	"php":                     {"hypertext-preprocessor"},
	"hypertext-preprocessor":  {"php"},
	"js":                      {"javascript"},
	"javascript":              {"js"},
	"ts":                      {"typescript"},
	"typescript":              {"ts"},
}

// ExpandSynonyms preprocesses a search query by expanding known synonym terms
// into OR groups compatible with websearch_to_tsquery. For example, "js tutorial"
// becomes "(js OR javascript) tutorial".
func ExpandSynonyms(query string) string {
	words := strings.Fields(query)
	if len(words) == 0 {
		return query
	}

	expanded := make([]string, 0, len(words))
	for _, word := range words {
		lower := strings.ToLower(word)
		if syns, ok := synonymMap[lower]; ok {
			parts := make([]string, 0, 1+len(syns))
			parts = append(parts, lower)
			parts = append(parts, syns...)
			expanded = append(expanded, "("+strings.Join(parts, " OR ")+")")
		} else {
			expanded = append(expanded, word)
		}
	}

	return strings.Join(expanded, " ")
}
