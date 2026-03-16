package mcphandler

import "testing"

func TestIsValidFileType(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"markdown", "markdown", true},
		{"pdf", "pdf", true},
		{"PDF uppercase", "PDF", true},
		{"exe rejected", "exe", false},
		{"empty string", "", false},
		{"injection attempt", `pdf" AND 1=1`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidFileType(tt.in); got != tt.want {
				t.Errorf("isValidFileType(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestSanitizeFilterValue(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"normal string", "golang", "golang"},
		{"double quote", `foo"bar`, `foo\"bar`},
		{"backslash", `foo\bar`, `foo\\bar`},
		{"both quote and backslash", `a\"b`, `a\\\"b`},
		{"meilisearch operators in value", `tag" AND is_public = true`, `tag\" AND is_public = true`},
		{"empty string", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeFilterValue(tt.in); got != tt.want {
				t.Errorf("sanitizeFilterValue(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
