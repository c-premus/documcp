// Package scope defines the canonical set of OAuth scopes and helpers for
// parsing, validating, and comparing space-delimited scope strings per RFC 6749.
package scope

import (
	"slices"
	"strings"
)

// Scope constants.
const (
	MCPAccess      = "mcp:access"
	MCPRead        = "mcp:read"
	MCPWrite       = "mcp:write"
	DocumentsRead  = "documents:read"
	DocumentsWrite = "documents:write"
	SearchRead     = "search:read"
	ZIMRead        = "zim:read"
	TemplatesRead  = "templates:read"
	TemplatesWrite = "templates:write"
	ServicesRead   = "services:read"
	ServicesWrite  = "services:write"
	Admin          = "admin"
)

// All is the set of all valid scopes.
var All = map[string]bool{
	MCPAccess:      true,
	MCPRead:        true,
	MCPWrite:       true,
	DocumentsRead:  true,
	DocumentsWrite: true,
	SearchRead:     true,
	ZIMRead:        true,
	TemplatesRead:  true,
	TemplatesWrite: true,
	ServicesRead:   true,
	ServicesWrite:  true,
	Admin:          true,
}

// Valid returns true if scope is a recognized scope string.
func Valid(s string) bool {
	return All[s]
}

// ParseScopes splits a space-delimited scope string into a deduplicated slice
// of non-empty scope tokens, preserving first-occurrence order.
func ParseScopes(s string) []string {
	seen := make(map[string]bool)
	var out []string
	for tok := range strings.SplitSeq(s, " ") {
		if tok != "" && !seen[tok] {
			seen[tok] = true
			out = append(out, tok)
		}
	}
	return out
}

// Normalize deduplicates and re-joins a scope string, preserving first-occurrence
// order. Use at input boundaries to ensure stored scope strings are clean.
func Normalize(s string) string {
	return strings.Join(ParseScopes(s), " ")
}

// IsSubset returns true if every scope in requested is also present in allowed.
// Both arguments are space-delimited strings per RFC 6749.
// An empty requested string is always a subset.
func IsSubset(requested, allowed string) bool {
	allowedSet := make(map[string]bool)
	for _, s := range ParseScopes(allowed) {
		allowedSet[s] = true
	}
	for _, s := range ParseScopes(requested) {
		if !allowedSet[s] {
			return false
		}
	}
	return true
}

// Intersect returns the scopes present in both a and b as a space-delimited
// string, preserving the order from a.
func Intersect(a, b string) string {
	bSet := make(map[string]bool)
	for _, s := range ParseScopes(b) {
		bSet[s] = true
	}
	var out []string
	for _, s := range ParseScopes(a) {
		if bSet[s] {
			out = append(out, s)
		}
	}
	return strings.Join(out, " ")
}

// Union returns the deduplicated union of scopes from a and b, sorted
// lexicographically.
func Union(a, b string) string {
	set := make(map[string]bool)
	for _, s := range ParseScopes(a) {
		set[s] = true
	}
	for _, s := range ParseScopes(b) {
		set[s] = true
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	slices.Sort(out)
	return strings.Join(out, " ")
}

// UserScopes returns the set of scopes a user is entitled to grant. Admins can
// grant all known scopes; regular users can grant DefaultScopes only.
func UserScopes(isAdmin bool) string {
	if isAdmin {
		out := make([]string, 0, len(All))
		for s := range All {
			out = append(out, s)
		}
		slices.Sort(out)
		return strings.Join(out, " ")
	}
	return DefaultScopes()
}

// ValidateAll returns the list of scopes in the space-delimited string that are
// not recognized. An empty input yields a nil slice.
func ValidateAll(scopes string) []string {
	var invalid []string
	for _, s := range ParseScopes(scopes) {
		if !All[s] {
			invalid = append(invalid, s)
		}
	}
	return invalid
}

// DefaultScopes returns the default read-only scope set for new client
// registrations.
func DefaultScopes() string {
	return strings.Join([]string{
		MCPAccess,
		DocumentsRead,
		SearchRead,
		ZIMRead,
		TemplatesRead,
		ServicesRead,
	}, " ")
}
