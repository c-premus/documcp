package model

import (
	"encoding/json"
	"testing"
)

func TestZimArchive_ParseTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		tags    json.RawMessage
		want    []string
		wantErr bool
	}{
		{
			name: "null tags",
			tags: nil,
			want: nil,
		},
		{
			name: "empty array",
			tags: json.RawMessage(`[]`),
			want: []string{},
		},
		{
			name: "multiple tags",
			tags: json.RawMessage(`["wikipedia","english","nopic"]`),
			want: []string{"wikipedia", "english", "nopic"},
		},
		{
			name:    "invalid JSON",
			tags:    json.RawMessage(`{bad`),
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
