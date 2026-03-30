package stringutil

import "testing"

func TestSlugify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "simple lowercase name", input: "hello world", want: "hello-world"},
		{name: "mixed case is lowered", input: "Hello World", want: "hello-world"},
		{name: "hyphens preserved", input: "my-template", want: "my-template"},
		{name: "underscores become hyphens", input: "my_template_name", want: "my-template-name"},
		{name: "special characters removed", input: "hello! @world #2024", want: "hello-world-2024"},
		{name: "combining accent removed but base letter kept", input: "cafe\u0301 template", want: "cafe-template"},
		{name: "leading and trailing spaces trimmed", input: "  my template  ", want: "my-template"},
		{name: "empty string", input: "", want: ""},
		{name: "only spaces", input: "   ", want: ""},
		{name: "only special characters", input: "!@#$%^&*()", want: ""},
		{name: "consecutive spaces collapse to single hyphen", input: "hello   world", want: "hello-world"},
		{name: "consecutive hyphens collapse", input: "hello---world", want: "hello-world"},
		{name: "mixed separators collapse", input: "hello - _ world", want: "hello-world"},
		{name: "numbers preserved", input: "template 123", want: "template-123"},
		{name: "leading special chars trimmed", input: "---hello", want: "hello"},
		{name: "trailing special chars trimmed", input: "hello---", want: "hello"},
		{name: "long name with many words", input: "This Is A Really Long Template Name With Many Words", want: "this-is-a-really-long-template-name-with-many-words"},
		{name: "single character", input: "a", want: "a"},
		{name: "single digit", input: "9", want: "9"},
		{name: "dots removed", input: "docker.compose.yml", want: "dockercomposeyml"},
		{name: "CJK characters removed", input: "hello \u4e16\u754c", want: "hello"},
		{name: "emoji removed", input: "rocket \U0001f680 template", want: "rocket-template"},
		{name: "mixed case with numbers and dots", input: "Service 2.0", want: "service-20"},
		{name: "triple hyphens in input", input: "a---b", want: "a-b"},
		{name: "already hyphenated", input: "already-hyphenated", want: "already-hyphenated"},
		{name: "all special chars", input: "!!!", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Slugify(tt.input)
			if got != tt.want {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
