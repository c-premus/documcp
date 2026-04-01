package security

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SafeStoragePath validates that filePath resolves inside storagePath,
// preventing path-traversal attacks via manipulated DB values.
// Returns the joined absolute path or an error.
func SafeStoragePath(storagePath, filePath string) (string, error) {
	fullPath := filepath.Join(storagePath, filePath)

	absStorage, err := filepath.Abs(storagePath)
	if err != nil {
		return "", fmt.Errorf("resolving storage root: %w", err)
	}
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("resolving target path: %w", err)
	}
	if !strings.HasPrefix(absPath, absStorage+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %q escapes storage root", filePath)
	}
	return absPath, nil
}
