package kiwix

import (
	"testing"
	"time"
)

func TestCache_GetMiss(t *testing.T) {
	c := newCache()

	val, ok := c.get("nonexistent")
	if ok {
		t.Error("expected miss for nonexistent key")
	}
	if val != nil {
		t.Errorf("expected nil value, got %v", val)
	}
}

func TestCache_SetAndGet(t *testing.T) {
	c := newCache()

	c.set("key", "value", 1*time.Hour)

	val, ok := c.get("key")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if val != "value" {
		t.Errorf("value = %v, want %q", val, "value")
	}
}

func TestCache_Expiration(t *testing.T) {
	c := newCache()

	// Set with a very short TTL.
	c.set("ephemeral", "data", 1*time.Millisecond)

	// Wait for expiration.
	time.Sleep(5 * time.Millisecond)

	val, ok := c.get("ephemeral")
	if ok {
		t.Error("expected miss for expired key")
	}
	if val != nil {
		t.Errorf("expected nil for expired entry, got %v", val)
	}

	// Verify the entry was deleted from the map.
	c.mu.RLock()
	_, exists := c.entries["ephemeral"]
	c.mu.RUnlock()
	if exists {
		t.Error("expired entry should be removed from map")
	}
}

func TestCache_OverwriteKey(t *testing.T) {
	c := newCache()

	c.set("key", "first", 1*time.Hour)
	c.set("key", "second", 1*time.Hour)

	val, ok := c.get("key")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if val != "second" {
		t.Errorf("value = %v, want %q", val, "second")
	}
}

func TestValidateArchiveName(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "valid simple name",
			input:   "devdocs-go",
			wantErr: false,
		},
		{
			name:    "valid name with dots",
			input:   "wikipedia.en.2025",
			wantErr: false,
		},
		{
			name:      "empty name",
			input:     "",
			wantErr:   true,
			errSubstr: "must not be empty",
		},
		{
			name:      "contains forward slash",
			input:     "archive/name",
			wantErr:   true,
			errSubstr: "path separators",
		},
		{
			name:      "contains backslash",
			input:     "archive\\name",
			wantErr:   true,
			errSubstr: "path separators",
		},
		{
			name:      "contains dot-dot",
			input:     "archive..traversal",
			wantErr:   true,
			errSubstr: "dot-dot",
		},
		{
			name:      "contains null byte",
			input:     "archive\x00name",
			wantErr:   true,
			errSubstr: "null bytes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateArchiveName(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errSubstr != "" && !containsStr(err.Error(), tt.errSubstr) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errSubstr)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// containsStr avoids importing strings in this test file.
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
