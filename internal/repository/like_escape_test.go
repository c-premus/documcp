package repository

import "testing"

func TestEscapeLike(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no special chars", "hello", "hello"},
		{"percent", "100%", `100\%`},
		{"underscore", "under_score", `under\_score`},
		{"backslash", `back\slash`, `back\\slash`},
		{"all special chars", `a%b_c\d`, `a\%b\_c\\d`},
		{"empty string", "", ""},
		{"only special chars", `%_\`, `\%\_\\`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := escapeLike(tc.input)
			if got != tc.want {
				t.Errorf("escapeLike(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
