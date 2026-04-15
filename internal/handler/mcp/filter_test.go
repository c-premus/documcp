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
