package mcphandler

import (
	"testing"

	"git.999.haus/chris/DocuMCP-go/internal/search"
)

func TestIndexToSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		indexUID string
		want     string
	}{
		{
			name:     "documents index returns document",
			indexUID: search.IndexDocuments,
			want:     "document",
		},
		{
			name:     "git_templates index returns git_template",
			indexUID: search.IndexGitTemplates,
			want:     "git_template",
		},
		{
			name:     "zim_archives index returns zim_archive",
			indexUID: search.IndexZimArchives,
			want:     "zim_archive",
		},
		{
			name:     "unknown index returns input unchanged",
			indexUID: "unknown_index",
			want:     "unknown_index",
		},
		{
			name:     "empty string returns empty string",
			indexUID: "",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := indexToSource(tt.indexUID)
			if got != tt.want {
				t.Errorf("indexToSource(%q) = %q, want %q", tt.indexUID, got, tt.want)
			}
		})
	}
}
