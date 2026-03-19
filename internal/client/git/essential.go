package git

import (
	"path/filepath"
	"strings"
)

// essentialExact lists file paths that are essential when matched exactly
// (case-insensitive comparison on the base name).
var essentialExact = []string{
	"CLAUDE.md",
	"template.json",
	"README.md",
}

// IsEssentialFile checks if a file path matches essential file patterns.
// Essential patterns: CLAUDE.md, memory-bank/*.md, template.json, README.md, .claude/**/*.
func IsEssentialFile(path string) bool {
	// Normalize separators.
	clean := filepath.ToSlash(path)

	// Check exact base-name matches.
	base := filepath.Base(clean)
	for _, name := range essentialExact {
		if strings.EqualFold(base, name) {
			return true
		}
	}

	// memory-bank/*.md — one level deep, markdown files.
	if matched, _ := filepath.Match("memory-bank/*.md", clean); matched {
		return true
	}

	// .claude/**/* — anything under the .claude directory.
	if strings.HasPrefix(clean, ".claude/") {
		return true
	}

	return false
}
