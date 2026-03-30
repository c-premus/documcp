package model

import (
	"database/sql"
	"testing"
)

func TestZimArchive_ParseTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		tags    sql.NullString
		want    []string
		wantErr bool
	}{
		{
			name: "null tags",
			tags: sql.NullString{Valid: false},
			want: nil,
		},
		{
			name: "empty array",
			tags: sql.NullString{String: `[]`, Valid: true},
			want: []string{},
		},
		{
			name: "multiple tags",
			tags: sql.NullString{String: `["wikipedia","english","nopic"]`, Valid: true},
			want: []string{"wikipedia", "english", "nopic"},
		},
		{
			name:    "invalid JSON",
			tags:    sql.NullString{String: `{bad`, Valid: true},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			za := &ZimArchive{Tags: tt.tags}
			got, err := za.ParseTags()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.want == nil {
				if got != nil {
					t.Fatalf("got %v, want nil", got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d tags, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("tag[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
