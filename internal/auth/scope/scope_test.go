package scope

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		scope string
		want  bool
	}{
		{"known scope mcp:access", MCPAccess, true},
		{"known scope admin", Admin, true},
		{"known scope documents:write", DocumentsWrite, true},
		{"unknown scope", "bogus:scope", false},
		{"empty string", "", false},
		{"prefix of valid scope", "mcp", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, Valid(tt.scope))
		})
	}
}

func TestParseScopes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"normal string", "mcp:access documents:read", []string{"mcp:access", "documents:read"}},
		{"single scope", "admin", []string{"admin"}},
		{"empty string", "", nil},
		{"extra spaces", "  mcp:access   search:read  ", []string{"mcp:access", "search:read"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, ParseScopes(tt.in))
		})
	}
}

func TestIsSubset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		requested string
		allowed   string
		want      bool
	}{
		{"exact match", "mcp:access", "mcp:access", true},
		{"subset", "mcp:access", "mcp:access documents:read", true},
		{"superset is not subset", "mcp:access documents:read", "mcp:access", false},
		{"empty requested", "", "mcp:access", true},
		{"empty allowed with non-empty requested", "mcp:access", "", false},
		{"both empty", "", "", true},
		{"different order", "documents:read mcp:access", "mcp:access documents:read", true},
		{"disjoint", "admin", "mcp:access", false},
		{"partial overlap", "mcp:access admin", "mcp:access documents:read", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, IsSubset(tt.requested, tt.allowed))
		})
	}
}

func TestValidateAll(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"all valid", "mcp:access documents:read admin", nil},
		{"some invalid", "mcp:access bogus nope", []string{"bogus", "nope"}},
		{"all invalid", "foo bar", []string{"foo", "bar"}},
		{"empty string", "", nil},
		{"single valid", "admin", nil},
		{"single invalid", "nope", []string{"nope"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, ValidateAll(tt.in))
		})
	}
}

func TestDefaultScopes(t *testing.T) {
	t.Parallel()

	got := DefaultScopes()
	assert.Equal(t,
		"mcp:access documents:read search:read zim:read confluence:read templates:read services:read",
		got,
	)

	// Every scope in the default string must be valid.
	for _, s := range ParseScopes(got) {
		assert.True(t, Valid(s), "default scope %q should be valid", s)
	}
}
