package security_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/c-premus/documcp/internal/security"
)

func TestSafeStoragePath(t *testing.T) {
	t.Parallel()

	// Create a real temp directory for the storage root.
	storageDir := t.TempDir()

	// Create a file inside storage.
	validFile := filepath.Join(storageDir, "docs", "file.pdf")
	if err := os.MkdirAll(filepath.Dir(validFile), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(validFile, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		filePath  string
		wantErr   bool
		wantPath  string // empty = don't check exact path
	}{
		{
			name:     "valid nested path",
			filePath: "docs/file.pdf",
			wantPath: validFile,
		},
		{
			name:    "traversal with dot-dot",
			filePath: "../etc/passwd",
			wantErr: true,
		},
		{
			name:    "traversal with encoded dot-dot",
			filePath: "docs/../../etc/passwd",
			wantErr: true,
		},
		{
			name:     "non-existent file falls back to Abs",
			filePath: "does-not-exist.txt",
			wantErr:  false, // Abs fallback — subsequent os.Open will fail
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := security.SafeStoragePath(storageDir, tt.filePath)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got path %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantPath != "" && got != tt.wantPath {
				t.Errorf("got %q, want %q", got, tt.wantPath)
			}
		})
	}
}

func TestSafeStoragePath_SymlinkEscape(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	outsideDir := t.TempDir()

	// Create a target file outside storage.
	outsideFile := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Create a symlink inside storage pointing outside.
	symlink := filepath.Join(storageDir, "escape")
	if err := os.Symlink(outsideDir, symlink); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	// The symlink resolves outside storage — must be rejected.
	_, err := security.SafeStoragePath(storageDir, "escape/secret.txt")
	if err == nil {
		t.Error("expected error for symlink escape, got nil")
	}
}
