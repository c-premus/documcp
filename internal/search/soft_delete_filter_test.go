package search

import (
	"testing"
)

func TestSoftDeleteFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		indexUID string
		want     string
	}{
		{
			name:     "documents index returns soft delete filter",
			indexUID: IndexDocuments,
			want:     "__soft_deleted = false",
		},
		{
			name:     "git_templates index returns soft delete filter",
			indexUID: IndexGitTemplates,
			want:     "__soft_deleted = false",
		},
		{
			name:     "zim_archives index returns empty filter",
			indexUID: IndexZimArchives,
			want:     "",
		},
		{
			name:     "unknown index returns empty filter",
			indexUID: "unknown_index",
			want:     "",
		},
		{
			name:     "empty string returns empty filter",
			indexUID: "",
			want:     "",
		},
		{
			name:     "case sensitive mismatch returns empty filter",
			indexUID: "Documents",
			want:     "",
		},
		{
			name:     "index with trailing space returns empty filter",
			indexUID: "documents ",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := softDeleteFilter(tt.indexUID)
			if got != tt.want {
				t.Errorf("softDeleteFilter(%q) = %q, want %q", tt.indexUID, got, tt.want)
			}
		})
	}
}
