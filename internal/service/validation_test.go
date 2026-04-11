package service

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateTags(t *testing.T) {
	cases := []struct {
		name    string
		tags    []string
		wantErr bool
		wantMsg string
	}{
		{
			name:    "nil tags is valid",
			tags:    nil,
			wantErr: false,
		},
		{
			name:    "empty slice is valid",
			tags:    []string{},
			wantErr: false,
		},
		{
			name:    "small set is valid",
			tags:    []string{"a", "b", "c"},
			wantErr: false,
		},
		{
			name:    "exactly max count is valid",
			tags:    makeTags(MaxTagsPerDocument, "t"),
			wantErr: false,
		},
		{
			name:    "over max count rejected",
			tags:    makeTags(MaxTagsPerDocument+1, "t"),
			wantErr: true,
			wantMsg: "maximum 50 tags allowed",
		},
		{
			name:    "exactly max length is valid",
			tags:    []string{strings.Repeat("x", MaxTagLength)},
			wantErr: false,
		},
		{
			name:    "over max length rejected",
			tags:    []string{strings.Repeat("x", MaxTagLength+1)},
			wantErr: true,
			wantMsg: "exceeds 100 characters",
		},
		{
			name:    "only one long tag among many rejected",
			tags:    []string{"ok", strings.Repeat("x", MaxTagLength+1), "also-ok"},
			wantErr: true,
			wantMsg: "tag at index 1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateTags(tc.tags)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !errors.Is(err, ErrTagValidation) {
					t.Errorf("error is not ErrTagValidation: %v", err)
				}
				if tc.wantMsg != "" && !strings.Contains(err.Error(), tc.wantMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.wantMsg)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func makeTags(n int, prefix string) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = prefix
	}
	return out
}
