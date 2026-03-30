package search_test

import (
	"testing"

	"github.com/c-premus/documcp/internal/search"
)

func TestExpandSynonyms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		query string
		want  string
	}{
		{
			name:  "empty string returns empty",
			query: "",
			want:  "",
		},
		{
			name:  "only spaces returns only spaces",
			query: "   ",
			want:  "   ",
		},
		{
			name:  "unknown word passes through unchanged",
			query: "golang",
			want:  "golang",
		},
		{
			name:  "multiple unknown words pass through unchanged",
			query: "hello world",
			want:  "hello world",
		},
		{
			name:  "js expands to js OR javascript",
			query: "js",
			want:  "(js OR javascript)",
		},
		{
			name:  "javascript expands to javascript OR js",
			query: "javascript",
			want:  "(javascript OR js)",
		},
		{
			name:  "ts expands to ts OR typescript",
			query: "ts",
			want:  "(ts OR typescript)",
		},
		{
			name:  "typescript expands to typescript OR ts",
			query: "typescript",
			want:  "(typescript OR ts)",
		},
		{
			name:  "php expands to php OR hypertext-preprocessor",
			query: "php",
			want:  "(php OR hypertext-preprocessor)",
		},
		{
			name:  "hypertext-preprocessor expands to hypertext-preprocessor OR php",
			query: "hypertext-preprocessor",
			want:  "(hypertext-preprocessor OR php)",
		},
		{
			name:  "multiple words with one synonym",
			query: "js tutorial",
			want:  "(js OR javascript) tutorial",
		},
		{
			name:  "multiple words with multiple synonyms",
			query: "js ts framework",
			want:  "(js OR javascript) (ts OR typescript) framework",
		},
		{
			name:  "uppercase JS matches case-insensitively",
			query: "JS",
			want:  "(js OR javascript)",
		},
		{
			name:  "mixed case Js matches case-insensitively",
			query: "Js",
			want:  "(js OR javascript)",
		},
		{
			name:  "uppercase PHP matches case-insensitively",
			query: "PHP",
			want:  "(php OR hypertext-preprocessor)",
		},
		{
			name:  "uppercase TYPESCRIPT matches case-insensitively",
			query: "TYPESCRIPT",
			want:  "(typescript OR ts)",
		},
		{
			name:  "single space between words preserved",
			query: "js ts",
			want:  "(js OR javascript) (ts OR typescript)",
		},
		{
			name:  "mixed synonym and non-synonym words",
			query: "learn php quickly",
			want:  "learn (php OR hypertext-preprocessor) quickly",
		},
		{
			name:  "all three synonym pairs in one query",
			query: "php js ts",
			want:  "(php OR hypertext-preprocessor) (js OR javascript) (ts OR typescript)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := search.ExpandSynonyms(tt.query)
			if got != tt.want {
				t.Errorf("ExpandSynonyms(%q) = %q, want %q", tt.query, got, tt.want)
			}
		})
	}
}
