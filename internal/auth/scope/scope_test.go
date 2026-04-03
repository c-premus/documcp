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
		{"duplicates removed", "mcp:access mcp:access documents:read", []string{"mcp:access", "documents:read"}},
		{"duplicates preserve first-occurrence order", "admin mcp:access admin documents:read mcp:access", []string{"admin", "mcp:access", "documents:read"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, ParseScopes(tt.in))
		})
	}
}

func TestNormalize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"no duplicates unchanged", "mcp:access documents:read", "mcp:access documents:read"},
		{"duplicates removed", "mcp:access mcp:access documents:read", "mcp:access documents:read"},
		{"extra spaces collapsed", "  mcp:access   search:read  ", "mcp:access search:read"},
		{"empty string", "", ""},
		{"single scope", "admin", "admin"},
		{"preserves first-occurrence order", "admin mcp:access admin documents:read mcp:access", "admin mcp:access documents:read"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, Normalize(tt.in))
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

func TestIntersect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    string
		b    string
		want string
	}{
		{"overlapping scopes", "mcp:access documents:read admin", "documents:read admin search:read", "documents:read admin"},
		{"no overlap", "mcp:access mcp:read", "documents:read admin", ""},
		{"a empty", "", "mcp:access documents:read", ""},
		{"b empty", "mcp:access documents:read", "", ""},
		{"both empty", "", "", ""},
		{"identical strings", "mcp:access documents:read", "mcp:access documents:read", "mcp:access documents:read"},
		{"order preserved from a", "admin mcp:access documents:read", "documents:read mcp:access admin", "admin mcp:access documents:read"},
		{"duplicate scopes in a", "mcp:access mcp:access documents:read", "mcp:access", "mcp:access"},
		{"single scope overlap", "mcp:access", "mcp:access admin", "mcp:access"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, Intersect(tt.a, tt.b))
		})
	}
}

func TestUnion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    string
		b    string
		want string
	}{
		{"disjoint sets combined", "mcp:access", "documents:read", "documents:read mcp:access"},
		{"overlapping sets deduplicated", "mcp:access documents:read", "documents:read admin", "admin documents:read mcp:access"},
		{"a empty", "", "mcp:access documents:read", "documents:read mcp:access"},
		{"b empty", "mcp:access documents:read", "", "documents:read mcp:access"},
		{"both empty", "", "", ""},
		{"identical strings", "mcp:access documents:read", "mcp:access documents:read", "documents:read mcp:access"},
		{"result is sorted lexicographically", "zim:read admin mcp:access", "documents:read", "admin documents:read mcp:access zim:read"},
		{"single scope each", "admin", "mcp:access", "admin mcp:access"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, Union(tt.a, tt.b))
		})
	}
}

func TestUserScopes(t *testing.T) {
	t.Parallel()

	t.Run("admin gets all scopes", func(t *testing.T) {
		t.Parallel()
		got := ParseScopes(UserScopes(true))
		assert.Len(t, got, len(All), "admin should receive all %d scopes", len(All))
	})

	t.Run("admin scopes are sorted", func(t *testing.T) {
		t.Parallel()
		got := ParseScopes(UserScopes(true))
		for i := 1; i < len(got); i++ {
			assert.True(t, got[i-1] <= got[i],
				"admin scopes not sorted: %q should come before %q", got[i-1], got[i])
		}
	})

	t.Run("all admin scopes are valid", func(t *testing.T) {
		t.Parallel()
		for _, s := range ParseScopes(UserScopes(true)) {
			assert.True(t, Valid(s), "admin scope %q should be valid", s)
		}
	})

	t.Run("non-admin gets default scopes only", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, DefaultScopes(), UserScopes(false))
	})
}

func TestDefaultScopes(t *testing.T) {
	t.Parallel()

	got := DefaultScopes()
	assert.Equal(t,
		"mcp:access documents:read search:read zim:read templates:read services:read",
		got,
	)

	// Every scope in the default string must be valid.
	for _, s := range ParseScopes(got) {
		assert.True(t, Valid(s), "default scope %q should be valid", s)
	}
}
