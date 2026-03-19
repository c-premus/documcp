// Package scope defines the canonical set of OAuth scopes and helpers for
// parsing, validating, and comparing space-delimited scope strings per RFC 6749.
package scope

import "strings"

// Scope constants.
const (
	MCPAccess      = "mcp:access"
	MCPRead        = "mcp:read"
	MCPWrite       = "mcp:write"
	DocumentsRead  = "documents:read"
	DocumentsWrite = "documents:write"
	SearchRead     = "search:read"
	ZIMRead        = "zim:read"
	ConfluenceRead = "confluence:read"
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
	ConfluenceRead: true,
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

// ParseScopes splits a space-delimited scope string into a slice of non-empty
// scope tokens.
func ParseScopes(s string) []string {
	var out []string
	for _, tok := range strings.Split(s, " ") {
		if tok != "" {
			out = append(out, tok)
		}
	}
	return out
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
		ConfluenceRead,
		TemplatesRead,
		ServicesRead,
	}, " ")
}
